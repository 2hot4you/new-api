package controller

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This package's TestMain opens no database. Never silently fall back to a
// different engine; every fixture owns and removes its exact disposable DB.
func catalogSyncPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	require.NotEmpty(t, dsn, "disposable loopback PostgreSQL is required")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.True(t, net.ParseIP(u.Hostname()).IsLoopback())
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	require.NoError(t, err)
	adminPool, err := admin.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, adminPool.Close()) })
	name := fmt.Sprintf("catalog_sync_api_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
	t.Cleanup(func() { require.NoError(t, admin.Exec(`DROP DATABASE "`+name+`"`).Error) })
	u.Path = "/" + name
	db, err := gorm.Open(postgres.Open(u.String()), config)
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	previous := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = previous })
	var version string
	require.NoError(t, db.Raw("SELECT version()").Scan(&version).Error)
	t.Log(version)
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.Vendor{}, &model.Option{}))
	require.NoError(t, db.Create(&model.Model{ModelName: "source-model", BillingCurrency: "USD", Status: 1}).Error)
	require.NoError(t, db.Create(&model.Option{Key: "ModelPrice", Value: `{"source-model":0.2}`}).Error)
	return db
}

func catalogSyncTarget(t *testing.T, lifetime ...time.Duration) (*gorm.DB, catalogmanifest.Actor, catalogmanifest.Plan) {
	t.Helper()
	db := catalogSyncPostgres(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Task{}, &model.TaskPlugin{}, &model.Midjourney{}, &model.SystemTask{}, &model.User{}, &model.UserSession{}, &model.SystemInstance{}))
	require.NoError(t, model.MigrateCatalogSync(db))
	priorRegistry, priorType := jsplugin.DefaultRegistry, common.MainDatabaseType()
	priorNode, priorMaster, priorStart := common.GetNodeIdentity(), common.IsMasterNode, common.StartTime
	priorRedis, priorMemory := common.RedisEnabled, common.MemoryCacheEnabled
	t.Cleanup(func() {
		jsplugin.DefaultRegistry = priorRegistry
		common.SetMainDatabaseType(priorType)
		common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = priorNode.Name, priorNode.Source, priorNode.ManuallyConfigured
		common.IsMasterNode, common.StartTime = priorMaster, priorStart
		common.RedisEnabled, common.MemoryCacheEnabled = priorRedis, priorMemory
	})
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	common.NodeName, common.NodeNameSource, common.NodeNameManuallyConfigured = "catalog-api-test", common.NodeNameSourceManual, true
	common.IsMasterNode, common.StartTime = true, time.Now().Unix()-10
	t.Setenv("CATALOG_SYNC_SINGLE_INSTANCE", "true")
	require.NoError(t, service.ReportCurrentSystemInstance())
	require.NoError(t, model.InitOptionMapBootstrap(context.Background()))
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	user := model.User{Username: "catalog-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1}
	require.NoError(t, db.Create(&user).Error)
	session := model.UserSession{SID: "catalog-api-session", UserID: user.Id, UserAuthVersion: 1, Version: 1, Status: model.UserSessionStatusActive, ExpiresAt: time.Now().Add(time.Hour).Unix(), RefreshHash: strings.Repeat("a", 64), LoginMethod: "password"}
	require.NoError(t, db.Create(&session).Error)
	actor := catalogmanifest.Actor{UserID: user.Id, SessionID: session.SID, TargetID: "target", AuthVersion: 1, SessionVersion: 1}
	snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.NoError(t, err)
	value, err := common.Marshal(catalogmanifest.VendorValue{Name: "new-vendor", Description: "from-dev", Status: 1})
	require.NoError(t, err)
	snapshot.Entries = append(snapshot.Entries, catalogmanifest.Entry{Kind: catalogmanifest.KindVendor, Key: "new-vendor", Value: string(value)})
	snapshot.Coverage[catalogmanifest.KindVendor]++
	snapshot.Digest, err = catalogmanifest.SnapshotDigest(snapshot)
	require.NoError(t, err)
	now := time.Now()
	if len(lifetime) > 0 {
		now = now.Add(-10*time.Minute + lifetime[0])
	}
	plan, err := model.CreateCatalogSyncPlan(context.Background(), snapshot, actor, now)
	require.NoError(t, err)
	require.True(t, catalogmanifest.PlanExecutable(plan, time.Now()))
	return db, actor, plan
}

