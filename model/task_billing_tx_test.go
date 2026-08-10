package model

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func prepareTaskBillingDeltaTxTest(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&TaskBillingJob{}))
	require.NoError(t, DB.Exec("DELETE FROM task_billing_jobs").Error)
	t.Cleanup(func() { _ = DB.Exec("DELETE FROM task_billing_jobs").Error })
}

func seedTaskBillingDeltaTx(t *testing.T, source string, fromQuota int, targetQuota int) (*Task, *TaskBillingJob) {
	t.Helper()
	operation := TaskBillingOperationSettle
	if targetQuota == 0 {
		operation = TaskBillingOperationRefund
	}
	return seedTaskBillingDeltaTxWithOperation(t, source, fromQuota, operation)
}

func seedTaskBillingDeltaTxWithOperation(t *testing.T, source string, fromQuota int, operation TaskBillingOperation) (*Task, *TaskBillingJob) {
	t.Helper()
	user := &User{Id: 8101, Username: "billing-tx-user", Password: "billing-tx-password", Quota: 1000, AuthVersion: 1}
	require.NoError(t, DB.Create(user).Error)
	token := &Token{Id: 8102, UserId: user.Id, Key: "billing-tx-token", RemainQuota: 700, UsedQuota: fromQuota}
	require.NoError(t, DB.Create(token).Error)
	channel := &Channel{Id: 8103, Name: "billing-tx-channel", UsedQuota: 0}
	require.NoError(t, DB.Create(channel).Error)

	task := &Task{
		TaskID:    "billing-tx-task",
		UserId:    user.Id,
		ChannelId: channel.Id,
		Quota:     fromQuota,
		Status:    TaskStatusSuccess,
		PrivateData: TaskPrivateData{
			BillingSource: source,
			TokenId:       token.Id,
		},
	}
	insertTask(t, task)

	job := &TaskBillingJob{
		TaskID:         task.ID,
		IdempotencyKey: taskBillingJobIdempotencyKey(task.ID),
		Operation:      operation,
		FromQuota:      fromQuota,
		Status:         TaskBillingJobStatusProcessing,
		Attempts:       1,
		LockedBy:       "billing-worker",
		LockedUntil:    time.Now().Add(time.Minute).Unix(),
	}
	require.NoError(t, DB.Create(job).Error)
	return task, job
}

func TestTaskBillingDeltaTxZeroQuotaSettleCountsRequestButRefundDoesNot(t *testing.T) {
	for _, tc := range []struct {
		name         string
		fromQuota    int
		operation    TaskBillingOperation
		wantRequests int
	}{
		{name: "successful zero quota settle", fromQuota: 0, operation: TaskBillingOperationSettle, wantRequests: 1},
		{name: "zero target refund", fromQuota: 200, operation: TaskBillingOperationRefund, wantRequests: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareTaskBillingDeltaTxTest(t)
			task, _ := seedTaskBillingDeltaTxWithOperation(t, "wallet", tc.fromQuota, tc.operation)

			require.NoError(t, applyTaskBillingDeltaForTest(t, task, 0))
			var user User
			require.NoError(t, DB.First(&user, task.UserId).Error)
			assert.Equal(t, tc.wantRequests, user.RequestCount)
			assert.Zero(t, user.UsedQuota)
		})
	}
}

func TestTaskBillingDeltaTxPositiveSettleRecordsBoundedWalletAndTokenDebt(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, 350)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", task.UserId).Update("quota", 50).Error)
	require.NoError(t, DB.Model(&Token{}).Where("id = ?", task.PrivateData.TokenId).Update("remain_quota", 50).Error)

	require.NoError(t, applyTaskBillingDeltaForTest(t, task, 350))
	user, token, _, storedTask, job := reloadTaskBillingDeltaRows(t, task)
	assert.Equal(t, -100, user.Quota)
	assert.Equal(t, -100, token.RemainQuota)
	assert.Equal(t, 350, token.UsedQuota)
	assert.Equal(t, 350, storedTask.Quota)
	assert.Equal(t, TaskBillingJobStatusSucceeded, job.Status)
}

