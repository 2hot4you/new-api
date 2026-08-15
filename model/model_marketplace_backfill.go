package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// localMarketplaceMetadataSeeds20260815 is migration-only source data. It is
// intentionally independent from runtime catalog profiles so persisted rows
// remain authoritative after the local compatibility backfill has run.
var localMarketplaceMetadataSeeds20260815 = []Model{
	newLocalLLMMarketplaceSeed(
		"deepseek-v4-flash-202605",
		"面向高效对话、推理、编程与 Agent 工作流的长上下文模型；支持通过 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 384_000, "", "2026-05-01", true,
	),
	newLocalLLMMarketplaceSeed(
		"deepseek-v4-pro-202606",
		"面向复杂推理、编程与 Agent 工作流的旗舰长上下文模型；支持通过 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 384_000, "", "2026-06-01", true,
	),
	newLocalLLMMarketplaceSeed(
		"glm-5.2",
		"面向长程编程与复杂推理的旗舰模型，支持长上下文、深度思考和工具调用；可通过 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 131_072, "", "2026-06-13", true,
	),
	newLocalLLMMarketplaceSeed(
		"kimi-k3",
		"面向通用对话、内容生成与复杂问答的文本模型；支持通过 OpenAI 兼容 Chat Completions API 调用。",
		1_048_576, 131_072, "", "2026-07-16", true,
	),
	newLocalLLMMarketplaceSeed(
		"minimax-m3",
		"面向长上下文编程与 Agent 工作流的模型；当前通过本网关的 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 128_000, "", "2026-06-01", false,
	),
	newLocalLLMMarketplaceSeed(
		"qwen3.5-flash",
		"面向低成本长上下文对话的模型；当前通过本网关的 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 65_536, "", "2026-02-23", true,
	),
	newLocalLLMMarketplaceSeed(
		"qwen3.5-plus",
		"面向更高能力长上下文对话的模型；当前通过本网关的 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 65_536, "2025-04", "2026-02-16", false,
	),
	{
		ModelName:             "doubao-seedance-2-0-260128",
		DisplayName:           "doubao-seedance-2-0-260128",
		Description:           "新一代多模态视频生成模型，支持文本、图片和视频参考，擅长复杂运动、真实物理、精准控制及同步音频生成。",
		Tags:                  "video,multimodal",
		ReleaseDate:           "2026-01-28",
		InputModalities:       []string{"text", "image", "video", "audio"},
		OutputModalities:      []string{"video", "audio"},
		Capabilities:          []string{"video_generation", "video_editing", "audio_generation", "web_search"},
		MetadataSource:        "Molii local API",
		MetadataVerifiedAt:    "2026-08-13",
		SupportedResolutions:  []string{"480p", "720p", "1080p", "4k"},
		SupportedAspectRatios: []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"},
		MaxInputImages:        9,
		OutputFormats:         []string{"url"},
		MinDuration:           4,
		MaxDuration:           15,
		ReferenceModalities:   []string{"image", "video", "audio"},
	},
	{
		ModelName:             "doubao-seedance-2-0-fast-260128",
		DisplayName:           "doubao-seedance-2-0-fast-260128",
		Description:           "Seedance 2.0 加速版，支持文本、图片和视频参考及同步音频生成，面向低延迟创意迭代，支持 480p/720p。",
		Tags:                  "video,multimodal",
		ReleaseDate:           "2026-01-28",
		InputModalities:       []string{"text", "image", "video", "audio"},
		OutputModalities:      []string{"video", "audio"},
		Capabilities:          []string{"video_generation", "video_editing", "audio_generation", "web_search"},
		MetadataSource:        "Molii local API",
		MetadataVerifiedAt:    "2026-08-13",
		SupportedResolutions:  []string{"480p", "720p"},
		SupportedAspectRatios: []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"},
		MaxInputImages:        9,
		OutputFormats:         []string{"url"},
		MinDuration:           4,
		MaxDuration:           15,
		ReferenceModalities:   []string{"image", "video", "audio"},
	},
	newLocalGrokImageMarketplaceSeed("grok-imagine-image", "支持文本生成图片与参考图编辑，提供 1K、2K 输出，生成速度更快。"),
	newLocalGrokImageMarketplaceSeed("grok-imagine-image-quality", "更注重画面细节和质量的图片生成与编辑模型，支持 1K、2K 输出。"),
	{
		ModelName:             "grok-imagine-video",
		DisplayName:           "grok-imagine-video",
		Description:           "异步文生视频、图生视频与视频编辑模型，支持 480p、720p 输出。",
		Tags:                  "video,generation,editing",
		ReleaseDate:           "2026-08-05",
		InputModalities:       []string{"text", "image", "video"},
		OutputModalities:      []string{"video"},
		Capabilities:          []string{"video_generation", "video_editing"},
		MetadataSource:        "Molii local API",
		MetadataVerifiedAt:    "2026-08-13",
		SupportedResolutions:  []string{"480p", "720p"},
		SupportedAspectRatios: localGrokAspectRatios(),
		MaxInputImages:        1,
		OutputFormats:         []string{"url"},
		MinDuration:           1,
		MaxDuration:           15,
		ReferenceModalities:   []string{"image", "video"},
	},
	{
		ModelName:             "grok-imagine-video-1.5",
		DisplayName:           "grok-imagine-video-1.5",
		Description:           "异步图生视频模型，支持 480p、720p、1080p 输出。",
		Tags:                  "video,generation",
		ReleaseDate:           "2026-08-05",
		InputModalities:       []string{"text", "image"},
		OutputModalities:      []string{"video"},
		Capabilities:          []string{"video_generation"},
		MetadataSource:        "Molii local API",
		MetadataVerifiedAt:    "2026-08-13",
		SupportedResolutions:  []string{"480p", "720p", "1080p"},
		SupportedAspectRatios: localGrokAspectRatios(),
		MaxInputImages:        7,
		OutputFormats:         []string{"url"},
		MinDuration:           1,
		MaxDuration:           15,
		ReferenceModalities:   []string{"image"},
	},
}

