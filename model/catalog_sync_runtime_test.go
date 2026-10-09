package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func runtimePendingOperation(t *testing.T, db *gorm.DB) catalogmanifest.Result {
	t.Helper()
	actor := catalogBusinessActor(t, db)
	plan, err := CreateCatalogSyncPlan(context.Background(), catalogBusinessPriceSource(t, db), actor, time.Now())
	require.NoError(t, err)
	updatePricingLock.Lock()
	result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "runtime-pending", actor)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy, "actual apply must commit then retain pending on required rebuild contention")
	return result
}

func runtimeStageForTest(t *testing.T, db *gorm.DB) *catalogRuntimeStage {
	t.Helper()
	var input catalogRuntimeInput
	require.NoError(t, TryWithCatalogWriteBarrier(context.Background(), func() error {
		return catalogReferenceTransaction(context.Background(), db, jsplugin.DefaultRegistry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			var err error
			input, err = captureCatalogRuntimeTx(tx, state, pin)
			return err
		})
	}))
	stage, err := stageCatalogRuntime(input)
	require.NoError(t, err)
	return stage
}

// Register before typed/registry fixture cleanup so restored prices are in
// place before restoring the old root's readiness and derived cache views.
func preserveOrdinaryCatalogRuntime(t *testing.T) {
	t.Helper()
	catalogBarrier.Lock()
	priorRuntime := catalogRuntime
	updatePricingLock.Lock()
	priorPricing, priorVendors, priorTime := pricingMap, vendorsList, lastGetPricingTime
	modelSupportEndpointsLock.Lock()
	priorEndpoints, priorTypes := supportedEndpointMap, modelSupportEndpointTypes
	modelEnableGroupsLock.Lock()
	priorGroups, priorQuotas := modelEnableGroups, modelQuotaTypeMap
	modelEnableGroupsLock.Unlock()
	modelSupportEndpointsLock.Unlock()
	updatePricingLock.Unlock()
	priorName, priorFX := common.SystemName, operation_setting.USDExchangeRate
	catalogBarrier.Unlock()
	t.Cleanup(func() {
		catalogBarrier.Lock()
		defer catalogBarrier.Unlock()
		catalogRuntime = priorRuntime
		common.SystemName, operation_setting.USDExchangeRate = priorName, priorFX
		updatePricingLock.Lock()
		defer updatePricingLock.Unlock()
		modelSupportEndpointsLock.Lock()
		defer modelSupportEndpointsLock.Unlock()
		modelEnableGroupsLock.Lock()
		defer modelEnableGroupsLock.Unlock()
		pricingMap, vendorsList, lastGetPricingTime = priorPricing, priorVendors, priorTime
		supportedEndpointMap, modelSupportEndpointTypes = priorEndpoints, priorTypes
		modelEnableGroups, modelQuotaTypeMap = priorGroups, priorQuotas
		ratio_setting.InvalidateExposedDataCache()
	})
}

func initializeOrdinaryCatalogTest(t *testing.T, db *gorm.DB) {
	t.Helper()
	preserveOrdinaryCatalogRuntime(t)
	preserveCatalogCandidate(t)
	priorRegistry := jsplugin.DefaultRegistry
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	t.Cleanup(func() { jsplugin.DefaultRegistry = priorRegistry })
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &Task{}, &TaskPlugin{}, &Midjourney{}, &SystemTask{}))
}

func TestCatalogRuntimeOrdinaryWriters(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"ordinary":0}`, "billing_setting.billing_mode": `{}`, "SystemName": "ordinary-test"}))
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			assert.Equal(t, int64(1), state.Revision)
			assert.Equal(t, state.Revision, state.RuntimeRevision, "ordinary save must acknowledge its actual runtime")
			assert.Equal(t, "ready", state.PublicationState)
			require.NoError(t, WithCatalogPricingRead(context.Background(), "ordinary", func() error {
				price, ok := ratio_setting.GetModelPrice("ordinary", false)
				assert.True(t, ok)
				assert.Zero(t, price)
				return nil
			}))
			vendor := Vendor{Name: "ordinary-vendor", Status: 1}
			require.NoError(t, vendor.Insert())
			entry := Model{ModelName: "ordinary", VendorID: vendor.Id, BillingCurrency: "USD", Status: 1}
			require.NoError(t, entry.Insert())
			entry.BillingCurrency = "CNY"
			require.NoError(t, entry.Update())
			require.NoError(t, WithCatalogPricingRead(context.Background(), "ordinary", func() error {
				currencies, err := LoadModelBillingCurrencies(db, []string{"ordinary"}, false)
				require.NoError(t, err)
				assert.Equal(t, "CNY", string(currencies["ordinary"].BillingCurrency))
				price, ok := ratio_setting.GetModelPrice("ordinary", false)
				assert.True(t, ok)
				assert.Zero(t, price)
				return nil
			}))
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			assert.Equal(t, int64(4), state.Revision)
			assert.Equal(t, state.Revision, state.RuntimeRevision)
			require.NoError(t, ReorderModels([]int{entry.Id}))
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"ordinary":0}`}))
			var noOp CatalogSyncState
			require.NoError(t, db.First(&noOp, CatalogSyncStateID).Error)
			assert.Equal(t, state, noOp, "no-op writers must not stale a prepared plan")
			var operations int64
			require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&operations).Error)
			assert.Zero(t, operations, "ordinary writes have no managed operation")
		})
	}
}

func TestCatalogRuntimeOrdinaryOptions(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			actor := catalogBusinessActor(t, db)
			oldName, oldFX := common.SystemName, operation_setting.USDExchangeRate
			t.Cleanup(func() { common.SystemName, operation_setting.USDExchangeRate = oldName, oldFX })
			values := map[string]string{
				"ModelPrice":                             `{"historical-orphan":0,"removed":9}`,
				"billing_setting.billing_mode":           `{}`,
				"tool_price_setting.prices":              `{"web_search":12,"web_search:custom*":0}`,
				"molii_grok_tool_price.image_generation": "0.17",
				"molii_grok_price.image_standard_1k":     "0",
				"starai_video_price.standard_720p":       "13.25",
				"task_pricing_setting.sora_size_ratio":   `{"1792x1024":3}`,
				"USDExchangeRate":                        "7.25", "SystemName": "ordinary-local",
			}
			require.NoError(t, UpdateOptionsBulk(values))
			assert.Equal(t, "ordinary-local", common.SystemName)
			assert.Equal(t, 7.25, operation_setting.USDExchangeRate)
			for key, value := range values {
				var row Option
				require.NoError(t, db.First(&row, commonKeyCol+" = ?", key).Error)
				assert.Equal(t, value, row.Value)
			}
			var expressionRows int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "billing_setting.billing_expr").Count(&expressionRows).Error)
			assert.Zero(t, expressionRows, "generic Seedance menu fields do not synthesize stored expressions")
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{}`}))
			_, removed := ratio_setting.GetModelPrice("removed", false)
			assert.False(t, removed)
			require.NoError(t, UpdateOption("molii_grok_tool_price.image_generation", "0.19"))
			before := runtimeStageForTest(t, db).input
			for _, invalid := range []map[string]string{
				{"ModelPrice": `{"bad":-1}`, "SystemName": "must-not-persist"},
				{"molii_grok_price.not_a_price": "1"},
				{"tool_price_setting.prices": `{"web_search":-1}`},
			} {
				require.Error(t, UpdateOptionsBulk(invalid))
				assert.True(t, before.same(runtimeStageForTest(t, db).input))
			}
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{}`}))
			_, err = GetCatalogSyncPlan(context.Background(), plan.ID, actor)
			require.NoError(t, err, "ordinary no-op preserves actual plan freshness")
		})
	}
}