func TestTaskBillingDeltaTxUnlimitedTokenPreservesSettleAndRefundAccounting(t *testing.T) {
	for _, tc := range []struct {
		name       string
		target     int
		wantRemain int
		wantUsed   int
	}{
		{name: "settle", target: 350, wantRemain: 550, wantUsed: 350},
		{name: "refund", target: 0, wantRemain: 900, wantUsed: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareTaskBillingDeltaTxTest(t)
			task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, tc.target)
			require.NoError(t, DB.Model(&Token{}).Where("id = ?", task.PrivateData.TokenId).Update("unlimited_quota", true).Error)

			require.NoError(t, applyTaskBillingDeltaForTest(t, task, tc.target))
			_, token, _, _, _ := reloadTaskBillingDeltaRows(t, task)
			assert.True(t, token.UnlimitedQuota)
			assert.Equal(t, tc.wantRemain, token.RemainQuota)
			assert.Equal(t, tc.wantUsed, token.UsedQuota)
		})
	}
}

func TestTaskBillingDeltaTxSucceededNoOpReloadsAuthoritativeTaskIdentity(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, 350)
	require.NoError(t, applyTaskBillingDeltaForTest(t, task, 350))

	stale := &Task{ID: task.ID, UserId: 99991, ChannelId: 99992, PrivateData: TaskPrivateData{TokenId: 99993}}
	require.NoError(t, applyTaskBillingDeltaForTest(t, stale, 350))
	assert.Equal(t, task.UserId, stale.UserId)
	assert.Equal(t, task.ChannelId, stale.ChannelId)
	assert.Equal(t, task.PrivateData.TokenId, stale.PrivateData.TokenId)
	assert.Equal(t, 350, stale.Quota)
}

func TestTaskBillingDeltaPostCommitCacheSyncRepairsWalletSettleAndRefund(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fromQuota int
		target    int
		wantQuota int
	}{
		{name: "settle", fromQuota: 200, target: 350, wantQuota: 850},
		{name: "refund", fromQuota: 200, target: 0, wantQuota: 1200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareTaskBillingDeltaTxTest(t)
			server := useUserCacheMiniRedis(t)
			task, _ := seedTaskBillingDeltaTx(t, "wallet", tc.fromQuota, tc.target)
			var user User
			require.NoError(t, DB.First(&user, task.UserId).Error)
			require.NoError(t, populateUserCache(user))
			assert.True(t, server.Exists(getUserCacheKey(user.Id)))

			require.NoError(t, ApplyTaskBillingDelta(task, tc.target, "billing-worker", 1))
			cachedQuota, err := getUserQuotaCache(user.Id)
			require.NoError(t, err)
			assert.Equal(t, tc.wantQuota, cachedQuota)
		})
	}
}

func TestTaskBillingDeltaPostCommitCacheFailureNeverReplaysDatabaseDelta(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	useUserCacheMiniRedis(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, 350)
	require.NoError(t, common.RDB.Close())

	require.NoError(t, ApplyTaskBillingDelta(task, 350, "billing-worker", 1))
	require.NoError(t, ApplyTaskBillingDelta(task, 350, "billing-worker", 1))
	user, token, channel, storedTask, job := reloadTaskBillingDeltaRows(t, task)
	assert.Equal(t, 850, user.Quota)
	assert.Equal(t, 550, token.RemainQuota)
	assert.EqualValues(t, 350, channel.UsedQuota)
	assert.Equal(t, 350, storedTask.Quota)
	assert.Equal(t, TaskBillingJobStatusSucceeded, job.Status)
}

