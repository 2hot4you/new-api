package model

// CatalogVendorProfile is the reviewed seed used only when a persisted vendor
// is missing fields. The Vendor row remains authoritative at runtime.
type CatalogVendorProfile struct {
	Name        string
	Icon        string
	Description string
}

// CatalogModelProfile contains public facts reviewed for one exact model ID.
// Persisted Model rows remain authoritative and may be edited by administrators.
type CatalogModelProfile struct {
	ModelName          string
	VendorName         string
	Description        string
	Icon               string
	Tags               string
	ContextLength      int
	MaxOutputTokens    int
	KnowledgeCutoff    string
	ReleaseDate        string
	InputModalities    []string
	OutputModalities   []string
	Capabilities       []string
	MetadataSource     string
	MetadataVerifiedAt string
}

var catalogVendorProfiles = map[string]CatalogVendorProfile{
	"MiniMax": {
		Name:        "MiniMax",
		Icon:        "Minimax.Color",
		Description: "专注于通用大模型与多模态智能能力，为对话、编程、Agent 和内容创作提供模型服务。",
	},
	"阿里巴巴": {
		Name:        "阿里巴巴",
		Icon:        "Qwen.Color",
		Description: "通义系列模型提供通用对话、推理、编程与多模态能力，覆盖高性能与高性价比场景。",
	},
	"字节跳动": {
		Name:        "字节跳动",
		Icon:        "Doubao.Color",
		Description: "豆包模型系列覆盖文本、图像与视频生成，面向多模态内容创作和企业应用。",
	},
	"xAI": {
		Name:        "xAI",
		Icon:        "XAI",
		Description: "xAI 的 Grok Imagine 系列面向高质量图片与视频生成、编辑和创意工作流。",
	},
	"DeepSeek": {
		Name:        "DeepSeek",
		Icon:        "DeepSeek.Color",
		Description: "DeepSeek 提供面向推理、编程和 Agent 工作流的高性能大语言模型。",
	},
	"智谱": {
		Name:        "智谱",
		Icon:        "Zhipu.Color",
		Description: "智谱 GLM 系列覆盖通用对话、推理、编程和长上下文应用。",
	},
	"Moonshot": {
		Name:        "Moonshot",
		Icon:        "Moonshot",
		Description: "Moonshot 的 Kimi 系列专注于长上下文理解、复杂推理和智能 Agent 场景。",
	},
}

var catalogModelProfiles = map[string]CatalogModelProfile{
	"deepseek-v4-flash-202605": llmCatalogProfile(
		"deepseek-v4-flash-202605", "DeepSeek",
		"面向高效对话、推理、编程与 Agent 工作流的长上下文模型；支持通过 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 384_000, "", "", true,
	),
	"deepseek-v4-pro-202606": llmCatalogProfile(
		"deepseek-v4-pro-202606", "DeepSeek",
		"面向复杂推理、编程与 Agent 工作流的旗舰长上下文模型；支持通过 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 384_000, "", "", true,
	),
	"glm-5.2": llmCatalogProfile(
		"glm-5.2", "智谱",
		"面向长程编程与复杂推理的旗舰模型，支持长上下文、深度思考和工具调用；可通过 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 131_072, "", "2026-06-13", true,
	),
	"kimi-k3": llmCatalogProfile(
		"kimi-k3", "Moonshot",
		"面向通用对话、内容生成与复杂问答的文本模型；支持通过 OpenAI 兼容 Chat Completions API 调用。",
		1_048_576, 131_072, "", "2026-07-16", true,
	),
	"minimax-m3": llmCatalogProfile(
		"minimax-m3", "MiniMax",
		"面向长上下文编程与 Agent 工作流的模型；当前通过本网关的 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 128_000, "", "2026-06-01", false,
	),
	"qwen3.5-flash": llmCatalogProfile(
		"qwen3.5-flash", "阿里巴巴",
		"面向低成本长上下文对话的模型；当前通过本网关的 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 65_536, "", "2026-02-23", true,
	),
	"qwen3.5-plus": llmCatalogProfile(
		"qwen3.5-plus", "阿里巴巴",
		"面向更高能力长上下文对话的模型；当前通过本网关的 OpenAI 兼容 Chat Completions API 调用。",
		1_000_000, 65_536, "2025-04", "2026-02-16", false,
	),
	"doubao-seedance-2-0-260128": {
		ModelName: "doubao-seedance-2-0-260128", VendorName: "字节跳动",
		Description: "新一代多模态视频生成模型，支持文本、图片和视频参考，擅长复杂运动、真实物理、精准控制及同步音频生成。",
		Tags:        "video,multimodal", InputModalities: []string{"text", "image", "video", "audio"}, OutputModalities: []string{"video", "audio"},
		Capabilities: []string{"video_generation", "video_editing", "audio_generation", "web_search"}, MetadataSource: "Molii", MetadataVerifiedAt: "2026-08-13",
	},
	"doubao-seedance-2-0-fast-260128": {
		ModelName: "doubao-seedance-2-0-fast-260128", VendorName: "字节跳动",
		Description: "Seedance 2.0 加速版，支持文本、图片和视频参考及同步音频生成，面向低延迟创意迭代，支持 480p/720p。",
		Tags:        "video,multimodal", InputModalities: []string{"text", "image", "video", "audio"}, OutputModalities: []string{"video", "audio"},
		Capabilities: []string{"video_generation", "video_editing", "audio_generation", "web_search"}, MetadataSource: "Molii", MetadataVerifiedAt: "2026-08-13",
	},
	"grok-imagine-image": {
		ModelName: "grok-imagine-image", VendorName: "xAI",
		Description: "支持文本生成图片与参考图编辑，提供 1K、2K 输出，生成速度更快。",
		Tags:        "image,generation,editing", InputModalities: []string{"text", "image"}, OutputModalities: []string{"image"},
		Capabilities: []string{"image_generation", "image_editing"}, MetadataSource: "Molii", MetadataVerifiedAt: "2026-08-13",
	},
	"grok-imagine-image-quality": {
		ModelName: "grok-imagine-image-quality", VendorName: "xAI",
		Description: "更注重画面细节和质量的图片生成与编辑模型，支持 1K、2K 输出。",
		Tags:        "image,generation,editing", InputModalities: []string{"text", "image"}, OutputModalities: []string{"image"},
		Capabilities: []string{"image_generation", "image_editing"}, MetadataSource: "Molii", MetadataVerifiedAt: "2026-08-13",
	},
	"grok-imagine-video": {
		ModelName: "grok-imagine-video", VendorName: "xAI",
		Description: "异步文生视频、图生视频与视频编辑模型，支持 480p、720p 输出。",
		Tags:        "video,generation,editing", InputModalities: []string{"text", "image", "video"}, OutputModalities: []string{"video"},
		Capabilities: []string{"video_generation", "video_editing"}, MetadataSource: "Molii", MetadataVerifiedAt: "2026-08-13",
	},
	"grok-imagine-video-1.5": {
		ModelName: "grok-imagine-video-1.5", VendorName: "xAI",
		Description: "异步图生视频模型，支持 480p、720p、1080p 输出。",
		Tags:        "video,generation", InputModalities: []string{"text", "image"}, OutputModalities: []string{"video"},
		Capabilities: []string{"video_generation"}, MetadataSource: "Molii", MetadataVerifiedAt: "2026-08-13",
	},
}