func TestCatalogRuntimeOrdinaryRawNoop(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			actor := catalogBusinessActor(t, db)
			const key = "tool_price_setting.prices"
			require.NoError(t, UpdateOptionsBulk(map[string]string{key: `{"web_search":12,"web_search:custom*":0}`}))
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			for _, change := range []struct {
				name, raw string
				bulk      bool
			}{
				{"formatting", `{ "web_search" : 12, "web_search:custom*" : 0 }`, true},
				{"ordering", `{"web_search:custom*":0,"web_search":12}`, false},
			} {
				t.Run(change.name, func(t *testing.T) {
					if change.bulk {
						require.NoError(t, UpdateOptionsBulk(map[string]string{key: change.raw}))
					} else {
						require.NoError(t, UpdateOption(key, change.raw))
					}
					var persisted Option
					require.NoError(t, db.First(&persisted, commonKeyCol+" = ?", key).Error)
					assert.Equal(t, change.raw, persisted.Value)
					common.OptionMapRWMutex.RLock()
					published := common.OptionMap[key]
					common.OptionMapRWMutex.RUnlock()
					assert.Equal(t, change.raw, published, "options reads must return the exact accepted raw value")
					require.NoError(t, WithCatalogPricingRead(context.Background(), "plain-model", func() error {
						assert.Equal(t, 12.0, operation_setting.GetToolPriceForModel("web_search", "plain-model"))
						assert.Zero(t, operation_setting.GetToolPriceForModel("web_search", "custom-model"))
						return nil
					}))
					var after CatalogSyncState
					require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
					assert.Equal(t, before, after, "format-only saves must preserve the entire coordination state")
					fresh, err := GetCatalogSyncPlan(context.Background(), plan.ID, actor)
					require.NoError(t, err)
					assert.Equal(t, plan, fresh, "the prepared plan remains valid and unchanged")
				})
			}
			var operations int64
			require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&operations).Error)
			assert.Zero(t, operations)
		})
	}
}

func TestCatalogRuntimeOrdinaryHistoricalRetention(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			stored := `{"retired::orphan":"tier(\"raw\", u(\"seconds\") * 0.371)"}`
			require.NoError(t, db.Create(&Option{Key: billing_setting.PluginBillingExprOption, Value: stored}).Error)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"historical-orphan":0}`}))
			var row Option
			require.NoError(t, db.First(&row, commonKeyCol+" = ?", billing_setting.PluginBillingExprOption).Error)
			assert.Equal(t, stored, row.Value)
			require.ErrorContains(t, UpdateOptionsBulk(map[string]string{billing_setting.PluginBillingExprOption: `{"retired::orphan":"u(\"seconds\") * 9"}`}), "does not declare")
			require.NoError(t, UpdateOptionsBulk(map[string]string{billing_setting.PluginBillingExprOption: `{}`}))
			require.NoError(t, db.First(&row, commonKeyCol+" = ?", billing_setting.PluginBillingExprOption).Error)
			assert.Equal(t, `{}`, row.Value)
			var priceRows int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "ModelRatio").Count(&priceRows).Error)
			assert.Zero(t, priceRows, "ordinary typed defaults must not become stored options")
		})
	}
}

func TestCatalogRuntimeOrdinaryFailureRecovery(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			updatePricingLock.Lock()
			err := UpdateOption("molii_grok_tool_price.image_generation", "0.23")
			updatePricingLock.Unlock()
			require.ErrorIs(t, err, ErrCatalogWriterBusy)
			var pending CatalogSyncState
			require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
			assert.Equal(t, int64(1), pending.Revision)
			assert.Zero(t, pending.RuntimeRevision)
			assert.Equal(t, "committed_pending_publish", pending.PublicationState)
			assert.Empty(t, pending.PendingOperationID)
			require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "any-model", func() error { t.Error("pending capture ran"); return nil }), ErrCatalogPublicationPending)
			require.ErrorIs(t, UpdateOption("molii_grok_tool_price.image_generation", "0.25"), ErrCatalogPublicationPending)
			vendor := Vendor{Name: "blocked"}
			require.ErrorIs(t, vendor.Insert(), ErrCatalogPublicationPending)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			var ready CatalogSyncState
			require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
			assert.Equal(t, pending.Revision, ready.Revision)
			assert.Equal(t, ready.Revision, ready.RuntimeRevision)
			var row Option
			require.NoError(t, db.First(&row, commonKeyCol+" = ?", "molii_grok_tool_price.image_generation").Error)
			assert.Equal(t, "0.23", row.Value)
			require.NoError(t, WithCatalogPricingRead(context.Background(), "any-model", func() error { return nil }))
			assertRuntimeLocksReleased(t)
		})
	}
}

func TestCatalogRuntimeOrdinaryFreshRecheck(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			prepared, err := prepareOrdinaryCatalogMutation(context.Background(), db, map[string]string{"ModelPrice": `{"late":1}`})
			require.NoError(t, err)
			canceled, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = prepareOrdinaryCatalogMutation(canceled, db, nil)
			require.ErrorIs(t, err, context.Canceled)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"winner":2}`}))
			err = WithCatalogWriteBarrier(func() error {
				return commitOrdinaryCatalogMutationGuarded(context.Background(), db, prepared, func(*gorm.DB) error {
					t.Error("obsolete preparation reached mutation callback")
					return nil
				}, nil)
			})
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			assert.False(t, prepared.wrote)
			price, ok := ratio_setting.GetModelPrice("winner", false)
			assert.True(t, ok)
			assert.Equal(t, 2.0, price)
			// Metadata-only callbacks cannot smuggle an unstaged pricing write.
			err = WithModelMetadataTransaction(func(tx *gorm.DB) error {
				return tx.Model(&Option{}).Where(commonKeyCol+" = ?", "ModelPrice").Update("value", `{"smuggled":9}`).Error
			})
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			var row Option
			require.NoError(t, db.First(&row, commonKeyCol+" = ?", "ModelPrice").Error)
			assert.Equal(t, `{"winner":2}`, row.Value)
		})
	}
}

func TestCatalogRuntimeOrdinaryPreparationBudget(t *testing.T) {
	db := catalogFenceTestDB(t, "sqlite")
	preserveOrdinaryCatalogRuntime(t)
	catalogBusinessActor(t, db)
	attempts := 0
	err := withOrdinaryCatalogMutation(db, nil, func(prepared *catalogOrdinaryMutation) error {
		attempts++
		assert.False(t, prepared.wrote)
		return ErrCatalogSyncPlanStale // Another writer won before the SQL callback.
	})
	require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
	assert.Equal(t, 64, attempts, "pre-callback stale preparation has a finite retry cap")
	for _, result := range []error{ErrCatalogSyncPlanStale, ErrCatalogCommitUncertain} {
		attempts = 0
		err = withOrdinaryCatalogMutation(db, nil, func(prepared *catalogOrdinaryMutation) error {
			attempts++
			prepared.wrote = true
			return result
		})
		require.ErrorIs(t, err, result)
		assert.Equal(t, 1, attempts, "a started callback is never replayed")
	}
}

func TestCatalogRuntimeOrdinaryNativeAcknowledgements(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			pool, err := db.DB()
			require.NoError(t, err)
			loan, err := pool.Conn(context.Background())
			require.NoError(t, err)
			defer loan.Close()
			require.NoError(t, loan.Raw(func(native any) error {
				connection := &catalogBusinessLostAckConnection{Conn: native.(driver.Conn)}
				observed := sql.OpenDB(catalogBusinessLostAckConnector{connection, pool.Driver()})
				observed.SetMaxOpenConns(1)
				defer observed.Close()
				root := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
				root.Statement.ConnPool = observed
				DB = root
				defer func() { DB = db }()
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("ordinary-lost-ack", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_states" {
						if values, ok := tx.Statement.Dest.(map[string]any); ok && values["publication_state"] != nil {
							connection.armed.Store(true)
						}
					}
				}))
				defer db.Callback().Update().Remove("ordinary-lost-ack")
				calls := 0
				err := WithModelMetadataTransaction(func(tx *gorm.DB) error {
					calls++
					return tx.Create(&Vendor{Name: "exactly-once"}).Error
				})
				require.ErrorIs(t, err, ErrCatalogCommitUncertain, "a matching ordinary postimage is not unique attempt identity")
				assert.Equal(t, 1, calls)
				var state CatalogSyncState
				require.NoError(t, root.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, "committed_pending_publish", state.PublicationState)
				assert.Zero(t, state.RuntimeRevision)
				require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
				assert.Equal(t, int64(2), connection.dropped.Load(), "mutation and recovery ACK actually lost their native responses")
				require.NoError(t, root.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, int64(1), state.Revision)
				assert.Equal(t, state.Revision, state.RuntimeRevision)
				assert.Equal(t, "ready", state.PublicationState)
				return nil
			}))
		})
	}
}

