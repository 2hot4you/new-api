package model

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Draft construction is structural only. The source values come exclusively
// from the persisted full source; mutable diff values omit audit provenance.
func catalogPlanDraft(before catalogmanifest.Snapshot, plan catalogmanifest.Plan) (catalogmanifest.Snapshot, []string, error) {
	entries := make(map[string]catalogmanifest.Entry)
	source := make(map[string]catalogmanifest.Entry)
	for _, entry := range before.Entries {
		entries[catalogmanifest.EntryID(entry)] = entry
	}
	for _, entry := range plan.Snapshot.Entries {
		source[catalogmanifest.EntryID(entry)] = entry
	}
	strict := []string{}
	for _, change := range plan.Changes {
		id := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: change.Kind, Key: change.Key})
		switch change.Action {
		case "delete":
			delete(entries, id)
		case "create", "update", "adopt":
			if change.After == nil {
				continue
			}
			entry, ok := source[id]
			if !ok {
				return catalogmanifest.Snapshot{}, nil, ErrCatalogSyncPlanStale
			}
			entries[id] = entry
			if entry.Kind != catalogmanifest.KindVendor && entry.Kind != catalogmanifest.KindModel {
				strict = append(strict, id)
			}
		case "preserve", "unchanged", "blocked", "conflict":
		default:
			return catalogmanifest.Snapshot{}, nil, ErrCatalogSyncPlanStale
		}
	}
	draft := before
	draft.Entries = make([]catalogmanifest.Entry, 0, len(entries))
	draft.Coverage = make(map[string]int)
	for _, kind := range catalogmanifest.Kinds() {
		draft.Coverage[kind] = 0
	}
	for _, id := range slices.Sorted(maps.Keys(entries)) {
		entry := entries[id]
		draft.Entries = append(draft.Entries, entry)
		draft.Coverage[entry.Kind]++
	}
	var err error
	draft.Digest, err = catalogmanifest.SnapshotDigest(draft)
	if err != nil {
		return draft, nil, err
	}
	return draft, strict, catalogmanifest.ValidateSnapshotStructure(draft)
}

