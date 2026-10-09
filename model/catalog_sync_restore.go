package model

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"gorm.io/gorm"
)

func CreateCatalogSyncRestorePlan(ctx context.Context, operationID string, actor catalogmanifest.Actor, now time.Time) (catalogmanifest.Plan, error) {
	if operationID == "" {
		return catalogmanifest.Plan{}, ErrCatalogSyncPlanUnavailable
	}
	return prepareCatalogSyncPlan(ctx, catalogmanifest.Snapshot{}, actor, now, "", "", catalogmanifest.Resolution{}, operationID)
}

// A restore is constructed exclusively from the durable local preimages, never
// from previous source values. Snapshot carries the inverse values plus local
// dependencies; Changes alone defines the original operation's affected scope.
func catalogRestorePlanTx(tx *gorm.DB, operationID string, actor catalogmanifest.Actor, now time.Time, before catalogmanifest.Snapshot, base catalogmanifest.Baseline) (catalogmanifest.Plan, catalogOperationBackup, error) {
	var latest CatalogSyncOperation
	var backup catalogOperationBackup
	if err := tx.Where("state = ?", "succeeded").Order("revision DESC").Order("id DESC").First(&latest).Error; err != nil {
		return catalogmanifest.Plan{}, backup, ErrCatalogSyncPlanUnavailable
	}
	if latest.ID != operationID {
		return catalogmanifest.Plan{}, backup, ErrCatalogSyncPlanUnavailable
	}
	var binding catalogOperationBinding
	if common.UnmarshalJsonStr(string(latest.History), &binding) != nil || binding.Actor.TargetID != actor.TargetID || binding.PlanID != latest.PlanID || (binding.Kind != "sync" && binding.Kind != "restore") {
		return catalogmanifest.Plan{}, backup, ErrCatalogSyncPlanUnavailable
	}
	if common.UnmarshalJsonStr(string(latest.Backup), &backup) != nil || backup.Version != 1 || len(backup.Before) == 0 {
		return catalogmanifest.Plan{}, backup, ErrCatalogSyncPlanUnavailable
	}
	var originalRow CatalogSyncPlan
	if err := tx.First(&originalRow, "id = ?", latest.PlanID).Error; err != nil {
		return catalogmanifest.Plan{}, backup, ErrCatalogSyncPlanUnavailable
	}
	// The original actor need not have a live session now. This is provenance
	// verification only; the new restore actor is authorized independently.
	originalPlan, err := decodeCatalogSyncPlan(originalRow, binding.Actor, time.Unix(0, 0))
	if err != nil || originalPlan.Digest != binding.Digest || originalPlan.Kind != binding.Kind || originalPlan.RestoreOperationID != binding.RestoreOperationID {
		return catalogmanifest.Plan{}, backup, ErrCatalogSyncPlanStale
	}
	current := make(map[string]catalogmanifest.Entry)
	after := make(map[string]catalogmanifest.Entry)
	for _, entry := range before.Entries {
		current[catalogmanifest.EntryID(entry)] = entry
	}
	for _, entry := range backup.After {
		after[catalogmanifest.EntryID(entry)] = entry
	}
	plan := catalogmanifest.Plan{Kind: "restore", RestoreOperationID: operationID, Actor: actor, TargetDigest: before.Digest, BaselineGeneration: base.Generation, ExpiresAt: now.Add(10 * time.Minute).Unix(), Snapshot: before}
	plan.Snapshot.SourceID = originalPlan.Snapshot.SourceID
	plan.Snapshot.ObjectVersions = maps.Clone(before.ObjectVersions)
	seen := make(map[string]bool)
	for _, saved := range backup.Before {
		id := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: saved.Kind, Key: saved.Key})
		old, exists := current[id]
		post, expected := after[id]
		if seen[id] || exists != expected || exists && old != post {
			return plan, backup, ErrCatalogSyncPlanStale
		}
		seen[id] = true
		if saved.Kind == catalogmanifest.KindModelPrice || saved.Kind == catalogmanifest.KindPluginPrice {
			key, err := catalogmanifest.DecodePriceKey(saved.Key)
			if err != nil {
				return plan, backup, ErrCatalogSyncPlanStale
			}
			modelID := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: catalogmanifest.KindModel, Key: key.Model})
			expectedVersion := backup.ObjectVersions[modelID]
			if expectedVersion == "" {
				expectedVersion = backup.Baseline.ObjectVersions[modelID]
			}
			if expectedVersion == "" || expectedVersion != before.ObjectVersions[modelID] {
				return plan, backup, ErrCatalogSyncPlanStale
			}
		}
		if exists && (saved.Kind == catalogmanifest.KindModel || saved.Kind == catalogmanifest.KindVendor) && (backup.ObjectVersions[id] == "" || backup.ObjectVersions[id] != before.ObjectVersions[id]) {
			return plan, backup, ErrCatalogSyncPlanStale
		}
		change := catalogmanifest.Change{Kind: saved.Kind, Key: saved.Key, Reason: "restore_preimage", Action: "adopt"}
		if exists {
			change.Before = &old
		}
		if !saved.Exists {
			if exists {
				change.Action = "delete"
			}
		} else {
			var value any
			switch saved.Kind {
			case catalogmanifest.KindVendor:
				if saved.Vendor == nil {
					return plan, backup, ErrCatalogSyncPlanUnavailable
				}
				v := saved.Vendor
				value = catalogmanifest.VendorValue{Name: v.Name, Description: v.Description, Icon: v.Icon, Status: v.Status, DisplayOrder: v.DisplayOrder, CreatedTime: v.CreatedTime, UpdatedTime: v.UpdatedTime}
			case catalogmanifest.KindModel:
				if saved.Model == nil {
					return plan, backup, ErrCatalogSyncPlanUnavailable
				}
				vendorName := ""
				if saved.Model.VendorID != 0 {
					// The sealed original Before retains the authoritative vendor
					// name. A current ID lookup could follow a later vendor rename
					// and silently change the relationship being restored.
					for _, original := range originalPlan.Changes {
						if original.Kind == saved.Kind && original.Key == saved.Key && original.Before != nil {
							var metadata catalogmanifest.ModelValue
							if common.UnmarshalJsonStr(original.Before.Value, &metadata) != nil {
								return plan, backup, ErrCatalogSyncPlanStale
							}
							vendorName = metadata.Vendor
							break
						}
					}
					if vendorName == "" {
						return plan, backup, ErrCatalogSyncPlanStale
					}
				}
				value = managedModelValue(*saved.Model, vendorName)
			default:
				if saved.Price == nil {
					return plan, backup, ErrCatalogSyncPlanUnavailable
				}
				value = saved.Price
			}
			encoded, err := common.Marshal(value)
			if err != nil {
				return plan, backup, err
			}
			canonical, err := catalogmanifest.CanonicalJSON(string(encoded))
			if err != nil {
				return plan, backup, err
			}
			entry := catalogmanifest.Entry{Kind: saved.Kind, Key: saved.Key, Value: canonical}
			current[id], change.After = entry, &entry
			if !exists {
				change.Action = "create"
			} else if old.Value != entry.Value {
				change.Action = "update"
			}
		}
		plan.Changes = append(plan.Changes, change)
	}
	// The shared reference policy needs unchanged target-only metadata too,
	// notably live model-to-vendor relationships. Preserve changes never write
	// data or enter the affected backup scope.
	for _, entry := range before.Entries {
		if !seen[catalogmanifest.EntryID(entry)] {
			plan.Changes = append(plan.Changes, catalogmanifest.Change{Kind: entry.Kind, Key: entry.Key, Action: "preserve", Reason: "target_only", Before: &entry})
		}
	}
	plan.Snapshot.Entries = nil
	plan.Snapshot.Coverage = make(map[string]int)
	for _, kind := range catalogmanifest.Kinds() {
		plan.Snapshot.Coverage[kind] = 0
	}
	for _, id := range slices.Sorted(maps.Keys(current)) {
		entry := current[id]
		plan.Snapshot.Entries = append(plan.Snapshot.Entries, entry)
		plan.Snapshot.Coverage[entry.Kind]++
	}
	plan.Snapshot.Digest, err = catalogmanifest.SnapshotDigest(plan.Snapshot)
	if err == nil {
		err = catalogmanifest.ValidateSnapshotStructure(plan.Snapshot)
	}
	return plan, backup, err
}

