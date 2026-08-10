package constant

import (
	"fmt"
	"strings"
)

// ImagineModelFamily groups Imagine models that have the same upstream
// capability and price contract. Mappings may only stay within one family.
type ImagineModelFamily string

const (
	ImagineModelFamilyUnknown          ImagineModelFamily = ""
	ImagineModelFamilySeedanceStandard ImagineModelFamily = "seedance_standard"
	ImagineModelFamilySeedanceFast     ImagineModelFamily = "seedance_fast"
	ImagineModelFamilyGrokImage        ImagineModelFamily = "grok_image"
	ImagineModelFamilyGrokImageQuality ImagineModelFamily = "grok_image_quality"
	ImagineModelFamilyGrokVideoLegacy  ImagineModelFamily = "grok_video_legacy"
	ImagineModelFamilyGrokVideo15      ImagineModelFamily = "grok_video_15"
)

// GetImagineModelFamily returns the capability and pricing family for the
// Molii Imagine models that require mapping restrictions.
func GetImagineModelFamily(modelName string) ImagineModelFamily {
	switch strings.TrimSpace(modelName) {
	case "doubao-seedance-2-0-260128":
		return ImagineModelFamilySeedanceStandard
	case "doubao-seedance-2-0-fast-260128":
		return ImagineModelFamilySeedanceFast
	case "grok-imagine-image":
		return ImagineModelFamilyGrokImage
	case "grok-imagine-image-quality":
		return ImagineModelFamilyGrokImageQuality
	case "grok-imagine-video":
		return ImagineModelFamilyGrokVideoLegacy
	case "grok-imagine-video-1.5":
		return ImagineModelFamilyGrokVideo15
	default:
		return ImagineModelFamilyUnknown
	}
}

// ValidateImagineModelMapping prevents a known Imagine model from being
// redirected to a model with a different capability or price contract.
// Mappings unrelated to Imagine remain unrestricted.
func ValidateImagineModelMapping(requestedModel, billedModel string) error {
	requestedFamily := GetImagineModelFamily(requestedModel)
	billedFamily := GetImagineModelFamily(billedModel)
	if requestedFamily == ImagineModelFamilyUnknown && billedFamily == ImagineModelFamilyUnknown {
		return nil
	}
	if requestedFamily != ImagineModelFamilyUnknown && requestedFamily == billedFamily {
		return nil
	}
	return fmt.Errorf("incompatible Imagine model mapping: %s -> %s", requestedModel, billedModel)
}
