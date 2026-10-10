package model

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCatalogStartupEnrollmentRequired(t *testing.T) {
	require.NotEmpty(t, os.Getenv("TEST_POSTGRES_DSN"), "task PostgreSQL is required")
	db := catalogFenceTestDB(t, "postgres")
	actor := catalogBusinessActor(t, db)
	t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "false")
	_, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
	require.Error(t, err, "a browser-independent preview must reject absent single-instance permission")
	var plans int64
	require.NoError(t, db.Model(&CatalogSyncPlan{}).Count(&plans).Error)
	assert.Zero(t, plans)
}

func catalogStartupInstanceFixture(t *testing.T) (*gorm.DB, catalogmanifest.Actor) {
	t.Helper()
	require.NotEmpty(t, os.Getenv("TEST_POSTGRES_DSN"), "task PostgreSQL is required")
	db := catalogFenceTestDB(t, "postgres")
	preserveOrdinaryCatalogRuntime(t)
	actor := catalogBusinessActor(t, db)
	require.NoError(t, db.AutoMigrate(&SystemInstance{}))
	previous, master, started := common.GetNodeIdentity(), common.IsMasterNode, common.StartTime
	t.Cleanup(func() {
		common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = previous.Name, previous.Source, previous.ManuallyConfigured
		common.IsMasterNode, common.StartTime = master, started
	})
	common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = "catalog-test", common.NodeNameSourceManual, true
	common.IsMasterNode, common.StartTime = true, time.Now().Unix()-10
	t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "true")
	catalogStartupHeartbeat(t)
	return db, actor
}

func catalogStartupHeartbeat(t *testing.T) {
	t.Helper()
	require.NoError(t, UpsertSystemInstance(common.NodeName, map[string]any{
		"schema_version": 1, "node": common.GetNodeIdentity(), "role": map[string]any{"is_master": true},
		"runtime": map[string]any{"started_at": common.StartTime},
	}, common.StartTime, time.Now().Unix()))
}

func TestCatalogStartupHeartbeatTimestamps(t *testing.T) {
	db, actor := catalogStartupInstanceFixture(t)
	info := map[string]any{
		"schema_version": 1, "node": common.GetNodeIdentity(), "role": map[string]any{"is_master": true},
		"runtime": map[string]any{"started_at": common.StartTime},
	}
	t.Run("explicit-create-and-update", func(t *testing.T) {
		require.NoError(t, db.Where("node_name = ?", common.NodeName).Delete(&SystemInstance{}).Error)
		firstSeen := time.Now().Unix() - 2
		require.NoError(t, UpsertSystemInstance(common.NodeName, info, common.StartTime, firstSeen))
		var first SystemInstance
		require.NoError(t, db.First(&first, "node_name = ?", common.NodeName).Error)
		assert.Equal(t, firstSeen, first.CreatedAt, "first creation uses the captured heartbeat time, not a later hook clock")
		assert.Equal(t, firstSeen, first.LastSeenAt)
		assert.Equal(t, firstSeen, first.UpdatedAt)
		_, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
		require.NoError(t, err, "a delayed real first heartbeat remains eligible without weakening the strict guard")

		info["extra"] = map[string]any{"heartbeat": "updated"}
		laterSeen := firstSeen + 1
		require.NoError(t, UpsertSystemInstance(common.NodeName, info, common.StartTime, laterSeen))
		var updated SystemInstance
		require.NoError(t, db.First(&updated, "node_name = ?", common.NodeName).Error)
		assert.Equal(t, firstSeen, updated.CreatedAt, "conflict update preserves original creation identity")
		assert.Equal(t, laterSeen, updated.LastSeenAt)
		assert.Equal(t, laterSeen, updated.UpdatedAt)
		assert.Equal(t, common.StartTime, updated.StartedAt)
		assert.Contains(t, updated.Info, `"heartbeat":"updated"`)
		_, err = CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
		require.NoError(t, err)
	})
	t.Run("zero-timestamp-default", func(t *testing.T) {
		require.NoError(t, db.Where("node_name = ?", common.NodeName).Delete(&SystemInstance{}).Error)
		before := time.Now().Unix()
		require.NoError(t, UpsertSystemInstance(common.NodeName, info, common.StartTime, 0))
		after := time.Now().Unix()
		var instance SystemInstance
		require.NoError(t, db.First(&instance, "node_name = ?", common.NodeName).Error)
		assert.GreaterOrEqual(t, instance.LastSeenAt, before)
		assert.LessOrEqual(t, instance.LastSeenAt, after)
		assert.Positive(t, instance.LastSeenAt)
		assert.Equal(t, instance.LastSeenAt, instance.CreatedAt)
		assert.Equal(t, instance.LastSeenAt, instance.UpdatedAt)
		_, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
		require.NoError(t, err)
	})
}

