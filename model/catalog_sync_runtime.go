package model

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingmoney"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Accessed only under catalogBarrier. A durable ready row alone does not prove
// that this process has loaded it (in particular after restart or failed ack).
var catalogRuntime struct {
	db       *gorm.DB
	revision int64
	ready    bool
	input    *catalogRuntimeInput
}

// A failed authoritative capture must be sticky even while other readers keep
// the writer unavailable. This DB-scoped latch is not a readiness proof; only a
// complete successfully acknowledged reconstruction may clear it.
var catalogRuntimeInvalid atomic.Pointer[gorm.DB]

type catalogRuntimeInput struct {
	state        CatalogSyncState
	options      map[string]string
	dependencies catalogValidationInput
	digest       string
	operation    CatalogSyncOperation
	generation   *jsplugin.RoutingGeneration
	currencies   map[string]ModelBillingCurrency
	channels     []*Channel
	abilities    []Ability
	metadata     []Model
	vendors      []Vendor
	routing      string
	exchangeRate float64
	fingerprint  string
}

type catalogRuntimeStage struct {
	input           catalogRuntimeInput
	candidate       *catalogPricingCandidate
	aliases         map[string]TaskAliasTarget
	compatibility   map[string]bool
	projection      *catalogPricingProjection
	channelCache    *channelCacheProjection
	optionsOnly     bool // complete unchanged epoch: publish no pricing/FX/registry globals
	optionMapLocked bool // install reservation owns OptionMap across the entire batch
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
	err := withCatalogBarrierContext(ctx, true, func() error {
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
	ctx = catalogSyncContext(ctx, "ordinary")
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
		changed = post.digest != fresh.digest || post.exchangeRate != fresh.exchangeRate
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
	if changed {
		observeCatalogPending(ctx, &prepared.stage.input, true)
	}
	if !changed && prepared.previous.state.RuntimeFingerprint != "" {
		acknowledged := catalogRuntime.ready && catalogRuntime.db == DB && catalogRuntimeInvalid.Load() != DB && catalogRuntime.input != nil && prepared.previous.fingerprint == prepared.previous.state.RuntimeFingerprint && prepared.previous.same(*catalogRuntime.input)
		if !acknowledged {
			// SQL-relative no-op does not mean runtime-relative no-op. In
			// particular external same-revision price+FX drift must never swap
			// its typed prices while retaining an old acknowledged money input.
			catalogRuntime.ready = false
			catalogRuntimeInvalid.Store(DB)
			var gateErr error
			if prepared.previous.fingerprint != prepared.previous.state.RuntimeFingerprint {
				gateErr = catalogReferenceTransaction(ctx, db, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
					fresh, err := captureCatalogRuntimeTx(tx, state, pin)
					if err != nil {
						return err
					}
					if !prepared.stage.input.same(fresh) {
						return ErrCatalogSyncPlanStale
					}
					return tx.Model(&CatalogSyncState{}).Where("id = ?", state.ID).Update("publication_state", "committed_pending_publish").Error
				})
			}
			return errors.Join(ErrCatalogPublicationPending, gateErr)
		}
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
	canonicalOptions, exchangeRate, err := canonicalCatalogRuntimeOptions(input.options)
	if err != nil {
		return input, err
	}
	input.exchangeRate = exchangeRate
	if err := tx.Order("id").Find(&input.channels).Error; err != nil {
		return input, err
	}
	if err := tx.Order(clause.OrderByColumn{Column: clause.Column{Name: "group"}}).Order("model").Order("channel_id").Find(&input.abilities).Error; err != nil {
		return input, err
	}
	// Accounting/health observations and live polling progress are not routing
	// configuration. Including them would continuously invalidate publication
	// under traffic and could starve recovery even without an admin mutation.
	routingChannels := make([]Channel, len(input.channels))
	for i, channel := range input.channels {
		routingChannels[i] = *channel
		routingChannels[i].UsedQuota, routingChannels[i].Balance, routingChannels[i].BalanceUpdatedTime = 0, 0, 0
		routingChannels[i].ResponseTime, routingChannels[i].TestTime = 0, 0
		routingChannels[i].ChannelInfo.MultiKeyPollingIndex = 0
		routingChannels[i].OtherSettings, err = catalogRoutingOtherSettings(channel.OtherSettings)
		if err != nil {
			if channel.Type == constant.ChannelTypeAdvancedCustom {
				if _, decodeErr := channel.parseOtherSettings(); decodeErr != nil {
					// Preserve the existing advanced-channel validation boundary:
					// bind malformed raw identity, then the mandatory lifecycle
					// projection rejects it before any install/ACK, even disabled
					// or without memory caching. Never repair the original channel.
					// Ambiguity accepted by the typed decoder still fails below.
					routingChannels[i].OtherSettings = channel.OtherSettings
					continue
				}
				return input, fmt.Errorf("decode advanced custom channel settings: channel_id=%d: %w", channel.Id, err)
			}
			return input, err
		}
	}
	routing, err := common.Marshal([]any{routingChannels, input.abilities})
	if err != nil {
		return input, err
	}
	input.routing = string(routing)
	if err := tx.Order("id").Find(&input.metadata).Error; err != nil {
		return input, err
	}
	if err := tx.Order("id").Find(&input.vendors).Error; err != nil {
		return input, err
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
	// No process-local generation number or pointer belongs in a shared epoch.
	// Canonical dependency facts bind plugin source/identity/schema instead.
	encoded, err := common.Marshal([]any{state.IncarnationKey, input.digest, canonicalOptions, input.dependencies.canonical, input.routing})
	if err != nil {
		return input, err
	}
	input.fingerprint = fmt.Sprintf("sha256:%x", sha256.Sum256(encoded))
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
	return input.state == other.state && input.routing == other.routing && input.digest == other.digest && input.dependencies == other.dependencies && input.generation == other.generation && input.fingerprint == other.fingerprint && input.operation.ID == other.operation.ID && input.operation.PlanID == other.operation.PlanID && input.operation.CreatedAt == other.operation.CreatedAt && input.operation.State == other.operation.State && input.operation.Revision == other.operation.Revision && input.operation.Backup == other.operation.Backup && input.operation.History == other.operation.History && input.operation.Result == other.operation.Result
}

// Normalize only routing identity, never the original channel or persisted JSON.
// RawMessage and CanonicalJSON retain unknown fields and exact decimal values.
func catalogRoutingOtherSettings(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return "", nil
	}
	if !catalogReferenceJSONUnambiguous(raw) {
		return "", errors.New("channel settings contain invalid or ambiguous JSON")
	}
	var fields map[string]common.RawMessage
	if err := common.UnmarshalJsonStr(raw, &fields); err != nil || fields == nil {
		return "", errors.New("channel settings must be a JSON object")
	}
	delete(fields, "upstream_model_update_last_check_time")
	delete(fields, "upstream_model_update_last_detected_models")
	delete(fields, "upstream_model_update_last_removed_models")
	if len(fields) == 0 {
		return "", nil
	}
	encoded, err := common.Marshal(fields)
	if err != nil {
		return "", err
	}
	return catalogmanifest.CanonicalJSON(string(encoded))
}

// Preserve exact accepted raw storage/display values separately from semantic
// epoch identity. CanonicalJSON normalizes decimals without float64 conversion;
// duplicate map identities still fail closed instead of being canonicalized
// into the same fingerprint as a valid last-key-wins interpretation.
func canonicalCatalogRuntimeOptions(options map[string]string) (map[string]string, float64, error) {
	canonical := maps.Clone(options)
	exchangeRate := 7.3
	if raw, exists := options["USDExchangeRate"]; exists {
		var err error
		exchangeRate, err = strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, 0, errors.New("catalog exchange rate is invalid")
		}
		canonical["USDExchangeRate"] = strconv.FormatFloat(exchangeRate, 'g', -1, 64)
	}
	if _, err := billingmoney.NewContext("CNY", exchangeRate); err != nil {
		return nil, 0, errors.New("catalog exchange rate is invalid")
	}
	whitelist := catalogmanifest.PriceOptions()
	for key, raw := range options {
		if key == "USDExchangeRate" {
			continue
		}
		if spec, known := whitelist[key]; known && spec.Map {
			if _, err := decodeCatalogPricingMap(key, raw); err != nil {
				return nil, 0, err
			}
		}
		if value, err := catalogmanifest.CanonicalJSON(raw); err == nil {
			canonical[key] = value
		}
	}
	return canonical, exchangeRate, nil
}

func (input catalogRuntimeInput) stale(other catalogRuntimeInput, phase string) error {
	return fmt.Errorf("catalog phase=%s state_changed=%t routing_changed=%t content_changed=%t dependencies_changed=%t generation_changed=%t options_changed=%t: %w", phase, input.state != other.state, input.routing != other.routing, input.digest != other.digest, input.dependencies != other.dependencies, input.generation != other.generation, !maps.Equal(input.options, other.options), ErrCatalogSyncPlanStale)
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
	err := publishCatalogSyncRevision(catalogSyncContext(ctx, "publication"), revision, nil)
	return errors.Join(ctx.Err(), err)
}

// publishOptions is reserved for authoritative option reload. It runs after
// the final fresh recheck and real pin release, under the continuous writer.
func publishCatalogSyncRevision(ctx context.Context, revision int64, publishOptions func(context.Context, *catalogRuntimeStage) error) error {
	var input catalogRuntimeInput
	var captureErr error
	var unchanged bool
	var transitioned bool
	// Exact identity, not errors.Is: cleanup/rollback errors joined to this
	// known pre-capture target conflict must still invalidate local selection.
	obsoleteTarget := fmt.Errorf("catalog expected revision is obsolete: %w", ErrCatalogSyncPlanStale)
	err := tryWithCatalogCaptureBarrier(ctx, func() (captureResult error) {
		activeDB := DB
		defer func() {
			if (captureResult != nil && captureResult != obsoleteTarget) || captureErr != nil {
				catalogRuntimeInvalid.Store(activeDB)
			}
		}()
		if trace, ok := ctx.Value(catalogSyncTraceKey{}).(*catalogSyncTrace); ok {
			trace.localRevision = catalogRuntime.revision
		}
		return catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			input.state = *state
			if state.PublicationState != "ready" && state.PublicationState != "committed_pending_publish" {
				return ErrCatalogPublicationPending
			}
			if state.Revision > revision && state.RuntimeRevision <= state.Revision {
				return obsoleteTarget
			}
			if state.Revision != revision || state.RuntimeRevision > revision {
				return ErrCatalogSyncPlanStale
			}
			input, captureErr = captureCatalogRuntimeTx(tx, state, pin)
			unchanged = captureErr == nil && state.PublicationState == "ready" && state.RuntimeFingerprint == input.fingerprint && catalogRuntimeInvalid.Load() != DB && catalogRuntime.ready && catalogRuntime.db == DB && catalogRuntime.input != nil && catalogRuntime.input.same(input)
			// A local startup has no published projection to invalidate. Keep
			// shared readiness only if committed content still matches its epoch.
			localStartup := captureErr == nil && state.RuntimeFingerprint != "" && state.RuntimeFingerprint == input.fingerprint
			if state.PublicationState == "ready" && !unchanged && !localStartup {
				if state.PendingOperationID != "" {
					return ErrCatalogSyncPlanStale
				}
				if err := tx.Model(&CatalogSyncState{}).Where("id = ?", state.ID).Update("publication_state", "committed_pending_publish").Error; err != nil {
					return err
				}
				state.PublicationState = "committed_pending_publish"
				input.state = *state
				transitioned = true
			}
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
		recordCatalogSyncPhase(ctx, "capture", &input, err)
		return err
	}
	observeCatalogPending(ctx, &input, transitioned)
	recordCatalogSyncPhase(ctx, "capture", &input, captureErr)
	if captureErr != nil {
		return captureErr
	}
	if unchanged {
		recordCatalogSyncPhase(ctx, "noop", &input, nil)
		if publishOptions == nil {
			return nil
		}
		loadOptions := func(optionsOnly bool) error {
			// Recheck the complete capture, not revision alone, before loading
			// unrelated settings. The loader may publish no new pricing/FX/plugin
			// input, so a read guard keeps healthy request selection available.
			if err := catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
				fresh, err := captureCatalogRuntimeTx(tx, state, pin)
				if err != nil {
					return err
				}
				if !input.same(fresh) {
					return ErrCatalogSyncPlanStale
				}
				return nil
			}); err != nil {
				catalogRuntimeInvalid.Store(DB)
				return err
			}
			err := publishOptions(ctx, &catalogRuntimeStage{input: input, optionsOnly: optionsOnly})
			if errors.Is(err, ErrCatalogSyncPlanStale) {
				catalogRuntimeInvalid.Store(DB)
			}
			recordCatalogSyncPhase(ctx, "options", &input, err)
			return err
		}
		err := tryWithCatalogCaptureBarrier(ctx, func() error { return loadOptions(true) })
		if errors.Is(err, errCatalogOptionsNeedWriter) {
			// No loader side effects have happened. End R before reserving W,
			// then repeat the complete freshness proof and load the current batch.
			// Contention returns to the coordinator with no locks held.
			err = withCatalogRecoveryWriter(ctx, func() error { return loadOptions(false) })
		}
		if errors.Is(err, ErrCatalogSyncPlanStale) {
			// End the first read guard before a fresh gate transaction. Close
			// only the same durable epoch, not a later publisher's ready epoch.
			gateErr := tryWithCatalogCaptureBarrier(ctx, func() error {
				return catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					if *state != input.state {
						return nil
					}
					return tx.Model(&CatalogSyncState{}).Where("id = ?", state.ID).Update("publication_state", "committed_pending_publish").Error
				})
			})
			err = errors.Join(err, gateErr)
		}
		return err
	}
	stage, err := stageCatalogRuntime(input)
	recordCatalogSyncPhase(ctx, "stage", &input, err)
	if err != nil {
		return err
	}
	err = withCatalogRecoveryWriter(ctx, func() error { return publishCatalogRuntimeAndOptionsGuarded(ctx, stage, publishOptions) })
	recordCatalogSyncPhase(ctx, "complete", &stage.input, err)
	return err
}

