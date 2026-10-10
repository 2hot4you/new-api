package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func catalogAliasRemovalFixture(t *testing.T) (*gorm.DB, Model, Channel) {
	t.Helper()
	require.NotEmpty(t, os.Getenv("TEST_POSTGRES_DSN"), "requires task PostgreSQL")
	db := catalogFenceTestDB(t, "postgres")
	initializeOrdinaryCatalogTest(t, db)
	priorAlias := taskAliasViewPtr.Load()
	priorMemory := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { taskAliasViewPtr.Store(priorAlias); common.MemoryCacheEnabled = priorMemory })
	catalogValidationFixture(t, db, jsplugin.DefaultRegistry)
	require.NoError(t, db.Create(&Option{Key: "billing_setting.billing_mode", Value: `{}`}).Error)
	alias := Model{ModelName: "ordinary-alias", BillingCurrency: "USD"}
	require.NoError(t, db.Create(&alias).Error)
	mapping := `{"ordinary-alias":"hop","hop":"pricing-usage-model","kept":"pricing-usage-model"}`
	channel := Channel{Key: "test", Name: "removal", Models: "ordinary-alias,pricing-usage-model", ModelMapping: &mapping, Status: common.ChannelStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(db))
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	return db, alias, channel
}

func TestCatalogAliasRemovalMultipleChannels(t *testing.T) {
	db, alias, first := catalogAliasRemovalFixture(t)
	kept := Model{ModelName: "kept", BillingCurrency: "USD"}
	require.NoError(t, db.Create(&kept).Error)
	priority, weight, tag := int64(7), uint(9), "routing-tag"
	second := Channel{Key: "second", Models: "ordinary-alias,kept", ModelMapping: first.ModelMapping, Status: common.ChannelStatusEnabled, Group: "default,vip", Priority: &priority, Weight: &weight, Tag: &tag}
	disabled := Channel{Key: "disabled", Models: " ordinary-alias ,kept", ModelMapping: first.ModelMapping, Status: common.ChannelStatusManuallyDisabled, Group: "default"}
	untouched := Channel{Key: "unrelated", Models: ",kept,", ModelMapping: first.ModelMapping, Status: common.ChannelStatusEnabled, Group: "default"}
	for _, channel := range []*Channel{&second, &disabled, &untouched} {
		require.NoError(t, db.Create(channel).Error)
		require.NoError(t, channel.UpdateAbilities(db))
	}
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	before := runtimeStageForTest(t, db).input
	result, err := DeleteModelMetadata([]int{alias.Id, alias.Id}, true, false)
	require.NoError(t, err)
	assert.Equal(t, ModelDeleteResult{DeletedCount: 1, UpdatedChannels: 3}, result)
	for _, fixture := range []struct {
		channel Channel
		models  string
	}{{first, "pricing-usage-model"}, {second, "kept"}, {disabled, "kept"}, {untouched, ",kept,"}} {
		var actual Channel
		require.NoError(t, db.First(&actual, fixture.channel.Id).Error)
		assert.Equal(t, fixture.models, actual.Models)
		fixture.channel.Models = fixture.models
		assert.Equal(t, fixture.channel, actual, "only Models may change")
	}
	var abilities []Ability
	require.NoError(t, db.Where("channel_id = ?", second.Id).Order(commonGroupCol).Find(&abilities).Error)
	require.Len(t, abilities, 2)
	for _, ability := range abilities {
		assert.Equal(t, "kept", ability.Model)
		assert.True(t, ability.Enabled)
		assert.Equal(t, priority, *ability.Priority)
		assert.Equal(t, weight, ability.Weight)
		assert.Equal(t, tag, *ability.Tag)
	}
	var removed int64
	require.NoError(t, db.Model(&Ability{}).Where("model = ?", alias.ModelName).Count(&removed).Error)
	assert.Zero(t, removed)
	require.NoError(t, db.Model(&Model{}).Where("id = ?", alias.Id).Count(&removed).Error)
	assert.Zero(t, removed)
	require.NoError(t, db.First(&kept, kept.Id).Error)
	after := runtimeStageForTest(t, db).input
	assert.Equal(t, before.state.Revision+1, after.state.Revision)
	assert.Equal(t, after.state.Revision, after.state.RuntimeRevision)
	assert.Equal(t, "ready", after.state.PublicationState)
	deps, err := catalogRuntimeDependencies(after)
	require.NoError(t, err)
	assert.NotContains(t, deps.Aliases, "ordinary-alias")
	assert.Equal(t, "pricing-usage-model", deps.Aliases["kept"].Declared)
	_, exists := ResolveTaskModelAlias(jsplugin.DefaultRegistry.Generation(), alias.ModelName)
	assert.False(t, exists)
	assert.Equal(t, before.options, after.options, "deleting metadata never grants pricing deletion")
	result, err = DeleteModelMetadata([]int{alias.Id}, true, false)
	require.ErrorContains(t, err, "selected models changed")
	assert.Equal(t, ModelDeleteResult{}, result)
	assert.Equal(t, after.state, runtimeStageForTest(t, db).input.state, "rejected repeat must not advance revision")
}