func TestCatalogStartupEligibilityFacts(t *testing.T) {
	db, actor := catalogStartupInstanceFixture(t)
	var healthy SystemInstance
	require.NoError(t, db.First(&healthy).Error)
	for _, scenario := range []string{"healthy", "absent-permission", "invalid-permission", "hostname", "unsafe-name", "slave", "missing-self", "stale", "future", "start-mismatch", "info-mismatch", "unknown-schema", "malformed", "created-missing", "created-after-heartbeat", "updated-mismatch", "other-live", "other-future", "other-stale"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "true")
			common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = "catalog-test", common.NodeNameSourceManual, true
			common.IsMasterNode = true
			require.NoError(t, db.Where("1=1").Delete(&SystemInstance{}).Error)
			require.NoError(t, db.Create(&healthy).Error)
			switch scenario {
			case "absent-permission":
				t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "")
			case "invalid-permission":
				t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "TRUE")
			case "hostname":
				common.NodeNameSource, common.NodeNameManuallyConfigured = common.NodeNameSourceHostname, false
			case "unsafe-name":
				common.NodeName = "unsafe name"
			case "slave":
				common.IsMasterNode = false
			case "missing-self":
				require.NoError(t, db.Where("1=1").Delete(&SystemInstance{}).Error)
			case "stale":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).Update("last_seen_at", time.Now().Unix()-91).Error)
			case "future":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).Update("last_seen_at", time.Now().Unix()+60).Error)
			case "start-mismatch":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).Update("started_at", common.StartTime-1).Error)
			case "info-mismatch":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).Update("info", `{"schema_version":1,"node":{"name":"someone-else"}}`).Error)
			case "unknown-schema":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).Update("info", `{"schema_version":2}`).Error)
			case "malformed":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).Update("info", `{`).Error)
			case "created-missing":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).UpdateColumn("created_at", 0).Error)
			case "created-after-heartbeat":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).UpdateColumn("created_at", healthy.LastSeenAt+1).Error)
			case "updated-mismatch":
				require.NoError(t, db.Model(&SystemInstance{}).Where("node_name = ?", healthy.NodeName).Update("updated_at", 0).Error)
			case "other-live", "other-future", "other-stale":
				seen := time.Now().Unix()
				if scenario == "other-future" {
					seen += 60
				}
				if scenario == "other-stale" {
					seen -= 91
				}
				require.NoError(t, db.Create(&SystemInstance{NodeName: "other", LastSeenAt: seen}).Error)
			}
			if scenario == "created-missing" || scenario == "created-after-heartbeat" {
				var corrupted SystemInstance
				require.NoError(t, db.First(&corrupted, "node_name = ?", healthy.NodeName).Error)
				expected := healthy
				expected.CreatedAt = 0
				if scenario == "created-after-heartbeat" {
					expected.CreatedAt = healthy.LastSeenAt + 1
				}
				require.Equal(t, expected, corrupted, "creation-time corruption must preserve every other eligibility fact, including heartbeat/update timestamps")
			}
			_, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			if scenario == "healthy" || scenario == "other-stale" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrCatalogInstanceIneligible)
			}
		})
	}
}

func TestCatalogStartupApplyRechecksHeartbeat(t *testing.T) {
	db, actor := catalogStartupInstanceFixture(t)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
	require.NoError(t, err)
	require.NoError(t, db.Create(&SystemInstance{NodeName: "new-node", LastSeenAt: time.Now().Unix()}).Error)
	_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "must-not-write", actor)
	require.ErrorIs(t, err, ErrCatalogInstanceIneligible)
	var count int64
	require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, db.Where("node_name = ?", "new-node").Delete(&SystemInstance{}).Error)
	_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "healthy-write", actor)
	require.NoError(t, err)
	var operation CatalogSyncOperation
	require.NoError(t, db.First(&operation, "id = ?", "healthy-write").Error)
	assert.Equal(t, "succeeded", operation.State)
}

func TestCatalogStartupHeartbeatLock(t *testing.T) {
	db, _ := catalogStartupInstanceFixture(t)
	finished := make(chan error, 1)
	err := catalogReferenceTransaction(context.Background(), db, jsplugin.DefaultRegistry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
		if err := catalogSensitiveInstanceTx(tx); err != nil {
			return err
		}
		go func() { finished <- UpsertSystemInstance("new-node", nil, time.Now().Unix(), time.Now().Unix()) }()
		catalogWaitForDBLock(t, db, "postgres", finished)
		return nil
	})
	require.NoError(t, err)
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("heartbeat remained blocked after sensitive commit")
	}
	require.ErrorIs(t, catalogReferenceTransaction(context.Background(), db, jsplugin.DefaultRegistry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
		return catalogSensitiveInstanceTx(tx)
	}), ErrCatalogInstanceIneligible)
}

