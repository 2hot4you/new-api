package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"gorm.io/gorm"
)

// Accessed only under catalogBarrier. A durable ready row alone does not prove
// that this process has loaded it (in particular after restart or failed ack).
var catalogRuntime struct {
	db       *gorm.DB
	revision int64
	ready    bool
}

type catalogRuntimeInput struct {
	state        CatalogSyncState
	options      map[string]string
	dependencies catalogValidationInput
	digest       string
	operation    CatalogSyncOperation
	generation   *jsplugin.RoutingGeneration
	currencies   map[string]ModelBillingCurrency
}

type catalogRuntimeStage struct {
	input         catalogRuntimeInput
	candidate     *catalogPricingCandidate
	aliases       map[string]TaskAliasTarget
	compatibility map[string]bool
}

// An ordinary mutation has no managed plan, backup or operation. Preparation
// captures actual previous facts before constructing a detached prospective
// candidate. Only commitOrdinaryCatalogMutationGuarded may bind its postimage.
type catalogOrdinaryMutation struct {
	previous catalogRuntimeInput
	stage    *catalogRuntimeStage
	wrote    bool
	removal  *catalogModelRemoval
}

func prepareOrdinaryCatalogMutation(ctx context.Context, db *gorm.DB, updates map[string]string) (*catalogOrdinaryMutation, error) {
	previous, err := captureOrdinaryCatalogMutation(ctx, db)
	if err != nil {
		return nil, err
	}
	return stageOrdinaryCatalogMutation(previous, updates)
}

// Capture ends the reference transaction, registry pin and writer before any
// save-specific validation or prospective compilation starts.
func captureOrdinaryCatalogMutation(ctx context.Context, db *gorm.DB) (catalogRuntimeInput, error) {
	return captureOrdinaryCatalogMutationWithRemoval(ctx, db, nil)
}

// removal is reserved for DeleteModelMetadata's explicit channel-removal
// operation, never a speculative business callback or generic drift allowance.
func captureOrdinaryCatalogMutationWithRemoval(ctx context.Context, db *gorm.DB, removal *catalogModelRemoval) (catalogRuntimeInput, error) {
	if err := ctx.Err(); err != nil {
		return catalogRuntimeInput{}, err
	}
	if db == nil || DB == nil || db.Statement.ConnPool != DB.Statement.ConnPool {
		return catalogRuntimeInput{}, errors.New("ordinary catalog publication requires the active root database")
	}
	var previous catalogRuntimeInput
	err := WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(ctx, db, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			if state.PublicationState != "ready" || state.PendingOperationID != "" || state.Revision != state.RuntimeRevision {
				return ErrCatalogPublicationPending
			}
			var err error
			previous, err = captureCatalogRuntimeTx(tx, state, pin)
			if err == nil && removal != nil {
				err = removal.capture(tx)
			}
			return err
		})
	})
	return previous, err
}

func stageOrdinaryCatalogMutation(previous catalogRuntimeInput, updates map[string]string) (*catalogOrdinaryMutation, error) {
	prospective := previous
	prospective.options = maps.Clone(previous.options)
	whitelist := catalogmanifest.PriceOptions()
	for key, value := range updates {
		if _, known := whitelist[key]; known || catalogmanifest.IsPricingNamespace(key) || key == "USDExchangeRate" {
			prospective.options[key] = value
		}
	}
	stage, err := stageProspectiveCatalogRuntime(prospective, previous.options, nil)
	if err != nil {
		return nil, err
	}
	return &catalogOrdinaryMutation{previous: previous, stage: stage}, nil
}

