package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSeedancePricesUseRMBPerMillionTokenBaselines(t *testing.T) {
	prices := GetDefaultModelRatioMap()

	standard, ok := prices["doubao-seedance-2-0-260128"]
	require.True(t, ok)
	assert.InDelta(t, 23, standard, 1e-12)

	fast, ok := prices["doubao-seedance-2-0-fast-260128"]
	require.True(t, ok)
	assert.InDelta(t, 18.5, fast, 1e-12)

	mini, ok := prices["doubao-seedance-2-0-mini-260615"]
	require.True(t, ok)
	assert.InDelta(t, 11.5, mini, 1e-12)

	seedance25, ok := prices["doubao-seedance-2-5-260628"]
	require.True(t, ok)
	assert.InDelta(t, 35, seedance25, 1e-12)
}