func TestCatalogSyncReceiptDoesNotMutateCommittedOperation(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t)
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("catalog-api-ack-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "catalog_sync_operations" {
			if values, ok := tx.Statement.Dest.(map[string]any); ok && values["state"] == "succeeded" {
				tx.AddError(errors.New("injected publication ACK failure"))
			}
		}
	}))
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "receipt-operation", actor)
	require.Error(t, err)
	require.Equal(t, "receipt-operation", result.OperationID, "real apply committed before ACK failure")
	require.NoError(t, db.Callback().Update().Remove("catalog-api-ack-failure"))
	var before model.CatalogSyncState
	require.NoError(t, db.First(&before).Error)
	require.Equal(t, "committed_pending_publish", before.PublicationState)
	var anchor int64
	require.NoError(t, db.Table("marketplace_order_locks").Select("version").Scan(&anchor).Error)
	for _, scenario := range []string{"consumed-plan", "expired-plan", "pruned-plan"} {
		t.Run(scenario, func(t *testing.T) {
			if scenario == "expired-plan" {
				require.NoError(t, db.Model(&model.CatalogSyncPlan{}).Where("id = ?", plan.ID).Update("expires_at", time.Now().Unix()-1).Error)
			}
			if scenario == "pruned-plan" {
				require.NoError(t, db.Where("id = ?", plan.ID).Delete(&model.CatalogSyncPlan{}).Error)
			}
			got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, result, got)
			var after model.CatalogSyncState
			require.NoError(t, db.First(&after).Error)
			assert.Equal(t, before, after)
			var afterAnchor int64
			require.NoError(t, db.Table("marketplace_order_locks").Select("version").Scan(&afterAnchor).Error)
			assert.Equal(t, anchor, afterAnchor, "receipt lookup must not run the target anchor-writing engine")
			if scenario == "pruned-plan" {
				_, err = model.GetCatalogSyncOperation(context.Background(), result.OperationID)
				assert.Error(t, err, "derived history still requires retained plan provenance")
				_, _, err = model.ListCatalogSyncOperations(context.Background(), 0, 20)
				assert.Error(t, err)
			}
		})
	}
}

func TestCatalogSyncHistoryRejectsChangedProvenance(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t, 3*time.Second)
	_, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "history-operation", actor)
	require.NoError(t, err)
	rows, count, err := model.ListCatalogSyncOperations(context.Background(), -1, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	require.Len(t, rows, 1)
	assert.Equal(t, "succeeded", rows[0].State)
	assert.Equal(t, "dev", rows[0].Summary.SourceID)
	assert.Empty(t, rows[0].History)
	assert.Empty(t, rows[0].Backup)
	assert.Empty(t, rows[0].Result)
	encoded, err := common.Marshal(rows)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), actor.SessionID)
	got, err := model.GetCatalogSyncOperation(context.Background(), "history-operation")
	require.NoError(t, err)
	assert.Equal(t, rows[0], got)
	_, err = model.GetCatalogSyncOperation(context.Background(), "absent-operation")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	// Expire the real consumed preview without changing its sealed provenance.
	require.Eventually(t, func() bool { return time.Now().Unix() >= plan.ExpiresAt }, 5*time.Second, 10*time.Millisecond)
	got, err = model.GetCatalogSyncOperation(context.Background(), "history-operation")
	require.NoError(t, err)
	assert.Equal(t, rows[0], got)
	plan.Snapshot.SourceID = "forged-origin"
	changed, err := common.Marshal(plan)
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.CatalogSyncPlan{}).Where("id = ?", plan.ID).Update("body", string(changed)).Error)
	_, _, err = model.ListCatalogSyncOperations(context.Background(), 0, 20)
	assert.Error(t, err, "history must authenticate source/action provenance, not only compare copied digest strings")
	_, err = model.GetCatalogSyncOperation(context.Background(), "history-operation")
	assert.Error(t, err)
}