func newLocalLLMMarketplaceSeed(modelName, description string, contextLength, maxOutputTokens int, knowledgeCutoff, releaseDate string, structuredOutput bool) Model {
	capabilities := []string{"streaming", "system_prompt", "reasoning", "tools"}
	supportedParameters := []string{"stream", "tools", "tool_choice", "reasoning_effort"}
	if structuredOutput {
		capabilities = append(capabilities, "structured_output")
		supportedParameters = append(supportedParameters, "response_format")
	}
	return Model{
		ModelName:           modelName,
		DisplayName:         modelName,
		Description:         description,
		Tags:                "text,chat,context",
		ContextLength:       contextLength,
		MaxOutputTokens:     maxOutputTokens,
		KnowledgeCutoff:     knowledgeCutoff,
		ReleaseDate:         releaseDate,
		InputModalities:     []string{"text"},
		OutputModalities:    []string{"text"},
		Capabilities:        capabilities,
		MetadataSource:      "models.dev",
		MetadataVerifiedAt:  "2026-08-13",
		SupportedParameters: supportedParameters,
	}
}

func newLocalGrokImageMarketplaceSeed(modelName, description string) Model {
	return Model{
		ModelName:             modelName,
		DisplayName:           modelName,
		Description:           description,
		Tags:                  "image,generation,editing",
		ReleaseDate:           "2026-08-05",
		InputModalities:       []string{"text", "image"},
		OutputModalities:      []string{"image"},
		Capabilities:          []string{"image_generation", "image_editing"},
		MetadataSource:        "Molii local API",
		MetadataVerifiedAt:    "2026-08-13",
		SupportedResolutions:  []string{"1k", "2k"},
		SupportedAspectRatios: append(localGrokAspectRatios(), "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20", "auto"),
		MaxInputImages:        3,
		OutputFormats:         []string{"url"},
		ReferenceModalities:   []string{"image"},
	}
}

func localGrokAspectRatios() []string {
	return []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}
}

