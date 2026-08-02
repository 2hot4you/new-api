package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultStarAIVideoPrices(t *testing.T) {
	tests := []struct {
		model      string
		resolution string
		hasVideo   bool
		want       float64
	}{
		{"doubao-seedance-2-0-260128", "720p", false, 46},
		{"doubao-seedance-2-0-260128", "720p", true, 28},
		{"doubao-seedance-2-0-260128", "1080p", false, 51},
		{"doubao-seedance-2-0-260128", "1080p", true, 31},
		{"doubao-seedance-2-0-260128", "4k", false, 26},
		{"doubao-seedance-2-0-260128", "4k", true, 16},
		{"doubao-seedance-2-0-fast-260128", "720p", false, 37},
		{"doubao-seedance-2-0-fast-260128", "720p", true, 22},
	}
	for _, tt := range tests {
		got, ok := GetStarAIVideoPrice(tt.model, tt.resolution, tt.hasVideo)
		require.True(t, ok)
		assert.Equal(t, tt.want, got)
	}
}

func TestStarAIVideoPricingMatrix(t *testing.T) {
	standard, ok := GetStarAIVideoPricing("doubao-seedance-2-0-260128")
	require.True(t, ok)
	assert.Equal(t, "cny_per_million_tokens", standard.Unit)
	assert.Equal(t, 24, standard.FPS)
	assert.Equal(t, 1, standard.ExtraFrames)
	require.Len(t, standard.Rows, 3)
	assert.Equal(t, []string{"480p", "720p"}, standard.Rows[0].Resolutions)
	assert.Equal(t, StarAIVideoPriceRow{Resolutions: []string{"1080p"}, WithoutVideo: 51, WithVideo: 31}, standard.Rows[1])
	assert.Equal(t, StarAIVideoPriceRow{Resolutions: []string{"4K"}, WithoutVideo: 26, WithVideo: 16}, standard.Rows[2])
	assert.Empty(t, standard.UnsupportedResolutions)

	fast, ok := GetStarAIVideoPricing("doubao-seedance-2-0-fast-260128")
	require.True(t, ok)
	require.Len(t, fast.Rows, 1)
	assert.Equal(t, StarAIVideoPriceRow{Resolutions: []string{"480p", "720p"}, WithoutVideo: 37, WithVideo: 22}, fast.Rows[0])
	assert.Equal(t, []string{"1080p", "4K"}, fast.UnsupportedResolutions)

	_, ok = GetStarAIVideoPricing("unrelated-model")
	assert.False(t, ok)
}
