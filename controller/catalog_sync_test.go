package controller

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func catalogSourceEnvironment(t *testing.T) (map[string]string, string) {
	t.Helper()
	token := "reader-a." + base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("a", 32)))
	verifier := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(token)))
	return map[string]string{"CATALOG_SYNC_ROLE": "source", "CATALOG_SYNC_SOURCE_ID": "dev", "CATALOG_SYNC_SINGLE_INSTANCE": "true", "CATALOG_SYNC_READERS_JSON": `{"reader-a":{"verifier":"` + verifier + `","status":"active"}}`}, token
}

func TestCatalogSyncConfiguration(t *testing.T) {
	runtime, err := service.NewCatalogSyncRuntime(func(string) string { return "" })
	require.NoError(t, err)
	assert.Equal(t, "disabled", runtime.Status().Role)
	assert.False(t, runtime.Status().SourceReady)
	assert.False(t, runtime.Status().TargetReady)
	assert.False(t, runtime.Status().ManagementReady)
	for _, tc := range []struct{ key, value string }{
		{"CATALOG_SYNC_ROLE", "Source"}, {"CATALOG_SYNC_ROLE", "unknown"},
		{"CATALOG_SYNC_SINGLE_INSTANCE", "TRUE"}, {"CATALOG_SYNC_SINGLE_INSTANCE", ""},
		{"CATALOG_SYNC_SOURCE_ID", " dev"}, {"CATALOG_SYNC_SOURCE_ID", ""},
		{"CATALOG_SYNC_SOURCE_ID", "bad/id"}, {"CATALOG_SYNC_SOURCE_ID", strings.Repeat("x", 65)},
		{"CATALOG_SYNC_TOKEN", "private-test-secret"}, {"CATALOG_SYNC_TARGET_ID", "target"},
		{"CATALOG_SYNC_EXTERNAL_ORIGIN", "http://example.com"}, {"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://example.com/path"},
		{"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://user@example.com"}, {"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://example.com?x=1"},
		{"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://example.com#"}, {"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://example.com:0"},
		{"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://example.com."}, {"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://127.0.0.1"},
		{"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://0x7f.0x0.0x0.0x1"}, {"CATALOG_SYNC_EXTERNAL_ORIGIN", "https://example.9999999999999999999999999"},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			env, token := catalogSourceEnvironment(t)
			env[tc.key] = tc.value
			runtime, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
			require.ErrorIs(t, err, service.ErrCatalogSyncConfiguration)
			require.NotNil(t, runtime)
			assert.False(t, runtime.Status().SourceReady)
			assert.ErrorIs(t, runtime.AuthenticateReader(token), service.ErrCatalogSyncReaderDenied)
			assert.NotContains(t, err.Error(), token)
		})
	}
	env, token := catalogSourceEnvironment(t)
	runtime, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	assert.True(t, runtime.Status().SourceReady)
	assert.False(t, runtime.Status().ManagementReady)
	_, err = runtime.ExternalOrigin()
	assert.Error(t, err)
	env["CATALOG_SYNC_EXTERNAL_ORIGIN"] = "https://EXAMPLE.com:443"
	runtime, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	origin, err := runtime.ExternalOrigin()
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", origin)
	assert.False(t, runtime.Status().ManagementReady, "source credentials never grant management readiness")
	encoded, err := common.Marshal(runtime)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), token)
	assert.NotContains(t, string(encoded), "verifier")
	assert.NotContains(t, fmt.Sprintf("%+v %#v", runtime, runtime), token)
	delete(env, "CATALOG_SYNC_READERS_JSON")
	env["CATALOG_SYNC_ROLE"], env["CATALOG_SYNC_TARGET_ID"], env["CATALOG_SYNC_TOKEN"] = "target", "prod", token
	runtime, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	assert.True(t, runtime.Status().TargetReady)
	assert.True(t, runtime.Status().ManagementReady)
	assert.False(t, runtime.Status().SourceReady)
	assert.ErrorIs(t, runtime.AuthenticateReader(token), service.ErrCatalogSyncReaderDenied)
	delete(env, "CATALOG_SYNC_EXTERNAL_ORIGIN")
	runtime, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	assert.True(t, runtime.Status().TargetReady)
	assert.False(t, runtime.Status().ManagementReady)
}

