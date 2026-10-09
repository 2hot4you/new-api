package catalogmanifest

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// BuildPlan performs a managed three-way diff without changing its inputs.
// Metadata in Change.Before/Base/After contains mutable fields only: source
// audit timestamps remain exclusively in Snapshot. Apply explicit mutable
// columns, preserve target audit lifecycle, and back up full authoritative rows
// separately. A Change.After is not a full wire entry for ValidateSnapshot.
// Baseline.Entries must instead retain full previously applied source entries.
func BuildPlan(source Snapshot, target Snapshot, base Baseline, actor Actor, now time.Time) (Plan, error) {
	if err := ValidateSnapshot(source); err != nil {
		return Plan{}, fmt.Errorf("source: %w", err)
	}
	if len(source.Entries) == 0 {
		return Plan{}, fmt.Errorf("empty source cannot establish managed changes")
	}
	if err := ValidateSnapshot(target); err != nil {
		return Plan{}, fmt.Errorf("target: %w", err)
	}
	if base.SourceID != "" && base.SourceID != source.SourceID {
		return Plan{}, fmt.Errorf("managed source identity changed")
	}
	if base.SourceID == "" && (len(base.Entries) != 0 || len(base.ObjectVersions) != 0 || base.Generation != 0) {
		return Plan{}, fmt.Errorf("baseline source identity is required")
	}
	if base.Generation < 0 {
		return Plan{}, fmt.Errorf("invalid baseline generation")
	}
	if base.SourceID != "" {
		baselineSnapshot := Snapshot{SchemaVersion: SchemaVersion, SourceID: base.SourceID, Complete: true, Capabilities: RequiredCapabilities(), Coverage: make(map[string]int), Entries: base.Entries}
		for _, kind := range Kinds() {
			baselineSnapshot.Coverage[kind] = 0
		}
		for _, entry := range base.Entries {
			baselineSnapshot.Coverage[entry.Kind]++
		}
		var err error
		baselineSnapshot.Digest, err = SnapshotDigest(baselineSnapshot)
		if err != nil {
			return Plan{}, fmt.Errorf("baseline: %w", err)
		}
		if err := ValidateSnapshot(baselineSnapshot); err != nil {
			return Plan{}, fmt.Errorf("baseline: %w", err)
		}
	}
	sourceEntries, err := mutableEntries(source.Entries)
	if err != nil {
		return Plan{}, err
	}
	targetEntries, err := mutableEntries(target.Entries)
	if err != nil {
		return Plan{}, err
	}
	baseEntries, err := mutableEntries(base.Entries)
	if err != nil {
		return Plan{}, fmt.Errorf("baseline: %w", err)
	}
	plan := Plan{Snapshot: cloneSnapshot(source), TargetDigest: target.Digest, BaselineGeneration: base.Generation, Actor: actor, ExpiresAt: now.Add(10 * time.Minute).Unix()}
	identities := make(map[string]bool)
	for _, entries := range []map[string]*Entry{sourceEntries, targetEntries, baseEntries} {
		for id := range entries {
			identities[id] = true
		}
	}
	for id := range identities {
		s, t, b := sourceEntries[id], targetEntries[id], baseEntries[id]
		identity := s
		if identity == nil {
			identity = t
		}
		if identity == nil {
			identity = b
		}
		change := Change{Kind: identity.Kind, Key: identity.Key, Before: t, Base: b, After: s}
		switch {
		case s == nil && b == nil:
			change.Action, change.Reason = "preserve", "target_only"
		case s == nil && t == nil:
			change.Action, change.Reason = "adopt", "already_absent"
		case b != nil && !equalEntry(t, b) && !equalEntry(t, s):
			change.Action, change.Reason = "conflict", "local_modified"
		case s == nil:
			change.Action, change.Reason = "delete", "source_removed"
		case equalEntry(t, s):
			change.Action, change.Reason = "adopt", "source_matches_target"
			if equalEntry(b, s) {
				change.Action, change.Reason = "unchanged", "managed_unchanged"
			}
		case t == nil:
			change.Action, change.Reason = "create", "source_added"
		case b == nil && base.SourceID != "":
			change.Action, change.Reason = "conflict", "local_modified"
		default:
			change.Action, change.Reason = "update", "source_updated"
		}
		if b != nil && t != nil && (identity.Kind == KindModel || identity.Kind == KindVendor) {
			if expected := base.ObjectVersions[id]; expected != "" {
				current := target.ObjectVersions[id]
				if current == "" {
					change.Action, change.Reason = "blocked", "missing_object_version"
				} else if current != expected {
					change.Action, change.Reason = "conflict", "object_recreated"
					if s == nil {
						change.Action, change.Reason = "blocked", "recreated_object_not_managed"
					}
				}
			}
		}
		if change.Action == "preserve" && (change.Kind == KindModelPrice || change.Kind == KindPluginPrice) {
			key, _ := DecodePriceKey(change.Key)
			if sourceModel := sourceEntries[EntryID(Entry{Kind: KindModel, Key: key.Model})]; sourceModel != nil {
				var metadata ModelValue
				var price PriceValue
				if err := common.UnmarshalJsonStr(sourceModel.Value, &metadata); err != nil {
					return Plan{}, err
				}
				if err := common.UnmarshalJsonStr(t.Value, &price); err != nil {
					return Plan{}, err
				}
				if metadata.BillingCurrency != price.BillingCurrency {
					change.Action, change.Reason = "blocked", "preserved_price_currency_mismatch"
				}
			}
		}
		plan.Changes = append(plan.Changes, change)
	}
	// A local conflict in one model field requires one explicit decision for
	// metadata, currency and every model/plugin price in that model's unit.
	conflicts := make(map[string]bool)
	for _, change := range plan.Changes {
		if change.Action == "conflict" || change.Action == "blocked" {
			conflicts[ConfirmationUnit(change)] = true
		}
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if conflicts[ConfirmationUnit(*change)] && change.Action != "preserve" && change.Action != "blocked" && change.Action != "conflict" {
			change.Action, change.Reason = "conflict", "confirmation_unit_conflict"
		}
	}
	sortChanges(plan.Changes)
	plan.Digest, err = CanonicalPlanDigest(plan)
	return plan, err
}

