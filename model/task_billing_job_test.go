package model

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFinalizeTaskAndEnqueueBillingWithContextCancellationRollsBack(t *testing.T) {
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "context-cancel.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Task{}, &TaskBillingJob{}))
	DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		DB = previousDB
		require.NoError(t, sqlDB.Close())
	})
	task := createInProgressBillingTask(t, "task_billing_context_cancel", 125)
	task.Status = TaskStatusFailure
	task.Progress = "100%"

	type contextKey struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "billing-context"))
	callbackName := "test:cancel_billing_job_create"
	sawContext := false
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Name != "TaskBillingJob" {
			return
		}
		sawContext = tx.Statement.Context.Value(contextKey{}) == "billing-context"
		cancel()
		tx.AddError(context.Canceled)
	}))
	t.Cleanup(func() { DB.Callback().Create().Remove(callbackName) })

	won, err := FinalizeTaskAndEnqueueBillingWithContext(ctx, task, TaskStatusInProgress, newRefundBillingJob(task))
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, won)
	assert.True(t, sawContext)
	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusInProgress, reloaded.Status)
	var jobs int64
	require.NoError(t, DB.Model(&TaskBillingJob{}).Where("task_id = ?", task.ID).Count(&jobs).Error)
	assert.Zero(t, jobs)
}

func prepareTaskBillingJobTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&TaskBillingJob{}))
	require.NoError(t, DB.Exec("DELETE FROM task_billing_jobs").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM task_billing_jobs")
	})
}

func createInProgressBillingTask(t *testing.T, taskID string, quota int) *Task {
	t.Helper()
	task := &Task{
		TaskID:   taskID,
		Status:   TaskStatusInProgress,
		Progress: "50%",
		Quota:    quota,
	}
	insertTask(t, task)
	return task
}

func reloadBillingJob(t *testing.T, taskID int64) *TaskBillingJob {
	t.Helper()
	var job TaskBillingJob
	require.NoError(t, DB.Where("task_id = ?", taskID).First(&job).Error)
	return &job
}

func newRefundBillingJob(task *Task) *TaskBillingJob {
	target := 0
	return &TaskBillingJob{
		TaskID:         task.ID,
		IdempotencyKey: fmt.Sprintf("async-task:%d:terminal-v1", task.ID),
		Operation:      TaskBillingOperationRefund,
		FromQuota:      task.Quota,
		TargetQuota:    &target,
	}
}

func TestFinalizeTaskAndEnqueueBillingIsAtomic(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	task := createInProgressBillingTask(t, "task_billing_atomic", 120)
	task.Status = TaskStatusFailure
	task.Progress = "100%"

	won, err := FinalizeTaskAndEnqueueBilling(task, TaskStatusInProgress, newRefundBillingJob(task))
	require.NoError(t, err)
	require.True(t, won)

	job := reloadBillingJob(t, task.ID)
	assert.Equal(t, TaskBillingJobStatusPending, job.Status)
	assert.Equal(t, 120, job.FromQuota)
	require.NotNil(t, job.TargetQuota)
	assert.Zero(t, *job.TargetQuota)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusFailure, reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
}

func TestFinalizeTaskAndEnqueueBillingCASLoserCreatesNoJob(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	task := createInProgressBillingTask(t, "task_billing_loser", 90)
	require.NoError(t, DB.Model(&Task{}).Where("id = ?", task.ID).Update("status", TaskStatusFailure).Error)
	task.Status = TaskStatusSuccess

	won, err := FinalizeTaskAndEnqueueBilling(task, TaskStatusInProgress, newRefundBillingJob(task))
	require.NoError(t, err)
	assert.False(t, won)

	var count int64
	require.NoError(t, DB.Model(&TaskBillingJob{}).Where("task_id = ?", task.ID).Count(&count).Error)
	assert.Zero(t, count)
}