// Caller owns the final common writer; write is SQL-only and invoked once,
// after a fresh authoritative recheck. It must never compile or publish, commit
// its transaction, or assume that an error means its COMMIT did not happen.
// publishOther runs after the real commit/pin release but before the catalog
// acknowledgement, for supplied noncatalog values in the same transaction.
func commitOrdinaryCatalogMutationGuarded(ctx context.Context, db *gorm.DB, prepared *catalogOrdinaryMutation, write func(*gorm.DB) error, publishOther func() error) error {
	changed := false
	err := catalogReferenceTransaction(ctx, db, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
		fresh, err := captureCatalogRuntimeTx(tx, state, pin)
		if err != nil {
			return err
		}
		if !prepared.previous.same(fresh) {
			return ErrCatalogSyncPlanStale
		}
		if prepared.removal != nil {
			if err := prepared.removal.verify(tx, false); err != nil {
				return err
			}
		}
		prepared.wrote = true
		if err := write(tx); err != nil {
			return err
		}
		post, err := captureCatalogRuntimeTx(tx, state, pin)
		if err != nil {
			return err
		}
		expectedDependencies := fresh.dependencies
		if prepared.removal != nil {
			if err := prepared.removal.verify(tx, true); err != nil {
				return err
			}
			expectedDependencies = prepared.stage.input.dependencies
		}
		if !maps.Equal(post.options, prepared.stage.input.options) || post.dependencies != expectedDependencies || post.generation != fresh.generation || post.state != fresh.state {
			return ErrCatalogSyncPlanStale
		}
		changed = post.digest != fresh.digest
		if changed {
			state.Revision++
			state.CurrentDigest = post.digest
			state.PublicationState = "committed_pending_publish"
			if err := tx.Model(&CatalogSyncState{}).Where("id = ?", state.ID).Updates(map[string]any{
				"revision": state.Revision, "current_digest": state.CurrentDigest,
				"publication_state": state.PublicationState, "pending_operation_id": "",
			}).Error; err != nil {
				return err
			}
			post.state = *state
		}
		prepared.stage.input = post
		return nil
	})
	if errors.Is(err, ErrCatalogCommitUncertain) && changed && prepared.stage.input.state.PublicationState == "committed_pending_publish" {
		// Inspect the exact durable postimage without replaying write. Even a
		// match cannot identify this attempt against another process: ordinary
		// mutations have no unique operation ID. Retain uncertainty and pending;
		// explicit recovery can publish the authoritative committed revision.
		lookup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		lookupErr := catalogReferenceTransaction(lookup, db, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			actual, captureErr := captureCatalogRuntimeTx(tx, state, pin)
			if captureErr != nil {
				return captureErr
			}
			if !prepared.stage.input.same(actual) {
				return ErrCatalogSyncPlanStale
			}
			return nil
		})
		if lookupErr != nil {
			err = errors.Join(err, lookupErr)
		}
	}
	if err != nil {
		if errors.Is(err, ErrCatalogCommitUncertain) {
			catalogRuntime.ready = false
		}
		return err
	}
	if publishOther != nil {
		if err := publishOther(); err != nil {
			if changed {
				catalogRuntime.ready = false
			}
			return err
		}
	}
	if !changed {
		// A canonical no-op may still persist a new raw JSON representation.
		// Publish the complete detached candidate after commit/pin release,
		// under the retained writer, without changing revision or readiness.
		prepared.stage.candidate.publishGuarded()
		return nil
	}
	return publishCatalogRuntimeGuarded(ctx, prepared.stage)
}

// Only preparation may be repeated after another writer wins the fresh
// recheck. Once the SQL callback starts, every result (including uncertainty)
// returns directly. This preserves concurrent ordinary appends without replaying
// closures that mutate IDs, counters or caller-owned result objects.
func withOrdinaryCatalogMutation(db *gorm.DB, updates map[string]string, commit func(*catalogOrdinaryMutation) error) error {
	return withPreparedOrdinaryCatalogMutation(func(ctx context.Context) (*catalogOrdinaryMutation, error) {
		return prepareOrdinaryCatalogMutation(ctx, db, updates)
	}, commit)
}

// prepare is a detached, repeatable preparation, never a business SQL callback.
func withPreparedOrdinaryCatalogMutation(prepare func(context.Context) (*catalogOrdinaryMutation, error), commit func(*catalogOrdinaryMutation) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for range 64 {
		if err := ctx.Err(); err != nil {
			return err
		}
		prepared, err := prepare(ctx)
		if err != nil {
			return err
		}
		err = WithCatalogWriteBarrier(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return commit(prepared)
		})
		if !errors.Is(err, ErrCatalogSyncPlanStale) || prepared.wrote {
			return err
		}
	}
	return ErrCatalogSyncPlanStale
}

