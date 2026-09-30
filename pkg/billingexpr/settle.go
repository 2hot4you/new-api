package billingexpr

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingmoney"
)

// ExpressionAmounts converts raw expression output into an actual monetary
// amount in both the frozen source currency and the internal USD ledger unit.
func ExpressionAmounts(exprOutput float64, snap *BillingSnapshot) (billingmoney.Amounts, error) {
	sourceCost := exprOutput
	if snap.TaskUsageBilling {
		sourceCost = exprOutput
	} else {
		switch snap.ExprVersion {
		default: // v1: coefficients are source-currency prices per 1M tokens.
			sourceCost = exprOutput / 1_000_000
		}
	}
	sourceCurrency := snap.SourceCurrency
	if sourceCurrency == "" {
		sourceCurrency = string(billingmoney.CurrencyUSD)
	}
	ctx, err := billingmoney.NewContext(sourceCurrency, snap.CNYPerUSD)
	if err != nil {
		return billingmoney.Amounts{}, err
	}
	return ctx.Normalize(sourceCost)
}

// ComputeTieredQuota runs the Expr from a frozen BillingSnapshot against
// actual token counts and returns the settlement result.
func ComputeTieredQuota(snap *BillingSnapshot, params TokenParams) (TieredResult, error) {
	return ComputeTieredQuotaWithRequest(snap, params, RequestInput{})
}

func ComputeTieredQuotaWithRequest(snap *BillingSnapshot, params TokenParams, request RequestInput) (TieredResult, error) {
	if snap.TaskUsageBilling && UsesFixedPricingByHash(snap.ExprString, snap.ExprHash) {
		return TieredResult{}, fmt.Errorf("fixed pricing is not supported for task usage expressions")
	}
	cost, trace, err := RunExprByHashWithRequest(snap.ExprString, snap.ExprHash, params, request)
	if err != nil {
		return TieredResult{}, err
	}

	amounts, err := ExpressionAmounts(cost, snap)
	if err != nil {
		return TieredResult{}, err
	}
	quotaBeforeGroup := amounts.CostUSD * snap.QuotaPerUnit
	afterGroup, clamp := common.QuotaRoundChecked(quotaBeforeGroup * snap.GroupRatio)
	crossed := trace.MatchedTier != snap.EstimatedTier

	result := TieredResult{
		ImageCount:             trace.ImageCount,
		BillingUnit:            trace.BillingUnit,
		FixedPrice:             trace.FixedPrice,
		ActualQuotaBeforeGroup: quotaBeforeGroup,
		ActualQuotaAfterGroup:  afterGroup,
		ActualSourceCost:       amounts.SourceCost,
		ActualCostUSD:          amounts.CostUSD,
		MatchedTier:            trace.MatchedTier,
		RequestRules:           trace.RequestRules,
		CrossedTier:            crossed,
		Clamp:                  clamp,
	}
	if trace.BillingUnit == BillingUnitToken && UsedVarsByHash(snap.ExprString, snap.ExprHash)["img_cr"] {
		result.BillingTokens = &params
	}
	return result, nil
}

// ApplyResultToSnapshot promotes settled values into the durable snapshot so
// later audit logs describe the actual charge while retaining the frozen
// currency and exchange-rate context.
func ApplyResultToSnapshot(snap *BillingSnapshot, result TieredResult) {
	if snap == nil {
		return
	}
	snap.EstimatedImageCount = result.ImageCount
	snap.EstimatedQuotaBeforeGroup = result.ActualQuotaBeforeGroup
	snap.EstimatedQuotaAfterGroup = result.ActualQuotaAfterGroup
	snap.EstimatedSourceCost = result.ActualSourceCost
	snap.EstimatedCostUSD = result.ActualCostUSD
	snap.EstimatedTier = result.MatchedTier
	snap.EstimatedBillingUnit = result.BillingUnit
	snap.EstimatedFixedPrice = result.FixedPrice
}
