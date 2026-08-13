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

func reconcileCatalogVendor(tx *gorm.DB, vendorName string, summary *CatalogReconcileSummary) (int, error) {
	if vendorName == "" {
		return 0, nil
	}
	var vendor Vendor
	err := tx.Unscoped().Where("name = ?", vendorName).First(&vendor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		profile, _ := GetCatalogVendorProfile(vendorName)
		now := common.GetTimestamp()
		vendor = Vendor{
			Name: vendorName, Description: profile.Description, Icon: getDefaultVendorIcon(vendorName),
			Status: 1, CreatedTime: now, UpdatedTime: now,
		}
		if err := tx.Create(&vendor).Error; err != nil {
			return 0, err
		}
		summary.CreatedVendors++
		return vendor.Id, nil
	}
	if err != nil {
		return 0, err
	}
	if vendor.DeletedAt.Valid {
		return 0, nil
	}
	profile, ok := GetCatalogVendorProfile(vendorName)
	if !ok {
		return vendor.Id, nil
	}
	updates := map[string]any{}
	if strings.TrimSpace(vendor.Icon) == "" && profile.Icon != "" {
		updates["icon"] = profile.Icon
	}
	if strings.TrimSpace(vendor.Description) == "" && profile.Description != "" {
		updates["description"] = profile.Description
	}
	if len(updates) == 0 {
		return vendor.Id, nil
	}
	updates["updated_time"] = common.GetTimestamp()
	if err := tx.Model(&Vendor{}).Where("id = ?", vendor.Id).Updates(updates).Error; err != nil {
		return 0, err
	}
	summary.UpdatedVendors++
	return vendor.Id, nil
}

func newCatalogModel(modelName string, vendorID int, profile CatalogModelProfile, known bool) Model {
	now := common.GetTimestamp()
	entry := Model{
		ModelName: modelName, VendorID: vendorID, Status: 1, SyncOfficial: 1,
		NameRule: NameRuleExact, CreatedTime: now, UpdatedTime: now,
	}
	if known {
		entry.Description = profile.Description
		entry.Icon = profile.Icon
		entry.Tags = profile.Tags
		entry.ContextLength = profile.ContextLength
		entry.MaxOutputTokens = profile.MaxOutputTokens
		entry.KnowledgeCutoff = profile.KnowledgeCutoff
		entry.ReleaseDate = profile.ReleaseDate
		entry.InputModalities = append([]string(nil), profile.InputModalities...)
		entry.OutputModalities = append([]string(nil), profile.OutputModalities...)
		entry.Capabilities = append([]string(nil), profile.Capabilities...)
		entry.MetadataSource = profile.MetadataSource
		entry.MetadataVerifiedAt = profile.MetadataVerifiedAt
	}
	return entry
}

func fillCatalogModelBlanks(entry *Model, vendorID int, profile CatalogModelProfile, known bool) map[string]any {
	updates := map[string]any{}
	if entry.VendorID == 0 && vendorID != 0 {
		updates["vendor_id"] = vendorID
	}
	if !known {
		return updates
	}
	if strings.TrimSpace(entry.Description) == "" && profile.Description != "" {
		updates["description"] = profile.Description
	}
	if strings.TrimSpace(entry.Icon) == "" && profile.Icon != "" {
		updates["icon"] = profile.Icon
	}
	if strings.TrimSpace(entry.Tags) == "" && profile.Tags != "" {
		updates["tags"] = profile.Tags
	}
	if entry.ContextLength == 0 && profile.ContextLength != 0 {
		updates["context_length"] = profile.ContextLength
	}
	if entry.MaxOutputTokens == 0 && profile.MaxOutputTokens != 0 {
		updates["max_output_tokens"] = profile.MaxOutputTokens
	}
	if strings.TrimSpace(entry.KnowledgeCutoff) == "" && profile.KnowledgeCutoff != "" {
		updates["knowledge_cutoff"] = profile.KnowledgeCutoff
	}
	if strings.TrimSpace(entry.ReleaseDate) == "" && profile.ReleaseDate != "" {
		updates["release_date"] = profile.ReleaseDate
	}
	if len(entry.InputModalities) == 0 && len(profile.InputModalities) != 0 {
		updates["input_modalities"] = catalogStringSliceJSON(profile.InputModalities)
	}
	if len(entry.OutputModalities) == 0 && len(profile.OutputModalities) != 0 {
		updates["output_modalities"] = catalogStringSliceJSON(profile.OutputModalities)
	}
	if len(entry.Capabilities) == 0 && len(profile.Capabilities) != 0 {
		updates["capabilities"] = catalogStringSliceJSON(profile.Capabilities)
	}
	if strings.TrimSpace(entry.MetadataSource) == "" && profile.MetadataSource != "" {
		updates["metadata_source"] = profile.MetadataSource
	}
	if strings.TrimSpace(entry.MetadataVerifiedAt) == "" && profile.MetadataVerifiedAt != "" {
		updates["metadata_verified_at"] = profile.MetadataVerifiedAt
	}
	return updates
}

func catalogStringSliceJSON(values []string) string {
	encoded, _ := common.Marshal(values)
	return string(encoded)
}

func ReconcileEnabledModelMetadata() (CatalogReconcileSummary, error) {
	summary := CatalogReconcileSummary{}
	err := DB.Transaction(func(tx *gorm.DB) error {
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
			profile, known := GetCatalogModelProfile(modelName)
			vendorName := profile.VendorName
			if vendorName == "" {
				vendorName = InferCatalogVendorName(modelName)
			}
			vendorID, err := reconcileCatalogVendor(tx, vendorName, &summary)
			if err != nil {
				return err
			}

			var entry Model
			err = tx.Unscoped().Where("model_name = ?", modelName).First(&entry).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				entry = newCatalogModel(modelName, vendorID, profile, known)
				if err := tx.Create(&entry).Error; err != nil {
					return err
				}
				summary.CreatedModels++
				continue
			}
			if err != nil {
				return err
			}
			if entry.DeletedAt.Valid {
				continue
			}
			updates := fillCatalogModelBlanks(&entry, vendorID, profile, known)
			if len(updates) == 0 {
				continue
			}
			updates["updated_time"] = common.GetTimestamp()
			if err := tx.Model(&Model{}).Where("id = ?", entry.Id).Updates(updates).Error; err != nil {
				return err
			}
			summary.UpdatedModels++
		}
		return nil
	})
	return summary, err
}
