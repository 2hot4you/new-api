package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A successful real boot must publish the committed catalog before returning,
// including when the optional managed-sync feature has never been enabled.
func TestCatalogInitResourcesDefaultDisabled(t *testing.T) {
	catalogInitResourcesProcesses(t, "ordinary")
}

func TestCatalogInitResourcesPending(t *testing.T) {
	catalogInitResourcesProcesses(t, "pending")
}

func TestCatalogInitResourcesPlugin(t *testing.T)        { catalogInitResourcesProcesses(t, "plugin") }
func TestCatalogInitResourcesPartialPlugin(t *testing.T) { catalogInitResourcesProcesses(t, "partial") }
func TestCatalogInitResourcesRebuildFailure(t *testing.T) {
	catalogInitResourcesProcesses(t, "pending-rebuild")
}
func TestCatalogInitResourcesPermissionMissing(t *testing.T) {
	catalogInitResourcesProcesses(t, "pending-disabled")
}
func TestCatalogInitResourcesUpgradeRepeat(t *testing.T) { catalogInitResourcesProcesses(t, "upgrade") }
func TestCatalogInitResourcesNonMaster(t *testing.T)     { catalogInitResourcesProcesses(t, "slave") }
func TestCatalogInitResourcesCorruptSource(t *testing.T) { catalogInitResourcesProcesses(t, "corrupt") }

const catalogBootPluginSource = `
export const meta = {apiVersion:1, key:"boot-plugin", name:"Boot plugin", version:"1.0.0", author:{name:"Test"}, models:["boot-plugin-model"], usageSchema:{seconds:{type:"number",unit:"second"}}, fetchMode:"per_task"};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
`