func TestCatalogAliasRemovalFreshProof(t *testing.T) {
	for _, drift := range []string{"identity", "unrelated-model", "channel-models", "mapping", "disabled-channel", "routing-fields", "new-channel", "deleted-channel", "price", "plugin", "generation"} {
		t.Run(drift, func(t *testing.T) {
			db, alias, channel := catalogAliasRemovalFixture(t)
			prepared, err := prepareCatalogModelRemoval(context.Background(), db, []int{alias.Id})
			require.NoError(t, err)
			switch drift {
			case "identity":
				require.NoError(t, db.Model(&Model{}).Where("id = ?", alias.Id).Update("model_name", "renamed").Error)
			case "unrelated-model":
				require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "pricing-usage-model").Update("description", "changed").Error)
			case "channel-models":
				require.NoError(t, db.Model(&channel).Update("models", "ordinary-alias,pricing-usage-model,unrelated").Error)
			case "mapping":
				require.NoError(t, db.Model(&channel).Update("model_mapping", `{"ordinary-alias":"pricing-usage-model","unexposed":"other"}`).Error)
			case "disabled-channel":
				require.NoError(t, db.Create(&Channel{Key: "disabled", Models: "ordinary-alias", Status: common.ChannelStatusManuallyDisabled}).Error)
			case "routing-fields":
				require.NoError(t, db.Model(&channel).Updates(map[string]any{"group": "vip", "priority": 7, "weight": 9, "tag": "new", "type": 2}).Error)
			case "new-channel":
				require.NoError(t, db.Create(&Channel{Key: "new", Models: "pricing-usage-model", Status: common.ChannelStatusEnabled}).Error)
			case "deleted-channel":
				require.NoError(t, db.Delete(&channel).Error)
			case "price":
				require.NoError(t, db.Create(&Option{Key: "ModelPrice", Value: `{"unrelated":1}`}).Error)
			case "plugin":
				require.NoError(t, db.Model(&TaskPlugin{}).Where("active = ?", true).Update("version", "2.0.0").Error)
			case "generation":
				_, err := jsplugin.DefaultRegistry.RegisterFactory(strings.ReplaceAll(pricingUsagePluginSource("1.0.0", `{seconds:{type:"number",unit:"second"}}`), "pricing-usage", "new-generation"), jsplugin.Options{})
				require.NoError(t, err)
			}
			calls := 0
			err = WithCatalogWriteBarrier(func() error {
				return metadataTransactionGuarded(prepared, func(tx *gorm.DB) error { calls++; return tx.Delete(&Model{}, alias.Id).Error })
			})
			require.Error(t, err)
			assert.Zero(t, calls, "unprepared drift must reject before the single SQL callback")
			assert.False(t, prepared.wrote)
			var count int64
			require.NoError(t, db.Model(&Model{}).Where("id = ?", alias.Id).Count(&count).Error)
			assert.Equal(t, int64(1), count)
		})
	}
}

