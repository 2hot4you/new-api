package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/brand_setting"
	"github.com/stretchr/testify/require"
)

func TestBrandColorOptionIsNormalizedBeforePersistence(t *testing.T) {
	value, err := normalizeOptionValue(
		brand_setting.ClaudeyeLightMarkColorKey,
		" #abcdef ",
	)
	require.NoError(t, err)
	require.Equal(t, "#ABCDEF", value)
}

func TestBrandColorOptionRejectsInjection(t *testing.T) {
	_, err := normalizeOptionValue(
		brand_setting.ClaudeyeLightTextColorKey,
		`#000000\" onload=\"alert(1)`,
	)
	require.Error(t, err)
}
