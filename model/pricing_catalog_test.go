package model

import (
	"reflect"
	"testing"
)

func TestMarketplaceCatalogMetadataIsVerifiedAndExact(t *testing.T) {
	tests := []struct {
		model           string
		context         int
		maxOutput       int
		knowledgeCutoff string
		releaseDate     string
		capabilities    []string
		billingCurrency string
	}{
		{
			model:        "deepseek-v4-flash-202605",
			context:      1_000_000,
			maxOutput:    384_000,
			capabilities: []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		},
		{
			model:        "deepseek-v4-pro-202606",
			context:      1_000_000,
			maxOutput:    384_000,
			capabilities: []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		},
		{
			model:        "glm-5.2",
			context:      1_000_000,
			maxOutput:    131_072,
			releaseDate:  "2026-06-13",
			capabilities: []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		},
		{
			model:        "kimi-k3",
			context:      1_048_576,
			maxOutput:    131_072,
			releaseDate:  "2026-07-16",
			capabilities: []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
		},
		{
			model:           "minimax-m3",
			context:         1_000_000,
			maxOutput:       128_000,
			releaseDate:     "2026-06-01",
			capabilities:    []string{"streaming", "system_prompt", "reasoning", "tools"},
			billingCurrency: "CNY",
		},
		{
			model:           "qwen3.5-flash",
			context:         1_000_000,
			maxOutput:       65_536,
			releaseDate:     "2026-02-23",
			capabilities:    []string{"streaming", "system_prompt", "reasoning", "tools", "structured_output"},
			billingCurrency: "CNY",
		},
		{
			model:           "qwen3.5-plus",
			context:         1_000_000,
			maxOutput:       65_536,
			knowledgeCutoff: "2025-04",
			releaseDate:     "2026-02-16",
			capabilities:    []string{"streaming", "system_prompt", "reasoning", "tools"},
			billingCurrency: "CNY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			meta, ok := getMarketplaceCatalogMetadata(tt.model)
			if !ok {
				t.Fatalf("missing metadata for %s", tt.model)
			}
			if meta.ContextLength != tt.context {
				t.Errorf("context = %d, want %d", meta.ContextLength, tt.context)
			}
			if meta.MaxOutputTokens != tt.maxOutput {
				t.Errorf("max output = %d, want %d", meta.MaxOutputTokens, tt.maxOutput)
			}
			if meta.KnowledgeCutoff != tt.knowledgeCutoff {
				t.Errorf("knowledge cutoff = %q, want %q", meta.KnowledgeCutoff, tt.knowledgeCutoff)
			}
			if meta.ReleaseDate != tt.releaseDate {
				t.Errorf("release date = %q, want %q", meta.ReleaseDate, tt.releaseDate)
			}
			if !reflect.DeepEqual(meta.InputModalities, []string{"text"}) {
				t.Errorf("input modalities = %v, want only text", meta.InputModalities)
			}
			if !reflect.DeepEqual(meta.OutputModalities, []string{"text"}) {
				t.Errorf("output modalities = %v, want only text", meta.OutputModalities)
			}
			if !reflect.DeepEqual(meta.Capabilities, tt.capabilities) {
				t.Errorf("capabilities = %v, want %v", meta.Capabilities, tt.capabilities)
			}
			if meta.MetadataSource != "models.dev" {
				t.Errorf("metadata source = %q, want models.dev", meta.MetadataSource)
			}
			if meta.MetadataVerifiedAt != "2026-08-13" {
				t.Errorf("metadata verified at = %q, want 2026-08-13", meta.MetadataVerifiedAt)
			}
			if meta.BillingCurrency != tt.billingCurrency {
				t.Errorf("billing currency = %q, want %q", meta.BillingCurrency, tt.billingCurrency)
			}
		})
	}

	for _, unknown := range []string{"qwen3.5-turbo", "deepseek-v4-preview", "grok-imagine-video"} {
		if _, ok := getMarketplaceCatalogMetadata(unknown); ok {
			t.Fatalf("metadata must not apply to unknown model %q", unknown)
		}
	}
}
