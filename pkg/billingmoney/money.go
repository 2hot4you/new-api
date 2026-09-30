package billingmoney

import (
	"fmt"
	"math"
	"strings"
)

type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyCNY Currency = "CNY"
)

type Context struct {
	SourceCurrency Currency `json:"source_currency"`
	CNYPerUSD      float64  `json:"cny_per_usd"`
}

type Amounts struct {
	SourceCost float64 `json:"source_cost"`
	CostUSD    float64 `json:"cost_usd"`
}

func NewContext(source string, cnyPerUSD float64) (Context, error) {
	ctx := Context{
		SourceCurrency: Currency(strings.ToUpper(strings.TrimSpace(source))),
		CNYPerUSD:      cnyPerUSD,
	}
	if err := ctx.validate(); err != nil {
		return Context{}, err
	}
	return ctx, nil
}

func (ctx Context) Normalize(sourceCost float64) (Amounts, error) {
	if err := ctx.validate(); err != nil {
		return Amounts{}, err
	}
	if sourceCost < 0 || math.IsNaN(sourceCost) || math.IsInf(sourceCost, 0) {
		return Amounts{}, fmt.Errorf("source cost must be a finite, non-negative number")
	}
	costUSD := sourceCost
	if ctx.SourceCurrency == CurrencyCNY {
		costUSD = sourceCost / ctx.CNYPerUSD
	}
	return Amounts{SourceCost: sourceCost, CostUSD: costUSD}, nil
}

func (ctx Context) validate() error {
	switch ctx.SourceCurrency {
	case CurrencyUSD:
		return nil
	case CurrencyCNY:
		if ctx.CNYPerUSD <= 0 || math.IsNaN(ctx.CNYPerUSD) || math.IsInf(ctx.CNYPerUSD, 0) {
			return fmt.Errorf("CNY per USD must be a finite, positive number")
		}
		return nil
	default:
		return fmt.Errorf("source currency must be USD or CNY")
	}
}
