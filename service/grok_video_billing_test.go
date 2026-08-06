package service

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGrokVideoBillingSnapshotOperations(t *testing.T) {
	tests := []struct {
		name            string
		action          string
		request         relaycommon.TaskSubmitReq
		modelName       string
		estimatedSecs   int
		estimatedRes    string
		wantOperation   string
		wantInputType   string
		wantImageCount  int
		wantEstimated   float64
		wantVideoBilled float64
	}{
		{name: "text", request: relaycommon.TaskSubmitReq{Duration: 5, Resolution: "480p", AspectRatio: "16:9"}, modelName: grokVideoModelLegacy, estimatedSecs: 5, estimatedRes: "480p", wantOperation: textToVideoOperation, wantInputType: "text", wantEstimated: 5},
		{name: "image", request: relaycommon.TaskSubmitReq{Image: "https://fixture.invalid/input.png", Duration: 6, Resolution: "720p", AspectRatio: "9:16"}, modelName: grokVideoModelLegacy, estimatedSecs: 6, estimatedRes: "720p", wantOperation: imageToVideoOperation, wantInputType: "image", wantImageCount: 1, wantEstimated: 6},
		{name: "image 1.5 1080p", request: relaycommon.TaskSubmitReq{Image: "file_fixture", Duration: 4, Resolution: "1080p", AspectRatio: "16:9"}, modelName: grokVideoModel15, estimatedSecs: 4, estimatedRes: "1080p", wantOperation: imageToVideoOperation, wantInputType: "image", wantImageCount: 1, wantEstimated: 4},
		{name: "edit", action: grokVideoEditOperation, request: relaycommon.TaskSubmitReq{Video: "https://fixture.invalid/input.mp4"}, modelName: grokVideoModelLegacy, estimatedSecs: 9, estimatedRes: "720p", wantOperation: grokVideoEditOperation, wantInputType: "video", wantEstimated: 8.7, wantVideoBilled: 8.7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)
			c.Set("task_request", tt.request)
			info := &relaycommon.RelayInfo{
				TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: tt.action}, OriginModelName: tt.modelName,
				EstimatedVideoSeconds: tt.estimatedSecs, EstimatedVideoResolution: tt.estimatedRes,
				PriceData: types.PriceData{GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1.25}},
			}
			got := BuildGrokVideoBillingSnapshot(c, info, 12345)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantOperation, got.Operation)
			assert.Equal(t, tt.wantInputType, got.InputType)
			assert.Equal(t, tt.wantImageCount, got.InputImageCount)
			assert.Equal(t, tt.wantEstimated, got.EstimatedDurationSeconds)
			assert.Equal(t, tt.wantVideoBilled, got.VideoInputBilledSeconds)
			assert.Equal(t, 1.25, got.GroupRatio)
			assert.NotContains(t, string(mustJSON(t, got)), "fixture.invalid")
		})
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return encoded
}

func TestCalculateGrokVideoBillingCostsByOperation(t *testing.T) {
	tests := []struct {
		name     string
		snapshot model.GrokVideoBillingSnapshot
		seconds  float64
		want     float64
	}{
		{name: "text", snapshot: model.GrokVideoBillingSnapshot{Operation: textToVideoOperation, OutputUnitPrice: 0.05}, seconds: 6, want: 0.3},
		{name: "image", snapshot: model.GrokVideoBillingSnapshot{Operation: imageToVideoOperation, OutputUnitPrice: 0.07, ImageInputUnitPrice: 0.002, InputImageCount: 1}, seconds: 6, want: 0.422},
		{name: "edit", snapshot: model.GrokVideoBillingSnapshot{Operation: grokVideoEditOperation, OutputUnitPrice: 0.05, VideoInputUnitPrice: 0.01, VideoInputBilledSeconds: 6}, seconds: 6, want: 0.36},
		{name: "explicit zero", snapshot: model.GrokVideoBillingSnapshot{Operation: imageToVideoOperation, InputImageCount: 1}, seconds: 6, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calculateGrokVideoBillingCosts(&tt.snapshot, tt.seconds)
			assert.InDelta(t, tt.want, tt.snapshot.Subtotal, 1e-12)
		})
	}
}

func TestGrokVideoBillingSnapshotPreservesExplicitZeroPrices(t *testing.T) {
	snapshot := model.GrokVideoBillingSnapshot{
		Version: 1, Model: "grok-imagine-video", Operation: "video_edit", InputType: "video",
		OutputUnitPrice: 0, ImageInputUnitPrice: 0, VideoInputUnitPrice: 0,
	}

	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	text := string(encoded)
	assert.Contains(t, text, `"output_unit_price":0`)
	assert.Contains(t, text, `"image_input_unit_price":0`)
	assert.Contains(t, text, `"video_input_unit_price":0`)

	var decoded model.GrokVideoBillingSnapshot
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, snapshot, decoded)
}

func TestConfigureGrokVideoFinalUsageRequiresValidSnapshot(t *testing.T) {
	bc := &model.TaskBillingContext{}
	assert.False(t, ConfigureGrokVideoFinalUsage(bc, nil, "/v1/videos"))
	assert.False(t, bc.FinalUsageLogOnly)
	assert.Empty(t, bc.RequestPath)
	assert.Nil(t, bc.GrokVideoBilling)

	valid := &model.GrokVideoBillingSnapshot{
		Version: 1, Model: grokVideoModelLegacy, Operation: textToVideoOperation, InputType: "text",
		EstimatedDurationSeconds: 5, EstimatedResolution: "480p", GroupRatio: 1,
	}
	assert.True(t, ConfigureGrokVideoFinalUsage(bc, valid, "/v1/videos/generations"))
	assert.True(t, bc.FinalUsageLogOnly)
	assert.Equal(t, "/v1/videos/generations", bc.RequestPath)
	assert.Same(t, valid, bc.GrokVideoBilling)
}

func TestFinalizeGrokVideoBillingUsesSettledQuotaAsLedgerAuthority(t *testing.T) {
	task := &model.Task{Quota: 180001, PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		GroupRatio: 0.5,
		GrokVideoBilling: &model.GrokVideoBillingSnapshot{
			Version: 1, Model: "grok-imagine-video", Operation: "video_edit", InputType: "video",
			ActualDurationSeconds: 6, ActualResolution: "480p", VideoInputBilledSeconds: 6,
			OutputUnitPrice: 0.05, VideoInputUnitPrice: 0.01,
			OutputCost: 0.3, VideoInputCost: 0.06, Subtotal: 0.36,
		},
	}}}

	got, content := finalGrokVideoBilling(task)
	require.NotNil(t, got)
	assert.Equal(t, 0.5, got.GroupRatio)
	assert.InDelta(t, float64(task.Quota)/common.QuotaPerUnit, got.FinalCost, 1e-12)
	assert.Contains(t, content, "Grok")
	assert.Contains(t, content, "480p")
	assert.NotContains(t, content, "task_")
}
