package model

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteOldTerminalTaskBatchPreservesActiveAndRecentTasks(t *testing.T) {
	truncateTables(t)
	cutoff := time.Now().Unix() - 100
	rows := []*Task{
		{TaskID: "old-success", CreatedAt: cutoff - 20, UpdatedAt: cutoff - 20, Status: TaskStatusSuccess},
		{TaskID: "old-failure", CreatedAt: cutoff - 10, UpdatedAt: cutoff - 10, Status: TaskStatusFailure},
		{TaskID: "old-running", CreatedAt: cutoff - 30, UpdatedAt: cutoff - 30, Status: TaskStatusInProgress},
		{TaskID: "old-queued", CreatedAt: cutoff - 40, UpdatedAt: cutoff - 40, Status: TaskStatusQueued},
		{TaskID: "recent-success", CreatedAt: cutoff + 10, UpdatedAt: cutoff + 10, Status: TaskStatusSuccess},
	}
	for _, row := range rows {
		require.NoError(t, DB.Create(row).Error)
	}

	count, err := CountOldTerminalTasks(context.Background(), cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	deleted, err := DeleteOldTerminalTaskBatch(context.Background(), cutoff, 100)
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	var remaining []Task
	require.NoError(t, DB.Order("task_id").Find(&remaining).Error)
	remainingIDs := make([]string, 0, len(remaining))
	for _, task := range remaining {
		remainingIDs = append(remainingIDs, task.TaskID)
	}
	assert.Equal(t, []string{"old-queued", "old-running", "recent-success"}, remainingIDs)
}