func TestCatalogSyncReaderConfigAndAuthentication(t *testing.T) {
	env, token := catalogSourceEnvironment(t)
	valid := env["CATALOG_SYNC_READERS_JSON"]
	for _, raw := range []string{"", "null", "[]", "{}", valid + valid,
		strings.Replace(valid, `"status":"active"`, `"status":"active","status":"revoked"`, 1),
		strings.Replace(valid, `"status":"active"`, `"status":"active","extra":true`, 1),
		strings.Replace(valid, `"status":"active"`, `"status":"ACTIVE"`, 1),
		strings.Replace(valid, `"status":"active"`, `"status":null`, 1),
		strings.Replace(valid, "sha256:", "bcrypt:", 1),
		strings.Replace(valid, "reader-a", "bad/id", 1),
		strings.TrimSuffix(valid, "}") + "," + strings.TrimPrefix(valid, "{"),
	} {
		env["CATALOG_SYNC_READERS_JSON"] = raw
		_, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
		assert.ErrorIs(t, err, service.ErrCatalogSyncConfiguration, "reader config must fail closed")
	}
	env["CATALOG_SYNC_READERS_JSON"] = valid
	runtime, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	require.NoError(t, runtime.AuthenticateReader(token))
	for _, candidate := range []string{"", "Bearer " + token, token + "=", "reader-a." + strings.Repeat("b", 43), strings.Replace(token, "reader-a", "unknown", 1)} {
		assert.ErrorIs(t, runtime.AuthenticateReader(candidate), service.ErrCatalogSyncReaderDenied)
	}
	env["CATALOG_SYNC_READERS_JSON"] = strings.Replace(valid, "active", "revoked", 1)
	revoked, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	assert.ErrorIs(t, revoked.AuthenticateReader(token), service.ErrCatalogSyncReaderDenied)
	assert.False(t, revoked.Status().SourceReady)
	newToken := "reader-b." + base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("b", 32)))
	newReader := fmt.Sprintf(`"reader-b":{"verifier":"sha256:%x","status":"active"}`, sha256.Sum256([]byte(newToken)))
	env["CATALOG_SYNC_READERS_JSON"] = strings.TrimSuffix(valid, "}") + "," + newReader + "}"
	rotating, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	require.NoError(t, rotating.AuthenticateReader(token))
	require.NoError(t, rotating.AuthenticateReader(newToken))
	env["CATALOG_SYNC_READERS_JSON"] = strings.Replace(env["CATALOG_SYNC_READERS_JSON"], `"status":"active"`, `"status":"revoked"`, 1)
	rotated, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	assert.ErrorIs(t, rotated.AuthenticateReader(token), service.ErrCatalogSyncReaderDenied)
	require.NoError(t, rotated.AuthenticateReader(newToken))
}

func TestCatalogSyncReaderBoundedConcurrentRateLimit(t *testing.T) {
	env, token := catalogSourceEnvironment(t)
	runtime, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for range 80 {
		wg.Go(func() {
			if runtime.AuthenticateReader(token) == nil {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int64(60), accepted.Load())
	assert.ErrorIs(t, runtime.AuthenticateReader(token), service.ErrCatalogSyncReaderDenied)
	// Attacker-controlled identities cannot consume the configured ID's bucket.
	runtime, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	for i := range 100 {
		assert.ErrorIs(t, runtime.AuthenticateReader(fmt.Sprintf("unknown-%d.%s", i, strings.Split(token, ".")[1])), service.ErrCatalogSyncReaderDenied)
	}
	require.NoError(t, runtime.AuthenticateReader(token))
}

func TestCatalogSyncReaderLimitsAndWindow(t *testing.T) {
	env, token := catalogSourceEnvironment(t)
	var readers = map[string]any{}
	for i := range 65 {
		readers[fmt.Sprintf("reader-%d", i)] = map[string]string{"verifier": fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(token))), "status": "active"}
	}
	raw, err := common.Marshal(readers)
	require.NoError(t, err)
	env["CATALOG_SYNC_READERS_JSON"] = string(raw)
	_, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	assert.ErrorIs(t, err, service.ErrCatalogSyncConfiguration)
	delete(readers, "reader-64")
	raw, err = common.Marshal(readers)
	require.NoError(t, err)
	env["CATALOG_SYNC_READERS_JSON"] = string(raw)
	_, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	env["CATALOG_SYNC_READERS_JSON"] = strings.Repeat(" ", 32769)
	_, err = service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	assert.ErrorIs(t, err, service.ErrCatalogSyncConfiguration)
	synctest.Test(t, func(t *testing.T) {
		env, token := catalogSourceEnvironment(t)
		runtime, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
		require.NoError(t, err)
		for range 60 {
			assert.ErrorIs(t, runtime.AuthenticateReader("reader-a.invalid"), service.ErrCatalogSyncReaderDenied)
		}
		assert.ErrorIs(t, runtime.AuthenticateReader(token), service.ErrCatalogSyncReaderDenied, "failed attempts exhaust the same credential bucket")
		time.Sleep(59 * time.Second)
		assert.ErrorIs(t, runtime.AuthenticateReader(token), service.ErrCatalogSyncReaderDenied)
		time.Sleep(time.Second)
		require.NoError(t, runtime.AuthenticateReader(token), "new fixed window permits a valid credential")
	})
}

func TestCatalogSyncDisabledAndTargetConfigurations(t *testing.T) {
	for _, role := range []string{"disabled", "target"} {
		for _, mutation := range []string{"token", "readers", "source", "target", "single"} {
			t.Run(role+"/"+mutation, func(t *testing.T) {
				env, token := catalogSourceEnvironment(t)
				readers := env["CATALOG_SYNC_READERS_JSON"]
				delete(env, "CATALOG_SYNC_READERS_JSON")
				env["CATALOG_SYNC_ROLE"] = role
				if role == "target" {
					env["CATALOG_SYNC_TARGET_ID"] = "prod"
					env["CATALOG_SYNC_TOKEN"] = token
				}
				switch mutation {
				case "token":
					if role == "target" {
						env["CATALOG_SYNC_TOKEN"] = "bad-token"
					} else {
						env["CATALOG_SYNC_TOKEN"] = token
					}
				case "readers":
					env["CATALOG_SYNC_READERS_JSON"] = readers
				case "source":
					env["CATALOG_SYNC_SOURCE_ID"] = "bad/source"
				case "target":
					env["CATALOG_SYNC_TARGET_ID"] = "bad/target"
				case "single":
					env["CATALOG_SYNC_SINGLE_INSTANCE"] = "false"
				}
				_, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
				assert.ErrorIs(t, err, service.ErrCatalogSyncConfiguration)
			})
		}
	}
}

func TestCatalogSyncConfiguredSourcePGWire(t *testing.T) {
	db := catalogSyncPostgres(t)
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "source-model").Update("billing_currency", "CNY").Error)
	env, token := catalogSourceEnvironment(t)
	runtime, err := service.NewCatalogSyncRuntime(func(k string) string { return env[k] })
	require.NoError(t, err)
	require.NoError(t, runtime.AuthenticateReader(token))
	snapshot, err := model.ExportManagedCatalog(context.Background(), runtime.Status().SourceID)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.ApiSuccess(ctx, snapshot)
	require.Equal(t, 200, recorder.Code)
	var wire struct {
		Success bool                     `json:"success"`
		Message string                   `json:"message"`
		Data    catalogmanifest.Snapshot `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &wire))
	assert.True(t, wire.Success)
	assert.Empty(t, wire.Message)
	require.NoError(t, catalogmanifest.ValidateSnapshot(wire.Data))
	assert.Equal(t, snapshot, wire.Data)
	assert.Contains(t, recorder.Body.String(), `\"billing_currency\":\"CNY\"`)
	assert.NotContains(t, recorder.Body.String(), token)
	assert.NotContains(t, recorder.Body.String(), "verifier")
}

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

