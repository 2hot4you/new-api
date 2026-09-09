package model

import (
	"errors"

	"gorm.io/gorm"
)

const modelBillingCurrencyMigrationKey = "migration.model_billing_currency.v1"

var legacyCNYCatalogModels = []string{
	"minimax-m3",
	"qwen3.5-flash",
	"qwen3.5-plus",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
	"doubao-seedance-2-0-mini-260615",
	"doubao-seedance-2-5-260628",
	"grok-imagine-image",
	"grok-imagine-image-quality",
	"grok-imagine-image-2.0",
	"grok-imagine-video",
	"grok-imagine-video-1.5",
}

// migrateModelBillingCurrency preserves the currency previously implied by
// hard-coded catalog rules. After this one-time backfill, model metadata is the
// only source of truth and administrators may freely change the value.
func migrateModelBillingCurrency(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var marker Option
		err := tx.Where("key = ?", modelBillingCurrencyMigrationKey).First(&marker).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Model(&Model{}).
			Where("model_name IN ?", legacyCNYCatalogModels).
			Update("billing_currency", "CNY").Error; err != nil {
			return err
		}
		return tx.Create(&Option{Key: modelBillingCurrencyMigrationKey, Value: "completed"}).Error
	})
}
