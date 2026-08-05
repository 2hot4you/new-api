package operation_setting

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type MoliiGrokToolPriceSetting struct {
	WebSearch         float64 `json:"web_search"`
	XSearch           float64 `json:"x_search"`
	CodeExecution     float64 `json:"code_execution"`
	AttachmentSearch  float64 `json:"attachment_search"`
	CollectionsSearch float64 `json:"collections_search"`
	ImageGeneration   float64 `json:"image_generation"`
}

var moliiGrokToolPriceSetting = MoliiGrokToolPriceSetting{
	WebSearch:         5,
	XSearch:           5,
	CodeExecution:     5,
	AttachmentSearch:  10,
	CollectionsSearch: 2.5,
	ImageGeneration:   0.05,
}

func init() {
	config.GlobalConfig.Register("molii_grok_tool_price", &moliiGrokToolPriceSetting)
}

func getMoliiGrokToolPrice(toolName, modelName string) (float64, bool) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "grok-") {
		return 0, false
	}
	var price float64
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "web_search", "web_search_preview":
		price = moliiGrokToolPriceSetting.WebSearch
	case "x_search":
		price = moliiGrokToolPriceSetting.XSearch
	case "code_execution", "code_interpreter":
		price = moliiGrokToolPriceSetting.CodeExecution
	case "attachment_search":
		price = moliiGrokToolPriceSetting.AttachmentSearch
	case "collections_search", "file_search":
		price = moliiGrokToolPriceSetting.CollectionsSearch
	case "image_generation":
		// The shared surcharge engine consumes prices per 1K calls, while the
		// Imagine price is configured per completed image.
		price = moliiGrokToolPriceSetting.ImageGeneration * 1000
	default:
		return 0, false
	}
	if price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0, false
	}
	return price, true
}