func TestCatalogRuntimeOrdinaryMetadataCallbacks(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			first, second := Vendor{Name: "first"}, Vendor{Name: "second"}
			require.NoError(t, first.Insert())
			require.NoError(t, second.Insert())
			require.NoError(t, ReorderVendors([]int{second.Id, first.Id}))
			first.Description = "ordinary vendor edit"
			require.NoError(t, first.Update())
			model := Model{ModelName: "metadata-model", VendorID: first.Id, Status: 1, SyncOfficial: 1}
			require.NoError(t, model.Insert())
			var actual Model
			require.NoError(t, db.First(&actual, model.Id).Error)
			var actualVendor Vendor
			require.NoError(t, db.First(&actualVendor, first.Id).Error)
			result, err := ApplyMetadataSync([]MetadataSyncUpdate{{
				MetadataSyncSelection: MetadataSyncSelection{ModelName: model.ModelName, RecordVersion: MetadataRecordVersion(&actual, &actualVendor, nil), Fields: []string{"description"}},
				Values:                MetadataValues{Description: "official update", NameRule: NameRuleExact, Status: 1},
			}}, nil)
			require.NoError(t, err)
			assert.Len(t, result.UpdatedModels, 1)
			channel := Channel{Key: "test", Name: "ordinary-reconcile", Models: "reconciled-model", Status: common.ChannelStatusEnabled}
			require.NoError(t, db.Create(&channel).Error)
			require.NoError(t, db.Create(&Ability{Group: "default", Model: "reconciled-model", ChannelId: channel.Id, Enabled: true}).Error)
			summary, err := ReconcileEnabledModelMetadata()
			require.NoError(t, err)
			assert.Equal(t, 1, summary.CreatedModels)
			var reconciled Model
			require.NoError(t, db.First(&reconciled, "model_name = ?", "reconciled-model").Error)
			require.NoError(t, ReorderModels([]int{reconciled.Id, model.Id}))
			require.NoError(t, second.Delete())
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			summary, err = ReconcileEnabledModelMetadata()
			require.NoError(t, err)
			assert.Zero(t, summary.CreatedModels)
			var after CatalogSyncState
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Equal(t, before, after)
			assert.Equal(t, after.Revision, after.RuntimeRevision)
			assert.Equal(t, "ready", after.PublicationState)
			// These are real deferred data migrations, exercised only after the
			// full schema/plugin prerequisites above, never a startup bypass.
			require.NoError(t, migrateModelBillingCurrency(db))
			require.NoError(t, BackfillLocalMarketplaceMetadata(db))
			require.NoError(t, InitializeMarketplaceDisplayOrders(db))
		})
	}
}

func TestCatalogRuntimeOrdinaryMissingPrerequisites(t *testing.T) {
	db := catalogSyncTestDB(t, "sqlite")
	for _, table := range []any{&Option{}, &Model{}, &Vendor{}} {
		require.NoError(t, db.AutoMigrate(table))
	}
	require.NoError(t, MigrateCatalogSync(db))
	callbackRan := false
	require.Error(t, WithModelMetadataTransaction(func(*gorm.DB) error { callbackRan = true; return nil }))
	assert.False(t, callbackRan)
	require.Error(t, InitializeMarketplaceDisplayOrders(db))
	require.Error(t, migrateModelBillingCurrency(db))
	require.Error(t, BackfillLocalMarketplaceMetadata(db))
	var state CatalogSyncState
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Zero(t, state.Revision)
}

