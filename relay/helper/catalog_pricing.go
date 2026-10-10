package helper

import (
	"fmt"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// CaptureRequestPricing starts an explicit attempt selection. Candidate names
// are derived without inspecting mutable prices, then resolved inside one read
// boundary. It performs no body access, expression evaluation or provider work.
// Helpers reuse an existing selection; image/task orchestration may explicitly
// call this again only when beginning a newly selected attempt.
func CaptureRequestPricing(c *gin.Context, info *relaycommon.RelayInfo) error {
	if info == nil || c == nil || c.Request == nil {
		return fmt.Errorf("request pricing requires a request context")
	}
	names := make([]string, 0)
	seen := make(map[string]bool)
	for _, identity := range []string{info.GetOriginModelName(), info.GetBillingModelName(), info.GetUpstreamModelName()} {
		if identity == "" {
			continue
		}
		for _, name := range append([]string{identity}, billingModelCandidates(identity)...) {
			if name != "" && !seen[name] {
				names = append(names, name)
				seen[name] = true
			}
		}
	}
	var selected *relaycommon.RequestPricingSelection
	var billingName string
	err := model.WithCatalogPricingReads(c.Request.Context(), names, func() error {
		billingName = info.GetBillingModelName()
		if info.BillingModelName == "" {
			billingName = ResolveBillingModelName(info.GetOriginModelName())
		}
		values := make(map[string]relaycommon.ModelPricing, len(names))
		defaultPrices := ratio_setting.GetDefaultModelPriceMap()
		for _, name := range names {
			value := relaycommon.ModelPricing{Mode: billing_setting.GetBillingMode(name)}
			value.Expression, value.HasExpression = billing_setting.GetBillingExpr(name)
			value.ExpressionVersion = billingexpr.ExprVersion(value.Expression)
			value.Price, value.HasPrice = ratio_setting.GetModelPrice(name, false)
			value.DefaultPrice, value.HasDefaultPrice = defaultPrices[name]
			value.Ratio, value.HasRatio, value.RatioMatch = ratio_setting.GetModelRatio(name)
			value.CompletionRatio = ratio_setting.GetCompletionRatio(name)
			value.CacheRatio, _ = ratio_setting.GetCacheRatio(name)
			value.CacheCreationRatio, _ = ratio_setting.GetCreateCacheRatio(name)
			value.ImageRatio, _ = ratio_setting.GetImageRatio(name)
			value.AudioRatio = ratio_setting.GetAudioRatio(name)
			value.AudioCompletionRatio = ratio_setting.GetAudioCompletionRatio(name)
			money, _, err := model.ResolveBillingMoneyContext(model.DB.WithContext(c.Request.Context()), name)
			if err != nil {
				return fmt.Errorf("model %s billing currency resolution failed: %w", name, err)
			}
			value.Money = money
			values[name] = value
		}
		if _, ok := values[billingName]; !ok {
			return fmt.Errorf("billing identity %q was not guarded", billingName)
		}
		selected = relaycommon.NewRequestPricingSelection(values, operation_setting.CaptureToolPrices(), ratio_setting.GetMoliiGrokPriceSettingCopy(), ratio_setting.GetStarAIVideoPriceSettingCopy())
		return nil
	})
	if err != nil {
		return err
	}
	if info.BillingModelName != "" || billingName != info.OriginModelName {
		info.BillingModelName = billingName
	}
	info.PricingSelection = selected
	return nil
}

func selectedModelPricing(c *gin.Context, info *relaycommon.RelayInfo) (relaycommon.ModelPricing, error) {
	if info.PricingSelection == nil {
		if err := CaptureRequestPricing(c, info); err != nil {
			return relaycommon.ModelPricing{}, err
		}
	}
	price, ok := info.PricingSelection.Model(info.GetBillingModelName())
	if !ok {
		return relaycommon.ModelPricing{}, fmt.Errorf("model %s is outside the request pricing selection", info.GetBillingModelName())
	}
	return price, nil
}
