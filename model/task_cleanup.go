package model

import (
	"context"

	"gorm.io/gorm"
)

var terminalTaskStatuses = []TaskStatus{
	TaskStatusSuccess,
	TaskStatusFailure,
}

func oldTerminalTaskQuery(ctx context.Context, targetTimestamp int64) *gorm.DB {
	return DB.WithContext(ctx).
		Model(&Task{}).
		Where("created_at < ?", targetTimestamp).
		Where("status IN ?", terminalTaskStatuses)
}

func CountOldTerminalTasks(ctx context.Context, targetTimestamp int64) (int64, error) {
	var total int64
	err := oldTerminalTaskQuery(ctx, targetTimestamp).Count(&total).Error
	return total, err
}

func DeleteOldTerminalTaskBatch(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	result := oldTerminalTaskQuery(ctx, targetTimestamp).
		Limit(limit).
		Delete(&Task{})
	return result.RowsAffected, result.Error
}