func TestCatalogAliasRemovalPostProofAndGuards(t *testing.T) {
	db, alias, channel := catalogAliasRemovalFixture(t)
	before := runtimeStageForTest(t, db).input
	for _, mode := range []string{"generic", "missing-channel-write", "unprepared-channel-write"} {
		t.Run(mode, func(t *testing.T) {
			prepared, err := prepareCatalogModelRemoval(context.Background(), db, []int{alias.Id})
			if mode == "generic" {
				prepared, err = prepareOrdinaryCatalogMutation(context.Background(), db, nil)
			}
			require.NoError(t, err)
			calls := 0
			err = WithCatalogWriteBarrier(func() error {
				return metadataTransactionGuarded(prepared, func(tx *gorm.DB) error {
					calls++
					if err := tx.Delete(&Model{}, alias.Id).Error; err != nil {
						return err
					}
					if mode == "missing-channel-write" {
						return nil
					}
					models := "pricing-usage-model"
					if mode == "unprepared-channel-write" {
						models = ""
					}
					return tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("models", models).Error
				})
			})
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			assert.Equal(t, 1, calls)
			assert.True(t, before.same(runtimeStageForTest(t, db).input), "rejected postimage rolls back")
		})
	}
	for _, ids := range [][]int{nil, {0}, {-1}, make([]int, 1001)} {
		result, err := DeleteModelMetadata(ids, true, false)
		require.Error(t, err)
		assert.Equal(t, ModelDeleteResult{}, result)
	}
	_, err := DeleteModelMetadata([]int{alias.Id}, true, true)
	require.ErrorContains(t, err, "pricing removal is not supported")
	require.NoError(t, db.Model(&Model{}).Where("id = ?", alias.Id).Update("name_rule", NameRulePrefix).Error)
	_, err = DeleteModelMetadata([]int{alias.Id}, true, false)
	require.ErrorContains(t, err, "only exact-match")
	result, err := DeleteModelMetadata([]int{alias.Id}, false, false)
	require.NoError(t, err, "metadata-only deletion still permits nonexact records")
	assert.Equal(t, ModelDeleteResult{DeletedCount: 1}, result)
	require.NoError(t, db.First(&channel, channel.Id).Error)
	assert.Equal(t, "ordinary-alias,pricing-usage-model", channel.Models)
}

func TestCatalogAliasRemovalPendingRecovery(t *testing.T) {
	db, alias, channel := catalogAliasRemovalFixture(t)
	_, existed := ResolveTaskModelAlias(jsplugin.DefaultRegistry.Generation(), alias.ModelName)
	require.True(t, existed, "prewarm the actual alias cache before the committed deletion")
	var frozen string
	require.NoError(t, WithCatalogPricingRead(context.Background(), alias.ModelName, func() error { frozen, _ = billing_setting.GetBillingExpr("pricing-usage-model"); return nil }))
	before := runtimeStageForTest(t, db).input.state
	updatePricingLock.Lock()
	result, err := DeleteModelMetadata([]int{alias.Id}, true, false)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	assert.Equal(t, ModelDeleteResult{}, result, "error results remain empty, including known committed failure")
	var pending CatalogSyncState
	require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
	assert.Equal(t, before.Revision+1, pending.Revision)
	assert.Equal(t, "committed_pending_publish", pending.PublicationState)
	assert.Empty(t, pending.PendingOperationID)
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), alias.ModelName, func() error { t.Error("pending reader escaped"); return nil }), ErrCatalogPublicationPending)
	_, err = DeleteModelMetadata([]int{alias.Id}, true, false)
	require.ErrorIs(t, err, ErrCatalogPublicationPending)
	assert.Equal(t, `u("seconds") * 0.4`, frozen, "a captured request keeps its existing inputs")
	require.NoError(t, db.First(&channel, channel.Id).Error)
	assert.Equal(t, "pricing-usage-model", channel.Models)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	var ready CatalogSyncState
	require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
	assert.Equal(t, pending.Revision, ready.Revision)
	assert.Equal(t, ready.Revision, ready.RuntimeRevision)
	assert.Equal(t, "ready", ready.PublicationState)
	_, exists := ResolveTaskModelAlias(jsplugin.DefaultRegistry.Generation(), alias.ModelName)
	assert.False(t, exists, "acknowledged recovery must not serve the old alias until TTL expiry")
	assertRuntimeLocksReleased(t)
}

