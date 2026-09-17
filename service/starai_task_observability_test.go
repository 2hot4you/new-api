package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureStarAITaskTimingBuildsExactPhaseDurations(t *testing.T) {
	task := &model.Task{
		SubmitTime: 1000,
		StartTime:  1133,
		FinishTime: 2107,
	}
	payload := []byte(`{
		"code":"success",
		"data":{
			"submit_time":1002,
			"start_time":1131,
			"finish_time":2105,
			"data":{"created_at":1002,"updated_at":2105}
		}
	}`)

	CaptureStarAITaskTiming(task, payload, model.TaskStatusSuccess, 2107)

	require.NotNil(t, task.PrivateData.Timing)
	assert.EqualValues(t, 1000, task.PrivateData.Timing.PlatformSubmittedAt)
	assert.EqualValues(t, 1002, task.PrivateData.Timing.UpstreamSubmittedAt)
	assert.EqualValues(t, 1131, task.PrivateData.Timing.UpstreamStartedAt)
	assert.EqualValues(t, 2105, task.PrivateData.Timing.UpstreamFinishedAt)
	assert.EqualValues(t, 2107, task.PrivateData.Timing.PlatformFinishedObservedAt)

	summary := BuildTaskTimingSummary(task)
	require.NotNil(t, summary)
	assert.EqualValues(t, 2, *summary.SubmissionSeconds)
	assert.EqualValues(t, 129, *summary.UpstreamQueueSeconds)
	assert.EqualValues(t, 974, *summary.UpstreamGenerationSeconds)
	assert.EqualValues(t, 1103, *summary.UpstreamTotalSeconds)
	assert.EqualValues(t, 2, *summary.StartDetectionDelaySeconds)
	assert.EqualValues(t, 2, *summary.FinishDetectionDelaySeconds)
	assert.EqualValues(t, 1107, *summary.PlatformTotalSeconds)
}

func TestCaptureStarAITaskTimingRecordsFirstProgressObservationOnce(t *testing.T) {
	task := &model.Task{SubmitTime: 1000}
	payload := []byte(`{"data":{"submit_time":1001,"start_time":1010}}`)

	CaptureStarAITaskTiming(task, payload, model.TaskStatusInProgress, 1012)
	CaptureStarAITaskTiming(task, payload, model.TaskStatusInProgress, 1020)

	require.NotNil(t, task.PrivateData.Timing)
	assert.EqualValues(t, 1012, task.PrivateData.Timing.PlatformFirstInProgressAt)
}

func TestCaptureStarAITaskTimingPrefersExplicitPlatformRequestStart(t *testing.T) {
	task := &model.Task{SubmitTime: 1000}
	payload := []byte(`{"data":{"submit_time":1002}}`)

	CaptureStarAITaskTiming(task, payload, model.TaskStatusSubmitted, 1003, 995)

	require.NotNil(t, task.PrivateData.Timing)
	assert.EqualValues(t, 995, task.PrivateData.Timing.PlatformSubmittedAt)
	summary := BuildTaskTimingSummary(task)
	require.NotNil(t, summary)
	assert.EqualValues(t, 7, *summary.SubmissionSeconds)
}

func TestBuildTaskTimingSummaryFallsBackToSanitizedLegacyData(t *testing.T) {
	task := &model.Task{
		SubmitTime: 1000,
		StartTime:  1012,
		FinishTime: 1025,
		Data: []byte(`{
			"data":{
				"submit_time":1001,
				"start_time":1010,
				"finish_time":1023
			}
		}`),
	}

	summary := BuildTaskTimingSummary(task)

	require.NotNil(t, summary)
	assert.EqualValues(t, 9, *summary.UpstreamQueueSeconds)
	assert.EqualValues(t, 13, *summary.UpstreamGenerationSeconds)
	assert.EqualValues(t, 2, *summary.StartDetectionDelaySeconds)
	assert.EqualValues(t, 2, *summary.FinishDetectionDelaySeconds)
	assert.EqualValues(t, 25, *summary.PlatformTotalSeconds)
}

func TestBuildTaskTimingSummaryOmitsNegativeClockSkewDurations(t *testing.T) {
	task := &model.Task{
		SubmitTime: 1000,
		StartTime:  1005,
		FinishTime: 1010,
		PrivateData: model.TaskPrivateData{Timing: &model.TaskTimingSnapshot{
			PlatformSubmittedAt:        1000,
			UpstreamSubmittedAt:        1006,
			UpstreamStartedAt:          1004,
			UpstreamFinishedAt:         1009,
			PlatformFirstInProgressAt:  1005,
			PlatformFinishedObservedAt: 1010,
		}},
	}

	summary := BuildTaskTimingSummary(task)

	require.NotNil(t, summary)
	assert.Nil(t, summary.UpstreamQueueSeconds)
	assert.NotNil(t, summary.UpstreamGenerationSeconds)
	assert.EqualValues(t, 1, *summary.StartDetectionDelaySeconds)
}
