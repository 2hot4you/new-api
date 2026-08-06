package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestTaskUsesFinalUsageLogPreservesRolloutCompatibility(t *testing.T) {
	legacyGrok := &model.Task{Platform: constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeMoliiGrokAIGC)), PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{}}}
	newGrok := &model.Task{Platform: legacyGrok.Platform, PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{FinalUsageLogOnly: true}}}
	starAI := &model.Task{Platform: constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeStarAI))}

	assert.False(t, taskUsesFinalUsageLog(legacyGrok))
	assert.True(t, taskUsesFinalUsageLog(newGrok))
	assert.True(t, taskUsesFinalUsageLog(starAI))
}

func TestMoliiGrokBillingContextRoundTripKeepsRolloutAndSnapshot(t *testing.T) {
	original := model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		FinalUsageLogOnly: true,
		RequestPath:       "/v1/videos/edits",
		GrokVideoBilling: &model.GrokVideoBillingSnapshot{
			Version: 1, Model: "grok-imagine-video", Operation: "video_edit", InputType: "video",
			OutputUnitPrice: 0, ImageInputUnitPrice: 0, VideoInputUnitPrice: 0,
		},
	}}
	encoded, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.Contains(t, string(encoded), `"final_usage_log_only":true`)
	assert.Contains(t, string(encoded), `"output_unit_price":0`)

	var decoded model.TaskPrivateData
	assert.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.True(t, decoded.BillingContext.FinalUsageLogOnly)
	assert.Equal(t, "/v1/videos/edits", decoded.BillingContext.RequestPath)
	assert.Equal(t, original.BillingContext.GrokVideoBilling, decoded.BillingContext.GrokVideoBilling)
}

func TestMoliiGrokFixedPriceBillingContextKeepsVideoAuditFieldsWithoutTokenEstimate(t *testing.T) {
	task := &model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		ModelPrice:          0.5,
		GroupRatio:          1,
		PerCallBilling:      true,
		EstimatedTokens:     0,
		EstimatedSeconds:    5,
		EstimatedResolution: "480p",
		EstimatedRatio:      "16:9",
	}}}

	other := taskBillingOther(task)
	assert.Equal(t, 5, other["estimated_seconds"])
	assert.Equal(t, "480p", other["estimated_resolution"])
	assert.Equal(t, "16:9", other["estimated_ratio"])
	assert.NotContains(t, other, "estimated_tokens")
}
