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
	result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "runtime-pending", actor)
	require.NoError(t, err)
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
			require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "managed-model", func() error {
				t.Fatal("pending input must not reach selection")
				return nil
			}), ErrCatalogPublicationPending)
			require.NoError(t, PublishCatalogSyncRevision(context.Background(), result.Revision))
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
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "scope-operation", actor)
			require.NoError(t, err)
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
