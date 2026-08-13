package model

import "testing"

func TestCatalogModelProfilesCoverPublishedMoliiModels(t *testing.T) {
	tests := map[string]string{
		"deepseek-v4-flash-202605":        "DeepSeek",
		"deepseek-v4-pro-202606":          "DeepSeek",
		"glm-5.2":                         "智谱",
		"kimi-k3":                         "Moonshot",
		"minimax-m3":                      "MiniMax",
		"qwen3.5-flash":                   "阿里巴巴",
		"qwen3.5-plus":                    "阿里巴巴",
		"doubao-seedance-2-0-260128":      "字节跳动",
		"doubao-seedance-2-0-fast-260128": "字节跳动",
		"grok-imagine-image":              "xAI",
		"grok-imagine-image-quality":      "xAI",
		"grok-imagine-video":              "xAI",
		"grok-imagine-video-1.5":          "xAI",
	}

	allowedModalities := map[string]bool{"text": true, "image": true, "audio": true, "video": true, "file": true}
	allowedCapabilities := map[string]bool{
		"streaming": true, "system_prompt": true, "reasoning": true, "tools": true,
		"structured_output": true, "image_generation": true, "image_editing": true,
		"video_generation": true, "video_editing": true, "audio_generation": true, "web_search": true,
	}

	for modelName, vendorName := range tests {
		profile, ok := GetCatalogModelProfile(modelName)
		if !ok {
			t.Errorf("missing profile for %s", modelName)
			continue
		}
		if profile.ModelName != modelName || profile.VendorName != vendorName {
			t.Errorf("profile %s identity = %q/%q", modelName, profile.ModelName, profile.VendorName)
		}
		if profile.Description == "" {
			t.Errorf("profile %s has no description", modelName)
		}
		for _, modality := range append(append([]string{}, profile.InputModalities...), profile.OutputModalities...) {
			if !allowedModalities[modality] {
				t.Errorf("profile %s has unsupported modality %q", modelName, modality)
			}
		}
		for _, capability := range profile.Capabilities {
			if !allowedCapabilities[capability] {
				t.Errorf("profile %s has unsupported capability %q", modelName, capability)
			}
		}
	}

	if _, ok := GetCatalogModelProfile("grok-imagine-video-1.5-preview"); ok {
		t.Fatal("retired preview model must not be published")
	}
}

func TestCatalogVendorProfilesContainMarketplaceHeadings(t *testing.T) {
	vendors := []string{"MiniMax", "阿里巴巴", "字节跳动", "xAI", "DeepSeek", "智谱", "Moonshot"}
	for _, vendorName := range vendors {
		profile, ok := GetCatalogVendorProfile(vendorName)
		if !ok {
			t.Errorf("missing vendor profile for %s", vendorName)
			continue
		}
		if profile.Name != vendorName || profile.Icon == "" || profile.Description == "" {
			t.Errorf("incomplete vendor profile for %s: %+v", vendorName, profile)
		}
	}
}

func TestInferCatalogVendorName(t *testing.T) {
	tests := map[string]string{
		"qwen-custom":     "阿里巴巴",
		"grok-custom":     "xAI",
		"doubao-custom":   "字节跳动",
		"unmatched-model": "",
	}
	for modelName, expected := range tests {
		if actual := InferCatalogVendorName(modelName); actual != expected {
			t.Errorf("InferCatalogVendorName(%q) = %q, want %q", modelName, actual, expected)
		}
	}
}