// All compiler work is between root transactions. The second transaction
// rebuilds Before/Draft from fresh authoritative rows and compares exact inputs.
func prepareCatalogSyncPlan(ctx context.Context, source catalogmanifest.Snapshot, actor catalogmanifest.Actor, now time.Time, existingID, expectedDigest string, choices catalogmanifest.Resolution, restoreID string) (catalogmanifest.Plan, error) {
	var plan catalogmanifest.Plan
	if !validCatalogActor(actor) {
		return plan, ErrCatalogSyncPlanUnavailable
	}
	var before catalogmanifest.Snapshot
	var base catalogmanifest.Baseline
	var references catalogReferenceView
	var input catalogValidationInput
	var revision int64
	err := TryWithCatalogWriteBarrier(ctx, func() error {
		return catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			if err := catalogSensitiveInstanceTx(tx); err != nil {
				return err
			}
			if state.PublicationState != "ready" {
				return ErrCatalogPublicationPending
			}
			var err error
			if existingID != "" {
				var row CatalogSyncPlan
				if err = tx.First(&row, "id = ?", existingID).Error; err != nil {
					return err
				}
				plan, err = decodeCatalogSyncPlan(row, actor, time.Now())
				if err != nil {
					return err
				}
				if row.Digest != expectedDigest {
					return ErrCatalogSyncPlanStale
				}
				source = plan.Snapshot
				restoreID = plan.RestoreOperationID
			}
			before, err = captureCatalogTargetTx(ctx, tx, actor.TargetID, state, true)
			if err != nil {
				return err
			}
			base, err = LoadCatalogSyncBaselineTx(tx, state)
			if err != nil {
				return err
			}
			if restoreID != "" {
				if err := authenticateCatalogActorTx(tx, actor); err != nil {
					return err
				}
				inverse, _, err := catalogRestorePlanTx(tx, restoreID, actor, now, before, base)
				if err != nil {
					return err
				}
				if existingID == "" {
					plan = inverse
				}
			}
			references, err = captureCatalogReferencesTx(tx, pin)
			if err != nil {
				return err
			}
			input, err = captureCatalogValidationInputTx(tx, pin, before, before, nil)
			revision = state.Revision
			return err
		})
	})
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	if existingID == "" {
		if restoreID == "" {
			plan, err = catalogmanifest.BuildPlan(source, before, base, actor, now)
		}
		if err == nil {
			plan.ID, err = catalogRandomID()
		}
	} else {
		if plan.TargetDigest != before.Digest || plan.BaselineGeneration != base.Generation {
			return catalogmanifest.Plan{}, ErrCatalogSyncPlanStale
		}
		plan, err = catalogmanifest.ResolvePlan(plan, choices)
	}
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	blocked, referenceDigest, err := catalogReferenceDecision(references, plan, base)
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	if existingID != "" && plan.ReferenceDigest != referenceDigest {
		return catalogmanifest.Plan{}, ErrCatalogSyncPlanStale
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if reason := blocked[catalogmanifest.EntryID(catalogmanifest.Entry{Kind: change.Kind, Key: change.Key})]; reason != "" {
			change.Action, change.Reason = "blocked", reason
		}
	}
	plan.ReferenceDigest = referenceDigest
	draft, strict, err := catalogPlanDraft(before, plan)
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	var data catalogValidationData
	if err := common.UnmarshalJsonStr(input.canonical, &data); err != nil {
		return catalogmanifest.Plan{}, err
	}
	canonical, err := catalogmanifest.CanonicalSnapshotContent(draft)
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	if err = common.UnmarshalJsonStr(canonical, &data.Draft); err != nil {
		return catalogmanifest.Plan{}, err
	}
	slices.Sort(strict)
	data.StrictPrices = strict
	encoded, err := common.Marshal(data)
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	input.canonical, err = catalogmanifest.CanonicalJSON(string(encoded))
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	attestation, err := stageCatalogValidation(input)
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	plan.ValidationDigest = attestation.digest
	plan.Digest, err = catalogmanifest.CanonicalPlanDigest(plan)
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	body, err := common.Marshal(plan)
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	row := CatalogSyncPlan{ID: plan.ID, Body: CatalogSyncText(body), Digest: plan.Digest, Validation: CatalogSyncText(attestation.payload), TargetRevision: revision, BaselineGeneration: base.Generation, ExpiresAt: plan.ExpiresAt}
	err = TryWithCatalogWriteBarrier(ctx, func() error {
		return catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			if state.PublicationState != "ready" {
				return ErrCatalogPublicationPending
			}
			if _, err := recheckCatalogPlanTx(ctx, tx, state, pin, row, plan); err != nil {
				return err
			}
			if existingID == "" {
				return tx.Create(&row).Error
			}
			var current CatalogSyncPlan
			if err := tx.First(&current, "id = ?", existingID).Error; err != nil {
				return err
			}
			if current.Digest != expectedDigest {
				return ErrCatalogSyncPlanStale
			}
			if _, err := decodeCatalogSyncPlan(current, actor, time.Now()); err != nil {
				return err
			}
			if current.Digest == row.Digest {
				return nil
			}
			return tx.Model(&CatalogSyncPlan{}).Where("id = ? AND digest = ?", row.ID, expectedDigest).Updates(map[string]any{"body": row.Body, "digest": row.Digest, "validation": row.Validation}).Error
		})
	})
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	return plan, nil
}

func recheckCatalogPlanTx(ctx context.Context, tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin, row CatalogSyncPlan, plan catalogmanifest.Plan) (catalogmanifest.Snapshot, error) {
	if err := catalogSensitiveInstanceTx(tx); err != nil {
		return catalogmanifest.Snapshot{}, err
	}
	if (plan.Kind != "sync" && plan.Kind != "restore") || row.TargetRevision != state.Revision || row.BaselineGeneration != state.BaselineGeneration {
		return catalogmanifest.Snapshot{}, ErrCatalogSyncPlanStale
	}
	before, err := captureCatalogTargetTx(ctx, tx, plan.Actor.TargetID, state, true)
	if err != nil {
		return before, err
	}
	if before.Digest != plan.TargetDigest {
		return before, ErrCatalogSyncPlanStale
	}
	base, err := LoadCatalogSyncBaselineTx(tx, state)
	if err != nil {
		return before, err
	}
	if base.Generation != plan.BaselineGeneration {
		return before, ErrCatalogSyncPlanStale
	}
	if plan.Kind == "restore" {
		if err := authenticateCatalogActorTx(tx, plan.Actor); err != nil {
			return before, err
		}
		if _, _, err := catalogRestorePlanTx(tx, plan.RestoreOperationID, plan.Actor, time.Now(), before, base); err != nil {
			return before, err
		}
	}
	references, err := captureCatalogReferencesTx(tx, pin)
	if err != nil {
		return before, err
	}
	blocked, digest, err := catalogReferenceDecision(references, plan, base)
	if err != nil {
		return before, err
	}
	if digest != plan.ReferenceDigest {
		return before, ErrCatalogSyncPlanStale
	}
	for _, change := range plan.Changes {
		if reason := blocked[catalogmanifest.EntryID(catalogmanifest.Entry{Kind: change.Kind, Key: change.Key})]; reason != "" && change.Action != "blocked" {
			return before, ErrCatalogSyncPlanStale
		}
	}
	draft, strict, err := catalogPlanDraft(before, plan)
	if err != nil {
		return before, err
	}
	input, err := captureCatalogValidationInputTx(tx, pin, before, draft, strict)
	if err != nil {
		return before, err
	}
	return before, verifyCatalogValidation(row, plan, input)
}