// All SQL reaches the real native PostgreSQL connection. The default fault
// loses its successful COMMIT response; afterCommit can instead interleave
// actual external DDL after server commit and before the helper returns.
type catalogReadLostAckConnection struct {
	driver.Conn
	commits                 atomic.Int64
	postCommitMetadataReads atomic.Int64
	afterCommit             func() error
	beforeSQL               func(context.Context, string) error
}

func (c *catalogReadLostAckConnection) Close() error { return nil } // original loan owns the native connection
func (c *catalogReadLostAckConnection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.beforeSQL != nil {
		if err := c.beforeSQL(ctx, query); err != nil {
			return nil, err
		}
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}
func (c *catalogReadLostAckConnection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.beforeSQL != nil {
		if err := c.beforeSQL(ctx, query); err != nil {
			return nil, err
		}
	}
	if c.commits.Load() > 0 && strings.Contains(query, "FROM pg_catalog.pg_class") {
		c.postCommitMetadataReads.Add(1)
	}
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
	if tx.connection.afterCommit != nil {
		return tx.connection.afterCommit()
	}
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

func catalogSyncBeforeQueryDDL(t *testing.T, db *gorm.DB, table string, statements ...string) *atomic.Bool {
	t.Helper()
	var ran atomic.Bool
	name := "catalog-read-concurrent-ddl"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != table || ran.Swap(true) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, db.WithContext(ctx).Transaction(func(ddl *gorm.DB) error {
			for _, statement := range statements {
				if err := ddl.Exec(statement).Error; err != nil {
					return err
				}
			}
			return nil
		}), "DDL must actually commit after the helper's final metadata check")
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove(name)) })
	return &ran
}

func TestCatalogSyncSourceConcurrentInheritance(t *testing.T) {
	db := catalogSyncPostgres(t)
	require.NoError(t, db.Exec("CREATE TABLE unrelated_vendors (LIKE vendors INCLUDING ALL)").Error)
	require.NoError(t, db.Exec("INSERT INTO unrelated_vendors (id, name) VALUES (100, 'outside-verified-relation')").Error)
	ran := catalogSyncBeforeQueryDDL(t, db, "vendors", "ALTER TABLE unrelated_vendors INHERIT vendors")
	snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.True(t, ran.Load())
	if err != nil {
		assert.False(t, snapshot.Complete)
		return
	}
	assert.Equal(t, 0, snapshot.Coverage[catalogmanifest.KindVendor], "concurrent inheritance must not expand the verified physical source")
}

func TestCatalogSyncReceiptConcurrentInheritance(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t)
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "inherited-receipt", actor)
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE unrelated_operations (LIKE catalog_sync_operations INCLUDING ALL)").Error)
	// Move the actual Apply receipt; never fabricate a successful operation.
	require.NoError(t, db.Exec("WITH moved AS (DELETE FROM catalog_sync_operations RETURNING *) INSERT INTO unrelated_operations SELECT * FROM moved").Error)
	ran := catalogSyncBeforeQueryDDL(t, db, "catalog_sync_operations", "ALTER TABLE unrelated_operations INHERIT catalog_sync_operations")
	got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
	require.True(t, ran.Load())
	assert.False(t, found, "receipt must not come from an unverified child relation")
	assert.Equal(t, catalogmanifest.Result{}, got)
	if err != nil {
		t.Logf("lookup failed closed: %v", err)
	}
}

func TestCatalogSyncSourceConcurrentSchemaSwap(t *testing.T) {
	db := catalogSyncPostgres(t)
	require.NoError(t, db.Exec("CREATE SCHEMA shadow").Error)
	for _, table := range []string{"models", "vendors", "options"} {
		require.NoError(t, db.Exec("CREATE TABLE shadow."+table+" (LIKE public."+table+" INCLUDING ALL)").Error)
		require.NoError(t, db.Exec("INSERT INTO shadow."+table+" SELECT * FROM public."+table).Error)
	}
	require.NoError(t, db.Exec("UPDATE shadow.models SET display_name = 'outside-verified-schema'").Error)
	ran := catalogSyncBeforeQueryDDL(t, db, "vendors", "ALTER SCHEMA public RENAME TO original", "ALTER SCHEMA shadow RENAME TO public")
	snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.True(t, ran.Load())
	if err != nil {
		assert.False(t, snapshot.Complete)
		return
	}
	encoded, err := common.Marshal(snapshot)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "outside-verified-schema", "name reuse must not redirect verified source consumers")
}

