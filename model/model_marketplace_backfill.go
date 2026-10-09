package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// localMarketplaceMetadataSeeds is migration-only source data. It is
// intentionally independent from runtime catalog profiles so persisted rows
// remain authoritative after the local compatibility backfill has run.
var localMarketplaceMetadataSeeds = []Model{
	{
		ModelName:           "deepseek-flash",
		DisplayName:         "DeepSeek V4.1 Flash",
		Description:         "DeepSeek V4.1 Flash 高效多模态模型，面向对话、推理、编程和 Agent 工作流，支持文本与图像输入、思考模式及工具调用。",
		DescriptionEN:       "DeepSeek V4.1 Flash is an efficient multimodal model for chat, reasoning, coding, and agent workflows with text and image input, thinking, and tool use.",
		Icon:                "DeepSeek.Color",
		Tags:                "text,chat,reasoning,vision,tools",
		ContextLength:       1_000_000,
		MaxOutputTokens:     384_000,
		ReleaseDate:         "2026-09-10",
		InputModalities:     []string{"text", "image"},
		OutputModalities:    []string{"text"},
		Capabilities:        []string{"function_calling", "streaming", "vision", "json_mode", "reasoning", "tools", "system_prompt", "caching"},
		SupportedParameters: []string{"stream", "max_tokens", "tools", "tool_choice", "reasoning_effort", "response_format"},
		MetadataSource:      "https://api-docs.deepseek.com/zh-cn/news/news260910/",
		MetadataVerifiedAt:  "2026-09-29",
	},
	{
		ModelName:           "gemini-3.8-flash",
		DisplayName:         "Gemini 3.8 Flash",
		Description:         "Google 面向长程软件工程、自主 Agent 和复杂企业工作流的高效多模态模型，支持百万级上下文与内置工具。",
		DescriptionEN:       "Google's efficient multimodal model for long-horizon software engineering, autonomous agents, and complex enterprise workflows with a one-million-token context and built-in tools.",
		Icon:                "Gemini.Color",
		Tags:                "text,chat,reasoning,multimodal,tools",
		ContextLength:       1_048_576,
		MaxOutputTokens:     65_536,
		KnowledgeCutoff:     "2025-01",
		ReleaseDate:         "2026-09-02",
		InputModalities:     []string{"text", "image", "video", "audio", "file"},
		OutputModalities:    []string{"text"},
		Capabilities:        []string{"function_calling", "streaming", "vision", "structured_output", "reasoning", "tools", "system_prompt", "web_search", "code_interpreter", "caching"},
		SupportedParameters: []string{"stream", "max_tokens", "tools", "tool_choice", "reasoning_effort", "response_format"},
		MetadataSource:      "https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash",
		MetadataVerifiedAt:  "2026-09-29",
	},
	newAnthropic55MarketplaceSeed(
		"claude-sonnet-5-5", "Claude Sonnet 5.5",
		"Anthropic 兼顾智能、速度与成本的先进模型，适合复杂编码、Agent 和知识工作，并支持自适应思考。",
		"Anthropic's advanced balance of intelligence, speed, and cost for complex coding, agentic, and knowledge work with adaptive thinking.",
		"2026-09-28", "https://platform.claude.com/docs/en/models/sonnet-5-5/overview",
	),
	newAnthropic55MarketplaceSeed(
		"claude-opus-5-5", "Claude Opus 5.5",
		"Anthropic 面向长时间运行的 Agent、复杂编码和高难度知识工作的旗舰模型，默认采用自适应思考。",
		"Anthropic's flagship model for long-running agents, complex coding, and demanding knowledge work with adaptive thinking enabled by default.",
		"2026-09-22", "https://platform.claude.com/docs/en/models/opus-5-5/overview",
	),
	newAnthropic55MarketplaceSeed(
		"claude-fable-5-1", "Claude Fable 5.1",
		"Anthropic 面向高难度推理与长程 Agent 工作流的模型，支持百万级上下文、视觉和工具调用。",
		"Anthropic model for demanding reasoning and long-horizon agent workflows with a one-million-token context, vision, and tool use.",
		"2026-09-01", "https://platform.claude.com/docs/en/models/fable-5-1/overview",
	),
	newGPT6MarketplaceSeed(
		"gpt-6-sol", "GPT-6 Sol",
		"OpenAI 面向复杂编码、推理和 Agent 工作流的高能力模型，支持百万级上下文、视觉和丰富工具。",
		"OpenAI high-capability model for complex coding, reasoning, and agent workflows with a one-million-token context, vision, and rich tool support.",
		"2026-04-20", "2026-09-22", "https://developers.openai.com/api/docs/models/gpt-6-sol",
	),
	newGPT6MarketplaceSeed(
		"gpt-6-luna", "GPT-6 Luna",
		"OpenAI 面向高吞吐、聚焦任务的高效模型，在速度和成本之间取得平衡，并支持视觉与工具调用。",
		"OpenAI efficient model for high-volume focused tasks, balancing speed and cost with vision and tool support.",
		"2026-05-18", "2026-09-22", "https://developers.openai.com/api/docs/models/gpt-6-luna",
	),
	newGPT6MarketplaceSeed(
		"gpt-6-astra", "GPT-6 Astra",
		"OpenAI 面向最困难端到端工作的旗舰模型，适合深度推理、编码、计算机操作、研究和文档任务。",
		"OpenAI flagship model for the hardest end-to-end work across deep reasoning, coding, computer use, research, and documents.",
		"2026-04-30", "2026-09-03", "https://developers.openai.com/api/docs/models/gpt-6-astra",
	),
	newGPTImage25MarketplaceSeed(
		"gpt-image-2.5-sunburst", "GPT Image 2.5 Sunburst",
		"OpenAI 最高能力的图像生成与精细编辑模型，支持多图参考、透明背景、灵活分辨率及最高 4K 输出。",
		"OpenAI's most capable image generation and precision editing model with multi-image references, transparent backgrounds, flexible resolutions, and output up to 4K.",
		"https://developers.openai.com/api/docs/models/gpt-image-2.5-sunburst",
	),
	newGPTImage25MarketplaceSeed(
		"gpt-image-2.5-flare", "GPT Image 2.5 Flare",
		"OpenAI 面向日常创意工作的高速高质量图像生成与编辑模型，支持多图参考、透明背景和最高 4K 输出。",
		"OpenAI's fast, high-quality image generation and editing model for everyday creative work with multi-image references, transparent backgrounds, and output up to 4K.",
		"https://developers.openai.com/api/docs/models/gpt-image-2.5-flare",
	),
	{
		ModelName:       "grok-4.7",
		DisplayName:     "Grok 4.7",
		Description:     "xAI 新一代旗舰推理模型，面向编码、Agent 与专业知识工作，强化长任务执行、自检和长上下文管理。",
		DescriptionEN:   "xAI's frontier reasoning model for coding, agentic tasks, and knowledge work, with stronger long-horizon execution, self-verification, and long-context management.",
		Icon:            "Grok.Color",
		Tags:            "text,chat,reasoning,multimodal,tools",
		ContextLength:   500_000,
		MaxOutputTokens: 499_996,
		ReleaseDate:     "2026-09-21",
		InputModalities: []string{"text", "image"},
		OutputModalities: []string{
			"text",
		},
		Capabilities: []string{
			"function_calling", "streaming", "vision", "json_mode", "structured_output",
			"reasoning", "tools", "system_prompt", "web_search", "code_interpreter", "caching",
		},
		SupportedParameters: []string{
			"stream", "temperature", "top_p", "max_tokens", "tools", "tool_choice", "reasoning_effort", "response_format",
		},
		MetadataSource:     "https://docs.x.ai/developers/models/grok-4.7",
		MetadataVerifiedAt: "2026-10-01",
	},
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
	{
		ModelName:             "doubao-seedance-2-0-mini-260615",
		DisplayName:           "doubao-seedance-2-0-mini-260615",
		Description:           "面向高性价比视频生成场景的 Seedance 2.0 Mini，支持文本、图片、视频和音频参考，输出 480p/720p 视频。",
		Tags:                  "video,multimodal",
		ReleaseDate:           "2026-06-15",
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
	{
		ModelName:             "doubao-seedance-2-5-260628",
		DisplayName:           "doubao-seedance-2-5-260628",
		Description:           "Seedance 2.5 视频生成模型，支持文本、图片、视频和音频参考、视频编辑与更长时长输出，最高支持 1080p。",
		Tags:                  "video,multimodal",
		ReleaseDate:           "2026-06-28",
		InputModalities:       []string{"text", "image", "video", "audio"},
		OutputModalities:      []string{"video", "audio"},
		Capabilities:          []string{"video_generation", "video_editing", "audio_generation", "web_search"},
		SupportedResolutions:  []string{"480p", "720p", "1080p"},
		SupportedAspectRatios: []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"},
		MaxInputImages:        30,
		OutputFormats:         []string{"url"},
		MinDuration:           4,
		MaxDuration:           30,
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

var localMarketplaceSeedVendorNames = map[string]string{
	"deepseek-flash":         "DeepSeek",
	"gemini-3.8-flash":       "Google",
	"claude-sonnet-5-5":      "Anthropic",
	"claude-opus-5-5":        "Anthropic",
	"claude-fable-5-1":       "Anthropic",
	"gpt-6-sol":              "OpenAI",
	"gpt-6-luna":             "OpenAI",
	"gpt-6-astra":            "OpenAI",
	"gpt-image-2.5-sunburst": "OpenAI",
	"gpt-image-2.5-flare":    "OpenAI",
	"grok-4.7":               "xAI",
}

func newAnthropic55MarketplaceSeed(modelName, displayName, description, descriptionEN, releaseDate, source string) Model {
	return Model{
		ModelName: modelName, DisplayName: displayName, Description: description, DescriptionEN: descriptionEN,
		Icon: "Claude.Color", Tags: "text,chat,reasoning,multimodal,tools",
		ContextLength: 1_000_000, MaxOutputTokens: 128_000, KnowledgeCutoff: "2026-06", ReleaseDate: releaseDate,
		InputModalities: []string{"text", "image"}, OutputModalities: []string{"text"},
		Capabilities:        []string{"function_calling", "streaming", "vision", "structured_output", "reasoning", "tools", "system_prompt", "web_search", "caching"},
		SupportedParameters: []string{"stream", "max_tokens", "tools", "tool_choice", "reasoning_effort", "response_format"},
		MetadataSource:      source, MetadataVerifiedAt: "2026-09-29",
	}
}

func newGPT6MarketplaceSeed(modelName, displayName, description, descriptionEN, knowledgeCutoff, releaseDate, source string) Model {
	return Model{
		ModelName: modelName, DisplayName: displayName, Description: description, DescriptionEN: descriptionEN,
		Icon: "OpenAI.Color", Tags: "text,chat,reasoning,multimodal,tools",
		ContextLength: 1_050_000, MaxOutputTokens: 128_000, KnowledgeCutoff: knowledgeCutoff, ReleaseDate: releaseDate,
		InputModalities: []string{"text", "image"}, OutputModalities: []string{"text"},
		Capabilities:        []string{"function_calling", "streaming", "vision", "structured_output", "reasoning", "tools", "system_prompt", "web_search", "code_interpreter", "caching"},
		SupportedParameters: []string{"stream", "max_tokens", "tools", "tool_choice", "reasoning_effort", "response_format"},
		MetadataSource:      source, MetadataVerifiedAt: "2026-09-29",
	}
}

func newGPTImage25MarketplaceSeed(modelName, displayName, description, descriptionEN, source string) Model {
	return Model{
		ModelName: modelName, DisplayName: displayName, Description: description, DescriptionEN: descriptionEN,
		Icon: "OpenAI.Color", Tags: "image,generation,editing,4k,transparent", ReleaseDate: "2026-09-08",
		InputModalities: []string{"text", "image"}, OutputModalities: []string{"image"},
		Capabilities: []string{"image_generation", "image_editing"}, SupportedParameters: []string{"quality"},
		SupportedResolutions:  []string{"auto", "1024x1024", "1536x1024", "1024x1536", "2048x2048", "2048x1152", "3840x2160", "2160x3840", "custom"},
		SupportedAspectRatios: []string{"auto", "1:1", "3:2", "2:3", "16:9", "9:16", "custom ≤3:1"},
		MaxInputImages:        16, OutputFormats: []string{"b64_json"}, ReferenceModalities: []string{"image"},
		MetadataSource: source, MetadataVerifiedAt: "2026-09-29",
	}
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
	if db.Dialector.Name() == "mysql" {
		// Adding NOT NULL TEXT columns to populated MySQL tables uses empty
		// strings, since portable TEXT defaults are unavailable. Repair only
		// those empty legacy values; preserve every configured JSON array.
		return db.Transaction(func(tx *gorm.DB) error {
			for _, column := range []string{"supported_parameters", "supported_resolutions", "supported_aspect_ratios", "output_formats", "reference_modalities"} {
				if err := tx.Model(&Model{}).Where(column+" IS NULL OR "+column+" = ?", "").UpdateColumn(column, "[]").Error; err != nil {
					return err
				}
			}
			return nil
		})
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "molii-new-api-20260815-model-marketplace-metadata").Error; err != nil {
			return fmt.Errorf("lock model marketplace metadata schema: %w", err)
		}
		expected := map[string]string{"display_name": "", "description_en": "", "marketplace_enabled": "false", "supported_parameters": "[]", "supported_resolutions": "[]", "supported_aspect_ratios": "[]", "max_input_images": "0", "output_formats": "[]", "min_duration": "0", "max_duration": "0", "reference_modalities": "[]"}
		columns, err := tx.Migrator().ColumnTypes(&Model{})
		if err != nil {
			return err
		}
		converged := 0
		for _, column := range columns {
			want, exists := expected[column.Name()]
			if !exists {
				continue
			}
			value, hasDefault := column.DefaultValue()
			nullable, known := column.Nullable()
			if hasDefault && strings.Trim(value, "'") == want && known && !nullable {
				converged++
			}
		}
		if converged != len(expected) {
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
		}
		if !tx.Migrator().HasIndex(&Model{}, "idx_models_marketplace_enabled_status") {
			if err := tx.Exec(`
			CREATE INDEX IF NOT EXISTS idx_models_marketplace_enabled_status
			ON public.models (marketplace_enabled, status)
			WHERE deleted_at IS NULL
		`).Error; err != nil {
				return fmt.Errorf("create model marketplace publication index: %w", err)
			}
		}
		return nil
	})
}

// BackfillLocalMarketplaceMetadata migrates reviewed local catalog facts into
// existing Model rows. It never creates models and does not overwrite non-empty
// administrator values, except for narrowly scoped corrections of known legacy
// system defaults.
func BackfillLocalMarketplaceMetadata(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("backfill local marketplace metadata: database is nil")
	}

	modelNames := make([]string, 0, len(localMarketplaceMetadataSeeds))
	for _, seed := range localMarketplaceMetadataSeeds {
		modelNames = append(modelNames, seed.ModelName)
	}

	return withMarketplaceOrderTransaction(db, func(tx *gorm.DB) error {
		if err := tx.Model(&Model{}).
			Where("model_name = ? AND max_input_images = ?", "doubao-seedance-2-5-260628", 9).
			UpdateColumn("max_input_images", 30).Error; err != nil {
			return fmt.Errorf("correct Seedance 2.5 legacy image limit: %w", err)
		}
		return backfillLocalMarketplaceMetadata(tx, modelNames)
	})
}

