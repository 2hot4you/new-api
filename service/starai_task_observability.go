package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type starAITimingEnvelope struct {
	SubmitTime int64 `json:"submit_time"`
	StartTime  int64 `json:"start_time"`
	FinishTime int64 `json:"finish_time"`
	CreatedAt  int64 `json:"created_at"`
	UpdatedAt  int64 `json:"updated_at"`
	Data       struct {
		SubmitTime int64 `json:"submit_time"`
		StartTime  int64 `json:"start_time"`
		FinishTime int64 `json:"finish_time"`
		CreatedAt  int64 `json:"created_at"`
		UpdatedAt  int64 `json:"updated_at"`
		Data       struct {
			CreatedAt int64 `json:"created_at"`
			UpdatedAt int64 `json:"updated_at"`
		} `json:"data"`
	} `json:"data"`
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func extractStarAITiming(payload []byte) model.TaskTimingSnapshot {
	var envelope starAITimingEnvelope
	if len(payload) == 0 || common.Unmarshal(payload, &envelope) != nil {
		return model.TaskTimingSnapshot{}
	}
	return model.TaskTimingSnapshot{
		UpstreamSubmittedAt: firstPositive(
			envelope.Data.SubmitTime,
			envelope.SubmitTime,
			envelope.Data.CreatedAt,
			envelope.Data.Data.CreatedAt,
			envelope.CreatedAt,
		),
		UpstreamStartedAt: firstPositive(envelope.Data.StartTime, envelope.StartTime),
		UpstreamFinishedAt: firstPositive(
			envelope.Data.FinishTime,
			envelope.FinishTime,
		),
	}
}

// CaptureStarAITaskTiming merges provider timestamps with platform-observed
// transition timestamps. First-observed values are immutable so later polling
// passes cannot erase the actual detection delay.
func CaptureStarAITaskTiming(task *model.Task, payload []byte, status model.TaskStatus, observedAt int64, platformRequestStartedAt ...int64) {
	if task == nil {
		return
	}
	if task.PrivateData.Timing == nil {
		task.PrivateData.Timing = &model.TaskTimingSnapshot{}
	}
	snapshot := task.PrivateData.Timing
	if snapshot.PlatformSubmittedAt == 0 {
		snapshot.PlatformSubmittedAt = task.SubmitTime
		if len(platformRequestStartedAt) > 0 && platformRequestStartedAt[0] > 0 {
			snapshot.PlatformSubmittedAt = platformRequestStartedAt[0]
		}
	}
	provider := extractStarAITiming(payload)
	if provider.UpstreamSubmittedAt > 0 {
		snapshot.UpstreamSubmittedAt = provider.UpstreamSubmittedAt
	}
	if provider.UpstreamStartedAt > 0 {
		snapshot.UpstreamStartedAt = provider.UpstreamStartedAt
	}
	if provider.UpstreamFinishedAt > 0 {
		snapshot.UpstreamFinishedAt = provider.UpstreamFinishedAt
	}
	if status == model.TaskStatusInProgress && snapshot.PlatformFirstInProgressAt == 0 {
		snapshot.PlatformFirstInProgressAt = observedAt
	}
	if (status == model.TaskStatusSuccess || status == model.TaskStatusFailure) && snapshot.PlatformFinishedObservedAt == 0 {
		snapshot.PlatformFinishedObservedAt = observedAt
	}
}

func nonNegativeDuration(start int64, finish int64) *int64 {
	if start <= 0 || finish < start {
		return nil
	}
	value := finish - start
	return &value
}

// BuildTaskTimingSummary creates a safe administrator projection. Historical
// tasks without a normalized snapshot fall back to already-sanitized task data.
func BuildTaskTimingSummary(task *model.Task) *dto.TaskTimingInfo {
	if task == nil {
		return nil
	}
	var snapshot model.TaskTimingSnapshot
	if task.PrivateData.Timing != nil {
		snapshot = *task.PrivateData.Timing
	} else {
		snapshot = extractStarAITiming(task.Data)
	}
	if snapshot.PlatformSubmittedAt == 0 {
		snapshot.PlatformSubmittedAt = task.SubmitTime
	}
	if snapshot.PlatformFirstInProgressAt == 0 {
		snapshot.PlatformFirstInProgressAt = task.StartTime
	}
	if snapshot.PlatformFinishedObservedAt == 0 {
		snapshot.PlatformFinishedObservedAt = task.FinishTime
	}
	if snapshot.PlatformSubmittedAt == 0 && snapshot.UpstreamSubmittedAt == 0 &&
		snapshot.UpstreamStartedAt == 0 && snapshot.UpstreamFinishedAt == 0 &&
		snapshot.PlatformFirstInProgressAt == 0 && snapshot.PlatformFinishedObservedAt == 0 {
		return nil
	}
	return &dto.TaskTimingInfo{
		PlatformSubmittedAt:         snapshot.PlatformSubmittedAt,
		UpstreamSubmittedAt:         snapshot.UpstreamSubmittedAt,
		UpstreamStartedAt:           snapshot.UpstreamStartedAt,
		UpstreamFinishedAt:          snapshot.UpstreamFinishedAt,
		PlatformFirstInProgressAt:   snapshot.PlatformFirstInProgressAt,
		PlatformFinishedObservedAt:  snapshot.PlatformFinishedObservedAt,
		SubmissionSeconds:           nonNegativeDuration(snapshot.PlatformSubmittedAt, snapshot.UpstreamSubmittedAt),
		UpstreamQueueSeconds:        nonNegativeDuration(snapshot.UpstreamSubmittedAt, snapshot.UpstreamStartedAt),
		UpstreamGenerationSeconds:   nonNegativeDuration(snapshot.UpstreamStartedAt, snapshot.UpstreamFinishedAt),
		UpstreamTotalSeconds:        nonNegativeDuration(snapshot.UpstreamSubmittedAt, snapshot.UpstreamFinishedAt),
		StartDetectionDelaySeconds:  nonNegativeDuration(snapshot.UpstreamStartedAt, snapshot.PlatformFirstInProgressAt),
		FinishDetectionDelaySeconds: nonNegativeDuration(snapshot.UpstreamFinishedAt, snapshot.PlatformFinishedObservedAt),
		PlatformTotalSeconds:        nonNegativeDuration(snapshot.PlatformSubmittedAt, snapshot.PlatformFinishedObservedAt),
	}
}
