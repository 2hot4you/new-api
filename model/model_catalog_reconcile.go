package model

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type CatalogReconcileSummary struct {
	CreatedModels  int `json:"created_models"`
	UpdatedModels  int `json:"updated_models"`
	CreatedVendors int `json:"created_vendors"`
	UpdatedVendors int `json:"updated_vendors"`
}

func enabledCatalogModelNames(tx *gorm.DB) ([]string, error) {
	var names []string
	err := tx.Table("abilities").
		Select("DISTINCT abilities.model").
		Joins("JOIN channels ON channels.id = abilities.channel_id").
		Where("abilities.enabled = ? AND channels.status = ?", true, common.ChannelStatusEnabled).
		Order("abilities.model ASC").
		Pluck("abilities.model", &names).Error
	return names, err
}

func newCatalogModel(modelName string) Model {
	now := common.GetTimestamp()
	return Model{
		ModelName: modelName, DisplayName: modelName, Status: 1, SyncOfficial: 1,
		NameRule: NameRuleExact, CreatedTime: now, UpdatedTime: now,
	}
}

func ReconcileEnabledModelMetadata() (CatalogReconcileSummary, error) {
	summary := CatalogReconcileSummary{}
	err := withMarketplaceOrderTransaction(DB, func(tx *gorm.DB) error {
		modelNames, err := enabledCatalogModelNames(tx)
		if err != nil {
			return err
		}
		sort.Strings(modelNames)
		for _, rawModelName := range modelNames {
			modelName := strings.TrimSpace(rawModelName)
			if modelName == "" || modelName == "grok-imagine-video-1.5-preview" {
				continue
			}
			var entry Model
			entryErr := tx.Unscoped().Where("model_name = ?", modelName).First(&entry).Error
			if entryErr != nil && !errors.Is(entryErr, gorm.ErrRecordNotFound) {
				return entryErr
			}
			if entryErr == nil && (entry.DeletedAt.Valid || entry.SyncOfficial == 0) {
				continue
			}

			if errors.Is(entryErr, gorm.ErrRecordNotFound) {
				entry = newCatalogModel(modelName)
				if err := createModelWithNextDisplayOrder(tx, &entry); err != nil {
					return err
				}
				summary.CreatedModels++
				continue
			}
		}
		return nil
	})
	return summary, err
}