func backfillLocalMarketplaceMetadata(tx *gorm.DB, modelNames []string) error {
	var rows []Model
	if err := tx.Where("model_name IN ?", modelNames).Find(&rows).Error; err != nil {
		return fmt.Errorf("load local marketplace models: %w", err)
	}
	rowsByName := make(map[string]*Model, len(rows))
	for index := range rows {
		rowsByName[rows[index].ModelName] = &rows[index]
	}

	for _, sourceSeed := range localMarketplaceMetadataSeeds {
		row := rowsByName[sourceSeed.ModelName]
		if row == nil {
			continue
		}
		if localMarketplaceSeedVendorNames[row.ModelName] != "" && row.SyncOfficial == 0 {
			continue
		}
		seed := sourceSeed
		if err := seed.NormalizeCatalogMetadata(); err != nil {
			return fmt.Errorf("normalize local marketplace metadata for %s: %w", seed.ModelName, err)
		}
		if err := updateLocalMarketplaceFieldsIfEmpty(tx, row.Id, &seed); err != nil {
			return fmt.Errorf("backfill local marketplace metadata for %s: %w", row.ModelName, err)
		}
		if err := assignLocalMarketplaceVendorIfEmpty(tx, row.Id, row.ModelName); err != nil {
			return fmt.Errorf("assign local marketplace vendor for %s: %w", row.ModelName, err)
		}

		var persisted Model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", row.Id).First(&persisted).Error; err != nil {
			return fmt.Errorf("reload local marketplace metadata for %s: %w", row.ModelName, err)
		}
		officialSyncEnabled := localMarketplaceSeedVendorNames[row.ModelName] == "" || persisted.SyncOfficial != 0
		if persisted.Status == 1 && officialSyncEnabled && !persisted.MarketplaceEnabled && persisted.EvaluateMarketplaceReadiness().Complete {
			if err := localMarketplaceModelUpdate(tx, persisted.Id, row.ModelName).
				Where("status = ? AND marketplace_enabled = ?", 1, false).
				UpdateColumn("marketplace_enabled", true).Error; err != nil {
				return fmt.Errorf("publish local marketplace metadata for %s: %w", row.ModelName, err)
			}
		}
	}
	return nil
}

