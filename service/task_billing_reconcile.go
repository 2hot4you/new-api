package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
	if err := model.DB.First(&authoritative, claimed.ID).Error; err != nil {
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
	if err := model.DB.First(&task, authoritative.TaskID).Error; err != nil {
		return err
	}
	if err := model.ApplyTaskBillingDelta(&task, targetQuota, authoritative.LockedBy, authoritative.Attempts); err != nil {
		return err
	}

	if err := recordTaskBillingReconciliationEvent(&authoritative, &task, targetQuota); err != nil {
		common.SysLog(fmt.Sprintf("failed to record task billing event taskbill_%d: %v", authoritative.ID, err))
	}
	return nil
}

func recordTaskBillingReconciliationEvent(job *model.TaskBillingJob, task *model.Task, targetQuota int) error {
	if job == nil || task == nil {
		return errors.New("billing event requires job and task")
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
	other["is_task"] = true
	other["task_id"] = task.TaskID
	other["billing_job_id"] = job.ID
	other["billing_event_key"] = fmt.Sprintf("taskbill_%d", job.ID)
	other["billing_operation"] = job.Operation
	other["pre_consumed_quota"] = job.FromQuota
	other["actual_quota"] = targetQuota

	username, _ := model.GetUsernameById(task.UserId, false)
	tokenName := ""
	if task.PrivateData.TokenId > 0 {
		if token, err := model.GetTokenById(task.PrivateData.TokenId); err == nil && token != nil {
			tokenName = token.Name
		}
	}
	return model.LOG_DB.Create(&model.Log{
		UserId:    task.UserId,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
		Username:  username,
		TokenName: tokenName,
		ModelName: taskModelName(task),
		Quota:     quota,
		UseTime:   taskElapsedSeconds(task),
		ChannelId: task.ChannelId,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		RequestId: fmt.Sprintf("taskbill_%d", job.ID),
		Other:     common.MapToJsonStr(other),
	}).Error
}

// RunTaskBillingReconciliationOnce claims and processes one bounded batch.
// Application errors are converted to fenced reschedule/review transitions;
// only claim/transition infrastructure failures are returned to the caller.
func RunTaskBillingReconciliationOnce(ctx context.Context, runnerID string) (TaskBillingReconciliationSummary, error) {
	return runTaskBillingReconciliationOnceAt(ctx, runnerID, common.GetTimestamp())
}

func runTaskBillingReconciliationOnceAt(ctx context.Context, runnerID string, now int64) (TaskBillingReconciliationSummary, error) {
	summary := TaskBillingReconciliationSummary{}
	if err := ctx.Err(); err != nil {
		return summary, err
	}
	runnerID = strings.TrimSpace(runnerID)
	claimed, err := model.ClaimTaskBillingJobs(
		runnerID,
		now,
		now+taskBillingReconciliationLeaseSeconds,
		taskBillingReconciliationBatchSize,
	)
	if err != nil {
		return summary, err
	}
	summary.Claimed = len(claimed)

	var transitionErrors []error
	for _, job := range claimed {
		if err := ctx.Err(); err != nil {
			if transitionErr := transitionFailedTaskBillingJob(job, now, err, &summary); transitionErr != nil {
				transitionErrors = append(transitionErrors, transitionErr)
			}
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
			if transitionErr := transitionFailedTaskBillingJob(job, now, err, &summary); transitionErr != nil {
				transitionErrors = append(transitionErrors, transitionErr)
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