func llmCatalogProfile(modelName, vendorName, description string, contextLength, maxOutputTokens int, knowledgeCutoff, releaseDate string, structuredOutput bool) CatalogModelProfile {
	capabilities := []string{"streaming", "system_prompt", "reasoning", "tools"}
	if structuredOutput {
		capabilities = append(capabilities, "structured_output")
	}
	return CatalogModelProfile{
		ModelName: modelName, VendorName: vendorName, Description: description, Tags: "text,chat,context",
		ContextLength: contextLength, MaxOutputTokens: maxOutputTokens, KnowledgeCutoff: knowledgeCutoff, ReleaseDate: releaseDate,
		InputModalities: []string{"text"}, OutputModalities: []string{"text"}, Capabilities: capabilities,
		MetadataSource: "models.dev", MetadataVerifiedAt: "2026-08-13",
	}
}

func GetCatalogModelProfile(modelName string) (CatalogModelProfile, bool) {
	profile, ok := catalogModelProfiles[modelName]
	return profile, ok
}

func GetCatalogVendorProfile(vendorName string) (CatalogVendorProfile, bool) {
	profile, ok := catalogVendorProfiles[vendorName]
	return profile, ok
}

// Compatibility view retained until pricing is fully sourced from Model rows.
type marketplaceCatalogMetadata struct {
	ContextLength      int
	MaxOutputTokens    int
	KnowledgeCutoff    string
	ReleaseDate        string
	InputModalities    []string
	OutputModalities   []string
	Capabilities       []string
	MetadataSource     string
	MetadataVerifiedAt string
	BillingCurrency    string
}

func getMarketplaceCatalogMetadata(modelName string) (marketplaceCatalogMetadata, bool) {
	profile, ok := GetCatalogModelProfile(modelName)
	if !ok || profile.ContextLength == 0 {
		return marketplaceCatalogMetadata{}, false
	}
	return marketplaceCatalogMetadata{
		ContextLength: profile.ContextLength, MaxOutputTokens: profile.MaxOutputTokens,
		KnowledgeCutoff: profile.KnowledgeCutoff, ReleaseDate: profile.ReleaseDate,
		InputModalities: profile.InputModalities, OutputModalities: profile.OutputModalities,
		Capabilities: profile.Capabilities, MetadataSource: profile.MetadataSource,
		MetadataVerifiedAt: profile.MetadataVerifiedAt, BillingCurrency: getMarketplaceBillingCurrency(modelName),
	}, true
}

func getMarketplaceBillingCurrency(modelName string) string {
	if modelName == "minimax-m3" || modelName == "qwen3.5-flash" || modelName == "qwen3.5-plus" {
		return "CNY"
	}
	return ""
}
