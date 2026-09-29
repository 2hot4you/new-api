package ratio_setting

import (
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildStarAIVideoBillingExpressionsPricesEverySeedanceTier(t *testing.T) {
	prices := StarAIVideoPriceSetting{
		Standard720p: 46, Standard720pVideo: 28,
		Standard1080p: 51, Standard1080pVideo: 31,
		Standard4K: 26, Standard4KVideo: 16,
		Fast720p: 37, Fast720pVideo: 22,
		Mini720p: 23, Mini720pVideo: 14,
		Seedance25720p: 70, Seedance25720pVideo: 42,
		Seedance251080p: 77, Seedance251080pVideo: 46,
	}
	expressions, err := BuildStarAIVideoBillingExpressions(prices)
	require.NoError(t, err)
	require.Len(t, expressions, 4)

	tests := []struct {
		model, resolution, videoInput, tier string
		wantCost                            float64
	}{
		{"doubao-seedance-2-0-260128", "720p", "none", "720p", 92},
		{"doubao-seedance-2-0-260128", "720p", "video", "720p_video", 56},
		{"doubao-seedance-2-0-260128", "1080p", "none", "1080p", 102},
		{"doubao-seedance-2-0-260128", "1080p", "video", "1080p_video", 62},
		{"doubao-seedance-2-0-260128", "4k", "none", "4k", 52},
		{"doubao-seedance-2-0-260128", "4k", "video", "4k_video", 32},
		{"doubao-seedance-2-0-fast-260128", "720p", "none", "720p", 74},
		{"doubao-seedance-2-0-fast-260128", "720p", "video", "720p_video", 44},
		{"doubao-seedance-2-0-mini-260615", "720p", "none", "720p", 46},
		{"doubao-seedance-2-0-mini-260615", "720p", "video", "720p_video", 28},
		{"doubao-seedance-2-5-260628", "720p", "none", "720p", 140},
		{"doubao-seedance-2-5-260628", "720p", "video", "720p_video", 84},
		{"doubao-seedance-2-5-260628", "1080p", "none", "1080p", 154},
		{"doubao-seedance-2-5-260628", "1080p", "video", "1080p_video", 92},
	}
	for _, test := range tests {
		t.Run(test.model+"/"+test.resolution+"/"+test.videoInput, func(t *testing.T) {
			cost, trace, runErr := billingexpr.RunExprWithRequest(expressions[test.model], billingexpr.TokenParams{}, billingexpr.RequestInput{Usage: map[string]any{
				"tokens": float64(2_000_000), "resolution": test.resolution, "video_input": test.videoInput,
			}})
			require.NoError(t, runErr)
			assert.Equal(t, test.wantCost, cost)
			assert.Equal(t, test.tier, trace.MatchedTier)
		})
	}
}

func TestBuildStarAIVideoBillingExpressionsRejectsInvalidPrices(t *testing.T) {
	prices := StarAIVideoPriceSetting{Standard720p: math.NaN()}
	_, err := BuildStarAIVideoBillingExpressions(prices)
	require.ErrorContains(t, err, "standard_720p")
}

func TestParseStarAIVideoPriceSettingRequiresCompleteExactMatrix(t *testing.T) {
	valid := `{"standard_720p":46,"standard_720p_video":28,"standard_1080p":51,"standard_1080p_video":31,"standard_4k":26,"standard_4k_video":16,"fast_720p":37,"fast_720p_video":22,"mini_720p":23,"mini_720p_video":14,"seedance_25_720p":70,"seedance_25_720p_video":42,"seedance_25_1080p":77,"seedance_25_1080p_video":46}`

	prices, err := ParseStarAIVideoPriceSetting(valid)
	require.NoError(t, err)
	assert.Equal(t, float64(46), prices.Standard720p)
	assert.Equal(t, float64(46), prices.Seedance251080pVideo)

	_, err = ParseStarAIVideoPriceSetting(`{"standard_720p":46}`)
	require.ErrorContains(t, err, "missing")

	_, err = ParseStarAIVideoPriceSetting(strings.TrimSuffix(valid, "}") + `,"unexpected":1}`)
	require.ErrorContains(t, err, "unsupported")

	_, err = ParseStarAIVideoPriceSetting(strings.Replace(valid, `"standard_720p":46`, `"standard_720p":null`, 1))
	require.ErrorContains(t, err, "must be a number")

	_, err = ParseStarAIVideoPriceSetting(strings.TrimSuffix(valid, "}") + `,"standard_720p":47}`)
	require.ErrorContains(t, err, "duplicate")
}

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
		{"doubao-seedance-2-0-mini-260615", "480p", false, 23},
		{"doubao-seedance-2-0-mini-260615", "720p", true, 14},
		{"doubao-seedance-2-5-260628", "480p", false, 70},
		{"doubao-seedance-2-5-260628", "720p", true, 42},
		{"doubao-seedance-2-5-260628", "1080p", false, 77},
		{"doubao-seedance-2-5-260628", "1080p", true, 46},
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

	mini, ok := GetStarAIVideoPricing("doubao-seedance-2-0-mini-260615")
	require.True(t, ok)
	require.Len(t, mini.Rows, 1)
	assert.Equal(t, StarAIVideoPriceRow{Resolutions: []string{"480p", "720p"}, WithoutVideo: 23, WithVideo: 14}, mini.Rows[0])
	assert.Equal(t, []string{"1080p", "4K"}, mini.UnsupportedResolutions)

	seedance25, ok := GetStarAIVideoPricing("doubao-seedance-2-5-260628")
	require.True(t, ok)
	require.Len(t, seedance25.Rows, 2)
	assert.Equal(t, StarAIVideoPriceRow{Resolutions: []string{"480p", "720p"}, WithoutVideo: 70, WithVideo: 42}, seedance25.Rows[0])
	assert.Equal(t, StarAIVideoPriceRow{Resolutions: []string{"1080p"}, WithoutVideo: 77, WithVideo: 46}, seedance25.Rows[1])
	assert.Equal(t, []string{"4K"}, seedance25.UnsupportedResolutions)

	_, ok = GetStarAIVideoPricing("unrelated-model")
	assert.False(t, ok)
}