func TestCatalogSyncReceiptConcurrentSchemaSwap(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t)
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "schema-receipt", actor)
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE SCHEMA shadow").Error)
	for _, table := range []string{"users", "user_sessions", "catalog_sync_operations"} {
		require.NoError(t, db.Exec("CREATE TABLE shadow."+table+" (LIKE public."+table+" INCLUDING ALL)").Error)
		require.NoError(t, db.Exec("INSERT INTO shadow."+table+" SELECT * FROM public."+table).Error)
	}
	require.NoError(t, db.Exec("UPDATE public.users SET role = ?", common.RoleAdminUser).Error)
	ran := catalogSyncBeforeQueryDDL(t, db, "users", "ALTER SCHEMA public RENAME TO original", "ALTER SCHEMA shadow RENAME TO public")
	got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
	require.True(t, ran.Load())
	assert.Error(t, err, "shadow root must not override authoritative real-root demotion")
	assert.False(t, found)
	assert.Equal(t, catalogmanifest.Result{}, got)
}

func TestCatalogSyncReadSchemaSwapABA(t *testing.T) {
	for _, consumer := range []string{"source", "receipt"} {
		t.Run(consumer, func(t *testing.T) {
			var db *gorm.DB
			var actor catalogmanifest.Actor
			var plan catalogmanifest.Plan
			tables, first, last := []string{"models", "vendors", "options"}, "vendors", "models"
			if consumer == "source" {
				db = catalogSyncPostgres(t)
			} else {
				db, actor, plan = catalogSyncTarget(t)
				_, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "aba-receipt", actor)
				require.NoError(t, err)
				tables, first, last = []string{"users", "user_sessions", "catalog_sync_operations"}, "users", "catalog_sync_operations"
			}
			require.NoError(t, db.Exec("CREATE SCHEMA shadow").Error)
			for _, table := range tables {
				require.NoError(t, db.Exec("CREATE TABLE shadow."+table+" (LIKE public."+table+" INCLUDING ALL)").Error)
				require.NoError(t, db.Exec("INSERT INTO shadow."+table+" SELECT * FROM public."+table).Error)
			}
			if consumer == "source" {
				require.NoError(t, db.Exec("UPDATE shadow.models SET display_name = 'aba-shadow'").Error)
			} else {
				require.NoError(t, db.Exec("UPDATE public.users SET role = ?", common.RoleAdminUser).Error)
			}
			ran := catalogSyncBeforeQueryDDL(t, db, first, "ALTER SCHEMA public RENAME TO original", "ALTER SCHEMA shadow RENAME TO public")
			var restored atomic.Bool
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("catalog-schema-restore", func(tx *gorm.DB) {
				if tx.Statement.Table != last || restored.Swap(true) {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				require.NoError(t, db.WithContext(ctx).Transaction(func(ddl *gorm.DB) error {
					if err := ddl.Exec("ALTER SCHEMA public RENAME TO shadow").Error; err != nil {
						return err
					}
					return ddl.Exec("ALTER SCHEMA original RENAME TO public").Error
				}))
			}))
			t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove("catalog-schema-restore")) })
			if consumer == "source" {
				snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
				assert.Error(t, err, "restoring names cannot erase intervening shadow reads")
				assert.False(t, snapshot.Complete)
			} else {
				got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, "aba-receipt", actor)
				assert.Error(t, err, "restoring names cannot erase shadow authorization")
				assert.False(t, found)
				assert.Equal(t, catalogmanifest.Result{}, got)
			}
			require.True(t, ran.Load())
			require.True(t, restored.Load())
		})
	}
}

func TestCatalogSyncSourceConcurrentEmptySchemaSwap(t *testing.T) {
	db := catalogSyncPostgres(t)
	require.NoError(t, db.Exec("CREATE SCHEMA shadow").Error)
	for _, table := range []string{"models", "vendors", "options"} {
		require.NoError(t, db.Exec("CREATE TABLE shadow."+table+" (LIKE public."+table+" INCLUDING ALL)").Error)
	}
	ran := catalogSyncBeforeQueryDDL(t, db, "vendors", "ALTER SCHEMA public RENAME TO original", "ALTER SCHEMA shadow RENAME TO public")
	snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
	require.True(t, ran.Load())
	assert.Error(t, err, "empty shadow tables must not establish a false complete deletion source")
	assert.False(t, snapshot.Complete)
}