// The empty manifests are ONLY an adapter to the existing desired/effective
// dependency capture. Actual committed options and metadata are bound below;
// this is never managed-source completeness or prospective-write attestation.
func captureCatalogRuntimeTx(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) (catalogRuntimeInput, error) {
	input := catalogRuntimeInput{state: *state, options: make(map[string]string), generation: pin.Generation}
	if state.PendingOperationID != "" {
		if err := catalogSensitiveInstanceTx(tx); err != nil {
			return input, err
		}
	}
	empty := catalogmanifest.Snapshot{SchemaVersion: catalogmanifest.SchemaVersion, SourceID: "runtime-dependencies", Complete: true, Capabilities: catalogmanifest.RequiredCapabilities(), Coverage: map[string]int{}}
	for _, kind := range catalogmanifest.Kinds() {
		empty.Coverage[kind] = 0
	}
	var err error
	empty.Digest, err = catalogmanifest.SnapshotDigest(empty)
	if err != nil {
		return input, err
	}
	input.dependencies, err = captureCatalogValidationInputTx(tx, pin, empty, empty, nil)
	if err != nil {
		return input, err
	}
	var options []Option
	if err := tx.Find(&options).Error; err != nil {
		return input, err
	}
	whitelist := catalogmanifest.PriceOptions()
	for _, option := range options {
		_, known := whitelist[option.Key]
		if !known && !catalogmanifest.IsPricingNamespace(option.Key) && option.Key != "USDExchangeRate" {
			continue
		}
		if _, duplicate := input.options[option.Key]; duplicate {
			return input, errors.New("ambiguous committed catalog options")
		}
		input.options[option.Key] = option.Value
	}
	// Includes full active model/vendor rows, hence raw currency, match rules,
	// metadata and local incarnation IDs; no strict Web orphan-price check.
	var currencies []Model
	if err := tx.Select("model_name", "billing_currency", "name_rule").Find(&currencies).Error; err != nil {
		return input, err
	}
	input.currencies = make(map[string]ModelBillingCurrency)
	for _, model := range currencies {
		currency, err := normalizeModelBillingCurrency(model.BillingCurrency)
		if err != nil {
			return input, fmt.Errorf("model %s billing currency: %w", model.ModelName, err)
		}
		if model.NameRule == NameRuleExact {
			input.currencies[model.ModelName] = ModelBillingCurrency{BillingCurrency: currency, HasMetadata: true}
		}
	}
	input.digest, err = catalogPersistedDigest(tx)
	if err != nil {
		return input, err
	}
	if state.PendingOperationID != "" {
		if err := tx.First(&input.operation, "id = ?", state.PendingOperationID).Error; err != nil {
			return input, err
		}
		var binding catalogOperationBinding
		if err := common.UnmarshalJsonStr(string(input.operation.History), &binding); err != nil || !validCatalogActor(binding.Actor) || binding.Digest == "" {
			return input, ErrCatalogPublicationPending
		}
		result, found, err := catalogOperationResult(tx, input.operation.ID, binding)
		if err != nil || !found || result.Revision != state.Revision || input.operation.Revision != state.Revision || input.operation.State != result.State {
			return input, ErrCatalogPublicationPending
		}
	}
	return input, nil
}

func (input catalogRuntimeInput) same(other catalogRuntimeInput) bool {
	return input.state == other.state && input.digest == other.digest && input.dependencies == other.dependencies && input.generation == other.generation && maps.Equal(input.options, other.options) && input.operation.ID == other.operation.ID && input.operation.PlanID == other.operation.PlanID && input.operation.CreatedAt == other.operation.CreatedAt && input.operation.State == other.operation.State && input.operation.Revision == other.operation.Revision && input.operation.Backup == other.operation.Backup && input.operation.History == other.operation.History && input.operation.Result == other.operation.Result
}

// Committed compiler/smoke phase. Its argument owns detached committed facts;
// callers must release SQL transactions, registry pins and catalog locks first.
func stageCatalogRuntime(input catalogRuntimeInput) (*catalogRuntimeStage, error) {
	dependencies, err := catalogRuntimeDependencies(input)
	if err != nil {
		return nil, err
	}
	candidate, err := stageCommittedCatalogPricing(input.options, dependencies)
	if err != nil {
		return nil, err
	}
	return stageCatalogCompatibility(input, candidate, dependencies)
}

// Prospective callers supply actual persisted previous rows and strict leaves
// derived from their sealed plan. The committed entry point above is reserved
// for recovery; this private seam shares only detached compatibility staging.
func stageProspectiveCatalogRuntime(input catalogRuntimeInput, previous map[string]string, strict []catalogPricingLeaf) (*catalogRuntimeStage, error) {
	dependencies, err := catalogRuntimeDependencies(input)
	if err != nil {
		return nil, err
	}
	candidate, err := stageProspectiveCatalogPricing(input.options, previous, dependencies, strict)
	if err != nil {
		return nil, err
	}
	return stageCatalogCompatibility(input, candidate, dependencies)
}

