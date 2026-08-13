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

func InferCatalogVendorName(modelName string) string {
	modelLower := strings.ToLower(strings.TrimSpace(modelName))
	for _, rule := range defaultVendorRules {
		if strings.Contains(modelLower, rule.Pattern) {
			return rule.Vendor
		}
	}
	return ""
}
