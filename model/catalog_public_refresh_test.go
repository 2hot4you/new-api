package model

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func catalogPublicRefreshFixture(t *testing.T, memory bool) (*gorm.DB, []Channel) {
	t.Helper()
	channelSyncLock.RLock()
	previousGroups, previousChannels, previousAdvanced := group2model2channels, channelsIDM, channel2advancedCustomConfig
	channelSyncLock.RUnlock()
	previousAliases := taskAliasViewPtr.Load()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		group2model2channels, channelsIDM, channel2advancedCustomConfig = previousGroups, previousChannels, previousAdvanced
		channelSyncLock.Unlock()
		taskAliasViewPtr.Store(previousAliases)
	})
	db := catalogReloadTestDB(t)
	previousMemory := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = memory
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemory })
	require.NoError(t, db.Create(&Model{ModelName: "public-refresh", BillingCurrency: "CNY", NameRule: NameRuleExact, Status: 1}).Error)
	firstPriority, secondPriority := int64(20), int64(10)
	channels := []Channel{
		{Type: 1, Key: "first", Name: "first", Models: "public-refresh", Group: "default", Status: common.ChannelStatusEnabled, Priority: &firstPriority},
		{Type: 1, Key: "second", Name: "second", Models: "public-refresh", Group: "default", Status: common.ChannelStatusEnabled, Priority: &secondPriority},
	}
	for i := range channels {
		require.NoError(t, channels[i].Insert())
	}
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"public-refresh":3}`, "USDExchangeRate": "7"}))
	require.NoError(t, WithCatalogPricingRead(context.Background(), "public-refresh", func() error { return nil }))
	return db, channels
}

// Public channel controllers commit these mutations before calling InitChannelCache.
// Existing exact metadata makes reconciliation a genuine SQL no-op, not a repair
// that incidentally publishes the changed routing input.
func TestCatalogPublicRefreshPublishesCommittedRouting(t *testing.T) {
	for _, memory := range []bool{true, false} {
		for _, mutation := range []string{"priority", "mapping"} {
			t.Run(fmt.Sprintf("%s/memory=%t", mutation, memory), func(t *testing.T) {
				db, channels := catalogPublicRefreshFixture(t, memory)
				var before CatalogSyncState
				require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
				if mutation == "priority" {
					order, err := GetChannelPriorityOrder()
					require.NoError(t, err)
					changed, err := ReorderChannelPriority(ChannelPriorityMove{Revision: order.Revision, ID: channels[1].Id, TargetID: channels[0].Id, Position: "before"})
					require.NoError(t, err)
					require.NotEmpty(t, changed.Changes)
				} else {
					mapping := `{"public-refresh":"mapped-upstream"}`
					channels[0].ModelMapping = &mapping
					require.NoError(t, channels[0].Update())
				}
				var committed CatalogSyncState
				require.NoError(t, db.First(&committed, CatalogSyncStateID).Error)
				require.Equal(t, before, committed, "raw routing mutations retain the catalog revision contract")
				businessWrites := 0
				observeWrite := func(tx *gorm.DB) {
					if tx.Statement.Table != "catalog_sync_states" {
						businessWrites++
					}
				}
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("public-refresh-writes", observeWrite))
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("public-refresh-writes", observeWrite))
				require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("public-refresh-writes", observeWrite))
				defer db.Callback().Create().Remove("public-refresh-writes")
				defer db.Callback().Update().Remove("public-refresh-writes")
				defer db.Callback().Delete().Remove("public-refresh-writes")

				InitChannelCache()

				var after CatalogSyncState
				require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
				assert.Equal(t, "ready", after.PublicationState)
				assert.Equal(t, before.Revision, after.Revision)
				assert.Equal(t, after.Revision, after.RuntimeRevision)
				assert.Empty(t, after.PendingOperationID)
				assert.NotEmpty(t, after.RuntimeFingerprint)
				assert.NotEqual(t, before.RuntimeFingerprint, after.RuntimeFingerprint)
				assert.Zero(t, businessWrites, "refresh must not replay the committed mutation or write unchanged metadata/options")
				require.NoError(t, WithCatalogPricingRead(context.Background(), "public-refresh", func() error {
					money, found, err := ResolvePublishedCatalogMoneyContext("public-refresh")
					require.NoError(t, err)
					assert.True(t, found)
					assert.Equal(t, "CNY", string(money.SourceCurrency))
					assert.Equal(t, float64(7), money.CNYPerUSD)
					assert.Equal(t, float64(7), operation_setting.USDExchangeRate)
					assert.Equal(t, float64(3), ratio_setting.GetModelPriceMap()["public-refresh"])
					assert.Equal(t, after.RuntimeFingerprint, catalogRuntime.input.fingerprint)
					selected, err := GetRandomSatisfiedChannel("default", "public-refresh", 0, nil)
					require.NoError(t, err)
					require.NotNil(t, selected)
					if mutation == "priority" {
						assert.Equal(t, channels[1].Id, selected.Id)
					} else {
						assert.Equal(t, channels[0].Id, selected.Id)
						assert.JSONEq(t, `{"public-refresh":"mapped-upstream"}`, selected.GetModelMapping())
					}
					return nil
				}))
			})
		}
	}
}

func TestCatalogPublicRefreshUnchangedKeepsAcknowledgedEpoch(t *testing.T) {
	for _, memory := range []bool{true, false} {
		t.Run(fmt.Sprintf("memory=%t", memory), func(t *testing.T) {
			db, _ := catalogPublicRefreshFixture(t, memory)
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			writes := 0
			observeWrite := func(*gorm.DB) { writes++ }
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("public-refresh-readonly", observeWrite))
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("public-refresh-readonly", observeWrite))
			require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("public-refresh-readonly", observeWrite))
			defer db.Callback().Create().Remove("public-refresh-readonly")
			defer db.Callback().Update().Remove("public-refresh-readonly")
			defer db.Callback().Delete().Remove("public-refresh-readonly")
			InitChannelCache()
			var after CatalogSyncState
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Zero(t, writes, "unchanged refresh must not transiently gate shared pricing or ACK again")
			assert.Equal(t, before, after)
			require.NoError(t, WithCatalogPricingRead(context.Background(), "public-refresh", func() error { return nil }))
		})
	}
}

func TestCatalogPublicRefreshRecoveryFailureStaysClosed(t *testing.T) {
	for _, memory := range []bool{true, false} {
		t.Run(fmt.Sprintf("memory=%t", memory), func(t *testing.T) {
			db, channels := catalogPublicRefreshFixture(t, memory)
			mapping := `{"public-refresh":"unpublished-upstream"}`
			channels[0].ModelMapping = &mapping
			require.NoError(t, channels[0].Update())
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			failures := 0
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("public-refresh-failure", func(tx *gorm.DB) {
				if tx.Statement.Table == "vendors" {
					failures++
					tx.AddError(errors.New("private-recovery-detail"))
				}
			}))
			defer db.Callback().Query().Remove("public-refresh-failure")
			var output bytes.Buffer
			common.LogWriterMu.Lock()
			previousLog, previousError := gin.DefaultWriter, gin.DefaultErrorWriter
			gin.DefaultWriter, gin.DefaultErrorWriter = &output, &output
			common.LogWriterMu.Unlock()
			defer func() {
				common.LogWriterMu.Lock()
				gin.DefaultWriter, gin.DefaultErrorWriter = previousLog, previousError
				common.LogWriterMu.Unlock()
			}()

			InitChannelCache()

			assert.Positive(t, failures, "exercise an actual authoritative capture failure")
			var after CatalogSyncState
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Equal(t, "committed_pending_publish", after.PublicationState)
			assert.Equal(t, before.Revision, after.Revision)
			assert.Equal(t, before.RuntimeFingerprint, after.RuntimeFingerprint)
			assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "public-refresh", func() error {
				t.Error("failed recovery admitted a pricing read")
				return nil
			}), ErrCatalogPublicationPending)
			if memory {
				cached, err := CacheGetChannel(channels[0].Id)
				require.NoError(t, err)
				assert.Empty(t, cached.GetModelMapping(), "failed recovery must return before unrelated route-cache installation")
			}
			assert.NotContains(t, output.String(), "existing catalog will remain available")
			assert.NotContains(t, output.String(), "channels synced from database")
			assert.NotContains(t, output.String(), "private-recovery-detail", "public failure logs must not leak raw capture errors")
			assert.Contains(t, output.String(), "result=failed")
			require.NoError(t, db.Callback().Query().Remove("public-refresh-failure"))
			InitChannelCache()
			require.NoError(t, WithCatalogPricingRead(context.Background(), "public-refresh", func() error { return nil }))
		})
	}
}