func catalogInitResourcesProcesses(t *testing.T, mode string) {
	t.Helper()
	pending := strings.HasPrefix(mode, "pending")
	if os.Getenv("CATALOG_STARTUP_TEST_CHILD") == "true" {
		first := os.Getenv("CATALOG_STARTUP_TEST_STAGE") == "0"
		if !first && mode == "partial" {
			_, err := jsplugin.DefaultRegistry.Register(catalogBootPluginSource, jsplugin.Options{})
			require.NoError(t, err)
		}
		err := InitResources()
		if !first && (mode == "partial" || mode == "corrupt" || mode == "pending-rebuild" || mode == "pending-disabled") {
			require.Error(t, err, "unsafe startup must fail before serving")
			require.Error(t, model.WithCatalogPricingRead(context.Background(), "boot-model", func() error { t.Error("failed boot selected a price"); return nil }))
			if pending {
				var operation model.CatalogSyncOperation
				require.NoError(t, model.DB.First(&operation, "id = ?", "boot-operation").Error)
				assert.Equal(t, "committed_pending_publish", operation.State)
			} else if mode == "partial" {
				_, retained := jsplugin.DefaultRegistry.Get("boot-plugin")
				assert.True(t, retained, "an incumbent does not establish desired generation readiness")
			}
			sqlDB, dbErr := model.DB.DB()
			require.NoError(t, dbErr)
			require.NoError(t, sqlDB.Close())
			return
		}
		require.NoError(t, err)
		require.NoError(t, model.WithCatalogPricingRead(context.Background(), "startup-probe", func() error { return nil }))
		if pending {
			if first {
				catalogStartupCommitPending(t)
				if mode == "pending-rebuild" {
					require.NoError(t, model.DB.Create(&model.Channel{Name: "invalid-cache", Key: "test", Type: constant.ChannelTypeAdvancedCustom, Status: common.ChannelStatusEnabled, Models: "boot-model", Group: "default", OtherSettings: "{"}).Error)
				}
			} else {
				var operation model.CatalogSyncOperation
				require.NoError(t, model.DB.First(&operation, "id = ?", "boot-operation").Error)
				assert.Equal(t, "succeeded", operation.State)
				var result catalogmanifest.Result
				require.NoError(t, common.UnmarshalJsonStr(string(operation.Result), &result))
				assert.Equal(t, "committed_pending_publish", result.State)
				price, ok := ratio_setting.GetModelPrice("boot-model", false)
				assert.True(t, ok)
				assert.Equal(t, 7.0, price)
			}
		}
		if mode == "plugin" || mode == "partial" || mode == "corrupt" {
			if first {
				source := catalogBootPluginSource
				if mode == "partial" {
					source = "invalid javascript {"
				}
				require.NoError(t, model.SaveTaskPlugin(&model.TaskPlugin{Key: "boot-plugin", APIVersion: 1, Version: "1.0.0", Source: model.LongText(source), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source))), Enabled: true}))
				if mode == "corrupt" {
					require.NoError(t, model.DB.Model(&model.TaskPlugin{}).Where("key = ?", "boot-plugin").Update("source_hash", strings.Repeat("0", 64)).Error)
				}
				require.NoError(t, model.DB.Create(&model.Model{ModelName: "boot-plugin-model", BillingCurrency: "CNY"}).Error)
				require.NoError(t, model.DB.Create(&[]model.Option{{Key: "TaskPluginEnabled", Value: "true"}, {Key: "billing_setting.billing_mode", Value: `{"boot-plugin-model":"tiered_expr"}`}, {Key: "billing_setting.billing_expr", Value: `{"boot-plugin-model":"u(\"seconds\") * 0.4"}`}}).Error)
			} else {
				plugin, ok := jsplugin.DefaultRegistry.Get("boot-plugin")
				require.True(t, ok)
				assert.Equal(t, "1.0.0", plugin.Meta.Version)
				expression, ok := billing_setting.GetBillingExpr("boot-plugin-model")
				assert.True(t, ok)
				assert.Equal(t, `u("seconds") * 0.4`, expression)
			}
		}
		if mode == "upgrade" || mode == "slave" {
			if first {
				require.NoError(t, model.DB.Where("key IN ?", []string{"migration.model_billing_currency.v1", "migration.model_billing_currency.v2"}).Delete(&model.Option{}).Error)
				require.NoError(t, model.DB.Create(&model.Model{ModelName: "doubao-seedance-2-5-260628", BillingCurrency: "USD", MaxInputImages: 9, SyncOfficial: 1}).Error)
			} else {
				var entry model.Model
				require.NoError(t, model.DB.Where("model_name = ?", "doubao-seedance-2-5-260628").First(&entry).Error)
				if mode == "slave" {
					assert.Equal(t, "USD", entry.BillingCurrency)
					assert.Equal(t, 9, entry.MaxInputImages)
					assert.Zero(t, entry.DisplayOrder)
				} else {
					assert.Equal(t, "CNY", entry.BillingCurrency)
					assert.Equal(t, 30, entry.MaxInputImages)
					assert.Positive(t, entry.DisplayOrder)
					assert.NotEmpty(t, entry.DisplayName)
					var markers int64
					require.NoError(t, model.DB.Model(&model.Option{}).Where("key IN ?", []string{"migration.model_billing_currency.v1", "migration.model_billing_currency.v2"}).Count(&markers).Error)
					assert.EqualValues(t, 2, markers)
				}
			}
		}
		require.NoError(t, model.CloseDB())
		return
	}
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	require.NotEmpty(t, dsn, "task PostgreSQL is required")
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	var version string
	require.NoError(t, admin.Raw("SELECT version()").Scan(&version).Error)
	t.Log(version)
	name := fmt.Sprintf("catalog_startup_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
	defer func() { require.NoError(t, admin.Exec(`DROP DATABASE "`+name+`"`).Error) }()
	u.Path = "/" + name
	stages := 2
	if mode == "upgrade" {
		stages = 3
	}
	for stage := range stages {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v", "--log-dir="+t.TempDir())
		cmd.Dir = t.TempDir() // Never load a workspace .env or write its logs.
		cmd.Env = append(os.Environ(), "CATALOG_STARTUP_TEST_CHILD=true", "SQL_DSN="+u.String(), "LOG_SQL_DSN=", "REDIS_CONN_STRING=", "NODE_TYPE=master", "NODE_NAME=catalog-startup-test", "CATALOG_SYNC_ROLE=disabled", "CATALOG_SYNC_SINGLE_INSTANCE=false", "MEMORY_CACHE_ENABLED=false")
		cmd.Env = append(cmd.Env, fmt.Sprintf("CATALOG_STARTUP_TEST_STAGE=%d", stage))
		if pending {
			cmd.Env = append(cmd.Env, "CATALOG_SYNC_SINGLE_INSTANCE=true")
		}
		if stage > 0 && mode == "pending-disabled" {
			cmd.Env = append(cmd.Env, "CATALOG_SYNC_SINGLE_INSTANCE=false")
		}
		if mode == "pending-rebuild" {
			cmd.Env = append(cmd.Env, "MEMORY_CACHE_ENABLED=true")
		}
		if stage > 0 && mode == "slave" {
			cmd.Env = append(cmd.Env, "NODE_TYPE=slave")
		}
		output, runErr := cmd.CombinedOutput()
		cancel()
		require.NoError(t, runErr, "%s", output)
	}
}