func TestFinalizeTaskAndEnqueueBillingRollsBackTaskWhenJobInsertFails(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	task := createInProgressBillingTask(t, "task_billing_rollback", 75)
	require.NoError(t, DB.Create(newRefundBillingJob(task)).Error)
	job := newRefundBillingJob(task)
	task.Status = TaskStatusFailure

	won, err := FinalizeTaskAndEnqueueBilling(task, TaskStatusInProgress, job)
	require.Error(t, err)
	assert.False(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusInProgress, reloaded.Status)
}

func TestTaskBillingJobEnforcesUniqueTaskAndIdempotencyKeys(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	firstTask := createInProgressBillingTask(t, "task_billing_unique_a", 30)
	secondTask := createInProgressBillingTask(t, "task_billing_unique_b", 40)

	first := newRefundBillingJob(firstTask)
	require.NoError(t, DB.Create(first).Error)

	duplicateTask := newRefundBillingJob(firstTask)
	duplicateTask.IdempotencyKey = "async-task:other:terminal-v1"
	require.Error(t, DB.Create(duplicateTask).Error)

	duplicateKey := newRefundBillingJob(secondTask)
	duplicateKey.IdempotencyKey = first.IdempotencyKey
	require.Error(t, DB.Create(duplicateKey).Error)
}

func TestClaimTaskBillingJobsUsesPendingOrderAndRecoversExpiredLeases(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	now := time.Now().Unix()

	newJob := func(taskID string, nextAttemptAt int64) *TaskBillingJob {
		task := createInProgressBillingTask(t, taskID, 50)
		job := newRefundBillingJob(task)
		job.NextAttemptAt = nextAttemptAt
		require.NoError(t, DB.Create(job).Error)
		return job
	}

	first := newJob("task_billing_claim_first", now-20)
	second := newJob("task_billing_claim_second", now-10)
	_ = newJob("task_billing_claim_future", now+60)
	expired := newJob("task_billing_claim_expired", now-30)
	require.NoError(t, DB.Model(&TaskBillingJob{}).Where("id = ?", expired.ID).Updates(map[string]any{
		"status":       TaskBillingJobStatusProcessing,
		"locked_by":    "dead-worker",
		"locked_until": now - 1,
	}).Error)
	active := newJob("task_billing_claim_active", now-40)
	require.NoError(t, DB.Model(&TaskBillingJob{}).Where("id = ?", active.ID).Updates(map[string]any{
		"status":       TaskBillingJobStatusProcessing,
		"locked_by":    "active-worker",
		"locked_until": now + 60,
	}).Error)

	claimed, err := ClaimTaskBillingJobs("worker-a", now, now+30, 3)
	require.NoError(t, err)
	require.Len(t, claimed, 3)
	assert.Equal(t, []int64{expired.ID, first.ID, second.ID}, []int64{claimed[0].ID, claimed[1].ID, claimed[2].ID})
	for _, job := range claimed {
		assert.Equal(t, TaskBillingJobStatusProcessing, job.Status)
		assert.Equal(t, "worker-a", job.LockedBy)
		assert.Equal(t, now+30, job.LockedUntil)
		assert.Equal(t, 1, job.Attempts)
	}

	unclaimed := reloadBillingJob(t, active.TaskID)
	assert.Equal(t, "active-worker", unclaimed.LockedBy)
}