func catalogRuntimeDependencies(input catalogRuntimeInput) (catalogPricingDependencies, error) {
	var facts catalogValidationData
	if err := common.UnmarshalJsonStr(input.dependencies.canonical, &facts); err != nil {
		return catalogPricingDependencies{}, err
	}
	dependencies := catalogPricingDependencies{Plugins: make(map[string]jsplugin.Meta), Aliases: facts.Aliases}
	for key, plugin := range facts.Effective {
		dependencies.Plugins[key] = jsplugin.Meta{Key: key, APIVersion: plugin.APIVersion, Version: plugin.Version, Models: plugin.Models, UsageSchema: plugin.UsageSchema, UsageProfiles: plugin.UsageProfiles, RequiredCapabilities: plugin.Capabilities}
	}
	return dependencies, nil
}

func stageCatalogCompatibility(input catalogRuntimeInput, candidate *catalogPricingCandidate, dependencies catalogPricingDependencies) (*catalogRuntimeStage, error) {
	stage := &catalogRuntimeStage{input: input, candidate: candidate, aliases: dependencies.Aliases, compatibility: make(map[string]bool)}
	// Cover every possible existing resolver result, including empty/unconfigured
	// and built-in fallback. The resolver still runs in the existing projection.
	expressions := map[string]bool{"": true}
	for _, values := range []map[string]string{candidate.billing.BillingExpr, candidate.billing.PluginBillingExpr, billing_setting.GetBuiltinBillingExprCopy()} {
		for _, expression := range values {
			expressions[expression] = true
		}
	}
	for _, plugin := range dependencies.Plugins {
		for _, name := range plugin.Models {
			schema, _ := plugin.UsageForModel(name)
			for expression := range expressions {
				key, err := catalogCompatibilityKey(expression, schema)
				if err != nil {
					return nil, err
				}
				if _, staged := stage.compatibility[key]; !staged {
					stage.compatibility[key] = billing_setting.TaskExprCompatible(expression, schema)
				}
			}
		}
	}
	return stage, nil
}

func catalogCompatibilityKey(expression string, schema map[string]jsplugin.UsageFieldSchema) (string, error) {
	if schema == nil {
		schema = map[string]jsplugin.UsageFieldSchema{}
	}
	encoded, err := common.Marshal([]any{expression, schema})
	return string(encoded), err
}

func (stage *catalogRuntimeStage) compatible(expression string, schema map[string]jsplugin.UsageFieldSchema) (bool, error) {
	key, err := catalogCompatibilityKey(expression, schema)
	if err != nil {
		return false, err
	}
	compatible, exists := stage.compatibility[key]
	if !exists {
		return false, ErrCatalogSyncPlanStale
	}
	return compatible, nil
}

func PublishCatalogSyncRevision(ctx context.Context, revision int64) error {
	return publishCatalogSyncRevision(ctx, revision, nil)
}

// publishOptions is reserved for authoritative option reload. It runs after
// the final fresh recheck and real pin release, under the continuous writer.
func publishCatalogSyncRevision(ctx context.Context, revision int64, publishOptions func(context.Context, *catalogRuntimeStage) error) error {
	var input catalogRuntimeInput
	var captureErr error
	err := TryWithCatalogWriteBarrier(ctx, func() error {
		return catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			if state.Revision != revision || state.RuntimeRevision > revision || (state.PublicationState != "ready" && state.PublicationState != "committed_pending_publish") {
				return ErrCatalogSyncPlanStale
			}
			// A recovery of an apparently ready row must block ordinary writers
			// too: this process has not yet established complete runtime readiness.
			// No catalog value, revision, baseline or operation result is changed.
			if state.PublicationState == "ready" {
				if state.PendingOperationID != "" {
					return ErrCatalogSyncPlanStale
				}
				if err := tx.Model(&CatalogSyncState{}).Where("id = ?", state.ID).Update("publication_state", "committed_pending_publish").Error; err != nil {
					return err
				}
				state.PublicationState = "committed_pending_publish"
			}
			input, captureErr = captureCatalogRuntimeTx(tx, state, pin)
			// Retain the durable gate even if committed data/dependencies cannot
			// be reconstructed. Returning captureErr here would undo that gate.
			return nil
		})
	})
	if errors.Is(err, ErrCatalogCommitUncertain) {
		lookup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var state CatalogSyncState
		lookupErr := DB.WithContext(lookup).First(&state, CatalogSyncStateID).Error
		if lookupErr == nil && state == input.state && state.PublicationState == "committed_pending_publish" {
			err = nil
		}
	}
	if err != nil {
		return err
	}
	if captureErr != nil {
		return captureErr
	}
	stage, err := stageCatalogRuntime(input)
	if err != nil {
		return err
	}
	return TryWithCatalogWriteBarrier(ctx, func() error { return publishCatalogRuntimeAndOptionsGuarded(ctx, stage, publishOptions) })
}