func TestCatalogAliasRemovalPublicationBoundary(t *testing.T) {
	db, alias, _ := catalogAliasRemovalFixture(t)
	deletes, rebuilds, pending, acks := 0, 0, 0, 0
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("alias-delete-count", func(tx *gorm.DB) {
		if tx.Statement.Table == "models" {
			deletes++
		}
	}))
	defer db.Callback().Delete().Remove("alias-delete-count")
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("alias-state-boundary", func(tx *gorm.DB) {
		if tx.Statement.Table != "catalog_sync_states" {
			return
		}
		values, ok := tx.Statement.Dest.(map[string]any)
		if !ok {
			return
		}
		if values["publication_state"] == "committed_pending_publish" {
			pending++
		}
		if values["publication_state"] == "ready" {
			acks++
		}
		if values["publication_state"] != nil {
			assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), alias.ModelName, func() error { t.Error("reader escaped writer"); return nil }), ErrCatalogWriterBusy)
		}
	}))
	defer db.Callback().Update().Remove("alias-state-boundary")
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("alias-rebuild-boundary", func(tx *gorm.DB) {
		if tx.Statement.Table != "vendors" || tx.Statement.ConnPool != db.Statement.ConnPool || acks != 0 {
			return
		}
		rebuilds++
		assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), alias.ModelName, func() error { t.Error("reader escaped rebuild"); return nil }), ErrCatalogWriterBusy)
		done := make(chan struct{})
		go func() { jsplugin.DefaultRegistry.SetEnabled(true); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("registry pin retained during rebuild")
		}
	}))
	result, err := DeleteModelMetadata([]int{alias.Id}, true, false)
	require.NoError(t, db.Callback().Query().Remove("alias-rebuild-boundary"))
	require.NoError(t, err)
	assert.Equal(t, ModelDeleteResult{DeletedCount: 1, UpdatedChannels: 1}, result)
	assert.Equal(t, 1, deletes)
	assert.Equal(t, 1, pending)
	assert.Equal(t, 1, acks)
	assert.Positive(t, rebuilds)
}

func TestCatalogAliasRemovalNativeAcknowledgements(t *testing.T) {
	for _, lost := range []string{"committed_pending_publish", "ready"} {
		t.Run(lost, func(t *testing.T) {
			db, alias, _ := catalogAliasRemovalFixture(t)
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
				deletes := 0
				require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("alias-native-delete", func(tx *gorm.DB) {
					if tx.Statement.Table == "models" {
						deletes++
					}
				}))
				defer db.Callback().Delete().Remove("alias-native-delete")
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("alias-native-loss", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_states" {
						if values, ok := tx.Statement.Dest.(map[string]any); ok && values["publication_state"] == lost {
							connection.armed.Store(true)
						}
					}
				}))
				result, err := DeleteModelMetadata([]int{alias.Id}, true, false)
				require.NoError(t, db.Callback().Update().Remove("alias-native-loss"))
				if lost == "committed_pending_publish" {
					require.ErrorIs(t, err, ErrCatalogCommitUncertain)
					assert.Equal(t, ModelDeleteResult{}, result)
				} else {
					require.NoError(t, err)
					assert.Equal(t, 1, result.DeletedCount)
				}
				assert.Equal(t, 1, deletes)
				assert.Equal(t, int64(1), connection.dropped.Load())
				var state CatalogSyncState
				require.NoError(t, root.First(&state, CatalogSyncStateID).Error)
				if lost == "committed_pending_publish" {
					assert.Equal(t, lost, state.PublicationState)
					require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
				}
				var ready CatalogSyncState
				require.NoError(t, root.First(&ready, CatalogSyncStateID).Error)
				assert.Equal(t, state.Revision, ready.Revision)
				assert.Equal(t, ready.Revision, ready.RuntimeRevision)
				assert.Equal(t, "ready", ready.PublicationState)
				return nil
			}))
		})
	}
}

