package catalogsync

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const catalogSyncAdvisoryLockID int64 = 742173944822463563

type ApplyResult struct {
	ConfirmationDigest string      `json:"confirmation_digest"`
	BackupDigest       string      `json:"backup_digest"`
	Summary            PlanSummary `json:"summary"`
}

func Apply(ctx context.Context, db *gorm.DB, snapshot Snapshot, expectedDigest string, backupWriter io.Writer) (ApplyResult, error) {
	if expectedDigest == "" {
		return ApplyResult{}, fmt.Errorf("confirmation digest is required")
	}
	if backupWriter == nil {
		return ApplyResult{}, fmt.Errorf("backup writer is required")
	}

	var result ApplyResult
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() != "postgres" {
			return fmt.Errorf("catalog sync apply supports PostgreSQL only")
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", catalogSyncAdvisoryLockID).Error; err != nil {
			return fmt.Errorf("acquire catalog sync lock: %w", err)
		}
		plan, err := BuildPlan(ctx, tx, snapshot)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare([]byte(plan.ConfirmationDigest), []byte(expectedDigest)) != 1 {
			return fmt.Errorf("confirmation digest no longer matches target state; run plan again")
		}
		backup, err := Export(ctx, tx)
		if err != nil {
			return fmt.Errorf("create backup snapshot: %w", err)
		}
		if err := WriteSnapshot(backupWriter, backup); err != nil {
			return fmt.Errorf("write backup: %w", err)
		}
		if syncWriter, ok := backupWriter.(interface{ Sync() error }); ok {
			if err := syncWriter.Sync(); err != nil {
				return fmt.Errorf("sync backup: %w", err)
			}
		}
		if err := applyPlan(tx, plan); err != nil {
			return err
		}
		result = ApplyResult{
			ConfirmationDigest: plan.ConfirmationDigest,
			BackupDigest:       backup.ContentDigest,
			Summary:            plan.Summary,
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ApplyResult{}, err
	}
	return result, nil
}

func applyPlan(tx *gorm.DB, plan Plan) error {
	vendorIDs := make(map[string]int)
	for _, change := range plan.Vendors {
		if err := applyVendorChange(tx, change); err != nil {
			return err
		}
	}
	var vendors []model.Vendor
	if err := tx.Find(&vendors).Error; err != nil {
		return fmt.Errorf("reload target vendors: %w", err)
	}
	for _, vendor := range vendors {
		vendorIDs[vendor.Name] = vendor.Id
	}
	for _, change := range plan.Models {
		vendorID := 0
		if change.After.Vendor != "" {
			var exists bool
			vendorID, exists = vendorIDs[change.After.Vendor]
			if !exists {
				return fmt.Errorf("target vendor %q is unavailable for model %q", change.After.Vendor, change.ModelName)
			}
		}
		if err := applyModelChange(tx, change, vendorID); err != nil {
			return err
		}
	}
	for _, change := range plan.Options {
		switch change.Action {
		case ChangeCreate, ChangeUpdate:
			if change.After == nil {
				return fmt.Errorf("pricing option %q has no desired value", change.Key)
			}
			option := model.Option{Key: change.Key, Value: *change.After}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value"}),
			}).Create(&option).Error; err != nil {
				return fmt.Errorf("upsert pricing option %q: %w", change.Key, err)
			}
		case ChangeDelete:
			if err := tx.Where("key = ?", change.Key).Delete(&model.Option{}).Error; err != nil {
				return fmt.Errorf("clear stale pricing option %q: %w", change.Key, err)
			}
		default:
			return fmt.Errorf("unsupported pricing option action %q", change.Action)
		}
	}
	return nil
}