func TestCatalogStartupPendingEligibility(t *testing.T) {
	db, actor := catalogStartupInstanceFixture(t)
	plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
	require.NoError(t, err)
	updatePricingLock.Lock()
	result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "pending-eligibility", actor)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "false")
	require.ErrorIs(t, RecoverCatalogSyncRuntime(context.Background()), ErrCatalogInstanceIneligible)
	var state CatalogSyncState
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Equal(t, "pending-eligibility", state.PendingOperationID)
	t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "true")
	blocker := db.Begin()
	require.NoError(t, blocker.Exec("LOCK TABLE system_instances IN ACCESS EXCLUSIVE MODE").Error)
	defer blocker.Rollback()
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() { finished <- RecoverCatalogSyncRuntime(ctx) }()
	catalogWaitForDBLock(t, db, "postgres", finished)
	cancel()
	select {
	case err := <-finished:
		require.True(t, errors.Is(err, context.Canceled), "%v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("eligibility SQL did not cancel while blocker held")
	}
	require.NoError(t, blocker.Rollback().Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Equal(t, result.Revision, state.RuntimeRevision)
	assert.Equal(t, "ready", state.PublicationState)
}

func TestCatalogStartupFinalAckEligibility(t *testing.T) {
	db, actor := catalogStartupInstanceFixture(t)
	plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
	require.NoError(t, err)
	observed := false
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("new-node-before-ack", func(tx *gorm.DB) {
		if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == db.Statement.ConnPool {
			observed = true
			tx.AddError(UpsertSystemInstance("new-node", nil, time.Now().Unix(), time.Now().Unix()))
		}
	}))
	_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "ack-eligibility", actor)
	require.NoError(t, db.Callback().Query().Remove("new-node-before-ack"))
	require.True(t, observed)
	require.ErrorIs(t, err, ErrCatalogInstanceIneligible)
	var operation CatalogSyncOperation
	require.NoError(t, db.First(&operation, "id = ?", "ack-eligibility").Error)
	assert.Equal(t, "committed_pending_publish", operation.State)
	require.NoError(t, db.Where("node_name = ?", "new-node").Delete(&SystemInstance{}).Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
}

func TestCatalogStartupInstanceStorage(t *testing.T) {
	db, actor := catalogStartupInstanceFixture(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("CREATE SCHEMA shadow").Error)
	require.NoError(t, db.Exec("CREATE TABLE shadow.system_instances (LIKE public.system_instances INCLUDING ALL)").Error)
	require.NoError(t, db.Exec("SET search_path TO shadow, public").Error)
	_, err = CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
	require.NoError(t, err, "sensitive checks must use the reference transaction's trusted schema, not an empty shadow")
	require.NoError(t, db.Exec("SET search_path TO public").Error)
	require.NoError(t, db.Exec("ALTER TABLE system_instances ENABLE ROW LEVEL SECURITY").Error)
	_, err = CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
	require.ErrorIs(t, err, ErrCatalogInstanceIneligible)
	require.NoError(t, db.Exec("ALTER TABLE system_instances DISABLE ROW LEVEL SECURITY").Error)
	require.NoError(t, db.Exec("CREATE TABLE inherited_instance () INHERITS (system_instances)").Error)
	_, err = CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
	require.ErrorIs(t, err, ErrCatalogInstanceIneligible)
}

func TestCatalogStartupMissingAbilityCache(t *testing.T) {
	db, _ := catalogStartupInstanceFixture(t)
	channel := Channel{Name: "missing-ability", Key: "test", Status: common.ChannelStatusEnabled, Models: "route-model", Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	var originalChannel Channel
	require.NoError(t, db.First(&originalChannel, channel.Id).Error)
	previous := common.MemoryCacheEnabled
	channelSyncLock.Lock()
	channels, groups, advanced := channelsIDM, group2model2channels, channel2advancedCustomConfig
	channelSyncLock.Unlock()
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previous
		channelSyncLock.Lock()
		channelsIDM, group2model2channels, channel2advancedCustomConfig = channels, groups, advanced
		channelSyncLock.Unlock()
	})
	for _, refresh := range []string{"ordinary", "lifecycle"} {
		t.Run(refresh, func(t *testing.T) {
			require.NotPanics(t, func() {
				if refresh == "ordinary" {
					InitChannelCache()
				} else {
					require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
				}
			})
			selected, err := GetRandomSatisfiedChannel("default", "route-model", 0, nil)
			require.NoError(t, err)
			require.NotNil(t, selected)
			assert.Equal(t, channel.Id, selected.Id)
			var count int64
			require.NoError(t, db.Model(&Ability{}).Count(&count).Error)
			assert.Zero(t, count, "read-only projection must not repair ability SQL")
			var persisted Channel
			require.NoError(t, db.First(&persisted, channel.Id).Error)
			assert.Equal(t, originalChannel, persisted, "both projections preserve the complete channel SQL row")
		})
	}
}