func TestCatalogRuntimeOrdinaryForeignRoot(t *testing.T) {
	active := catalogFenceTestDB(t, "sqlite")
	preserveOrdinaryCatalogRuntime(t)
	catalogBusinessActor(t, active)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"active":3}`}))
	foreign := catalogFenceTestDB(t, "sqlite")
	DB = active
	before := runtimeStageForTest(t, active).input
	called := false
	err := withMarketplaceOrderTransaction(foreign, func(tx *gorm.DB) error {
		called = true
		return tx.Create(&Vendor{Name: "must-not-write"}).Error
	})
	require.ErrorContains(t, err, "active root database")
	assert.False(t, called)
	var count int64
	require.NoError(t, foreign.Model(&Vendor{}).Count(&count).Error)
	assert.Zero(t, count)
	assert.True(t, before.same(runtimeStageForTest(t, active).input))
	price, ok := ratio_setting.GetModelPrice("active", false)
	assert.True(t, ok)
	assert.Equal(t, 3.0, price)
}

func TestCatalogRuntimeOrdinaryCurrencyBoundary(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			entry := Model{ModelName: "currency-model", BillingCurrency: "USD"}
			require.NoError(t, entry.Insert())
			require.NoError(t, UpdateOptionsBulk(map[string]string{
				"billing_setting.billing_mode": `{"currency-model":"tiered_expr"}`,
				"billing_setting.billing_expr": `{"currency-model":"p * 3"}`,
			}))
			rebuilt := false
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("ordinary-currency-boundary", func(tx *gorm.DB) {
				if tx.Statement.Table != "vendors" || tx.Statement.ConnPool != db.Statement.ConnPool {
					return
				}
				rebuilt = true
				assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), entry.ModelName, func() error {
					t.Error("selection escaped between currency commit and runtime acknowledgement")
					return nil
				}), ErrCatalogWriterBusy)
				assert.ErrorIs(t, TryWithCatalogWriteBarrier(context.Background(), func() error { return nil }), ErrCatalogWriterBusy)
				published := make(chan struct{})
				go func() { jsplugin.DefaultRegistry.SetEnabled(true); close(published) }()
				select {
				case <-published:
				case <-time.After(3 * time.Second):
					t.Error("registry pin retained during ordinary required rebuild")
				}
			}))
			entry.BillingCurrency = "CNY"
			err := entry.Update()
			require.NoError(t, db.Callback().Query().Remove("ordinary-currency-boundary"))
			require.NoError(t, err)
			require.True(t, rebuilt)
			require.NoError(t, WithCatalogPricingRead(context.Background(), entry.ModelName, func() error {
				money, _, err := ResolveBillingMoneyContext(db, entry.ModelName)
				require.NoError(t, err)
				assert.Equal(t, "CNY", string(money.SourceCurrency))
				expression, ok := billing_setting.GetBillingExpr(entry.ModelName)
				assert.True(t, ok)
				assert.Equal(t, "p * 3", expression)
				return nil
			}))
			assertRuntimeLocksReleased(t)
		})
	}
}

func TestCatalogRuntimeOrdinaryUnknownMutation(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			preserveOrdinaryCatalogRuntime(t)
			catalogBusinessActor(t, db)
			pool, err := db.DB()
			require.NoError(t, err)
			loan, err := pool.Conn(context.Background())
			require.NoError(t, err)
			defer loan.Close()
			require.NoError(t, loan.Raw(func(native any) error {
				connection := &catalogBusinessLostAckConnection{Conn: native.(driver.Conn)}
				observed := sql.OpenDB(catalogBusinessLostAckConnector{connection, pool.Driver()})
				observed.SetMaxOpenConns(1)
				defer observed.Close()
				root := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
				root.Statement.ConnPool = observed
				DB = root
				defer func() { DB = db }()
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("ordinary-unknown-commit", func(tx *gorm.DB) {
					if tx.Statement.Table == "vendors" {
						connection.armed.Store(true)
					}
				}))
				lookupFailure := errors.New("ordinary durable lookup unavailable")
				require.NoError(t, db.Callback().Query().Before("gorm:query").Register("ordinary-unknown-lookup", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_states" && connection.dropped.Load() == 1 {
						tx.AddError(lookupFailure)
					}
				}))
				calls := 0
				err := WithModelMetadataTransaction(func(tx *gorm.DB) error {
					calls++
					return tx.Create(&Vendor{Name: "committed-once"}).Error
				})
				require.NoError(t, db.Callback().Create().Remove("ordinary-unknown-commit"))
				require.NoError(t, db.Callback().Query().Remove("ordinary-unknown-lookup"))
				require.ErrorIs(t, err, ErrCatalogCommitUncertain)
				require.ErrorIs(t, err, lookupFailure)
				assert.Equal(t, 1, calls)
				assert.Equal(t, int64(1), connection.dropped.Load())
				var state CatalogSyncState
				require.NoError(t, root.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, "committed_pending_publish", state.PublicationState)
				require.ErrorIs(t, WithModelMetadataTransaction(func(*gorm.DB) error { t.Error("pending callback ran"); return nil }), ErrCatalogPublicationPending)
				require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
				require.NoError(t, root.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, int64(1), state.Revision)
				assert.Equal(t, state.Revision, state.RuntimeRevision)
				var count int64
				require.NoError(t, root.Model(&Vendor{}).Count(&count).Error)
				assert.Equal(t, int64(1), count)
				return nil
			}))
		})
	}
}

// A detached candidate is insufficient: failure after the actual mutation must
// leave a recoverable pending operation, never permit another mutation, and
// never replace its original receipt or preimage during recovery.
func TestCatalogRuntimeManagedFailures(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			for _, phase := range []string{"rebuild", "ack", "plugin-drift"} {
				t.Run(phase, func(t *testing.T) {
					db := catalogFenceTestDB(t, engine)
					actor := catalogBusinessActor(t, db)
					plan, err := CreateCatalogSyncPlan(context.Background(), catalogBusinessPriceSource(t, db), actor, time.Now())
					require.NoError(t, err)
					injected := errors.New("managed publication unavailable")
					rebuilt := false
					require.NoError(t, db.Callback().Query().Before("gorm:query").Register("managed-rebuild", func(tx *gorm.DB) {
						if tx.Statement.Table != "vendors" || tx.Statement.ConnPool != db.Statement.ConnPool {
							return
						}
						rebuilt = true
						assert.ErrorIs(t, TryWithCatalogWriteBarrier(context.Background(), func() error { return nil }), ErrCatalogWriterBusy)
						assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error { return nil }), ErrCatalogWriterBusy)
						var state CatalogSyncState
						require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
						assert.Equal(t, "committed_pending_publish", state.PublicationState)
						assert.Equal(t, int64(1), state.Revision)
						// A writer acquisition proves the committing registry pin was
						// released, not merely that another reader pin is available.
						published := make(chan struct{})
						go func() { jsplugin.DefaultRegistry.SetEnabled(phase != "plugin-drift"); close(published) }()
						select {
						case <-published:
						case <-time.After(3 * time.Second):
							t.Fatal("actual mutation pin survived into required rebuild")
						}
						if phase == "rebuild" {
							tx.AddError(injected)
						}
					}))
					require.NoError(t, db.Callback().Update().Before("gorm:update").Register("managed-ack", func(tx *gorm.DB) {
						if tx.Statement.Table == "catalog_sync_operations" && phase == "ack" {
							if values, ok := tx.Statement.Dest.(map[string]any); ok && values["state"] == "succeeded" {
								tx.AddError(injected)
							}
						}
					}))
					result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "managed-failure", actor)
					require.NoError(t, db.Callback().Query().Remove("managed-rebuild"))
					require.NoError(t, db.Callback().Update().Remove("managed-ack"))
					require.Error(t, err)
					require.True(t, rebuilt)
					if phase != "plugin-drift" {
						require.ErrorIs(t, err, injected)
					}
					assertRuntimePending(t, db, result)
					assertRuntimeLocksReleased(t)
					var original CatalogSyncOperation
					require.NoError(t, db.First(&original, "id = ?", result.OperationID).Error)
					assert.Equal(t, "committed_pending_publish", original.State)
					if phase == "plugin-drift" {
						jsplugin.DefaultRegistry.SetEnabled(true)
					}
					replay, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
					require.NoError(t, err)
					assert.Equal(t, result, replay)
					require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
					var recovered CatalogSyncOperation
					require.NoError(t, db.First(&recovered, "id = ?", result.OperationID).Error)
					assert.Equal(t, "succeeded", recovered.State)
					assert.Equal(t, original.Result, recovered.Result)
					assert.Equal(t, original.Backup, recovered.Backup)
					assert.Equal(t, original.History, recovered.History)
					catalogBusinessFixturePublished(t, db, result)
					restore, err := CreateCatalogSyncRestorePlan(context.Background(), result.OperationID, actor, time.Now())
					require.NoError(t, err)
					inverse, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "managed-safe-inverse", actor)
					require.NoError(t, err)
					catalogBusinessFixturePublished(t, db, inverse)
					var count int64
					require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
					assert.Equal(t, int64(2), count)
				})
			}
		})
	}
}

func TestCatalogRuntimeManagedFreshRecheck(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			for _, drift := range []string{"raw-fx", "currency", "desired-plugin", "actor"} {
				t.Run(drift, func(t *testing.T) {
					db := catalogFenceTestDB(t, engine)
					actor := catalogBusinessActor(t, db)
					plan, err := CreateCatalogSyncPlan(context.Background(), catalogBusinessPriceSource(t, db), actor, time.Now())
					require.NoError(t, err)
					before, err := catalogPersistedDigest(db)
					require.NoError(t, err)
					captures := 0
					require.NoError(t, db.Callback().Query().After("gorm:query").Register("managed-final-drift", func(tx *gorm.DB) {
						if _, ok := tx.Statement.Dest.(*CatalogSyncState); !ok {
							return
						}
						captures++
						if captures != 2 { // fresh final root after detached staging
							return
						}
						db := tx.Session(&gorm.Session{NewDB: true})
						switch drift {
						case "raw-fx":
							tx.AddError(db.Create(&Option{Key: "USDExchangeRate", Value: "7.333"}).Error)
						case "currency":
							tx.AddError(db.Model(&Model{}).Where("model_name = ?", "managed-model").Update("billing_currency", "USD").Error)
						case "desired-plugin":
							tx.AddError(db.Create(&Option{Key: "TaskPluginEnabled", Value: "false"}).Error)
						case "actor":
							tx.AddError(db.Model(&User{}).Where("id = ?", actor.UserID).Update("role", common.RoleCommonUser).Error)
						}
					}))
					_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "must-not-commit", actor)
					require.NoError(t, db.Callback().Query().Remove("managed-final-drift"))
					require.Error(t, err)
					require.GreaterOrEqual(t, captures, 2)
					after, err := catalogPersistedDigest(db)
					require.NoError(t, err)
					assert.Equal(t, before, after)
					var count int64
					require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
					assert.Zero(t, count)
				})
			}
		})
	}
}

func TestCatalogRuntimeManagedResetAndStrictAdoption(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			for key, value := range map[string]string{"ModelPrice": `{"removed":9}`, "starai_video_price.standard_720p": "13.25"} {
				require.NoError(t, db.Create(&Option{Key: key, Value: value}).Error)
			}
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			assert.Contains(t, ratio_setting.GetExposedData()["model_price"], "removed")
			require.NoError(t, db.Where(map[string]any{"key": []string{"ModelPrice", "starai_video_price.standard_720p"}}).Delete(&Option{}).Error)
			for key, value := range map[string]string{"ModelRatio": `{}`, "billing_setting.billing_mode": `{}`, "tool_price_setting.prices": `{"web_search":0}`, "molii_grok_price.image_standard_1k": "0", "USDExchangeRate": "7.25"} {
				require.NoError(t, db.Create(&Option{Key: key, Value: value}).Error)
			}
			source := catalogValidationFixture(t, db, jsplugin.DefaultRegistry)
			source.SourceID = "dev"
			var err error
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			// A real sealed same-value adoption must carry a strict source leaf.
			// Removing effective providers in a detached copy demonstrates that
			// the prospective seam cannot claim runtime historical retention.
			stage := runtimeStageForTest(t, db)
			next, strict, err := catalogPlanPricing(stage.input.options, plan)
			require.NoError(t, err)
			require.NotEmpty(t, strict)
			var facts catalogValidationData
			require.NoError(t, common.UnmarshalJsonStr(stage.input.dependencies.canonical, &facts))
			facts.Effective = nil
			facts.Aliases = nil
			encoded, err := common.Marshal(facts)
			require.NoError(t, err)
			stage.input.dependencies.canonical = string(encoded)
			stage.input.options = next
			_, err = stageCatalogRuntime(stage.input)
			require.NoError(t, err, "committed historical retention alone would accept this stale local price")
			_, err = stageProspectiveCatalogRuntime(stage.input, next, strict)
			require.Error(t, err, "server-derived same-value adoption must remove historical retention")
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "strict-adoption", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, result)
			assert.NotContains(t, ratio_setting.GetExposedData()["model_price"], "removed")
			common.OptionMapRWMutex.RLock()
			assert.Equal(t, "{}", common.OptionMap["ModelRatio"])
			assert.JSONEq(t, `{"web_search":0}`, common.OptionMap["tool_price_setting.prices"])
			assert.Equal(t, "0", common.OptionMap["molii_grok_price.image_standard_1k"])
			assert.NotEqual(t, "13.25", common.OptionMap["starai_video_price.standard_720p"])
			common.OptionMapRWMutex.RUnlock()
			var persisted Option
			require.NoError(t, db.Where(map[string]any{"key": "USDExchangeRate"}).First(&persisted).Error)
			assert.Equal(t, "7.25", persisted.Value)
			var defaults int64
			require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": []string{"ModelPrice", "starai_video_price.standard_720p"}}).Count(&defaults).Error)
			assert.Zero(t, defaults, "compiled resets must not become persisted source prices")
		})
	}
}

func TestCatalogRuntimeManagedUnknownMutation(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			pool, err := db.DB()
			require.NoError(t, err)
			loan, err := pool.Conn(context.Background())
			require.NoError(t, err)
			defer loan.Close()
			require.NoError(t, loan.Raw(func(native any) error {
				connection := &catalogBusinessLostAckConnection{Conn: native.(driver.Conn)}
				observed := sql.OpenDB(catalogBusinessLostAckConnector{connection, pool.Driver()})
				observed.SetMaxOpenConns(1)
				defer observed.Close()
				root := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
				root.Statement.ConnPool = observed
				DB = root
				defer func() { DB = db }()
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("managed-unknown-commit", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						connection.armed.Store(true)
					}
				}))
				lookupFailure := errors.New("durable lookup unavailable")
				require.NoError(t, db.Callback().Query().Before("gorm:query").Register("managed-unknown-lookup", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" && tx.Statement.ConnPool == observed && connection.dropped.Load() == 1 {
						tx.AddError(lookupFailure)
					}
				}))
				result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "unknown-mutation", actor)
				require.NoError(t, db.Callback().Create().Remove("managed-unknown-commit"))
				require.NoError(t, db.Callback().Query().Remove("managed-unknown-lookup"))
				require.ErrorIs(t, err, ErrCatalogCommitUncertain)
				require.ErrorIs(t, err, lookupFailure)
				assert.Equal(t, catalogmanifest.Result{}, result, "an unproven provisional receipt must not escape as a known committed result")
				assert.Equal(t, int64(1), connection.dropped.Load())
				var operation CatalogSyncOperation
				require.NoError(t, root.First(&operation, "id = ?", "unknown-mutation").Error)
				assert.Equal(t, "committed_pending_publish", operation.State)
				var receipt catalogmanifest.Result
				require.NoError(t, common.UnmarshalJsonStr(string(operation.Result), &receipt))
				assertRuntimePending(t, root, receipt)
				replayed, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, receipt.OperationID, actor)
				require.NoError(t, err)
				assert.Equal(t, receipt, replayed)
				require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
				catalogBusinessFixturePublished(t, root, receipt)
				connection.armed.Store(true)
				replayed, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, receipt.OperationID, actor)
				require.NoError(t, err, "exact replay still reconciles a lost acknowledgement of its capture transaction")
				assert.Equal(t, receipt, replayed)
				var count int64
				require.NoError(t, root.Model(&CatalogSyncOperation{}).Count(&count).Error)
				assert.Equal(t, int64(1), count)
				return nil
			}))
		})
	}
}

// A publisher that only swaps typed settings, or acknowledges before rebuilding,
// must fail this real committed-operation lifecycle test.
func TestCatalogRuntimePublication(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			preserveCatalogCandidate(t)
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogBusinessPriceSource(t, db)
			require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "managed-model").Update("billing_currency", "USD").Error)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			oldExpression, _ := billing_setting.GetBillingExpr("managed-model")
			assert.Equal(t, "p * 2", oldExpression)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "runtime-publication", actor)
			require.NoError(t, err)
			var original CatalogSyncOperation
			require.NoError(t, db.First(&original, "id = ?", result.OperationID).Error)
			assert.Equal(t, "succeeded", original.State, "actual apply must finish publication before reporting live success")
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			assert.Equal(t, result.Revision, state.RuntimeRevision)
			assert.Equal(t, "ready", state.PublicationState)
			assert.Empty(t, state.PendingOperationID)
			var published CatalogSyncOperation
			require.NoError(t, db.First(&published, "id = ?", result.OperationID).Error)
			assert.Equal(t, "succeeded", published.State)
			assert.Equal(t, original.Result, published.Result)
			assert.Equal(t, original.Backup, published.Backup)
			assert.Equal(t, original.History, published.History)
			require.NoError(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error {
				expression, ok := billing_setting.GetBillingExpr("managed-model")
				assert.True(t, ok)
				assert.Equal(t, `tier("base", p * 7.123 + c * 3)`, expression)
				currencies, err := LoadModelBillingCurrencies(db, []string{"managed-model"}, false)
				assert.Equal(t, "CNY", string(currencies["managed-model"].BillingCurrency))
				return err
			}))
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			replayed, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
			require.NoError(t, err)
			assert.Equal(t, result, replayed)
			var count int64
			require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
			assert.Equal(t, int64(1), count)
			require.Error(t, PublishCatalogSyncRevision(context.Background(), result.Revision+1))
			common.OptionMapRWMutex.RLock()
			assert.NotEmpty(t, common.OptionMap["billing_setting.billing_expr"])
			common.OptionMapRWMutex.RUnlock()
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), result.OperationID, actor, time.Now())
			require.NoError(t, err)
			inverse, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "runtime-inverse", actor)
			require.NoError(t, err)
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			assert.Equal(t, inverse.Revision, state.RuntimeRevision)
			assert.Equal(t, "ready", state.PublicationState)
			require.NoError(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error {
				expression, _ := billing_setting.GetBillingExpr("managed-model")
				assert.Equal(t, "p * 2", expression)
				return nil
			}))
		})
	}
}

func TestCatalogRuntimeFailureRecovery(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			preserveCatalogCandidate(t)
			db := catalogFenceTestDB(t, engine)
			result := runtimePendingOperation(t, db)
			var original CatalogSyncOperation
			require.NoError(t, db.First(&original, "id = ?", result.OperationID).Error)
			before := captureCatalogCandidateOptions()
			require.NoError(t, db.Create(&Option{Key: "tool_price_setting.prices", Value: `{"broken":-1}`}).Error)
			require.Error(t, PublishCatalogSyncRevision(context.Background(), result.Revision))
			assert.Equal(t, before, captureCatalogCandidateOptions(), "a late invalid option must not partially publish")
			require.NoError(t, db.Where(map[string]any{"key": "tool_price_setting.prices"}).Delete(&Option{}).Error)
			injected := errors.New("required rebuild unavailable")
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("runtime-cache-failure", func(tx *gorm.DB) {
				if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == db.Statement.ConnPool {
					tx.AddError(injected)
				}
			}))
			err := PublishCatalogSyncRevision(context.Background(), result.Revision)
			require.NoError(t, db.Callback().Query().Remove("runtime-cache-failure"))
			require.ErrorIs(t, err, injected)
			assertRuntimePending(t, db, result)
			injected = errors.New("ack update unavailable")
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("runtime-ack-failure", func(tx *gorm.DB) {
				if tx.Statement.Table == "catalog_sync_operations" {
					tx.AddError(injected)
				}
			}))
			err = RecoverCatalogSyncRuntime(context.Background())
			require.NoError(t, db.Callback().Update().Remove("runtime-ack-failure"))
			require.ErrorIs(t, err, injected)
			assertRuntimePending(t, db, result)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			var published CatalogSyncOperation
			require.NoError(t, db.First(&published, "id = ?", result.OperationID).Error)
			assert.Equal(t, "succeeded", published.State)
			assert.Equal(t, original.Result, published.Result)
			assert.Equal(t, original.Backup, published.Backup)
		})
	}
}

func assertRuntimePending(t *testing.T, db *gorm.DB, result catalogmanifest.Result) {
	t.Helper()
	var state CatalogSyncState
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Equal(t, "committed_pending_publish", state.PublicationState)
	assert.NotEqual(t, state.Revision, state.RuntimeRevision)
	assert.Equal(t, result.OperationID, state.PendingOperationID)
	require.ErrorIs(t, (&Vendor{Name: "blocked-write"}).Insert(), ErrCatalogPublicationPending)
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error {
		t.Fatal("failed publication must not select any new price")
		return nil
	}), ErrCatalogPublicationPending)
}

func TestCatalogRuntimeDrift(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			preserveCatalogCandidate(t)
			db := catalogFenceTestDB(t, engine)
			result := runtimePendingOperation(t, db)
			for _, field := range []string{"currency", "raw-option", "generation", "baseline"} {
				t.Run(field, func(t *testing.T) {
					stage := runtimeStageForTest(t, db)
					before := captureCatalogCandidateOptions()
					switch field {
					case "currency":
						require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "managed-model").Update("billing_currency", "USD").Error)
					case "raw-option":
						require.NoError(t, db.Create(&Option{Key: "ModelPrice", Value: `{"historical-orphan":0}`}).Error)
					case "generation":
						_, err := jsplugin.DefaultRegistry.RegisterFactory(pricingUsagePluginSource("1.0.0", `{seconds:{type:"number",unit:"second"}}`), jsplugin.Options{})
						require.NoError(t, err)
					case "baseline":
						require.NoError(t, db.Model(&CatalogSyncState{}).Where("id = ?", CatalogSyncStateID).Update("baseline_generation", gorm.Expr("baseline_generation + 1")).Error)
					}
					err := TryWithCatalogWriteBarrier(context.Background(), func() error { return publishCatalogRuntimeGuarded(context.Background(), stage) })
					require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
					assert.Equal(t, before, captureCatalogCandidateOptions())
					assertRuntimePending(t, db, result)
				})
			}
		})
	}
}

func TestCatalogRuntimeScopeAndAcquisition(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			preserveCatalogCandidate(t)
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			called := false
			read := func() error { called = true; return nil }
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			require.ErrorIs(t, WithCatalogPricingRead(ctx, "any", read), context.Canceled)
			require.False(t, called)
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "any", read), ErrCatalogWriterBusy)
				require.ErrorIs(t, PublishCatalogSyncRevision(context.Background(), 0), ErrCatalogWriterBusy)
				return nil
			}))
			source := catalogSyncTestSource(t)
			require.NoError(t, appendManagedEntry(&source, catalogmanifest.KindModel, "changed-model", catalogmanifest.ModelValue{ModelName: "changed-model", BillingCurrency: "CNY"}))
			var err error
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			pendingReads := 0
			injected := errors.New("publication recheck unavailable")
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("runtime-scope-before-publish", func(tx *gorm.DB) {
				if state, ok := tx.Statement.Dest.(*CatalogSyncState); ok && state.PendingOperationID == "scope-operation" {
					pendingReads++
					if pendingReads == 2 {
						tx.AddError(injected)
					}
				}
			}))
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "scope-operation", actor)
			require.NoError(t, db.Callback().Query().Remove("runtime-scope-before-publish"))
			require.ErrorIs(t, err, injected)
			require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "changed-model", read), ErrCatalogPublicationPending)
			require.NoError(t, WithCatalogPricingRead(context.Background(), "unaffected-model", read))
			require.True(t, called)
			var operation CatalogSyncOperation
			require.NoError(t, db.First(&operation, "id = ?", result.OperationID).Error)
			var backup catalogOperationBackup
			require.NoError(t, common.UnmarshalJsonStr(string(operation.Backup), &backup))
			backup.After = append(backup.After, catalogmanifest.Entry{Kind: catalogmanifest.KindModelPrice, Key: "unbound-price", Value: "{}"})
			malformed, err := common.Marshal(backup)
			require.NoError(t, err)
			require.NoError(t, db.Model(&operation).Update("backup", string(malformed)).Error)
			require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "unaffected-model", read), ErrCatalogPublicationPending, "incomplete backup scope cannot prove any selection unaffected")
			require.NoError(t, db.Model(&CatalogSyncOperation{}).Where("id = ?", result.OperationID).Update("backup", "{}").Error)
			require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "unaffected-model", read), ErrCatalogPublicationPending)
		})
	}
}

func TestCatalogRuntimeLostAcknowledgement(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			preserveCatalogCandidate(t)
			db := catalogFenceTestDB(t, engine)
			result := runtimePendingOperation(t, db)
			pool, err := db.DB()
			require.NoError(t, err)
			loan, err := pool.Conn(context.Background())
			require.NoError(t, err)
			defer loan.Close()
			require.NoError(t, loan.Raw(func(native any) error {
				connection := &catalogBusinessLostAckConnection{Conn: native.(driver.Conn)}
				observed := sql.OpenDB(catalogBusinessLostAckConnector{connection, pool.Driver()})
				observed.SetMaxOpenConns(1)
				defer observed.Close()
				root := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
				root.Statement.ConnPool = observed
				DB = root
				defer func() { DB = db }()
				var once sync.Once
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("runtime-lost-ack", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						once.Do(func() { connection.armed.Store(true) })
					}
				}))
				defer db.Callback().Update().Remove("runtime-lost-ack")
				require.NoError(t, PublishCatalogSyncRevision(context.Background(), result.Revision))
				var state CatalogSyncState
				require.NoError(t, root.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, "ready", state.PublicationState)
				require.NoError(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error { return nil }))
				return nil
			}))
		})
	}
}

func TestCatalogRuntimeResetAndHistoricalOrphan(t *testing.T) {
	preserveCatalogCandidate(t)
	db := catalogFenceTestDB(t, "sqlite")
	catalogBusinessActor(t, db)
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["USDExchangeRate"] = "7.25"
	common.OptionMap["SiteName"] = "target-only"
	common.OptionMapRWMutex.Unlock()
	for key, raw := range map[string]string{"ModelPrice": `{"historical-orphan":0,"remove":9}`, "ModelRatio": `{}`, "tool_price_setting.prices": `{"web_search":0}`, "starai_video_price.standard_720p": "13.25"} {
		require.NoError(t, db.Create(&Option{Key: key, Value: raw}).Error)
	}
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	value, exists := ratio_setting.GetModelPrice("historical-orphan", false)
	assert.True(t, exists)
	assert.Zero(t, value)
	assert.Empty(t, ratio_setting.GetModelRatioCopy())
	assert.Contains(t, ratio_setting.GetExposedData()["model_price"], "remove")
	require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": "ModelPrice"}).Update("value", `{"historical-orphan":0}`).Error)
	require.NoError(t, db.Where(map[string]any{"key": "starai_video_price.standard_720p"}).Delete(&Option{}).Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	_, exists = ratio_setting.GetModelPrice("remove", false)
	assert.False(t, exists)
	assert.NotContains(t, ratio_setting.GetExposedData()["model_price"], "remove")
	common.OptionMapRWMutex.RLock()
	options := maps.Clone(common.OptionMap)
	common.OptionMapRWMutex.RUnlock()
	assert.Equal(t, "7.25", options["USDExchangeRate"])
	assert.Equal(t, "target-only", options["SiteName"])
	assert.NotEqual(t, "13.25", options["starai_video_price.standard_720p"])
	var saved []Option
	require.NoError(t, db.Find(&saved).Error)
	for _, option := range saved {
		assert.False(t, strings.HasPrefix(option.Key, "billing_setting."), "recovery must not persist generated expressions/defaults")
	}
}

func TestCatalogRuntimeReadyRecoveryFailureBlocksWrites(t *testing.T) {
	preserveCatalogCandidate(t)
	db := catalogFenceTestDB(t, "sqlite")
	catalogBusinessActor(t, db)
	require.NoError(t, db.Create(&Option{Key: "ModelPrice", Value: `{"invalid":-1}`}).Error)
	require.Error(t, RecoverCatalogSyncRuntime(context.Background()))
	var state CatalogSyncState
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Equal(t, "committed_pending_publish", state.PublicationState, "an unready process cannot leave ordinary writers falsely enabled")
	require.ErrorIs(t, (&Vendor{Name: "blocked-recovery-write"}).Insert(), ErrCatalogPublicationPending)
}

func TestCatalogRuntimeCacheContention(t *testing.T) {
	preserveCatalogCandidate(t)
	db := catalogFenceTestDB(t, "sqlite")
	result := runtimePendingOperation(t, db)
	updatePricingLock.Lock()
	err := PublishCatalogSyncRevision(context.Background(), result.Revision)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	assertRuntimePending(t, db, result)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
}

func runtimeAdvancedChannel(t *testing.T, db *gorm.DB, memoryCache bool) {
	t.Helper()
	original := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = memoryCache
	t.Cleanup(func() { common.MemoryCacheEnabled = original })
	channel := Channel{Key: "runtime-advanced", Type: constant.ChannelTypeAdvancedCustom, Models: "managed-model", Status: common.ChannelStatusEnabled, OtherSettings: `{}`}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&Ability{ChannelId: channel.Id, Model: "managed-model", Group: "default", Enabled: true}).Error)
}

func assertRuntimeLocksReleased(t *testing.T) {
	t.Helper()
	for name, lock := range map[string]interface {
		TryLock() bool
		Unlock()
	}{
		"catalog": &catalogBarrier, "pricing": &updatePricingLock, "endpoints": &modelSupportEndpointsLock,
	} {
		require.True(t, lock.TryLock(), name+" lock leaked")
		lock.Unlock()
	}
}

func TestCatalogRuntimeAdvancedCacheContention(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			for _, cancelAtRebuild := range []bool{false, true} {
				name := "busy"
				if cancelAtRebuild {
					name = "canceled"
				}
				t.Run(name, func(t *testing.T) {
					preserveCatalogCandidate(t)
					db := catalogFenceTestDB(t, engine)
					result := runtimePendingOperation(t, db)
					runtimeAdvancedChannel(t, db, true)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					entered := make(chan struct{})
					var once sync.Once
					require.NoError(t, db.Callback().Query().After("gorm:query").Register("runtime-advanced-entered", func(tx *gorm.DB) {
						if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == db.Statement.ConnPool {
							once.Do(func() { close(entered) })
						}
					}))
					defer db.Callback().Query().Remove("runtime-advanced-entered")
					channelSyncLock.Lock()
					var release sync.Once
					defer release.Do(channelSyncLock.Unlock)
					done := make(chan error, 1)
					go func() { done <- PublishCatalogSyncRevision(ctx, result.Revision) }()
					select {
					case <-entered:
					case <-time.After(5 * time.Second):
						t.Fatal("publisher did not reach actual advanced-channel rebuild")
					}
					if cancelAtRebuild {
						cancel()
					}
					var err error
					select {
					case err = <-done:
					case <-time.After(time.Second):
						// Release the test-owned lock before reporting RED, avoiding a
						// stranded publisher goroutine or poisoned later tests.
						release.Do(channelSyncLock.Unlock)
						cancel()
						err = <-done
						t.Error("publisher remained blocked on channel cache after cancellation")
					}
					require.True(t, errors.Is(err, ErrCatalogWriterBusy) || errors.Is(err, context.Canceled), "unexpected error: %v", err)
					if !cancelAtRebuild {
						require.ErrorIs(t, err, ErrCatalogWriterBusy)
					}
					assertRuntimeLocksReleased(t)
					assertRuntimePending(t, db, result)
					release.Do(channelSyncLock.Unlock)
					require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
					assertRuntimeLocksReleased(t)
					require.True(t, channelSyncLock.TryLock(), "successful retry leaked channel-cache read lock")
					channelSyncLock.Unlock()
				})
			}
		})
	}
}

func TestCatalogRuntimeQueryHelperSemantics(t *testing.T) {
	db := catalogFenceTestDB(t, "sqlite")
	runtimeAdvancedChannel(t, db, false)
	var saved Channel
	require.NoError(t, db.First(&saved).Error)
	for _, all := range []bool{false, true} {
		ordinary, err := GetChannelById(saved.Id, all)
		require.NoError(t, err)
		bound, err := getChannelByID(db.WithContext(context.Background()), saved.Id, all)
		require.NoError(t, err)
		assert.Equal(t, ordinary, bound)
		if all {
			assert.Equal(t, saved.Key, bound.Key)
		} else {
			assert.Empty(t, bound.Key)
		}
	}
	ordinary, err := GetAllEnableAbilityWithChannels()
	require.NoError(t, err)
	bound, err := getAllEnableAbilityWithChannels(db.WithContext(context.Background()))
	require.NoError(t, err)
	require.Len(t, bound, 1)
	assert.Equal(t, ordinary, bound)
	assert.Equal(t, constant.ChannelTypeAdvancedCustom, bound[0].ChannelType)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = getChannelByID(db.WithContext(ctx), saved.Id, true)
	require.ErrorIs(t, err, context.Canceled)
	_, err = getAllEnableAbilityWithChannels(db.WithContext(ctx))
	require.ErrorIs(t, err, context.Canceled)
	// The pre-existing malformed-settings repair is also a DB operation;
	// cancellation must prevent that fallback from escaping through root DB.
	require.NoError(t, db.Model(&saved).Update("settings", "{").Error)
	saved.OtherSettings = "{"
	saved.getOtherSettings(db.WithContext(ctx))
	var persisted Channel
	require.NoError(t, db.First(&persisted, saved.Id).Error)
	assert.Equal(t, "{", persisted.OtherSettings)
	persisted.GetOtherSettings()
	require.NoError(t, db.First(&persisted, saved.Id).Error)
	assert.Equal(t, "{}", persisted.OtherSettings, "ordinary repair remains unchanged")
}

func TestCatalogRuntimeRebuildSQLContext(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			for _, table := range []string{"abilities", "models", "vendors", "channels"} {
				t.Run(table, func(t *testing.T) {
					preserveCatalogCandidate(t)
					db := catalogFenceTestDB(t, engine)
					result := runtimePendingOperation(t, db)
					runtimeAdvancedChannel(t, db, false)
					type contextKey struct{}
					ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "publisher"))
					defer cancel()
					seen := false
					var queryErr error
					before := func(tx *gorm.DB) {
						if tx.Statement.Table == table && tx.Statement.ConnPool == db.Statement.ConnPool {
							seen = true
							assert.Equal(t, "publisher", tx.Statement.Context.Value(contextKey{}), "actual rebuild SQL lost publisher context")
							cancel() // The real driver query must observe cancellation; no injected SQL error.
						}
					}
					after := func(tx *gorm.DB) {
						if tx.Statement.Table == table && tx.Statement.ConnPool == db.Statement.ConnPool {
							queryErr = tx.Error
						}
					}
					require.NoError(t, db.Callback().Query().Before("gorm:query").Register("runtime-query-cancel", before))
					require.NoError(t, db.Callback().Query().After("gorm:query").Register("runtime-query-observe", after))
					require.NoError(t, db.Callback().Row().Before("gorm:row").Register("runtime-row-cancel", before))
					require.NoError(t, db.Callback().Row().After("gorm:row").Register("runtime-row-observe", after))
					err := PublishCatalogSyncRevision(ctx, result.Revision)
					require.NoError(t, db.Callback().Query().Remove("runtime-query-cancel"))
					require.NoError(t, db.Callback().Query().Remove("runtime-query-observe"))
					require.NoError(t, db.Callback().Row().Remove("runtime-row-cancel"))
					require.NoError(t, db.Callback().Row().Remove("runtime-row-observe"))
					require.True(t, seen, "required query was not reached")
					require.ErrorIs(t, queryErr, context.Canceled, "actual SQL must fail, not only subsequent acknowledgement")
					require.ErrorIs(t, err, context.Canceled)
					assertRuntimeLocksReleased(t)
					assertRuntimePending(t, db, result)
					require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
				})
			}
		})
	}
}

func TestCatalogRuntimeRejectsInvalidCurrency(t *testing.T) {
	preserveCatalogCandidate(t)
	db := catalogFenceTestDB(t, "sqlite")
	catalogBusinessActor(t, db)
	require.NoError(t, db.Create(&Model{ModelName: "invalid-currency", BillingCurrency: "EUR"}).Error)
	before := captureCatalogCandidateOptions()
	require.Error(t, RecoverCatalogSyncRuntime(context.Background()))
	assert.Equal(t, before, captureCatalogCandidateOptions())
}

func TestCatalogRuntimeCompatibilityAndCacheRemoval(t *testing.T) {
	preserveCatalogCandidate(t)
	db := catalogFenceTestDB(t, "sqlite")
	catalogBusinessActor(t, db)
	_, err := jsplugin.DefaultRegistry.RegisterFactory(pricingUsagePluginSource("1.0.0", `{seconds:{type:"number",unit:"second"}}`), jsplugin.Options{})
	require.NoError(t, err)
	channel := Channel{Key: "runtime-fixture", Type: constant.ChannelTypeTaskPlugin, Status: common.ChannelStatusEnabled, Models: "pricing-usage-model"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&Ability{ChannelId: channel.Id, Model: "pricing-usage-model", Group: "default", Enabled: true}).Error)
	vendor := Vendor{Name: "runtime-cache-vendor", Status: 1}
	require.NoError(t, db.Create(&vendor).Error)
	require.NoError(t, db.Create(&Model{ModelName: "pricing-usage-model", BillingCurrency: "CNY", VendorID: vendor.Id}).Error)
	for key, raw := range map[string]string{
		"billing_setting.billing_mode":        `{"pricing-usage-model":"tiered_expr"}`,
		"billing_setting.billing_expr":        `{"pricing-usage-model":"u(\"seconds\") * 0.4"}`,
		"billing_setting.plugin_billing_expr": `{"pricing-usage-probe::pricing-usage-model":"u(\"seconds\") * 0.3"}`,
	} {
		require.NoError(t, db.Create(&Option{Key: key, Value: raw}).Error)
	}
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	items := GetPricing()
	require.Len(t, items, 1)
	require.Len(t, GetVendors(), 1)
	require.Len(t, items[0].BillingPluginVariants, 1)
	assert.Equal(t, `u("seconds") * 0.3`, items[0].BillingPluginVariants[0].BillingExpr)
	require.NoError(t, db.Model(&CatalogSyncState{}).Where("id = ?", CatalogSyncStateID).Update("publication_state", "committed_pending_publish").Error)
	stage := runtimeStageForTest(t, db)
	valid, err := stage.compatible("", map[string]jsplugin.UsageFieldSchema{"seconds": {Type: "number", Unit: "second"}})
	require.NoError(t, err)
	assert.False(t, valid, "a genuinely unconfigured expression remains incompatible")
	stage.compatibility = map[string]bool{}
	require.ErrorIs(t, TryWithCatalogWriteBarrier(context.Background(), func() error { return publishCatalogRuntimeGuarded(context.Background(), stage) }), ErrCatalogSyncPlanStale, "missing staging evidence must not execute compiler or silently hide the variant")
	require.NoError(t, db.Delete(&Ability{}, "channel_id = ?", channel.Id).Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	assert.Empty(t, GetPricing())
	assert.Empty(t, GetVendors())
	assert.Empty(t, GetModelSupportEndpointTypes("pricing-usage-model"))
}

func TestCatalogRuntimeRecheckAfterRebuild(t *testing.T) {
	preserveCatalogCandidate(t)
	db := catalogFenceTestDB(t, "sqlite")
	result := runtimePendingOperation(t, db)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("runtime-rebuild-pause", func(tx *gorm.DB) {
		if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == db.Statement.ConnPool {
			once.Do(func() { close(entered); <-release })
		}
	}))
	defer db.Callback().Query().Remove("runtime-rebuild-pause")
	done := make(chan error, 1)
	go func() { done <- PublishCatalogSyncRevision(context.Background(), result.Revision) }()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("publication ended before rebuild: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("publication did not reach rebuild")
	}
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error { return nil }), ErrCatalogWriterBusy)
	// Registry publication must be possible here: no SQL fence or registry pin
	// may survive into typed publication/rebuild. Ack must reject the new epoch.
	_, err := jsplugin.DefaultRegistry.RegisterFactory(pricingUsagePluginSource("1.0.0", `{seconds:{type:"number",unit:"second"}}`), jsplugin.Options{})
	require.NoError(t, err)
	close(release)
	require.ErrorIs(t, <-done, ErrCatalogSyncPlanStale)
	assertRuntimePending(t, db, result)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
}

func TestCatalogRuntimeRejectsCorruptOperation(t *testing.T) {
	preserveCatalogCandidate(t)
	db := catalogFenceTestDB(t, "sqlite")
	result := runtimePendingOperation(t, db)
	require.NoError(t, db.Model(&CatalogSyncOperation{}).Where("id = ?", result.OperationID).Update("history", `{}`).Error)
	require.Error(t, PublishCatalogSyncRevision(context.Background(), result.Revision), "unproven durable operation identity cannot be acknowledged")
}

func TestCatalogRuntimeUnknownReadyState(t *testing.T) {
	preserveCatalogCandidate(t)
	db := catalogFenceTestDB(t, "sqlite")
	catalogBusinessActor(t, db)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	require.NoError(t, db.Model(&CatalogSyncState{}).Where("id = ?", CatalogSyncStateID).Update("pending_operation_id", "unknown-operation").Error)
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "any", func() error { return nil }), ErrCatalogPublicationPending)
}

// Lifecycle projection must not invoke the legacy whole-channel repair Save.
func TestCatalogRuntimeAdvancedSettingsReadOnly(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			preserveCatalogCandidate(t)
			db := catalogFenceTestDB(t, engine)
			result := runtimePendingOperation(t, db)
			runtimeAdvancedChannel(t, db, false)
			var channel Channel
			require.NoError(t, db.First(&channel).Error)
			require.NoError(t, db.Model(&channel).Updates(map[string]any{
				"settings": "{", "key": "preserve-channel-secret", "used_quota": 123,
				"other_info": `{"target":"local"}`, "balance": 12.5,
				"molii_grok_management_access_token": "preserve-management-secret",
			}).Error)
			require.NoError(t, db.First(&channel, channel.Id).Error)
			writes := 0
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("runtime-channel-write", func(tx *gorm.DB) {
				if tx.Statement.Table == "channels" {
					writes++
				}
			}))
			defer db.Callback().Update().Remove("runtime-channel-write")
			err := PublishCatalogSyncRevision(context.Background(), result.Revision)
			assert.Error(t, err, "malformed lifecycle settings must fail publication")
			var after Channel
			require.NoError(t, db.First(&after, channel.Id).Error)
			assert.Equal(t, channel, after, "publication must preserve the complete actual channel row")
			assert.Zero(t, writes, "lifecycle must not attempt a channel write")
			assertRuntimeLocksReleased(t)
			require.ErrorContains(t, err, "decode advanced custom channel settings")
			assertRuntimePending(t, db, result)
			// Ordinary callers retain their explicit legacy repair behavior.
			after.GetOtherSettings()
			require.NoError(t, db.First(&after, channel.Id).Error)
			assert.Equal(t, "{}", after.OtherSettings)
			assert.Equal(t, 1, writes)
			valid := `{"advanced_custom":{"advanced_routes":[{"incoming_path":"/v1/responses","upstream_path":"/v1/responses"}]}}`
			require.NoError(t, db.Model(&channel).Update("settings", valid).Error)
			require.NoError(t, db.First(&channel, channel.Id).Error)
			writes = 0
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			require.NoError(t, db.First(&after, channel.Id).Error)
			assert.Equal(t, channel, after)
			assert.Zero(t, writes)
			assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIResponse}, GetModelSupportEndpointTypes("managed-model"))
			require.NoError(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error { return nil }))
		})
	}
}

func TestCatalogRuntimeGroupIndexContention(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			for _, canceled := range []bool{false, true} {
				name := "busy"
				if canceled {
					name = "canceled"
				}
				t.Run(name, func(t *testing.T) {
					preserveCatalogCandidate(t)
					db := catalogFenceTestDB(t, engine)
					result := runtimePendingOperation(t, db)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					entered := make(chan struct{})
					var once sync.Once
					require.NoError(t, db.Callback().Query().After("gorm:query").Register("runtime-group-entered", func(tx *gorm.DB) {
						if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == db.Statement.ConnPool {
							once.Do(func() {
								if canceled {
									cancel()
								}
								close(entered)
							})
						}
					}))
					defer db.Callback().Query().Remove("runtime-group-entered")
					modelEnableGroupsLock.Lock()
					var release sync.Once
					defer release.Do(modelEnableGroupsLock.Unlock)
					groupsBefore, quotaBefore := maps.Clone(modelEnableGroups), maps.Clone(modelQuotaTypeMap)
					done := make(chan error, 1)
					go func() { done <- PublishCatalogSyncRevision(ctx, result.Revision) }()
					select {
					case <-entered:
					case <-time.After(5 * time.Second):
						t.Fatal("publication did not reach required rebuild")
					}
					var err error
					select {
					case err = <-done:
					case <-time.After(time.Second):
						release.Do(modelEnableGroupsLock.Unlock)
						err = <-done
						t.Errorf("publication blocked on held group lock: %v", err)
						return
					}
					if canceled {
						require.ErrorIs(t, err, context.Canceled)
					} else {
						require.ErrorIs(t, err, ErrCatalogWriterBusy)
					}
					assert.Equal(t, groupsBefore, modelEnableGroups)
					assert.Equal(t, quotaBefore, modelQuotaTypeMap)
					assertRuntimeLocksReleased(t)
					assertRuntimePending(t, db, result)
					release.Do(modelEnableGroupsLock.Unlock)
					require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
					assertRuntimeLocksReleased(t)
					require.True(t, modelEnableGroupsLock.TryLock(), "retry leaked group lock")
					modelEnableGroupsLock.Unlock()
					var state CatalogSyncState
					require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
					assert.Equal(t, result.Revision, state.Revision)
					assert.Equal(t, result.Revision, state.RuntimeRevision)
					assert.Equal(t, "ready", state.PublicationState)
					require.NoError(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error { return nil }))
				})
			}
		})
	}
}
