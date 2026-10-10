package common

import (
	"maps"

	"github.com/QuantumNous/new-api/pkg/billingmoney"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// ModelPricing contains raw inputs, not evaluated quota. Presence flags preserve
// explicit zero and empty expressions. Values returned to consumers are copies.
type ModelPricing struct {
	Mode                 string
	Expression           string
	HasExpression        bool
	ExpressionVersion    int
	Price                float64
	HasPrice             bool
	DefaultPrice         float64
	HasDefaultPrice      bool
	Ratio                float64
	HasRatio             bool
	RatioMatch           string
	CompletionRatio      float64
	CacheRatio           float64
	CacheCreationRatio   float64
	ImageRatio           float64
	AudioRatio           float64
	AudioCompletionRatio float64
	Money                billingmoney.Context
}

// RequestPricingSelection owns one catalog generation. The private map and
// immutable tool index are never exposed; zero-valued entries never read live
// settings. Image/task consumers may reuse the direct table copies and named
// anchors, but must establish their attempt's complete identities before capture.
type RequestPricingSelection struct {
	models   map[string]ModelPricing
	tools    operation_setting.ToolPrices
	grok     ratio_setting.MoliiGrokPriceSetting
	seedance ratio_setting.StarAIVideoPriceSetting
}

func NewRequestPricingSelection(models map[string]ModelPricing, tools operation_setting.ToolPrices, grok ratio_setting.MoliiGrokPriceSetting, seedance ratio_setting.StarAIVideoPriceSetting) *RequestPricingSelection {
	return &RequestPricingSelection{models: maps.Clone(models), tools: tools, grok: grok, seedance: seedance}
}

func (s *RequestPricingSelection) Model(name string) (ModelPricing, bool) {
	if s == nil {
		return ModelPricing{}, false
	}
	value, ok := s.models[name]
	return value, ok
}

func (s *RequestPricingSelection) ToolPrice(name, model string) float64 {
	return s.tools.PriceForModel(name, model)
}

func (s *RequestPricingSelection) GrokPrices() ratio_setting.MoliiGrokPriceSetting { return s.grok }
func (s *RequestPricingSelection) SeedancePrices() ratio_setting.StarAIVideoPriceSetting {
	return s.seedance
}

// ToolPrice uses the request's selection even when its lookup returns zero.
// The legacy fallback is for contexts that have never selected request pricing.
func (info *RelayInfo) ToolPrice(name, model string) float64 {
	if info != nil && info.PricingSelection != nil {
		return info.PricingSelection.ToolPrice(name, model)
	}
	return operation_setting.GetToolPriceForModel(name, model)
}
