package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func marketplaceBaseModel() Model {
	return Model{
		ModelName:        "qwen-test",
		DisplayName:      " Qwen Test ",
		Description:      " 中文简介 ",
		VendorID:         1,
		ReleaseDate:      " 2026-08-15 ",
		InputModalities:  []string{"text"},
		OutputModalities: []string{"text"},
		Capabilities:     []string{"streaming"},
		SupportedParameters: []string{
			"stream",
		},
		ContextLength:   131072,
		MaxOutputTokens: 8192,
	}
}

func TestEvaluateMarketplaceReadinessByCategory(t *testing.T) {
	tests := []struct {
		name     string
		model    Model
		category MarketplaceCategory
		complete bool
		missing  []string
	}{
		{
			name:     "complete LLM",
			model:    marketplaceBaseModel(),
			category: MarketplaceCategoryLLM,
			complete: true,
		},
		{
			name: "incomplete LLM",
			model: Model{
				ModelName:        "incomplete-llm",
				InputModalities:  []string{"text"},
				OutputModalities: []string{"text"},
			},
			category: MarketplaceCategoryLLM,
			missing: []string{
				"capabilities", "context_length", "description", "display_name", "max_output_tokens", "release_date", "supported_parameters", "vendor_id",
			},
		},
		{
			name: "complete image model",
			model: func() Model {
				model := marketplaceBaseModel()
				model.InputModalities = []string{"text"}
				model.OutputModalities = []string{"image"}
				model.Capabilities = []string{"image_generation"}
				model.SupportedResolutions = []string{"1024x1024"}
				model.SupportedAspectRatios = []string{"1:1"}
				model.OutputFormats = []string{"url"}
				return model
			}(),
			category: MarketplaceCategoryImage,
			complete: true,
		},
		{
			name: "image editing requires max input images",
			model: func() Model {
				model := marketplaceBaseModel()
				model.InputModalities = []string{"text", "image"}
				model.OutputModalities = []string{"image"}
				model.Capabilities = []string{"image_editing"}
				model.SupportedResolutions = []string{"1024x1024"}
				model.SupportedAspectRatios = []string{"1:1"}
				model.OutputFormats = []string{"b64_json"}
				return model
			}(),
			category: MarketplaceCategoryImage,
			missing:  []string{"max_input_images"},
		},
		{
			name: "complete video model",
			model: func() Model {
				model := marketplaceBaseModel()
				model.InputModalities = []string{"text", "image"}
				model.OutputModalities = []string{"video"}
				model.Capabilities = []string{"video_generation"}
				model.SupportedResolutions = []string{"720p"}
				model.SupportedAspectRatios = []string{"16:9"}
				model.OutputFormats = []string{"url"}
				model.MinDuration = 5
				model.MaxDuration = 10
				model.ReferenceModalities = []string{"image"}
				return model
			}(),
			category: MarketplaceCategoryVideo,
			complete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.model.EvaluateMarketplaceReadiness()
			require.Equal(t, tt.category, got.Category)
			require.Equal(t, tt.complete, got.Complete)
			if tt.complete {
				require.Empty(t, got.Missing)
			} else {
				require.Equal(t, tt.missing, got.Missing)
			}
		})
	}
}

