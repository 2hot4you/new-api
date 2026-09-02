package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"gorm.io/gorm"
)

const (
	taskBillingReconciliationBatchSize    = 50
	taskBillingReconciliationLeaseSeconds = int64(60)
	taskBillingReconciliationMaxAttempts  = 10
)

var ErrTaskBillingIntentIndeterminate = errors.New("task billing intent is indeterminate")

type TaskBillingReconciliationSummary struct {
	Claimed        int `json:"claimed"`
	Succeeded      int `json:"succeeded"`
	Rescheduled    int `json:"rescheduled"`
	ReviewRequired int `json:"review_required"`
}

// ResolveTerminalBillingIntent converts the persisted terminal intent to the
// authoritative target quota. Refunds always settle to zero; successful jobs
// must carry the terminal quota snapshot, including an explicit zero value.
func ResolveTerminalBillingIntent(job *model.TaskBillingJob) (int, error) {
	if job == nil {
		return 0, fmt.Errorf("%w: billing job is required", ErrTaskBillingIntentIndeterminate)
	}
	switch job.Operation {
	case model.TaskBillingOperationRefund:
		return 0, nil
	case model.TaskBillingOperationSettle:
		if job.TargetQuota == nil {
			return 0, fmt.Errorf("%w: settle target quota is missing", ErrTaskBillingIntentIndeterminate)
		}
		if *job.TargetQuota < 0 || *job.TargetQuota > common.MaxQuota {
			return 0, fmt.Errorf("%w: settle target quota is invalid", ErrTaskBillingIntentIndeterminate)
		}
		return *job.TargetQuota, nil
	default:
		return 0, fmt.Errorf("%w: unsupported operation %q", ErrTaskBillingIntentIndeterminate, job.Operation)
	}
}

func taskBillingRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 5 * time.Second
	case 2:
		return 30 * time.Second
	case 3:
		return 2 * time.Minute
	case 4:
		return 10 * time.Minute
	default:
		return time.Hour
	}
}

// ApplyTaskBillingJob applies one claimed job. The main-database transaction
// in model.ApplyTaskBillingDelta is the idempotency boundary. The event log is
// intentionally emitted only after that transaction commits, and a log-store
// failure is best-effort: it never turns committed money into retryable work.
func ApplyTaskBillingJob(ctx context.Context, claimed *model.TaskBillingJob) error {
	if claimed == nil || claimed.ID <= 0 {
		return errors.New("claimed billing job is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var authoritative model.TaskBillingJob
	if err := model.DB.WithContext(ctx).First(&authoritative, claimed.ID).Error; err != nil {
		return err
	}
	if authoritative.Status == model.TaskBillingJobStatusSucceeded {
		return nil
	}
	if authoritative.Status != model.TaskBillingJobStatusProcessing ||
		authoritative.LockedBy != claimed.LockedBy || authoritative.Attempts != claimed.Attempts {
		return errors.New("task billing claim is no longer owned by this worker")
	}

	targetQuota, err := ResolveTerminalBillingIntent(&authoritative)
	if err != nil {
		return err
	}
	var task model.Task
	if err := model.DB.WithContext(ctx).First(&task, authoritative.TaskID).Error; err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Bind cancellation to the caller-owned transaction while retaining Task 2's
	// exact transaction body and fencing. Once committed, cache/log work stays
	// best-effort and can never make the funding delta retryable.
	if err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return model.ApplyTaskBillingDeltaTx(tx, &task, targetQuota, authoritative.LockedBy, authoritative.Attempts)
	}); err != nil {
		return err
	}
	model.SyncTaskBillingCachesAfterCommit(&task)

	if err := recordTaskBillingReconciliationEvent(ctx, &authoritative, &task, targetQuota); err != nil {
		common.SysLog(fmt.Sprintf("failed to record task billing event taskbill_%d: %v", authoritative.ID, err))
	}
	return nil
}

