package model

import (
	"strings"
)

// 简化的供应商映射规则
var defaultVendorRules = []struct {
	Pattern string
	Vendor  string
}{
	{"gpt", "OpenAI"},
	{"dall-e", "OpenAI"},
	{"whisper", "OpenAI"},
	{"o1", "OpenAI"},
	{"o3", "OpenAI"},
	{"claude", "Anthropic"},
	{"gemini", "Google"},
	{"moonshot", "Moonshot"},
	{"kimi", "Moonshot"},
	{"chatglm", "智谱"},
	{"glm-", "智谱"},
	{"qwen", "阿里巴巴"},
	{"deepseek", "DeepSeek"},
	{"abab", "MiniMax"},
	{"minimax", "MiniMax"},
	{"ernie", "百度"},
	{"spark", "讯飞"},
	{"hunyuan", "腾讯"},
	{"command", "Cohere"},
	{"@cf/", "Cloudflare"},
	{"360", "360"},
	{"yi", "零一万物"},
	{"jina", "Jina"},
	{"mistral", "Mistral"},
	{"grok", "xAI"},
	{"llama", "Meta"},
	{"doubao", "字节跳动"},
	{"kling", "快手"},
	{"jimeng", "即梦"},
	{"vidu", "Vidu"},
}

// 供应商默认图标映射
var defaultVendorIcons = map[string]string{
	"OpenAI":     "OpenAI",
	"Anthropic":  "Claude.Color",
	"Google":     "Gemini.Color",
	"Moonshot":   "Moonshot",
	"智谱":         "Zhipu.Color",
	"阿里巴巴":       "Qwen.Color",
	"DeepSeek":   "DeepSeek.Color",
	"MiniMax":    "Minimax.Color",
	"百度":         "Wenxin.Color",
	"讯飞":         "Spark.Color",
	"腾讯":         "Hunyuan.Color",
	"Cohere":     "Cohere.Color",
	"Cloudflare": "Cloudflare.Color",
	"360":        "Ai360.Color",
	"零一万物":       "Yi.Color",
	"Jina":       "Jina",
	"Mistral":    "Mistral.Color",
	"xAI":        "XAI",
	"Meta":       "Ollama",
	"字节跳动":       "Doubao.Color",
	"快手":         "Kling.Color",
	"即梦":         "Jimeng.Color",
	"Vidu":       "Vidu",
	"微软":         "AzureAI",
	"Microsoft":  "AzureAI",
	"Azure":      "AzureAI",
}

var defaultModelDescriptionI18nKeys = map[string]string{
	"doubao-seedance-2-0-260128":      "Seedance 2.0 standard model description",
	"doubao-seedance-2-0-fast-260128": "Seedance 2.0 fast model description",
	"grok-imagine-image":              "Grok Imagine Image model description",
	"grok-imagine-image-quality":      "Grok Imagine Image Quality model description",
	"grok-imagine-video":              "Grok Imagine Video model description",
	"grok-imagine-video-1.5":          "Grok Imagine Video 1.5 model description",
	"minimax-m3":                      "MiniMax M3 model description",
	"qwen3.5-flash":                   "Qwen3.5 Flash model description",
	"qwen3.5-plus":                    "Qwen3.5 Plus model description",
	"deepseek-v4-flash-202605":        "DeepSeek V4 Flash model description",
	"deepseek-v4-pro-202606":          "DeepSeek V4 Pro model description",
	"glm-5.2":                         "GLM 5.2 model description",
	"kimi-k3":                         "Kimi K3 model description",
}

func getDefaultModelDescriptionI18nKey(modelName string) string {
	return defaultModelDescriptionI18nKeys[modelName]
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}

		// 匹配供应商
		vendorID := 0
		if vendorName := InferCatalogVendorName(modelName); vendorName != "" {
			vendorID = getOrCreateVendor(vendorName, vendorMap)
		}

		// 创建模型元数据
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  vendorID,
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

// 查找或创建供应商
func getOrCreateVendor(vendorName string, vendorMap map[int]*Vendor) int {
	// 查找现有供应商
	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id
		}
	}

	// 创建新供应商
	newVendor := &Vendor{
		Name:        vendorName,
		Status:      1,
		Icon:        getDefaultVendorIcon(vendorName),
		Description: getDefaultVendorDescription(vendorName),
	}

	if err := newVendor.Insert(); err != nil {
		return 0
	}

	vendorMap[newVendor.Id] = newVendor
	return newVendor.Id
}

// 获取供应商默认图标
func getDefaultVendorIcon(vendorName string) string {
	if profile, ok := GetCatalogVendorProfile(vendorName); ok {
		return profile.Icon
	}
	if icon, exists := defaultVendorIcons[vendorName]; exists {
		return icon
	}
	return ""
}

func getDefaultVendorDescription(vendorName string) string {
	if profile, ok := GetCatalogVendorProfile(vendorName); ok {
		return profile.Description
	}
	return ""
}

func InferCatalogVendorName(modelName string) string {
	modelLower := strings.ToLower(strings.TrimSpace(modelName))
	for _, rule := range defaultVendorRules {
		if strings.Contains(modelLower, rule.Pattern) {
			return rule.Vendor
		}
	}
	return ""
}