func TestCatalogSyncReadInheritanceDetachedAfterCommit(t *testing.T) {
	for _, consumer := range []string{"source", "receipt"} {
		t.Run(consumer, func(t *testing.T) {
			var db *gorm.DB
			var actor catalogmanifest.Actor
			var plan catalogmanifest.Plan
			table := "vendors"
			if consumer == "source" {
				db = catalogSyncPostgres(t)
			} else {
				db, actor, plan = catalogSyncTarget(t)
				_, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "detached-receipt", actor)
				require.NoError(t, err)
				table = "catalog_sync_operations"
			}
			require.NoError(t, db.Exec("CREATE TABLE detached_child (LIKE "+table+" INCLUDING ALL)").Error)
			if consumer == "source" {
				require.NoError(t, db.Exec("INSERT INTO detached_child (id, name) VALUES (100, 'detached-vendor')").Error)
			} else {
				require.NoError(t, db.Exec("WITH moved AS (DELETE FROM catalog_sync_operations RETURNING *) INSERT INTO detached_child SELECT * FROM moved").Error)
			}
			ran := catalogSyncBeforeQueryDDL(t, db, table, "ALTER TABLE detached_child INHERIT "+table)
			pool, err := db.DB()
			require.NoError(t, err)
			loan, err := pool.Conn(context.Background())
			require.NoError(t, err)
			defer loan.Close()
			var detached bool
			require.NoError(t, loan.Raw(func(native any) error {
				connection := &catalogReadLostAckConnection{Conn: native.(driver.Conn), afterCommit: func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					err := db.WithContext(ctx).Exec("ALTER TABLE detached_child NO INHERIT " + table).Error
					detached = err == nil
					return err
				}}
				observed := sql.OpenDB(catalogReadLostAckConnector{connection: connection, driver: pool.Driver()})
				observed.SetMaxOpenConns(1)
				defer observed.Close()
				root := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
				root.Statement.ConnPool = observed
				model.DB = root
				defer func() { model.DB = db }()
				if consumer == "source" {
					snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
					assert.Error(t, err, "post-commit detach cannot erase intervening unverified inheritance")
					assert.False(t, snapshot.Complete)
				} else {
					got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, "detached-receipt", actor)
					assert.Error(t, err)
					assert.False(t, found)
					assert.Equal(t, catalogmanifest.Result{}, got)
				}
				assert.Equal(t, int64(1), connection.commits.Load())
				assert.Positive(t, connection.postCommitMetadataReads.Load(), "fresh metadata proof must use the same reserved native connection")
				return nil
			}))
			require.True(t, ran.Load())
			require.True(t, detached, "real DDL detached after read commit, before any completion claim")
			var count int64
			require.NoError(t, db.Raw("SELECT count(*) FROM pg_catalog.pg_inherits WHERE inhparent = ?::regclass", table).Scan(&count).Error)
			require.Zero(t, count)
		})
	}
}

func TestCatalogSyncReadRejectsHistoricalInheritanceHint(t *testing.T) {
	for _, consumer := range []string{"source", "receipt"} {
		t.Run(consumer, func(t *testing.T) {
			var db *gorm.DB
			var actor catalogmanifest.Actor
			var plan catalogmanifest.Plan
			table := "vendors"
			if consumer == "source" {
				db = catalogSyncPostgres(t)
			} else {
				db, actor, plan = catalogSyncTarget(t)
				_, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "historic-hint-receipt", actor)
				require.NoError(t, err)
				table = "catalog_sync_operations"
			}
			require.NoError(t, db.Exec("CREATE TABLE historical_child (LIKE "+table+" INCLUDING ALL)").Error)
			require.NoError(t, db.Exec("ALTER TABLE historical_child INHERIT "+table).Error)
			require.NoError(t, db.Exec("ALTER TABLE historical_child NO INHERIT "+table).Error)
			var hint bool
			require.NoError(t, db.Raw("SELECT relhassubclass FROM pg_catalog.pg_class WHERE oid = ?::regclass", table).Scan(&hint).Error)
			require.True(t, hint, "PostgreSQL retains the historical hint after detachment")
			if consumer == "source" {
				snapshot, err := model.ExportManagedCatalog(context.Background(), "dev")
				assert.Error(t, err, "read proof conservatively rejects stale true inheritance hints")
				assert.False(t, snapshot.Complete)
			} else {
				got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, "historic-hint-receipt", actor)
				assert.Error(t, err)
				assert.False(t, found)
				assert.Equal(t, catalogmanifest.Result{}, got)
			}
		})
	}
}

func TestCatalogSyncTargetHistoricalHintPolicyUnchanged(t *testing.T) {
	db, actor, plan := catalogSyncTarget(t)
	require.NoError(t, db.Exec("CREATE TABLE historical_vendors (LIKE vendors INCLUDING ALL)").Error)
	require.NoError(t, db.Exec("ALTER TABLE historical_vendors INHERIT vendors").Error)
	require.NoError(t, db.Exec("ALTER TABLE historical_vendors NO INHERIT vendors").Error)
	var hint bool
	require.NoError(t, db.Raw("SELECT relhassubclass FROM pg_catalog.pg_class WHERE oid = 'vendors'::regclass").Scan(&hint).Error)
	require.True(t, hint)
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "unchanged-target-hint-policy", actor)
	require.NoError(t, err, "new read-only proof restriction must not change the original target fence policy")
	assert.Equal(t, "unchanged-target-hint-policy", result.OperationID)
	var operation model.CatalogSyncOperation
	require.NoError(t, db.First(&operation, "id = ?", result.OperationID).Error)
	assert.Equal(t, "succeeded", operation.State)
}

// Preserve complete durable rows, including anchor, baseline, backup/history,
// incarnation and runtime ACK fields. Sequence allocation is not transactional.
func catalogSyncDurableRows(t *testing.T, db *gorm.DB, namespace string, tables []string) map[string]string {
	t.Helper()
	rows := make(map[string]string, len(tables))
	for _, table := range tables {
		var value string
		require.NoError(t, db.Raw(`SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY to_jsonb(r)::text), '[]'::jsonb)::text FROM "`+namespace+`"."`+table+`" r`).Scan(&value).Error)
		rows[table] = fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
	}
	return rows
}

func catalogSyncTargetShadow(t *testing.T, db *gorm.DB, empty bool) []string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw("SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public' ORDER BY tablename").Scan(&tables).Error)
	require.NotEmpty(t, tables)
	require.NoError(t, db.Exec("CREATE SCHEMA shadow").Error)
	for _, table := range tables {
		require.NoError(t, db.Exec(`CREATE TABLE shadow."`+table+`" (LIKE public."`+table+`" INCLUDING ALL)`).Error)
		if !empty || table != "models" && table != "vendors" {
			require.NoError(t, db.Exec(`INSERT INTO shadow."`+table+`" SELECT * FROM public."`+table+`"`).Error)
		}
	}
	return tables
}

