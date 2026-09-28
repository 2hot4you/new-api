package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPersistedChannelMoliiRegistryMeanings(t *testing.T) {
	tests := []struct {
		name          string
		persistedType int
		declaredType  int
		wantName      string
		wantBaseURL   string
	}{
		{
			name:          "61 remains StarAI",
			persistedType: 61,
			declaredType:  ChannelTypeStarAI,
			wantName:      "Molii Volcengine Imagine API",
			wantBaseURL:   "https://openapi.starcube.art",
		},
		{
			name:          "62 remains Molii Grok AIGC",
			persistedType: 62,
			declaredType:  ChannelTypeMoliiGrokAIGC,
			wantName:      "Molii Grok Imagine API",
			wantBaseURL:   "https://api.wxiai.com/xai",
		},
		{
			name:          "63 remains Task Plugin",
			persistedType: 63,
			declaredType:  ChannelTypeTaskPlugin,
			wantName:      "Task Plugin",
			wantBaseURL:   "",
		},
		{
			name:          "64 remains ByteDance Seedance",
			persistedType: 64,
			declaredType:  ChannelTypeByteDanceSeedance,
			wantName:      "ByteDance Seedance",
			wantBaseURL:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.persistedType, test.declaredType)
			assert.Equal(t, test.wantName, GetChannelTypeName(test.persistedType))
			assert.Equal(t, test.wantBaseURL, GetChannelBaseURL(test.persistedType))
		})
	}
}

func TestUpstreamRc38ChannelsUseUnusedPersistedIDs(t *testing.T) {
	tests := []struct {
		name          string
		persistedType int
		declaredType  int
		wantName      string
	}{
		{
			name:          "vLLM uses the first ID after Molii channels",
			persistedType: 65,
			declaredType:  ChannelTypeVLLM,
			wantName:      "vLLM",
		},
		{
			name:          "SGLang follows vLLM without reusing Task Plugin",
			persistedType: 66,
			declaredType:  ChannelTypeSGLang,
			wantName:      "SGLang",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.persistedType, test.declaredType)
			assert.Equal(t, test.wantName, GetChannelTypeName(test.persistedType))
			assert.Empty(t, GetChannelBaseURL(test.persistedType))
		})
	}
}
