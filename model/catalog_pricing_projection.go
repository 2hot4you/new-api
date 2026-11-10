package model

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// A complete detached display projection. Building this may fail; installing
// it cannot. All corresponding reader locks are acquired before installation.
type catalogPricingProjection struct {
	prices    []Pricing
	vendors   []PricingVendor
	endpoints map[string][]constant.EndpointType
	supported map[string]common.EndpointInfo
	groups    map[string][]string
	quota     map[string]int
}

func (p *catalogPricingProjection) install() {
	pricingMap, vendorsList = p.prices, p.vendors
	modelSupportEndpointTypes, supportedEndpointMap = p.endpoints, p.supported
	modelEnableGroups, modelQuotaTypeMap = p.groups, p.quota
	lastGetPricingTime = time.Now()
}

// A nil candidate retains the ordinary lazy-refresh semantics. Lifecycle
// projections use only their detached candidate, never already-swapped globals.
type catalogProjectionPrices struct{ candidate *catalogPricingCandidate }

func (p catalogProjectionPrices) ratio(key, name string) (float64, bool) {
	if p.candidate != nil {
		if key == "ModelPrice" || key == "ModelRatio" || key == "CompletionRatio" || key == "AudioRatio" || key == "AudioCompletionRatio" {
			name = ratio_setting.FormatMatchingModelName(name)
		}
		value, ok := p.candidate.ratios[key][name]
		if ok {
			return value, true
		}
		switch key {
		case "ModelPrice":
			return -1, false
		case "ModelRatio":
			return 37.5, operation_setting.SelfUseModeEnabled
		case "CacheRatio":
			return ratio_setting.DefaultCacheRatio, false
		case "CreateCacheRatio":
			return ratio_setting.DefaultCreateCacheRatio, false
		default:
			return 1, false
		}
	}
	switch key {
	case "ModelPrice":
		return ratio_setting.GetModelPrice(name, false)
	case "ModelRatio":
		value, ok, _ := ratio_setting.GetModelRatio(name)
		return value, ok
	case "CacheRatio":
		return ratio_setting.GetCacheRatio(name)
	case "CreateCacheRatio":
		return ratio_setting.GetCreateCacheRatio(name)
	case "ImageRatio":
		return ratio_setting.GetImageRatio(name)
	case "AudioRatio":
		return ratio_setting.GetAudioRatio(name), ratio_setting.ContainsAudioRatio(name)
	case "AudioCompletionRatio":
		return ratio_setting.GetAudioCompletionRatio(name), ratio_setting.ContainsAudioCompletionRatio(name)
	}
	return 0, false
}

func (p catalogProjectionPrices) configuredRatio(name string) bool {
	if p.candidate == nil {
		return ratio_setting.HasConfiguredModelRatio(name)
	}
	_, ok := p.candidate.ratios["ModelRatio"][ratio_setting.FormatMatchingModelName(name)]
	return ok
}

func (p catalogProjectionPrices) mode(name string) string {
	if p.candidate == nil {
		return billing_setting.GetBillingMode(name)
	}
	if mode, ok := p.candidate.billing.BillingMode[name]; ok {
		return mode
	}
	if _, builtin := billing_setting.GetBuiltinBillingExpr(name); builtin {
		_, price := p.ratio("ModelPrice", name)
		if !price && !p.configuredRatio(name) {
			return billing_setting.BillingModeTieredExpr
		}
	}
	return billing_setting.BillingModeRatio
}

func (p catalogProjectionPrices) expression(name string) (string, bool) {
	if p.candidate == nil {
		return billing_setting.GetBillingExpr(name)
	}
	if expr, ok := p.candidate.billing.BillingExpr[name]; ok {
		return expr, true
	}
	if p.mode(name) == billing_setting.BillingModeTieredExpr {
		return billing_setting.GetBuiltinBillingExpr(name)
	}
	return "", false
}

func (p catalogProjectionPrices) builtin(name string) bool {
	if p.candidate == nil {
		return billing_setting.IsBuiltinBillingExprFallback(name)
	}
	if _, configured := p.candidate.billing.BillingExpr[name]; configured {
		return false
	}
	_, exists := billing_setting.GetBuiltinBillingExpr(name)
	return exists && p.mode(name) == billing_setting.BillingModeTieredExpr
}

func (p catalogProjectionPrices) plugin(plugin, name string) (string, bool) {
	if p.candidate == nil {
		return billing_setting.GetPluginBillingExpr(plugin, name)
	}
	expr, ok := p.candidate.billing.PluginBillingExpr[billing_setting.PluginBillingExprKey(plugin, name)]
	return expr, ok
}

func (p catalogProjectionPrices) task(plugin, name string) (string, bool) {
	if expr, ok := p.plugin(plugin, name); ok {
		return expr, true
	}
	if p.mode(name) == billing_setting.BillingModeTieredExpr {
		return p.expression(name)
	}
	return "", false
}

func (p catalogProjectionPrices) configured(name string) bool {
	if p.candidate == nil {
		return IsModelPricingConfigured(name)
	}
	if _, price := p.ratio("ModelPrice", name); price {
		return true
	}
	if p.configuredRatio(name) {
		return true
	}
	if p.candidate != nil && strings.HasSuffix(name, ratio_setting.CompactModelSuffix) {
		if _, ok := p.candidate.ratios["ModelRatio"][ratio_setting.CompactWildcardModelKey]; ok {
			return true
		}
	}
	expr, ok := p.expression(name)
	return ok && strings.TrimSpace(expr) != ""
}

func (p catalogProjectionPrices) completion(name string) float64 {
	if p.candidate == nil {
		return ratio_setting.GetCompletionRatio(name)
	}
	var configured *float64
	if value, ok := p.candidate.ratios["CompletionRatio"][ratio_setting.FormatMatchingModelName(name)]; ok {
		configured = &value
	}
	return ratio_setting.ResolveCompletionRatio(name, configured).Ratio
}

func (p catalogProjectionPrices) video(name string) (*ratio_setting.StarAIVideoPricing, bool) {
	if p.candidate == nil {
		return ratio_setting.GetStarAIVideoPricing(name)
	}
	return p.candidate.video.CatalogPricing(name)
}

func (p catalogProjectionPrices) grok(name string) (*ratio_setting.MoliiGrokCatalogPricing, bool) {
	if p.candidate == nil {
		return ratio_setting.GetMoliiGrokCatalogPricing(name)
	}
	return p.candidate.grok.CatalogPricing(name)
}