func TestTaskBillingDeltaTxFinalCASLoserRollsBackEveryRow(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, 350)
	const callbackName = "test:force_task_billing_final_cas_loser"
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(db *gorm.DB) {
		if db.Statement.Schema != nil && db.Statement.Schema.Name == "TaskBillingJob" {
			db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "1 = 0"}}})
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Update().Remove(callbackName) })

	require.ErrorIs(t, applyTaskBillingDeltaForTest(t, task, 350), errTaskBillingJobLeaseLost)
	user, token, channel, storedTask, job := reloadTaskBillingDeltaRows(t, task)
	assert.Equal(t, 1000, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Equal(t, 700, token.RemainQuota)
	assert.Equal(t, 200, token.UsedQuota)
	assert.Zero(t, channel.UsedQuota)
	assert.Equal(t, 200, storedTask.Quota)
	assert.Equal(t, TaskBillingJobStatusProcessing, job.Status)
}

func TestTaskBillingDeltaTxConcurrentSameClaimAppliesOnlyOneTarget(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, 350)
	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, target := range []int{350, 400} {
		target := target
		go func() {
			ready.Done()
			<-start
			candidate := &Task{ID: task.ID}
			results <- ApplyTaskBillingDelta(candidate, target, "billing-worker", 1)
		}()
	}
	ready.Wait()
	close(start)
	err1, err2 := <-results, <-results
	assert.NotEqual(t, err1 == nil, err2 == nil, "exactly one competing target must commit")

	var storedTask Task
	require.NoError(t, DB.First(&storedTask, task.ID).Error)
	assert.Contains(t, []int{350, 400}, storedTask.Quota)
	user, token, channel, _, job := reloadTaskBillingDeltaRows(t, task)
	assert.Equal(t, 1200-storedTask.Quota, user.Quota)
	assert.Equal(t, 900-storedTask.Quota, token.RemainQuota)
	assert.Equal(t, storedTask.Quota, token.UsedQuota)
	assert.EqualValues(t, storedTask.Quota, channel.UsedQuota)
	assert.Equal(t, TaskBillingJobStatusSucceeded, job.Status)
}

func TestTaskBillingDeltaTxConfiguredDatabaseConcurrentClaims(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		dbType    common.DatabaseType
		dialector func(string) gorm.Dialector
	}{
		{name: "mysql", env: "TEST_MYSQL_DSN", dbType: common.DatabaseTypeMySQL, dialector: func(dsn string) gorm.Dialector {
			return mysql.Open(dsn)
		}},
		{name: "postgres", env: "TEST_POSTGRES_DSN", dbType: common.DatabaseTypePostgreSQL, dialector: func(dsn string) gorm.Dialector {
			return postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(tc.env))
			if dsn == "" {
				t.Skip(tc.env + " is not configured")
			}
			db, err := gorm.Open(tc.dialector(dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(4)
			defer sqlDB.Close()
			require.NoError(t, db.AutoMigrate(&User{}, &Token{}, &Channel{}, &Task{}, &TaskBillingJob{}))

			oldDB := DB
			oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
			oldRedisEnabled := common.RedisEnabled
			DB = db
			common.SetDatabaseTypes(tc.dbType, oldLogType)
			common.RedisEnabled = false
			defer func() {
				DB = oldDB
				common.SetDatabaseTypes(oldMainType, oldLogType)
				common.RedisEnabled = oldRedisEnabled
			}()

			suffix := fmt.Sprintf("%s-%d", tc.name, time.Now().UnixNano())
			user := &User{Username: "billing-tx-" + suffix, Password: "billing-tx-password", Quota: 1000, AuthVersion: 1}
			require.NoError(t, db.Create(user).Error)
			token := &Token{UserId: user.Id, Key: "billing-token-" + suffix, RemainQuota: 700, UsedQuota: 200}
			require.NoError(t, db.Create(token).Error)
			channel := &Channel{Name: "billing-channel-" + suffix}
			require.NoError(t, db.Create(channel).Error)
			task := &Task{
				TaskID: "billing-task-" + suffix, UserId: user.Id, ChannelId: channel.Id,
				Quota: 200, Status: TaskStatusSuccess,
				PrivateData: TaskPrivateData{BillingSource: taskBillingSourceWallet, TokenId: token.Id},
			}
			require.NoError(t, db.Create(task).Error)
			job := &TaskBillingJob{
				TaskID: task.ID, IdempotencyKey: taskBillingJobIdempotencyKey(task.ID),
				Operation: TaskBillingOperationSettle, FromQuota: 200,
				Status: TaskBillingJobStatusProcessing, Attempts: 1,
				LockedBy: "billing-worker", LockedUntil: time.Now().Add(time.Minute).Unix(),
			}
			require.NoError(t, db.Create(job).Error)
			defer func() {
				_ = db.Where("task_id = ?", task.ID).Delete(&TaskBillingJob{}).Error
				_ = db.Delete(&Task{}, task.ID).Error
				_ = db.Delete(&Token{}, token.Id).Error
				_ = db.Delete(&Channel{}, channel.Id).Error
				_ = db.Delete(&User{}, user.Id).Error
			}()

			start := make(chan struct{})
			results := make(chan error, 2)
			for _, target := range []int{350, 400} {
				target := target
				go func() {
					<-start
					results <- ApplyTaskBillingDelta(&Task{ID: task.ID}, target, "billing-worker", 1)
				}()
			}
			close(start)
			err1, err2 := <-results, <-results
			assert.NotEqual(t, err1 == nil, err2 == nil, "exactly one configured-database transaction must commit")

			var storedTask Task
			require.NoError(t, db.First(&storedTask, task.ID).Error)
			assert.Contains(t, []int{350, 400}, storedTask.Quota)
			var storedUser User
			var storedToken Token
			var storedChannel Channel
			var storedJob TaskBillingJob
			require.NoError(t, db.First(&storedUser, user.Id).Error)
			require.NoError(t, db.First(&storedToken, token.Id).Error)
			require.NoError(t, db.First(&storedChannel, channel.Id).Error)
			require.NoError(t, db.Where("task_id = ?", task.ID).First(&storedJob).Error)
			assert.Equal(t, 1200-storedTask.Quota, storedUser.Quota)
			assert.Equal(t, 900-storedTask.Quota, storedToken.RemainQuota)
			assert.Equal(t, storedTask.Quota, storedToken.UsedQuota)
			assert.EqualValues(t, storedTask.Quota, storedChannel.UsedQuota)
			assert.Equal(t, TaskBillingJobStatusSucceeded, storedJob.Status)
		})
	}
}

