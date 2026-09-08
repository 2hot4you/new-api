package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestInitConstantEnvUsesSevenDayStarAIAssetDefault(t *testing.T) {
	previousTTL := constant.StarAIAssetTTLHours
	t.Cleanup(func() { constant.StarAIAssetTTLHours = previousTTL })
	t.Setenv("STARAI_ASSET_TTL_HOURS", "")

	initStarAIAssetTTLHours()

	require.Equal(t, 168, constant.StarAIAssetTTLHours)
}

func TestInitConstantEnvHonorsStarAIAssetTTLOverride(t *testing.T) {
	previousTTL := constant.StarAIAssetTTLHours
	t.Cleanup(func() { constant.StarAIAssetTTLHours = previousTTL })
	t.Setenv("STARAI_ASSET_TTL_HOURS", "72")

	initStarAIAssetTTLHours()

	require.Equal(t, 72, constant.StarAIAssetTTLHours)
}
