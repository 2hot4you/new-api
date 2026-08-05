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