// BackfillLocalMarketplaceMetadata migrates reviewed local catalog facts into
// existing Model rows. It never creates models and never overwrites a non-empty
// administrator value.
func BackfillLocalMarketplaceMetadata(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("backfill local marketplace metadata: database is nil")
	}

	modelNames := make([]string, 0, len(localMarketplaceMetadataSeeds20260815))
	for _, seed := range localMarketplaceMetadataSeeds20260815 {
		modelNames = append(modelNames, seed.ModelName)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var rows []Model
		if err := tx.Where("model_name IN ?", modelNames).Find(&rows).Error; err != nil {
			return fmt.Errorf("load local marketplace models: %w", err)
		}
		rowsByName := make(map[string]*Model, len(rows))
		for index := range rows {
			rowsByName[rows[index].ModelName] = &rows[index]
		}

		for _, sourceSeed := range localMarketplaceMetadataSeeds20260815 {
			row := rowsByName[sourceSeed.ModelName]
			if row == nil {
				continue
			}
			seed := sourceSeed
			if err := seed.NormalizeCatalogMetadata(); err != nil {
				return fmt.Errorf("normalize local marketplace metadata for %s: %w", seed.ModelName, err)
			}

			updates := make(map[string]interface{})
			applyLocalMarketplaceSeed(row, &seed, updates)
			if row.Status == 1 && !row.MarketplaceEnabled && row.EvaluateMarketplaceReadiness().Complete {
				row.MarketplaceEnabled = true
				updates["marketplace_enabled"] = true
			}
			if len(updates) == 0 {
				continue
			}
			if err := tx.Model(&Model{}).Where("id = ?", row.Id).Updates(updates).Error; err != nil {
				return fmt.Errorf("backfill local marketplace metadata for %s: %w", row.ModelName, err)
			}
		}
		return nil
	})
}

func applyLocalMarketplaceSeed(row, seed *Model, updates map[string]interface{}) {
	fillLocalMarketplaceString(updates, "display_name", &row.DisplayName, seed.DisplayName)
	fillLocalMarketplaceString(updates, "description", &row.Description, seed.Description)
	fillLocalMarketplaceString(updates, "description_en", &row.DescriptionEN, seed.DescriptionEN)
	fillLocalMarketplaceString(updates, "icon", &row.Icon, seed.Icon)
	fillLocalMarketplaceString(updates, "tags", &row.Tags, seed.Tags)
	fillLocalMarketplaceInt(updates, "context_length", &row.ContextLength, seed.ContextLength)
	fillLocalMarketplaceInt(updates, "max_output_tokens", &row.MaxOutputTokens, seed.MaxOutputTokens)
	fillLocalMarketplaceString(updates, "knowledge_cutoff", &row.KnowledgeCutoff, seed.KnowledgeCutoff)
	fillLocalMarketplaceString(updates, "release_date", &row.ReleaseDate, seed.ReleaseDate)
	fillLocalMarketplaceStrings(updates, "input_modalities", &row.InputModalities, seed.InputModalities)
	fillLocalMarketplaceStrings(updates, "output_modalities", &row.OutputModalities, seed.OutputModalities)
	fillLocalMarketplaceStrings(updates, "capabilities", &row.Capabilities, seed.Capabilities)
	fillLocalMarketplaceString(updates, "metadata_source", &row.MetadataSource, seed.MetadataSource)
	fillLocalMarketplaceString(updates, "metadata_verified_at", &row.MetadataVerifiedAt, seed.MetadataVerifiedAt)
	fillLocalMarketplaceStrings(updates, "supported_parameters", &row.SupportedParameters, seed.SupportedParameters)
	fillLocalMarketplaceStrings(updates, "supported_resolutions", &row.SupportedResolutions, seed.SupportedResolutions)
	fillLocalMarketplaceStrings(updates, "supported_aspect_ratios", &row.SupportedAspectRatios, seed.SupportedAspectRatios)
	fillLocalMarketplaceInt(updates, "max_input_images", &row.MaxInputImages, seed.MaxInputImages)
	fillLocalMarketplaceStrings(updates, "output_formats", &row.OutputFormats, seed.OutputFormats)
	fillLocalMarketplaceInt(updates, "min_duration", &row.MinDuration, seed.MinDuration)
	fillLocalMarketplaceInt(updates, "max_duration", &row.MaxDuration, seed.MaxDuration)
	fillLocalMarketplaceStrings(updates, "reference_modalities", &row.ReferenceModalities, seed.ReferenceModalities)
}

func fillLocalMarketplaceString(updates map[string]interface{}, column string, current *string, seed string) {
	if strings.TrimSpace(*current) != "" || strings.TrimSpace(seed) == "" {
		return
	}
	*current = seed
	updates[column] = seed
}

func fillLocalMarketplaceInt(updates map[string]interface{}, column string, current *int, seed int) {
	if *current != 0 || seed == 0 {
		return
	}
	*current = seed
	updates[column] = seed
}

func fillLocalMarketplaceStrings(updates map[string]interface{}, column string, current *[]string, seed []string) {
	if len(*current) != 0 || len(seed) == 0 {
		return
	}
	value := append([]string(nil), seed...)
	*current = value
	encoded, _ := json.Marshal(value)
	updates[column] = string(encoded)
}
