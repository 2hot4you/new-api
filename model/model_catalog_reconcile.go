package model

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"gorm.io/gorm"
)

type CatalogReconcileSummary struct {
	CreatedModels  int `json:"created_models"`
	UpdatedModels  int `json:"updated_models"`
	CreatedVendors int `json:"created_vendors"`
	UpdatedVendors int `json:"updated_vendors"`
}

type catalogReconcileEpoch struct {
	db          *gorm.DB
	fingerprint string
}

var catalogReconciledEpoch atomic.Pointer[catalogReconcileEpoch]

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
	return reconcileEnabledModelMetadata(context.Background(), false)
}

func reconcileEnabledModelMetadata(ctx context.Context, cancellable bool) (CatalogReconcileSummary, error) {
	summary := CatalogReconcileSummary{}
	if cached := catalogReconciledEpoch.Load(); cancellable && cached != nil && cached.db == DB {
		unchanged := false
		err := tryWithCatalogCaptureBarrier(ctx, func() (captureErr error) {
			activeDB := DB
			defer func() {
				if captureErr != nil {
					catalogRuntimeInvalid.Store(activeDB)
				}
			}()
			return catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
				if state.PublicationState != "ready" || state.Revision != state.RuntimeRevision || state.PendingOperationID != "" {
					return ErrCatalogPublicationPending
				}
				input, err := captureCatalogRuntimeTx(tx, state, pin)
				if err != nil {
					return err
				}
				unchanged = input.fingerprint == state.RuntimeFingerprint && input.fingerprint == cached.fingerprint
				return nil
			})
		})
		if err != nil {
			return summary, err
		}
		if unchanged {
			return summary, nil
		}
	}
	transaction := withMarketplaceOrderTransactionEpoch
	if cancellable {
		transaction = func(db *gorm.DB, fn func(*gorm.DB) error) (*catalogReconcileEpoch, error) {
			return withMarketplaceOrderTransactionContextEpoch(ctx, db, fn)
		}
	}
	epoch, err := transaction(DB, func(tx *gorm.DB) error {
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
		officialModelNames := make([]string, 0, len(modelNames))
		for _, modelName := range modelNames {
			if localMarketplaceSeedVendorNames[modelName] != "" {
				officialModelNames = append(officialModelNames, modelName)
			}
		}
		if len(officialModelNames) == 0 {
			return nil
		}
		return backfillLocalMarketplaceMetadata(tx, officialModelNames)
	})
	if err == nil && epoch != nil {
		catalogReconciledEpoch.Store(epoch)
	}
	return summary, err
}