func TestCatalogSyncSourceReadOnlyCoherentSnapshot(t *testing.T) {
	db := catalogSyncPostgres(t)
	require.NoError(t, db.Create(&model.Option{Key: "Secret", Value: "must-not-export"}).Error)
	var observed bool
	var once sync.Once
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("catalog-source-snapshot", func(tx *gorm.DB) {
		if tx.Statement.Table != "vendors" {
			return
		}
		once.Do(func() {
			observed = true
			probe := tx.Session(&gorm.Session{NewDB: true})
			var readOnly, isolation string
			require.NoError(t, probe.Raw("SHOW transaction_read_only").Scan(&readOnly).Error)
			require.NoError(t, probe.Raw("SHOW transaction_isolation").Scan(&isolation).Error)
			assert.Equal(t, "on", readOnly)
			assert.Equal(t, "repeatable read", isolation)
			require.NoError(t, probe.Exec("SAVEPOINT read_only_probe").Error)
			err := probe.Exec("UPDATE models SET display_name = 'forbidden'").Error
			require.ErrorContains(t, err, "read-only transaction")
			require.NoError(t, probe.Exec("ROLLBACK TO SAVEPOINT read_only_probe").Error)
			require.NoError(t, probe.Exec("RELEASE SAVEPOINT read_only_probe").Error)
			assert.ErrorIs(t, model.TryWithCatalogWriteBarrier(context.Background(), func() error { return nil }), model.ErrCatalogWriterBusy)
			// A real concurrent writer is allowed by ACCESS SHARE. Its new model
			// and price must both remain outside this already-established snapshot.
			require.NoError(t, db.Transaction(func(writer *gorm.DB) error {
				if err := writer.Create(&model.Model{ModelName: "later-model", BillingCurrency: "USD"}).Error; err != nil {
					return err
				}
				return writer.Model(&model.Option{}).Where("key = ?", "ModelPrice").Update("value", `{"source-model":0.2,"later-model":0.4}`).Error
			}))
		})
	}))
	snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.NoError(t, err)
	require.True(t, observed)
	assert.True(t, snapshot.Complete)
	assert.Equal(t, 1, snapshot.Coverage[catalogmanifest.KindModel])
	assert.Equal(t, 1, snapshot.Coverage[catalogmanifest.KindModelPrice])
	encoded, err := common.Marshal(snapshot)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "must-not-export")
	assert.NotContains(t, string(encoded), "later-model")
	assert.Empty(t, snapshot.ObjectVersions)
	require.NoError(t, db.Callback().Query().Remove("catalog-source-snapshot"))
	next, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.NoError(t, err)
	assert.Equal(t, 2, next.Coverage[catalogmanifest.KindModel])
	assert.Equal(t, 2, next.Coverage[catalogmanifest.KindModelPrice])
	var display string
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "source-model").Select("display_name").Scan(&display).Error)
	assert.NotEqual(t, "forbidden", display)
}

func TestCatalogSyncSourceRejectsPrelockReplacement(t *testing.T) {
	db := catalogSyncPostgres(t)
	var replaced atomic.Bool
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("catalog-source-replacement", func(tx *gorm.DB) {
		if !strings.HasPrefix(tx.Statement.SQL.String(), `LOCK TABLE "public"."models"`) || replaced.Swap(true) {
			return
		}
		require.NoError(t, db.Transaction(func(writer *gorm.DB) error {
			if err := writer.Exec("ALTER TABLE models RENAME TO old_models").Error; err != nil {
				return err
			}
			return writer.Exec("CREATE TABLE models (LIKE old_models INCLUDING ALL)").Error
		}))
	}))
	snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.True(t, replaced.Load())
	require.ErrorContains(t, err, "changed during read acquisition")
	assert.False(t, snapshot.Complete)
	require.NoError(t, db.Callback().Raw().Remove("catalog-source-replacement"))
}

func catalogSyncWaitForLock(t *testing.T, db *gorm.DB, query string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var count int64
		err := db.Raw("SELECT count(*) FROM pg_catalog.pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE ?", query+"%").Scan(&count).Error
		return err == nil && count > 0
	}, 5*time.Second, 10*time.Millisecond, "real PostgreSQL lock wait must be observed")
}