func applyVendorChange(tx *gorm.DB, change VendorChange) error {
	now := time.Now().Unix()
	values := map[string]any{
		"name": change.After.Name, "description": change.After.Description, "icon": change.After.Icon,
		"status": change.After.Status, "display_order": change.After.DisplayOrder, "updated_time": now,
	}
	switch change.Action {
	case ChangeCreate:
		entry := model.Vendor{
			Name: change.After.Name, Description: change.After.Description, Icon: change.After.Icon,
			Status: change.After.Status, DisplayOrder: change.After.DisplayOrder, CreatedTime: now, UpdatedTime: now,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("create vendor %q: %w", change.Name, err)
		}
		values["status"] = change.After.Status
		if err := tx.Model(&model.Vendor{}).Where("id = ?", entry.Id).Updates(values).Error; err != nil {
			return fmt.Errorf("normalize created vendor %q: %w", change.Name, err)
		}
	case ChangeUpdate:
		result := tx.Model(&model.Vendor{}).Where("name = ?", change.Name).Updates(values)
		if result.Error != nil {
			return fmt.Errorf("update vendor %q: %w", change.Name, result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update vendor %q: target changed", change.Name)
		}
	default:
		return fmt.Errorf("unsupported vendor action %q", change.Action)
	}
	return nil
}

func applyModelChange(tx *gorm.DB, change ModelChange, vendorID int) error {
	now := time.Now().Unix()
	entry := modelFromRecord(change.After, vendorID)
	entry.UpdatedTime = now
	columns := []string{
		"model_name", "display_name", "description", "description_en", "icon", "tags", "vendor_id",
		"billing_currency", "endpoints", "status", "sync_official", "name_rule", "context_length",
		"max_output_tokens", "knowledge_cutoff", "release_date", "input_modalities", "output_modalities",
		"capabilities", "metadata_source", "metadata_verified_at", "marketplace_enabled", "display_order",
		"supported_parameters", "supported_resolutions", "supported_aspect_ratios", "max_input_images",
		"output_formats", "min_duration", "max_duration", "reference_modalities", "updated_time",
	}
	switch change.Action {
	case ChangeCreate:
		entry.CreatedTime = now
		if err := tx.Create(&entry).Error; err != nil {
			return fmt.Errorf("create model %q: %w", change.ModelName, err)
		}
		createdID := entry.Id
		entry = modelFromRecord(change.After, vendorID)
		entry.Id = createdID
		entry.CreatedTime = now
		entry.UpdatedTime = now
		if err := tx.Model(&model.Model{}).Where("id = ?", entry.Id).Select(columns).Updates(&entry).Error; err != nil {
			return fmt.Errorf("normalize created model %q: %w", change.ModelName, err)
		}
	case ChangeUpdate:
		result := tx.Model(&model.Model{}).Where("model_name = ?", change.ModelName).Select(columns).Updates(&entry)
		if result.Error != nil {
			return fmt.Errorf("update model %q: %w", change.ModelName, result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update model %q: target changed", change.ModelName)
		}
	default:
		return fmt.Errorf("unsupported model action %q", change.Action)
	}
	return nil
}

func modelFromRecord(record ModelRecord, vendorID int) model.Model {
	return model.Model{
		ModelName: record.ModelName, DisplayName: record.DisplayName, Description: record.Description,
		DescriptionEN: record.DescriptionEN, Icon: record.Icon, Tags: record.Tags, VendorID: vendorID,
		BillingCurrency: record.BillingCurrency, Endpoints: record.Endpoints, Status: record.Status,
		SyncOfficial: record.SyncOfficial, NameRule: record.NameRule, ContextLength: record.ContextLength,
		MaxOutputTokens: record.MaxOutputTokens, KnowledgeCutoff: record.KnowledgeCutoff, ReleaseDate: record.ReleaseDate,
		InputModalities: record.InputModalities, OutputModalities: record.OutputModalities,
		Capabilities: record.Capabilities, MetadataSource: record.MetadataSource,
		MetadataVerifiedAt: record.MetadataVerifiedAt, MarketplaceEnabled: record.MarketplaceEnabled,
		DisplayOrder: record.DisplayOrder, SupportedParameters: record.SupportedParameters,
		SupportedResolutions: record.SupportedResolutions, SupportedAspectRatios: record.SupportedAspectRatios,
		MaxInputImages: record.MaxInputImages, OutputFormats: record.OutputFormats, MinDuration: record.MinDuration,
		MaxDuration: record.MaxDuration, ReferenceModalities: record.ReferenceModalities,
	}
}