func TestNormalizeMarketplaceMetadata(t *testing.T) {
	t.Run("normalizes duplicate arrays", func(t *testing.T) {
		model := marketplaceBaseModel()
		model.InputModalities = []string{"text", "image"}
		model.SupportedParameters = []string{" stream ", "temperature", "stream", ""}
		model.SupportedResolutions = []string{" 1024x1024 ", "1024x1024", ""}
		model.SupportedAspectRatios = []string{" 1:1 ", "1:1", ""}
		model.OutputFormats = []string{" url ", "url", ""}
		model.ReferenceModalities = []string{" image ", "image", ""}

		require.NoError(t, model.NormalizeCatalogMetadata())
		require.Equal(t, "Qwen Test", model.DisplayName)
		require.Equal(t, "中文简介", model.Description)
		require.Equal(t, "2026-08-15", model.ReleaseDate)
		require.Equal(t, []string{"stream", "temperature"}, model.SupportedParameters)
		require.Equal(t, []string{"1024x1024"}, model.SupportedResolutions)
		require.Equal(t, []string{"1:1"}, model.SupportedAspectRatios)
		require.Equal(t, []string{"url"}, model.OutputFormats)
		require.Equal(t, []string{"image"}, model.ReferenceModalities)
	})

	tests := []struct {
		name  string
		model Model
	}{
		{
			name:  "rejects invalid duration range",
			model: Model{MinDuration: 10, MaxDuration: 5},
		},
		{
			name: "rejects invalid reference modality",
			model: Model{
				InputModalities:     []string{"text"},
				ReferenceModalities: []string{"image"},
			},
		},
		{
			name:  "rejects unknown controlled parameter",
			model: Model{SupportedParameters: []string{"invalid"}},
		},
		{
			name:  "rejects unknown output format",
			model: Model{OutputFormats: []string{"data_uri"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, tt.model.NormalizeCatalogMetadata())
		})
	}
}

func TestMarketplaceMetadataConditionalRules(t *testing.T) {
	t.Run("category priority follows video image audio embedding", func(t *testing.T) {
		model := marketplaceBaseModel()
		model.OutputModalities = []string{"video"}
		model.Capabilities = []string{"embeddings", "video_generation"}
		require.Equal(t, MarketplaceCategoryVideo, InferMarketplaceCategory(&model))
	})

	t.Run("LLM requires text modalities and bounded output tokens", func(t *testing.T) {
		model := marketplaceBaseModel()
		model.InputModalities = []string{"image"}
		model.OutputModalities = []string{"file"}
		model.Capabilities = []string{"streaming"}
		model.MaxOutputTokens = model.ContextLength + 1

		got := model.EvaluateMarketplaceReadiness()
		require.False(t, got.Complete)
		require.Equal(t, []string{"input_modalities", "max_output_tokens", "output_modalities"}, got.Missing)
	})

	t.Run("image editing requires image input and image output", func(t *testing.T) {
		model := marketplaceBaseModel()
		model.Capabilities = []string{"image_generation", "image_editing"}
		model.SupportedResolutions = []string{"1024x1024"}
		model.SupportedAspectRatios = []string{"1:1"}
		model.OutputFormats = []string{"url"}
		model.MaxInputImages = 1

		got := model.EvaluateMarketplaceReadiness()
		require.False(t, got.Complete)
		require.Equal(t, []string{"input_modalities", "output_modalities"}, got.Missing)
	})

	t.Run("video does not require image output formats or reference inputs", func(t *testing.T) {
		model := marketplaceBaseModel()
		model.OutputModalities = []string{"video"}
		model.Capabilities = []string{"video_generation"}
		model.SupportedResolutions = []string{"720p"}
		model.SupportedAspectRatios = []string{"16:9"}
		model.MinDuration = 5
		model.MaxDuration = 10

		got := model.EvaluateMarketplaceReadiness()
		require.True(t, got.Complete)
		require.Empty(t, got.Missing)
	})

	t.Run("video reports invalid duration range and reference modality", func(t *testing.T) {
		model := marketplaceBaseModel()
		model.InputModalities = []string{"text", "file"}
		model.OutputModalities = []string{"video"}
		model.Capabilities = []string{"video_generation"}
		model.SupportedResolutions = []string{"720p"}
		model.SupportedAspectRatios = []string{"16:9"}
		model.MinDuration = 10
		model.MaxDuration = 5
		model.ReferenceModalities = []string{"file"}

		got := model.EvaluateMarketplaceReadiness()
		require.False(t, got.Complete)
		require.Equal(t, []string{"max_duration", "min_duration", "reference_modalities"}, got.Missing)
		require.Error(t, model.NormalizeCatalogMetadata())
	})

	t.Run("normalization rejects unsupported reference modality", func(t *testing.T) {
		model := marketplaceBaseModel()
		model.InputModalities = []string{"text", "file"}
		model.ReferenceModalities = []string{"file"}
		require.ErrorContains(t, model.NormalizeCatalogMetadata(), "reference_modalities contains unsupported value")
	})
}
