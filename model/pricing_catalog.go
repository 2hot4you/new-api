package model

// marketplaceCatalogMetadata contains only public catalog facts that Molii has
// reviewed for the exact model ID. models.dev is a reference source, while this
// registry remains authoritative for what the Molii relay currently exposes.
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

var exactMarketplaceCatalogMetadata = map[string]marketplaceCatalogMetadata{
	"deepseek-v4-flash-202605": {
		ContextLength:      1_000_000,
		MaxOutputTokens:    384_000,
		InputModalities:    []string{"text"},
		OutputModalities:   []string{"text"},
		Capabilities:       []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		MetadataSource:     "models.dev",
		MetadataVerifiedAt: "2026-08-13",
	},
	"deepseek-v4-pro-202606": {
		ContextLength:      1_000_000,
		MaxOutputTokens:    384_000,
		InputModalities:    []string{"text"},
		OutputModalities:   []string{"text"},
		Capabilities:       []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		MetadataSource:     "models.dev",
		MetadataVerifiedAt: "2026-08-13",
	},
	"glm-5.2": {
		ContextLength:      1_000_000,
		MaxOutputTokens:    131_072,
		ReleaseDate:        "2026-06-13",
		InputModalities:    []string{"text"},
		OutputModalities:   []string{"text"},
		Capabilities:       []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		MetadataSource:     "models.dev",
		MetadataVerifiedAt: "2026-08-13",
	},
	"kimi-k3": {
		ContextLength:      1_048_576,
		MaxOutputTokens:    131_072,
		ReleaseDate:        "2026-07-16",
		InputModalities:    []string{"text"},
		OutputModalities:   []string{"text"},
		Capabilities:       []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		MetadataSource:     "models.dev",
		MetadataVerifiedAt: "2026-08-13",
	},
	"minimax-m3": {
		ContextLength:      1_000_000,
		MaxOutputTokens:    128_000,
		ReleaseDate:        "2026-06-01",
		InputModalities:    []string{"text"},
		OutputModalities:   []string{"text"},
		Capabilities:       []string{"streaming", "system_prompt", "reasoning", "tools"},
		MetadataSource:     "models.dev",
		MetadataVerifiedAt: "2026-08-13",
		BillingCurrency:    "CNY",
	},
	"qwen3.5-flash": {
		ContextLength:      1_000_000,
		MaxOutputTokens:    65_536,
		ReleaseDate:        "2026-02-23",
		InputModalities:    []string{"text"},
		OutputModalities:   []string{"text"},
		Capabilities:       []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		MetadataSource:     "models.dev",
		MetadataVerifiedAt: "2026-08-13",
		BillingCurrency:    "CNY",
	},
	"qwen3.5-plus": {
		ContextLength:      1_000_000,
		MaxOutputTokens:    65_536,
		KnowledgeCutoff:    "2025-04",
		ReleaseDate:        "2026-02-16",
		InputModalities:    []string{"text"},
		OutputModalities:   []string{"text"},
		Capabilities:       []string{"streaming", "system_prompt", "reasoning", "tools"},
		MetadataSource:     "models.dev",
		MetadataVerifiedAt: "2026-08-13",
		BillingCurrency:    "CNY",
	},
}

func getMarketplaceCatalogMetadata(modelName string) (marketplaceCatalogMetadata, bool) {
	metadata, ok := exactMarketplaceCatalogMetadata[modelName]
	return metadata, ok
}
