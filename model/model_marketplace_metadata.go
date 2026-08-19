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
	"quality": {},
}

var allowedMarketplaceOutputFormats = map[string]struct{}{
	"url": {}, "b64_json": {},
}

var allowedMarketplaceReferenceModalities = map[string]struct{}{
	"image": {}, "video": {}, "audio": {},
}

type MarketplaceReadiness struct {
	Category MarketplaceCategory
	Complete bool
	Missing  []string
}

// EvaluateMarketplaceBlockers returns runtime availability blockers in stable
// order. Publication intent is deliberately not part of this calculation.
func EvaluateMarketplaceBlockers(vendorEnabled bool, pricingConfigured bool, groupCount int, endpointCount int) []string {
	blockers := make([]string, 0, 4)
	if !vendorEnabled {
		blockers = append(blockers, "vendor_disabled")
	}
	if !pricingConfigured {
		blockers = append(blockers, "pricing_missing")
	}
	if groupCount <= 0 {
		blockers = append(blockers, "group_unavailable")
	}
	if endpointCount <= 0 {
		blockers = append(blockers, "endpoint_unavailable")
	}
	return blockers
}

// InferMarketplaceCategory derives the marketplace category solely from a
// model's declared capabilities and modalities.
func InferMarketplaceCategory(model *Model) MarketplaceCategory {
	if model == nil {
		return MarketplaceCategoryLLM
	}
	if containsString(model.Capabilities, "video_generation") ||
		containsString(model.OutputModalities, "video") {
		return MarketplaceCategoryVideo
	}
	if containsString(model.Capabilities, "image_generation") ||
		containsString(model.OutputModalities, "image") {
		return MarketplaceCategoryImage
	}
	if containsString(model.Capabilities, "audio_generation") ||
		containsString(model.OutputModalities, "audio") {
		return MarketplaceCategoryAudio
	}
	if containsString(model.Capabilities, "embeddings") {
		return MarketplaceCategoryEmbedding
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

	missing := make(map[string]struct{})
	addMissing := func(field string) {
		missing[field] = struct{}{}
	}
	if strings.TrimSpace(mi.ModelName) == "" {
		addMissing("model_name")
	}
	if strings.TrimSpace(mi.DisplayName) == "" {
		addMissing("display_name")
	}
	if strings.TrimSpace(mi.Description) == "" {
		addMissing("description")
	}
	if mi.VendorID <= 0 {
		addMissing("vendor_id")
	}
	if strings.TrimSpace(mi.ReleaseDate) == "" {
		addMissing("release_date")
	}
	if len(mi.InputModalities) == 0 {
		addMissing("input_modalities")
	}
	if len(mi.OutputModalities) == 0 {
		addMissing("output_modalities")
	}
	if len(mi.Capabilities) == 0 {
		addMissing("capabilities")
	}

	category := InferMarketplaceCategory(mi)
	switch category {
	case MarketplaceCategoryLLM:
		if len(mi.SupportedParameters) == 0 || hasInvalidCatalogValues(mi.SupportedParameters, allowedMarketplaceParameters) {
			addMissing("supported_parameters")
		}
		if mi.ContextLength <= 0 {
			addMissing("context_length")
		}
		if mi.MaxOutputTokens <= 0 {
			addMissing("max_output_tokens")
		}
		if mi.ContextLength > 0 && mi.MaxOutputTokens > mi.ContextLength {
			addMissing("max_output_tokens")
		}
		if !containsString(mi.InputModalities, "text") {
			addMissing("input_modalities")
		}
		if !containsString(mi.OutputModalities, "text") {
			addMissing("output_modalities")
		}
	case MarketplaceCategoryImage:
		appendVisualReadinessMissing(mi, addMissing)
		if len(mi.OutputFormats) == 0 || hasInvalidCatalogValues(mi.OutputFormats, allowedMarketplaceOutputFormats) {
			addMissing("output_formats")
		}
		if !containsString(mi.OutputModalities, "image") {
			addMissing("output_modalities")
		}
		if containsString(mi.Capabilities, "image_editing") && !containsString(mi.InputModalities, "image") {
			addMissing("input_modalities")
		}
		if containsString(mi.Capabilities, "image_editing") && mi.MaxInputImages <= 0 {
			addMissing("max_input_images")
		}
	case MarketplaceCategoryVideo:
		appendVisualReadinessMissing(mi, addMissing)
		if mi.MinDuration <= 0 {
			addMissing("min_duration")
		}
		if mi.MaxDuration <= 0 {
			addMissing("max_duration")
		}
		if mi.MinDuration > 0 && mi.MaxDuration > 0 && mi.MinDuration > mi.MaxDuration {
			addMissing("min_duration")
			addMissing("max_duration")
		}
		if !containsString(mi.OutputModalities, "video") {
			addMissing("output_modalities")
		}
		if hasInvalidReferenceModalities(mi.ReferenceModalities, mi.InputModalities) {
			addMissing("reference_modalities")
		}
	}

	missingFields := make([]string, 0, len(missing))
	for field := range missing {
		missingFields = append(missingFields, field)
	}
	sort.Strings(missingFields)
	return MarketplaceReadiness{
		Category: category,
		Complete: len(missingFields) == 0,
		Missing:  missingFields,
	}
}

func appendVisualReadinessMissing(model *Model, addMissing func(string)) {
	if len(model.SupportedResolutions) == 0 {
		addMissing("supported_resolutions")
	}
	if len(model.SupportedAspectRatios) == 0 {
		addMissing("supported_aspect_ratios")
	}
}

func hasInvalidReferenceModalities(referenceModalities []string, inputModalities []string) bool {
	for _, modality := range referenceModalities {
		if _, ok := allowedMarketplaceReferenceModalities[modality]; !ok || !containsString(inputModalities, modality) {
			return true
		}
	}
	return false
}

func hasInvalidCatalogValues(values []string, allowed map[string]struct{}) bool {
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return true
		}
	}
	return false
}

func normalizeMarketplaceCatalogMetadata(mi *Model) error {
	if mi.ContextLength < 0 {
		return fmt.Errorf("context_length must be non-negative")
	}
	if mi.MaxOutputTokens < 0 {
		return fmt.Errorf("max_output_tokens must be non-negative")
	}
	if InferMarketplaceCategory(mi) == MarketplaceCategoryLLM && mi.ContextLength > 0 && mi.MaxOutputTokens > mi.ContextLength {
		return fmt.Errorf("max_output_tokens must not exceed context_length")
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
	if err := validateCatalogValues("reference_modalities", mi.ReferenceModalities, allowedMarketplaceReferenceModalities); err != nil {
		return err
	}
	if hasInvalidReferenceModalities(mi.ReferenceModalities, mi.InputModalities) {
		return fmt.Errorf("reference_modalities must be a subset of input_modalities")
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
