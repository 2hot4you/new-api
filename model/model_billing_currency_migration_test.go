package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMigrateModelBillingCurrencyBackfillsOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "billing-currency.db")), &gorm.Config{})
	require.NoError(t, err)
	testMigrateModelBillingCurrencyBackfillsOnce(t, db)
}

func TestMigrateModelBillingCurrencyBackfillsOncePostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MODEL_BILLING_CURRENCY_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("MODEL_BILLING_CURRENCY_POSTGRES_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&Model{}, &Option{}))
	t.Cleanup(func() {
		require.NoError(t, db.Migrator().DropTable(&Model{}, &Option{}))
	})
	testMigrateModelBillingCurrencyBackfillsOnce(t, db)
}

func testMigrateModelBillingCurrencyBackfillsOnce(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&Option{}, &Model{}))

	models := []Model{
		{ModelName: "minimax-m3", BillingCurrency: "USD"},
		{ModelName: "ordinary-model", BillingCurrency: "USD"},
	}
	require.NoError(t, db.Create(&models).Error)
	require.NoError(t, migrateModelBillingCurrency(db))

	var migrated Model
	require.NoError(t, db.Where("model_name = ?", "minimax-m3").First(&migrated).Error)
	require.Equal(t, "CNY", migrated.BillingCurrency)
	var ordinary Model
	require.NoError(t, db.Where("model_name = ?", "ordinary-model").First(&ordinary).Error)
	require.Equal(t, "USD", ordinary.BillingCurrency)

	require.NoError(t, db.Model(&migrated).Update("billing_currency", "USD").Error)
	require.NoError(t, migrateModelBillingCurrency(db))
	require.NoError(t, db.Where("model_name = ?", "minimax-m3").First(&migrated).Error)
	require.Equal(t, "USD", migrated.BillingCurrency, "a completed migration must not overwrite an administrator edit")
}