func TestCatalogSyncSourceDDLStabilityAndCancellation(t *testing.T) {
	db := catalogSyncPostgres(t)
	entered, release := make(chan struct{}), make(chan struct{})
	resume := sync.OnceFunc(func() { close(release) })
	defer resume()
	var once sync.Once
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("catalog-source-pause", func(tx *gorm.DB) {
		if tx.Statement.Table == "vendors" {
			once.Do(func() { close(entered); <-release })
		}
	}))
	finished := make(chan error, 1)
	go func() { _, err := model.ExportManagedCatalog(context.Background(), "dev"); finished <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("source did not reach its actual projection")
	}
	ddl := make(chan error, 1)
	go func() { ddl <- db.Exec("ALTER TABLE models ADD COLUMN ddl_probe integer").Error }()
	catalogSyncWaitForLock(t, db, "ALTER TABLE models ADD COLUMN ddl_probe")
	resume()
	require.NoError(t, <-finished)
	require.NoError(t, <-ddl)
	require.NoError(t, db.Callback().Query().Remove("catalog-source-pause"))
	blocker := db.Begin()
	require.NoError(t, blocker.Error)
	defer blocker.Rollback()
	require.NoError(t, blocker.Exec("LOCK TABLE models IN ACCESS EXCLUSIVE MODE").Error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _, err := model.ExportManagedCatalog(ctx, "dev"); finished <- err }()
	catalogSyncWaitForLock(t, db, `LOCK TABLE "public"."models"`)
	cancel()
	select {
	case err := <-finished:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("source cancellation did not release its transaction")
	}
	require.NoError(t, model.TryWithCatalogWriteBarrier(context.Background(), func() error { return nil }))
	require.NoError(t, blocker.Rollback().Error)
	_, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.NoError(t, err)
}

func TestCatalogSyncReceiptCurrentAuthorizationAndBinding(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t)
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "auth-operation", actor)
	require.NoError(t, err)
	otherRoot := model.User{Username: "other-catalog-root", AffCode: "other-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1}
	require.NoError(t, db.Create(&otherRoot).Error)
	for sid, userID := range map[string]int{"same-root-other-session": actor.UserID, "other-root-session": otherRoot.Id} {
		require.NoError(t, db.Create(&model.UserSession{SID: sid, UserID: userID, UserAuthVersion: 1, Version: 1, Status: model.UserSessionStatusActive, ExpiresAt: time.Now().Add(time.Hour).Unix(), RefreshHash: strings.Repeat(sid[:1], 64), LoginMethod: "password"}).Error)
	}
	for _, scenario := range []string{"actor", "sid", "valid-other-session", "valid-other-root", "auth-version", "session-version", "target", "plan", "digest", "unknown-operation", "empty-operation", "padded-operation", "long-operation", "revoked", "expired", "disabled", "demoted", "stored-auth-version", "stored-session-version"} {
		t.Run(scenario, func(t *testing.T) {
			candidate, planID, digest, operation := actor, plan.ID, plan.Digest, result.OperationID
			switch scenario {
			case "actor":
				candidate.UserID++
			case "sid":
				candidate.SessionID = "other-session"
			case "valid-other-session":
				candidate.SessionID = "same-root-other-session"
			case "valid-other-root":
				candidate.UserID, candidate.SessionID = otherRoot.Id, "other-root-session"
			case "auth-version":
				candidate.AuthVersion++
			case "session-version":
				candidate.SessionVersion++
			case "target":
				candidate.TargetID = "other-target"
			case "plan":
				planID = "other-plan"
			case "digest":
				digest = "other-digest"
			case "unknown-operation":
				operation = "unknown-operation"
			case "empty-operation":
				operation = ""
			case "padded-operation":
				operation = " padded "
			case "long-operation":
				operation = strings.Repeat("a", 65)
			case "revoked":
				require.NoError(t, db.Exec("UPDATE user_sessions SET revoked_at = 1").Error)
			case "expired":
				require.NoError(t, db.Exec("UPDATE user_sessions SET expires_at = ?", time.Now().Unix()-1).Error)
			case "disabled":
				require.NoError(t, db.Exec("UPDATE users SET status = ?", common.UserStatusDisabled).Error)
			case "demoted":
				require.NoError(t, db.Exec("UPDATE users SET role = ?", common.RoleAdminUser).Error)
			case "stored-auth-version":
				require.NoError(t, db.Exec("UPDATE users SET auth_version = 2").Error)
			case "stored-session-version":
				require.NoError(t, db.Exec("UPDATE user_sessions SET version = 2").Error)
			}
			got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), planID, digest, operation, candidate)
			if scenario == "unknown-operation" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				if scenario == "valid-other-session" || scenario == "valid-other-root" {
					assert.ErrorIs(t, err, model.ErrCatalogSyncOperationConflict, "live root authorization does not relax exact receipt binding")
				}
			}
			assert.False(t, found)
			assert.Equal(t, catalogmanifest.Result{}, got)
			require.NoError(t, db.Exec("UPDATE users SET role = ?, status = ?, auth_version = 1", common.RoleRootUser, common.UserStatusEnabled).Error)
			require.NoError(t, db.Exec("UPDATE user_sessions SET version = 1, revoked_at = 0, expires_at = ?", time.Now().Add(time.Hour).Unix()).Error)
		})
	}
	got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, result, got)
}