func recordTaskBillingReconciliationEvent(ctx context.Context, job *model.TaskBillingJob, task *model.Task, targetQuota int) error {
	if job == nil || task == nil {
		return errors.New("billing event requires job and task")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	logType := model.LogTypeConsume
	content := "asynchronous task billing settled"
	quota := targetQuota
	if job.Operation == model.TaskBillingOperationRefund {
		logType = model.LogTypeRefund
		content = "asynchronous task billing refunded"
		quota = job.FromQuota
	}
	other := taskBillingOther(task)
	// Billing events are user-visible accounting records. The persisted task keeps
	// root diagnostics, but an upstream task identifier must never be copied into
	// the accounting log payload.
	delete(other, "root_info")
	other["is_task"] = true
	other["task_id"] = task.TaskID
	other["billing_job_id"] = job.ID
	other["billing_event_key"] = fmt.Sprintf("taskbill_%d", job.ID)
	other["billing_operation"] = job.Operation
	other["pre_consumed_quota"] = job.FromQuota
	other["actual_quota"] = targetQuota
	completionTokens := 0
	if billingContext := task.PrivateData.BillingContext; billingContext != nil {
		completionTokens = billingContext.ActualTokens
	}
	elapsedSeconds := taskElapsedSeconds(task)
	perfmetrics.Record(perfmetrics.Sample{
		Model:        taskModelName(task),
		Group:        task.Group,
		LatencyMs:    int64(elapsedSeconds) * 1000,
		Success:      job.Operation == model.TaskBillingOperationSettle,
		OutputTokens: int64(completionTokens),
		GenerationMs: int64(elapsedSeconds) * 1000,
	})
	return model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:           task.UserId,
		LogType:          logType,
		Content:          content,
		ChannelId:        task.ChannelId,
		ModelName:        taskModelName(task),
		Quota:            quota,
		CompletionTokens: completionTokens,
		TokenId:          task.PrivateData.TokenId,
		Group:            task.Group,
		Other:            other,
		UseTimeSeconds:   elapsedSeconds,
		NodeName:         task.PrivateData.NodeName,
		RequestId:        fmt.Sprintf("taskbill_%d", job.ID),
	})
}

// RunTaskBillingReconciliationOnce claims and processes one bounded batch.
// Application errors are converted to fenced reschedule/review transitions;
// only claim/transition infrastructure failures are returned to the caller.
func RunTaskBillingReconciliationOnce(ctx context.Context, runnerID string) (TaskBillingReconciliationSummary, error) {
	return runTaskBillingReconciliationOnceWithClock(ctx, runnerID, common.GetTimestamp)
}

func runTaskBillingReconciliationOnceAt(ctx context.Context, runnerID string, now int64) (TaskBillingReconciliationSummary, error) {
	return runTaskBillingReconciliationOnceWithClock(ctx, runnerID, func() int64 { return now })
}

func runTaskBillingReconciliationOnceWithClock(ctx context.Context, runnerID string, now func() int64) (TaskBillingReconciliationSummary, error) {
	summary := TaskBillingReconciliationSummary{}
	if err := ctx.Err(); err != nil {
		return summary, err
	}
	if now == nil {
		return summary, errors.New("task billing reconciliation clock is required")
	}
	claimNow := now()
	runnerID = strings.TrimSpace(runnerID)
	claimed, err := model.ClaimTaskBillingJobs(
		runnerID,
		claimNow,
		claimNow+taskBillingReconciliationLeaseSeconds,
		taskBillingReconciliationBatchSize,
	)
	if err != nil {
		return summary, err
	}
	summary.Claimed = len(claimed)

	var transitionErrors []error
	for _, job := range claimed {
		if err := ctx.Err(); err != nil {
			if transitionErr := transitionFailedTaskBillingJob(job, now(), err, &summary); transitionErr != nil {
				transitionErrors = append(transitionErrors, transitionErr)
			}
			transitionErrors = append(transitionErrors, err)
			continue
		}
		if _, err := ResolveTerminalBillingIntent(job); err != nil {
			if transitionErr := model.RequireTaskBillingReview(job.ID, job.LockedBy, job.Attempts, err.Error()); transitionErr != nil {
				transitionErrors = append(transitionErrors, transitionErr)
			} else {
				summary.ReviewRequired++
			}
			continue
		}
		if err := ApplyTaskBillingJob(ctx, job); err != nil {
			if transitionErr := transitionFailedTaskBillingJob(job, now(), err, &summary); transitionErr != nil {
				transitionErrors = append(transitionErrors, transitionErr)
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				transitionErrors = append(transitionErrors, err)
			}
			continue
		}
		summary.Succeeded++
	}
	return summary, errors.Join(transitionErrors...)
}

func transitionFailedTaskBillingJob(job *model.TaskBillingJob, now int64, applyErr error, summary *TaskBillingReconciliationSummary) error {
	if job.Attempts >= taskBillingReconciliationMaxAttempts {
		if err := model.RequireTaskBillingReview(job.ID, job.LockedBy, job.Attempts, applyErr.Error()); err != nil {
			return err
		}
		summary.ReviewRequired++
		return nil
	}
	nextAttemptAt := now + int64(taskBillingRetryDelay(job.Attempts)/time.Second)
	if err := model.RescheduleTaskBillingJob(job.ID, job.LockedBy, job.Attempts, nextAttemptAt, applyErr.Error()); err != nil {
		return err
	}
	summary.Rescheduled++
	return nil
}