// Ownership is restored independently of local values. Only objects actually
// restored by this operation acquire their current incarnation. Unaffected old
// tokens remain old, so a locally recreated object never gains delete authority.
func restoreCatalogBaselineTx(tx *gorm.DB, operationID string, after catalogmanifest.Snapshot, expectedGeneration int64) error {
	var original CatalogSyncOperation
	if err := tx.First(&original, "id = ?", operationID).Error; err != nil {
		return err
	}
	var backup catalogOperationBackup
	if common.UnmarshalJsonStr(string(original.Backup), &backup) != nil || backup.Version != 1 {
		return ErrCatalogSyncPlanStale
	}
	var state CatalogSyncState
	if err := tx.First(&state, CatalogSyncStateID).Error; err != nil {
		return err
	}
	versions := maps.Clone(backup.Baseline.ObjectVersions)
	if versions == nil {
		versions = make(map[string]string)
	}
	for _, entry := range backup.Before {
		id := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: entry.Kind, Key: entry.Key})
		if entry.Exists && (entry.Kind == catalogmanifest.KindVendor || entry.Kind == catalogmanifest.KindModel) {
			if after.ObjectVersions[id] == "" {
				return ErrCatalogSyncPlanStale
			}
			var localID int
			if entry.Kind == catalogmanifest.KindVendor && entry.Vendor != nil {
				localID = entry.Vendor.Id
			}
			if entry.Kind == catalogmanifest.KindModel && entry.Model != nil {
				localID = entry.Model.Id
			}
			mac := hmac.New(sha256.New, []byte(state.IncarnationKey))
			fmt.Fprintf(mac, "%s\x00%d", entry.Kind, localID)
			if localID != 0 && versions[id] == fmt.Sprintf("%x", mac.Sum(nil)) {
				versions[id] = after.ObjectVersions[id]
			}
		}
	}
	generation := expectedGeneration + 1
	rows := make([]CatalogSyncBaseline, 0, len(backup.Baseline.Entries))
	for _, entry := range backup.Baseline.Entries {
		id := catalogmanifest.EntryID(entry)
		rows = append(rows, CatalogSyncBaseline{IdentityHash: fmt.Sprintf("%x", sha256.Sum256([]byte(id))), SourceID: CatalogSyncText(backup.Baseline.SourceID), Generation: generation, Kind: entry.Kind, EntryKey: CatalogSyncText(entry.Key), EntryValue: CatalogSyncText(entry.Value), ObjectVersion: versions[id]})
	}
	result := tx.Model(&CatalogSyncState{}).Where("id = ? AND baseline_generation = ?", CatalogSyncStateID, expectedGeneration).Updates(map[string]any{"source_id": backup.Baseline.SourceID, "baseline_generation": generation})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrCatalogSyncPlanStale
	}
	if err := tx.Where("1 = 1").Delete(&CatalogSyncBaseline{}).Error; err != nil {
		return err
	}
	if len(rows) != 0 {
		return tx.CreateInBatches(rows, 100).Error
	}
	return nil
}