func applyTaskBillingDeltaForTest(t *testing.T, task *Task, targetQuota int) error {
	t.Helper()
	return applyTaskBillingDeltaWithClaimForTest(t, task, targetQuota, "billing-worker", 1)
}

func applyTaskBillingDeltaWithClaimForTest(t *testing.T, task *Task, targetQuota int, expectedLockedBy string, expectedAttempt int) error {
	t.Helper()
	return DB.Transaction(func(tx *gorm.DB) error {
		return ApplyTaskBillingDeltaTx(tx, task, targetQuota, expectedLockedBy, expectedAttempt)
	})
}

func reloadTaskBillingDeltaRows(t *testing.T, task *Task) (User, Token, Channel, Task, TaskBillingJob) {
	t.Helper()
	var user User
	var token Token
	var channel Channel
	var storedTask Task
	var job TaskBillingJob
	require.NoError(t, DB.First(&user, task.UserId).Error)
	require.NoError(t, DB.First(&token, task.PrivateData.TokenId).Error)
	require.NoError(t, DB.First(&channel, task.ChannelId).Error)
	require.NoError(t, DB.First(&storedTask, task.ID).Error)
	require.NoError(t, DB.Where("task_id = ?", task.ID).First(&job).Error)
	return user, token, channel, storedTask, job
}