func RecoverCatalogSyncRuntime(ctx context.Context) error {
	return recoverCatalogSyncRuntime(ctx, nil)
}

func recoverCatalogSyncRuntime(ctx context.Context, publishOptions func(context.Context, *catalogRuntimeStage) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var state CatalogSyncState
	if err := DB.WithContext(ctx).First(&state, CatalogSyncStateID).Error; err != nil {
		return err
	}
	return publishCatalogSyncRevision(ctx, state.Revision, publishOptions)
}

// Caller owns the common writer. Recheck and ack use real root transactions;
// typed publication and derived rebuild happen only AFTER their pins release.
func publishCatalogRuntimeGuarded(ctx context.Context, stage *catalogRuntimeStage) error {
	return publishCatalogRuntimeAndOptionsGuarded(ctx, stage, nil)
}

func publishCatalogRuntimeAndOptionsGuarded(ctx context.Context, stage *catalogRuntimeStage, publishOptions func(context.Context, *catalogRuntimeStage) error) error {
	if stage == nil || stage.input.state.PublicationState != "committed_pending_publish" {
		return ErrCatalogPublicationPending
	}
	err := catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
		fresh, err := captureCatalogRuntimeTx(tx, state, pin)
		if err != nil {
			return err
		}
		if !stage.input.same(fresh) {
			return ErrCatalogSyncPlanStale
		}
		return nil
	})
	if err != nil {
		return err
	}
	catalogRuntime.ready = false
	if publishOptions != nil {
		if err := publishOptions(ctx, stage); err != nil {
			return err
		}
	}
	stage.candidate.publishGuarded()
	if common.MemoryCacheEnabled {
		if err := refreshChannelCache(DB.WithContext(ctx), true); err != nil {
			return err
		}
	}
	if err := publishCatalogTaskAliasView(stage); err != nil {
		return err
	}
	if err := refreshCatalogPricingGuarded(ctx, stage); err != nil {
		return err
	}
	err = catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
		fresh, err := captureCatalogRuntimeTx(tx, state, pin)
		if err != nil {
			return err
		}
		if !stage.input.same(fresh) {
			return ErrCatalogSyncPlanStale
		}
		if state.PendingOperationID != "" {
			result := tx.Model(&CatalogSyncOperation{}).Where("id = ? AND revision = ? AND state = ?", state.PendingOperationID, state.Revision, "committed_pending_publish").Update("state", "succeeded")
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrCatalogSyncPlanStale
			}
		}
		return tx.Model(&CatalogSyncState{}).Where("id = ? AND revision = ? AND baseline_generation = ?", CatalogSyncStateID, state.Revision, state.BaselineGeneration).Updates(map[string]any{"runtime_revision": state.Revision, "publication_state": "ready", "pending_operation_id": ""}).Error
	})
	if errors.Is(err, ErrCatalogCommitUncertain) {
		// An unknown acknowledgement is reconciled, never retried as a write.
		lookup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var state CatalogSyncState
		lookupErr := DB.WithContext(lookup).First(&state, CatalogSyncStateID).Error
		expected := stage.input.state
		expected.RuntimeRevision, expected.PublicationState, expected.PendingOperationID = expected.Revision, "ready", ""
		if lookupErr == nil && state == expected {
			if stage.input.operation.ID != "" {
				var operation CatalogSyncOperation
				lookupErr = DB.WithContext(lookup).First(&operation, "id = ?", stage.input.operation.ID).Error
				if lookupErr == nil && (operation.State != "succeeded" || operation.Revision != state.Revision || operation.PlanID != stage.input.operation.PlanID || operation.Backup != stage.input.operation.Backup || operation.Result != stage.input.operation.Result || operation.History != stage.input.operation.History) {
					lookupErr = ErrCatalogSyncPlanStale
				}
			}
			if lookupErr == nil {
				err = nil
			}
		}
	}
	if err != nil {
		return err
	}
	catalogRuntime.db, catalogRuntime.revision, catalogRuntime.ready = DB, stage.input.state.Revision, true
	return nil
}