func TestCatalogAliasRemovalSQLRollback(t *testing.T) {
	db, alias, channel := catalogAliasRemovalFixture(t)
	before := runtimeStageForTest(t, db).input
	injected := errors.New("delete failed after channel mutation")
	deletes := 0
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("alias-delete-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "models" {
			deletes++
			tx.AddError(injected)
		}
	}))
	result, err := DeleteModelMetadata([]int{alias.Id}, true, false)
	require.NoError(t, db.Callback().Delete().Remove("alias-delete-failure"))
	require.ErrorIs(t, err, injected)
	assert.Equal(t, ModelDeleteResult{}, result)
	assert.Equal(t, 1, deletes)
	assert.True(t, before.same(runtimeStageForTest(t, db).input))
	require.NoError(t, db.First(&channel, channel.Id).Error)
	assert.Equal(t, "ordinary-alias,pricing-usage-model", channel.Models)
	var count int64
	require.NoError(t, db.Model(&Ability{}).Where("channel_id = ? AND model = ?", channel.Id, alias.ModelName).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCatalogAliasRemovalFoldedSurvivor(t *testing.T) {
	db, alias, _ := catalogAliasRemovalFixture(t)
	upper := Model{ModelName: "ORDINARY-ALIAS", BillingCurrency: "USD"}
	require.NoError(t, db.Create(&upper).Error)
	mapping := `{"ORDINARY-ALIAS":"pricing-usage-model"}`
	channel := Channel{Key: "folded", Models: upper.ModelName, ModelMapping: &mapping, Status: common.ChannelStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, channel.UpdateAbilities(db))
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	result, err := DeleteModelMetadata([]int{alias.Id}, true, false)
	require.NoError(t, err)
	assert.Equal(t, ModelDeleteResult{DeletedCount: 1, UpdatedChannels: 1}, result)
	deps, err := catalogRuntimeDependencies(runtimeStageForTest(t, db).input)
	require.NoError(t, err)
	assert.Equal(t, TaskAliasTarget{Alias: "ORDINARY-ALIAS", Declared: "pricing-usage-model", PluginKey: "pricing-usage-probe"}, deps.Aliases["ordinary-alias"])
	require.NoError(t, db.First(&channel, channel.Id).Error)
	assert.Equal(t, "ORDINARY-ALIAS", channel.Models, "explicit removal is case-sensitive while alias resolution folds ASCII")
}

func TestCatalogAliasRemovalAliasCacheContention(t *testing.T) {
	db, alias, _ := catalogAliasRemovalFixture(t)
	taskAliasRebuildMu.Lock()
	_, err := DeleteModelMetadata([]int{alias.Id}, true, false)
	taskAliasRebuildMu.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	var pending CatalogSyncState
	require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
	assert.Equal(t, "committed_pending_publish", pending.PublicationState)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	var ready CatalogSyncState
	require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
	assert.Equal(t, pending.Revision, ready.Revision)
	assert.Equal(t, "ready", ready.PublicationState)
	_, exists := ResolveTaskModelAlias(jsplugin.DefaultRegistry.Generation(), alias.ModelName)
	assert.False(t, exists)
}

func enableCatalogAliasMemoryCache(t *testing.T) {
	t.Helper()
	channelSyncLock.Lock()
	priorChannels, priorGroups, priorAdvanced := channelsIDM, group2model2channels, channel2advancedCustomConfig
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		channelsIDM, group2model2channels, channel2advancedCustomConfig = priorChannels, priorGroups, priorAdvanced
		channelSyncLock.Unlock()
	})
	common.MemoryCacheEnabled = true
	InitChannelCache()
}

