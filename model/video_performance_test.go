package model

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStarAIVideoPerformanceAggregatesTerminalTasksByModelAndGroup(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI))
	tasks := []*Task{
		{
			TaskID: "success-fast", Platform: platform, Group: "default",
			SubmitTime: now - 900, FinishTime: now - 600, Status: TaskStatusSuccess,
			Properties: Properties{OriginModelName: "doubao-seedance-2-0-260128"},
		},
		{
			TaskID: "success-slow", Platform: platform, Group: "vip",
			SubmitTime: now - 1800, FinishTime: now - 1100, Status: TaskStatusSuccess,
			PrivateData: TaskPrivateData{BillingContext: &TaskBillingContext{
				OriginModelName: "doubao-seedance-2-0-260128",
			}},
		},
		{
			TaskID: "failed", Platform: platform, Group: "default",
			SubmitTime: now - 1200, FinishTime: now - 1000, Status: TaskStatusFailure,
			Properties: Properties{OriginModelName: "doubao-seedance-2-0-260128"},
		},
		{
			TaskID: "pending", Platform: platform, Group: "default",
			SubmitTime: now - 300, Status: TaskStatusInProgress,
			Properties: Properties{OriginModelName: "doubao-seedance-2-0-260128"},
		},
		{
			TaskID: "other-model", Platform: platform, Group: "default",
			SubmitTime: now - 300, FinishTime: now - 100, Status: TaskStatusSuccess,
			Properties: Properties{OriginModelName: "doubao-seedance-2-0-fast-260128"},
		},
	}
	for _, task := range tasks {
		insertTask(t, task)
	}

	result, err := GetStarAIVideoPerformance("doubao-seedance-2-0-260128", 24)
	require.NoError(t, err)

	assert.Equal(t, int64(4), result.Summary.SubmittedCount)
	assert.Equal(t, int64(2), result.Summary.SuccessCount)
	assert.Equal(t, int64(1), result.Summary.FailureCount)
	assert.Equal(t, int64(1), result.Summary.PendingCount)
	assert.Equal(t, 66.67, result.Summary.SuccessRate)
	assert.Equal(t, int64(500), result.Summary.AverageDurationSeconds)
	assert.Equal(t, int64(300), result.Summary.P50DurationSeconds)
	assert.Equal(t, int64(700), result.Summary.P95DurationSeconds)
	assert.Equal(t, int64(1), result.Summary.SlowTaskCount)
	require.Len(t, result.Groups, 2)
	assert.Equal(t, "default", result.Groups[0].Group)
	assert.Equal(t, int64(3), result.Groups[0].SubmittedCount)
	assert.Equal(t, "vip", result.Groups[1].Group)
	assert.Equal(t, int64(1), result.Groups[1].SlowTaskCount)
	assert.NotEmpty(t, result.Series)
}

func TestGetStarAIVideoPerformanceExcludesTasksOutsideWindow(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	insertTask(t, &Task{
		TaskID:     "old-task",
		Platform:   constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI)),
		SubmitTime: now - 25*3600,
		FinishTime: now - 24*3600,
		Status:     TaskStatusSuccess,
		Properties: Properties{OriginModelName: "doubao-seedance-2-0-260128"},
	})

	result, err := GetStarAIVideoPerformance("doubao-seedance-2-0-260128", 24)
	require.NoError(t, err)
	assert.Zero(t, result.Summary.SubmittedCount)
	assert.Empty(t, result.Groups)
	assert.Empty(t, result.Series)
}
