package model

import (
	"fmt"
	"sort"
	"strings"
)

type MarketplaceCategory string

const (
	MarketplaceCategoryLLM       MarketplaceCategory = "llm"
	MarketplaceCategoryImage     MarketplaceCategory = "image"
	MarketplaceCategoryVideo     MarketplaceCategory = "video"
	MarketplaceCategoryAudio     MarketplaceCategory = "audio"
	MarketplaceCategoryEmbedding MarketplaceCategory = "embedding"
)

var allowedMarketplaceParameters = map[string]struct{}{
	"stream": {}, "temperature": {}, "top_p": {}, "max_tokens": {},
	"tools": {}, "tool_choice": {}, "reasoning_effort": {}, "response_format": {},
}

var allowedMarketplaceOutputFormats = map[string]struct{}{
	"url": {}, "b64_json": {},
}

type MarketplaceReadiness struct {
	Category MarketplaceCategory
	Complete bool
	Missing  []string
}

// InferMarketplaceCategory derives the marketplace category solely from a
// model's declared capabilities and modalities.
func InferMarketplaceCategory(model *Model) MarketplaceCategory {
	if model == nil {
		return MarketplaceCategoryLLM
	}
	if containsString(model.Capabilities, "embeddings") {
		return MarketplaceCategoryEmbedding
	}
	if containsString(model.Capabilities, "video_generation") ||
		containsString(model.Capabilities, "video_editing") ||
		containsString(model.OutputModalities, "video") {
		return MarketplaceCategoryVideo
	}
	if containsString(model.Capabilities, "image_generation") ||
		containsString(model.Capabilities, "image_editing") ||
		containsString(model.OutputModalities, "image") {
		return MarketplaceCategoryImage
	}
	if containsString(model.Capabilities, "audio_generation") ||
		containsString(model.OutputModalities, "audio") {
		return MarketplaceCategoryAudio
	}
	return MarketplaceCategoryLLM
}

// EvaluateMarketplaceReadiness returns all missing marketplace metadata without
// mutating the model or querying the database.
func (mi *Model) EvaluateMarketplaceReadiness() MarketplaceReadiness {
	if mi == nil {
		return MarketplaceReadiness{
			Category: MarketplaceCategoryLLM,
			Missing:  []string{"model"},
		}
	}

	missing := make([]string, 0)
	if strings.TrimSpace(mi.ModelName) == "" {
		missing = append(missing, "model_name")
	}
	if strings.TrimSpace(mi.DisplayName) == "" {
		missing = append(missing, "display_name")
	}
	if strings.TrimSpace(mi.Description) == "" {
		missing = append(missing, "description")
	}
	if mi.VendorID <= 0 {
		missing = append(missing, "vendor_id")
	}
	if strings.TrimSpace(mi.ReleaseDate) == "" {
		missing = append(missing, "release_date")
	}
	if len(mi.InputModalities) == 0 {
		missing = append(missing, "input_modalities")
	}
	if len(mi.OutputModalities) == 0 {
		missing = append(missing, "output_modalities")
	}
	if len(mi.Capabilities) == 0 {
		missing = append(missing, "capabilities")
	}

	category := InferMarketplaceCategory(mi)
	switch category {
	case MarketplaceCategoryLLM:
		if len(mi.SupportedParameters) == 0 {
			missing = append(missing, "supported_parameters")
		}
		if mi.ContextLength <= 0 {
			missing = append(missing, "context_length")
		}
		if mi.MaxOutputTokens <= 0 {
			missing = append(missing, "max_output_tokens")
		}
	case MarketplaceCategoryImage:
		appendImageReadinessMissing(mi, &missing)
	case MarketplaceCategoryVideo:
		appendImageReadinessMissing(mi, &missing)
		if mi.MinDuration <= 0 {
			missing = append(missing, "min_duration")
		}
		if mi.MaxDuration <= 0 {
			missing = append(missing, "max_duration")
		}
		if len(mi.ReferenceModalities) == 0 {
			missing = append(missing, "reference_modalities")
		}
	}

	sort.Strings(missing)
	return MarketplaceReadiness{
		Category: category,
		Complete: len(missing) == 0,
		Missing:  missing,
	}
}

func appendImageReadinessMissing(model *Model, missing *[]string) {
	if len(model.SupportedResolutions) == 0 {
		*missing = append(*missing, "supported_resolutions")
	}
	if len(model.SupportedAspectRatios) == 0 {
		*missing = append(*missing, "supported_aspect_ratios")
	}
	if len(model.OutputFormats) == 0 {
		*missing = append(*missing, "output_formats")
	}
	if containsString(model.Capabilities, "image_editing") && model.MaxInputImages <= 0 {
		*missing = append(*missing, "max_input_images")
	}
}

func normalizeMarketplaceCatalogMetadata(mi *Model) error {
	if mi.ContextLength < 0 {
		return fmt.Errorf("context_length must be non-negative")
	}
	if mi.MaxOutputTokens < 0 {
		return fmt.Errorf("max_output_tokens must be non-negative")
	}
	if mi.MaxInputImages < 0 {
		return fmt.Errorf("max_input_images must be non-negative")
	}
	if mi.MinDuration < 0 {
		return fmt.Errorf("min_duration must be non-negative")
	}
	if mi.MaxDuration < 0 {
		return fmt.Errorf("max_duration must be non-negative")
	}
	if mi.MinDuration > mi.MaxDuration && mi.MaxDuration != 0 {
		return fmt.Errorf("min_duration must not exceed max_duration")
	}

	mi.DisplayName = strings.TrimSpace(mi.DisplayName)
	mi.Description = strings.TrimSpace(mi.Description)
	mi.DescriptionEN = strings.TrimSpace(mi.DescriptionEN)
	mi.SupportedParameters = normalizeLookupValues(mi.SupportedParameters)
	mi.SupportedResolutions = normalizeLookupValues(mi.SupportedResolutions)
	mi.SupportedAspectRatios = normalizeLookupValues(mi.SupportedAspectRatios)
	mi.OutputFormats = normalizeLookupValues(mi.OutputFormats)
	mi.ReferenceModalities = normalizeLookupValues(mi.ReferenceModalities)

	if err := validateCatalogValues("supported_parameters", mi.SupportedParameters, allowedMarketplaceParameters); err != nil {
		return err
	}
	if err := validateCatalogValues("output_formats", mi.OutputFormats, allowedMarketplaceOutputFormats); err != nil {
		return err
	}
	for _, modality := range mi.ReferenceModalities {
		if !containsString(mi.InputModalities, modality) {
			return fmt.Errorf("reference_modalities must be a subset of input_modalities")
		}
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