func catalogStartupCommitPending(t *testing.T) {
	t.Helper()
	require.NoError(t, service.ReportCurrentSystemInstance())
	user := model.User{Username: "catalog-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.UserSession{SID: "boot-session", UserID: user.Id, UserAuthVersion: 1, Version: 1, Status: model.UserSessionStatusActive, ExpiresAt: time.Now().Add(time.Hour).Unix(), RefreshHash: strings.Repeat("a", 64), LoginMethod: "password"}).Error)
	actor := catalogmanifest.Actor{UserID: user.Id, SessionID: "boot-session", TargetID: "target", AuthVersion: 1, SessionVersion: 1}
	source := catalogmanifest.Snapshot{SchemaVersion: 2, SourceID: "dev", Complete: true, ExportedAt: 42, Capabilities: catalogmanifest.RequiredCapabilities(), Coverage: map[string]int{}}
	for _, kind := range catalogmanifest.Kinds() {
		source.Coverage[kind] = 0
	}
	value, err := common.Marshal(catalogmanifest.ModelValue{ModelName: "boot-model", BillingCurrency: "USD"})
	require.NoError(t, err)
	key, err := catalogmanifest.EncodePriceKey(catalogmanifest.PriceKey{Option: "ModelPrice", Model: "boot-model", Path: "/boot-model"})
	require.NoError(t, err)
	price, err := common.Marshal(catalogmanifest.PriceValue{Value: "7", BillingCurrency: "USD", Unit: "request"})
	require.NoError(t, err)
	source.Entries = []catalogmanifest.Entry{{Kind: "model", Key: "boot-model", Value: string(value)}, {Kind: "model_price", Key: key, Value: string(price)}}
	source.Coverage["model"], source.Coverage["model_price"] = 1, 1
	source.Digest, err = catalogmanifest.SnapshotDigest(source)
	require.NoError(t, err)
	plan, err := model.CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
	require.NoError(t, err)
	injected := errors.New("startup fixture required rebuild failure")
	require.NoError(t, model.DB.Callback().Query().Before("gorm:query").Register("boot-rebuild-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "vendors" && tx.Statement.ConnPool == model.DB.Statement.ConnPool {
			tx.AddError(injected)
		}
	}))
	result, err := model.ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "boot-operation", actor)
	require.ErrorIs(t, err, injected)
	require.NoError(t, model.DB.Callback().Query().Remove("boot-rebuild-failure"))
	assert.Equal(t, "committed_pending_publish", result.State)
	var state model.CatalogSyncState
	require.NoError(t, model.DB.First(&state, model.CatalogSyncStateID).Error)
	assert.Equal(t, "boot-operation", state.PendingOperationID)
	assert.Equal(t, "committed_pending_publish", state.PublicationState)
	require.NoError(t, model.InitOptionMapBootstrap(context.Background()))
	var afterBootstrap model.CatalogSyncState
	require.NoError(t, model.DB.First(&afterBootstrap, model.CatalogSyncStateID).Error)
	assert.Equal(t, state, afterBootstrap, "startup prerequisite loading must not publish or ACK catalog rows")
	require.Error(t, model.WithCatalogPricingRead(context.Background(), "boot-model", func() error { t.Error("bootstrap bypassed pending publication"); return nil }))
}

func TestInitializeChannelCacheAtStartupAcceptsMultipleStarAIChannelsWithMemoryCacheDisabled(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}, &model.Option{}))
	require.NoError(t, model.MigrateCatalogSync(db))
	model.DB = db
	for _, name := range []string{"first", "second"} {
		require.NoError(t, db.Create(&model.Channel{
			Type:   constant.ChannelTypeStarAI,
			Status: common.ChannelStatusEnabled,
			Name:   name,
			Models: "doubao-seedance-2-0-260128",
			Group:  "default",
		}).Error)
	}

	var output bytes.Buffer
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		common.LogWriterMu.Unlock()
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.NotPanics(t, initializeChannelCacheAtStartup)
	assert.NotContains(t, output.String(), "channels synced from database")
}

func TestInitializeChannelCacheAtStartupCreatesLocalMetadataDraftWithoutMemoryCache(t *testing.T) {
	previousDB := model.DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}, &model.Option{}))
	require.NoError(t, model.MigrateCatalogSync(db))
	channel := model.Channel{Status: common.ChannelStatusEnabled, Name: "catalog", Key: "test", Models: "glm-5.2", Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "glm-5.2", ChannelId: channel.Id, Enabled: true}).Error)
	model.DB = db

	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
	})

	initializeChannelCacheAtStartup()

	var entry model.Model
	require.NoError(t, db.Where("model_name = ?", "glm-5.2").First(&entry).Error)
	assert.Equal(t, "glm-5.2", entry.DisplayName)
	assert.Zero(t, entry.ContextLength)
	assert.Zero(t, entry.VendorID)
	assert.Empty(t, entry.Description)
	assert.False(t, entry.MarketplaceEnabled)
}
