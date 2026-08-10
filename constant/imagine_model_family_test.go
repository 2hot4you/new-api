package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateImagineModelMappingRejectsIncompatibleFamilies(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		billed    string
	}{
		{name: "Seedance standard to fast", requested: "doubao-seedance-2-0-260128", billed: "doubao-seedance-2-0-fast-260128"},
		{name: "Grok basic to quality", requested: "grok-imagine-image", billed: "grok-imagine-image-quality"},
		{name: "Grok image to video", requested: "grok-imagine-image", billed: "grok-imagine-video"},
		{name: "Grok legacy to 1.5", requested: "grok-imagine-video", billed: "grok-imagine-video-1.5"},
		{name: "Grok 1.5 to retired preview", requested: "grok-imagine-video-1.5", billed: "grok-imagine-video-1.5-preview"},
		{name: "retired preview to Grok 1.5", requested: "grok-imagine-video-1.5-preview", billed: "grok-imagine-video-1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, ValidateImagineModelMapping(tt.requested, tt.billed))
		})
	}
}

func TestValidateImagineModelMappingAllowsCompatibleAndUnrelatedModels(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		billed    string
		family    ImagineModelFamily
	}{
		{name: "identity", requested: "grok-imagine-image", billed: "grok-imagine-image", family: ImagineModelFamilyGrokImage},
		{name: "Grok 1.5 identity", requested: "grok-imagine-video-1.5", billed: "grok-imagine-video-1.5", family: ImagineModelFamilyGrokVideo15},
		{name: "ordinary channel mapping", requested: "gpt-4o", billed: "gpt-4.1", family: ImagineModelFamilyUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.family, GetImagineModelFamily(tt.requested))
			assert.NoError(t, ValidateImagineModelMapping(tt.requested, tt.billed))
		})
	}
}
