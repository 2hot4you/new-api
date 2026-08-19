package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultMoliiGrokImagePrices(t *testing.T) {
	tests := []struct {
		model      string
		resolution string
		wantOutput float64
		wantInput  float64
	}{
		{"grok-imagine-image", "1k", 0.02, 0.002},
		{"grok-imagine-image", "2K", 0.02, 0.002},
		{"grok-imagine-image-quality", "1k", 0.05, 0.01},
		{"grok-imagine-image-quality", "2k", 0.07, 0.01},
	}
	for _, tt := range tests {
		output, input, ok := GetMoliiGrokImagePrices(tt.model, tt.resolution)
		require.True(t, ok)
		assert.Equal(t, tt.wantOutput, output)
		assert.Equal(t, tt.wantInput, input)
	}
	_, _, ok := GetMoliiGrokImagePrices("other", "1k")
	assert.False(t, ok)
}

func TestGrokImagineImage20PricesFollowQualityAndResolution(t *testing.T) {
	tests := []struct {
		quality    string
		resolution string
		wantOutput float64
	}{
		{quality: "low", resolution: "1k", wantOutput: 0.04},
		{quality: "low", resolution: "2k", wantOutput: 0.06},
		{quality: "medium", resolution: "1k", wantOutput: 0.06},
		{quality: "medium", resolution: "2k", wantOutput: 0.08},
		{quality: "", resolution: "1k", wantOutput: 0.06},
	}
	for _, tt := range tests {
		output, input, ok := GetMoliiGrokImagePricesForQuality(
			"grok-imagine-image-2.0",
			tt.resolution,
			tt.quality,
		)
		require.True(t, ok)
		assert.Equal(t, tt.wantOutput, output)
		assert.Equal(t, 0.01, input)
	}

	_, _, ok := GetMoliiGrokImagePricesForQuality("grok-imagine-image-2.0", "1k", "high")
	assert.False(t, ok)
	_, _, ok = GetMoliiGrokImagePricesForQuality("grok-imagine-image-2.0", "4k", "medium")
	assert.False(t, ok)
}

func TestDefaultMoliiGrokVideoPrices(t *testing.T) {
	tests := []struct {
		model      string
		resolution string
		wantOutput float64
		wantImage  float64
		wantVideo  float64
	}{
		{"grok-imagine-video-1.5", "480p", 0.08, 0.01, 0},
		{"grok-imagine-video-1.5", "720p", 0.14, 0.01, 0},
		{"grok-imagine-video-1.5", "1080p", 0.25, 0.01, 0},
		{"grok-imagine-video", "480p", 0.05, 0.002, 0.01},
		{"grok-imagine-video", "720P", 0.07, 0.002, 0.01},
	}
	for _, tt := range tests {
		output, image, video, ok := GetMoliiGrokVideoPrices(tt.model, tt.resolution)
		require.True(t, ok)
		assert.Equal(t, tt.wantOutput, output)
		assert.Equal(t, tt.wantImage, image)
		assert.Equal(t, tt.wantVideo, video)
	}
	_, _, _, ok := GetMoliiGrokVideoPrices("grok-imagine-video", "1080p")
	assert.False(t, ok)
}

func TestMoliiGrokCatalogPricingExposesDirectPrices(t *testing.T) {
	image, ok := GetMoliiGrokCatalogPricing("grok-imagine-image-quality")
	require.True(t, ok)
	assert.Equal(t, "image", image.Kind)
	assert.Equal(t, 0.05, image.OutputPrices["1k"])
	assert.Equal(t, 0.07, image.OutputPrices["2k"])
	assert.Equal(t, 0.01, image.ImageInputPrice)

	image20, ok := GetMoliiGrokCatalogPricing("grok-imagine-image-2.0")
	require.True(t, ok)
	assert.Equal(t, map[string]float64{
		"low/1k":    0.04,
		"low/2k":    0.06,
		"medium/1k": 0.06,
		"medium/2k": 0.08,
	}, image20.OutputPrices)
	assert.Equal(t, 0.01, image20.ImageInputPrice)

	video, ok := GetMoliiGrokCatalogPricing("grok-imagine-video")
	require.True(t, ok)
	assert.Equal(t, "second", video.OutputUnit)
	assert.Equal(t, 0.05, video.OutputPrices["480p"])
	assert.Equal(t, 0.07, video.OutputPrices["720p"])
	assert.Equal(t, 0.002, video.ImageInputPrice)
	assert.Equal(t, 0.01, video.VideoInputPrice)
	assert.NotContains(t, video.OutputPrices, "1080p")

	_, ok = GetMoliiGrokCatalogPricing("grok-imagine-video-1.5-preview")
	assert.False(t, ok)
}