func TestCatalogSyncReceiptPhysicalAuthorizationStorage(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t)
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "physical-operation", actor)
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	for _, scenario := range []struct {
		name    string
		setup   []string
		restore []string
	}{
		{"users-rls", []string{"ALTER TABLE users ENABLE ROW LEVEL SECURITY"}, []string{"ALTER TABLE users DISABLE ROW LEVEL SECURITY"}},
		{"sessions-forced-rls", []string{"ALTER TABLE user_sessions ENABLE ROW LEVEL SECURITY", "ALTER TABLE user_sessions FORCE ROW LEVEL SECURITY"}, []string{"ALTER TABLE user_sessions NO FORCE ROW LEVEL SECURITY", "ALTER TABLE user_sessions DISABLE ROW LEVEL SECURITY"}},
		{"operations-rls", []string{"ALTER TABLE catalog_sync_operations ENABLE ROW LEVEL SECURITY"}, []string{"ALTER TABLE catalog_sync_operations DISABLE ROW LEVEL SECURITY"}},
		{"operations-view", []string{"ALTER TABLE catalog_sync_operations RENAME TO original_operations", "CREATE VIEW catalog_sync_operations AS SELECT * FROM original_operations"}, []string{"DROP VIEW catalog_sync_operations", "ALTER TABLE original_operations RENAME TO catalog_sync_operations"}},
		{"sessions-temporary-shadow", []string{"CREATE TEMP TABLE user_sessions (LIKE public.user_sessions INCLUDING ALL)"}, []string{"DROP TABLE pg_temp.user_sessions"}},
		{"users-unlogged", []string{"ALTER TABLE users SET UNLOGGED"}, []string{"ALTER TABLE users SET LOGGED"}},
		{"auth-primary-key", []string{"ALTER TABLE users DROP CONSTRAINT users_pkey"}, []string{"ALTER TABLE users ADD PRIMARY KEY (id)"}},
		{"mixed-namespace", []string{"CREATE SCHEMA receipt_shadow", "CREATE TABLE receipt_shadow.users (LIKE public.users INCLUDING ALL)", "SET search_path TO receipt_shadow, public"}, []string{"SET search_path TO public", "DROP TABLE receipt_shadow.users", "DROP SCHEMA receipt_shadow"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			for _, statement := range scenario.setup {
				require.NoError(t, db.Exec(statement).Error)
			}
			t.Cleanup(func() {
				for _, statement := range scenario.restore {
					require.NoError(t, db.Exec(statement).Error)
				}
			})
			got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
			require.Error(t, err)
			assert.False(t, found)
			assert.Equal(t, catalogmanifest.Result{}, got)
		})
	}
	got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, result, got)
	// Lookup depends only on auth and immutable receipts, not target readiness,
	// a plan table, heartbeat, plugin schema, or a mutable catalog relation.
	require.NoError(t, db.Exec("DROP TABLE catalog_sync_plans, system_instances, models, vendors, options").Error)
	got, found, err = model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, result, got)
}