func RecoverCatalogSyncRuntime(ctx context.Context) error {
	return recoverCatalogSyncRuntime(ctx, nil)
}

func recoverCatalogSyncRuntime(ctx context.Context, publishOptions func(context.Context, *catalogRuntimeStage) error) error {
	ctx, cancel := context.WithTimeout(catalogSyncContext(ctx, "recovery"), 30*time.Second)
	defer cancel()
	select {
	case catalogRecoveryPermit <- struct{}{}:
		defer func() { <-catalogRecoveryPermit }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return retryCatalogRefresh(ctx, "recovery", func(ctx context.Context) error {
		var state CatalogSyncState
		if err := DB.WithContext(ctx).First(&state, CatalogSyncStateID).Error; err != nil {
			catalogRuntimeInvalid.Store(DB)
			return err
		}
		return publishCatalogSyncRevision(ctx, state.Revision, publishOptions)
	})
}

// Caller owns the common writer. Recheck and ack use real root transactions;
// typed publication and derived rebuild happen only AFTER their pins release.
func publishCatalogRuntimeGuarded(ctx context.Context, stage *catalogRuntimeStage) error {
	return publishCatalogRuntimeAndOptionsGuarded(ctx, stage, nil)
}

func publishCatalogRuntimeAndOptionsGuarded(ctx context.Context, stage *catalogRuntimeStage, publishOptions func(context.Context, *catalogRuntimeStage) error) error {
	ctx = catalogSyncContext(ctx, "publication")
	ctx.Value(catalogSyncTraceKey{}).(*catalogSyncTrace).localRevision = catalogRuntime.revision
	if stage == nil || (stage.input.state.PublicationState != "committed_pending_publish" && stage.input.state.PublicationState != "ready") {
		return ErrCatalogPublicationPending
	}
	observeCatalogPending(ctx, &stage.input, false)
	err := catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
		fresh, err := captureCatalogRuntimeTx(tx, state, pin)
		if err != nil {
			return err
		}
		if !stage.input.same(fresh) {
			return stage.input.stale(fresh, "recheck")
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Ordinary and managed commits bind their actual metadata/routing postimage
	// after prospective compilation. Build all derived views from that postimage
	// before acquiring install locks or making any globally visible exchange.
	stage.channelCache, err = buildChannelCacheProjection(nil, stage.input.channels, stage.input.abilities, true)
	if err != nil {
		return err
	}
	if err := updatePricingWithCatalogStage(nil, stage); err != nil {
		return err
	}
	release, err := acquireCatalogInstallLocks(ctx)
	recordCatalogSyncPhase(ctx, "acquire", &stage.input, err)
	if err != nil {
		return err
	}
	defer release()
	stage.optionMapLocked = true
	defer func() { stage.optionMapLocked = false }()
	// All locks are reserved before this final authoritative proof. A failed
	// proof leaves every old projection intact and releases every reservation.
	if err := catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
		fresh, err := captureCatalogRuntimeTx(tx, state, pin)
		if err != nil {
			return err
		}
		if !stage.input.same(fresh) {
			return stage.input.stale(fresh, "preinstall")
		}
		return nil
	}); err != nil {
		recordCatalogSyncPhase(ctx, "recheck", &stage.input, err)
		return err
	}
	recordCatalogSyncPhase(ctx, "recheck", &stage.input, nil)
	catalogRuntime.ready = false
	if publishOptions != nil {
		if err := publishOptions(ctx, stage); err != nil {
			return err
		}
	}
	rawFX, configuredFX := stage.input.options["USDExchangeRate"]
	if !configuredFX {
		rawFX = strconv.FormatFloat(stage.input.exchangeRate, 'g', -1, 64)
	}
	stage.candidate.publishWithMoneyLocked(stage.input.exchangeRate, rawFX)
	if common.MemoryCacheEnabled {
		stage.channelCache.install()
	}
	taskAliasViewPtr.Store(&taskAliasView{generation: stage.input.generation.Number, expiresAt: time.Now().Add(taskAliasViewTTL), byFold: maps.Clone(stage.aliases)})
	stage.projection.install()
	ratio_setting.InvalidateExposedDataCache()
	recordCatalogSyncPhase(ctx, "install", &stage.input, nil)
	release() // Readers see the complete bundle; SQL acknowledgement holds no cache locks.
	err = catalogReferenceTransaction(ctx, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
		fresh, err := captureCatalogRuntimeTx(tx, state, pin)
		if err != nil {
			return err
		}
		if !stage.input.same(fresh) {
			return stage.input.stale(fresh, "ack")
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
		return tx.Model(&CatalogSyncState{}).Where("id = ? AND revision = ? AND baseline_generation = ?", CatalogSyncStateID, state.Revision, state.BaselineGeneration).Updates(map[string]any{"runtime_revision": state.Revision, "runtime_fingerprint": stage.input.fingerprint, "current_digest": stage.input.digest, "publication_state": "ready", "pending_operation_id": ""}).Error
	})
	if errors.Is(err, ErrCatalogCommitUncertain) {
		// An unknown acknowledgement is reconciled, never retried as a write.
		lookup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var state CatalogSyncState
		lookupErr := DB.WithContext(lookup).First(&state, CatalogSyncStateID).Error
		expected := stage.input.state
		expected.RuntimeRevision, expected.PublicationState, expected.PendingOperationID = expected.Revision, "ready", ""
		expected.RuntimeFingerprint, expected.CurrentDigest = stage.input.fingerprint, stage.input.digest
		if lookupErr == nil && state == expected {
			expectedInput := stage.input
			expectedInput.state, expectedInput.operation = expected, CatalogSyncOperation{}
			lookupErr = catalogReferenceTransaction(lookup, DB, jsplugin.DefaultRegistry, func(tx *gorm.DB, actualState *CatalogSyncState, pin *jsplugin.GenerationPin) error {
				actual, err := captureCatalogRuntimeTx(tx, actualState, pin)
				if err != nil {
					return err
				}
				if !expectedInput.same(actual) {
					return expectedInput.stale(actual, "ack_lookup")
				}
				return nil
			})
			if lookupErr == nil && stage.input.operation.ID != "" {
				var operation CatalogSyncOperation
				lookupErr = DB.WithContext(lookup).First(&operation, "id = ?", stage.input.operation.ID).Error
				if lookupErr == nil && (operation.ID != stage.input.operation.ID || operation.CreatedAt != stage.input.operation.CreatedAt || operation.State != "succeeded" || operation.Revision != state.Revision || operation.PlanID != stage.input.operation.PlanID || operation.Backup != stage.input.operation.Backup || operation.Result != stage.input.operation.Result || operation.History != stage.input.operation.History) {
					lookupErr = ErrCatalogSyncPlanStale
				}
			}
			if lookupErr == nil {
				err = nil
			}
		}
	}
	if err != nil {
		recordCatalogSyncPhase(ctx, "ack", &stage.input, err)
		return err
	}
	stage.input.state.RuntimeRevision, stage.input.state.PublicationState, stage.input.state.PendingOperationID = stage.input.state.Revision, "ready", ""
	stage.input.state.RuntimeFingerprint, stage.input.state.CurrentDigest = stage.input.fingerprint, stage.input.digest
	stage.input.operation = CatalogSyncOperation{}
	catalogRuntime.db, catalogRuntime.revision, catalogRuntime.ready = DB, stage.input.state.Revision, true
	catalogRuntime.input = &stage.input
	catalogRuntimeInvalid.CompareAndSwap(DB, nil)
	recordCatalogSyncPhase(ctx, "ack", &stage.input, nil)
	return nil
}