func catalogSyncSwapTarget(t *testing.T, db *gorm.DB, restore bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	first, second := "ALTER SCHEMA public RENAME TO original", "ALTER SCHEMA shadow RENAME TO public"
	if restore {
		first, second = "ALTER SCHEMA public RENAME TO shadow", "ALTER SCHEMA original RENAME TO public"
	}
	require.NoError(t, db.WithContext(ctx).Transaction(func(ddl *gorm.DB) error {
		if err := ddl.Exec(first).Error; err != nil {
			return err
		}
		return ddl.Exec(second).Error
	}), "actual schema DDL must commit despite target relation locks")
}

func TestCatalogSyncTargetNamespaceOrdinaryRollback(t *testing.T) {
	for _, window := range []string{"preflight", "verified", "callback", "empty-callback"} {
		for _, aba := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/aba=%t", window, aba), func(t *testing.T) {
				db, _, _ := catalogSyncTarget(t)
				tables := catalogSyncTargetShadow(t, db, window == "empty-callback")
				var original, shadow map[string]string
				var swapped bool
				calls := 0
				generation := jsplugin.DefaultRegistry.Generation()
				swap := func() {
					original = catalogSyncDurableRows(t, db, "public", tables)
					shadow = catalogSyncDurableRows(t, db, "shadow", tables)
					catalogSyncSwapTarget(t, db, false)
					swapped = true
				}
				if window == "preflight" {
					require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("target-preflight-swap", func(tx *gorm.DB) {
						if !swapped && strings.HasPrefix(tx.Statement.SQL.String(), "SET LOCAL search_path") {
							swap()
							if aba {
								catalogSyncSwapTarget(t, db, true)
							}
						}
					}))
					defer db.Callback().Raw().Remove("target-preflight-swap")
				}
				if window == "verified" {
					require.NoError(t, db.Callback().Query().Before("gorm:query").Register("target-verified-swap", func(tx *gorm.DB) {
						if !swapped && tx.Statement.Table == "catalog_sync_states" {
							swap()
							if aba {
								catalogSyncSwapTarget(t, db, true)
							}
						}
					}))
					defer db.Callback().Query().Remove("target-verified-swap")
				}
				err := model.WithModelMetadataTransaction(func(tx *gorm.DB) error {
					calls++
					if strings.HasSuffix(window, "callback") {
						swap()
					}
					err := tx.Exec("INSERT INTO vendors (id, name, status, display_order) VALUES (990, 'namespace-write', 1, 0)").Error
					if strings.HasSuffix(window, "callback") && aba {
						catalogSyncSwapTarget(t, db, true)
					}
					return err
				})
				require.True(t, swapped)
				assert.Error(t, err, "namespace redirection must roll back before durable target commit")
				assert.NotErrorIs(t, err, model.ErrCatalogCommitUncertain)
				originalName, shadowName := "original", "public"
				if aba {
					originalName, shadowName = "public", "shadow"
				}
				assert.Equal(t, original, catalogSyncDurableRows(t, db, originalName, tables))
				assert.Equal(t, shadow, catalogSyncDurableRows(t, db, shadowName, tables))
				assert.Same(t, generation, jsplugin.DefaultRegistry.Generation())
				if strings.HasSuffix(window, "callback") {
					assert.Equal(t, 1, calls)
				} else {
					assert.Zero(t, calls)
				}
				if !aba {
					catalogSyncSwapTarget(t, db, true)
				}
				// Successful next ordinary save proves SQL locks, writer and pin drain.
				require.NoError(t, model.WithModelMetadataTransaction(func(tx *gorm.DB) error {
					return tx.Exec("UPDATE models SET display_name = 'after-rollback'").Error
				}))
				assert.NoError(t, jsplugin.DefaultRegistry.SetGenerationPreparer(nil))
			})
		}
	}
}

func TestCatalogSyncTargetNamespaceRelationReplacement(t *testing.T) {
	db, _, _ := catalogSyncTarget(t)
	before := catalogSyncDurableRows(t, db, "public", []string{"marketplace_order_locks", "catalog_sync_states", "vendors"})
	var replaced bool
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("target-replace-relation", func(tx *gorm.DB) {
		if replaced || !strings.HasPrefix(tx.Statement.SQL.String(), "SET LOCAL search_path") {
			return
		}
		replaced = true
		require.NoError(t, db.Transaction(func(ddl *gorm.DB) error {
			if err := ddl.Exec("ALTER TABLE vendors RENAME TO original_vendors").Error; err != nil {
				return err
			}
			return ddl.Exec("CREATE TABLE vendors (LIKE original_vendors INCLUDING ALL)").Error
		}))
	}))
	calls := 0
	err := model.WithModelMetadataTransaction(func(tx *gorm.DB) error {
		calls++
		return tx.Exec("INSERT INTO vendors (id, name) VALUES (991, 'replaced-relation')").Error
	})
	require.NoError(t, db.Callback().Raw().Remove("target-replace-relation"))
	require.True(t, replaced)
	assert.ErrorContains(t, err, "identity changed during acquisition")
	assert.Zero(t, calls)
	assert.Equal(t, before, catalogSyncDurableRows(t, db, "public", []string{"marketplace_order_locks", "catalog_sync_states", "vendors"}))
}

