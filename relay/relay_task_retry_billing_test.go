package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestTaskRetryRecomputesInferredBillingAndPreservesOverride(t *testing.T) {
	for _, tc := range []struct {
		name, override string
		secondPerCall  bool
	}{
		{"recompute mapped tail", "", false},
		{"preserve explicit override", "explicit-price", false},
		{"clear stale tiered snapshot", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saveBillingConfig(t)
			oldPrices := ratio_setting.ModelPrice2JSONString()
			t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldPrices)) })
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString("{}"))
			require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
				"billing_setting.billing_mode": `{"first-tail":"tiered_expr","second-tail":"tiered_expr","explicit-price":"tiered_expr"}`,
				"billing_setting.billing_expr": `{"first-tail":"tier(\"first\",2)","second-tail":"tier(\"second\",3)","explicit-price":"tier(\"explicit\",4)"}`,
			}))
			c, info := newTaskSubmitContext(t, "retry-alias", `{"retry-alias":"first-tail"}`)
			pinMappingOrderPlugin(t, c, billingFallbackPlugin)
			info.OriginModelName = "retry-alias"
			info.BillingModelName = tc.override
			info.UserGroup, info.UsingGroup = "default", "default"
			common.SetContextKey(c, constant.ContextKeyChannelId, 1)
			_, err := RelayTaskSubmit(c, info)
			require.NotNil(t, err, "fixture has no funded billing account")
			require.NotEqual(t, "model_price_error", err.Code)
			require.NotNil(t, info.TieredBillingSnapshot)
			wantFirst := "first-tail"
			if tc.override != "" {
				wantFirst = tc.override
			}
			require.Equal(t, wantFirst, info.TieredBillingSnapshot.ModelName)

			// The controller reuses RelayInfo while selecting a new channel.
			common.SetContextKey(c, constant.ContextKeyChannelId, 2)
			c.Set("model_mapping", `{"retry-alias":"second-tail"}`)
			if tc.secondPerCall {
				require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"retry-alias":5}`))
			}
			_, err = RelayTaskSubmit(c, info)
			require.NotNil(t, err)
			require.NotEqual(t, "model_price_error", err.Code)
			require.Equal(t, "retry-alias", info.OriginModelName)
			require.Equal(t, "second-tail", info.UpstreamModelName)
			if tc.secondPerCall {
				require.Nil(t, info.TieredBillingSnapshot)
				require.Equal(t, "retry-alias", info.GetBillingModelName())
				require.True(t, info.PriceData.UsePrice)
				require.Equal(t, float64(5), info.PriceData.ModelPrice)
			} else {
				require.NotNil(t, info.TieredBillingSnapshot)
				want := "second-tail"
				tier := "second"
				if tc.override != "" {
					want = tc.override
					tier = "explicit"
				}
				require.Equal(t, want, info.GetBillingModelName())
				require.Equal(t, want, info.TieredBillingSnapshot.ModelName)
				require.Equal(t, tier, info.TieredBillingSnapshot.EstimatedTier)
			}
		})
	}
}

func TestTaskSelfUseDefaultDoesNotMaskMappedExpression(t *testing.T) {
	saveBillingConfig(t)
	previous := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() { operation_setting.SelfUseModeEnabled = previous })
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"declared-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"declared-model":"tier(\"mapped\",3)"}`,
	}))
	c, info := newTaskSubmitContext(t, "unpriced-self-use@thinking:on", `{"unpriced-self-use":"declared-model"}`)
	pinMappingOrderPlugin(t, c, billingFallbackPlugin)
	info.OriginModelName = "unpriced-self-use@thinking:on"
	info.UserGroup, info.UsingGroup = "default", "default"
	_, err := RelayTaskSubmit(c, info)
	require.NotNil(t, err)
	require.NotEqual(t, "model_price_error", err.Code)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "declared-model", info.TieredBillingSnapshot.ModelName)
	require.Equal(t, "mapped", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, "unpriced-self-use@thinking:on", info.OriginModelName)
}
