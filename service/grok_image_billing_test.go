package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type zeroCostBillingSettler struct {
	preConsumed int
	settled     int
}

func (s *zeroCostBillingSettler) Settle(actualQuota int) error {
	s.settled = actualQuota
	return nil
}

func (*zeroCostBillingSettler) Refund(*gin.Context) {}

func (*zeroCostBillingSettler) NeedsRefund() bool { return false }

func (s *zeroCostBillingSettler) GetPreConsumedQuota() int { return s.preConsumed }

func (*zeroCostBillingSettler) Reserve(int) error { return nil }

func TestGrokImageZeroSubtotalSettlesAtZeroQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-image-quality",
		StartTime:       time.Now(),
		PriceData: hosttypes.PriceData{
			UsePrice:   true,
			ModelPrice: 1,
			GroupRatioInfo: hosttypes.GroupRatioInfo{
				GroupRatio: 1,
			},
		},
		GrokImageBilling: &relaycommon.GrokImageBillingSnapshot{
			Version:              1,
			Model:                "grok-imagine-image-quality",
			Operation:            "generation",
			Resolution:           "2k",
			AspectRatio:          "16:9",
			RequestedOutputCount: 1,
			OutputCount:          1,
			OutputUnitPrice:      0,
			InputUnitPrice:       0,
			Subtotal:             0,
		},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, &dto.Usage{PromptTokens: 1, TotalTokens: 1})
	assert.Zero(t, summary.Quota, "zero-priced Grok images must not settle at the fixed-price anchor or one-quota minimum")
	billing := &zeroCostBillingSettler{preConsumed: int(common.QuotaPerUnit)}
	relayInfo.Billing = billing
	require.NoError(t, SettleBilling(ctx, relayInfo, summary.Quota))
	assert.Zero(t, billing.settled, "settlement must receive zero so the billing session refunds the pre-consumed anchor")

	other := map[string]interface{}{}
	content := appendGrokImageBillingLog(other, relayInfo, summary.GroupRatio, summary.Quota)
	assert.Contains(t, content, "= ¥0.000000")
	assert.Zero(t, relayInfo.GrokImageBilling.FinalCost)
}

func TestGrokImageZeroSubtotalDoesNotAffectOtherFixedPriceModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: "other-fixed-price-model",
		StartTime:       time.Now(),
		PriceData: hosttypes.PriceData{
			UsePrice:       true,
			ModelPrice:     1,
			GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
		GrokImageBilling: &relaycommon.GrokImageBillingSnapshot{Version: 1, Subtotal: 0},
	}

	summary := calculateTextQuotaSummary(ctx, relayInfo, &dto.Usage{PromptTokens: 1, TotalTokens: 1})
	assert.Equal(t, int(common.QuotaPerUnit), summary.Quota)
}

func TestAppendGrokImageBillingLogUsesActualEditCountsAndSettledQuota(t *testing.T) {
	snapshot := &relaycommon.GrokImageBillingSnapshot{
		Version:              1,
		Model:                "grok-imagine-image-quality",
		Operation:            "edit",
		Resolution:           "2k",
		AspectRatio:          "16:9",
		RequestedOutputCount: 2,
		OutputCount:          1,
		InputImageCount:      2,
		OutputUnitPrice:      0.07,
		InputUnitPrice:       0.01,
		OutputCost:           0.07,
		InputCost:            0.02,
		Subtotal:             0.09,
	}
	relayInfo := &relaycommon.RelayInfo{GrokImageBilling: snapshot}
	other := map[string]interface{}{}
	settledQuota := int(0.135 * common.QuotaPerUnit)

	content := appendGrokImageBillingLog(other, relayInfo, 1.5, settledQuota)

	assert.Equal(t, "Grok 图片编辑, 模型 grok-imagine-image-quality, 分辨率 2K, 比例 16:9, 输出 1 张, 输入 2 张, 计费 (¥0.070000 × 1 + ¥0.010000 × 2) × 1.5000 = ¥0.135000", content)
	got, ok := other["grok_image_billing"].(*relaycommon.GrokImageBillingSnapshot)
	require.True(t, ok)
	assert.Equal(t, 1, got.Version)
	assert.InDelta(t, 1.5, got.GroupRatio, 0.000001)
	assert.InDelta(t, 0.135, got.FinalCost, 0.000001)
	assert.InDelta(t, 0.09, got.Subtotal, 0.000001)
}

func TestAppendGrokImageBillingLogGenerationAndOrdinaryCompatibility(t *testing.T) {
	other := map[string]interface{}{}
	relayInfo := &relaycommon.RelayInfo{GrokImageBilling: &relaycommon.GrokImageBillingSnapshot{
		Version:              1,
		Model:                "grok-imagine-image",
		Operation:            "generation",
		Resolution:           "1k",
		AspectRatio:          "1:1",
		RequestedOutputCount: 3,
		OutputCount:          2,
		OutputUnitPrice:      0.02,
		OutputCost:           0.04,
		Subtotal:             0.04,
	}}

	content := appendGrokImageBillingLog(other, relayInfo, 1.25, int(0.05*common.QuotaPerUnit))
	assert.Equal(t, "Grok 图片生成, 模型 grok-imagine-image, 分辨率 1K, 比例 1:1, 输出 2 张, 计费 (¥0.020000 × 2) × 1.2500 = ¥0.050000", content)
	assert.Contains(t, common.MapToJsonStr(other), `"input_cost":0`)

	ordinary := map[string]interface{}{}
	assert.Empty(t, appendGrokImageBillingLog(ordinary, &relaycommon.RelayInfo{}, 1, 123))
	assert.NotContains(t, ordinary, "grok_image_billing")
}