func TestCatalogSyncTargetNamespaceManagedRollback(t *testing.T) {
	for _, restore := range []bool{false, true} {
		for _, aba := range []bool{false, true} {
			t.Run(fmt.Sprintf("restore=%t/aba=%t", restore, aba), func(t *testing.T) {
				db, actor, plan := catalogSyncTarget(t)
				if restore {
					_, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "original-apply", actor)
					require.NoError(t, err)
					plan, err = model.CreateCatalogSyncRestorePlan(context.Background(), "original-apply", actor, time.Now())
					require.NoError(t, err)
					plan, err = model.ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
					require.NoError(t, err)
				}
				require.True(t, catalogmanifest.PlanExecutable(plan, time.Now()))
				tables := catalogSyncTargetShadow(t, db, false)
				var original, shadow map[string]string
				var wroteState, swapped bool
				operationReads, writes := 0, 0
				generation := jsplugin.DefaultRegistry.Generation()
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("target-mutation-start", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						writes++
						original = catalogSyncDurableRows(t, db, "public", tables)
						shadow = catalogSyncDurableRows(t, db, "shadow", tables)
					}
				}))
				require.NoError(t, db.Callback().Update().After("gorm:update").Register("target-state-written", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_states" && writes > 0 {
						wroteState = true
					}
				}))
				require.NoError(t, db.Callback().Query().After("gorm:query").Register("target-last-consumer", func(tx *gorm.DB) {
					if !wroteState || swapped || tx.Statement.Table != "catalog_sync_operations" {
						return
					}
					operationReads++
					if operationReads != 2 {
						return
					}
					catalogSyncSwapTarget(t, db, false)
					if aba {
						catalogSyncSwapTarget(t, db, true)
					}
					swapped = true
				}))
				result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "namespace-operation", actor)
				require.NoError(t, db.Callback().Create().Remove("target-mutation-start"))
				require.NoError(t, db.Callback().Update().Remove("target-state-written"))
				require.NoError(t, db.Callback().Query().Remove("target-last-consumer"))
				require.True(t, swapped, "real mutation reached its final receipt consumer")
				assert.Equal(t, 1, writes)
				assert.Error(t, err)
				assert.NotErrorIs(t, err, model.ErrCatalogCommitUncertain)
				assert.Equal(t, catalogmanifest.Result{}, result)
				originalName, shadowName := "original", "public"
				if aba {
					originalName, shadowName = "public", "shadow"
				}
				assert.Equal(t, original, catalogSyncDurableRows(t, db, originalName, tables))
				assert.Equal(t, shadow, catalogSyncDurableRows(t, db, shadowName, tables))
				assert.Same(t, generation, jsplugin.DefaultRegistry.Generation())
				if !aba {
					catalogSyncSwapTarget(t, db, true)
				}
				result, err = model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "after-namespace-rollback", actor)
				require.NoError(t, err)
				assert.Equal(t, "after-namespace-rollback", result.OperationID)
				operation, err := model.GetCatalogSyncOperation(context.Background(), result.OperationID)
				require.NoError(t, err)
				assert.Equal(t, "succeeded", operation.State)
			})
		}
	}
}

func TestCatalogSyncTargetNamespaceShadowAuthorization(t *testing.T) {
	for _, aba := range []bool{false, true} {
		t.Run(fmt.Sprintf("aba=%t", aba), func(t *testing.T) {
			db, actor, plan := catalogSyncTarget(t)
			tables := catalogSyncTargetShadow(t, db, false)
			require.NoError(t, db.Exec("UPDATE public.users SET role = ?", common.RoleAdminUser).Error)
			original := catalogSyncDurableRows(t, db, "public", tables)
			shadow := catalogSyncDurableRows(t, db, "shadow", tables)
			ran := catalogSyncBeforeQueryDDL(t, db, "users", "ALTER SCHEMA public RENAME TO original", "ALTER SCHEMA shadow RENAME TO public")
			var shadowSessionRead, restored bool
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("target-shadow-session", func(tx *gorm.DB) {
				if !shadowSessionRead && tx.Statement.Table == "user_sessions" {
					shadowSessionRead = tx.Error == nil
				}
				if aba && shadowSessionRead && !restored && tx.Statement.Table == "users" {
					catalogSyncSwapTarget(t, db, true)
					restored = true
				}
			}))
			result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "shadow-authorization", actor)
			require.NoError(t, db.Callback().Query().Remove("target-shadow-session"))
			require.True(t, ran.Load())
			require.True(t, shadowSessionRead, "actual transient shadow root/session authorization was read")
			assert.ErrorContains(t, err, "namespace changed before commit")
			assert.Equal(t, catalogmanifest.Result{}, result)
			originalName, shadowName := "original", "public"
			if aba {
				originalName, shadowName = "public", "shadow"
			}
			assert.Equal(t, original, catalogSyncDurableRows(t, db, originalName, tables))
			assert.Equal(t, shadow, catalogSyncDurableRows(t, db, shadowName, tables))
		})
	}
}

// Lend one actual native PG connection to the fault driver; production still
// receives a real *sql.DB and owns its root transaction and reserved connection.
func catalogSyncObserveTarget(t *testing.T, db *gorm.DB, run func(*catalogReadLostAckConnection)) {
	t.Helper()
	pool, err := db.DB()
	require.NoError(t, err)
	loan, err := pool.Conn(context.Background())
	require.NoError(t, err)
	defer loan.Close()
	require.NoError(t, loan.Raw(func(native any) error {
		connection := &catalogReadLostAckConnection{Conn: native.(driver.Conn), afterCommit: func() error { return nil }}
		observed := sql.OpenDB(catalogReadLostAckConnector{connection: connection, driver: pool.Driver()})
		observed.SetMaxOpenConns(1)
		defer observed.Close()
		root := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
		root.Statement.ConnPool = observed
		model.DB = root
		defer func() { model.DB = db }()
		run(connection)
		return nil
	}))
}