func TestCatalogSyncReceiptAuthoritativeLocksAndCancellation(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t)
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "locked-operation", actor)
	require.NoError(t, err)
	blocker := db.Begin()
	require.NoError(t, blocker.Exec("UPDATE users SET role = ? WHERE id = ?", common.RoleAdminUser, actor.UserID).Error)
	t.Cleanup(func() { blocker.Rollback() })
	finished := make(chan error, 1)
	go func() {
		_, _, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
		finished <- err
	}()
	catalogSyncWaitForLock(t, db, `SELECT%FROM "users"%FOR UPDATE`)
	require.NoError(t, blocker.Commit().Error)
	require.ErrorIs(t, <-finished, model.ErrCatalogSyncPlanUnavailable, "locking authoritative role read observes committed demotion without version bump")
	require.NoError(t, db.Exec("UPDATE users SET role = ? WHERE id = ?", common.RoleRootUser, actor.UserID).Error)
	blocker = db.Begin()
	require.NoError(t, blocker.Exec("LOCK TABLE catalog_sync_operations IN ACCESS EXCLUSIVE MODE").Error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		got, found, err := model.LookupCatalogSyncOperationResult(ctx, plan.ID, plan.Digest, result.OperationID, actor)
		if found || got != (catalogmanifest.Result{}) {
			finished <- errors.New("canceled lookup returned receipt")
			return
		}
		finished <- err
	}()
	catalogSyncWaitForLock(t, db, `LOCK TABLE "public"."catalog_sync_operations"`)
	cancel()
	select {
	case err := <-finished:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("receipt cancellation did not drain transaction")
	}
	require.NoError(t, blocker.Rollback().Error)
	// An independent writer proves auth-row and relation locks did not leak.
	ctx, done := context.WithTimeout(context.Background(), time.Second)
	defer done()
	require.NoError(t, db.WithContext(ctx).Exec("UPDATE users SET username = username WHERE id = ?", actor.UserID).Error)
	got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, result, got)
}

func TestCatalogSyncSourceRestrictedRole(t *testing.T) {
	db := catalogSyncPostgres(t)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	role := fmt.Sprintf("catalog_source_reader_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE ROLE "+role+" NOLOGIN").Error)
	t.Cleanup(func() {
		require.NoError(t, db.Exec("RESET ROLE").Error)
		require.NoError(t, db.Exec("DROP OWNED BY "+role).Error)
		require.NoError(t, db.Exec("DROP ROLE "+role).Error)
	})
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO "+role).Error)
	require.NoError(t, db.Exec("GRANT SELECT ON models, vendors, options TO "+role).Error)
	require.NoError(t, db.Exec("SET ROLE "+role).Error)
	snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.NoError(t, err, "source requires only SELECT and ACCESS SHARE, not target mutation privileges")
	assert.True(t, snapshot.Complete)
	assert.Equal(t, 1, snapshot.Coverage[catalogmanifest.KindModel])
	require.NoError(t, db.Exec("RESET ROLE").Error)
	require.NoError(t, db.Exec("ALTER TABLE models ENABLE ROW LEVEL SECURITY").Error)
	require.NoError(t, db.Exec("SET ROLE "+role).Error)
	var rows int64
	require.NoError(t, db.Model(&model.Model{}).Count(&rows).Error)
	require.Zero(t, rows, "real limited role is filtered by RLS")
	snapshot, err = model.ExportManagedCatalog(context.Background(), "dev")
	require.Error(t, err)
	assert.False(t, snapshot.Complete)
}

type catalogReadLostAckConnector struct {
	connection *catalogReadLostAckConnection
	driver     driver.Driver
}

func (c catalogReadLostAckConnector) Connect(context.Context) (driver.Conn, error) {
	return c.connection, nil
}
func (c catalogReadLostAckConnector) Driver() driver.Driver { return c.driver }

// All SQL reaches the real native PostgreSQL connection. Only the successful
// COMMIT response is lost, after the server has committed and released locks.
type catalogReadLostAckConnection struct {
	driver.Conn
	commits atomic.Int64
}

func (c *catalogReadLostAckConnection) Close() error { return nil } // original loan owns the native connection
func (c *catalogReadLostAckConnection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}
func (c *catalogReadLostAckConnection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}
func (c *catalogReadLostAckConnection) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}
func (c *catalogReadLostAckConnection) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &catalogReadLostAckTx{Tx: tx, connection: c}, nil
}