func TestCatalogAliasRemovalMemoryRecovery(t *testing.T) {
	db, alias, channel := catalogAliasRemovalFixture(t)
	enableCatalogAliasMemoryCache(t)
	before, err := GetRandomSatisfiedChannel("default", alias.ModelName, 0, nil)
	require.NoError(t, err)
	require.NotNil(t, before)
	assert.Equal(t, channel.Id, before.Id)
	updatePricingLock.Lock()
	_, err = DeleteModelMetadata([]int{alias.Id}, true, false)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	var pending CatalogSyncState
	require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
	assert.Equal(t, "committed_pending_publish", pending.PublicationState)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	after, err := GetRandomSatisfiedChannel("default", alias.ModelName, 0, nil)
	require.NoError(t, err)
	assert.Nil(t, after, "ready recovery must remove deleted models from the actual memory routing index")
	cached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.Equal(t, "pricing-usage-model", cached.Models)
	assert.Equal(t, "ordinary-alias,pricing-usage-model", before.Models, "previously selected channel remains frozen")
	var ready CatalogSyncState
	require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
	assert.Equal(t, pending.Revision, ready.Revision)
	assert.Equal(t, ready.Revision, ready.RuntimeRevision)
}

func TestCatalogAliasRemovalMemoryProjection(t *testing.T) {
	db, alias, channel := catalogAliasRemovalFixture(t)
	info := ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling}
	valid := `{"advanced_custom":{"advanced_routes":[{"incoming_path":"/v1/responses","upstream_path":"/v1/responses"}]}}`
	require.NoError(t, db.Model(&channel).Updates(map[string]any{"type": constant.ChannelTypeAdvancedCustom, "settings": valid, "key": "first\nsecond", "channel_info": info}).Error)
	enableCatalogAliasMemoryCache(t)
	channelSyncLock.Lock()
	channelsIDM[channel.Id].ChannelInfo.MultiKeyPollingIndex = 1
	channelSyncLock.Unlock()
	writes := 0
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("alias-memory-channel-writes", func(tx *gorm.DB) {
		if tx.Statement.Table == "channels" {
			writes++
		}
	}))
	defer db.Callback().Update().Remove("alias-memory-channel-writes")
	updatePricingLock.Lock()
	_, err := DeleteModelMetadata([]int{alias.Id}, true, false)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	assert.Equal(t, 1, writes, "only the explicit removal callback may update channels")
	cached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.Equal(t, 1, cached.ChannelInfo.MultiKeyPollingIndex)
	assert.Equal(t, []string{"first", "second"}, cached.Keys)
	assert.Equal(t, "pricing-usage-model", cached.Models)
	channelSyncLock.RLock()
	config := channel2advancedCustomConfig[channel.Id]
	channelSyncLock.RUnlock()
	require.NotNil(t, config)
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAIResponse}, GetModelSupportEndpointTypes("pricing-usage-model"))
	require.NoError(t, db.First(&channel, channel.Id).Error)
	assert.Equal(t, 0, channel.ChannelInfo.MultiKeyPollingIndex, "runtime polling state must not become a channel SQL write")
	assert.Equal(t, valid, channel.OtherSettings)
}