func assignLocalMarketplaceVendorIfEmpty(tx *gorm.DB, modelID int, modelName string) error {
	vendorName := localMarketplaceSeedVendorNames[modelName]
	if vendorName == "" {
		return nil
	}
	var vendor Vendor
	result := tx.Where("LOWER(TRIM(name)) = LOWER(?)", vendorName).Limit(1).Find(&vendor)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return tx.Model(&Model{}).
		Where("id = ? AND COALESCE(vendor_id, 0) = 0 AND sync_official <> ?", modelID, 0).
		UpdateColumn("vendor_id", vendor.Id).Error
}

func localMarketplaceModelUpdate(tx *gorm.DB, modelID int, modelName string) *gorm.DB {
	query := tx.Model(&Model{}).Where("id = ?", modelID)
	if localMarketplaceSeedVendorNames[modelName] != "" {
		query = query.Where("sync_official <> ?", 0)
	}
	return query
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
		{"metadata_source", seed.MetadataSource},
		{"metadata_verified_at", seed.MetadataVerifiedAt},
	}
	for _, field := range stringFields {
		if strings.TrimSpace(field.value) == "" {
			continue
		}
		emptyCondition := fmt.Sprintf("TRIM(COALESCE(%s, '')) = ''", field.column)
		if field.column == "display_name" && localMarketplaceSeedVendorNames[seed.ModelName] != "" {
			emptyCondition = "(" + emptyCondition + " OR (TRIM(COALESCE(display_name, '')) = model_name AND TRIM(COALESCE(description, '')) = '' AND COALESCE(vendor_id, 0) = 0))"
		}
		if err := localMarketplaceModelUpdate(tx, id, seed.ModelName).Where(emptyCondition).UpdateColumn(field.column, field.value).Error; err != nil {
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
		if err := localMarketplaceModelUpdate(tx, id, seed.ModelName).Where(emptyCondition).UpdateColumn(field.column, field.value).Error; err != nil {
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
		if err := localMarketplaceModelUpdate(tx, id, seed.ModelName).Where(emptyCondition).UpdateColumn(field.column, string(encoded)).Error; err != nil {
			return fmt.Errorf("fill %s: %w", field.column, err)
		}
	}
	return nil
}
