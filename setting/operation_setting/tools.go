package operation_setting

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// ---------------------------------------------------------------------------
// Tool call prices ($/1K calls, admin-configurable)
// DB key: tool_price_setting.prices
//
// Key format:
//   - "tool_name"              → default price for all models
//   - "tool_name:model_prefix*" → override for models matching the prefix
//
// Effective index: hardcoded defaults → hardcoded model overrides → valid
// operator values. Lookup uses the longest model prefix before the tool
// default, and a matched numeric zero is terminal.
// ---------------------------------------------------------------------------

const ToolPriceOptionKey = "tool_price_setting.prices"

const (
	defaultWebSearchToolPrice        = 10.0
	defaultWebSearchPreviewToolPrice = 10.0
	defaultFileSearchToolPrice       = 2.5
	defaultGoogleSearchToolPrice     = 14.0
	defaultImageGenerationToolPrice  = 150.0
	defaultSearchPreviewModelPrice   = 25.0
)

// seedHardcodedToolPrices injects compile-time built-in fallbacks (tool
// defaults and model-prefix overrides) into the destination. The source is
// constants, not a mutable package map or operator configuration.
func seedHardcodedToolPrices(prices map[string]float64) {
	prices["web_search"] = defaultWebSearchToolPrice
	prices["web_search_preview"] = defaultWebSearchPreviewToolPrice
	prices["file_search"] = defaultFileSearchToolPrice
	prices["google_search"] = defaultGoogleSearchToolPrice
	prices["image_generation"] = defaultImageGenerationToolPrice
	prices["web_search_preview:gpt-4o*"] = defaultSearchPreviewModelPrice
	prices["web_search_preview:gpt-4.1*"] = defaultSearchPreviewModelPrice
	prices["web_search_preview:gpt-4o-mini*"] = defaultSearchPreviewModelPrice
	prices["web_search_preview:gpt-4.1-mini*"] = defaultSearchPreviewModelPrice
}

// ToolPriceSetting is managed by config.GlobalConfig.Register.
// Prices holds operator overrides only; hardcoded fallbacks live in the index.
type ToolPriceSetting struct {
	Prices map[string]float64 `json:"prices"`
}

var toolPriceSetting = DefaultToolPriceSetting()

func DefaultToolPriceSetting() ToolPriceSetting {
	return ToolPriceSetting{Prices: make(map[string]float64)}
}

var toolPriceMu sync.Mutex

// PreparedToolPrices owns both the administrator values and their immutable
// lookup index. Build before acquiring a catalog publication lock.
type PreparedToolPrices struct {
	prices map[string]float64
	index  *toolPriceIndex
}

func PrepareToolPrices(prices map[string]float64) (*PreparedToolPrices, error) {
	owned := maps.Clone(prices)
	if owned == nil {
		owned = make(map[string]float64)
	}
	for name, price := range owned {
		if strings.TrimSpace(name) == "" || !isValidToolPrice(price) {
			return nil, fmt.Errorf("invalid tool price %q", name)
		}
	}
	return &PreparedToolPrices{prices: owned, index: buildToolPriceIndex(owned)}, nil
}

// PublishToolPrices performs no parsing, validation or index building. The
// caller owns the catalog writer; individual lookups use the immutable index.
func PublishToolPrices(prepared *PreparedToolPrices) {
	toolPriceMu.Lock()
	toolPriceSetting.Prices = maps.Clone(prepared.prices)
	currentIndex.Store(prepared.index)
	toolPriceMu.Unlock()
}

func init() {
	config.GlobalConfig.Register("tool_price_setting", &toolPriceSetting)
	RebuildToolPriceIndex()
}

// ---------------------------------------------------------------------------
// Precomputed price index (atomic, lock-free on read path)
// ---------------------------------------------------------------------------

type prefixEntry struct {
	prefix string
	price  float64
}

type toolPriceIndex struct {
	defaults map[string]float64
	prefixes map[string][]prefixEntry
}

var currentIndex atomic.Pointer[toolPriceIndex]