// ConfirmationUnit is the only accepted overwrite key for a change. A model
// and all its model/plugin price leaves share the canonical model EntryID.
// Vendor, special and tool leaves keep their independent EntryID identity.
func ConfirmationUnit(change Change) string {
	entry := Entry{Kind: change.Kind, Key: change.Key}
	if change.Kind == KindModelPrice || change.Kind == KindPluginPrice {
		if key, err := DecodePriceKey(change.Key); err == nil {
			return EntryID(Entry{Kind: KindModel, Key: key.Model})
		}
	}
	return EntryID(entry)
}

// ResolvePlan replaces confirmation choices without changing the pinned source
// or authoritative blocked outcomes. Unresolved conflicts remain visible and
// prevent execution. Overwrite choices identify entire ConfirmationUnit values,
// never individual fields/prices within a model. Expiry/auth checks belong to
// the caller (and PlanExecutable); resolving itself does not extend expiry.
func ResolvePlan(plan Plan, choices Resolution) (Plan, error) {
	digest, err := CanonicalPlanDigest(plan)
	if err != nil {
		return Plan{}, err
	}
	if plan.Digest == "" || plan.Digest != digest {
		return Plan{}, fmt.Errorf("plan content digest mismatch")
	}
	allowed := make(map[string]bool)
	for _, change := range plan.Changes {
		if change.Action != "blocked" && overwriteConflict(change) {
			allowed[ConfirmationUnit(change)] = true
		}
	}
	selected := make(map[string]bool)
	for _, key := range choices.OverwriteKeys {
		if !allowed[key] {
			return Plan{}, fmt.Errorf("unknown conflict confirmation unit")
		}
		if selected[key] {
			return Plan{}, fmt.Errorf("duplicate conflict confirmation unit")
		}
		selected[key] = true
	}
	plan.Snapshot = cloneSnapshot(plan.Snapshot)
	plan.Changes = slices.Clone(plan.Changes)
	for i := range plan.Changes {
		change := &plan.Changes[i]
		for _, pointer := range []**Entry{&change.Before, &change.Base, &change.After} {
			if *pointer != nil {
				copy := **pointer
				*pointer = &copy
			}
		}
		if change.Action == "blocked" || !overwriteConflict(*change) {
			continue
		}
		change.Action = "conflict"
		if !selected[ConfirmationUnit(*change)] {
			continue
		}
		switch {
		case change.After == nil && change.Before == nil:
			change.Action = "adopt"
		case change.After == nil:
			change.Action = "delete"
		case change.Before == nil:
			change.Action = "create"
		case equalEntry(change.Before, change.After):
			change.Action = "adopt"
		default:
			change.Action = "update"
		}
	}
	plan.Resolution = Resolution{ConfirmDeletes: choices.ConfirmDeletes}
	if len(choices.OverwriteKeys) != 0 {
		plan.Resolution.OverwriteKeys = slices.Clone(choices.OverwriteKeys)
		slices.Sort(plan.Resolution.OverwriteKeys)
	}
	sortChanges(plan.Changes)
	plan.Digest, err = CanonicalPlanDigest(plan)
	return plan, err
}

