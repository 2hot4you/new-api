package catalogmanifest

import (
	"slices"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func diffJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

func TestManagedEmptyRestoredBaseline(t *testing.T) {
	source, target := diffSnapshot(t, diffModel(t, "source")), diffSnapshot(t)
	plan, err := BuildPlan(source, target, Baseline{Generation: 2}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	assert.Equal(t, int64(2), plan.BaselineGeneration)
	for _, base := range []Baseline{{Generation: -1}, {Generation: 2, Entries: source.Entries}, {Generation: 2, ObjectVersions: map[string]string{"model": "token"}}, {Generation: 2, SourceID: "another-source"}} {
		_, err := BuildPlan(source, target, base, Actor{}, time.Unix(1000, 0))
		require.Error(t, err)
	}
}

func TestManagedPlanOperationBinding(t *testing.T) {
	plan, err := BuildPlan(diffSnapshot(t, diffModel(t, "source")), diffSnapshot(t), Baseline{}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	assert.Equal(t, "sync", plan.Kind)
	for _, modify := range []func(*Plan){
		func(p *Plan) { p.Kind = "restore"; p.RestoreOperationID = "original-operation" },
		func(p *Plan) { p.ValidationDigest = "validation-commitment" },
		func(p *Plan) { p.ReferenceDigest = "reference-commitment" },
	} {
		changed := plan
		modify(&changed)
		digest, err := CanonicalPlanDigest(changed)
		require.NoError(t, err)
		assert.NotEqual(t, plan.Digest, digest)
		assert.False(t, PlanExecutable(changed, time.Unix(1001, 0)))
	}
	for _, binding := range [][2]string{{"", ""}, {"unknown", ""}, {"sync", "original-operation"}, {"restore", ""}, {"restore", " "}} {
		changed := plan
		changed.Kind, changed.RestoreOperationID = binding[0], binding[1]
		_, err := CanonicalPlanDigest(changed)
		require.Error(t, err, "invalid operation binding must not be sealable")
	}
}

func TestManagedStructuralValidationDoesNotEvaluate(t *testing.T) {
	for _, expression := range []string{`"p *"`, `"-1"`, `"fixed(1)"`} {
		price := diffPrice(t, "billing_setting.plugin_billing_expr", "model", "plugin", "/plugin::model", expression)
		snapshot := diffSnapshot(t, diffModel(t, "source"), price)
		require.NoError(t, ValidateSnapshotStructure(snapshot))
		require.Error(t, ValidateSnapshot(snapshot))
	}
	zero := diffSnapshot(t, diffModel(t, "source"), diffPrice(t, "ModelRatio", "model", "", "/model", "0"))
	require.NoError(t, ValidateSnapshotStructure(zero))
	for _, modify := range []func(*Snapshot){
		func(s *Snapshot) { s.SchemaVersion++ },
		func(s *Snapshot) { s.Coverage[KindModel]++ },
		func(s *Snapshot) {
			s.Entries[1].Value = diffJSON(t, PriceValue{Value: "0", BillingCurrency: "CNY", Unit: "legacy_ratio"})
		},
	} {
		changed := cloneSnapshot(zero)
		modify(&changed)
		diffSeal(t, &changed)
		require.Error(t, ValidateSnapshotStructure(changed))
	}
}

// A recreated or unverifiable model must never silently adopt existing prices.
func TestManagedIncarnationAndConfirmationUnit(t *testing.T) {
	model := diffModel(t, "old")
	price := diffPrice(t, "ModelRatio", "model", "", "/model", "1")
	plugin := diffPrice(t, "billing_setting.plugin_billing_expr", "model", "plugin", "/plugin::model", `"v1: p * 2"`)
	entries := []Entry{model, price, plugin}
	for _, test := range []struct{ name, token, wantModel string }{
		{"recreated", "new-incarnation", "conflict"},
		{"expected token missing", "", "blocked"},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := diffSnapshot(t, entries...)
			target.ObjectVersions = map[string]string{EntryID(model): test.token}
			diffSeal(t, &target)
			base := Baseline{SourceID: "dev", Entries: entries, ObjectVersions: map[string]string{EntryID(model): "original"}}
			plan, err := BuildPlan(diffSnapshot(t, entries...), target, base, Actor{}, time.Unix(1000, 0))
			require.NoError(t, err)
			require.Len(t, plan.Changes, 3)
			for _, change := range plan.Changes {
				assert.Equal(t, EntryID(model), ConfirmationUnit(change))
				want := "conflict"
				if change.Kind == KindModel {
					want = test.wantModel
				}
				assert.Equal(t, want, change.Action)
			}
			resolved, err := ResolvePlan(plan, Resolution{OverwriteKeys: []string{EntryID(model)}})
			require.NoError(t, err)
			assert.Equal(t, test.token != "", PlanExecutable(resolved, time.Unix(1001, 0)))
		})
	}
}

// A currency/metadata local conflict cannot be acknowledged for only a price.
func TestResolveManagedPlan(t *testing.T) {
	old := diffModel(t, "old")
	local := diffModel(t, "local")
	price := diffPrice(t, "ModelRatio", "model", "", "/model", "1")
	plugin := diffPrice(t, "billing_setting.plugin_billing_expr", "model", "plugin", "/plugin::model", `"v1: p * 2"`)
	source := diffSnapshot(t, old, price, plugin)
	plan, err := BuildPlan(source, diffSnapshot(t, local, price, plugin), Baseline{SourceID: "dev", Entries: []Entry{old, price, plugin}}, Actor{UserID: 1}, time.Unix(1000, 0))
	require.NoError(t, err)
	for _, change := range plan.Changes {
		assert.Equal(t, "conflict", change.Action)
	}
	assert.False(t, PlanExecutable(plan, time.Unix(1001, 0)))
	for _, keys := range [][]string{{"unknown"}, {EntryID(price)}, {EntryID(old), EntryID(old)}} {
		_, err := ResolvePlan(plan, Resolution{OverwriteKeys: keys})
		require.Error(t, err)
	}
	resolved, err := ResolvePlan(plan, Resolution{OverwriteKeys: []string{EntryID(old)}})
	require.NoError(t, err)
	assert.Equal(t, source, resolved.Snapshot)
	assert.NotEqual(t, plan.Digest, resolved.Digest)
	assert.True(t, PlanExecutable(resolved, time.Unix(1001, 0)))
	assert.False(t, PlanExecutable(resolved, time.Unix(1600, 0)))
	for _, change := range resolved.Changes {
		want := "adopt"
		if change.Kind == KindModel {
			want = "update"
		}
		assert.Equal(t, want, change.Action)
	}
	reset, err := ResolvePlan(resolved, Resolution{})
	require.NoError(t, err)
	assert.Equal(t, plan.Digest, reset.Digest)
	for _, change := range reset.Changes {
		assert.Equal(t, "conflict", change.Action)
	}
	assert.Equal(t, "conflict", plan.Changes[0].Action)
	// Pinned source and the input plan must not alias resolution output.
	resolved.Snapshot.Capabilities["catalog_scope"] = "mutated"
	resolved.Changes[0].After.Value = "mutated"
	assert.Equal(t, source, plan.Snapshot)
	assert.NotEqual(t, "mutated", plan.Changes[0].After.Value)
}

func TestManagedDeletionConsentAndBlockedReferences(t *testing.T) {
	model := diffModel(t, "old")
	leaf := diffPrice(t, "tool_price_setting.prices", "", "", "/managed", "1")
	plan, err := BuildPlan(diffSnapshot(t, model), diffSnapshot(t, model, leaf), Baseline{SourceID: "dev", Entries: []Entry{model, leaf}}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	assert.False(t, PlanExecutable(plan, time.Unix(1001, 0)))
	resolved, err := ResolvePlan(plan, Resolution{ConfirmDeletes: true})
	require.NoError(t, err)
	assert.True(t, PlanExecutable(resolved, time.Unix(1001, 0)))
	assert.NotEqual(t, plan.Digest, resolved.Digest)
	// Later reference checks append a separate, authoritative blocking outcome.
	plan.Changes = append(plan.Changes, Change{Kind: leaf.Kind, Key: leaf.Key, Action: "blocked", Reason: "unsafe_price_fallback", Before: &leaf})
	plan.Digest, err = CanonicalPlanDigest(plan)
	require.NoError(t, err)
	resolved, err = ResolvePlan(plan, Resolution{ConfirmDeletes: true})
	require.NoError(t, err)
	assert.Contains(t, resolved.Changes, plan.Changes[len(plan.Changes)-1])
	assert.False(t, PlanExecutable(resolved, time.Unix(1001, 0)))
	_, err = ResolvePlan(plan, Resolution{OverwriteKeys: []string{EntryID(leaf)}, ConfirmDeletes: true})
	require.Error(t, err)
}

// Timestamp noise must not create conflicts; publication dates remain business data.
func TestManagedMetadataProvenance(t *testing.T) {
	for _, kind := range []string{KindModel, KindVendor} {
		t.Run(kind, func(t *testing.T) {
			var source, target Entry
			if kind == KindModel {
				source = diffModel(t, "old")
				var value ModelValue
				require.NoError(t, common.UnmarshalJsonStr(source.Value, &value))
				value.CreatedTime, value.UpdatedTime = 1, 2
				source.Value = diffJSON(t, value)
				value.CreatedTime, value.UpdatedTime = 3, 4
				target = Entry{Kind: kind, Key: source.Key, Value: diffJSON(t, value)}
			} else {
				source = Entry{Kind: kind, Key: "vendor", Value: diffJSON(t, VendorValue{Name: "vendor", CreatedTime: 1, UpdatedTime: 2})}
				target = Entry{Kind: kind, Key: "vendor", Value: diffJSON(t, VendorValue{Name: "vendor", CreatedTime: 3, UpdatedTime: 4})}
			}
			snapshot := diffSnapshot(t, source)
			plan, err := BuildPlan(snapshot, diffSnapshot(t, target), Baseline{SourceID: "dev", Entries: []Entry{source}}, Actor{}, time.Unix(1000, 0))
			require.NoError(t, err)
			assert.Equal(t, "unchanged", plan.Changes[0].Action)
			assert.False(t, PlanExecutable(plan, time.Unix(1001, 0)))
			assert.Equal(t, snapshot, plan.Snapshot)
			for _, entry := range []*Entry{plan.Changes[0].Before, plan.Changes[0].Base, plan.Changes[0].After} {
				assert.NotContains(t, entry.Value, "created_time")
				assert.NotContains(t, entry.Value, "updated_time")
			}
		})
	}
	source := diffModel(t, "old")
	var value ModelValue
	require.NoError(t, common.UnmarshalJsonStr(source.Value, &value))
	value.ReleaseDate = "2026-10-09"
	source.Value = diffJSON(t, value)
	plan, err := BuildPlan(diffSnapshot(t, source), diffSnapshot(t, diffModel(t, "old")), Baseline{}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	assert.Equal(t, "update", plan.Changes[0].Action)
	assert.Contains(t, plan.Changes[0].After.Value, "2026-10-09")
}

// Digest equality must survive ordering but detect changed actor/state/consent.
func TestManagedPlanDigestAndTampering(t *testing.T) {
	model := diffModel(t, "old")
	leaf := diffPrice(t, "tool_price_setting.prices", "", "", "/managed", "1")
	actor := Actor{UserID: 1, SessionID: "session", TargetID: "target", AuthVersion: 2, SessionVersion: 3}
	plan, err := BuildPlan(diffSnapshot(t, model, leaf), diffSnapshot(t), Baseline{}, actor, time.Unix(1000, 0))
	require.NoError(t, err)
	for _, test := range []struct {
		name   string
		mutate func(*Plan)
	}{
		{"user", func(p *Plan) { p.Actor.UserID++ }},
		{"session", func(p *Plan) { p.Actor.SessionID += "other" }},
		{"target", func(p *Plan) { p.Actor.TargetID += "other" }},
		{"auth version", func(p *Plan) { p.Actor.AuthVersion++ }},
		{"session version", func(p *Plan) { p.Actor.SessionVersion++ }},
		{"expiry", func(p *Plan) { p.ExpiresAt++ }},
		{"baseline generation", func(p *Plan) { p.BaselineGeneration++ }},
		{"target state", func(p *Plan) { p.TargetDigest += "other" }},
		{"source timestamp", func(p *Plan) { p.Snapshot.ExportedAt++ }},
		{"source identity", func(p *Plan) { p.Snapshot.SourceID += "other" }},
		{"delete consent", func(p *Plan) { p.Resolution.ConfirmDeletes = true }},
		{"source incarnation", func(p *Plan) { p.Snapshot.ObjectVersions = map[string]string{EntryID(model): "new"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			modified := plan
			test.mutate(&modified)
			digest, err := CanonicalPlanDigest(modified)
			require.NoError(t, err)
			assert.NotEqual(t, plan.Digest, digest)
			assert.False(t, PlanExecutable(modified, time.Unix(1001, 0)))
			_, err = ResolvePlan(modified, Resolution{})
			require.Error(t, err)
		})
	}
	reordered := plan
	reordered.Changes = slices.Clone(plan.Changes)
	slices.Reverse(reordered.Changes)
	reordered.Snapshot.Entries = slices.Clone(plan.Snapshot.Entries)
	slices.Reverse(reordered.Snapshot.Entries)
	digest, err := CanonicalPlanDigest(reordered)
	require.NoError(t, err)
	assert.Equal(t, plan.Digest, digest)
	// Target incarnations are transitively pinned by the validated target digest.
	target := diffSnapshot(t, model)
	target.ObjectVersions = map[string]string{EntryID(model): "one"}
	diffSeal(t, &target)
	first, err := BuildPlan(diffSnapshot(t, model), target, Baseline{}, actor, time.Unix(1000, 0))
	require.NoError(t, err)
	target.ObjectVersions[EntryID(model)] = "two"
	diffSeal(t, &target)
	second, err := BuildPlan(diffSnapshot(t, model), target, Baseline{}, actor, time.Unix(1000, 0))
	require.NoError(t, err)
	assert.NotEqual(t, first.Digest, second.Digest)
}

// Deleting by an old name must not grant ownership of a replacement object.
func TestManagedRecreatedRemovalBlocked(t *testing.T) {
	old := diffModel(t, "old")
	other := Entry{Kind: KindModel, Key: "other", Value: diffJSON(t, ModelValue{ModelName: "other", BillingCurrency: "USD"})}
	target := diffSnapshot(t, old, other)
	target.ObjectVersions = map[string]string{EntryID(old): "recreated"}
	diffSeal(t, &target)
	plan, err := BuildPlan(diffSnapshot(t, other), target, Baseline{SourceID: "dev", Entries: []Entry{old, other}, ObjectVersions: map[string]string{EntryID(old): "original"}}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	var removal Change
	for _, change := range plan.Changes {
		if change.Key == old.Key {
			removal = change
		}
	}
	assert.Equal(t, "blocked", removal.Action)
	_, err = ResolvePlan(plan, Resolution{OverwriteKeys: []string{EntryID(old)}, ConfirmDeletes: true})
	require.Error(t, err)
	resolved, err := ResolvePlan(plan, Resolution{ConfirmDeletes: true})
	require.NoError(t, err)
	assert.False(t, PlanExecutable(resolved, time.Unix(1001, 0)))
}

// Preserving a target-only coefficient never permits silently changing its currency.
func TestManagedPreservedLocalPriceCurrencyBlocked(t *testing.T) {
	model := diffModel(t, "old")
	local := diffPrice(t, "ModelRatio", "model", "", "/model", "1")
	plugin := diffPrice(t, "billing_setting.plugin_billing_expr", "model", "plugin", "/plugin::model", `"v1: p * 2"`)
	var value ModelValue
	require.NoError(t, common.UnmarshalJsonStr(model.Value, &value))
	value.BillingCurrency = "CNY"
	sourceModel := Entry{Kind: KindModel, Key: model.Key, Value: diffJSON(t, value)}
	for _, price := range []Entry{local, plugin} {
		plan, err := BuildPlan(diffSnapshot(t, sourceModel), diffSnapshot(t, model, price), Baseline{SourceID: "dev", Entries: []Entry{model}}, Actor{}, time.Unix(1000, 0))
		require.NoError(t, err)
		var preserved Change
		for _, change := range plan.Changes {
			if change.Kind == price.Kind {
				preserved = change
			}
		}
		assert.Equal(t, "blocked", preserved.Action)
		assert.JSONEq(t, price.Value, preserved.Before.Value)
		resolved, err := ResolvePlan(plan, Resolution{OverwriteKeys: []string{EntryID(model)}})
		require.NoError(t, err)
		assert.False(t, PlanExecutable(resolved, time.Unix(1001, 0)))
	}
}

func TestManagedCorruptBaselineRejected(t *testing.T) {
	model := diffModel(t, "old")
	for _, entries := range [][]Entry{
		{{Kind: KindModel, Key: "model", Value: "{}"}},
		{model, model},
		{{Kind: KindToolPrice, Key: `{"option":"unsupported","path":"/x"}`, Value: diffJSON(t, PriceValue{Value: "1", BillingCurrency: "USD", Unit: "thousand_calls"})}},
	} {
		plan, err := BuildPlan(diffSnapshot(t, model), diffSnapshot(t, model), Baseline{SourceID: "dev", Entries: entries}, Actor{}, time.Unix(1000, 0))
		require.Error(t, err)
		assert.Empty(t, plan.Changes)
	}
}

func TestManagedCanonicalIdentitiesAndValues(t *testing.T) {
	model := diffModel(t, "old")
	price := diffPrice(t, "ModelRatio", "model", "", "/model", "1")
	sourcePrice := price
	sourcePrice.Key = `{"plugin":"","path":"/model","option":"ModelRatio","model":"model"}`
	sourcePrice.Value = diffJSON(t, PriceValue{Value: "1.0", BillingCurrency: "USD", Unit: "legacy_ratio"})
	plan, err := BuildPlan(diffSnapshot(t, sourcePrice, model), diffSnapshot(t, model, price), Baseline{SourceID: "dev", Entries: []Entry{model, price}}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	for _, change := range plan.Changes {
		assert.Equal(t, "unchanged", change.Action)
	}
	canonical, err := BuildPlan(diffSnapshot(t, model, price), diffSnapshot(t, model, price), Baseline{SourceID: "dev", Entries: []Entry{price, model}}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	assert.Equal(t, canonical.Digest, plan.Digest)
	assert.Equal(t, sourcePrice, plan.Snapshot.Entries[0])
}

// Independent reference checks can contribute several blockers to one identity.
func TestManagedBlockedDigestOrderingAndResolution(t *testing.T) {
	old := diffModel(t, "old")
	plan, err := BuildPlan(diffSnapshot(t, old), diffSnapshot(t, diffModel(t, "local")), Baseline{SourceID: "dev", Entries: []Entry{old}}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	plan.Changes = append(plan.Changes,
		Change{Kind: KindModel, Key: "model", Action: "blocked", Reason: "active_task"},
		Change{Kind: KindModel, Key: "model", Action: "blocked", Reason: "active_channel"},
	)
	plan.Digest, err = CanonicalPlanDigest(plan)
	require.NoError(t, err)
	reordered := plan
	reordered.Changes = slices.Clone(plan.Changes)
	slices.Reverse(reordered.Changes)
	digest, err := CanonicalPlanDigest(reordered)
	require.NoError(t, err)
	assert.Equal(t, plan.Digest, digest)
	resolved, err := ResolvePlan(plan, Resolution{OverwriteKeys: []string{EntryID(old)}, ConfirmDeletes: true})
	require.NoError(t, err)
	assert.False(t, PlanExecutable(resolved, time.Unix(1001, 0)))
	for _, reason := range []string{"active_task", "active_channel"} {
		assert.True(t, slices.ContainsFunc(resolved.Changes, func(change Change) bool { return change.Action == "blocked" && change.Reason == reason }))
	}
}

func TestManagedCurrencyAndExpressionsGrouped(t *testing.T) {
	old := diffModel(t, "old")
	local := diffModel(t, "local")
	var metadata ModelValue
	require.NoError(t, common.UnmarshalJsonStr(old.Value, &metadata))
	metadata.BillingCurrency = "CNY"
	sourceModel := Entry{Kind: KindModel, Key: "model", Value: diffJSON(t, metadata)}
	expression := diffPrice(t, "billing_setting.billing_expr", "model", "", "/model", `"v1: p * 1"`)
	plugin := diffPrice(t, "billing_setting.plugin_billing_expr", "model", "plugin", "/plugin::model", `"v1: p * 1"`)
	sourceEntries := []Entry{sourceModel}
	for _, entry := range []Entry{expression, plugin} {
		key, err := DecodePriceKey(entry.Key)
		require.NoError(t, err)
		entry.Value = diffJSON(t, PriceValue{Value: `"v1: p * 2"`, BillingCurrency: "CNY", Unit: PriceOptions()[key.Option].Unit})
		sourceEntries = append(sourceEntries, entry)
	}
	plan, err := BuildPlan(diffSnapshot(t, sourceEntries...), diffSnapshot(t, local, expression, plugin), Baseline{SourceID: "dev", Entries: []Entry{old, expression, plugin}}, Actor{}, time.Unix(1000, 0))
	require.NoError(t, err)
	for _, change := range plan.Changes {
		assert.Equal(t, "conflict", change.Action)
	}
	resolved, err := ResolvePlan(plan, Resolution{OverwriteKeys: []string{EntryID(old)}})
	require.NoError(t, err)
	assert.True(t, PlanExecutable(resolved, time.Unix(1001, 0)))
	for _, change := range resolved.Changes {
		assert.Equal(t, "update", change.Action)
		if change.Kind == KindModel {
			continue
		}
		var price PriceValue
		require.NoError(t, common.UnmarshalJsonStr(change.After.Value, &price))
		assert.Equal(t, "CNY", price.BillingCurrency)
		assert.Equal(t, `"v1: p * 2"`, price.Value)
	}
}

func diffModel(t *testing.T, description string) Entry {
	t.Helper()
	return Entry{Kind: KindModel, Key: "model", Value: diffJSON(t, ModelValue{ModelName: "model", Description: description, BillingCurrency: "USD"})}
}

func diffPrice(t *testing.T, option, model, plugin, path, value string) Entry {
	t.Helper()
	key, err := EncodePriceKey(PriceKey{Option: option, Model: model, Plugin: plugin, Path: path})
	require.NoError(t, err)
	spec := PriceOptions()[option]
	currency := spec.Currency
	if model != "" {
		currency = "USD"
	}
	return Entry{Kind: spec.Kind, Key: key, Value: diffJSON(t, PriceValue{Value: value, BillingCurrency: currency, Unit: spec.Unit})}
}

func diffSnapshot(t *testing.T, entries ...Entry) Snapshot {
	t.Helper()
	coverage := make(map[string]int)
	for _, kind := range Kinds() {
		coverage[kind] = 0
	}
	for _, entry := range entries {
		coverage[entry.Kind]++
	}
	snapshot := Snapshot{SchemaVersion: SchemaVersion, SourceID: "dev", ExportedAt: 123, Complete: true, Capabilities: RequiredCapabilities(), Coverage: coverage, Entries: entries}
	diffSeal(t, &snapshot)
	return snapshot
}

func diffSeal(t *testing.T, snapshot *Snapshot) {
	t.Helper()
	var err error
	snapshot.Digest, err = SnapshotDigest(*snapshot)
	require.NoError(t, err)
}

// These cases catch accidental mirror pruning, missing local conflict detection,
// and conflation of independent nested option leaves.
func TestManagedThreeWayDiff(t *testing.T) {
	old := diffModel(t, "old")
	local := diffModel(t, "local")
	updated := diffModel(t, "new")
	keep := diffPrice(t, "tool_price_setting.prices", "", "", "/local", "2")
	managed := diffPrice(t, "tool_price_setting.prices", "", "", "/managed", "1")
	for _, test := range []struct {
		name           string
		source, target []Entry
		base           []Entry
		want           map[string]string
	}{
		{"first equal adopts", []Entry{old}, []Entry{old}, nil, map[string]string{EntryID(old): "adopt"}},
		{"first differing previews update", []Entry{updated}, []Entry{old}, nil, map[string]string{EntryID(old): "update"}},
		{"empty complete target creates", []Entry{old}, nil, nil, map[string]string{EntryID(old): "create"}},
		{"target only preserved", []Entry{old}, []Entry{old, keep}, nil, map[string]string{EntryID(old): "adopt", EntryID(keep): "preserve"}},
		{"source change replaces baseline", []Entry{updated}, []Entry{old}, []Entry{old}, map[string]string{EntryID(old): "update"}},
		{"local change conflicts even unchanged source", []Entry{old}, []Entry{local}, []Entry{old}, map[string]string{EntryID(old): "conflict"}},
		{"local deletion conflicts", []Entry{old}, nil, []Entry{old}, map[string]string{EntryID(old): "conflict"}},
		{"already source updates ownership", []Entry{updated}, []Entry{updated}, []Entry{old}, map[string]string{EntryID(old): "adopt"}},
		{"same managed unchanged", []Entry{old}, []Entry{old}, []Entry{old}, map[string]string{EntryID(old): "unchanged"}},
		{"only managed leaf removed", []Entry{old}, []Entry{old, managed, keep}, []Entry{old, managed}, map[string]string{EntryID(old): "unchanged", EntryID(managed): "delete", EntryID(keep): "preserve"}},
		{"modified managed removal conflicts", []Entry{old}, []Entry{old, keep}, []Entry{old, Entry{Kind: keep.Kind, Key: keep.Key, Value: managed.Value}}, map[string]string{EntryID(old): "unchanged", EntryID(keep): "conflict"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := Baseline{Entries: test.base}
			if len(test.base) > 0 {
				base.SourceID = "dev"
				base.Generation = 7
			}
			plan, err := BuildPlan(diffSnapshot(t, test.source...), diffSnapshot(t, test.target...), base, Actor{UserID: 1}, time.Unix(1000, 0))
			require.NoError(t, err)
			got := map[string]string{}
			for _, change := range plan.Changes {
				got[EntryID(Entry{Kind: change.Kind, Key: change.Key})] = change.Action
			}
			assert.Equal(t, test.want, got)
			assert.Equal(t, int64(1600), plan.ExpiresAt)
		})
	}
}

func TestManagedSourceSafety(t *testing.T) {
	model := diffModel(t, "old")
	for _, test := range []struct {
		name   string
		mutate func(*Snapshot, *Baseline)
	}{
		{"empty source", func(source *Snapshot, base *Baseline) { *source = diffSnapshot(t) }},
		{"partial source", func(source *Snapshot, base *Baseline) { source.Complete = false; diffSeal(t, source) }},
		{"source replaced", func(source *Snapshot, base *Baseline) { source.SourceID = "other"; diffSeal(t, source) }},
		{"coverage missing", func(source *Snapshot, base *Baseline) { delete(source.Coverage, KindVendor); diffSeal(t, source) }},
		{"baseline identity missing", func(source *Snapshot, base *Baseline) { base.SourceID = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := diffSnapshot(t, model)
			base := Baseline{SourceID: "dev", Entries: []Entry{model}}
			test.mutate(&source, &base)
			plan, err := BuildPlan(source, diffSnapshot(t, model), base, Actor{}, time.Unix(1000, 0))
			require.Error(t, err)
			assert.Empty(t, plan.Changes)
		})
	}
}
