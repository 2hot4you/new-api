package model

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		IdempotencyKey: "async-task:" + task.TaskID + ":terminal-v1",
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
	existingTask := createInProgressBillingTask(t, "task_billing_existing", 10)
	require.NoError(t, DB.Create(newRefundBillingJob(existingTask)).Error)

	task := createInProgressBillingTask(t, "task_billing_rollback", 75)
	job := newRefundBillingJob(task)
	job.IdempotencyKey = "async-task:" + existingTask.TaskID + ":terminal-v1"
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
	require.NoError(t, RescheduleTaskBillingJob(claimed[0].ID, "worker-a", now+60, longError))
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
	require.NoError(t, RequireTaskBillingReview(claimed[0].ID, "worker-b", "manual\nreview"))
	review := reloadBillingJob(t, task.ID)
	assert.Equal(t, TaskBillingJobStatusReviewRequired, review.Status)
	assert.Empty(t, review.LockedBy)
	assert.Equal(t, "manual review", review.LastError)
}

func TestTaskBillingJobTargetQuotaStaysNullableAndTaskRowsStayUnchanged(t *testing.T) {
	truncateTables(t)
	prepareTaskBillingJobTest(t)
	task := createInProgressBillingTask(t, "task_billing_nullable", 65)
	job := &TaskBillingJob{
		TaskID:         task.ID,
		IdempotencyKey: "async-task:" + task.TaskID + ":terminal-v1",
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