func isValidToolPrice(price float64) bool {
	return price >= 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

func decodeToolPricesJSON(value string, ignoreInvalidEntries bool) (map[string]float64, error) {
	rawValue := json.RawMessage(strings.TrimSpace(value))
	if common.GetJsonType(rawValue) != "object" {
		return nil, fmt.Errorf("工具价格必须是 JSON 对象")
	}

	var rawPrices map[string]json.RawMessage
	if err := common.Unmarshal(rawValue, &rawPrices); err != nil {
		return nil, fmt.Errorf("解析工具价格失败: %w", err)
	}

	prices := make(map[string]float64, len(rawPrices))
	for name, rawPrice := range rawPrices {
		var entryErr error
		if common.GetJsonType(rawPrice) != "number" {
			entryErr = fmt.Errorf("工具价格 %q 必须是非负数字", name)
		} else {
			var price float64
			if err := common.Unmarshal(rawPrice, &price); err != nil {
				entryErr = fmt.Errorf("解析工具价格 %q 失败: %w", name, err)
			} else if !isValidToolPrice(price) {
				entryErr = fmt.Errorf("工具价格 %q 必须是有限的非负数字", name)
			} else {
				prices[name] = price
			}
		}

		if entryErr == nil {
			continue
		}
		if !ignoreInvalidEntries {
			return nil, entryErr
		}
		common.SysError(entryErr.Error())
	}
	return prices, nil
}

// ValidateToolPricesJSON validates an operator-supplied complete price map.
// A numeric zero is valid and intentionally disables the matching rule.
func ValidateToolPricesJSON(value string) error {
	_, err := decodeToolPricesJSON(value, false)
	return err
}

// LoadToolPricesFromJSONString replaces the complete operator price map.
// Invalid legacy entries are ignored individually so valid sibling overrides
// survive, while missing built-in keys continue to use hardcoded fallbacks.
func LoadToolPricesFromJSONString(value string) {
	prices, err := decodeToolPricesJSON(value, true)
	if err != nil {
		common.SysError("加载工具价格失败，将使用硬编码兜底: " + err.Error())
		prices = make(map[string]float64)
	}
	toolPriceMu.Lock()
	toolPriceSetting.Prices = prices
	currentIndex.Store(buildToolPriceIndex(prices))
	toolPriceMu.Unlock()
}

// RebuildToolPriceIndex rebuilds the lookup index from the current config.
// Called on init and after config updates. Not on the billing hot path.
func RebuildToolPriceIndex() {
	toolPriceMu.Lock()
	defer toolPriceMu.Unlock()
	currentIndex.Store(buildToolPriceIndex(toolPriceSetting.Prices))
}

func buildToolPriceIndex(prices map[string]float64) *toolPriceIndex {
	merged := make(map[string]float64, 9+len(prices))
	seedHardcodedToolPrices(merged)
	for k, v := range prices {
		if !isValidToolPrice(v) {
			continue
		}
		merged[k] = v
	}

	idx := &toolPriceIndex{
		defaults: make(map[string]float64),
		prefixes: make(map[string][]prefixEntry),
	}

	for key, price := range merged {
		before, after, ok := strings.Cut(key, ":")
		if !ok {
			idx.defaults[key] = price
			continue
		}
		toolName := before
		modelPart := after
		prefix := strings.TrimSuffix(modelPart, "*")
		idx.prefixes[toolName] = append(idx.prefixes[toolName], prefixEntry{prefix: prefix, price: price})
	}

	for tool := range idx.prefixes {
		entries := idx.prefixes[tool]
		sort.Slice(entries, func(i, j int) bool {
			if len(entries[i].prefix) == len(entries[j].prefix) {
				return entries[i].prefix < entries[j].prefix
			}
			return len(entries[i].prefix) > len(entries[j].prefix)
		})
		idx.prefixes[tool] = entries
	}

	return idx
}

// GetToolPriceForModel returns the price ($/1K calls) for a tool given a model name.
// Lookup: longest prefix match → tool default → 0.
func GetToolPriceForModel(toolName, modelName string) float64 {
	if price, ok := getMoliiGrokToolPrice(toolName, modelName); ok {
		return price
	}
	idx := currentIndex.Load()
	if idx == nil {
		RebuildToolPriceIndex()
		idx = currentIndex.Load()
		if idx == nil {
			return 0
		}
	}

	if entries, ok := idx.prefixes[toolName]; ok && modelName != "" {
		for _, e := range entries {
			if strings.HasPrefix(modelName, e.prefix) {
				return e.price
			}
		}
	}

	if p, ok := idx.defaults[toolName]; ok {
		return p
	}
	return 0
}

// GetToolPrice is a convenience wrapper when no model name is needed.
func GetToolPrice(toolName string) float64 {
	return GetToolPriceForModel(toolName, "")
}

// SetToolPriceForTest injects a tool price and rebuilds the lookup index. Tests only.
func SetToolPriceForTest(name string, price float64) {
	toolPriceMu.Lock()
	defer toolPriceMu.Unlock()
	if toolPriceSetting.Prices == nil {
		toolPriceSetting.Prices = make(map[string]float64)
	}
	toolPriceSetting.Prices[name] = price
	currentIndex.Store(buildToolPriceIndex(toolPriceSetting.Prices))
}

// DeleteToolPriceForTest removes an injected tool price and rebuilds the index. Tests only.
func DeleteToolPriceForTest(name string) {
	toolPriceMu.Lock()
	defer toolPriceMu.Unlock()
	delete(toolPriceSetting.Prices, name)
	currentIndex.Store(buildToolPriceIndex(toolPriceSetting.Prices))
}

// ---------------------------------------------------------------------------
// Gemini audio input pricing (per-million tokens, model-specific)
// ---------------------------------------------------------------------------

const (
	Gemini25FlashPreviewInputAudioPrice     = 1.00
	Gemini25FlashProductionInputAudioPrice  = 1.00
	Gemini25FlashLitePreviewInputAudioPrice = 0.50
	Gemini25FlashNativeAudioInputAudioPrice = 3.00
	Gemini20FlashInputAudioPrice            = 0.70
	GeminiRoboticsER15InputAudioPrice       = 1.00
)

func GetGeminiInputAudioPricePerMillionTokens(modelName string) float64 {
	if strings.HasPrefix(modelName, "gemini-2.5-flash-preview-native-audio") {
		return Gemini25FlashNativeAudioInputAudioPrice
	}
	if strings.HasPrefix(modelName, "gemini-2.5-flash-preview-lite") {
		return Gemini25FlashLitePreviewInputAudioPrice
	}
	if strings.HasPrefix(modelName, "gemini-2.5-flash-preview") {
		return Gemini25FlashPreviewInputAudioPrice
	}
	if strings.HasPrefix(modelName, "gemini-2.5-flash") {
		return Gemini25FlashProductionInputAudioPrice
	}
	if strings.HasPrefix(modelName, "gemini-2.0-flash") {
		return Gemini20FlashInputAudioPrice
	}
	if strings.HasPrefix(modelName, "gemini-robotics-er-1.5") {
		return GeminiRoboticsER15InputAudioPrice
	}
	return 0
}