var ErrCatalogSyncOperationConflict = errors.New("catalog operation binding does not match")

type catalogOperationBinding struct {
	Actor                catalogmanifest.Actor
	PlanID, Kind, Digest string
	RestoreOperationID   string `json:",omitempty"`
}

type catalogBackupEntry struct {
	Kind, Key string
	Exists    bool
	Vendor    *Vendor                     `json:",omitempty"`
	Model     *Model                      `json:",omitempty"`
	Price     *catalogmanifest.PriceValue `json:",omitempty"`
}

type catalogOperationBackup struct {
	Version        int
	Before         []catalogBackupEntry
	After          []catalogmanifest.Entry
	ObjectVersions map[string]string
	OptionPresence map[string]bool
	Baseline       catalogmanifest.Baseline
}

func authenticateCatalogActorTx(tx *gorm.DB, actor catalogmanifest.Actor) error {
	if !validCatalogActor(actor) {
		return ErrCatalogSyncPlanUnavailable
	}
	if err := catalogAuthStorageTx(tx, actor); err != nil {
		return err
	}
	if err := ValidateAuthSessionWithTx(tx, AuthSessionIdentity{UserID: actor.UserID, SessionID: actor.SessionID, UserAuthVersion: int64(actor.AuthVersion), SessionVersion: int64(actor.SessionVersion)}); err != nil {
		return ErrCatalogSyncPlanUnavailable
	}
	// A locking current read is essential on MySQL: the catalog's consistent
	// snapshot predates these user/session locks.
	var user User
	if err := lockForUpdate(tx).Select("id", "role").First(&user, actor.UserID).Error; err != nil {
		return ErrCatalogSyncPlanUnavailable
	}
	if user.Role != common.RoleRootUser {
		return ErrCatalogSyncPlanUnavailable
	}
	return nil
}

// The shared session validator assumes transactional, ordinary auth tables.
// Hold relation metadata before checking that prerequisite. This locks only
// schema access here; the validator subsequently locks the exact user/session.
func catalogAuthStorageTx(tx *gorm.DB, actor catalogmanifest.Actor) error {
	for _, item := range []struct {
		model any
		key   string
		value any
	}{{&User{}, "id", actor.UserID}, {&UserSession{}, "sid", actor.SessionID}} {
		stmt := &gorm.Statement{DB: tx}
		if err := stmt.Parse(item.model); err != nil {
			return err
		}
		name := stmt.Schema.Table
		table := stmt.Quote(clause.Table{Name: name})
		key := stmt.Quote(clause.Column{Name: item.key})
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("LOCK TABLE " + table + " IN ROW SHARE MODE").Error; err != nil {
				return err
			}
		} else {
			// Even an absent identity acquires MySQL's transaction-duration MDL.
			rows, err := tx.Raw("SELECT "+key+" FROM "+table+" WHERE "+key+" = ?", item.value).Rows()
			if err != nil {
				return err
			}
			for rows.Next() {
				var identity any
				if err := rows.Scan(&identity); err != nil {
					rows.Close()
					return err
				}
			}
			err = rows.Err()
			closeErr := rows.Close()
			if err != nil || closeErr != nil {
				return errors.Join(err, closeErr)
			}
		}
		switch tx.Dialector.Name() {
		case "mysql":
			var actual, definition string
			if err := tx.Raw("SHOW CREATE TABLE "+table).Row().Scan(&actual, &definition); err != nil {
				return err
			}
			if !strings.HasPrefix(definition, "CREATE TABLE ") || !strings.Contains(definition, "\n) ENGINE=InnoDB ") || strings.Contains(definition, "PARTITION BY") {
				return errors.New("catalog auth storage must be persistent nonpartitioned InnoDB tables")
			}
		case "postgres":
			var kind, persistence string
			var rls, forced, inherited bool
			if err := tx.Raw("SELECT c.relkind, c.relpersistence, c.relrowsecurity, c.relforcerowsecurity, EXISTS (SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid) FROM pg_catalog.pg_class c WHERE c.oid=pg_catalog.to_regclass(?)", table).Row().Scan(&kind, &persistence, &rls, &forced, &inherited); err != nil {
				return err
			}
			if kind != "r" || persistence == "t" || rls || forced || inherited {
				return errors.New("catalog auth storage must be ordinary persistent non-RLS tables")
			}
		case "sqlite":
			parts := strings.Split(name, ".")
			if len(parts) > 2 || len(parts) == 2 && parts[0] != "main" {
				return errors.New("catalog auth storage must use main SQLite tables")
			}
			var kind, definition string
			if err := tx.Raw("SELECT type, sql FROM main.sqlite_master WHERE name = ?", parts[len(parts)-1]).Row().Scan(&kind, &definition); err != nil {
				return err
			}
			var temporary int
			if err := tx.Raw("SELECT count(*) FROM sqlite_temp_master WHERE name = ?", parts[len(parts)-1]).Row().Scan(&temporary); err != nil {
				return err
			}
			if kind != "table" || temporary != 0 || !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(definition)), "CREATE TABLE ") {
				return errors.New("catalog auth storage must use ordinary main SQLite tables")
			}
		default:
			return errors.New("catalog auth storage is unverified")
		}
		columns, err := tx.Migrator().ColumnTypes(item.model)
		if err != nil {
			return err
		}
		keys := []string{}
		for _, column := range columns {
			primary, known := column.PrimaryKey()
			if !known {
				return errors.New("catalog auth identity uniqueness is unverified")
			}
			if primary {
				keys = append(keys, column.Name())
			}
		}
		if len(keys) != 1 || keys[0] != item.key {
			return errors.New("catalog auth identity must have its exact primary key")
		}
	}
	return nil
}