// Copy immutable selection only. The callback must not execute expressions,
// plugins, network/body work, or reenter a catalog barrier. Real consumers are
// integrated in Task5b; existing frozen requests do not call this boundary.
func WithCatalogPricingRead(ctx context.Context, modelName string, capture func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !catalogBarrier.TryRLock() {
		return ErrCatalogWriterBusy
	}
	defer catalogBarrier.RUnlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if capture == nil || strings.TrimSpace(modelName) == "" || !catalogRuntime.ready || catalogRuntime.db != DB {
		return ErrCatalogPublicationPending
	}
	options := &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}
	if DB.Dialector.Name() == "sqlite" {
		options = nil
	}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state CatalogSyncState
		if err := tx.First(&state, CatalogSyncStateID).Error; err != nil {
			return err
		}
		if state.PublicationState == "ready" && state.PendingOperationID == "" && state.Revision == state.RuntimeRevision && state.RuntimeRevision == catalogRuntime.revision {
			return nil
		}
		if state.PublicationState != "committed_pending_publish" || state.RuntimeRevision != catalogRuntime.revision || state.Revision != state.RuntimeRevision+1 || state.PendingOperationID == "" {
			return ErrCatalogPublicationPending
		}
		var operation CatalogSyncOperation
		if err := tx.First(&operation, "id = ?", state.PendingOperationID).Error; err != nil {
			return ErrCatalogPublicationPending
		}
		var binding catalogOperationBinding
		if err := common.UnmarshalJsonStr(string(operation.History), &binding); err != nil {
			return ErrCatalogPublicationPending
		}
		result, found, err := catalogOperationResult(tx, operation.ID, binding)
		if err != nil || !found || result.Revision != state.Revision || operation.State != "committed_pending_publish" {
			return ErrCatalogPublicationPending
		}
		var backup catalogOperationBackup
		if err := common.UnmarshalJsonStr(string(operation.Backup), &backup); err != nil || backup.Version != 1 || len(backup.Before) == 0 {
			return ErrCatalogPublicationPending
		}
		before := make(map[string]catalogBackupEntry, len(backup.Before))
		for _, entry := range backup.Before {
			id := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: entry.Kind, Key: entry.Key})
			if _, duplicate := before[id]; duplicate || strings.TrimSpace(entry.Key) == "" {
				return ErrCatalogPublicationPending
			}
			before[id] = entry
			switch entry.Kind {
			case catalogmanifest.KindVendor:
				// Vendor display metadata does not select a billing price.
				if entry.Exists && (entry.Vendor == nil || entry.Vendor.Name != entry.Key) {
					return ErrCatalogPublicationPending
				}
			case catalogmanifest.KindModel:
				if entry.Key == modelName || (entry.Exists && (entry.Model == nil || entry.Model.ModelName != entry.Key || entry.Model.NameRule != NameRuleExact)) {
					return ErrCatalogPublicationPending
				}
			default:
				// Prices may have wildcard, family, plugin or built-in fallback
				// consumers. Until a complete scope proof exists, gate all new
				// selections for any price change or unknown backup identity.
				return ErrCatalogPublicationPending
			}
		}
		seen := make(map[string]bool, len(backup.After))
		for _, entry := range backup.After {
			id := catalogmanifest.EntryID(entry)
			if _, exists := before[id]; !exists || seen[id] {
				return ErrCatalogPublicationPending
			}
			seen[id] = true
			if entry.Kind == catalogmanifest.KindModel {
				var model catalogmanifest.ModelValue
				if err := common.UnmarshalJsonStr(entry.Value, &model); err != nil || model.ModelName != entry.Key || model.NameRule != NameRuleExact {
					return ErrCatalogPublicationPending
				}
			} else {
				var vendor catalogmanifest.VendorValue
				if err := common.UnmarshalJsonStr(entry.Value, &vendor); err != nil || vendor.Name != entry.Key {
					return ErrCatalogPublicationPending
				}
			}
		}
		for id, entry := range before {
			if !entry.Exists && !seen[id] {
				return ErrCatalogPublicationPending
			}
		}
		return nil
	}, options)
	if err != nil {
		return fmt.Errorf("catalog pricing selection: %w", err)
	}
	return capture()
}