// PlanExecutable checks the sealed plan, ten-minute expiry, whole-unit conflict
// consent and independent delete consent. It does not authenticate an actor,
// authorize a request, check DB/reference versions or publish runtime pricing.
// Those checks remain mandatory under the later apply transaction/barrier.
func PlanExecutable(plan Plan, now time.Time) bool {
	if now.Unix() >= plan.ExpiresAt {
		return false
	}
	digest, err := CanonicalPlanDigest(plan)
	if err != nil || plan.Digest == "" || digest != plan.Digest {
		return false
	}
	hasChanges := false
	for _, change := range plan.Changes {
		if overwriteConflict(change) && !slices.Contains(plan.Resolution.OverwriteKeys, ConfirmationUnit(change)) {
			return false
		}
		switch change.Action {
		case "conflict", "blocked":
			return false
		case "delete":
			if !plan.Resolution.ConfirmDeletes {
				return false
			}
			hasChanges = true
		case "create", "update", "adopt":
			hasChanges = true
		case "preserve", "unchanged":
		default:
			return false
		}
	}
	return hasChanges
}

func overwriteConflict(change Change) bool {
	return change.Action == "conflict" || change.Reason == "local_modified" || change.Reason == "object_recreated" || change.Reason == "confirmation_unit_conflict"
}

// mutableEntries canonicalizes values and strips source-only audit provenance.
func mutableEntries(entries []Entry) (map[string]*Entry, error) {
	result := make(map[string]*Entry)
	for _, original := range entries {
		entry, err := canonicalEntry(original)
		if err != nil {
			return nil, err
		}
		if entry.Kind == KindModel || entry.Kind == KindVendor {
			var fields map[string]common.RawMessage
			if err := common.UnmarshalJsonStr(entry.Value, &fields); err != nil || fields == nil {
				return nil, fmt.Errorf("invalid metadata")
			}
			delete(fields, "created_time")
			delete(fields, "updated_time")
			encoded, err := common.Marshal(fields)
			if err != nil {
				return nil, err
			}
			entry.Value, err = CanonicalJSON(string(encoded))
			if err != nil {
				return nil, err
			}
		}
		id := EntryID(entry)
		if _, duplicate := result[id]; duplicate {
			return nil, fmt.Errorf("duplicate baseline identity")
		}
		result[id] = &entry
	}
	return result, nil
}