func TestCatalogSyncTargetNamespaceProofFailures(t *testing.T) {
	for _, failure := range []string{"sql-error", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			db, actor, plan := catalogSyncTarget(t)
			var tables []string
			require.NoError(t, db.Raw("SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname = 'public' ORDER BY tablename").Scan(&tables).Error)
			var before map[string]string
			var callbackPID, callbackXID int64
			var mutation bool
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("target-proof-mutation", func(tx *gorm.DB) {
				if tx.Statement.Table == "catalog_sync_operations" {
					before = catalogSyncDurableRows(t, db, "public", tables)
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT pg_backend_pid(), txid_current()").Row().Scan(&callbackPID, &callbackXID))
					mutation = true
				}
			}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var proofObserved bool
			catalogSyncObserveTarget(t, db, func(connection *catalogReadLostAckConnection) {
				connection.beforeSQL = func(queryCtx context.Context, query string) error {
					if !mutation || !strings.Contains(query, "FROM pg_catalog.pg_namespace n WHERE n.oid") {
						return nil
					}
					proofObserved = true
					rows, err := connection.Conn.(driver.QueryerContext).QueryContext(queryCtx, "SELECT pg_backend_pid(), txid_current(), current_setting('transaction_isolation')", nil)
					require.NoError(t, err)
					values := make([]driver.Value, 3)
					require.NoError(t, rows.Next(values))
					require.NoError(t, rows.Close())
					assert.EqualValues(t, callbackPID, values[0])
					assert.EqualValues(t, callbackXID, values[1], "final proof runs in the same mutation transaction")
					assert.Equal(t, "read committed", values[2])
					if failure == "cancel" {
						cancel()
						return queryCtx.Err()
					}
					_, err = connection.Conn.(driver.ExecerContext).ExecContext(queryCtx, "SELECT 1/0", nil)
					return err // actual PostgreSQL SQL failure aborts the transaction
				}
				result, err := model.ApplyCatalogSyncPlan(ctx, plan.ID, plan.Digest, "proof-failure", actor)
				require.True(t, proofObserved)
				assert.Error(t, err)
				if failure == "cancel" {
					assert.ErrorIs(t, err, context.Canceled)
				} else {
					assert.ErrorContains(t, err, "division by zero")
				}
				assert.NotErrorIs(t, err, model.ErrCatalogCommitUncertain)
				assert.Equal(t, catalogmanifest.Result{}, result)
				assert.Equal(t, int64(1), connection.commits.Load(), "only the earlier capture transaction committed")
			})
			require.NoError(t, db.Callback().Create().Remove("target-proof-mutation"))
			assert.Equal(t, before, catalogSyncDurableRows(t, db, "public", tables))
			_, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "after-proof-failure", actor)
			require.NoError(t, err, "rollback drains locks and permits the next normal Apply")
			require.NoError(t, jsplugin.DefaultRegistry.SetGenerationPreparer(nil), "registry write proves no leaked pin")
		})
	}
}

func TestCatalogSyncTargetNamespaceLostCommitAck(t *testing.T) {
	for _, managed := range []bool{false, true} {
		t.Run(fmt.Sprintf("managed=%t", managed), func(t *testing.T) {
			db, actor, plan := catalogSyncTarget(t)
			writes, proofs := 0, 0
			lastQuery := ""
			var result catalogmanifest.Result
			var applyErr error
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("target-once-write", func(tx *gorm.DB) {
				if tx.Statement.Table == "catalog_sync_operations" {
					writes++
				}
			}))
			catalogSyncObserveTarget(t, db, func(connection *catalogReadLostAckConnection) {
				connection.beforeSQL = func(_ context.Context, query string) error {
					lastQuery = query
					if strings.Contains(query, "FROM pg_catalog.pg_namespace n WHERE n.oid") {
						proofs++
					}
					return nil
				}
				connection.afterCommit = func() error {
					assert.Contains(t, lastQuery, "FROM pg_catalog.pg_namespace n WHERE n.oid", "proof must be the final SQL statement before each native target commit")
					if connection.commits.Load() == 2 {
						return errors.New("injected lost target COMMIT acknowledgement")
					}
					return nil
				}
				if managed {
					result, applyErr = model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "lost-target-ack", actor)
					require.NoError(t, applyErr, "managed operation resolves the actual durable exact receipt")
				} else {
					applyErr = model.WithModelMetadataTransaction(func(tx *gorm.DB) error {
						writes++
						return tx.Exec("UPDATE models SET display_name = 'durable-ordinary'").Error
					})
					assert.ErrorIs(t, applyErr, model.ErrCatalogCommitUncertain, "ordinary mutation retains uncertainty and never replays")
					assert.ErrorContains(t, applyErr, "lost target COMMIT")
				}
				assert.GreaterOrEqual(t, proofs, 2)
			})
			require.NoError(t, db.Callback().Create().Remove("target-once-write"))
			assert.Equal(t, 1, writes)
			if managed {
				got, found, err := model.LookupCatalogSyncOperationResult(context.Background(), plan.ID, plan.Digest, result.OperationID, actor)
				require.NoError(t, err)
				require.True(t, found)
				assert.Equal(t, result, got)
				var count int64
				require.NoError(t, db.Model(&model.CatalogSyncOperation{}).Count(&count).Error)
				assert.Equal(t, int64(1), count)
			} else {
				var name string
				require.NoError(t, db.Model(&model.Model{}).Select("display_name").Scan(&name).Error)
				assert.Equal(t, "durable-ordinary", name)
				var state model.CatalogSyncState
				require.NoError(t, db.First(&state).Error)
				assert.Equal(t, "committed_pending_publish", state.PublicationState)
				assert.Equal(t, int64(0), state.RuntimeRevision)
				require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
			}
		})
	}
}
