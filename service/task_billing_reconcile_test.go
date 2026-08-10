package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTaskBillingReconciliationTest(t *testing.T) {
	t.Helper()
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.TaskBillingJob{}))
	require.NoError(t, model.DB.Exec("DELETE FROM task_billing_jobs").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM task_billing_jobs")
	})
}

func seedReconciliationJob(t *testing.T, task *model.Task, operation model.TaskBillingOperation, target *int, now int64) *model.TaskBillingJob {
	t.Helper()
	require.NoError(t, model.DB.Create(task).Error)
	job := &model.TaskBillingJob{
		TaskID:         task.ID,
		IdempotencyKey: fmt.Sprintf("task3:%d", task.ID),
		Operation:      operation,
		FromQuota:      task.Quota,
		TargetQuota:    target,
		Status:         model.TaskBillingJobStatusPending,
		NextAttemptAt:  now,
	}
	require.NoError(t, model.DB.Create(job).Error)
	return job
}

func loadReconciliationJob(t *testing.T, id int64) model.TaskBillingJob {
	t.Helper()
	var job model.TaskBillingJob
	require.NoError(t, model.DB.First(&job, id).Error)
	return job
}

func loadReconciliationTask(t *testing.T, id int64) model.Task {
	t.Helper()
	var task model.Task
	require.NoError(t, model.DB.First(&task, id).Error)
	return task
}