func TestTaskBillingDeltaTxWalletSettleAndSecondApplicationIsNoOp(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, 350)

	require.NoError(t, applyTaskBillingDeltaForTest(t, task, 350))
	user, token, channel, storedTask, job := reloadTaskBillingDeltaRows(t, task)
	assert.Equal(t, 850, user.Quota)
	assert.Equal(t, 350, user.UsedQuota)
	assert.Equal(t, 1, user.RequestCount)
	assert.Equal(t, 550, token.RemainQuota)
	assert.Equal(t, 350, token.UsedQuota)
	assert.EqualValues(t, 350, channel.UsedQuota)
	assert.Equal(t, 350, storedTask.Quota)
	assert.Equal(t, TaskBillingJobStatusSucceeded, job.Status)
	require.NotNil(t, job.TargetQuota)
	assert.Equal(t, 350, *job.TargetQuota)

	require.NoError(t, applyTaskBillingDeltaForTest(t, task, 350))
	user2, token2, channel2, storedTask2, job2 := reloadTaskBillingDeltaRows(t, task)
	assert.Equal(t, user.Quota, user2.Quota)
	assert.Equal(t, user.UsedQuota, user2.UsedQuota)
	assert.Equal(t, user.RequestCount, user2.RequestCount)
	assert.Equal(t, token.RemainQuota, token2.RemainQuota)
	assert.Equal(t, token.UsedQuota, token2.UsedQuota)
	assert.Equal(t, channel.UsedQuota, channel2.UsedQuota)
	assert.Equal(t, storedTask.Quota, storedTask2.Quota)
	assert.Equal(t, job.Status, job2.Status)
}

func TestTaskBillingDeltaTxWalletRefund(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, 0)

	require.NoError(t, applyTaskBillingDeltaForTest(t, task, 0))
	user, token, channel, storedTask, job := reloadTaskBillingDeltaRows(t, task)
	assert.Equal(t, 1200, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, user.RequestCount)
	assert.Equal(t, 900, token.RemainQuota)
	assert.Zero(t, token.UsedQuota)
	assert.Zero(t, channel.UsedQuota)
	assert.Zero(t, storedTask.Quota)
	assert.Equal(t, TaskBillingJobStatusSucceeded, job.Status)
}

func TestTaskBillingDeltaTxSubscriptionSettleAndRefund(t *testing.T) {
	for _, tc := range []struct {
		name       string
		target     int
		wantUsed   int64
		wantRemain int
		wantStats  int
	}{
		{name: "settle", target: 350, wantUsed: 350, wantRemain: 550, wantStats: 350},
		{name: "refund", target: 0, wantUsed: 0, wantRemain: 900, wantStats: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareTaskBillingDeltaTxTest(t)
			task, _ := seedTaskBillingDeltaTx(t, "subscription", 200, tc.target)
			sub := &UserSubscription{Id: 8104, UserId: task.UserId, AmountTotal: 1000, AmountUsed: 200, Status: "active", EndTime: time.Now().Add(time.Hour).Unix()}
			require.NoError(t, DB.Create(sub).Error)
			task.PrivateData.SubscriptionId = sub.Id
			require.NoError(t, DB.Model(&Task{}).Where("id = ?", task.ID).Update("private_data", task.PrivateData).Error)

			require.NoError(t, applyTaskBillingDeltaForTest(t, task, tc.target))
			var storedSub UserSubscription
			require.NoError(t, DB.First(&storedSub, sub.Id).Error)
			user, token, channel, storedTask, job := reloadTaskBillingDeltaRows(t, task)
			assert.EqualValues(t, tc.wantUsed, storedSub.AmountUsed)
			assert.Equal(t, 1000, user.Quota)
			assert.Equal(t, tc.wantStats, user.UsedQuota)
			assert.Equal(t, tc.wantRemain, token.RemainQuota)
			assert.EqualValues(t, tc.wantStats, channel.UsedQuota)
			assert.Equal(t, tc.target, storedTask.Quota)
			assert.Equal(t, TaskBillingJobStatusSucceeded, job.Status)
		})
	}
}

func TestTaskBillingDeltaTxRollsBackAllRowsWhenTokenUpdateFails(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 200, 350)
	require.NoError(t, DB.Delete(&Token{}, task.PrivateData.TokenId).Error)

	require.Error(t, applyTaskBillingDeltaForTest(t, task, 350))
	var user User
	var channel Channel
	var storedTask Task
	var job TaskBillingJob
	require.NoError(t, DB.First(&user, task.UserId).Error)
	require.NoError(t, DB.First(&channel, task.ChannelId).Error)
	require.NoError(t, DB.First(&storedTask, task.ID).Error)
	require.NoError(t, DB.Where("task_id = ?", task.ID).First(&job).Error)
	assert.Equal(t, 1000, user.Quota)
	assert.Zero(t, user.UsedQuota)
	assert.Zero(t, channel.UsedQuota)
	assert.Equal(t, 200, storedTask.Quota)
	assert.Equal(t, TaskBillingJobStatusProcessing, job.Status)
	assert.Nil(t, job.TargetQuota)
}

