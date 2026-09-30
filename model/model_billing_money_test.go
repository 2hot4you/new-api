package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/pkg/billingmoney"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newModelBillingMoneyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "billing-money.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Model{}))
	return db
}

func TestResolveBillingMoneyContext(t *testing.T) {
	db := newModelBillingMoneyTestDB(t)
	previousRate := operation_setting.USDExchangeRate
	operation_setting.USDExchangeRate = 7
	t.Cleanup(func() { operation_setting.USDExchangeRate = previousRate })

	require.NoError(t, db.Create(&[]Model{
		{ModelName: "usd-model", NameRule: NameRuleExact, BillingCurrency: "USD"},
		{ModelName: "cny-model", NameRule: NameRuleExact, BillingCurrency: "CNY"},
		{ModelName: "prefix-model", NameRule: NameRulePrefix, BillingCurrency: "CNY"},
		{ModelName: "invalid-model", NameRule: NameRuleExact, BillingCurrency: "USD"},
	}).Error)
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "invalid-model").Update("billing_currency", "EUR").Error)

	t.Run("exact USD metadata", func(t *testing.T) {
		ctx, hasMetadata, err := ResolveBillingMoneyContext(db, "usd-model")
		require.NoError(t, err)
		assert.True(t, hasMetadata)
		assert.Equal(t, billingmoney.CurrencyUSD, ctx.SourceCurrency)
		assert.Equal(t, 7.0, ctx.CNYPerUSD)
	})

	t.Run("exact CNY metadata", func(t *testing.T) {
		ctx, hasMetadata, err := ResolveBillingMoneyContext(db, "cny-model")
		require.NoError(t, err)
		assert.True(t, hasMetadata)
		assert.Equal(t, billingmoney.CurrencyCNY, ctx.SourceCurrency)
		assert.Equal(t, 7.0, ctx.CNYPerUSD)
	})

	t.Run("missing metadata defaults to USD", func(t *testing.T) {
		ctx, hasMetadata, err := ResolveBillingMoneyContext(db, "missing-model")
		require.NoError(t, err)
		assert.False(t, hasMetadata)
		assert.Equal(t, billingmoney.CurrencyUSD, ctx.SourceCurrency)
	})

	t.Run("non exact metadata does not set billing currency", func(t *testing.T) {
		ctx, hasMetadata, err := ResolveBillingMoneyContext(db, "prefix-model")
		require.NoError(t, err)
		assert.False(t, hasMetadata)
		assert.Equal(t, billingmoney.CurrencyUSD, ctx.SourceCurrency)
	})

	t.Run("invalid stored currency fails closed", func(t *testing.T) {
		_, _, err := ResolveBillingMoneyContext(db, "invalid-model")
		require.ErrorContains(t, err, "billing_currency")
	})
}

func TestLoadModelBillingCurrencies(t *testing.T) {
	db := newModelBillingMoneyTestDB(t)
	require.NoError(t, db.Create(&[]Model{
		{ModelName: "alpha", NameRule: NameRuleExact, BillingCurrency: "CNY"},
		{ModelName: "beta", NameRule: NameRuleExact, BillingCurrency: "USD"},
		{ModelName: "gamma", NameRule: NameRulePrefix, BillingCurrency: "CNY"},
	}).Error)

	for _, forUpdate := range []bool{false, true} {
		t.Run(map[bool]string{false: "read", true: "locked"}[forUpdate], func(t *testing.T) {
			loaded, err := LoadModelBillingCurrencies(db, []string{"missing", "gamma", "beta", "alpha", "alpha"}, forUpdate)
			require.NoError(t, err)
			assert.Equal(t, map[string]ModelBillingCurrency{
				"alpha":   {BillingCurrency: billingmoney.CurrencyCNY, HasMetadata: true},
				"beta":    {BillingCurrency: billingmoney.CurrencyUSD, HasMetadata: true},
				"gamma":   {BillingCurrency: billingmoney.CurrencyUSD, HasMetadata: false},
				"missing": {BillingCurrency: billingmoney.CurrencyUSD, HasMetadata: false},
			}, loaded)
		})
	}
}
