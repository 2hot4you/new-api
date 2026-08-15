package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestIsModelPricingConfiguredRejectsSelfUseFallback(t *testing.T) {
	originalSelfUseMode := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = originalSelfUseMode
	})

	require.False(t, IsModelPricingConfigured("marketplace-unknown-self-use-model"))
}

func TestIsModelPricingConfiguredAcceptsBillingExpressionOnly(t *testing.T) {
	// minimax-m3 has a non-empty tiered billing expression and intentionally
	// has no entry in the ratio or fixed-price defaults.
	require.True(t, IsModelPricingConfigured("minimax-m3"))
}
