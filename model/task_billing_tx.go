package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	taskBillingSourceWallet       = "wallet"
	taskBillingSourceSubscription = "subscription"
)

var errTaskBillingAlreadyAppliedWithDifferentTarget = errors.New("task billing already applied with a different target")

// ApplyTaskBillingDelta applies an async task billing job and synchronizes the
// affected caches only after the main-database transaction has committed.
func ApplyTaskBillingDelta(task *Task, targetQuota int, expectedLockedBy string, expectedAttempt int) error {
	if task == nil {
		return errors.New("task is required")
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		return ApplyTaskBillingDeltaTx(tx, task, targetQuota, expectedLockedBy, expectedAttempt)
	})
	if err != nil {
		return err
	}
	SyncTaskBillingCachesAfterCommit(task)
	return nil
}

// ApplyTaskBillingDeltaTx atomically adjusts the funding source, token quota,
// task quota, final usage statistics, and billing job state. The caller owns
// the transaction and must call SyncTaskBillingCachesAfterCommit only after a
// successful commit.
func ApplyTaskBillingDeltaTx(tx *gorm.DB, task *Task, targetQuota int, expectedLockedBy string, expectedAttempt int) error {
	if tx == nil {
		return errors.New("billing transaction is required")
	}
	if task == nil || task.ID <= 0 {
		return errors.New("persisted task is required")
	}
	if targetQuota < 0 || targetQuota > common.MaxQuota {
		return fmt.Errorf("invalid target quota: %d", targetQuota)
	}
	expectedLockedBy = strings.TrimSpace(expectedLockedBy)
	if expectedLockedBy == "" || expectedAttempt <= 0 {
		return errTaskBillingJobLeaseLost
	}

	var job TaskBillingJob
	if err := lockForUpdate(tx).Where("task_id = ?", task.ID).First(&job).Error; err != nil {
		return err
	}
	if job.Status == TaskBillingJobStatusSucceeded {
		if job.TargetQuota == nil || *job.TargetQuota != targetQuota {
			return errTaskBillingAlreadyAppliedWithDifferentTarget
		}
		var storedTask Task
		if err := lockForUpdate(tx).Where("id = ?", task.ID).First(&storedTask).Error; err != nil {
			return err
		}
		if storedTask.Quota != targetQuota {
			return fmt.Errorf("succeeded task billing quota mismatch: task=%d job=%d", storedTask.Quota, targetQuota)
		}
		*task = storedTask
		return nil
	}
	now := common.GetTimestamp()
	if job.Status != TaskBillingJobStatusProcessing || job.LockedBy != expectedLockedBy ||
		job.Attempts != expectedAttempt || job.LockedUntil < now {
		return errTaskBillingJobLeaseLost
	}
	if job.FromQuota < 0 || job.FromQuota > common.MaxQuota {
		return fmt.Errorf("invalid billing job from quota: %d", job.FromQuota)
	}
	if job.Operation == TaskBillingOperationRefund && targetQuota != 0 {
		return errors.New("refund billing job target quota must be zero")
	}
	if job.Operation != TaskBillingOperationRefund && job.Operation != TaskBillingOperationSettle {
		return fmt.Errorf("invalid billing job operation: %s", job.Operation)
	}

	var storedTask Task
	if err := lockForUpdate(tx).Where("id = ?", task.ID).First(&storedTask).Error; err != nil {
		return err
	}
	if storedTask.Quota != job.FromQuota {
		return fmt.Errorf("task billing from quota mismatch: task=%d job=%d", storedTask.Quota, job.FromQuota)
	}

	delta := int64(targetQuota) - int64(job.FromQuota)
	billingSource := strings.TrimSpace(storedTask.PrivateData.BillingSource)
	walletDelta := -delta
	if billingSource == taskBillingSourceSubscription {
		if err := applyTaskBillingSubscriptionDeltaTx(
			tx, storedTask.PrivateData.SubscriptionId, storedTask.UserId, delta,
		); err != nil {
			return err
		}
		walletDelta = 0
	} else if billingSource != taskBillingSourceWallet && billingSource != "" {
		return fmt.Errorf("invalid task billing source: %s", billingSource)
	}

	statisticsQuota := int64(0)
	requestCountDelta := int64(0)
	if targetQuota > 0 {
		statisticsQuota = int64(targetQuota)
	}
	if job.Operation == TaskBillingOperationSettle {
		requestCountDelta = 1
	}
	if _, err := applyTaskBillingUserDeltaTx(
		tx, storedTask.UserId, walletDelta, statisticsQuota, requestCountDelta,
	); err != nil {
		return err
	}
	if storedTask.PrivateData.TokenId > 0 {
		if _, err := applyTaskBillingTokenDeltaTx(
			tx, storedTask.PrivateData.TokenId, storedTask.UserId, delta,
		); err != nil {
			return err
		}
	}
	if err := applyTaskBillingChannelStatisticsTx(tx, storedTask.ChannelId, statisticsQuota); err != nil {
		return err
	}

	if err := tx.Model(&Task{}).Where("id = ? AND quota = ?", storedTask.ID, job.FromQuota).
		Update("quota", targetQuota).Error; err != nil {
		return err
	}
	target := targetQuota
	result := tx.Model(&TaskBillingJob{}).
		Where("id = ? AND status = ? AND locked_by = ? AND attempts = ? AND locked_until >= ?",
			job.ID, TaskBillingJobStatusProcessing, expectedLockedBy, expectedAttempt, common.GetTimestamp()).
		Updates(map[string]any{
			"target_quota": &target,
			"status":       TaskBillingJobStatusSucceeded,
			"locked_by":    "",
			"locked_until": 0,
			"last_error":   "",
			"updated_at":   common.GetTimestamp(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errTaskBillingJobLeaseLost
	}

	storedTask.Quota = targetQuota
	*task = storedTask
	return nil
}

func applyTaskBillingChannelStatisticsTx(tx *gorm.DB, channelID int, quota int64) error {
	if channelID <= 0 || quota <= 0 {
		return nil
	}
	var channel Channel
	if err := lockForUpdate(tx).Where("id = ?", channelID).First(&channel).Error; err != nil {
		return err
	}
	usedQuota := saturatingTaskBillingInt64(channel.UsedQuota, quota)
	return tx.Model(&Channel{}).Where("id = ?", channelID).Update("used_quota", usedQuota).Error
}

func saturatingTaskBillingInt64(current int64, delta int64) int64 {
	if current < 0 {
		current = 0
	}
	if delta <= 0 {
		return current
	}
	if current > math.MaxInt64-delta {
		return math.MaxInt64
	}
	return current + delta
}

// SyncTaskBillingCachesAfterCommit refreshes cache state from committed rows.
// It is deliberately best-effort and never calls a database mutation helper.
func SyncTaskBillingCachesAfterCommit(task *Task) {
	if task == nil || !common.RedisEnabled {
		return
	}
	if err := syncTaskBillingUserCacheAfterCommit(task.UserId); err != nil {
		common.SysLog(fmt.Sprintf("failed to synchronize task billing user cache for user %d: %v", task.UserId, err))
	}
	if task.PrivateData.TokenId > 0 {
		if err := syncTaskBillingTokenCacheAfterCommit(task.PrivateData.TokenId); err != nil {
			common.SysLog(fmt.Sprintf("failed to synchronize task billing token cache for token %d: %v", task.PrivateData.TokenId, err))
		}
	}
}