func TestResolveTerminalBillingIntentRefundSettleAndZero(t *testing.T) {
	zero := 0
	settled := 37

	tests := []struct {
		name string
		job  *model.TaskBillingJob
		want int
	}{
		{name: "refund ignores absent target", job: &model.TaskBillingJob{Operation: model.TaskBillingOperationRefund}, want: 0},
		{name: "settle", job: &model.TaskBillingJob{Operation: model.TaskBillingOperationSettle, TargetQuota: &settled}, want: 37},
		{name: "settle target zero remains settle", job: &model.TaskBillingJob{Operation: model.TaskBillingOperationSettle, TargetQuota: &zero}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTerminalBillingIntent(tt.job)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := ResolveTerminalBillingIntent(&model.TaskBillingJob{Operation: model.TaskBillingOperationSettle})
	assert.ErrorIs(t, err, ErrTaskBillingIntentIndeterminate)
	_, err = ResolveTerminalBillingIntent(&model.TaskBillingJob{Operation: "unknown"})
	assert.ErrorIs(t, err, ErrTaskBillingIntentIndeterminate)
}

func TestTaskBillingReconciliationRefund(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	now := time.Now().Unix()
	seedUser(t, 301, 900)
	seedToken(t, 301, 301, "reconcile-refund", 900)
	seedChannel(t, 301)
	task := makeTask(301, 301, 100, 301, BillingSourceWallet, 0)
	job := seedReconciliationJob(t, task, model.TaskBillingOperationRefund, nil, now)

	summary, err := runTaskBillingReconciliationOnceAt(context.Background(), "refund-worker", now)
	require.NoError(t, err)
	assert.Equal(t, TaskBillingReconciliationSummary{Claimed: 1, Succeeded: 1}, summary)
	assert.Equal(t, 1000, getUserQuota(t, 301))
	assert.Equal(t, 1000, getTokenRemainQuota(t, 301))
	assert.Equal(t, 0, loadReconciliationTask(t, task.ID).Quota)
	assert.Equal(t, model.TaskBillingJobStatusSucceeded, loadReconciliationJob(t, job.ID).Status)
	var event model.Log
	require.NoError(t, model.LOG_DB.Where("request_id = ?", fmt.Sprintf("taskbill_%d", job.ID)).First(&event).Error)
	assert.Equal(t, model.LogTypeRefund, event.Type)
}

func TestTaskBillingReconciliationSettleAndTargetZero(t *testing.T) {
	tests := []struct {
		name       string
		userID     int
		target     int
		wantWallet int
	}{
		{name: "settle positive", userID: 302, target: 35, wantWallet: 965},
		{name: "settle target zero", userID: 303, target: 0, wantWallet: 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTaskBillingReconciliationTest(t)
			now := time.Now().Unix()
			seedUser(t, tt.userID, 900)
			seedToken(t, tt.userID, tt.userID, fmt.Sprintf("reconcile-%d", tt.userID), 900)
			seedChannel(t, tt.userID)
			task := makeTask(tt.userID, tt.userID, 100, tt.userID, BillingSourceWallet, 0)
			job := seedReconciliationJob(t, task, model.TaskBillingOperationSettle, &tt.target, now)

			summary, err := runTaskBillingReconciliationOnceAt(context.Background(), "settle-worker", now)
			require.NoError(t, err)
			assert.Equal(t, 1, summary.Succeeded)
			assert.Equal(t, tt.wantWallet, getUserQuota(t, tt.userID))
			assert.Equal(t, tt.target, loadReconciliationTask(t, task.ID).Quota)
			assert.Equal(t, model.TaskBillingJobStatusSucceeded, loadReconciliationJob(t, job.ID).Status)
			var event model.Log
			require.NoError(t, model.LOG_DB.Where("request_id = ?", fmt.Sprintf("taskbill_%d", job.ID)).First(&event).Error)
			assert.Equal(t, model.LogTypeConsume, event.Type)
		})
	}
}

func TestTaskBillingReconciliationRetryBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 5 * time.Second},
		{attempt: 2, want: 30 * time.Second},
		{attempt: 3, want: 2 * time.Minute},
		{attempt: 4, want: 10 * time.Minute},
		{attempt: 5, want: time.Hour},
		{attempt: 9, want: time.Hour},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			assert.Equal(t, tt.want, taskBillingRetryDelay(tt.attempt))
		})
	}

	for _, tt := range []struct {
		name             string
		userID           int
		startingAttempts int
		wantDelay        int64
	}{
		{name: "first failure gets full five seconds", userID: 304, startingAttempts: 0, wantDelay: 5},
		{name: "second failure gets full thirty seconds", userID: 311, startingAttempts: 1, wantDelay: 30},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupTaskBillingReconciliationTest(t)
			claimNow := time.Now().Unix()
			failureNow := claimNow + 90
			seedUser(t, tt.userID, 900)
			seedToken(t, tt.userID, tt.userID, fmt.Sprintf("reconcile-backoff-%d", tt.userID), 900)
			seedChannel(t, tt.userID)
			task := makeTask(tt.userID, tt.userID, 100, tt.userID, BillingSourceSubscription, 999999)
			job := seedReconciliationJob(t, task, model.TaskBillingOperationRefund, nil, claimNow)
			require.NoError(t, model.DB.Model(&model.TaskBillingJob{}).Where("id = ?", job.ID).
				Update("attempts", tt.startingAttempts).Error)

			var clockCalls atomic.Int32
			clock := func() int64 {
				if clockCalls.Add(1) == 1 {
					return claimNow
				}
				return failureNow
			}
			summary, err := runTaskBillingReconciliationOnceWithClock(context.Background(), "retry-worker", clock)
			require.NoError(t, err)
			assert.Equal(t, 1, summary.Rescheduled)
			got := loadReconciliationJob(t, job.ID)
			assert.Equal(t, model.TaskBillingJobStatusPending, got.Status)
			assert.Equal(t, tt.startingAttempts+1, got.Attempts)
			assert.Equal(t, failureNow+tt.wantDelay, got.NextAttemptAt)
		})
	}
}

func TestTaskBillingReconciliationTenthAttemptRequiresReview(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	now := time.Now().Unix()
	seedUser(t, 305, 900)
	seedToken(t, 305, 305, "reconcile-review", 900)
	seedChannel(t, 305)
	task := makeTask(305, 305, 100, 305, BillingSourceSubscription, 999999)
	job := seedReconciliationJob(t, task, model.TaskBillingOperationRefund, nil, now)
	require.NoError(t, model.DB.Model(&model.TaskBillingJob{}).Where("id = ?", job.ID).Update("attempts", 9).Error)

	summary, err := runTaskBillingReconciliationOnceAt(context.Background(), "review-worker", now)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.ReviewRequired)
	got := loadReconciliationJob(t, job.ID)
	assert.Equal(t, model.TaskBillingJobStatusReviewRequired, got.Status)
	assert.Equal(t, 10, got.Attempts)
	assert.NotEmpty(t, got.LastError)
}