type CatalogSyncOperationSummary struct {
	Kind               string         `json:"kind"`
	RestoreOperationID string         `json:"restore_operation_id,omitempty"`
	SourceID           string         `json:"source_id"`
	TargetID           string         `json:"target_id"`
	ActorUserID        int            `json:"actor_user_id"`
	Digest             string         `json:"digest"`
	Actions            map[string]int `json:"actions"`
}

func ListCatalogSyncOperations(ctx context.Context, offset, limit int) ([]CatalogSyncOperation, int64, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}
	limit = min(limit, 100)
	var rows []CatalogSyncOperation
	var total int64
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&CatalogSyncOperation{}).Count(&total).Error; err != nil {
			return err
		}
		// Backup/Result are never fetched for a public history request. History
		// is private binding only; the returned row clears even that field.
		if err := tx.Select("id", "plan_id", "state", "revision", "created_at", "history").Order("revision DESC").Order("created_at DESC").Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		for i := range rows {
			row := &rows[i]
			var binding catalogOperationBinding
			if common.UnmarshalJsonStr(string(row.History), &binding) != nil {
				return ErrCatalogSyncOperationConflict
			}
			var stored CatalogSyncPlan
			if err := tx.Select("body").First(&stored, "id = ?", row.PlanID).Error; err != nil {
				return err
			}
			var plan catalogmanifest.Plan
			if common.UnmarshalJsonStr(string(stored.Body), &plan) != nil || binding.PlanID != row.PlanID || binding.Kind != plan.Kind || binding.RestoreOperationID != plan.RestoreOperationID || binding.Digest != plan.Digest {
				return ErrCatalogSyncOperationConflict
			}
			summary := &CatalogSyncOperationSummary{Kind: binding.Kind, RestoreOperationID: binding.RestoreOperationID, SourceID: plan.Snapshot.SourceID, TargetID: binding.Actor.TargetID, ActorUserID: binding.Actor.UserID, Digest: binding.Digest, Actions: make(map[string]int)}
			for _, change := range plan.Changes {
				summary.Actions[change.Action]++
			}
			row.Summary, row.History, row.Backup, row.Result = summary, "", "", ""
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