type catalogReadLostAckTx struct {
	driver.Tx
	connection *catalogReadLostAckConnection
}

func (tx *catalogReadLostAckTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	tx.connection.commits.Add(1)
	return errors.New("injected lost read COMMIT acknowledgement")
}

func TestCatalogSyncReadCommitFailureReturnsNoClaim(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t)
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "read-commit-operation", actor)
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	loan, err := pool.Conn(context.Background())
	require.NoError(t, err)
	defer loan.Close()
	require.NoError(t, loan.Raw(func(native any) error {
		connection := &catalogReadLostAckConnection{Conn: native.(driver.Conn)}
		observed := sql.OpenDB(catalogReadLostAckConnector{connection: connection, driver: pool.Driver()})
		observed.SetMaxOpenConns(1)
		defer observed.Close()
		root := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
		root.Statement.ConnPool = observed
		model.DB = root
		defer func() { model.DB = db }()
		snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
		require.ErrorContains(t, err, "lost read COMMIT")
		assert.False(t, snapshot.Complete)
		got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
		require.ErrorContains(t, err, "lost read COMMIT")
		assert.False(t, found)
		assert.Equal(t, catalogmanifest.Result{}, got)
		assert.Equal(t, int64(2), connection.commits.Load(), "one real read commit per call, no automatic retry")
		return nil
	}))
	got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, result, got)
}

func TestCatalogSyncSourceInputAndDetachedValidation(t *testing.T) {
	db := catalogSyncPostgres(t)
	_, err := model.ExportManagedCatalog(context.Background(), " ")
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = model.ExportManagedCatalog(ctx, "dev")
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, db.Create(&model.Option{Key: "billing_setting.billing_expr", Value: `{"source-model":"not valid expression"}`}).Error)
	snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.Error(t, err, "structural SQL capture cannot bypass the original full validator")
	assert.False(t, snapshot.Complete)
	require.NoError(t, model.TryWithCatalogWriteBarrier(context.Background(), func() error { return nil }))
	// A transaction must not be accepted as the owning entrypoint's root pool.
	tx := db.Begin()
	require.NoError(t, tx.Error)
	model.DB = tx
	_, err = model.ExportManagedCatalog(context.Background(), "dev")
	model.DB = db
	require.ErrorContains(t, err, "root database pool")
	require.NoError(t, tx.Rollback().Error)
}

func TestCatalogSyncSourceRejectsUnsafePhysicalView(t *testing.T) {
	for _, scenario := range []string{"rls", "forced-rls", "view", "inheritance", "unlogged", "temporary-shadow", "mixed-namespace"} {
		t.Run(scenario, func(t *testing.T) {
			db := catalogSyncPostgres(t)
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			switch scenario {
			case "rls":
				require.NoError(t, db.Exec("ALTER TABLE models ENABLE ROW LEVEL SECURITY").Error)
			case "forced-rls":
				require.NoError(t, db.Exec("ALTER TABLE models ENABLE ROW LEVEL SECURITY").Error)
				require.NoError(t, db.Exec("ALTER TABLE models FORCE ROW LEVEL SECURITY").Error)
			case "view":
				require.NoError(t, db.Exec("ALTER TABLE models RENAME TO private_models").Error)
				require.NoError(t, db.Exec("CREATE VIEW models AS SELECT * FROM private_models WHERE false").Error)
			case "inheritance":
				require.NoError(t, db.Exec("CREATE TABLE child_models () INHERITS (models)").Error)
			case "unlogged":
				require.NoError(t, db.Exec("ALTER TABLE models SET UNLOGGED").Error)
			case "temporary-shadow":
				require.NoError(t, db.Exec("CREATE TEMP TABLE vendors (LIKE public.vendors INCLUDING ALL)").Error)
			case "mixed-namespace":
				require.NoError(t, db.Exec("CREATE SCHEMA shadow").Error)
				require.NoError(t, db.Exec("CREATE TABLE shadow.vendors (LIKE public.vendors INCLUDING ALL)").Error)
				require.NoError(t, db.Exec("SET search_path TO shadow, public").Error)
			}
			snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
			assert.Error(t, err, "unsafe physical view cannot claim complete")
			assert.False(t, snapshot.Complete)
		})
	}
}