func TestTaskBillingReconciliationIndeterminateIntentRequiresReviewImmediately(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	now := time.Now().Unix()
	seedUser(t, 306, 900)
	seedChannel(t, 306)
	task := makeTask(306, 306, 100, 0, BillingSourceWallet, 0)
	job := seedReconciliationJob(t, task, model.TaskBillingOperationSettle, nil, now)

	summary, err := runTaskBillingReconciliationOnceAt(context.Background(), "intent-worker", now)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.ReviewRequired)
	got := loadReconciliationJob(t, job.ID)
	assert.Equal(t, model.TaskBillingJobStatusReviewRequired, got.Status)
	assert.Equal(t, 1, got.Attempts)
	assert.Equal(t, 900, getUserQuota(t, 306))
}

func TestTaskBillingReconciliationReclaimsExpiredLease(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	now := time.Now().Unix()
	seedUser(t, 307, 900)
	seedToken(t, 307, 307, "reconcile-expired", 900)
	seedChannel(t, 307)
	task := makeTask(307, 307, 100, 307, BillingSourceWallet, 0)
	job := seedReconciliationJob(t, task, model.TaskBillingOperationRefund, nil, now)
	require.NoError(t, model.DB.Model(&model.TaskBillingJob{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status":       model.TaskBillingJobStatusProcessing,
		"attempts":     1,
		"locked_by":    "dead-worker",
		"locked_until": now - 1,
	}).Error)

	summary, err := runTaskBillingReconciliationOnceAt(context.Background(), "replacement-worker", now)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Succeeded)
	got := loadReconciliationJob(t, job.ID)
	assert.Equal(t, model.TaskBillingJobStatusSucceeded, got.Status)
	assert.Equal(t, 2, got.Attempts)
	assert.Equal(t, 1000, getUserQuota(t, 307))
}

func TestTaskBillingReconciliationConcurrentWorkersApplyOnce(t *testing.T) {
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	dsn := fmt.Sprintf("file:task3-reconcile-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	require.Equal(t, 4, sqlDB.Stats().MaxOpenConnections)
	require.NoError(t, db.AutoMigrate(
		&model.Task{}, &model.User{}, &model.Token{}, &model.Log{}, &model.Channel{},
		&model.UserSubscription{}, &model.TaskBillingJob{},
	))
	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		require.NoError(t, sqlDB.Close())
	})

	now := time.Now().Unix()
	seedUser(t, 308, 900)
	seedToken(t, 308, 308, "reconcile-concurrent", 900)
	seedChannel(t, 308)
	target := 40
	task := makeTask(308, 308, 100, 308, BillingSourceWallet, 0)
	job := seedReconciliationJob(t, task, model.TaskBillingOperationSettle, &target, now)

	var activeClaims atomic.Int32
	var maxActiveClaims atomic.Int32
	var maxConnectionsInUse atomic.Int32
	claimGate := make(chan struct{})
	callbackName := "test:task_billing_claim_competition"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "TaskBillingJob" {
			return
		}
		connectionsInUse := int32(sqlDB.Stats().InUse)
		for {
			observed := maxConnectionsInUse.Load()
			if connectionsInUse <= observed || maxConnectionsInUse.CompareAndSwap(observed, connectionsInUse) {
				break
			}
		}
		arrival := activeClaims.Add(1)
		defer activeClaims.Add(-1)
		for {
			observed := maxActiveClaims.Load()
			if arrival <= observed || maxActiveClaims.CompareAndSwap(observed, arrival) {
				break
			}
		}
		if arrival == 2 {
			close(claimGate)
		}
		select {
		case <-claimGate:
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(func() { db.Callback().Query().Remove(callbackName) })

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, runnerID := range []string{"worker-a", "worker-b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			<-start
			_, err := runTaskBillingReconciliationOnceAt(context.Background(), id, now)
			errs <- err
		}(runnerID)
	}
	close(start)
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		assert.Contains(t, err.Error(), "locked")
	}
	assert.GreaterOrEqual(t, successes, 1)
	assert.GreaterOrEqual(t, maxActiveClaims.Load(), int32(2), "claims must overlap on distinct pooled connections")
	assert.GreaterOrEqual(t, maxConnectionsInUse.Load(), int32(2), "two SQL connections must be in use during claim competition")
	assert.Equal(t, model.TaskBillingJobStatusSucceeded, loadReconciliationJob(t, job.ID).Status)
	assert.Equal(t, 960, getUserQuota(t, 308))
	assert.Equal(t, 960, getTokenRemainQuota(t, 308))
	assert.Equal(t, 40, loadReconciliationTask(t, task.ID).Quota)
}