func catalogOperationResult(tx *gorm.DB, operationID string, binding catalogOperationBinding) (catalogmanifest.Result, bool, error) {
	var operation CatalogSyncOperation
	err := tx.First(&operation, "id = ?", operationID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return catalogmanifest.Result{}, false, nil
	}
	if err != nil {
		return catalogmanifest.Result{}, false, err
	}
	var stored catalogOperationBinding
	if err := common.UnmarshalJsonStr(string(operation.History), &stored); err != nil {
		return catalogmanifest.Result{}, true, err
	}
	if (stored.Kind != "sync" && stored.Kind != "restore") || (stored.Kind == "sync" && stored.RestoreOperationID != "") || (stored.Kind == "restore" && stored.RestoreOperationID == "") {
		return catalogmanifest.Result{}, true, ErrCatalogSyncOperationConflict
	}
	// The public apply request supplies the sealed digest, not a client kind or
	// inverse ID. Exact replay derives that intent from the immutable operation
	// binding, independently of preview expiry or plan-row retention.
	if binding.Kind == "" && binding.RestoreOperationID == "" {
		binding.Kind, binding.RestoreOperationID = stored.Kind, stored.RestoreOperationID
	}
	if stored != binding || operation.PlanID != binding.PlanID {
		return catalogmanifest.Result{}, true, ErrCatalogSyncOperationConflict
	}
	var result catalogmanifest.Result
	if err := common.UnmarshalJsonStr(string(operation.Result), &result); err != nil {
		return result, true, err
	}
	if result.OperationID != operation.ID || result.Revision != operation.Revision || result.State != "committed_pending_publish" {
		return catalogmanifest.Result{}, true, ErrCatalogSyncOperationConflict
	}
	return result, true, nil
}

