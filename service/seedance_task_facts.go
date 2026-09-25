package service

import (
	"encoding/json"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// Derive the displayed unit price from the actual local ratio snapshot, not
// the direct-provider price field or a backwards division of the charged cost.
func SeedanceResellerBillingDetail(task *model.Task) *dto.TaskSeedanceBilling {
	if task == nil || task.PrivateData.BillingContext == nil {
		return nil
	}
	bc := *task.PrivateData.BillingContext
	copyTask := *task
	copyTask.PrivateData.BillingContext = &bc
	quota := seedanceResellerTargetQuota(&copyTask, &relaycommon.TaskInfo{TotalTokens: bc.ActualTokens})
	if quota == nil || *quota != task.Quota {
		return nil
	}
	multiplier := 1.0
	for _, ratio := range bc.OtherRatios {
		multiplier *= ratio
	}
	unit := bc.ModelRatio * multiplier * 1_000_000 / common.QuotaPerUnit
	if math.IsNaN(unit) || math.IsInf(unit, 0) || unit <= 0 {
		return nil
	}
	params := dto.TaskVideoParams{Resolution: bc.EstimatedResolution, Ratio: bc.EstimatedRatio, Seconds: bc.EstimatedSeconds, HasVideo: bc.EstimatedHasVideo}
	ApplySeedanceVideoFacts(&params, task.Data)
	return &dto.TaskSeedanceBilling{ActualTokens: bc.ActualTokens, Resolution: params.Resolution, Ratio: params.Ratio, Seconds: params.Seconds, HasVideo: params.HasVideo, UnitPrice: unit, ModelRatio: &bc.ModelRatio, OtherRatio: &multiplier}
}

// SeedanceTaskFacts contains public, content-free facts only. It is also the
// persisted wire projection consumed by the next reseller hop.
type SeedanceTaskFacts struct {
	SubmitTime      int64   `json:"submit_time,omitempty"`
	StartTime       int64   `json:"start_time,omitempty"`
	FinishTime      int64   `json:"finish_time,omitempty"`
	Duration        float64 `json:"duration,omitempty"`
	Resolution      string  `json:"resolution,omitempty"`
	Ratio           string  `json:"ratio,omitempty"`
	InputImageCount *int    `json:"input_image_count,omitempty"`
	InputVideoCount *int    `json:"input_video_count,omitempty"`
	InputAudioCount *int    `json:"input_audio_count,omitempty"`
}

func CaptureSeedanceTaskFacts(task *model.Task, observed int64, started ...int64) {
	// The timing merger is provider-neutral; only this normalized projection,
	// never the reseller's raw upstream payload, enters it.
	CaptureStarAITaskTiming(task, task.Data, task.Status, observed, started...)
	var facts SeedanceTaskFacts
	if json.Unmarshal(task.Data, &facts) != nil {
		return
	}
	if facts.InputImageCount != nil && facts.InputVideoCount != nil && facts.InputAudioCount != nil {
		task.PrivateData.InputMedia = &model.TaskInputMediaSummary{ImageCount: *facts.InputImageCount, VideoCount: *facts.InputVideoCount, AudioCount: *facts.InputAudioCount}
	}
}

func ApplySeedanceVideoFacts(params *dto.TaskVideoParams, data []byte) {
	var facts SeedanceTaskFacts
	if json.Unmarshal(data, &facts) != nil {
		return
	}
	if facts.Duration > 0 && facts.Duration <= 300 {
		params.Seconds = int(facts.Duration)
	}
	switch facts.Resolution {
	case "480p", "720p", "1080p", "4k":
		params.Resolution = facts.Resolution
	}
	switch facts.Ratio {
	case "16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "3:2", "2:3", "adaptive":
		params.Ratio = facts.Ratio
	}
	for _, pair := range []struct {
		source *int
		target **int
	}{{facts.InputImageCount, &params.InputImageCount}, {facts.InputVideoCount, &params.InputVideoCount}, {facts.InputAudioCount, &params.InputAudioCount}} {
		if pair.source != nil && *pair.source >= 0 && *pair.source <= 50 {
			*pair.target = pair.source
		}
	}
	if params.InputVideoCount != nil {
		params.HasVideo = *params.InputVideoCount > 0
	}
}