func canonicalEntry(entry Entry) (Entry, error) {
	if !slices.Contains(Kinds(), entry.Kind) {
		return Entry{}, fmt.Errorf("unknown entry kind")
	}
	var err error
	entry.Value, err = CanonicalJSON(entry.Value)
	if err != nil {
		return Entry{}, err
	}
	if entry.Kind != KindModel && entry.Kind != KindVendor {
		key, err := DecodePriceKey(entry.Key)
		if err != nil {
			return Entry{}, err
		}
		entry.Key, err = EncodePriceKey(key)
		if err != nil {
			return Entry{}, err
		}
		var value PriceValue
		if err := common.UnmarshalJsonStr(entry.Value, &value); err != nil {
			return Entry{}, err
		}
		value.Value, err = CanonicalJSON(value.Value)
		if err != nil {
			return Entry{}, err
		}
		encoded, err := common.Marshal(value)
		if err != nil {
			return Entry{}, err
		}
		entry.Value, err = CanonicalJSON(string(encoded))
		if err != nil {
			return Entry{}, err
		}
	}
	return entry, nil
}

func equalEntry(a, b *Entry) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot.Entries = slices.Clone(snapshot.Entries)
	snapshot.Coverage = maps.Clone(snapshot.Coverage)
	snapshot.Capabilities = maps.Clone(snapshot.Capabilities)
	snapshot.ObjectVersions = maps.Clone(snapshot.ObjectVersions)
	return snapshot
}

func sortChanges(changes []Change) {
	slices.SortFunc(changes, func(a, b Change) int {
		if compared := strings.Compare(a.Kind, b.Kind); compared != 0 {
			return compared
		}
		if compared := strings.Compare(a.Key, b.Key); compared != 0 {
			return compared
		}
		if compared := strings.Compare(a.Action, b.Action); compared != 0 {
			return compared
		}
		if compared := strings.Compare(a.Reason, b.Reason); compared != 0 {
			return compared
		}
		// Multiple independent blockers may share an identity and reason.
		left, _ := common.Marshal(a)
		right, _ := common.Marshal(b)
		return strings.Compare(string(left), string(right))
	})
}

// CanonicalPlanDigest binds the complete pinned source (including provenance
// and export time), target digest/incarnations, baseline generation, actor,
// expiry, server ID, changelog and confirmation choices. After adding blocked
// reference checks or a server ID, callers must recompute and persist it.
func CanonicalPlanDigest(plan Plan) (string, error) {
	plan.Digest = ""
	plan.Snapshot = cloneSnapshot(plan.Snapshot)
	for i := range plan.Snapshot.Entries {
		entry, err := canonicalEntry(plan.Snapshot.Entries[i])
		if err != nil {
			return "", err
		}
		plan.Snapshot.Entries[i] = entry
	}
	slices.SortFunc(plan.Snapshot.Entries, func(a, b Entry) int {
		if compared := strings.Compare(a.Kind, b.Kind); compared != 0 {
			return compared
		}
		return strings.Compare(a.Key, b.Key)
	})
	plan.Changes = slices.Clone(plan.Changes)
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if !slices.Contains(Kinds(), change.Kind) {
			return "", fmt.Errorf("unknown change kind")
		}
		if change.Kind != KindModel && change.Kind != KindVendor {
			key, keyErr := DecodePriceKey(change.Key)
			if keyErr != nil {
				return "", keyErr
			}
			change.Key, keyErr = EncodePriceKey(key)
			if keyErr != nil {
				return "", keyErr
			}
		}
		for _, pointer := range []**Entry{&change.Before, &change.Base, &change.After} {
			if *pointer == nil {
				continue
			}
			entry, err := canonicalEntry(**pointer)
			if err != nil {
				return "", err
			}
			*pointer = &entry
		}
	}
	sortChanges(plan.Changes)
	plan.Resolution.OverwriteKeys = slices.Clone(plan.Resolution.OverwriteKeys)
	slices.Sort(plan.Resolution.OverwriteKeys)
	encoded, err := common.Marshal(plan)
	if err != nil {
		return "", err
	}
	canonical, err := CanonicalJSON(string(encoded))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(canonical))), nil
}