// Result is the immutable original receipt, including its pending state. The
// operation's live State becomes succeeded only after publication and exact ack.
func ApplyCatalogSyncPlan(ctx context.Context, planID, finalDigest, operationID string, actor catalogmanifest.Actor) (catalogmanifest.Result, error) {
	var result catalogmanifest.Result
	if !validCatalogActor(actor) || planID == "" || finalDigest == "" || operationID == "" || len(operationID) > 64 || strings.TrimSpace(operationID) != operationID {
		return result, ErrCatalogSyncPlanUnavailable
	}
	binding := catalogOperationBinding{Actor: actor, PlanID: planID, Digest: finalDigest}
	var input catalogRuntimeInput
	var prepared catalogmanifest.Plan
	var replay bool
	err := TryWithCatalogWriteBarrier(ctx, func() error {
		return catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			if err := authenticateCatalogActorTx(tx, actor); err != nil {
				return err
			}
			stored, found, err := catalogOperationResult(tx, operationID, binding)
			if err != nil {
				return err
			}
			if found {
				result, replay = stored, true
				return nil
			}
			var prior CatalogSyncOperation
			if err := tx.Where("plan_id = ?", planID).First(&prior).Error; err == nil {
				return ErrCatalogSyncOperationConflict
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if state.PublicationState != "ready" {
				return ErrCatalogPublicationPending
			}
			var row CatalogSyncPlan
			if err := tx.First(&row, "id = ?", planID).Error; err != nil {
				return err
			}
			prepared, err = decodeCatalogSyncPlan(row, actor, time.Now())
			if err != nil {
				return err
			}
			if row.Digest != finalDigest || !catalogmanifest.PlanExecutable(prepared, time.Now()) {
				return ErrCatalogSyncPlanStale
			}
			if _, err := recheckCatalogPlanTx(ctx, tx, state, pin, row, prepared); err != nil {
				return err
			}
			input, err = captureCatalogRuntimeTx(tx, state, pin)
			return err
		})
	})
	if errors.Is(err, ErrCatalogCommitUncertain) {
		// Capture can be an exact replay of an already durable operation. Keep
		// its original lost-response reconciliation; never stage or retry a
		// fresh mutation when the capture transaction outcome is uncertain.
		lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		stored, found, lookupErr := catalogOperationResult(DB.WithContext(lookupCtx), operationID, binding)
		if lookupErr != nil {
			return catalogmanifest.Result{}, errors.Join(err, lookupErr)
		}
		if found {
			return stored, nil
		}
	}
	if err != nil {
		return catalogmanifest.Result{}, err
	}
	if replay {
		return result, nil
	}
	// Neither SQL fences, registry pins nor catalog/pricing locks survive this
	// point. Full Task4 attestation above remains mandatory and is rechecked below.
	next, strict, err := catalogPlanPricing(input.options, prepared)
	if err != nil {
		return result, err
	}
	prospective := input
	prospective.options = next
	stage, err := stageProspectiveCatalogRuntime(prospective, input.options, strict)
	if err != nil {
		return result, err
	}
	committed := false
	err = TryWithCatalogWriteBarrier(ctx, func() error {
		err := catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			if err := authenticateCatalogActorTx(tx, actor); err != nil {
				return err
			}
			stored, found, err := catalogOperationResult(tx, operationID, binding)
			if err != nil {
				return err
			}
			if found {
				result = stored
				return nil
			}
			var prior CatalogSyncOperation
			if err := tx.Where("plan_id = ?", planID).First(&prior).Error; err == nil {
				return ErrCatalogSyncOperationConflict
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if state.PublicationState != "ready" {
				return ErrCatalogPublicationPending
			}
			var row CatalogSyncPlan
			if err := tx.First(&row, "id = ?", planID).Error; err != nil {
				return err
			}
			plan, err := decodeCatalogSyncPlan(row, actor, time.Now())
			if err != nil {
				return err
			}
			if row.Digest != finalDigest || !catalogmanifest.PlanExecutable(plan, time.Now()) {
				return ErrCatalogSyncPlanStale
			}
			binding.Kind, binding.RestoreOperationID = plan.Kind, plan.RestoreOperationID
			before, err := recheckCatalogPlanTx(ctx, tx, state, pin, row, plan)
			if err != nil {
				return err
			}
			fresh, err := captureCatalogRuntimeTx(tx, state, pin)
			if err != nil {
				return err
			}
			if !input.same(fresh) {
				return ErrCatalogSyncPlanStale
			}
			backup, err := captureCatalogBackupTx(tx, state, before, plan)
			if err != nil {
				return err
			}
			if !catalogmanifest.PlanExecutable(plan, time.Now()) {
				return ErrCatalogSyncPlanStale
			}
			backupJSON, err := common.Marshal(backup)
			if err != nil {
				return err
			}
			history, err := common.Marshal(binding)
			if err != nil {
				return err
			}
			result = catalogmanifest.Result{OperationID: operationID, State: "committed_pending_publish", Revision: state.Revision + 1}
			resultJSON, err := common.Marshal(result)
			if err != nil {
				return err
			}
			operation := CatalogSyncOperation{ID: operationID, PlanID: planID, State: result.State, Revision: result.Revision, CreatedAt: time.Now().Unix(), Backup: CatalogSyncText(backupJSON), History: CatalogSyncText(history), Result: CatalogSyncText(resultJSON)}
			if err := tx.Create(&operation).Error; err != nil {
				return err
			}
			if err := writeCatalogPlanTx(tx, plan, next); err != nil {
				return err
			}
			after, err := captureCatalogTargetTx(ctx, tx, actor.TargetID, state, true)
			if err != nil {
				return err
			}
			if err := verifyCatalogWrittenDraft(before, after, plan); err != nil {
				return err
			}
			if plan.Kind == "restore" {
				if err := restoreCatalogBaselineTx(tx, plan.RestoreOperationID, after, state.BaselineGeneration); err != nil {
					return err
				}
			} else {
				if _, err := saveCatalogSyncBaselineTx(tx, plan.Snapshot, after.ObjectVersions, state.BaselineGeneration, true); err != nil {
					return err
				}
			}
			affected := make(map[string]bool)
			for _, entry := range backup.Before {
				affected[catalogmanifest.EntryID(catalogmanifest.Entry{Kind: entry.Kind, Key: entry.Key})] = true
			}
			backup.ObjectVersions = make(map[string]string)
			for _, entry := range after.Entries {
				id := catalogmanifest.EntryID(entry)
				if affected[id] {
					backup.After = append(backup.After, entry)
					if token := after.ObjectVersions[id]; token != "" {
						backup.ObjectVersions[id] = token
					}
				}
			}
			backupJSON, err = common.Marshal(backup)
			if err != nil {
				return err
			}
			if err := tx.Model(&CatalogSyncOperation{}).Where("id = ?", operationID).Update("backup", CatalogSyncText(backupJSON)).Error; err != nil {
				return err
			}
			digest, err := catalogPersistedDigest(tx)
			if err != nil {
				return err
			}
			if err := tx.Model(&CatalogSyncState{}).Where("id = ?", CatalogSyncStateID).Updates(map[string]any{"revision": result.Revision, "current_digest": digest, "publication_state": result.State, "pending_operation_id": operationID}).Error; err != nil {
				return err
			}
			if err := tx.First(state, CatalogSyncStateID).Error; err != nil {
				return err
			}
			postimage, err := captureCatalogRuntimeTx(tx, state, pin)
			if err != nil {
				return err
			}
			if !maps.Equal(next, postimage.options) || input.dependencies != postimage.dependencies || input.generation != postimage.generation {
				return ErrCatalogSyncPlanStale
			}
			// The structural whole-catalog draft proof above and exact raw options
			// bind this already-compiled candidate to the actual transaction image.
			stage.input = postimage
			committed = true
			return nil
		})
		// A lost acknowledgement never causes mutation replay. Resolve only a
		// durable exact binding on a fresh connection, even if request expired.
		if errors.Is(err, ErrCatalogCommitUncertain) {
			lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			stored, found, lookupErr := catalogOperationResult(DB.WithContext(lookupCtx), operationID, binding)
			if lookupErr != nil {
				committed = false
				return errors.Join(err, lookupErr)
			}
			if found {
				result, err = stored, nil
			}
		}
		if err != nil {
			committed = false
			return err
		}
		if !committed { // exact replay discovered during the second transaction
			return nil
		}
		// Root COMMIT/cleanup has released its registry pin. Keep this SAME
		// writer through typed publish, read-only rebuild, and durable runtime ack.
		return publishCatalogRuntimeGuarded(ctx, stage)
	})
	if err != nil && !committed {
		return catalogmanifest.Result{}, err
	}
	return result, err
}