func TestTaskBillingDeltaTxSaturatesStatisticsWithoutWrapping(t *testing.T) {
	prepareTaskBillingDeltaTxTest(t)
	task, _ := seedTaskBillingDeltaTx(t, "wallet", 100, 100)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", task.UserId).Updates(map[string]any{
		"used_quota":    common.MaxQuota - 10,
		"request_count": common.MaxQuota,
	}).Error)
	require.NoError(t, DB.Model(&Channel{}).Where("id = ?", task.ChannelId).Update("used_quota", int64(math.MaxInt64-10)).Error)

	require.NoError(t, applyTaskBillingDeltaForTest(t, task, 100))
	user, _, channel, _, _ := reloadTaskBillingDeltaRows(t, task)
	assert.Equal(t, common.MaxQuota, user.UsedQuota)
	assert.Equal(t, common.MaxQuota, user.RequestCount)
	assert.EqualValues(t, int64(math.MaxInt64), channel.UsedQuota)
}

func TestTaskBillingDeltaTxRejectsInvalidFenceAndFromQuota(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(task *Task, job *TaskBillingJob)
	}{
		{name: "missing attempt", mutate: func(_ *Task, job *TaskBillingJob) { require.NoError(t, DB.Model(job).Update("attempts", 0).Error) }},
		{name: "expired lease", mutate: func(_ *Task, job *TaskBillingJob) {
			require.NoError(t, DB.Model(job).Update("locked_until", time.Now().Add(-time.Minute).Unix()).Error)
		}},
		{name: "from quota changed", mutate: func(task *Task, _ *TaskBillingJob) { require.NoError(t, DB.Model(task).Update("quota", 199).Error) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareTaskBillingDeltaTxTest(t)
			task, job := seedTaskBillingDeltaTx(t, "wallet", 200, 350)
			tc.mutate(task, job)

			require.Error(t, applyTaskBillingDeltaForTest(t, task, 350))
			var user User
			require.NoError(t, DB.First(&user, task.UserId).Error)
			assert.Equal(t, 1000, user.Quota)
		})
	}
}

func TestTaskBillingDeltaTxRejectsStaleClaimAfterReclaim(t *testing.T) {
	for _, tc := range []struct {
		name        string
		newLockedBy string
	}{
		{name: "same worker newer attempt", newLockedBy: "billing-worker"},
		{name: "different worker newer attempt", newLockedBy: "replacement-worker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareTaskBillingDeltaTxTest(t)
			task, job := seedTaskBillingDeltaTx(t, "wallet", 200, 350)
			require.NoError(t, DB.Model(job).Updates(map[string]any{
				"attempts":     2,
				"locked_by":    tc.newLockedBy,
				"locked_until": time.Now().Add(time.Minute).Unix(),
			}).Error)

			err := applyTaskBillingDeltaWithClaimForTest(t, task, 350, "billing-worker", 1)
			require.ErrorIs(t, err, errTaskBillingJobLeaseLost)

			user, token, channel, storedTask, storedJob := reloadTaskBillingDeltaRows(t, task)
			assert.Equal(t, 1000, user.Quota)
			assert.Zero(t, user.UsedQuota)
			assert.Equal(t, 700, token.RemainQuota)
			assert.Zero(t, channel.UsedQuota)
			assert.Equal(t, 200, storedTask.Quota)
			assert.Equal(t, TaskBillingJobStatusProcessing, storedJob.Status)
			assert.Equal(t, 2, storedJob.Attempts)
			assert.Equal(t, tc.newLockedBy, storedJob.LockedBy)
		})
	}
}