// Copy immutable selection only. The callback must not execute expressions,
// plugins, network/body work, or reenter a catalog barrier. Real consumers are
// integrated in Task5b; existing frozen requests do not call this boundary.
func WithCatalogPricingRead(ctx context.Context, modelName string, capture func() error) error {
	return WithCatalogPricingReads(ctx, []string{modelName}, capture)
}

// WithCatalogPricingReads protects every possible pricing and money identity
// selected by one request. Its callback has the same short-capture contract.
func WithCatalogPricingReads(ctx context.Context, modelNames []string, capture func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	identities := make(map[string]bool, len(modelNames))
	for _, name := range modelNames {
		if strings.TrimSpace(name) == "" {
			return ErrCatalogPublicationPending
		}
		identities[name] = true
	}
	if !tryCatalogReadLock() {
		return ErrCatalogWriterBusy
	}
	defer catalogBarrier.RUnlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if capture == nil || len(identities) == 0 || !catalogRuntime.ready || catalogRuntime.db != DB || catalogRuntimeInvalid.Load() == DB {
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
		if state.RuntimeFingerprint == "" || catalogRuntime.input == nil || state.RuntimeFingerprint != catalogRuntime.input.fingerprint {
			return ErrCatalogPublicationPending
		}
		if state.PublicationState == "ready" && state.PendingOperationID == "" && state.Revision == state.RuntimeRevision && state.RuntimeRevision == catalogRuntime.revision {
			return capture()
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
				if identities[entry.Key] || (entry.Exists && (entry.Model == nil || entry.Model.ModelName != entry.Key || entry.Model.NameRule != NameRuleExact)) {
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
		return capture()
	}, options)
	if err != nil {
		return fmt.Errorf("catalog pricing selection: %w", err)
	}
	return nil
}
