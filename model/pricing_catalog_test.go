package model

import "testing"

func TestThreeModelCatalogMetadataIsConservativeAndExact(t *testing.T) {
	tests := []struct {
		model     string
		maxOutput int
	}{
		{model: "minimax-m3", maxOutput: 0},
		{model: "qwen3.5-flash", maxOutput: 65_536},
		{model: "qwen3.5-plus", maxOutput: 65_536},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			meta, ok := getMarketplaceCatalogMetadata(tt.model)
			if !ok {
				t.Fatalf("missing metadata for %s", tt.model)
			}
			if meta.ContextLength != 1_000_000 {
				t.Errorf("context = %d, want 1000000", meta.ContextLength)
			}
			if meta.MaxOutputTokens != tt.maxOutput {
				t.Errorf("max output = %d, want %d", meta.MaxOutputTokens, tt.maxOutput)
			}
			if len(meta.InputModalities) != 1 || meta.InputModalities[0] != "text" {
				t.Errorf("input modalities = %v, want only text", meta.InputModalities)
			}
			if len(meta.OutputModalities) != 1 || meta.OutputModalities[0] != "text" {
				t.Errorf("output modalities = %v, want only text", meta.OutputModalities)
			}
			if len(meta.Capabilities) != 0 {
				t.Errorf("capabilities = %v, want none until relay support is verified", meta.Capabilities)
			}
			if meta.BillingCurrency != "CNY" {
				t.Errorf("billing currency = %q, want CNY", meta.BillingCurrency)
			}
		})
	}

	if _, ok := getMarketplaceCatalogMetadata("qwen3.5-turbo"); ok {
		t.Fatal("metadata must not apply to models outside the requested exact IDs")
	}
}
