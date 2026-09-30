package model

import (
	"fmt"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/pkg/billingmoney"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

type ModelBillingCurrency struct {
	BillingCurrency billingmoney.Currency
	HasMetadata     bool
}

func ResolveBillingMoneyContext(db *gorm.DB, modelName string) (billingmoney.Context, bool, error) {
	currencies, err := LoadModelBillingCurrencies(db, []string{modelName}, false)
	if err != nil {
		return billingmoney.Context{}, false, err
	}
	resolved := currencies[modelName]
	ctx, err := billingmoney.NewContext(string(resolved.BillingCurrency), operation_setting.USDExchangeRate)
	if err != nil {
		return billingmoney.Context{}, resolved.HasMetadata, fmt.Errorf("model %s billing currency: %w", modelName, err)
	}
	return ctx, resolved.HasMetadata, nil
}

func LoadModelBillingCurrencies(db *gorm.DB, modelNames []string, forUpdate bool) (map[string]ModelBillingCurrency, error) {
	names := make([]string, 0, len(modelNames))
	seen := make(map[string]struct{}, len(modelNames))
	for _, name := range modelNames {
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	slices.Sort(names)

	result := make(map[string]ModelBillingCurrency, len(names))
	for _, name := range names {
		result[name] = ModelBillingCurrency{BillingCurrency: billingmoney.CurrencyUSD}
	}
	if len(names) == 0 {
		return result, nil
	}

	var records []Model
	query := db.Select("model_name", "billing_currency", "name_rule").
		Where("model_name IN ? AND name_rule = ?", names, NameRuleExact).
		Order("model_name")
	if forUpdate {
		query = lockForUpdate(query)
	}
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	for _, record := range records {
		currency, err := normalizeModelBillingCurrency(record.BillingCurrency)
		if err != nil {
			return nil, fmt.Errorf("model %s: %w", record.ModelName, err)
		}
		result[record.ModelName] = ModelBillingCurrency{BillingCurrency: currency, HasMetadata: true}
	}
	return result, nil
}

func normalizeModelBillingCurrency(value string) (billingmoney.Currency, error) {
	currency := billingmoney.Currency(strings.ToUpper(strings.TrimSpace(value)))
	if currency == "" {
		currency = billingmoney.CurrencyUSD
	}
	if currency != billingmoney.CurrencyUSD && currency != billingmoney.CurrencyCNY {
		return "", fmt.Errorf("billing_currency must be USD or CNY")
	}
	return currency, nil
}
