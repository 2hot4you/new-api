package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		1_000_000, 65_536, "", "2026-02-16", false,
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
	newLocalGrokImage20MarketplaceSeed(),
	{
		ModelName:             "grok-imagine-video",
		DisplayName:           "grok-imagine-video",
		Description:           "异步文生视频、图生视频与视频编辑模型，支持 480p、720p 输出。",
		Tags:                  "video,generation,editing",
		ReleaseDate:           "2026-08-05",
		InputModalities:       []string{"text", "image", "video"},
		OutputModalities:      []string{"video"},
		Capabilities:          []string{"video_generation", "video_editing"},
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
		SupportedResolutions:  []string{"1k", "2k"},
		SupportedAspectRatios: append(localGrokAspectRatios(), "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20", "auto"),
		MaxInputImages:        3,
		OutputFormats:         []string{"url"},
		ReferenceModalities:   []string{"image"},
	}
}

func newLocalGrokImage20MarketplaceSeed() Model {
	seed := newLocalGrokImageMarketplaceSeed(
		"grok-imagine-image-2.0",
		"Grok Imagine 第二代图片生成与编辑模型，支持 Low、Medium 质量档位及 1K、2K 输出。",
	)
	seed.DescriptionEN = "Second-generation Grok Imagine image generation and editing model with Low and Medium quality tiers at 1K and 2K resolutions."
	seed.ReleaseDate = "2026-08-19"
	seed.SupportedParameters = []string{"quality"}
	return seed
}

func localGrokAspectRatios() []string {
	return []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}
}

func ensureModelMarketplaceMetadataSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ensure model marketplace metadata schema: database is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "molii-new-api-20260815-model-marketplace-metadata").Error; err != nil {
			return fmt.Errorf("lock model marketplace metadata schema: %w", err)
		}
		if err := tx.Exec(`
			UPDATE public.models
			SET display_name = COALESCE(display_name, ''),
			    description_en = COALESCE(description_en, ''),
			    marketplace_enabled = COALESCE(marketplace_enabled, false),
			    supported_parameters = COALESCE(supported_parameters, '[]'),
			    supported_resolutions = COALESCE(supported_resolutions, '[]'),
			    supported_aspect_ratios = COALESCE(supported_aspect_ratios, '[]'),
			    max_input_images = COALESCE(max_input_images, 0),
			    output_formats = COALESCE(output_formats, '[]'),
			    min_duration = COALESCE(min_duration, 0),
			    max_duration = COALESCE(max_duration, 0),
			    reference_modalities = COALESCE(reference_modalities, '[]')
		`).Error; err != nil {
			return fmt.Errorf("normalize model marketplace metadata nulls: %w", err)
		}
		if err := tx.Exec(`
			ALTER TABLE public.models
			  ALTER COLUMN display_name SET DEFAULT '',
			  ALTER COLUMN display_name SET NOT NULL,
			  ALTER COLUMN description_en SET DEFAULT '',
			  ALTER COLUMN description_en SET NOT NULL,
			  ALTER COLUMN marketplace_enabled SET DEFAULT false,
			  ALTER COLUMN marketplace_enabled SET NOT NULL,
			  ALTER COLUMN supported_parameters SET DEFAULT '[]',
			  ALTER COLUMN supported_parameters SET NOT NULL,
			  ALTER COLUMN supported_resolutions SET DEFAULT '[]',
			  ALTER COLUMN supported_resolutions SET NOT NULL,
			  ALTER COLUMN supported_aspect_ratios SET DEFAULT '[]',
			  ALTER COLUMN supported_aspect_ratios SET NOT NULL,
			  ALTER COLUMN max_input_images SET DEFAULT 0,
			  ALTER COLUMN max_input_images SET NOT NULL,
			  ALTER COLUMN output_formats SET DEFAULT '[]',
			  ALTER COLUMN output_formats SET NOT NULL,
			  ALTER COLUMN min_duration SET DEFAULT 0,
			  ALTER COLUMN min_duration SET NOT NULL,
			  ALTER COLUMN max_duration SET DEFAULT 0,
			  ALTER COLUMN max_duration SET NOT NULL,
			  ALTER COLUMN reference_modalities SET DEFAULT '[]',
			  ALTER COLUMN reference_modalities SET NOT NULL
		`).Error; err != nil {
			return fmt.Errorf("converge model marketplace metadata constraints: %w", err)
		}
		if err := tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_models_marketplace_enabled_status
			ON public.models (marketplace_enabled, status)
			WHERE deleted_at IS NULL
		`).Error; err != nil {
			return fmt.Errorf("create model marketplace publication index: %w", err)
		}
		return nil
	})
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
			if err := updateLocalMarketplaceFieldsIfEmpty(tx, row.Id, &seed); err != nil {
				return fmt.Errorf("backfill local marketplace metadata for %s: %w", row.ModelName, err)
			}

			var persisted Model
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", row.Id).First(&persisted).Error; err != nil {
				return fmt.Errorf("reload local marketplace metadata for %s: %w", row.ModelName, err)
			}
			if persisted.Status == 1 && !persisted.MarketplaceEnabled && persisted.EvaluateMarketplaceReadiness().Complete {
				if err := tx.Model(&Model{}).
					Where("id = ? AND status = ? AND marketplace_enabled = ?", persisted.Id, 1, false).
					UpdateColumn("marketplace_enabled", true).Error; err != nil {
					return fmt.Errorf("publish local marketplace metadata for %s: %w", row.ModelName, err)
				}
			}
		}
		return nil
	})
}

func updateLocalMarketplaceFieldsIfEmpty(tx *gorm.DB, id int, seed *Model) error {
	stringFields := []struct {
		column string
		value  string
	}{
		{"display_name", seed.DisplayName},
		{"description", seed.Description},
		{"description_en", seed.DescriptionEN},
		{"icon", seed.Icon},
		{"tags", seed.Tags},
		{"knowledge_cutoff", seed.KnowledgeCutoff},
		{"release_date", seed.ReleaseDate},
	}
	for _, field := range stringFields {
		if strings.TrimSpace(field.value) == "" {
			continue
		}
		emptyCondition := fmt.Sprintf("TRIM(COALESCE(%s, '')) = ''", field.column)
		if err := tx.Model(&Model{}).Where("id = ?", id).Where(emptyCondition).UpdateColumn(field.column, field.value).Error; err != nil {
			return fmt.Errorf("fill %s: %w", field.column, err)
		}
	}

	integerFields := []struct {
		column string
		value  int
	}{
		{"context_length", seed.ContextLength},
		{"max_output_tokens", seed.MaxOutputTokens},
		{"max_input_images", seed.MaxInputImages},
		{"min_duration", seed.MinDuration},
		{"max_duration", seed.MaxDuration},
	}
	for _, field := range integerFields {
		if field.value == 0 {
			continue
		}
		emptyCondition := fmt.Sprintf("COALESCE(%s, 0) = 0", field.column)
		if err := tx.Model(&Model{}).Where("id = ?", id).Where(emptyCondition).UpdateColumn(field.column, field.value).Error; err != nil {
			return fmt.Errorf("fill %s: %w", field.column, err)
		}
	}

	arrayFields := []struct {
		column string
		value  []string
	}{
		{"input_modalities", seed.InputModalities},
		{"output_modalities", seed.OutputModalities},
		{"capabilities", seed.Capabilities},
		{"supported_parameters", seed.SupportedParameters},
		{"supported_resolutions", seed.SupportedResolutions},
		{"supported_aspect_ratios", seed.SupportedAspectRatios},
		{"output_formats", seed.OutputFormats},
		{"reference_modalities", seed.ReferenceModalities},
	}
	for _, field := range arrayFields {
		if len(field.value) == 0 {
			continue
		}
		encoded, err := json.Marshal(field.value)
		if err != nil {
			return fmt.Errorf("encode %s: %w", field.column, err)
		}
		emptyCondition := fmt.Sprintf("%s IS NULL OR TRIM(%s) IN ('', '[]', 'null')", field.column, field.column)
		if err := tx.Model(&Model{}).Where("id = ?", id).Where(emptyCondition).UpdateColumn(field.column, string(encoded)).Error; err != nil {
			return fmt.Errorf("fill %s: %w", field.column, err)
		}
	}
	return nil
}