func captureCatalogBackupTx(tx *gorm.DB, state *CatalogSyncState, before catalogmanifest.Snapshot, plan catalogmanifest.Plan) (catalogOperationBackup, error) {
	backup := catalogOperationBackup{Version: 1, OptionPresence: make(map[string]bool)}
	var err error
	backup.Baseline, err = LoadCatalogSyncBaselineTx(tx, state)
	if err != nil {
		return backup, err
	}
	entries := make(map[string]catalogmanifest.Entry)
	for _, entry := range before.Entries {
		entries[catalogmanifest.EntryID(entry)] = entry
	}
	for _, change := range plan.Changes {
		if change.Action == "preserve" || change.Action == "unchanged" {
			continue
		}
		entry := catalogBackupEntry{Kind: change.Kind, Key: change.Key}
		old, exists := entries[catalogmanifest.EntryID(catalogmanifest.Entry{Kind: change.Kind, Key: change.Key})]
		entry.Exists = exists
		switch change.Kind {
		case catalogmanifest.KindVendor:
			if exists {
				entry.Vendor = &Vendor{}
				if err := tx.First(entry.Vendor, "name = ?", change.Key).Error; err != nil {
					return backup, err
				}
			}
		case catalogmanifest.KindModel:
			if exists {
				entry.Model = &Model{}
				if err := tx.First(entry.Model, "model_name = ?", change.Key).Error; err != nil {
					return backup, err
				}
			}
		default:
			key, err := catalogmanifest.DecodePriceKey(change.Key)
			if err != nil {
				return backup, err
			}
			var count int64
			if err := tx.Model(&Option{}).Where(map[string]any{"key": key.Option}).Count(&count).Error; err != nil {
				return backup, err
			}
			backup.OptionPresence[key.Option] = count != 0
			if exists {
				entry.Price = &catalogmanifest.PriceValue{}
				if err := common.UnmarshalJsonStr(old.Value, entry.Price); err != nil {
					return backup, err
				}
			}
		}
		backup.Before = append(backup.Before, entry)
	}
	return backup, nil
}