func TestCatalogAliasRemovalMemoryFailure(t *testing.T) {
	for _, failure := range []string{"cache-lock", "malformed-settings"} {
		t.Run(failure, func(t *testing.T) {
			db, alias, channel := catalogAliasRemovalFixture(t)
			enableCatalogAliasMemoryCache(t)
			if failure == "cache-lock" {
				channelSyncLock.Lock()
			} else {
				require.NoError(t, db.Model(&channel).Updates(map[string]any{"type": constant.ChannelTypeAdvancedCustom, "settings": "{"}).Error)
			}
			result, err := DeleteModelMetadata([]int{alias.Id}, true, false)
			if failure == "cache-lock" {
				channelSyncLock.Unlock()
				require.ErrorIs(t, err, ErrCatalogWriterBusy)
			} else {
				require.ErrorContains(t, err, "decode advanced custom channel settings")
				require.NoError(t, db.First(&channel, channel.Id).Error)
				assert.Equal(t, "{", channel.OtherSettings, "lifecycle never repairs or saves malformed channel settings")
				require.NoError(t, db.Model(&channel).Update("settings", "{}").Error)
			}
			assert.Equal(t, ModelDeleteResult{}, result)
			var pending CatalogSyncState
			require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
			assert.Equal(t, "committed_pending_publish", pending.PublicationState)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			var ready CatalogSyncState
			require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
			assert.Equal(t, pending.Revision, ready.Revision)
			assert.Equal(t, "ready", ready.PublicationState)
			route, err := GetRandomSatisfiedChannel("default", alias.ModelName, 0, nil)
			require.NoError(t, err)
			assert.Nil(t, route)
		})
	}
}

func TestCatalogAliasRemovalOrdinaryRefreshOverlap(t *testing.T) {
	db, alias, channel := catalogAliasRemovalFixture(t)
	enableCatalogAliasMemoryCache(t)
	captured, release := make(chan struct{}), make(chan struct{})
	var once atomic.Bool
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("alias-ordinary-refresh-paused", func(tx *gorm.DB) {
		if tx.Statement.Table == "channels" && tx.Statement.ConnPool == db.Statement.ConnPool && once.CompareAndSwap(false, true) {
			close(captured)
			<-release
		}
	}))
	ordinaryDone := make(chan struct{})
	go func() { InitChannelCache(); close(ordinaryDone) }()
	select {
	case <-captured:
	case <-time.After(5 * time.Second):
		t.Fatal("ordinary refresh did not capture SQL")
	}
	// A real ordinary snapshot has been read. The boundary must already be
	// held now, not merely when the maps are eventually installed.
	acquired := catalogBarrier.TryLock()
	if acquired {
		catalogBarrier.Unlock()
	}
	assert.False(t, acquired, "capture-through-publication read boundary is required to prevent delayed old snapshots")
	deleted := make(chan error, 1)
	go func() { _, err := DeleteModelMetadata([]int{alias.Id}, true, false); deleted <- err }()
	close(release)
	require.NoError(t, <-deleted)
	<-ordinaryDone
	require.NoError(t, db.Callback().Query().Remove("alias-ordinary-refresh-paused"))
	route, err := GetRandomSatisfiedChannel("default", alias.ModelName, 0, nil)
	require.NoError(t, err)
	assert.Nil(t, route)
	cached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.Equal(t, "pricing-usage-model", cached.Models)
}