func TestTaskBillingReconciliationPropagatesCancellationToDatabase(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	claimNow := time.Now().Unix()
	failureNow := claimNow + 20
	seedUser(t, 312, 900)
	seedToken(t, 312, 312, "reconcile-cancel", 900)
	seedChannel(t, 312)
	task := makeTask(312, 312, 100, 312, BillingSourceWallet, 0)
	job := seedReconciliationJob(t, task, model.TaskBillingOperationRefund, nil, claimNow)

	type contextMarker struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextMarker{}, "task3"))
	callbackName := "test:cancel_task_billing_query"
	var sawContext atomic.Bool
	var canceledQuery atomic.Bool
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "Task" {
			return
		}
		if !canceledQuery.CompareAndSwap(false, true) {
			return
		}
		if tx.Statement.Context.Value(contextMarker{}) == "task3" {
			sawContext.Store(true)
		}
		cancel()
		tx.AddError(context.Canceled)
	}))
	t.Cleanup(func() { model.DB.Callback().Query().Remove(callbackName) })

	var clockCalls atomic.Int32
	clock := func() int64 {
		if clockCalls.Add(1) == 1 {
			return claimNow
		}
		return failureNow
	}
	summary, err := runTaskBillingReconciliationOnceWithClock(ctx, "cancel-worker", clock)
	require.ErrorIs(t, err, context.Canceled)
	assert.True(t, sawContext.Load())
	assert.Equal(t, 1, summary.Rescheduled)
	got := loadReconciliationJob(t, job.ID)
	assert.Equal(t, model.TaskBillingJobStatusPending, got.Status)
	assert.Equal(t, failureNow+5, got.NextAttemptAt)
	assert.Equal(t, 900, getUserQuota(t, 312))
	assert.Equal(t, 100, loadReconciliationTask(t, task.ID).Quota)
}

func TestTaskBillingReconciliationLogFailureDoesNotReplayFunds(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	now := time.Now().Unix()
	seedUser(t, 309, 900)
	seedToken(t, 309, 309, "reconcile-log-failure", 900)
	seedChannel(t, 309)
	target := 25
	task := makeTask(309, 309, 100, 309, BillingSourceWallet, 0)
	job := seedReconciliationJob(t, task, model.TaskBillingOperationSettle, &target, now)

	callbackName := "test:fail_task_billing_event"
	require.NoError(t, model.LOG_DB.Callback().Create().Before("gorm:create").Register(callbackName, func(db *gorm.DB) {
		if db.Statement != nil && db.Statement.Schema != nil && db.Statement.Schema.Name == "Log" {
			db.AddError(errors.New("injected billing log failure"))
		}
	}))
	t.Cleanup(func() {
		model.LOG_DB.Callback().Create().Remove(callbackName)
	})

	first, err := runTaskBillingReconciliationOnceAt(context.Background(), "log-worker", now)
	require.NoError(t, err)
	assert.Equal(t, 1, first.Succeeded)
	assert.Equal(t, model.TaskBillingJobStatusSucceeded, loadReconciliationJob(t, job.ID).Status)
	assert.Equal(t, 975, getUserQuota(t, 309))

	second, err := runTaskBillingReconciliationOnceAt(context.Background(), "log-worker", now+1)
	require.NoError(t, err)
	assert.Zero(t, second.Claimed)
	assert.Equal(t, 975, getUserQuota(t, 309))
}

func TestApplyTaskBillingJobDoesNotReapplySucceededJob(t *testing.T) {
	setupTaskBillingReconciliationTest(t)
	now := time.Now().Unix()
	seedUser(t, 310, 900)
	seedChannel(t, 310)
	target := 20
	task := makeTask(310, 310, 100, 0, BillingSourceWallet, 0)
	job := seedReconciliationJob(t, task, model.TaskBillingOperationSettle, &target, now)
	claimed, err := model.ClaimTaskBillingJobs("apply-worker", now, now+60, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, ApplyTaskBillingJob(context.Background(), claimed[0]))
	assert.Equal(t, 980, getUserQuota(t, 310))

	require.NoError(t, ApplyTaskBillingJob(context.Background(), claimed[0]))
	assert.Equal(t, 980, getUserQuota(t, 310))
	assert.Equal(t, model.TaskBillingJobStatusSucceeded, loadReconciliationJob(t, job.ID).Status)
}
