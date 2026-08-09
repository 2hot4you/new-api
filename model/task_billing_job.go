package model

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type TaskBillingJobStatus string
type TaskBillingOperation string

const (
	TaskBillingJobStatusPending        TaskBillingJobStatus = "pending"
	TaskBillingJobStatusProcessing     TaskBillingJobStatus = "processing"
	TaskBillingJobStatusSucceeded      TaskBillingJobStatus = "succeeded"
	TaskBillingJobStatusReviewRequired TaskBillingJobStatus = "review_required"

	TaskBillingOperationRefund TaskBillingOperation = "refund"
	TaskBillingOperationSettle TaskBillingOperation = "settle"

	taskBillingJobLastErrorMaxBytes = 1024
)

var errTaskBillingJobLeaseLost = errors.New("task billing job lease lost")

var taskBillingJobSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer[[:space:]]+)[^[:space:],;]+`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|x-api-key|access[_-]?token|x-amz-signature|x-amz-credential|x-amz-security-token|signature|secret|token)[[:space:]]*(?:=|:)[[:space:]]*)[^&[:space:],;]+`),
}

type TaskBillingJob struct {
	ID             int64                `json:"id" gorm:"primaryKey"`
	TaskID         int64                `json:"task_id" gorm:"uniqueIndex"`
	IdempotencyKey string               `json:"idempotency_key" gorm:"type:varchar(191);uniqueIndex"`
	Operation      TaskBillingOperation `json:"operation" gorm:"type:varchar(32)"`
	FromQuota      int                  `json:"from_quota"`
	TargetQuota    *int                 `json:"target_quota"`
	Status         TaskBillingJobStatus `json:"status" gorm:"type:varchar(32);index:idx_task_billing_jobs_ready,priority:1;index:idx_task_billing_jobs_expired,priority:1"`
	Attempts       int                  `json:"attempts"`
	NextAttemptAt  int64                `json:"next_attempt_at" gorm:"bigint;index:idx_task_billing_jobs_ready,priority:2"`
	LockedBy       string               `json:"locked_by" gorm:"type:varchar(128)"`
	LockedUntil    int64                `json:"locked_until" gorm:"bigint;index:idx_task_billing_jobs_expired,priority:2"`
	LastError      string               `json:"last_error" gorm:"type:varchar(1024)"`
	CreatedAt      int64                `json:"created_at" gorm:"bigint"`
	UpdatedAt      int64                `json:"updated_at" gorm:"bigint"`
}

func (job *TaskBillingJob) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	if job.Status == "" {
		job.Status = TaskBillingJobStatusPending
	}
	if job.CreatedAt == 0 {
		job.CreatedAt = now
	}
	if job.UpdatedAt == 0 {
		job.UpdatedAt = now
	}
	job.LastError = sanitizeTaskBillingJobError(job.LastError)
	return nil
}

func FinalizeTaskAndEnqueueBilling(task *Task, fromStatus TaskStatus, job *TaskBillingJob) (bool, error) {
	if task == nil || job == nil {
		return false, errors.New("task and billing job are required")
	}
	if task.ID == 0 {
		return false, errors.New("persisted task is required")
	}
	if job.TaskID != 0 && job.TaskID != task.ID {
		return false, errors.New("billing job task id does not match task")
	}

	won := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		won, err = task.updateWithStatus(tx, fromStatus)
		if err != nil || !won {
			return err
		}

		job.TaskID = task.ID
		job.IdempotencyKey = taskBillingJobIdempotencyKey(task.ID)
		job.Status = TaskBillingJobStatusPending
		job.LockedBy = ""
		job.LockedUntil = 0
		return tx.Create(job).Error
	})
	if err != nil {
		return false, err
	}
	return won, nil
}

func taskBillingJobIdempotencyKey(taskID int64) string {
	return "async-task:" + strconv.FormatInt(taskID, 10) + ":terminal-v1"
}