func TestCatalogAliasRemovalMemorySQLCancellation(t *testing.T) {
	for _, table := range []string{"channels", "abilities"} {
		t.Run(table, func(t *testing.T) {
			db, _, _ := catalogAliasRemovalFixture(t)
			enableCatalogAliasMemoryCache(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan *gorm.DB, 1)
			observed := make(chan error, 1)
			var armed atomic.Bool
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("alias-memory-sql-block", func(tx *gorm.DB) {
				if tx.Statement.Table != table || tx.Statement.ConnPool != db.Statement.ConnPool || !armed.CompareAndSwap(false, true) {
					return
				}
				blocker := db.Begin()
				tx.AddError(blocker.Exec("LOCK TABLE " + table + " IN ACCESS EXCLUSIVE MODE").Error)
				tx.Statement.Settings.Store("alias-memory-observed", true)
				started <- blocker
			}))
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("alias-memory-sql-result", func(tx *gorm.DB) {
				if _, ok := tx.Statement.Settings.Load("alias-memory-observed"); ok {
					observed <- tx.Error
				}
			}))
			finished := make(chan error, 1)
			go func() { finished <- RecoverCatalogSyncRuntime(ctx) }()
			var blocker *gorm.DB
			select {
			case blocker = <-started:
			case err := <-finished:
				t.Fatalf("recovery missed required SQL: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("cache SQL not reached")
			}
			defer blocker.Rollback()
			catalogWaitForDBLock(t, db, "postgres", finished)
			cancel()
			select {
			case err := <-observed:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(2 * time.Second):
				_ = blocker.Rollback().Error
				t.Fatal("actual cache query did not honor publisher cancellation")
			}
			require.ErrorIs(t, <-finished, context.Canceled)
			require.NoError(t, blocker.Rollback().Error)
			require.NoError(t, db.Callback().Query().Remove("alias-memory-sql-block"))
			require.NoError(t, db.Callback().Query().Remove("alias-memory-sql-result"))
			var pending CatalogSyncState
			require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
			assert.Equal(t, "committed_pending_publish", pending.PublicationState)
			assertRuntimeLocksReleased(t)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			var ready CatalogSyncState
			require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
			assert.Equal(t, pending.Revision, ready.Revision)
			assert.Equal(t, "ready", ready.PublicationState)
		})
	}
}

func TestCatalogAliasAggregationDetached(t *testing.T) {
	registry := jsplugin.NewRegistry()
	source := pricingUsagePluginSource("1.0.0", `{seconds:{type:"number",unit:"second"}}`)
	source = strings.Replace(source, `["pricing-usage-model"]`, `["pricing-usage-model","other-tail"]`, 1)
	_, err := registry.RegisterFactory(source, jsplugin.Options{})
	require.NoError(t, err)
	_, err = registry.RegisterFactory(strings.ReplaceAll(pricingUsagePluginSource("1.0.0", `{seconds:{type:"number",unit:"second"}}`), "pricing-usage", "other-plugin"), jsplugin.Options{})
	require.NoError(t, err)
	channel := func(models, mapping string) Channel { return Channel{Models: models, ModelMapping: &mapping} }
	for _, test := range []struct {
		name     string
		channels []Channel
		want     map[string]TaskAliasTarget
		err      string
	}{
		{"chain-and-fold", []Channel{channel("Alias", `{"Alias":"hop","hop":"PRICING-USAGE-MODEL"}`), channel("alias", `{"alias":"pricing-usage-model"}`)}, map[string]TaskAliasTarget{"alias": {Alias: "Alias", Declared: "pricing-usage-model", PluginKey: "pricing-usage-probe"}}, ""},
		{"canonical-precedence", []Channel{channel("pricing-usage-model", `{"pricing-usage-model":"other-plugin-model"}`)}, map[string]TaskAliasTarget{}, ""},
		{"same-plugin-multiple-tails", []Channel{channel("alias", `{"alias":"pricing-usage-model"}`), channel("alias", `{"alias":"other-tail"}`)}, map[string]TaskAliasTarget{"alias": {Alias: "alias", PluginKey: "pricing-usage-probe"}}, ""},
		{"cross-plugin", []Channel{channel("alias", `{"alias":"pricing-usage-model"}`), channel("alias", `{"alias":"other-plugin-model"}`)}, nil, "multiple plugins"},
		{"cycle", []Channel{channel("alias", `{"alias":"hop","hop":"alias"}`)}, nil, "cycle"},
		{"duplicate", []Channel{channel("alias", `{"alias":"pricing-usage-model","alias":"other-tail"}`)}, nil, "ambiguous"},
		{"malformed", []Channel{channel("alias", `null`)}, nil, "malformed"},
		{"whitespace", []Channel{channel(" alias", `{"alias":"pricing-usage-model"}`)}, nil, "ambiguous"},
	} {
		t.Run(test.name, func(t *testing.T) {
			view, err := buildTaskAliasViewChannels(test.channels, registry.Generation(), true)
			if test.err != "" {
				require.ErrorContains(t, err, test.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, view.byFold)
		})
	}
}