func TestTaskBillingJobRetryAndReviewSanitizeErrors(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	now := time.Now().Unix()
	task := createInProgressBillingTask(t, "task_billing_retry", 25)
	require.NoError(t, DB.Create(newRefundBillingJob(task)).Error)
	claimed, err := ClaimTaskBillingJobs("worker-a", now, now+30, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	longError := "  upstream\n\tfailed: " + strings.Repeat("x", 2000)
	require.NoError(t, RescheduleTaskBillingJob(claimed[0].ID, "worker-a", claimed[0].Attempts, now+60, longError))
	retried := reloadBillingJob(t, task.ID)
	assert.Equal(t, TaskBillingJobStatusPending, retried.Status)
	assert.Empty(t, retried.LockedBy)
	assert.Zero(t, retried.LockedUntil)
	assert.Equal(t, now+60, retried.NextAttemptAt)
	assert.NotContains(t, retried.LastError, "\n")
	assert.NotContains(t, retried.LastError, "\t")
	assert.LessOrEqual(t, len(retried.LastError), taskBillingJobLastErrorMaxBytes)

	claimed, err = ClaimTaskBillingJobs("worker-b", now+60, now+90, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.NoError(t, RequireTaskBillingReview(claimed[0].ID, "worker-b", claimed[0].Attempts, "manual\nreview"))
	review := reloadBillingJob(t, task.ID)
	assert.Equal(t, TaskBillingJobStatusReviewRequired, review.Status)
	assert.Empty(t, review.LockedBy)
	assert.Equal(t, "manual review", review.LastError)
}

func TestTaskBillingJobClaimAttemptFencesSameWorkerReclaim(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	now := time.Now().Unix()
	task := createInProgressBillingTask(t, "task_billing_fence", 25)
	require.NoError(t, DB.Create(newRefundBillingJob(task)).Error)

	firstClaim, err := ClaimTaskBillingJobs("worker-a", now, now+10, 1)
	require.NoError(t, err)
	require.Len(t, firstClaim, 1)
	assert.Equal(t, 1, firstClaim[0].Attempts)

	secondClaim, err := ClaimTaskBillingJobs("worker-a", now+11, now+60, 1)
	require.NoError(t, err)
	require.Len(t, secondClaim, 1)
	assert.Equal(t, 2, secondClaim[0].Attempts)

	assert.ErrorIs(t,
		RescheduleTaskBillingJob(firstClaim[0].ID, "worker-a", firstClaim[0].Attempts, now+90, "stale retry"),
		errTaskBillingJobLeaseLost,
	)
	assert.ErrorIs(t,
		RequireTaskBillingReview(firstClaim[0].ID, "worker-a", firstClaim[0].Attempts, "stale review"),
		errTaskBillingJobLeaseLost,
	)
	assert.ErrorIs(t,
		CompleteTaskBillingJob(firstClaim[0].ID, "worker-a", firstClaim[0].Attempts),
		errTaskBillingJobLeaseLost,
	)
	require.NoError(t, CompleteTaskBillingJob(secondClaim[0].ID, "worker-a", secondClaim[0].Attempts))

	completed := reloadBillingJob(t, task.ID)
	assert.Equal(t, TaskBillingJobStatusSucceeded, completed.Status)
	assert.Empty(t, completed.LockedBy)
}

func TestTaskBillingJobLastErrorRedactsSecretsBeforePersistence(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	now := time.Now().Unix()
	task := createInProgressBillingTask(t, "task_billing_redaction", 25)
	require.NoError(t, DB.Create(newRefundBillingJob(task)).Error)
	claimed, err := ClaimTaskBillingJobs("worker-a", now, now+30, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	secretError := "request failed Authorization: Bearer bearer-secret api_key=api-secret " +
		"url=https://cdn.example/result?X-Amz-Signature=signed-secret&X-Amz-Credential=credential-secret"
	require.NoError(t, RescheduleTaskBillingJob(
		claimed[0].ID, "worker-a", claimed[0].Attempts, now+60, secretError,
	))

	persisted := reloadBillingJob(t, task.ID).LastError
	assert.NotContains(t, persisted, "bearer-secret")
	assert.NotContains(t, persisted, "api-secret")
	assert.NotContains(t, persisted, "signed-secret")
	assert.NotContains(t, persisted, "credential-secret")
	assert.Contains(t, persisted, "[REDACTED]")
}

func TestTaskBillingJobLastErrorNormalizesInvalidDatabaseTextBeforePersistence(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	now := time.Now().Unix()
	task := createInProgressBillingTask(t, "task_billing_invalid_text", 25)
	require.NoError(t, DB.Create(newRefundBillingJob(task)).Error)
	claimed, err := ClaimTaskBillingJobs("worker-a", now, now+30, 1)
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	invalidError := string([]byte{'b', 'a', 'd', 0, 1, 0xff, ' ', 'e', 'r', 'r', 'o', 'r'})
	require.NoError(t, RescheduleTaskBillingJob(
		claimed[0].ID, "worker-a", claimed[0].Attempts, now+60, invalidError,
	))

	persisted := reloadBillingJob(t, task.ID).LastError
	assert.True(t, utf8.ValidString(persisted))
	assert.NotContains(t, persisted, "\x00")
	assert.NotContains(t, persisted, "\x01")
	assert.Contains(t, persisted, "\uFFFD")
	for _, r := range persisted {
		assert.False(t, unicode.IsControl(r), "persisted control rune %U", r)
	}
}

func TestFinalizeTaskAndEnqueueBillingUsesInternalIDForIdempotency(t *testing.T) {
	for _, publicTaskID := range []string{"", "duplicate-public-task-id"} {
		publicTaskID := publicTaskID
		t.Run(fmt.Sprintf("public-id-%q", publicTaskID), func(t *testing.T) {
			truncateTables(t)
			prepareTaskBillingJobTest(t)

			first := createInProgressBillingTask(t, publicTaskID, 10)
			second := createInProgressBillingTask(t, publicTaskID, 20)
			for _, task := range []*Task{first, second} {
				task.Status = TaskStatusFailure
				job := newRefundBillingJob(task)
				job.IdempotencyKey = "caller-supplied-duplicate-key"
				won, err := FinalizeTaskAndEnqueueBilling(task, TaskStatusInProgress, job)
				require.NoError(t, err)
				require.True(t, won)
			}

			firstJob := reloadBillingJob(t, first.ID)
			secondJob := reloadBillingJob(t, second.ID)
			assert.Equal(t, fmt.Sprintf("async-task:%d:terminal-v1", first.ID), firstJob.IdempotencyKey)
			assert.Equal(t, fmt.Sprintf("async-task:%d:terminal-v1", second.ID), secondJob.IdempotencyKey)
			assert.NotEqual(t, firstJob.IdempotencyKey, secondJob.IdempotencyKey)
		})
	}
}

func TestTaskBillingJobSchemaUsesGORMV2PrimaryKey(t *testing.T) {
	statement := &gorm.Statement{DB: DB}
	require.NoError(t, statement.Parse(&TaskBillingJob{}))
	idField := statement.Schema.LookUpField("ID")
	require.NotNil(t, idField)
	assert.True(t, idField.PrimaryKey)
	assert.NotContains(t, idField.TagSettings, "AUTO_INCREMENT")
}

func TestTaskBillingJobMigrationCreatesExpiredLeaseCompositeIndex(t *testing.T) {
	prepareTaskBillingJobTest(t)
	type indexColumn struct {
		Seqno int
		Name  string
	}
	var columns []indexColumn
	require.NoError(t, DB.Raw("PRAGMA index_info('idx_task_billing_jobs_expired')").Scan(&columns).Error)
	require.Len(t, columns, 2)
	assert.Equal(t, "status", columns[0].Name)
	assert.Equal(t, "locked_until", columns[1].Name)
}

func TestTaskBillingJobTargetQuotaStaysNullableAndTaskRowsStayUnchanged(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	task := createInProgressBillingTask(t, "task_billing_nullable", 65)
	job := &TaskBillingJob{
		TaskID:         task.ID,
		IdempotencyKey: fmt.Sprintf("async-task:%d:terminal-v1", task.ID),
		Operation:      TaskBillingOperationSettle,
		FromQuota:      task.Quota,
		TargetQuota:    nil,
	}
	require.NoError(t, DB.Create(job).Error)

	reloadedJob := reloadBillingJob(t, task.ID)
	assert.Nil(t, reloadedJob.TargetQuota)

	var reloadedTask Task
	require.NoError(t, DB.First(&reloadedTask, task.ID).Error)
	assert.EqualValues(t, TaskStatusInProgress, reloadedTask.Status)
	assert.Equal(t, 65, reloadedTask.Quota)
	assert.Equal(t, "50%", reloadedTask.Progress)
}