func writeCatalogPlanTx(tx *gorm.DB, plan catalogmanifest.Plan, pricing map[string]string) error {
	now := time.Now().Unix()
	for _, kind := range []string{catalogmanifest.KindVendor, catalogmanifest.KindModel} {
		for _, change := range plan.Changes {
			if change.Kind != kind || change.Action != "create" && change.Action != "update" {
				continue
			}
			if kind == catalogmanifest.KindVendor {
				var record Vendor
				if err := common.UnmarshalJsonStr(change.After.Value, &record); err != nil {
					return err
				}
				record.UpdatedTime = now
				if change.Action == "create" {
					record.CreatedTime = now
					if err := tx.Create(&record).Error; err != nil {
						return err
					}
				} else {
					var old Vendor
					if err := tx.First(&old, "name = ?", change.Key).Error; err != nil {
						return err
					}
					record.Id, record.CreatedTime = old.Id, old.CreatedTime
				}
				// GORM default tags must not turn an explicit source zero into one.
				var value catalogmanifest.VendorValue
				if err := common.UnmarshalJsonStr(change.After.Value, &value); err != nil {
					return err
				}
				record.Status = value.Status
				if err := tx.Model(&record).Select("name", "description", "icon", "status", "display_order", "updated_time").Updates(&record).Error; err != nil {
					return err
				}
			} else {
				var record Model
				var value catalogmanifest.ModelValue
				if err := common.UnmarshalJsonStr(change.After.Value, &record); err != nil {
					return err
				}
				if err := common.UnmarshalJsonStr(change.After.Value, &value); err != nil {
					return err
				}
				if value.Vendor != "" {
					var vendor Vendor
					if err := tx.First(&vendor, "name = ?", value.Vendor).Error; err != nil {
						return err
					}
					record.VendorID = vendor.Id
				}
				record.UpdatedTime = now
				if change.Action == "create" {
					record.CreatedTime = now
					if err := tx.Create(&record).Error; err != nil {
						return err
					}
				} else {
					var old Model
					if err := tx.First(&old, "model_name = ?", change.Key).Error; err != nil {
						return err
					}
					record.Id, record.CreatedTime = old.Id, old.CreatedTime
				}
				record.Status, record.SyncOfficial = value.Status, value.SyncOfficial
				var fields map[string]common.RawMessage
				if err := common.UnmarshalJsonStr(change.After.Value, &fields); err != nil {
					return err
				}
				delete(fields, "vendor")
				delete(fields, "created_time")
				delete(fields, "updated_time")
				columns := append(slices.Sorted(maps.Keys(fields)), "vendor_id", "updated_time")
				if err := tx.Model(&record).Select(columns).Updates(&record).Error; err != nil {
					return err
				}
				// BeforeCreate and GORM's NOT NULL serializer normalize nil
				// slices. Preserve the attested JSON value, not SQL NULL or a
				// fabricated empty array; the ordinary JSON scanner reads both.
				arrays := make(map[string]any)
				for _, name := range []string{"supported_parameters", "supported_resolutions", "supported_aspect_ratios", "output_formats", "reference_modalities"} {
					arrays[name] = string(fields[name])
				}
				if err := tx.Model(&record).Updates(arrays).Error; err != nil {
					return err
				}
			}
		}
	}
	options := make(map[string]*Option)
	for _, change := range plan.Changes {
		if change.Kind == catalogmanifest.KindModel || change.Kind == catalogmanifest.KindVendor || change.Action != "create" && change.Action != "update" && change.Action != "delete" {
			continue
		}
		key, err := catalogmanifest.DecodePriceKey(change.Key)
		if err != nil {
			return err
		}
		value, exists := pricing[key.Option]
		if !exists {
			return ErrCatalogSyncPlanStale
		}
		options[key.Option] = &Option{Key: key.Option, Value: value}
	}
	for _, key := range slices.Sorted(maps.Keys(options)) {
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(options[key]).Error; err != nil {
			return err
		}
	}
	for _, kind := range []string{catalogmanifest.KindModel, catalogmanifest.KindVendor} {
		for _, change := range plan.Changes {
			if change.Kind != kind || change.Action != "delete" {
				continue
			}
			var err error
			if kind == catalogmanifest.KindModel {
				err = tx.Where("model_name = ?", change.Key).Delete(&Model{}).Error
			} else {
				err = tx.Where("name = ?", change.Key).Delete(&Vendor{}).Error
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// The exact option postimage, shared by prospective staging and the writer.
// Untouched raw rows retain byte identity. Adoption is strict even when it
// changes ownership only; missing/default values never stand in for that leaf.
func catalogPlanPricing(previous map[string]string, plan catalogmanifest.Plan) (map[string]string, []catalogPricingLeaf, error) {
	next := maps.Clone(previous)
	strict := []catalogPricingLeaf{}
	for _, change := range plan.Changes {
		if change.Kind == catalogmanifest.KindModel || change.Kind == catalogmanifest.KindVendor || (change.Action != "create" && change.Action != "update" && change.Action != "delete" && change.Action != "adopt") {
			continue
		}
		key, err := catalogmanifest.DecodePriceKey(change.Key)
		if err != nil {
			return nil, nil, err
		}
		leaf := strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(key.Path, "/"), "~1", "/"), "~0", "~")
		if change.Action != "delete" && change.After != nil {
			strict = append(strict, catalogPricingLeaf{Option: key.Option, Name: leaf})
		}
		if change.Action == "adopt" {
			continue
		}
		var value catalogmanifest.PriceValue
		if change.After != nil {
			if err := common.UnmarshalJsonStr(change.After.Value, &value); err != nil {
				return nil, nil, err
			}
		}
		if catalogmanifest.PriceOptions()[key.Option].Map {
			raw, exists := next[key.Option]
			if !exists {
				raw = "{}"
			}
			leaves, err := decodeCatalogPricingMap(key.Option, raw)
			if err != nil {
				return nil, nil, err
			}
			if change.Action == "delete" {
				delete(leaves, leaf)
			} else {
				leaves[leaf] = common.RawMessage(value.Value)
			}
			encoded, err := common.Marshal(leaves)
			if err != nil {
				return nil, nil, err
			}
			next[key.Option] = string(encoded)
		} else {
			if change.Action == "delete" {
				return nil, nil, ErrCatalogSyncPlanStale
			}
			next[key.Option] = value.Value
		}
	}
	return next, strict, nil
}

// Verify only structural persisted equality, not expressions or evaluators.
// Local audit timestamps and incarnation tokens are intentionally local; all
// business metadata, prices and retained target-only entries must match Draft.
func verifyCatalogWrittenDraft(before, after catalogmanifest.Snapshot, plan catalogmanifest.Plan) error {
	draft, _, err := catalogPlanDraft(before, plan)
	if err != nil {
		return err
	}
	actual := make(map[string]catalogmanifest.Entry)
	for _, entry := range after.Entries {
		actual[catalogmanifest.EntryID(entry)] = entry
	}
	for i := range draft.Entries {
		entry := &draft.Entries[i]
		if entry.Kind != catalogmanifest.KindVendor && entry.Kind != catalogmanifest.KindModel {
			continue
		}
		persisted, exists := actual[catalogmanifest.EntryID(*entry)]
		if !exists {
			return ErrCatalogSyncPlanStale
		}
		var wanted, got map[string]common.RawMessage
		if err := common.UnmarshalJsonStr(entry.Value, &wanted); err != nil {
			return err
		}
		if err := common.UnmarshalJsonStr(persisted.Value, &got); err != nil {
			return err
		}
		wanted["created_time"], wanted["updated_time"] = got["created_time"], got["updated_time"]
		encoded, err := common.Marshal(wanted)
		if err != nil {
			return err
		}
		entry.Value = string(encoded)
	}
	draft.ObjectVersions = after.ObjectVersions
	wanted, err := catalogmanifest.CanonicalSnapshotContent(draft)
	if err != nil {
		return err
	}
	got, err := catalogmanifest.CanonicalSnapshotContent(after)
	if err != nil {
		return err
	}
	if wanted != got {
		return ErrCatalogSyncPlanStale
	}
	return nil
}