func ClaimTaskBillingJobs(lockedBy string, now int64, lockedUntil int64, limit int) ([]*TaskBillingJob, error) {
	if strings.TrimSpace(lockedBy) == "" {
		return nil, errors.New("billing job worker id is required")
	}
	if limit <= 0 {
		return []*TaskBillingJob{}, nil
	}
	if lockedUntil <= now {
		return nil, errors.New("billing job lease must expire after claim time")
	}

	claimed := make([]*TaskBillingJob, 0, limit)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var candidates []*TaskBillingJob
		if err := tx.Where(
			"(status = ? AND next_attempt_at <= ?) OR (status = ? AND locked_until < ?)",
			TaskBillingJobStatusPending, now, TaskBillingJobStatusProcessing, now,
		).Order("next_attempt_at ASC").Order("id ASC").Limit(limit).Find(&candidates).Error; err != nil {
			return err
		}

		for _, candidate := range candidates {
			query := tx.Model(&TaskBillingJob{}).Where("id = ? AND status = ?", candidate.ID, candidate.Status)
			if candidate.Status == TaskBillingJobStatusPending {
				query = query.Where("next_attempt_at <= ?", now)
			} else {
				query = query.Where("locked_until < ?", now)
			}
			result := query.Updates(map[string]any{
				"status":       TaskBillingJobStatusProcessing,
				"attempts":     gorm.Expr("attempts + 1"),
				"locked_by":    lockedBy,
				"locked_until": lockedUntil,
				"updated_at":   now,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			candidate.Status = TaskBillingJobStatusProcessing
			candidate.Attempts++
			candidate.LockedBy = lockedBy
			candidate.LockedUntil = lockedUntil
			candidate.UpdatedAt = now
			claimed = append(claimed, candidate)
		}
		return nil
	})
	return claimed, err
}

func RescheduleTaskBillingJob(id int64, lockedBy string, claimAttempt int, nextAttemptAt int64, lastError string) error {
	return transitionClaimedTaskBillingJob(id, lockedBy, claimAttempt, map[string]any{
		"status":          TaskBillingJobStatusPending,
		"next_attempt_at": nextAttemptAt,
		"locked_by":       "",
		"locked_until":    0,
		"last_error":      sanitizeTaskBillingJobError(lastError),
		"updated_at":      common.GetTimestamp(),
	})
}

func RequireTaskBillingReview(id int64, lockedBy string, claimAttempt int, lastError string) error {
	return transitionClaimedTaskBillingJob(id, lockedBy, claimAttempt, map[string]any{
		"status":       TaskBillingJobStatusReviewRequired,
		"locked_by":    "",
		"locked_until": 0,
		"last_error":   sanitizeTaskBillingJobError(lastError),
		"updated_at":   common.GetTimestamp(),
	})
}

func CompleteTaskBillingJob(id int64, lockedBy string, claimAttempt int) error {
	return transitionClaimedTaskBillingJob(id, lockedBy, claimAttempt, map[string]any{
		"status":       TaskBillingJobStatusSucceeded,
		"locked_by":    "",
		"locked_until": 0,
		"last_error":   "",
		"updated_at":   common.GetTimestamp(),
	})
}

func transitionClaimedTaskBillingJob(id int64, lockedBy string, claimAttempt int, updates map[string]any) error {
	now := common.GetTimestamp()
	result := DB.Model(&TaskBillingJob{}).
		Where("id = ? AND status = ? AND locked_by = ? AND attempts = ? AND locked_until >= ?", id, TaskBillingJobStatusProcessing, lockedBy, claimAttempt, now).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errTaskBillingJobLeaseLost
	}
	return nil
}

func sanitizeTaskBillingJobError(message string) string {
	message = strings.ToValidUTF8(message, "\uFFFD")
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, message)
	for _, pattern := range taskBillingJobSecretPatterns {
		message = pattern.ReplaceAllString(message, "${1}[REDACTED]")
	}
	message = strings.Join(strings.Fields(message), " ")
	if len(message) <= taskBillingJobLastErrorMaxBytes {
		return message
	}
	message = message[:taskBillingJobLastErrorMaxBytes]
	for !utf8.ValidString(message) {
		message = message[:len(message)-1]
	}
	return message
}
