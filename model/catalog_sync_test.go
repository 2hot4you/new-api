package model

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func TestCatalogSyncRestoreScope(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			original, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "restore-original", actor)
			require.NoError(t, err)
			_, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err, "automatic publication permits a real inverse")
			catalogBusinessFixturePublished(t, db, original)
			require.NoError(t, db.Create(&Vendor{Name: "later-unrelated", Description: "keep"}).Error)
			require.NoError(t, db.Create(&Option{Key: "WebSearchPrice", Value: `{"later":4.25}`}).Error)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err, "latest published operation must have a server-created inverse")
			assert.Equal(t, "restore", restore.Kind)
			assert.Equal(t, original.OperationID, restore.RestoreOperationID)
			assert.False(t, catalogmanifest.PlanExecutable(restore, time.Now()), "deletion needs separate consent")
			restore, err = ResolveCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			result, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "inverse", actor)
			require.NoError(t, err)
			assert.Equal(t, "committed_pending_publish", result.State)
			assert.Equal(t, int64(2), result.Revision)
			var vendors []Vendor
			require.NoError(t, db.Find(&vendors).Error)
			require.Len(t, vendors, 1)
			assert.Equal(t, "later-unrelated", vendors[0].Name)
			var option Option
			require.NoError(t, db.Where(map[string]any{"key": "WebSearchPrice"}).First(&option).Error)
			assert.JSONEq(t, `{"later":4.25}`, option.Value)
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			assert.Equal(t, int64(2), state.BaselineGeneration)
			assert.Empty(t, state.SourceID)
			var count int64
			require.NoError(t, db.Model(&CatalogSyncBaseline{}).Count(&count).Error)
			assert.Zero(t, count)
			replayed, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "inverse", actor)
			require.NoError(t, err)
			assert.Equal(t, result, replayed)
			catalogBusinessFixturePublished(t, db, result)
			_, err = CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err, "empty restored baseline must permit a later sync")
			_, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.Error(t, err, "a later succeeded inverse supersedes the original")
		})
	}
}

func TestCatalogSyncRestoreGuards(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogSyncTestSource(t)
			require.NoError(t, appendManagedEntry(&source, catalogmanifest.KindModel, "created-model", catalogmanifest.ModelValue{ModelName: "created-model", BillingCurrency: "USD"}))
			var err error
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			original, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "guard-original", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, original)
			var vendor Vendor
			require.NoError(t, db.First(&vendor, "name = ?", "source-vendor").Error)
			local := Model{ModelName: "local-dependent", VendorID: vendor.Id, BillingCurrency: "USD"}
			require.NoError(t, db.Create(&local).Error)
			channel := Channel{Key: "PRIVATE-RESTORE-CHANNEL", Status: common.ChannelStatusEnabled, Models: "created-model"}
			require.NoError(t, db.Create(&channel).Error)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err)
			assert.False(t, catalogmanifest.PlanExecutable(restore, time.Now()))
			var blocked, routeBlocked bool
			for _, change := range restore.Changes {
				blocked = blocked || change.Reason == "vendor_still_referenced"
				routeBlocked = routeBlocked || change.Reason == "enabled_route_reference"
			}
			require.True(t, blocked, "inverse vendor deletion must see later target-only models")
			require.True(t, routeBlocked, "inverse model deletion must check current enabled routes")
			public, err := common.Marshal(restore)
			require.NoError(t, err)
			assert.NotContains(t, string(public), channel.Key)
			require.NoError(t, db.Delete(&local).Error)
			require.NoError(t, db.Delete(&channel).Error)
			restore, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err)
			restore, err = ResolveCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			// Fixture-only publication state proves the commit-time latest check
			// independently of target/baseline version changes.
			later := CatalogSyncOperation{ID: "guard-later", PlanID: "guard-later-plan", State: "succeeded", Revision: original.Revision + 1, Backup: "{}", History: "{}", Result: "{}"}
			require.NoError(t, db.Create(&later).Error)
			_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "guard-inverse", actor)
			require.ErrorIs(t, err, ErrCatalogSyncPlanUnavailable)
			require.NoError(t, db.Delete(&later).Error)
			require.NoError(t, db.Model(&vendor).Update("description", "related-edit").Error)
			_, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "guard-inverse", actor)
			require.Error(t, err)
			require.NoError(t, db.Model(&vendor).Update("description", "original").Error)
			require.NoError(t, db.Delete(&vendor).Error)
			vendor.Id = 0
			vendor.DeletedAt = gorm.DeletedAt{}
			require.NoError(t, db.Create(&vendor).Error)
			_, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale, "identical recreated vendor is not the original object")
		})
	}
}

func TestCatalogSyncRestorePreimages(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogBusinessPriceSource(t, db)
			var oldModel Model
			require.NoError(t, db.First(&oldModel, "model_name = ?", "managed-model").Error)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			original, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "price-original", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, original)
			require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": "billing_setting.billing_expr"}).Update("value", `{"managed-model":"tier(\"base\", p * 7.123 + c * 3)","local-model":"p * 19"}`).Error)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err)
			result, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "price-inverse", actor)
			require.NoError(t, err)
			var option Option
			require.NoError(t, db.Where(map[string]any{"key": "billing_setting.billing_expr"}).First(&option).Error)
			assert.JSONEq(t, `{"managed-model":"p * 2","local-model":"p * 19"}`, option.Value)
			var model Model
			require.NoError(t, db.First(&model, "model_name = ?", "managed-model").Error)
			assert.Equal(t, oldModel.Id, model.Id, "adoption-only metadata is not deleted/recreated")
			assert.Equal(t, oldModel.CreatedTime, model.CreatedTime)
			assert.Equal(t, "CNY", model.BillingCurrency)
			var operation CatalogSyncOperation
			require.NoError(t, db.First(&operation, "id = ?", result.OperationID).Error)
			var backup catalogOperationBackup
			require.NoError(t, common.UnmarshalJsonStr(string(operation.Backup), &backup))
			assert.Equal(t, int64(1), backup.Baseline.Generation)
			assert.NotEmpty(t, backup.Before)
			assert.NotEmpty(t, backup.After)
		})
	}
}

func TestCatalogSyncRestoreMissingAndOwnership(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			require.NoError(t, db.Create(&Model{ModelName: "priced", BillingCurrency: "USD"}).Error)
			require.NoError(t, db.Create(&Option{Key: "ModelRatio", Value: `{"priced":2.75}`}).Error)
			source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			first, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "ownership-first", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, first)
			require.NoError(t, db.Where(map[string]any{"key": "ModelRatio"}).Delete(&Option{}).Error)
			source.Entries = slices.DeleteFunc(source.Entries, func(entry catalogmanifest.Entry) bool {
				if entry.Kind == catalogmanifest.KindModelPrice {
					source.Coverage[entry.Kind]--
					return true
				}
				return false
			})
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			second, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "ownership-second", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, second)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), second.OperationID, actor, time.Now())
			require.NoError(t, err)
			_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "ownership-inverse", actor)
			require.NoError(t, err)
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": "ModelRatio"}).Count(&count).Error)
			assert.Zero(t, count, "a baseline value must never recreate an absent local price or container")
			var base []CatalogSyncBaseline
			require.NoError(t, db.Find(&base).Error)
			require.Len(t, base, 2, "prior managed price ownership is restored separately")
			for _, row := range base {
				assert.Equal(t, int64(3), row.Generation)
			}
		})
	}
}

func TestCatalogSyncRestoreBaselineIncarnations(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogSyncTestSource(t)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			first, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "inc-first", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, first)
			var prior CatalogSyncBaseline
			require.NoError(t, db.First(&prior).Error)
			var vendor Vendor
			require.NoError(t, db.First(&vendor).Error)
			require.NoError(t, db.Delete(&vendor).Error)
			vendor.Id, vendor.DeletedAt = 0, gorm.DeletedAt{}
			require.NoError(t, db.Create(&vendor).Error)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			plan, err = ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{OverwriteKeys: []string{catalogmanifest.EntryID(source.Entries[0])}})
			require.NoError(t, err)
			second, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "inc-second", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, second)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), second.OperationID, actor, time.Now())
			require.NoError(t, err)
			_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "inc-inverse", actor)
			require.NoError(t, err)
			var restored CatalogSyncBaseline
			require.NoError(t, db.First(&restored).Error)
			assert.Equal(t, prior.ObjectVersion, restored.ObjectVersion, "undo adoption must not grant prior ownership to an already recreated preimage")
			assert.Equal(t, int64(3), restored.Generation)
		})
	}
}

func TestCatalogSyncRestoreAuthorizationAndRollback(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			original, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "auth-original", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, original)
			// A new interactive session can independently authorize the restore.
			require.NoError(t, db.Model(&UserSession{}).Where("sid = ?", actor.SessionID).Update("revoked_at", 1).Error)
			_, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.Error(t, err)
			actor.SessionID = "new-restore-session"
			require.NoError(t, db.Create(&UserSession{SID: actor.SessionID, UserID: actor.UserID, UserAuthVersion: 1, Version: 1, Status: UserSessionStatusActive, ExpiresAt: time.Now().Add(time.Hour).Unix(), RefreshHash: strings.Repeat("b", 64), LoginMethod: "password"}).Error)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err)
			restore, err = ResolveCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			for _, mutation := range []struct {
				entity       any
				key, field   string
				value, reset any
			}{
				{&User{}, "id", "role", common.RoleAdminUser, common.RoleRootUser},
				{&UserSession{}, "sid", "version", 2, 1},
				{&UserSession{}, "sid", "revoked_at", 1, 0},
			} {
				identity := any(actor.SessionID)
				if mutation.key == "id" {
					identity = actor.UserID
				}
				require.NoError(t, db.Model(mutation.entity).Where(mutation.key+" = ?", identity).Update(mutation.field, mutation.value).Error)
				_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "auth-inverse", actor)
				require.Error(t, err)
				require.NoError(t, db.Model(mutation.entity).Where(mutation.key+" = ?", identity).Update(mutation.field, mutation.reset).Error)
			}
			var stored CatalogSyncPlan
			require.NoError(t, db.First(&stored, "id = ?", restore.ID).Error)
			for _, mutate := range []func(*catalogmanifest.Plan){
				func(p *catalogmanifest.Plan) { p.Kind = "sync"; p.RestoreOperationID = "" },
				func(p *catalogmanifest.Plan) { p.RestoreOperationID = "other-original" },
				func(p *catalogmanifest.Plan) { p.ExpiresAt = time.Now().Add(-time.Hour).Unix() },
			} {
				changed := restore
				mutate(&changed)
				body, err := common.Marshal(changed)
				require.NoError(t, err)
				require.NoError(t, db.Model(&CatalogSyncPlan{}).Where("id = ?", restore.ID).Update("body", CatalogSyncText(body)).Error)
				_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "auth-inverse", actor)
				require.Error(t, err)
			}
			require.NoError(t, db.Model(&CatalogSyncPlan{}).Where("id = ?", restore.ID).Update("body", stored.Body).Error)
			for _, failure := range []string{"create-backup", "final-backup"} {
				inject := func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						tx.AddError(errors.New("injected restore backup failure"))
					}
				}
				if failure == "create-backup" {
					require.NoError(t, db.Callback().Create().Before("gorm:create").Register("restore-backup-fail", inject))
				} else {
					require.NoError(t, db.Callback().Update().Before("gorm:update").Register("restore-backup-fail", inject))
				}
				_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "auth-inverse", actor)
				require.ErrorContains(t, err, "injected restore backup failure")
				if failure == "create-backup" {
					require.NoError(t, db.Callback().Create().Remove("restore-backup-fail"))
				} else {
					require.NoError(t, db.Callback().Update().Remove("restore-backup-fail"))
				}
				var count int64
				require.NoError(t, db.Model(&Vendor{}).Count(&count).Error)
				assert.Equal(t, int64(1), count)
				require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
				assert.Equal(t, int64(1), count)
				var state CatalogSyncState
				require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, int64(1), state.Revision)
				assert.Equal(t, int64(1), state.BaselineGeneration)
			}
			result, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "auth-inverse", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, result)
			replayed, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "auth-inverse", actor)
			require.NoError(t, err)
			assert.Equal(t, result, replayed, "replay keeps original pending result even after row succeeds")
			_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest+"bad", "auth-inverse", actor)
			require.ErrorIs(t, err, ErrCatalogSyncOperationConflict)
			_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "different-op", actor)
			require.ErrorIs(t, err, ErrCatalogSyncOperationConflict)
			rows, total, err := ListCatalogSyncOperations(context.Background(), 0, 0)
			require.NoError(t, err)
			require.Len(t, rows, 2)
			assert.Equal(t, int64(2), total)
			assert.Equal(t, "succeeded", rows[0].State)
			require.NotNil(t, rows[0].Summary)
			assert.Equal(t, "restore", rows[0].Summary.Kind)
			assert.Equal(t, "dev", rows[0].Summary.SourceID)
			assert.Equal(t, original.OperationID, rows[0].Summary.RestoreOperationID)
			for _, row := range rows {
				assert.Empty(t, row.Backup)
				assert.Empty(t, row.History)
				assert.Empty(t, row.Result)
			}
			public, err := common.Marshal(rows)
			require.NoError(t, err)
			assert.NotContains(t, string(public), actor.SessionID)
			assert.NotContains(t, string(public), "catalog-session")
			assert.NotContains(t, string(public), "RefreshHash")
		})
	}
}

func TestCatalogSyncRestoreLatestAndFallback(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			require.NoError(t, db.Create(&Model{ModelName: "priced", BillingCurrency: "USD"}).Error)
			source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
			require.NoError(t, err)
			key, err := catalogmanifest.EncodePriceKey(catalogmanifest.PriceKey{Option: "ModelRatio", Model: "priced", Path: "/priced"})
			require.NoError(t, err)
			require.NoError(t, appendManagedEntry(&source, catalogmanifest.KindModelPrice, key, catalogmanifest.PriceValue{Value: "3.25", BillingCurrency: "USD", Unit: "legacy_ratio"}))
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			original, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "latest-original", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, original)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err)
			var blocked bool
			for _, change := range restore.Changes {
				blocked = blocked || change.Reason == "price_fallback_unproven"
			}
			require.True(t, blocked, "inverse price removal must retain the existing fail-closed fallback policy")
			restore, err = ResolveCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "unsafe-price-inverse", actor)
			require.Error(t, err)
			for _, state := range []string{"failed", "committed_pending_publish"} {
				require.NoError(t, db.Model(&CatalogSyncOperation{}).Where("id = ?", original.OperationID).Update("state", state).Error)
				_, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
				require.Error(t, err)
			}
			require.NoError(t, db.Model(&CatalogSyncOperation{}).Where("id = ?", original.OperationID).Update("state", "succeeded").Error)
			// Fixture-only later states isolate latest-success checking from the
			// independent target revision gate; no production publisher is used.
			later := CatalogSyncOperation{ID: "later-failed", PlanID: "fixture-plan", State: "failed", Revision: original.Revision + 1, Backup: "{}", History: "{}", Result: "{}"}
			require.NoError(t, db.Create(&later).Error)
			_, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err)
			require.NoError(t, db.Model(&later).Update("state", "succeeded").Error)
			_, err = CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.ErrorIs(t, err, ErrCatalogSyncPlanUnavailable)
			_, err = GetCatalogSyncPlan(context.Background(), restore.ID, actor)
			require.ErrorIs(t, err, ErrCatalogSyncPlanUnavailable)
		})
	}
}

func TestCatalogSyncRestoreDeletedObject(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogSyncTestSource(t)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			first, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "deleted-first", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, first)
			var prior Vendor
			require.NoError(t, db.First(&prior).Error)
			source.Entries = nil
			source.Coverage[catalogmanifest.KindVendor] = 0
			require.NoError(t, appendManagedEntry(&source, catalogmanifest.KindVendor, "other-vendor", catalogmanifest.VendorValue{Name: "other-vendor", Status: 1}))
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			plan, err = ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			second, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "deleted-second", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, second)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), second.OperationID, actor, time.Now())
			require.NoError(t, err)
			restore, err = ResolveCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			result, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "deleted-inverse", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, result)
			var vendor Vendor
			require.NoError(t, db.First(&vendor).Error)
			assert.Equal(t, "source-vendor", vendor.Name)
			assert.Equal(t, "original", vendor.Description)
			assert.NotEqual(t, prior.Id, vendor.Id)
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			target, err := CaptureCatalogTargetTx(context.Background(), db, actor.TargetID, &state)
			require.NoError(t, err)
			var baseline CatalogSyncBaseline
			require.NoError(t, db.First(&baseline).Error)
			assert.Equal(t, target.ObjectVersions[catalogmanifest.EntryID(catalogmanifest.Entry{Kind: catalogmanifest.KindVendor, Key: "source-vendor"})], baseline.ObjectVersion)
			assert.Equal(t, int64(3), baseline.Generation)
		})
	}
}

func TestCatalogSyncRestoreDeletedModelDependency(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			for _, scenario := range []string{"unchanged", "renamed", "recreated", "joint-restore-fixture"} {
				t.Run(scenario, func(t *testing.T) {
					db := catalogFenceTestDB(t, engine)
					actor := catalogBusinessActor(t, db)
					vendor := Vendor{Name: "original-vendor", Status: 1}
					require.NoError(t, db.Create(&vendor).Error)
					require.NoError(t, db.Create(&Vendor{Name: "retained-vendor", Status: 1}).Error)
					require.NoError(t, db.Create(&Model{ModelName: "deleted-model", VendorID: vendor.Id, BillingCurrency: "USD"}).Error)
					source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
					require.NoError(t, err)
					plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
					require.NoError(t, err)
					first, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "dependency-first", actor)
					require.NoError(t, err)
					catalogBusinessFixturePublished(t, db, first)
					source.Entries = slices.DeleteFunc(source.Entries, func(entry catalogmanifest.Entry) bool { return entry.Kind == catalogmanifest.KindModel })
					source.Coverage[catalogmanifest.KindModel] = 0
					source.Digest, err = catalogmanifest.SnapshotDigest(source)
					require.NoError(t, err)
					plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
					require.NoError(t, err)
					plan, err = ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
					require.NoError(t, err)
					second, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "dependency-second", actor)
					require.NoError(t, err)
					catalogBusinessFixturePublished(t, db, second)
					if scenario == "joint-restore-fixture" {
						// Fixture only: current sync conservatively blocks deleting a
						// vendor alongside its referencing model. Build a durable joint
						// preimage from real local records, then exercise the real inverse
						// preview/apply; this does not claim a production joint producer.
						var operation CatalogSyncOperation
						require.NoError(t, db.First(&operation, "id = ?", second.OperationID).Error)
						var backup catalogOperationBackup
						require.NoError(t, common.UnmarshalJsonStr(string(operation.Backup), &backup))
						backup.Before = append(backup.Before, catalogBackupEntry{Kind: catalogmanifest.KindVendor, Key: vendor.Name, Exists: true, Vendor: &vendor})
						for i := range plan.Changes {
							if plan.Changes[i].Kind == catalogmanifest.KindVendor && plan.Changes[i].Key == vendor.Name {
								plan.Changes[i].Action, plan.Changes[i].After = "delete", nil
							}
						}
						plan.Snapshot.Entries = slices.DeleteFunc(plan.Snapshot.Entries, func(entry catalogmanifest.Entry) bool {
							return entry.Kind == catalogmanifest.KindVendor && entry.Key == vendor.Name
						})
						plan.Snapshot.Coverage[catalogmanifest.KindVendor]--
						plan.Snapshot.Digest, err = catalogmanifest.SnapshotDigest(plan.Snapshot)
						require.NoError(t, err)
						plan.Digest, err = catalogmanifest.CanonicalPlanDigest(plan)
						require.NoError(t, err)
						body, err := common.Marshal(plan)
						require.NoError(t, err)
						encodedBackup, err := common.Marshal(backup)
						require.NoError(t, err)
						var binding catalogOperationBinding
						require.NoError(t, common.UnmarshalJsonStr(string(operation.History), &binding))
						binding.Digest = plan.Digest
						history, err := common.Marshal(binding)
						require.NoError(t, err)
						require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
							if err := tx.Delete(&vendor).Error; err != nil {
								return err
							}
							if err := tx.Where("kind = ? AND entry_key = ?", catalogmanifest.KindVendor, vendor.Name).Delete(&CatalogSyncBaseline{}).Error; err != nil {
								return err
							}
							if err := tx.Model(&CatalogSyncPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{"body": CatalogSyncText(body), "digest": plan.Digest}).Error; err != nil {
								return err
							}
							if err := tx.Model(&CatalogSyncOperation{}).Where("id = ?", second.OperationID).Updates(map[string]any{"backup": CatalogSyncText(encodedBackup), "history": CatalogSyncText(history)}).Error; err != nil {
								return err
							}
							digest, err := catalogPersistedDigest(tx)
							if err != nil {
								return err
							}
							return tx.Model(&CatalogSyncState{}).Where("id = ?", CatalogSyncStateID).Update("current_digest", digest).Error
						}))
					}
					restore, err := CreateCatalogSyncRestorePlan(context.Background(), second.OperationID, actor, time.Now())
					require.NoError(t, err, "the original vendor is still valid or part of the joint inverse")
					if scenario == "unchanged" || scenario == "joint-restore-fixture" {
						_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "dependency-inverse", actor)
						require.NoError(t, err)
						var restoredVendor Vendor
						require.NoError(t, db.First(&restoredVendor, "name = ?", vendor.Name).Error)
						var restoredModel Model
						require.NoError(t, db.First(&restoredModel, "model_name = ?", "deleted-model").Error)
						assert.Equal(t, restoredVendor.Id, restoredModel.VendorID)
						if scenario == "unchanged" {
							assert.Equal(t, vendor.Id, restoredVendor.Id)
						} else {
							assert.NotEqual(t, vendor.Id, restoredVendor.Id)
						}
						return
					}
					var stateBefore CatalogSyncState
					require.NoError(t, db.First(&stateBefore, CatalogSyncStateID).Error)
					var baselineBefore []CatalogSyncBaseline
					require.NoError(t, db.Order("identity_hash").Find(&baselineBefore).Error)
					var operationCount int64
					require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&operationCount).Error)
					if scenario == "renamed" {
						require.NoError(t, db.Model(&vendor).Update("name", "renamed-local-vendor").Error)
					} else {
						require.NoError(t, db.Delete(&vendor).Error)
						vendor.Id, vendor.DeletedAt = 0, gorm.DeletedAt{}
						require.NoError(t, db.Create(&vendor).Error)
					}
					var replacementBefore Vendor
					require.NoError(t, db.First(&replacementBefore, vendor.Id).Error)
					_, err = CreateCatalogSyncRestorePlan(context.Background(), second.OperationID, actor, time.Now())
					require.Error(t, err, "restore must reject a renamed or same-name recreated vendor dependency")
					if scenario == "recreated" {
						require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
					}
					_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "dependency-inverse", actor)
					require.ErrorIs(t, err, ErrCatalogSyncPlanStale, "a previously valid inverse cannot attach to the replacement")
					var absent int64
					require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "deleted-model").Count(&absent).Error)
					assert.Zero(t, absent)
					var replacementAfter Vendor
					require.NoError(t, db.First(&replacementAfter, vendor.Id).Error)
					assert.Equal(t, replacementBefore, replacementAfter)
					var stateAfter CatalogSyncState
					require.NoError(t, db.First(&stateAfter, CatalogSyncStateID).Error)
					assert.Equal(t, stateBefore, stateAfter)
					var baselineAfter []CatalogSyncBaseline
					require.NoError(t, db.Order("identity_hash").Find(&baselineAfter).Error)
					assert.Equal(t, baselineBefore, baselineAfter)
					var operationsAfter int64
					require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&operationsAfter).Error)
					assert.Equal(t, operationCount, operationsAfter)
				})
			}
		})
	}
}

func TestCatalogSyncRestorePriceModelRecreated(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogBusinessPriceSource(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			first, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "price-inc-first", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, first)
			for i := range source.Entries {
				entry := &source.Entries[i]
				if entry.Kind != catalogmanifest.KindModelPrice {
					continue
				}
				key, err := catalogmanifest.DecodePriceKey(entry.Key)
				require.NoError(t, err)
				if key.Option == "billing_setting.billing_expr" {
					encoded, err := common.Marshal(catalogmanifest.PriceValue{Value: `"p * 8"`, BillingCurrency: "CNY", Unit: "expression"})
					require.NoError(t, err)
					entry.Value = string(encoded)
				}
			}
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			second, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "price-inc-second", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, second)
			var record Model
			require.NoError(t, db.First(&record, "model_name = ?", "managed-model").Error)
			require.NoError(t, db.Delete(&record).Error)
			record.Id = 0
			record.DeletedAt = gorm.DeletedAt{}
			require.NoError(t, db.Create(&record).Error)
			_, err = CreateCatalogSyncRestorePlan(context.Background(), second.OperationID, actor, time.Now())
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale, "a price-only inverse must not write against a recreated related model")
		})
	}
}

func TestCatalogSyncRestoreLostAck(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			original, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "ack-original", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, original)
			restore, err := CreateCatalogSyncRestorePlan(context.Background(), original.OperationID, actor, time.Now())
			require.NoError(t, err)
			restore, err = ResolveCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
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
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("restore-arm-mutation-ack", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						connection.armed.Store(true)
					}
				}))
				defer db.Callback().Create().Remove("restore-arm-mutation-ack")
				result, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "ack-inverse", actor)
				require.NoError(t, err)
				assert.Equal(t, int64(2), result.Revision)
				require.NoError(t, root.Where("id IN ?", []string{restore.ID, plan.ID}).Delete(&CatalogSyncPlan{}).Error)
				replayed, err := ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest, "ack-inverse", actor)
				require.NoError(t, err)
				assert.Equal(t, result, replayed)
				syncReplay, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "ack-original", actor)
				require.NoError(t, err)
				assert.Equal(t, original, syncReplay)
				_, err = ApplyCatalogSyncPlan(context.Background(), restore.ID, restore.Digest+"bad", "ack-inverse", actor)
				require.ErrorIs(t, err, ErrCatalogSyncOperationConflict)
				var count int64
				require.NoError(t, root.Model(&CatalogSyncOperation{}).Count(&count).Error)
				assert.Equal(t, int64(2), count)
				return nil
			}))
		})
	}
}

func TestCatalogSyncHistoryPagination(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			// Durable fixture rows exercise pagination without inventing a runtime
			// publisher or running 105 identical business operations.
			for i := range 105 {
				copy := plan
				copy.ID = fmt.Sprintf("history-plan-%03d", i)
				copy.Digest, err = catalogmanifest.CanonicalPlanDigest(copy)
				require.NoError(t, err)
				body, err := common.Marshal(copy)
				require.NoError(t, err)
				require.NoError(t, db.Create(&CatalogSyncPlan{ID: copy.ID, Body: CatalogSyncText(body), Digest: copy.Digest, ExpiresAt: copy.ExpiresAt}).Error)
				binding, err := common.Marshal(catalogOperationBinding{Actor: actor, PlanID: copy.ID, Kind: "sync", Digest: copy.Digest})
				require.NoError(t, err)
				require.NoError(t, db.Create(&CatalogSyncOperation{ID: fmt.Sprintf("history-%03d", i), PlanID: copy.ID, State: "succeeded", Revision: int64(i / 2), CreatedAt: 100, Backup: "PRIVATE-BACKUP", History: CatalogSyncText(binding), Result: "PRIVATE-RESULT"}).Error)
			}
			rows, total, err := ListCatalogSyncOperations(context.Background(), -1, 0)
			require.NoError(t, err)
			require.Len(t, rows, 20)
			assert.Equal(t, int64(105), total)
			assert.Equal(t, "history-104", rows[0].ID)
			assert.Equal(t, "history-103", rows[1].ID)
			next, total, err := ListCatalogSyncOperations(context.Background(), 20, 999)
			require.NoError(t, err)
			require.Len(t, next, 85)
			assert.Equal(t, int64(105), total)
			assert.Equal(t, "history-084", next[0].ID)
			maxRows, _, err := ListCatalogSyncOperations(context.Background(), 0, 999)
			require.NoError(t, err)
			assert.Len(t, maxRows, 100)
			for _, row := range maxRows {
				assert.Empty(t, row.Backup)
				assert.Empty(t, row.Result)
				assert.Empty(t, row.History)
				require.NotNil(t, row.Summary)
				assert.Equal(t, 1, row.Summary.Actions["create"])
				assert.Equal(t, actor.UserID, row.Summary.ActorUserID)
			}
			public, err := common.Marshal(maxRows)
			require.NoError(t, err)
			assert.NotContains(t, string(public), "PRIVATE-")
			assert.NotContains(t, string(public), actor.SessionID)
			empty, total, err := ListCatalogSyncOperations(context.Background(), 105, 10)
			require.NoError(t, err)
			assert.Empty(t, empty)
			assert.Equal(t, int64(105), total)
		})
	}
}

func TestCatalogSyncBusinessPreview(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			prior := jsplugin.DefaultRegistry
			jsplugin.DefaultRegistry = jsplugin.NewRegistry()
			t.Cleanup(func() { jsplugin.DefaultRegistry = prior })
			actor := catalogmanifest.Actor{UserID: 1, SessionID: "session", TargetID: "target", AuthVersion: 1, SessionVersion: 1}
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			require.NotEmpty(t, plan.ValidationDigest, "preview must bind the server-owned staged draft")
			require.NotEmpty(t, plan.ReferenceDigest, "preview must bind authoritative references")
			var row CatalogSyncPlan
			require.NoError(t, db.First(&row, "id = ?", plan.ID).Error)
			require.NotEmpty(t, row.Validation)
		})
	}
}

func catalogBusinessActor(t *testing.T, db *gorm.DB) catalogmanifest.Actor {
	t.Helper()
	preserveCatalogCandidate(t)
	require.NoError(t, db.AutoMigrate(&User{}, &UserSession{}))
	user := User{Username: "catalog-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled, AuthVersion: 1}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&UserSession{SID: "catalog-session", UserID: user.Id, UserAuthVersion: 1, Version: 1, Status: UserSessionStatusActive, ExpiresAt: time.Now().Add(time.Hour).Unix(), RefreshHash: strings.Repeat("a", 64), LoginMethod: "password"}).Error)
	prior := jsplugin.DefaultRegistry
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	t.Cleanup(func() { jsplugin.DefaultRegistry = prior })
	catalogPostgresLegacyPrerequisite(t, db, "actor")
	return catalogmanifest.Actor{UserID: user.Id, SessionID: "catalog-session", TargetID: "target", AuthVersion: 1, SessionVersion: 1}
}

func TestCatalogSyncBusinessApply(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogSyncTestSource(t)
			require.NoError(t, appendManagedEntry(&source, catalogmanifest.KindModel, "created-model", catalogmanifest.ModelValue{ModelName: "created-model", Vendor: "source-vendor", BillingCurrency: "CNY", ReleaseDate: "2026-10-01", CreatedTime: 13, UpdatedTime: 17, InputModalities: []string{"text"}, SupportedParameters: []string{"temperature"}, OutputFormats: []string{}}))
			var err error
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "business-operation", actor)
			require.NoError(t, err)
			assert.Equal(t, "committed_pending_publish", result.State)
			assert.Equal(t, int64(1), result.Revision)
			var vendor Vendor
			require.NoError(t, db.First(&vendor, "name = ?", "source-vendor").Error)
			assert.Equal(t, "original", vendor.Description)
			assert.NotEqual(t, int64(13), vendor.CreatedTime)
			var created Model
			require.NoError(t, db.First(&created, "model_name = ?", "created-model").Error)
			assert.Zero(t, created.Status)
			assert.Zero(t, created.SyncOfficial)
			assert.Equal(t, "CNY", created.BillingCurrency)
			assert.Equal(t, "2026-10-01", created.ReleaseDate)
			assert.Equal(t, []string{"temperature"}, created.SupportedParameters)
			assert.Nil(t, created.SupportedResolutions, "source null array must not silently become []")
			assert.Equal(t, []string{}, created.OutputFormats, "source empty array must remain distinct from null")
			assert.NotEqual(t, int64(13), created.CreatedTime)
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			assert.Equal(t, int64(1), state.BaselineGeneration)
			assert.Equal(t, result.Revision, state.RuntimeRevision)
			assert.Empty(t, state.PendingOperationID)
			replayed, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "business-operation", actor)
			require.NoError(t, err)
			assert.Equal(t, result, replayed)
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest+"bad", "business-operation", actor)
			require.ErrorIs(t, err, ErrCatalogSyncOperationConflict)
			var operation CatalogSyncOperation
			require.NoError(t, db.First(&operation, "id = ?", "business-operation").Error)
			assert.NotEmpty(t, operation.Backup)
			assert.NotContains(t, string(operation.Backup), state.IncarnationKey)
			public, err := common.Marshal(operation)
			require.NoError(t, err)
			assert.NotContains(t, string(public), "catalog-session")
			catalogBusinessFixturePublished(t, db, result)
			next, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			for _, change := range next.Changes {
				assert.NotEqual(t, "conflict", change.Action, change.Key)
			}
		})
	}
}

func TestCatalogSyncBusinessWaitedRoleDemotion(t *testing.T) {
	for _, engine := range []string{"mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			writer := db.Begin()
			require.NoError(t, writer.Error)
			defer writer.Rollback()
			require.NoError(t, writer.Model(&User{}).Where("id = ?", actor.UserID).Update("role", common.RoleAdminUser).Error)
			done := make(chan error, 1)
			go func() {
				_, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "waited-demotion", actor)
				done <- err
			}()
			catalogWaitForDBLock(t, db, engine, done)
			require.NoError(t, writer.Commit().Error)
			require.ErrorIs(t, <-done, ErrCatalogSyncPlanUnavailable)
			var count int64
			require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestCatalogSyncBusinessRollbackAndAuth(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			for _, table := range []string{"catalog_sync_operations", "catalog_sync_baselines"} {
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("catalog-business-fail", func(tx *gorm.DB) {
					if tx.Statement.Table == table {
						tx.AddError(errors.New("injected durable write failure"))
					}
				}))
				_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "rollback-operation", actor)
				require.ErrorContains(t, err, "injected durable write failure")
				require.NoError(t, db.Callback().Create().Remove("catalog-business-fail"))
				for _, entity := range []any{&Vendor{}, &CatalogSyncOperation{}, &CatalogSyncBaseline{}} {
					var count int64
					require.NoError(t, db.Model(entity).Count(&count).Error)
					assert.Zero(t, count, table)
				}
				var state CatalogSyncState
				require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
				assert.Zero(t, state.Revision)
				assert.Equal(t, "ready", state.PublicationState)
			}
			for _, field := range []string{"user", "session", "target", "auth", "version"} {
				wrong := actor
				switch field {
				case "user":
					wrong.UserID++
				case "session":
					wrong.SessionID += "wrong"
				case "target":
					wrong.TargetID += "wrong"
				case "auth":
					wrong.AuthVersion++
				case "version":
					wrong.SessionVersion++
				}
				_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "auth-operation", wrong)
				require.Error(t, err, field)
			}
			for _, mutation := range []struct {
				entity            any
				field             string
				changed, original any
			}{
				{&User{}, "role", common.RoleAdminUser, common.RoleRootUser}, {&User{}, "status", common.UserStatusDisabled, common.UserStatusEnabled}, {&User{}, "auth_version", 2, 1},
				{&UserSession{}, "version", 2, 1}, {&UserSession{}, "expires_at", time.Now().Add(-time.Hour).Unix(), time.Now().Add(time.Hour).Unix()}, {&UserSession{}, "revoked_at", 1, 0},
			} {
				require.NoError(t, db.Model(mutation.entity).Where("1 = 1").Update(mutation.field, mutation.changed).Error)
				_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "auth-operation", actor)
				require.Error(t, err, mutation.field)
				require.NoError(t, db.Model(mutation.entity).Where("1 = 1").Update(mutation.field, mutation.original).Error)
			}
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "finally-valid", actor)
			require.NoError(t, err)
		})
	}
}

func catalogBusinessPriceSource(t *testing.T, db *gorm.DB) catalogmanifest.Snapshot {
	t.Helper()
	require.NoError(t, db.Create(&Vendor{Name: "source-vendor", Status: 1}).Error)
	require.NoError(t, db.Create(&Model{ModelName: "managed-model", BillingCurrency: "CNY", Status: 1, CreatedTime: 123}).Error)
	require.NoError(t, db.Create(&Model{ModelName: "local-model", BillingCurrency: "USD", Status: 1}).Error)
	require.NoError(t, db.Create(&Option{Key: "billing_setting.billing_expr", Value: `{"managed-model":"p * 2", "local-model":"p * 9"}`}).Error)
	require.NoError(t, db.Create(&Option{Key: "billing_setting.billing_mode", Value: `{"managed-model":"tiered_expr"}`}).Error)
	source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
	require.NoError(t, err)
	retained := make([]catalogmanifest.Entry, 0, len(source.Entries))
	for _, entry := range source.Entries {
		if entry.Kind == catalogmanifest.KindModel && entry.Key == "local-model" {
			source.Coverage[entry.Kind]--
			continue
		}
		if entry.Kind == catalogmanifest.KindModelPrice {
			key, err := catalogmanifest.DecodePriceKey(entry.Key)
			require.NoError(t, err)
			if key.Model == "local-model" {
				source.Coverage[entry.Kind]--
				continue
			}
			if key.Option == "billing_setting.billing_expr" {
				value, err := common.Marshal(catalogmanifest.PriceValue{Value: `"tier(\"base\", p * 7.123 + c * 3)"`, BillingCurrency: "CNY", Unit: "expression"})
				require.NoError(t, err)
				entry.Value = string(value)
			}
		}
		retained = append(retained, entry)
	}
	source.Entries = retained
	source.Digest, err = catalogmanifest.SnapshotDigest(source)
	require.NoError(t, err)
	return source
}

func TestCatalogSyncBusinessPricesAndDrift(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogBusinessPriceSource(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			// An ordinary catalog save must invalidate an already staged plan.
			require.NoError(t, (&Vendor{Name: "later-local-vendor", Status: 1}).Insert())
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "stale-save", actor)
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			// Unrelated progress/private data changes have no billing-reference effect.
			task := Task{TaskID: "unrelated", Status: TaskStatusInProgress, Properties: Properties{OriginModelName: "local-model"}}
			require.NoError(t, db.Create(&task).Error)
			require.NoError(t, db.Model(&task).Updates(map[string]any{"progress": "99%", "fail_reason": "private unrelated text"}).Error)
			source.Entries[0].Value = "later dev update must not replace pinned source"
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "price-operation", actor)
			require.NoError(t, err)
			var option Option
			require.NoError(t, db.Where(map[string]any{"key": "billing_setting.billing_expr"}).First(&option).Error)
			var prices map[string]string
			require.NoError(t, common.UnmarshalJsonStr(option.Value, &prices))
			assert.Equal(t, `tier("base", p * 7.123 + c * 3)`, prices["managed-model"])
			assert.Equal(t, "p * 9", prices["local-model"])
			var record Model
			require.NoError(t, db.First(&record, "model_name = ?", "managed-model").Error)
			assert.Equal(t, "CNY", record.BillingCurrency)
			assert.Equal(t, int64(123), record.CreatedTime)
			var operation CatalogSyncOperation
			require.NoError(t, db.First(&operation, "id = ?", "price-operation").Error)
			assert.NotContains(t, string(operation.Backup), "local-model")
			assert.NotContains(t, string(operation.Backup), "private unrelated text")
		})
	}
}

func TestCatalogSyncBusinessTaskReferences(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogBusinessPriceSource(t, db)
			for _, tc := range []struct {
				name, origin              string
				terminal, frozen, perCall bool
				grok, malformed           bool
				blocked                   bool
			}{
				{name: "legacy-relevant", origin: "managed-model", blocked: true},
				{name: "unrelated", origin: "local-model"},
				{name: "terminal", origin: "managed-model", terminal: true},
				{name: "frozen-expression", origin: "managed-model", frozen: true},
				{name: "grok-frozen-expression", origin: "managed-model", frozen: true, grok: true},
				{name: "grok-malformed-expression", origin: "managed-model", frozen: true, grok: true, malformed: true, blocked: true},
				{name: "per-call-adjuster-unproven", origin: "managed-model", perCall: true, blocked: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					task := Task{TaskID: tc.name, Status: TaskStatusInProgress, Properties: Properties{OriginModelName: tc.origin}}
					if tc.grok {
						task.Platform = constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeMoliiGrokAIGC))
					}
					if tc.terminal {
						task.Status = TaskStatusSuccess
					}
					if tc.perCall {
						task.PrivateData.BillingContext = &TaskBillingContext{OriginModelName: tc.origin, PerCallBilling: true}
					}
					if tc.frozen {
						task.PrivateData.BillingContext = &TaskBillingContext{OriginModelName: tc.origin, TieredSnapshot: &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ModelName: tc.origin, ExprString: "p * 2", ExprHash: billingexpr.ExprHashString("p * 2"), TaskUsageBilling: true, QuotaPerUnit: 500000, GroupRatio: 1, SourceCurrency: "CNY", CNYPerUSD: 7}}
					}
					if tc.malformed {
						task.PrivateData.BillingContext.TieredSnapshot.ExprHash = "invalid"
						// A valid alternate V2 snapshot must not authorize falling
						// through the actual malformed generic-tiered dispatch.
						task.PrivateData.BillingContext.GrokVideoBilling = &GrokVideoBillingSnapshot{Version: 2, Model: "grok-imagine-video", Operation: "text_to_video", InputType: "text", EstimatedDurationSeconds: 6, EstimatedResolution: "480p", OutputUnitPrice: .05, SourceCurrency: "CNY", CNYPerUSD: 7, GroupRatio: 1}
					}
					require.NoError(t, db.Create(&task).Error)
					plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
					require.NoError(t, err)
					assert.Equal(t, !tc.blocked, catalogmanifest.PlanExecutable(plan, time.Now()))
					if tc.blocked {
						_, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "blocked-"+tc.name, actor)
						require.Error(t, err)
					}
					require.NoError(t, db.Delete(&task).Error)
				})
			}
		})
	}
}

// Existing business/restore tests now verify actual automatic publication.
func catalogBusinessFixturePublished(t *testing.T, db *gorm.DB, result catalogmanifest.Result) {
	t.Helper()
	var state CatalogSyncState
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	require.Equal(t, result.Revision, state.RuntimeRevision)
	require.Equal(t, "ready", state.PublicationState)
	require.Empty(t, state.PendingOperationID)
	var operation CatalogSyncOperation
	require.NoError(t, db.First(&operation, "id = ?", result.OperationID).Error)
	require.Equal(t, "succeeded", operation.State)
}

func TestCatalogSyncBusinessMidjourneyPriceRemoval(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogBusinessPriceSource(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "mj-price-baseline", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, result)
			source.Entries = slices.DeleteFunc(source.Entries, func(entry catalogmanifest.Entry) bool {
				if entry.Kind != catalogmanifest.KindModelPrice {
					return false
				}
				key, err := catalogmanifest.DecodePriceKey(entry.Key)
				require.NoError(t, err)
				if key.Option != "billing_setting.billing_expr" {
					return false
				}
				source.Coverage[entry.Kind]--
				return true
			})
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			channel := Channel{Key: "private", Status: 2, Models: "managed-model"}
			require.NoError(t, db.Create(&channel).Error)
			for _, tc := range []struct {
				name      string
				channelID int
			}{{"related", channel.Id}, {"unresolved", 0}} {
				t.Run(tc.name, func(t *testing.T) {
					legacy := Midjourney{Status: "IN_PROGRESS", ChannelId: tc.channelID}
					require.NoError(t, db.Create(&legacy).Error)
					defer func() { require.NoError(t, db.Delete(&legacy).Error) }()
					plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
					require.NoError(t, err)
					blocked := false
					for _, change := range plan.Changes {
						if change.Kind == catalogmanifest.KindModelPrice && change.After == nil {
							blocked = change.Action == "blocked" && change.Reason == "unfinished_task_reference"
						}
					}
					assert.True(t, blocked, "MJ model uncertainty must still block model-price removal")
				})
			}
		})
	}
}

func TestCatalogSyncBusinessMatchRuleExpansion(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			require.NoError(t, db.Create(&Model{ModelName: "managed", NameRule: NameRulePrefix, BillingCurrency: "USD", Status: 0}).Error)
			require.NoError(t, db.Create(&Channel{Key: "private", Status: common.ChannelStatusEnabled, Models: "new-managed-route"}).Error)
			source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
			require.NoError(t, err)
			for i := range source.Entries {
				if source.Entries[i].Kind == catalogmanifest.KindModel {
					var value catalogmanifest.ModelValue
					require.NoError(t, common.UnmarshalJsonStr(source.Entries[i].Value, &value))
					value.NameRule = NameRuleContains
					encoded, err := common.Marshal(value)
					require.NoError(t, err)
					source.Entries[i].Value = string(encoded)
				}
			}
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			assert.False(t, catalogmanifest.PlanExecutable(plan, time.Now()), "references matched only by the new rule must block unsafe metadata changes")
		})
	}
}

func TestCatalogSyncBusinessRemovals(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			vendor := Vendor{Name: "managed-vendor", Status: 1}
			require.NoError(t, db.Create(&vendor).Error)
			require.NoError(t, db.Create(&Vendor{Name: "retained-vendor", Status: 1}).Error)
			require.NoError(t, db.Create(&Model{ModelName: "managed-", NameRule: NameRulePrefix, BillingCurrency: "USD", Status: 0, VendorID: vendor.Id}).Error)
			source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			applied, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "adopt-removal-fixture", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, applied)
			source.Entries = slices.DeleteFunc(source.Entries, func(entry catalogmanifest.Entry) bool { return entry.Kind == catalogmanifest.KindModel })
			source.Coverage[catalogmanifest.KindModel] = 0
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			for _, scenario := range []string{"mapping-chain", "orphan-ability", "unfinished-task", "legacy-mj", "scheduler"} {
				t.Run(scenario, func(t *testing.T) {
					var cleanup func()
					switch scenario {
					case "mapping-chain":
						mapping := `{"alias":"middle","middle":"managed-live"}`
						row := Channel{Key: "never-export-this-key", Status: common.ChannelStatusEnabled, Models: "alias", ModelMapping: &mapping}
						require.NoError(t, db.Create(&row).Error)
						cleanup = func() { require.NoError(t, db.Delete(&row).Error) }
					case "orphan-ability":
						row := Ability{ChannelId: 123456, Model: "managed-live", Group: "default", Enabled: true}
						require.NoError(t, db.Create(&row).Error)
						cleanup = func() { require.NoError(t, db.Where("channel_id = ?", row.ChannelId).Delete(&Ability{}).Error) }
					case "unfinished-task":
						row := Task{TaskID: scenario, Status: TaskStatusUnknown, Properties: Properties{OriginModelName: "managed-live"}}
						require.NoError(t, db.Create(&row).Error)
						cleanup = func() { require.NoError(t, db.Delete(&row).Error) }
					case "legacy-mj":
						row := Midjourney{Status: "IN_PROGRESS"}
						require.NoError(t, db.Create(&row).Error)
						cleanup = func() { require.NoError(t, db.Delete(&row).Error) }
					case "scheduler":
						row := SystemTask{TaskID: "pending-channel", Type: SystemTaskTypeChannelTest, Status: SystemTaskStatusPending, Payload: "private-never-export"}
						require.NoError(t, db.Create(&row).Error)
						cleanup = func() { require.NoError(t, db.Delete(&row).Error) }
					}
					defer cleanup()
					blocked, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
					require.NoError(t, err)
					assert.False(t, catalogmanifest.PlanExecutable(blocked, time.Now()))
					found := false
					for _, change := range blocked.Changes {
						if change.Kind == catalogmanifest.KindModel {
							assert.Equal(t, "blocked", change.Action)
							found = true
						}
					}
					assert.True(t, found)
				})
			}
			// References gone: a hidden prefix row with no prices can be removed,
			// but only after the independent whole-plan deletion confirmation.
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			assert.False(t, catalogmanifest.PlanExecutable(plan, time.Now()))
			plan, err = ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			assert.True(t, catalogmanifest.PlanExecutable(plan, time.Now()))
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "safe-removal", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, result)
			// A local-only live hidden row still prevents removal of its vendor.
			require.NoError(t, db.Create(&Model{ModelName: "local-vendor-reference", BillingCurrency: "USD", VendorID: vendor.Id, Status: 0}).Error)
			source.Entries = slices.DeleteFunc(source.Entries, func(entry catalogmanifest.Entry) bool { return entry.Key == "managed-vendor" })
			source.Coverage[catalogmanifest.KindVendor]--
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			for _, change := range plan.Changes {
				if change.Key == "managed-vendor" {
					assert.Equal(t, "vendor_still_referenced", change.Reason)
					assert.Equal(t, "blocked", change.Action)
				}
			}
		})
	}
}

type catalogBusinessLostAckConnector struct {
	connection *catalogBusinessLostAckConnection
	driver     driver.Driver
}

func (c catalogBusinessLostAckConnector) Connect(context.Context) (driver.Conn, error) {
	return c.connection, nil
}
func (c catalogBusinessLostAckConnector) Driver() driver.Driver { return c.driver }

type catalogBusinessLostAckConnection struct {
	driver.Conn
	armed   atomic.Bool
	dropped atomic.Int64
}

func (c *catalogBusinessLostAckConnection) Close() error { return nil } // loan owner closes the real connection
func (c *catalogBusinessLostAckConnection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if native, ok := c.Conn.(driver.ExecerContext); ok {
		return native.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}
func (c *catalogBusinessLostAckConnection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if native, ok := c.Conn.(driver.QueryerContext); ok {
		return native.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}
func (c *catalogBusinessLostAckConnection) CheckNamedValue(value *driver.NamedValue) error {
	if native, ok := c.Conn.(driver.NamedValueChecker); ok {
		return native.CheckNamedValue(value)
	}
	return driver.ErrSkip
}
func (c *catalogBusinessLostAckConnection) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &catalogBusinessLostAckTx{Tx: tx, connection: c}, nil
}

type catalogBusinessLostAckTx struct {
	driver.Tx
	connection *catalogBusinessLostAckConnection
}

func (tx *catalogBusinessLostAckTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	if tx.connection.armed.Swap(false) {
		tx.connection.dropped.Add(1)
		return errors.New("injected lost durable commit acknowledgement")
	}
	return nil
}

func TestCatalogSyncBusinessLostAckAndConcurrent(t *testing.T) {
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
				entered, release := make(chan struct{}), make(chan struct{})
				resume := sync.OnceFunc(func() { close(release) })
				defer resume()
				var once sync.Once
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register("catalog-business-pause", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						connection.armed.Store(true)
						once.Do(func() { close(entered); <-release })
					}
				}))
				defer db.Callback().Create().Remove("catalog-business-pause")
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("catalog-business-ack", func(tx *gorm.DB) {
					if tx.Statement.Table == "catalog_sync_operations" {
						if values, ok := tx.Statement.Dest.(map[string]any); ok && values["state"] == "succeeded" {
							connection.armed.Store(true)
						}
					}
				}))
				defer db.Callback().Update().Remove("catalog-business-ack")
				type outcome struct {
					result catalogmanifest.Result
					err    error
				}
				done := make(chan outcome, 1)
				go func() {
					result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "lost-ack-operation", actor)
					done <- outcome{result, err}
				}()
				select {
				case <-entered:
				case result := <-done:
					return fmt.Errorf("apply ended before pause: %v", result.err)
				case <-time.After(10 * time.Second):
					return errors.New("apply did not reach operation backup")
				}
				_, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "lost-ack-operation", actor)
				require.ErrorIs(t, err, ErrCatalogWriterBusy)
				resume()
				result := <-done
				require.NoError(t, result.err)
				assert.Equal(t, "committed_pending_publish", result.result.State)
				assert.Equal(t, int64(2), connection.dropped.Load(), "both actual mutation and ready-ack committed before their responses were lost")
				catalogBusinessFixturePublished(t, db, result.result)
				replayed, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "lost-ack-operation", actor)
				require.NoError(t, err)
				assert.Equal(t, result.result, replayed)
				var count int64
				require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
				assert.Equal(t, int64(1), count)
				return nil
			}))
		})
	}
}

func TestCatalogSyncBusinessSpecialFrozenAndFallback(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			require.NoError(t, db.Create(&Vendor{Name: "keep-source-nonempty", Status: 1}).Error)
			require.NoError(t, db.Create(&Option{Key: "molii_grok_price.video_480p", Value: "0.05"}).Error)
			source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
			require.NoError(t, err)
			for i := range source.Entries {
				if source.Entries[i].Kind == catalogmanifest.KindSpecialPrice {
					encoded, err := common.Marshal(catalogmanifest.PriceValue{Value: "0.09", BillingCurrency: "CNY", Unit: "second"})
					require.NoError(t, err)
					source.Entries[i].Value = string(encoded)
				}
			}
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			t.Run("unresolved-midjourney-unrelated-special", func(t *testing.T) {
				legacy := Midjourney{Status: "IN_PROGRESS"}
				require.NoError(t, db.Create(&legacy).Error)
				defer func() { require.NoError(t, db.Delete(&legacy).Error) }()
				plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
				require.NoError(t, err)
				assert.True(t, catalogmanifest.PlanExecutable(plan, time.Now()), "known MJ family cannot consume Grok special prices")
				starai := source
				starai.Entries = slices.Clone(source.Entries)
				for i := range starai.Entries {
					if starai.Entries[i].Kind != catalogmanifest.KindSpecialPrice {
						continue
					}
					starai.Entries[i].Key, err = catalogmanifest.EncodePriceKey(catalogmanifest.PriceKey{Option: "starai_video_price.standard_720p", Path: ""})
					require.NoError(t, err)
					encoded, err := common.Marshal(catalogmanifest.PriceValue{Value: "30", BillingCurrency: "CNY", Unit: "million_tokens"})
					require.NoError(t, err)
					starai.Entries[i].Value = string(encoded)
				}
				starai.Digest, err = catalogmanifest.SnapshotDigest(starai)
				require.NoError(t, err)
				plan, err = CreateCatalogSyncPlan(context.Background(), starai, actor, time.Now())
				require.NoError(t, err)
				assert.True(t, catalogmanifest.PlanExecutable(plan, time.Now()), "known MJ family cannot consume StarAI special prices")
			})
			for _, tc := range []struct {
				name            string
				frozen, related bool
			}{{"incomplete-legacy", false, true}, {"valid-v2", true, true}, {"invalid-v2-operation", true, true}, {"unrelated-legacy", false, false}, {"unclassifiable", false, true}, {"starai-unknown-model", false, false}} {
				t.Run(tc.name, func(t *testing.T) {
					row := Task{TaskID: tc.name, Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeMoliiGrokAIGC)), Status: TaskStatusInProgress, Properties: Properties{OriginModelName: "grok-imagine-video"}}
					if !tc.related {
						row.Platform = "ordinary-plugin"
						row.Properties.OriginModelName = "unrelated-model"
					}
					if tc.name == "unclassifiable" {
						row.Platform = ""
						row.Properties.OriginModelName = ""
					}
					if tc.name == "starai-unknown-model" {
						row.Platform = constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeStarAI))
						row.Properties.OriginModelName = ""
					}
					if tc.frozen {
						row.PrivateData.BillingContext = &TaskBillingContext{OriginModelName: "grok-imagine-video", GroupRatio: 1, GrokVideoBilling: &GrokVideoBillingSnapshot{Version: 2, Model: "grok-imagine-video", Operation: "text_to_video", InputType: "text", EstimatedDurationSeconds: 6, EstimatedResolution: "480p", OutputUnitPrice: .05, SourceCurrency: "CNY", CNYPerUSD: 7, GroupRatio: 1}}
					}
					if tc.name == "invalid-v2-operation" {
						row.PrivateData.BillingContext.GrokVideoBilling.Operation = "unknown"
					}
					require.NoError(t, db.Create(&row).Error)
					plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
					require.NoError(t, err)
					assert.Equal(t, tc.frozen && tc.name != "invalid-v2-operation" || !tc.related, catalogmanifest.PlanExecutable(plan, time.Now()))
					require.NoError(t, db.Delete(&row).Error)
				})
			}
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "special-update", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, result)
			source.Entries = slices.DeleteFunc(source.Entries, func(entry catalogmanifest.Entry) bool { return entry.Kind == catalogmanifest.KindSpecialPrice })
			source.Coverage[catalogmanifest.KindSpecialPrice] = 0
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			for _, change := range plan.Changes {
				if change.Kind == catalogmanifest.KindSpecialPrice {
					assert.Equal(t, "blocked", change.Action)
					assert.Equal(t, "price_fallback_unproven", change.Reason)
				}
			}
		})
	}
}

func TestCatalogSyncBusinessPluginDriftAndAdoption(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogValidationFixture(t, db, jsplugin.DefaultRegistry)
			source.SourceID = "dev"
			var err error
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			var plugin TaskPlugin
			require.NoError(t, db.First(&plugin).Error)
			require.NoError(t, db.Model(&plugin).Update("source", "never-leak-invalid-private-source").Error)
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "plugin-drift", actor)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "never-leak")
			require.NoError(t, db.Model(&plugin).Update("source", plugin.Source).Error)
			require.NoError(t, db.Model(&plugin).Update("active", false).Error)
			require.NoError(t, jsplugin.DefaultRegistry.ReplaceOverrides(nil))
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "effective-drift", actor)
			require.Error(t, err)
			// Equal existing target usage expressions are new ownership adoption,
			// so the stale-local preservation exception must not authorize them.
			_, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.Error(t, err)
		})
	}
}

func TestCatalogSyncBusinessAuthLayout(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			switch engine {
			case "mysql":
				require.NoError(t, db.Exec("ALTER TABLE user_sessions ENGINE=MyISAM").Error)
			case "postgres":
				require.NoError(t, db.Exec("ALTER TABLE user_sessions ENABLE ROW LEVEL SECURITY").Error)
			case "sqlite":
				require.NoError(t, db.Exec("ALTER TABLE user_sessions RENAME TO original_user_sessions").Error)
				require.NoError(t, db.Exec("CREATE VIEW user_sessions AS SELECT * FROM original_user_sessions").Error)
			}
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "unsupported-auth-layout", actor)
			require.Error(t, err, "unsupported auth storage must not authorize a catalog commit")
			var count int64
			require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestCatalogSyncBusinessAuthTemporaryShadow(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), catalogSyncTestSource(t), actor, time.Now())
			require.NoError(t, err)
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			pool.SetMaxIdleConns(1)
			sql := "CREATE TEMPORARY TABLE user_sessions AS SELECT * FROM user_sessions"
			if engine == "mysql" {
				sql = "CREATE TEMPORARY TABLE user_sessions (sid varchar(64) PRIMARY KEY) ENGINE=InnoDB"
			}
			require.NoError(t, db.Exec(sql).Error)
			if engine == "postgres" {
				// The reviewed root helper pins the trusted schema ahead of
				// pg_temp. An active temporary copy must not mask revocation.
				require.NoError(t, db.Exec("UPDATE public.user_sessions SET status = 'revoked'").Error)
			}
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "shadow-auth", actor)
			require.Error(t, err)
			var count int64
			require.NoError(t, db.Model(&CatalogSyncOperation{}).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestCatalogSyncBusinessSeedanceFrozen(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogBusinessPriceSource(t, db)
			for _, tc := range []struct {
				name                      string
				ratio, group              float64
				captured, tiered, allowed bool
			}{
				{"saved-anchor", 2, 1, true, false, true},
				{"starai-saved-anchor", 2, 1, true, false, true},
				{"starai-missing-anchor", 0, 1, true, false, false},
				{"captured-zero-group", 2, 0, true, false, true},
				{"missing-anchor", 0, 1, true, false, false},
				{"uncaptured-zero-group", 2, 0, false, false, false},
				{"dispatch-not-generic-tiered", 2, 1, true, true, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					bc := &TaskBillingContext{OriginModelName: "managed-model", ModelRatio: tc.ratio, GroupRatio: tc.group, GroupRatioCaptured: tc.captured, OtherRatios: map[string]float64{"video": .5}}
					if tc.tiered {
						bc.TieredSnapshot = &billingexpr.BillingSnapshot{BillingMode: "tiered_expr", ModelName: "managed-model", TaskUsageBilling: true, ExprString: "p", ExprHash: billingexpr.ExprHashString("p"), QuotaPerUnit: 500000, SourceCurrency: "CNY", CNYPerUSD: 7}
					}
					row := Task{TaskID: tc.name, Platform: constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeByteDanceSeedance)), Status: TaskStatusInProgress, Properties: Properties{OriginModelName: "managed-model"}}
					if strings.HasPrefix(tc.name, "starai-") {
						row.Platform = constant.TaskPlatform(fmt.Sprint(constant.ChannelTypeStarAI))
					}
					row.PrivateData.BillingContext = bc
					require.NoError(t, db.Create(&row).Error)
					plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
					require.NoError(t, err)
					assert.Equal(t, tc.allowed, catalogmanifest.PlanExecutable(plan, time.Now()))
					require.NoError(t, db.Delete(&row).Error)
				})
			}
		})
	}
}

func TestCatalogSyncBusinessResolutionAndIncarnation(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			actor := catalogBusinessActor(t, db)
			source := catalogBusinessPriceSource(t, db)
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			var prior Model
			require.NoError(t, db.First(&prior, "model_name = ?", "managed-model").Error)
			require.NoError(t, db.Delete(&prior).Error)
			prior.Id = 0
			prior.DeletedAt = gorm.DeletedAt{}
			require.NoError(t, db.Create(&prior).Error)
			_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "recreated-after-preview", actor)
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			result, err := ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "first-price-baseline", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, result)
			require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": "billing_setting.billing_expr"}).Update("value", `{"managed-model":"p * 99","local-model":"p * 9"}`).Error)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			assert.False(t, catalogmanifest.PlanExecutable(plan, time.Now()))
			choice := catalogmanifest.Resolution{OverwriteKeys: []string{catalogmanifest.EntryID(catalogmanifest.Entry{Kind: catalogmanifest.KindModel, Key: "managed-model"})}}
			resolved, err := ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, choice)
			require.NoError(t, err)
			assert.True(t, catalogmanifest.PlanExecutable(resolved, time.Now()))
			revoked, err := ResolveCatalogSyncPlan(context.Background(), resolved.ID, resolved.Digest, actor, catalogmanifest.Resolution{})
			require.NoError(t, err)
			assert.False(t, catalogmanifest.PlanExecutable(revoked, time.Now()))
			_, err = ApplyCatalogSyncPlan(context.Background(), resolved.ID, resolved.Digest, "revoked-consent", actor)
			require.Error(t, err)
			resolved, err = ResolveCatalogSyncPlan(context.Background(), revoked.ID, revoked.Digest, actor, choice)
			require.NoError(t, err)
			result, err = ApplyCatalogSyncPlan(context.Background(), resolved.ID, resolved.Digest, "confirmed-overwrite", actor)
			require.NoError(t, err)
			catalogBusinessFixturePublished(t, db, result)
			// Removing an explicit effective expression may expose a builtin or
			// cheaper default; structural validity alone never authorizes that.
			source.Entries = slices.DeleteFunc(source.Entries, func(entry catalogmanifest.Entry) bool {
				if entry.Kind != catalogmanifest.KindModelPrice {
					return false
				}
				key, err := catalogmanifest.DecodePriceKey(entry.Key)
				require.NoError(t, err)
				if key.Option == "billing_setting.billing_expr" {
					source.Coverage[entry.Kind]--
					return true
				}
				return false
			})
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			plan, err = CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			found := false
			for _, change := range plan.Changes {
				if change.Reason == "price_fallback_unproven" {
					found = true
					assert.Equal(t, "blocked", change.Action)
				}
			}
			assert.True(t, found)
		})
	}
}

func catalogValidationFixture(t *testing.T, db *gorm.DB, registry *jsplugin.Registry) catalogmanifest.Snapshot {
	t.Helper()
	source := pricingUsagePluginSource("1.0.0", `{seconds:{type:"number",unit:"second"}}`)
	plugin, err := registry.Register(source, jsplugin.Options{})
	require.NoError(t, err)
	require.NoError(t, db.Create(&TaskPlugin{Key: plugin.Meta.Key, Version: plugin.Meta.Version, APIVersion: 1, Source: LongText(source), SourceHash: fmt.Sprintf("%x", sha256.Sum256([]byte(source))), Active: true, Enabled: true}).Error)
	require.NoError(t, db.Create(&Model{ModelName: "pricing-usage-model", BillingCurrency: "CNY"}).Error)
	require.NoError(t, db.Create(&Option{Key: "billing_setting.billing_expr", Value: `{"pricing-usage-model":"u(\"seconds\") * 0.4"}`}).Error)
	snapshot, err := ExportManagedCatalogTx(context.Background(), db, "target")
	require.NoError(t, err)
	return snapshot
}

func TestCatalogSyncValidationAttestation(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			snapshot := catalogValidationFixture(t, db, registry)
			var input catalogValidationInput
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
					var err error
					snapshot, err = captureCatalogTargetTx(context.Background(), tx, "target", state, true)
					if err != nil {
						return err
					}
					input, err = captureCatalogValidationInputTx(tx, pin, snapshot, snapshot, nil)
					return err
				})
			})
			require.NoError(t, err)
			attestation, err := stageCatalogValidation(input)
			require.NoError(t, err)
			plan := catalogmanifest.Plan{Kind: "sync", ValidationDigest: attestation.digest, ReferenceDigest: "server-reference-commitment"}
			row := CatalogSyncPlan{ID: "attestation", Validation: CatalogSyncText(attestation.payload), Digest: "fixture", Body: "fixture"}
			require.NoError(t, db.Create(&row).Error)
			var stored CatalogSyncPlan
			require.NoError(t, db.First(&stored, "id = ?", row.ID).Error)
			require.NoError(t, verifyCatalogValidation(stored, plan, input))
			reordered := snapshot
			reordered.Entries = slices.Clone(snapshot.Entries)
			slices.Reverse(reordered.Entries)
			pin, err := registry.TryPinGeneration()
			require.NoError(t, err)
			equivalent, err := captureCatalogValidationInputTx(db, pin, reordered, reordered, nil)
			pin.Release()
			require.NoError(t, err)
			require.NoError(t, verifyCatalogValidation(stored, plan, equivalent), "equivalent snapshot entry order must have one canonical commitment")
			for _, missing := range []string{"payload", "validation", "reference"} {
				badRow, badPlan := stored, plan
				switch missing {
				case "payload":
					badRow.Validation = ""
				case "validation":
					badPlan.ValidationDigest = ""
				case "reference":
					badPlan.ReferenceDigest = ""
				}
				require.ErrorIs(t, verifyCatalogValidation(badRow, badPlan, input), ErrCatalogSyncPlanStale)
			}
			public, err := common.Marshal(stored)
			require.NoError(t, err)
			assert.NotContains(t, string(public), attestation.payload)
			assert.NotContains(t, attestation.payload, "buildSubmitRequest")
			assert.NotContains(t, attestation.payload, "IncarnationKey")
			// Currency and raw expression changes are re-read from the DB and
			// yield a different full draft commitment, independent of the actor.
			for _, values := range []struct{ currency, expression string }{{"CNY", `u("seconds") * 0.8`}, {"USD", `u("seconds") * 0.4`}} {
				require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "pricing-usage-model").Update("billing_currency", values.currency).Error)
				encoded, err := common.Marshal(map[string]string{"pricing-usage-model": values.expression})
				require.NoError(t, err)
				require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": "billing_setting.billing_expr"}).Update("value", string(encoded)).Error)
				require.NoError(t, WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
						current, err := captureCatalogTargetTx(context.Background(), tx, "target", state, true)
						if err != nil {
							return err
						}
						changed, err := captureCatalogValidationInputTx(tx, pin, current, current, nil)
						if err != nil {
							return err
						}
						assert.ErrorIs(t, verifyCatalogValidation(stored, plan, changed), ErrCatalogSyncPlanStale)
						return nil
					})
				}))
			}
			// Source snapshot maps cannot alias the server-owned validation input.
			snapshot.Entries = slices.Clone(snapshot.Entries)
			snapshot.Entries[0].Value = "invalid changed caller-owned value"
			unchanged, err := stageCatalogValidation(input)
			require.NoError(t, err)
			assert.Equal(t, attestation, unchanged)
			snapshot, err = ExportManagedCatalogTx(context.Background(), db, "target")
			require.NoError(t, err)
			// A source edit with unchanged key/version/hash cannot masquerade as
			// the effective old program, even if all schema metadata still matches.
			require.NoError(t, db.Model(&TaskPlugin{}).Where("active = ?", true).Update("source", "secret-malformed-source").Error)
			pin, err = registry.TryPinGeneration()
			require.NoError(t, err)
			_, err = captureCatalogValidationInputTx(db, pin, snapshot, snapshot, nil)
			pin.Release()
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "secret-malformed-source")
		})
	}
}

// Fixed persisted declaration from Task3, before the attestation column exists.
type catalogPlanBeforeAttestation struct {
	ID                 string          `gorm:"primaryKey;size:64"`
	Body               CatalogSyncText `gorm:"not null"`
	Digest             string          `gorm:"size:71;not null"`
	TargetRevision     int64           `gorm:"not null"`
	BaselineGeneration int64           `gorm:"not null"`
	ExpiresAt          int64           `gorm:"not null;index"`
}

func (catalogPlanBeforeAttestation) TableName() string { return "catalog_sync_plans" }

func TestCatalogSyncValidationMigration(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&catalogPlanBeforeAttestation{}))
			require.NoError(t, db.Create(&catalogPlanBeforeAttestation{ID: "old-plan", Body: "old-body", Digest: "old-digest", TargetRevision: 7, BaselineGeneration: 3, ExpiresAt: 100}).Error)
			for attempt := range 3 {
				recorder := &migrationSQLRecorder{}
				require.NoError(t, MigrateCatalogSync(db.Session(&gorm.Session{Logger: recorder})))
				if attempt > 0 {
					assert.Empty(t, recorder.schemaMutations(), "repeated migration must not alter schema")
				}
			}
			var old CatalogSyncPlan
			require.NoError(t, db.First(&old, "id = ?", "old-plan").Error)
			assert.Equal(t, CatalogSyncText("old-body"), old.Body)
			assert.Equal(t, int64(7), old.TargetRevision)
			assert.Empty(t, old.Validation)
			require.ErrorIs(t, verifyCatalogValidation(old, catalogmanifest.Plan{}, catalogValidationInput{canonical: "fixture"}), ErrCatalogSyncPlanStale)
		})
	}
}

func TestCatalogSyncValidationPreservation(t *testing.T) {
	db := catalogFenceTestDB(t, "sqlite")
	registry := jsplugin.NewRegistry()
	snapshot := catalogValidationFixture(t, db, registry)
	require.NoError(t, registry.ReplaceOverrides(nil))
	require.NoError(t, db.Model(&TaskPlugin{}).Where("active = ?", true).Update("enabled", false).Error)
	pin, err := registry.TryPinGeneration()
	require.NoError(t, err)
	var priceID string
	for _, entry := range snapshot.Entries {
		if entry.Kind == catalogmanifest.KindModelPrice {
			priceID = catalogmanifest.EntryID(entry)
		}
	}
	require.NotEmpty(t, priceID)
	retained, err := captureCatalogValidationInputTx(db, pin, snapshot, snapshot, nil)
	require.NoError(t, err)
	adopted, err := captureCatalogValidationInputTx(db, pin, snapshot, snapshot, []string{priceID})
	pin.Release()
	require.NoError(t, err)
	_, err = stageCatalogValidation(retained)
	require.NoError(t, err, "unchanged retained local usage expression keeps existing preservation allowance")
	_, err = stageCatalogValidation(adopted)
	require.Error(t, err, "equal-to-target source adoption still requires a current schema binding")
	// Ordinary validation must retain its existing stale-preservation behavior.
	prior := jsplugin.DefaultRegistry
	jsplugin.DefaultRegistry = registry
	t.Cleanup(func() { jsplugin.DefaultRegistry = prior })
	values := PricingValues{"billing_setting.billing_expr": `u("seconds") * 0.4`, billing_setting.PluginBillingExprOption: map[string]any{"missing": `u("seconds") * 0.4`}}
	require.NoError(t, validateModelPricing("pricing-usage-model", values, values))
	// Existing stale plugin overrides retain the same exception, but source
	// adoption (even byte-identical) is not entitled to the previous value.
	require.NoError(t, db.Create(&Option{Key: billing_setting.PluginBillingExprOption, Value: `{"missing::pricing-usage-model":"invalid syntax ["}`}).Error)
	var stale catalogmanifest.Snapshot
	require.NoError(t, WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
			var err error
			stale, err = captureCatalogTargetTx(context.Background(), tx, "target", state, true)
			if err != nil {
				return err
			}
			retained, err = captureCatalogValidationInputTx(tx, pin, stale, stale, nil)
			if err != nil {
				return err
			}
			for _, entry := range stale.Entries {
				if entry.Kind == catalogmanifest.KindPluginPrice {
					priceID = catalogmanifest.EntryID(entry)
				}
			}
			adopted, err = captureCatalogValidationInputTx(tx, pin, stale, stale, []string{priceID})
			return err
		})
	}))
	_, err = stageCatalogValidation(retained)
	require.NoError(t, err)
	_, err = stageCatalogValidation(adopted)
	require.Error(t, err)
	_, err = ExportManagedCatalogTx(context.Background(), db, "target")
	require.Error(t, err, "ordinary/full export retains semantic expression validation")
}

func TestCatalogSyncValidationRuntimeStates(t *testing.T) {
	db := catalogFenceTestDB(t, "sqlite")
	registry := jsplugin.NewRegistry()
	snapshot := catalogValidationFixture(t, db, registry)
	key := "pricing-usage-probe"
	capture := func() error {
		pin, err := registry.TryPinGeneration()
		if err != nil {
			return err
		}
		defer pin.Release()
		_, err = captureCatalogValidationInputTx(db, pin, snapshot, snapshot, nil)
		return err
	}
	source := pricingUsagePluginSource("1.0.0", `{seconds:{type:"number",unit:"second"}}`)
	factory := strings.ReplaceAll(strings.ReplaceAll(source, "pricing-usage-probe", "factory-probe"), "pricing-usage-model", "factory-model")
	_, err := registry.RegisterFactory(factory, jsplugin.Options{})
	require.NoError(t, err)
	require.NoError(t, capture())
	require.NoError(t, db.Create(&Option{Key: "TaskPluginDisabledFactoryKeys", Value: `["factory-probe"]`}).Error)
	require.Error(t, capture(), "desired factory switch must be published")
	registry.SetDisabledFactoryKeys([]string{"factory-probe"})
	require.NoError(t, capture())
	compiled, ok := registry.Generation().Get(key)
	require.True(t, ok)
	require.NoError(t, registry.ReplaceOverrides([]*jsplugin.LoadedPlugin{{Meta: compiled.Meta}}))
	require.Error(t, capture(), "metadata without an actual compiled identity is not proof")
	_, err = registry.Register(source, jsplugin.Options{})
	require.NoError(t, err)
	otherSource := strings.ReplaceAll(strings.ReplaceAll(strings.Replace(source, "fetchMode:", "channelTypes:[77],fetchMode:", 1), key, "other-probe"), "pricing-usage-model", "other-model")
	other, err := registry.Register(otherSource, jsplugin.Options{})
	require.NoError(t, err)
	updated, err := jsplugin.CompilePlugin(strings.Replace(source, "fetchMode:", "channelTypes:[77],fetchMode:", 1), jsplugin.Options{})
	require.NoError(t, err)
	require.NoError(t, registry.ReplaceOverrides([]*jsplugin.LoadedPlugin{updated, other}))
	assert.Equal(t, "partial", registry.LastRebuildOutcome().Status)
	require.Error(t, capture(), "partial retained publication cannot attest")
}

func TestCatalogSyncSensitiveWriter(t *testing.T) {
	called := false
	require.NoError(t, TryWithCatalogWriteBarrier(context.Background(), func() error { called = true; return nil }))
	require.True(t, called)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, TryWithCatalogWriteBarrier(ctx, func() error { t.Fatal("cancelled entry must not run"); return nil }), context.Canceled)
	for _, read := range []bool{false, true} {
		if read {
			catalogBarrier.RLock()
		} else {
			catalogBarrier.Lock()
		}
		finished := make(chan error, 1)
		go func() {
			finished <- TryWithCatalogWriteBarrier(context.Background(), func() error { return errors.New("unexpected acquisition") })
		}()
		select {
		case err := <-finished:
			assert.ErrorIs(t, err, ErrCatalogWriterBusy)
		case <-time.After(time.Second):
			t.Error("sensitive writer acquisition blocked")
		}
		if read {
			catalogBarrier.RUnlock()
		} else {
			catalogBarrier.Unlock()
		}
	}
	sentinel := errors.New("callback failed")
	require.ErrorIs(t, TryWithCatalogWriteBarrier(context.Background(), func() error { return sentinel }), sentinel)
	require.NoError(t, TryWithCatalogWriteBarrier(context.Background(), func() error { return nil }), "failure must release the writer")
}

func TestCatalogSyncValidationDependencies(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			_ = catalogValidationFixture(t, db, registry)
			require.NoError(t, db.Create(&Model{ModelName: "alias", BillingCurrency: "CNY"}).Error)
			require.NoError(t, db.Model(&Option{}).Where(map[string]any{"key": "billing_setting.billing_expr"}).Update("value", `{"alias":"u(\"seconds\") * 0.5","pricing-usage-model":"u(\"seconds\") * 0.4"}`).Error)
			mapping := `{"alias":"pricing-usage-model"}`
			channel := Channel{Key: "must-not-appear-credential", Models: "alias", ModelMapping: &mapping, Status: common.ChannelStatusEnabled}
			require.NoError(t, db.Create(&channel).Error)
			snapshot, err := ExportManagedCatalogTx(context.Background(), db, "target")
			require.NoError(t, err)
			strict := []string{}
			for _, entry := range snapshot.Entries {
				if entry.Kind == catalogmanifest.KindModelPrice {
					strict = append(strict, catalogmanifest.EntryID(entry))
				}
			}
			capture := func() (catalogValidationInput, error) {
				var input catalogValidationInput
				err := WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, pin *jsplugin.GenerationPin) error {
						var err error
						input, err = captureCatalogValidationInputTx(tx, pin, snapshot, snapshot, strict)
						return err
					})
				})
				return input, err
			}
			input, err := capture()
			require.NoError(t, err)
			attestation, err := stageCatalogValidation(input)
			require.NoError(t, err)
			assert.NotContains(t, attestation.payload, "must-not-appear-credential")
			row := CatalogSyncPlan{Validation: CatalogSyncText(attestation.payload)}
			plan := catalogmanifest.Plan{ValidationDigest: attestation.digest, ReferenceDigest: "server-reference"}
			for _, raw := range []string{`null`, `{"alias":"x","alias":"pricing-usage-model"}`, `{"alias":42}`, `{"alias":null}`, `{"alias":"loop","loop":"alias"}`} {
				require.NoError(t, db.Model(&channel).Update("model_mapping", raw).Error)
				_, err := capture()
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "must-not-appear-credential")
			}
			require.NoError(t, db.Model(&channel).Update("model_mapping", `{"alias":"unrelated"}`).Error)
			changed, err := capture()
			require.NoError(t, err)
			require.ErrorIs(t, verifyCatalogValidation(row, plan, changed), ErrCatalogSyncPlanStale)
			_, err = stageCatalogValidation(changed)
			require.Error(t, err, "strict alias expression cannot validate against a stale cached alias")
			require.NoError(t, db.Model(&channel).Update("model_mapping", mapping).Error)
			updated := pricingUsagePluginSource("1.0.0", `{clips:{type:"number",unit:"count"}}`)
			_, err = registry.Register(updated, jsplugin.Options{})
			require.NoError(t, err)
			_, err = capture()
			require.Error(t, err, "effective source must match desired DB source")
			require.NoError(t, db.Model(&TaskPlugin{}).Where("active = ?", true).Updates(map[string]any{"source": updated, "source_hash": fmt.Sprintf("%x", sha256.Sum256([]byte(updated)))}).Error)
			changed, err = capture()
			require.NoError(t, err)
			require.ErrorIs(t, verifyCatalogValidation(row, plan, changed), ErrCatalogSyncPlanStale)
			_, err = stageCatalogValidation(changed)
			require.Error(t, err, "same-version schema update must validate with the new schema")
			require.NoError(t, db.Create(&Option{Key: "TaskPluginEnabled", Value: "false"}).Error)
			_, err = capture()
			require.Error(t, err, "pending master switch must block")
			registry.SetEnabled(false)
			changed, err = capture()
			require.NoError(t, err)
			require.ErrorIs(t, verifyCatalogValidation(row, plan, changed), ErrCatalogSyncPlanStale)
			require.Error(t, registry.SetGenerationPreparer(func(_, _ *jsplugin.RoutingGeneration) (jsplugin.PreparedRoutingGeneration, error) {
				return jsplugin.PreparedRoutingGeneration{}, errors.New("fixture publication failure")
			}))
			_, err = capture()
			require.Error(t, err, "failed/retained generation cannot attest")
		})
	}
}

func catalogSyncTestSource(t *testing.T) catalogmanifest.Snapshot {
	t.Helper()
	value, err := common.Marshal(catalogmanifest.VendorValue{Name: "source-vendor", Description: "original", Status: 1, DisplayOrder: 1, CreatedTime: 13, UpdatedTime: 17})
	require.NoError(t, err)
	source := catalogmanifest.Snapshot{SchemaVersion: 2, SourceID: "dev", Complete: true, ExportedAt: 42, Capabilities: catalogmanifest.RequiredCapabilities(), Coverage: map[string]int{}, Entries: []catalogmanifest.Entry{{Kind: "vendor", Key: "source-vendor", Value: string(value)}}}
	for _, kind := range catalogmanifest.Kinds() {
		source.Coverage[kind] = 0
	}
	source.Coverage["vendor"] = 1
	source.Digest, err = catalogmanifest.SnapshotDigest(source)
	require.NoError(t, err)
	return source
}

func TestCatalogSyncRefreshExcludesWriter(t *testing.T) {
	db := catalogSyncTestDB(t, "sqlite")
	require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Channel{}, &Ability{}))
	var reads atomic.Int32
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("catalog-refresh-reader", func(tx *gorm.DB) {
		reads.Add(1)
		if catalogBarrier.TryLock() {
			catalogBarrier.Unlock()
			t.Error("catalog writer entered during pricing refresh DB reads")
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("catalog-refresh-reader"); InvalidatePricingCache() })
	RefreshPricing()
	require.Positive(t, reads.Load())
}

func TestCatalogSyncStore(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			require.NoError(t, db.Create(&Vendor{Name: "source-vendor", Description: "original", Status: 1, DisplayOrder: 1, CreatedTime: 99, UpdatedTime: 101}).Error)
			actor := catalogmanifest.Actor{UserID: 1, SessionID: "opaque-session", TargetID: "local", AuthVersion: 2, SessionVersion: 3}
			source := catalogSyncTestSource(t)
			now := time.Now().UTC()
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, now)
			require.NoError(t, err)
			assert.NotEmpty(t, plan.ID)
			assert.Equal(t, now.Add(10*time.Minute).Unix(), plan.ExpiresAt)
			require.Len(t, plan.Changes, 1)
			assert.Equal(t, "adopt", plan.Changes[0].Action)
			assert.Equal(t, source, plan.Snapshot)
			got, err := GetCatalogSyncPlan(context.Background(), plan.ID, actor)
			require.NoError(t, err)
			assert.Equal(t, plan, got)
			resolved, err := ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{ConfirmDeletes: true})
			require.NoError(t, err)
			_, err = ResolveCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, actor, catalogmanifest.Resolution{})
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			_, err = ResolveCatalogSyncPlan(context.Background(), plan.ID, resolved.Digest, actor, resolved.Resolution)
			require.NoError(t, err, "repeating the already-current resolution is idempotent")
			for _, field := range []string{"user", "session", "target", "auth_version", "session_version"} {
				wrong := actor
				switch field {
				case "user":
					wrong.UserID++
				case "session":
					wrong.SessionID += "wrong"
				case "target":
					wrong.TargetID += "wrong"
				case "auth_version":
					wrong.AuthVersion++
				case "session_version":
					wrong.SessionVersion++
				}
				_, err := GetCatalogSyncPlan(context.Background(), plan.ID, wrong)
				require.Error(t, err, field)
			}
			expired, err := CreateCatalogSyncPlan(context.Background(), source, actor, now.Add(-11*time.Minute))
			require.NoError(t, err)
			_, err = GetCatalogSyncPlan(context.Background(), expired.ID, actor)
			require.Error(t, err)
			actor.AuthVersion = 0
			_, err = CreateCatalogSyncPlan(context.Background(), source, actor, now)
			require.Error(t, err)
			// Snapshots and operation backups exceed MySQL's 64 KiB TEXT ceiling.
			actor.AuthVersion = 2
			large := catalogSyncTestSource(t)
			var value catalogmanifest.VendorValue
			require.NoError(t, common.UnmarshalJsonStr(large.Entries[0].Value, &value))
			value.Description = strings.Repeat("x", 80_000)
			encoded, err := common.Marshal(value)
			require.NoError(t, err)
			large.Entries[0].Value = string(encoded)
			large.Digest, err = catalogmanifest.SnapshotDigest(large)
			require.NoError(t, err)
			largePlan, err := CreateCatalogSyncPlan(context.Background(), large, actor, now)
			require.NoError(t, err)
			large.Entries[0].Value = "changed caller buffer"
			stored, err := GetCatalogSyncPlan(context.Background(), largePlan.ID, actor)
			require.NoError(t, err)
			assert.Equal(t, string(encoded), stored.Snapshot.Entries[0].Value)
			require.NoError(t, db.Create(&CatalogSyncOperation{ID: largePlan.ID, PlanID: largePlan.ID, State: "prepared", Backup: CatalogSyncText(encoded), History: CatalogSyncText(encoded), Result: "{}"}).Error)
		})
	}
}

func TestCatalogSyncStoreBaselineIncarnation(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			vendor := Vendor{Name: "managed-vendor", Status: 1, CreatedTime: 10}
			require.NoError(t, db.Create(&vendor).Error)
			model := Model{ModelName: "managed-model", VendorID: vendor.Id, BillingCurrency: "CNY", CreatedTime: 20}
			require.NoError(t, db.Create(&model).Error)
			source, err := ExportManagedCatalogTx(context.Background(), db, "dev")
			require.NoError(t, err)
			var original catalogmanifest.Snapshot
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				return db.Transaction(func(tx *gorm.DB) error {
					state, err := LockCatalogMutationTx(tx)
					if err != nil {
						return err
					}
					original, err = CaptureCatalogTargetTx(context.Background(), tx, "target", state)
					if err != nil {
						return err
					}
					_, err = SaveCatalogSyncBaselineTx(tx, source, original.ObjectVersions, 0)
					return err
				})
			}))
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			baseline, err := LoadCatalogSyncBaselineTx(db, &state)
			require.NoError(t, err)
			assert.ElementsMatch(t, source.Entries, baseline.Entries, "baseline must retain full source audit metadata")
			assert.Equal(t, original.ObjectVersions, baseline.ObjectVersions)
			assert.Equal(t, int64(1), baseline.Generation)
			require.NoError(t, withMarketplaceOrderTransaction(db, func(tx *gorm.DB) error {
				if err := tx.Delete(&model).Error; err != nil {
					return err
				}
				model.Id, model.DeletedAt = 0, gorm.DeletedAt{}
				return tx.Create(&model).Error
			}))
			actor := catalogmanifest.Actor{UserID: 1, SessionID: "session", TargetID: "target", AuthVersion: 1, SessionVersion: 1}
			plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			for _, change := range plan.Changes {
				if change.Kind == "model" {
					assert.Equal(t, "conflict", change.Action)
				}
			}
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			recreated, err := CaptureCatalogTargetTx(context.Background(), db, "target", &state)
			require.NoError(t, err)
			id := catalogmanifest.EntryID(catalogmanifest.Entry{Kind: "model", Key: "managed-model"})
			assert.NotEqual(t, original.ObjectVersions[id], recreated.ObjectVersions[id])
			assert.Len(t, recreated.ObjectVersions[id], 64)
			assert.NotEqual(t, original.Digest, recreated.Digest)
			// Keep only the vendor so this complete source explicitly removes the model.
			source.Entries = nil
			for _, entry := range baseline.Entries {
				if entry.Kind == "vendor" {
					source.Entries = append(source.Entries, entry)
				}
			}
			source.Coverage["model"] = 0
			source.Digest, err = catalogmanifest.SnapshotDigest(source)
			require.NoError(t, err)
			removed, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
			require.NoError(t, err)
			for _, change := range removed.Changes {
				if change.Kind == "model" {
					assert.Equal(t, "blocked", change.Action)
					assert.Equal(t, "recreated_object_not_managed", change.Reason)
				}
			}
			_, err = SaveCatalogSyncBaselineTx(db, source, original.ObjectVersions, 0)
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
		})
	}
}

func TestCatalogSyncStoreOptionWritersAndNoop(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}))
			require.NoError(t, MigrateCatalogSync(db))
			previousOptions, previousRate := maps.Clone(common.OptionMap), operation_setting.USDExchangeRate
			previousAddress := system_setting.ServerAddress
			settings := config.GlobalConfig.ExportAllConfigs()
			t.Cleanup(func() {
				common.OptionMap = previousOptions
				operation_setting.USDExchangeRate = previousRate
				system_setting.ServerAddress = previousAddress
				require.NoError(t, config.GlobalConfig.LoadFromDB(settings))
			})
			common.OptionMap = map[string]string{}
			system_setting.ServerAddress = "https://catalog.example.com"
			*system_setting.GetPasskeySettings() = system_setting.PasskeySettings{RPID: "catalog.example.com", Origins: "https://catalog.example.com"}
			// Observe lock coverage before any option read, including reloads.
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("catalog-test-option-read", func(tx *gorm.DB) {
				if tx.Statement.Table != "options" {
					return
				}
				if catalogBarrier.TryRLock() {
					catalogBarrier.RUnlock()
					t.Error("option writer/reload read without catalog barrier")
				}
			}))
			t.Cleanup(func() { _ = db.Callback().Query().Remove("catalog-test-option-read") })
			require.NoError(t, UpdateOption("molii_grok_price.image_standard_1k", "0.125"))
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			assert.Equal(t, int64(1), before.Revision)
			require.NoError(t, UpdateOption("molii_grok_price.image_standard_1k", "0.125"))
			loadOptionsFromDatabase()
			require.NoError(t, UpdateOption("USDExchangeRate", "7.25"))
			var after CatalogSyncState
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Equal(t, before.Revision, after.Revision)
			assert.Equal(t, before.CurrentDigest, after.CurrentDigest)
			assert.Equal(t, 7.25, operation_setting.USDExchangeRate)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"molii_grok_price.image_standard_1k": "0.25", "ServerAddress": "https://catalog.example.com"}))
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Equal(t, before.Revision+1, after.Revision, "mixed passkey/catalog options keep the common transaction")
			// Existing anchor counter activity from a no-op reorder is not a revision.
			require.NoError(t, ReorderModels(nil))
			var reordered CatalogSyncState
			require.NoError(t, db.First(&reordered, CatalogSyncStateID).Error)
			assert.Equal(t, after.Revision, reordered.Revision)
		})
	}
}

// These are the persisted fields from released v1.0.0-rc.40 (model_meta.go and
// vendor_meta.go), not fields reflected out of the new Model under test.
type catalogReleasedModel struct {
	Id           int
	ModelName    string         `gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Description  string         `gorm:"type:text"`
	Icon         string         `gorm:"type:varchar(128)"`
	Tags         string         `gorm:"type:varchar(255)"`
	VendorID     int            `gorm:"index:idx_models_vendor_id"`
	Endpoints    string         `gorm:"type:text"`
	Status       int            `gorm:"default:1"`
	SyncOfficial int            `gorm:"default:1"`
	CreatedTime  int64          `gorm:"bigint"`
	UpdatedTime  int64          `gorm:"bigint"`
	DeletedAt    gorm.DeletedAt `gorm:"index:idx_models_deleted_at;uniqueIndex:uk_model_name_delete_at,priority:2"`
	NameRule     int            `gorm:"default:0"`
}

func (catalogReleasedModel) TableName() string { return "models" }

type catalogReleasedVendor struct {
	Id          int
	Name        string         `gorm:"size:128;not null;uniqueIndex:uk_vendor_name_delete_at,priority:1"`
	Description string         `gorm:"type:text"`
	Icon        string         `gorm:"type:varchar(128)"`
	Status      int            `gorm:"default:1"`
	CreatedTime int64          `gorm:"bigint"`
	UpdatedTime int64          `gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `gorm:"index:idx_vendors_deleted_at;uniqueIndex:uk_vendor_name_delete_at,priority:2"`
}

func (catalogReleasedVendor) TableName() string { return "vendors" }

func TestCatalogSyncStoreReleasedUpgrade(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&catalogReleasedModel{}, &catalogReleasedVendor{}, &Option{}))
			vendor := catalogReleasedVendor{Name: "released-vendor", Description: "preserved", Status: 1}
			require.NoError(t, db.Create(&vendor).Error)
			old := catalogReleasedModel{ModelName: "released-model", Description: "do not replace", VendorID: vendor.Id, CreatedTime: 123, UpdatedTime: 456}
			require.NoError(t, db.Create(&old).Error)
			for attempt := range 3 {
				recorder := &migrationSQLRecorder{}
				migrationDB := db.Session(&gorm.Session{Logger: recorder})
				require.NoError(t, migrationDB.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &marketplaceOrderLock{}))
				require.NoError(t, MigrateCatalogSync(migrationDB))
				require.NoError(t, ensureModelMarketplaceMetadataSchema(migrationDB))
				if attempt > 0 {
					assert.Empty(t, recorder.schemaMutations(), "repeat migration must not mutate schema")
				}
				var model Model
				require.NoError(t, db.First(&model, old.Id).Error)
				assert.Equal(t, "do not replace", model.Description)
				assert.Equal(t, vendor.Id, model.VendorID)
				assert.Equal(t, int64(123), model.CreatedTime)
				assert.Empty(t, model.SupportedParameters)
				assert.True(t, db.Migrator().HasIndex(&Model{}, "uk_model_name_delete_at"))
				assert.True(t, db.Migrator().HasIndex(&Vendor{}, "uk_vendor_name_delete_at"))
			}
		})
	}
}

func TestCatalogSyncSourceConcurrentMutation(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			if engine == "sqlite" {
				require.NoError(t, db.Exec("PRAGMA journal_mode=WAL").Error)
			}
			require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}))
			require.NoError(t, MigrateCatalogSync(db))
			vendor := Vendor{Name: "concurrent", Description: "before", Status: 1}
			require.NoError(t, db.Create(&vendor).Error)
			require.NoError(t, db.Create(&Model{ModelName: "concurrent-model", VendorID: vendor.Id, BillingCurrency: "USD"}).Error)
			// Simulate an independent committed DB writer between source queries.
			// A process mutex alone cannot protect PostgreSQL READ COMMITTED.
			require.NoError(t, WithCatalogReadSnapshot(context.Background(), func(tx *gorm.DB) error {
				var first Vendor
				if err := tx.First(&first, vendor.Id).Error; err != nil {
					return err
				}
				committed := make(chan error, 1)
				go func() {
					committed <- db.Transaction(func(write *gorm.DB) error {
						if err := write.Model(&Vendor{}).Where("id = ?", vendor.Id).Update("description", "after").Error; err != nil {
							return err
						}
						return write.Model(&Model{}).Where("model_name = ?", "concurrent-model").Update("billing_currency", "CNY").Error
					})
				}()
				if err := <-committed; err != nil {
					return err
				}
				snapshot, err := ExportManagedCatalogTx(context.Background(), tx, "dev")
				if err != nil {
					return err
				}
				for _, entry := range snapshot.Entries {
					if entry.Kind == "vendor" {
						var value catalogmanifest.VendorValue
						require.NoError(t, common.UnmarshalJsonStr(entry.Value, &value))
						assert.Equal(t, "before", value.Description)
					}
					if entry.Kind == "model" {
						var value catalogmanifest.ModelValue
						require.NoError(t, common.UnmarshalJsonStr(entry.Value, &value))
						assert.Equal(t, "USD", value.BillingCurrency)
					}
				}
				return nil
			}))
			// Observe the production writer at its DB boundary: it must exclude
			// billing/snapshot readers before any metadata reads or writes begin.
			var observed atomic.Bool
			writerPaused, releaseWriter := make(chan struct{}), make(chan struct{})
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("catalog-test-writer", func(tx *gorm.DB) {
				if tx.Statement.Table != "vendors" {
					return
				}
				observed.Store(true)
				if catalogBarrier.TryRLock() {
					catalogBarrier.RUnlock()
					t.Error("ordinary vendor writer did not hold catalog barrier")
				}
				close(writerPaused)
				<-releaseWriter
			}))
			t.Cleanup(func() { _ = db.Callback().Create().Remove("catalog-test-writer") })
			written := make(chan error, 1)
			go func() { written <- (&Vendor{Name: "normal-writer", Status: 1}).Insert() }()
			<-writerPaused
			readerStarted, readDone := make(chan struct{}), make(chan error, 1)
			var concurrent catalogmanifest.Snapshot
			go func() {
				close(readerStarted)
				readDone <- WithCatalogReadSnapshot(context.Background(), func(tx *gorm.DB) error {
					var err error
					concurrent, err = ExportManagedCatalogTx(context.Background(), tx, "dev")
					return err
				})
			}()
			<-readerStarted
			close(releaseWriter)
			require.NoError(t, <-written)
			require.NoError(t, <-readDone)
			assert.Equal(t, 2, concurrent.Coverage["vendor"], "export waits for the real ordinary writer to finish")
			assert.True(t, observed.Load())
			var state CatalogSyncState
			require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
			assert.Equal(t, int64(1), state.Revision)
		})
	}
}

// Every server fixture gets its own disposable database, and accepts only a
// loopback test DSN. Production dialectors are essential migration coverage.
func catalogSyncTestDB(t *testing.T, engine string) *gorm.DB {
	t.Helper()
	engine = catalogPostgresFixtureEngine(t, engine)
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	var db *gorm.DB
	var err error
	name := fmt.Sprintf("catalog_sync_%d", time.Now().UnixNano())
	switch engine {
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "catalog.db")), cfg)
	case "postgres":
		dsn := os.Getenv("TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("TEST_POSTGRES_DSN is not set")
		}
		u, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
		admin, openErr := gorm.Open(postgres.Open(dsn), cfg)
		require.NoError(t, openErr)
		require.NoError(t, admin.Exec(`CREATE DATABASE "`+name+`"`).Error)
		t.Cleanup(func() {
			require.NoError(t, admin.Exec(`DROP DATABASE "`+name+`"`).Error)
			sqlDB, _ := admin.DB()
			_ = sqlDB.Close()
		})
		u.Path = "/" + name
		db, err = gorm.Open(postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{DSN: u.String(), PreferSimpleProtocol: true}}}, cfg)
	case "mysql":
		dsn := os.Getenv("TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("TEST_MYSQL_DSN is not set")
		}
		parsed, parseErr := mysqldriver.ParseDSN(dsn)
		require.NoError(t, parseErr)
		require.True(t, strings.HasPrefix(parsed.Addr, "127.0.0.1:") || strings.HasPrefix(parsed.Addr, "localhost:"))
		admin, openErr := gorm.Open(mysql.Open(dsn), cfg)
		require.NoError(t, openErr)
		require.NoError(t, admin.Exec("CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4").Error)
		t.Cleanup(func() {
			require.NoError(t, admin.Exec("DROP DATABASE `"+name+"`").Error)
			sqlDB, _ := admin.DB()
			_ = sqlDB.Close()
		})
		parsed.DBName = name
		db, err = gorm.Open(mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{DSN: parsed.FormatDSN()}}}, cfg)
	}
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	var version string
	query := "SELECT version()"
	if engine == "sqlite" {
		query = "SELECT sqlite_version()"
	}
	require.NoError(t, db.Raw(query).Scan(&version).Error)
	t.Logf("%s: %s", engine, version)
	previous, previousType := DB, common.MainDatabaseType()
	DB = db
	switch engine {
	case "sqlite":
		common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	case "mysql":
		common.SetMainDatabaseType(common.DatabaseTypeMySQL)
	case "postgres":
		common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
	}
	initCol()
	t.Cleanup(func() { DB = previous; common.SetMainDatabaseType(previousType); initCol() })
	catalogPostgresLegacyPrerequisite(t, db, "ordinary")
	return db
}

// Removing portable handling of TEXT defaults must break actual fresh MySQL
// migration; reflecting into an invented fixture model cannot satisfy this.
func TestCatalogSyncStoreMigrations(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogSyncTestDB(t, engine)
			require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &marketplaceOrderLock{}))
			require.NoError(t, db.Create(&Model{ModelName: "portable-model", DescriptionEN: "existing description", SupportedParameters: []string{"stream"}}).Error)
			for range 2 {
				require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &marketplaceOrderLock{}))
			}
			var row Model
			require.NoError(t, db.First(&row).Error)
			assert.Equal(t, "existing description", row.DescriptionEN)
			assert.Equal(t, []string{"stream"}, row.SupportedParameters)
			assert.True(t, db.Migrator().HasIndex(&Model{}, "uk_model_name_delete_at"))
		})
	}
}

// The released Channel declaration predates management credentials. Keep this
// fixture independent of Channel so a broken production TEXT default is visible.
type catalogReleasedChannel struct {
	Id                 int
	Type               int    `gorm:"default:0"`
	Key                string `gorm:"not null"`
	OpenAIOrganization *string
	TestModel          *string
	Status             int    `gorm:"default:1"`
	Name               string `gorm:"index"`
	Weight             *uint  `gorm:"default:0"`
	CreatedTime        int64  `gorm:"bigint"`
	TestTime           int64  `gorm:"bigint"`
	ResponseTime       int
	BaseURL            *string `gorm:"column:base_url;default:''"`
	Other              string
	Balance            float64
	BalanceUpdatedTime int64 `gorm:"bigint"`
	Models             string
	Group              string  `gorm:"type:varchar(64);default:'default'"`
	UsedQuota          int64   `gorm:"bigint;default:0"`
	ModelMapping       *string `gorm:"type:text"`
	StatusCodeMapping  *string `gorm:"type:varchar(1024);default:''"`
	Priority           *int64  `gorm:"bigint;default:0"`
	AutoBan            *int    `gorm:"default:1"`
	OtherInfo          string
	Tag                *string     `gorm:"index"`
	Setting            *string     `gorm:"type:text"`
	ParamOverride      *string     `gorm:"type:text"`
	HeaderOverride     *string     `gorm:"type:text"`
	Remark             *string     `gorm:"type:varchar(255)"`
	ChannelInfo        ChannelInfo `gorm:"type:json"`
	OtherSettings      string      `gorm:"column:settings"`
}

func (catalogReleasedChannel) TableName() string { return "channels" }

func TestCatalogSyncChannelMigrations(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		for _, upgrade := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/upgrade=%t", engine, upgrade), func(t *testing.T) {
				db := catalogSyncTestDB(t, engine)
				if upgrade {
					require.NoError(t, db.AutoMigrate(&catalogReleasedChannel{}))
					require.NoError(t, db.Create(&catalogReleasedChannel{Id: 7, Key: "fixture-only", Name: "retained", Models: "fixture-model", CreatedTime: 123}).Error)
				}
				credentialChannel := Channel{Key: "fixture-only", Name: "configured", MoliiGrokManagementAccessToken: "fixture-management-token"}
				for pass := range 3 {
					recorder := &migrationSQLRecorder{}
					require.NoError(t, db.Session(&gorm.Session{Logger: recorder}).AutoMigrate(&Channel{}))
					if pass > 0 {
						assert.Empty(t, recorder.schemaMutations())
					} else {
						require.NoError(t, db.Create(&credentialChannel).Error)
					}
				}
				channel := Channel{Key: "fixture-only", Name: "new"}
				require.NoError(t, db.Create(&channel).Error)
				var got Channel
				require.NoError(t, db.First(&got, channel.Id).Error)
				assert.Empty(t, got.MoliiGrokManagementAccessToken)
				assert.True(t, db.Migrator().HasIndex(&Channel{}, "idx_channels_name"))
				got = Channel{}
				require.NoError(t, db.First(&got, credentialChannel.Id).Error)
				assert.Equal(t, "fixture-management-token", got.MoliiGrokManagementAccessToken)
				if upgrade {
					got = Channel{}
					require.NoError(t, db.First(&got, 7).Error)
					assert.Equal(t, "retained", got.Name)
					assert.Equal(t, "fixture-model", got.Models)
					assert.Equal(t, int64(123), got.CreatedTime)
					assert.Empty(t, got.MoliiGrokManagementAccessToken)
				}
			})
		}
	}
}

func catalogFenceTestDB(t *testing.T, engine string) *gorm.DB {
	t.Helper()
	db := catalogSyncTestDB(t, engine)
	require.NoError(t, db.AutoMigrate(&Model{}, &Vendor{}, &Option{}, &Channel{}, &Ability{}, &Task{}, &TaskPlugin{}, &Midjourney{}, &SystemTask{}))
	require.NoError(t, MigrateCatalogSync(db))
	catalogPostgresLegacyPrerequisite(t, db, "fence")
	return db
}

// Removing any physical table fence permits a bypass writer to commit while
// the authoritative catalog transaction is still open, including empty tables.
func TestCatalogSyncReferenceFences(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			pool, err := db.DB()
			require.NoError(t, err)
			registry := jsplugin.NewRegistry()
			for _, fixture := range []struct {
				name   string
				insert string
				update string
				remove string
			}{
				{"channels", "INSERT INTO channels (id, " + commonKeyCol + ", status, molii_grok_management_access_token) VALUES (10, 'fixture', 2, '')", "UPDATE channels SET status = 1 WHERE id = 10", "DELETE FROM channels WHERE id = 10"},
				{"abilities", "INSERT INTO abilities (" + commonGroupCol + ", model, channel_id, enabled) VALUES ('g', 'm', 10, " + commonFalseVal + ")", "UPDATE abilities SET enabled = " + commonTrueVal + " WHERE channel_id = 10", "DELETE FROM abilities WHERE channel_id = 10"},
				{"tasks", "INSERT INTO tasks (id, status) VALUES (10, 'SUCCESS')", "UPDATE tasks SET status = 'IN_PROGRESS' WHERE id = 10", "DELETE FROM tasks WHERE id = 10"},
				{"task_plugins", "INSERT INTO task_plugins (id, " + commonKeyCol + ", version, source_hash, source, api_version, enabled, active, created_at) VALUES (10, 'fixture', '1', 'fixture', '', 1, " + commonFalseVal + ", " + commonFalseVal + ", 0)", "UPDATE task_plugins SET enabled = " + commonTrueVal + ", active = " + commonTrueVal + " WHERE id = 10", "DELETE FROM task_plugins WHERE id = 10"},
				{"options", "INSERT INTO options (" + commonKeyCol + ", value) VALUES ('TaskPluginEnabled', 'false')", "UPDATE options SET value = 'true' WHERE " + commonKeyCol + " = 'TaskPluginEnabled'", "DELETE FROM options WHERE " + commonKeyCol + " = 'TaskPluginEnabled'"},
				{"vendors", "INSERT INTO vendors (id, name, deleted_at) VALUES (10, 'fixture', '2020-01-01')", "UPDATE vendors SET deleted_at = NULL WHERE id = 10", "DELETE FROM vendors WHERE id = 10"},
				{"models", "INSERT INTO models (id, model_name, description_en, supported_parameters, supported_resolutions, supported_aspect_ratios, output_formats, reference_modalities, deleted_at) VALUES (10, 'fixture', '', '[]', '[]', '[]', '[]', '[]', '2020-01-01')", "UPDATE models SET deleted_at = NULL WHERE id = 10", "DELETE FROM models WHERE id = 10"},
				{"midjourneys", "INSERT INTO midjourneys (id, status) VALUES (10, 'SUCCESS')", "UPDATE midjourneys SET status = 'IN_PROGRESS' WHERE id = 10", "DELETE FROM midjourneys WHERE id = 10"},
				{"system_tasks", "INSERT INTO system_tasks (id, status) VALUES (10, 'succeeded')", "UPDATE system_tasks SET status = 'pending' WHERE id = 10", "DELETE FROM system_tasks WHERE id = 10"},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					// ExecContext on an independently acquired connection is the actual
					// writer attempt; no goroutine-start signal or sleep stands in for it.
					second := strings.NewReplacer("(10,", "(20,", ", 10,", ", 20,", "'fixture'", "'second'", "'TaskPluginEnabled'", "'TaskPluginDisabledFactoryKeys'").Replace(fixture.insert)
					reverse := strings.NewReplacer("status = 1", "status = 2", "status = 'IN_PROGRESS'", "status = 'SUCCESS'", "enabled = "+commonTrueVal, "enabled = "+commonFalseVal, "active = "+commonTrueVal, "active = "+commonFalseVal, "value = 'true'", "value = 'false'", "deleted_at = NULL", "deleted_at = '2020-01-01'", "status = 'pending'", "status = 'succeeded'").Replace(fixture.update)
					for _, statement := range []string{fixture.insert, second, fixture.update, reverse, fixture.remove} {
						err := WithCatalogWriteBarrier(func() error {
							return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, state *CatalogSyncState, pin *jsplugin.GenerationPin) error {
								require.NotNil(t, state)
								require.NotNil(t, pin.Generation)
								catalogAssertWriterBlocked(t, pool, engine, statement)
								return nil
							})
						})
						require.NoError(t, err)
						// The same write is valid after the root COMMIT and all fences release.
						require.NoError(t, db.Exec(statement).Error)
					}
				})
			}
		})
	}
}

func catalogAssertWriterBlocked(t *testing.T, pool *sql.DB, engine, statement string) {
	t.Helper()
	conn, err := pool.Conn(context.Background())
	require.NoError(t, err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if engine == "sqlite" {
		_, err = conn.ExecContext(ctx, "PRAGMA busy_timeout = 0")
		require.NoError(t, err)
	}
	writer, err := conn.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer writer.Rollback()
	_, err = writer.ExecContext(ctx, statement)
	require.Error(t, err, "bypass writer completed before catalog commit: %s", statement)
}

func TestCatalogSyncReferenceRootRollback(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			stopped := errors.New("stop before commit")
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					require.NoError(t, tx.Create(&Option{Key: "rollback", Value: "uncommitted"}).Error)
					called := false
					err := catalogReferenceTransaction(context.Background(), tx, registry, func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error { called = true; return nil })
					require.Error(t, err, "nested/savepoint entry must be refused")
					require.False(t, called)
					return stopped
				})
			})
			require.ErrorIs(t, err, stopped)
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "rollback").Count(&count).Error)
			assert.Zero(t, count)
			require.NoError(t, db.Create(&Option{Key: "after-rollback"}).Error)
			// Explicit cancellation rejects the root before any business callback.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err = catalogReferenceTransaction(ctx, db, registry, func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("canceled callback ran")
				return nil
			})
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

// The observer delegates every operation to the actual database transaction.
// It only pauses the real COMMIT boundary, where callback-only leases are wrong.
type catalogCommitObserver struct {
	gorm.ConnPool
	commit   func() error
	rollback func() error
}

func (o *catalogCommitObserver) Commit() error   { return o.commit() }
func (o *catalogCommitObserver) Rollback() error { return o.rollback() }

func TestCatalogSyncReferenceCommitLease(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			before := registry.Generation().Number
			publication := make(chan struct{})
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, pin *jsplugin.GenerationPin) error {
					require.Equal(t, before, pin.Generation.Number)
					require.NoError(t, tx.Create(&Option{Key: "lease-commit", Value: "durable"}).Error)
					real := tx.Statement.ConnPool.(*sql.Tx)
					tx.Statement.ConnPool = &catalogCommitObserver{ConnPool: real, rollback: real.Rollback, commit: func() error {
						started := make(chan struct{})
						go func() { close(started); registry.SetEnabled(false); close(publication) }()
						<-started
						require.Eventually(t, func() bool {
							probe, err := registry.TryPinGeneration()
							if probe != nil {
								probe.Release()
							}
							return errors.Is(err, jsplugin.ErrGenerationBusy)
						}, time.Second, time.Millisecond, "registry writer must actually be queued at COMMIT")
						assert.Equal(t, before, registry.Generation().Number, "lease released on callback return")
						select {
						case <-publication:
							t.Error("registry switched before COMMIT")
						default:
						}
						err := real.Commit()
						assert.Equal(t, before, registry.Generation().Number, "lease must span actual COMMIT return")
						return err
					}}
					return nil
				})
			})
			require.NoError(t, err)
			select {
			case <-publication:
			case <-time.After(3 * time.Second):
				t.Fatal("registry lease leaked after root return")
			}
			assert.Greater(t, registry.Generation().Number, before)
			var option Option
			require.NoError(t, db.First(&option, commonKeyCol+" = ?", "lease-commit").Error)
			assert.Equal(t, "durable", option.Value)
			// A lost COMMIT acknowledgement must remain distinguishable even when
			// the actual database committed. The helper must not replay the callback.
			calls := 0
			err = WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					calls++
					require.NoError(t, tx.Create(&Option{Key: "uncertain", Value: "committed"}).Error)
					real := tx.Statement.ConnPool.(*sql.Tx)
					tx.Statement.ConnPool = &catalogCommitObserver{ConnPool: real, rollback: real.Rollback, commit: func() error {
						if err := real.Commit(); err != nil {
							return err
						}
						return errors.New("test transport lost commit acknowledgement")
					}}
					return nil
				})
			})
			require.ErrorIs(t, err, ErrCatalogCommitUncertain)
			assert.Equal(t, 1, calls)
			option = Option{}
			require.NoError(t, db.First(&option, commonKeyCol+" = ?", "uncertain").Error)
			assert.Equal(t, "committed", option.Value)
		})
	}
}

// Loan a real fixture connection while its owning sql.Conn.Raw callback holds
// exclusive ownership. Only native rollback completion is paused; statements,
// transaction creation, cancellation and the database locks remain real.
type catalogRollbackConnector struct {
	connection *catalogRollbackConnection
	driver     driver.Driver
}

func (c catalogRollbackConnector) Connect(context.Context) (driver.Conn, error) {
	return c.connection, nil
}
func (c catalogRollbackConnector) Driver() driver.Driver { return c.driver }

type catalogRollbackConnection struct {
	driver.Conn
	started chan struct{}
	resume  chan struct{}
}

// The original fixture sql.Conn owns and closes this loaned native connection.
func (c *catalogRollbackConnection) Close() error { return nil }
func (c *catalogRollbackConnection) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &catalogPausedRollback{Tx: tx, connection: c}, nil
}

type catalogPausedRollback struct {
	driver.Tx
	connection *catalogRollbackConnection
}

func (tx *catalogPausedRollback) Rollback() error {
	close(tx.connection.started)
	<-tx.connection.resume
	return tx.Tx.Rollback()
}

func TestCatalogSyncReferenceCancellationRollbackLease(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			pool, err := db.DB()
			require.NoError(t, err)
			loan, err := pool.Conn(context.Background())
			require.NoError(t, err)
			defer loan.Close()
			require.NoError(t, loan.Raw(func(native any) error {
				connection := &catalogRollbackConnection{Conn: native.(driver.Conn), started: make(chan struct{}), resume: make(chan struct{})}
				observed := sql.OpenDB(catalogRollbackConnector{connection: connection, driver: pool.Driver()})
				observed.SetMaxOpenConns(1)
				defer observed.Close()
				root := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
				root.Statement.ConnPool = observed
				registry := jsplugin.NewRegistry()
				before := registry.Generation().Number
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				callbackReady := make(chan struct{})
				callbackReturn := make(chan struct{})
				returnCallback := sync.OnceFunc(func() { close(callbackReturn) })
				resumeRollback := sync.OnceFunc(func() { close(connection.resume) })
				rollbackReturned := make(chan error, 1)
				result := make(chan error, 1)
				rootDone := make(chan struct{})
				publication := make(chan struct{})
				defer func() {
					cancel()
					returnCallback()
					resumeRollback()
					<-rootDone
				}()
				go func() {
					defer close(rootDone)
					result <- WithCatalogWriteBarrier(func() error {
						return catalogReferenceTransaction(ctx, root, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
							if err := tx.Create(&Option{Key: "cancel-rollback", Value: "must-not-commit"}).Error; err != nil {
								return err
							}
							real := tx.Statement.ConnPool.(*sql.Tx)
							tx.Statement.ConnPool = &catalogCommitObserver{ConnPool: real, commit: real.Commit, rollback: func() error {
								err := real.Rollback()
								rollbackReturned <- err
								return err
							}}
							close(callbackReady)
							<-callbackReturn
							return ctx.Err()
						})
					})
				}()
				select {
				case <-callbackReady:
				case err := <-result:
					return fmt.Errorf("root failed before cancellation: %w", err)
				case <-time.After(5 * time.Second):
					t.Fatal("root did not acquire its lease")
				}
				go func() { registry.SetEnabled(false); close(publication) }()
				require.Eventually(t, func() bool {
					probe, err := registry.TryPinGeneration()
					if probe != nil {
						probe.Release()
					}
					return errors.Is(err, jsplugin.ErrGenerationBusy)
				}, time.Second, time.Millisecond, "publisher must actually queue behind the lease")
				cancel()
				select {
				case <-connection.started:
				case <-time.After(3 * time.Second):
					t.Fatal("database/sql cancellation did not enter native rollback")
				}
				returnCallback()
				select {
				case err := <-rollbackReturned:
					assert.ErrorIs(t, err, sql.ErrTxDone, "cancellation worker must own the still-active rollback")
				case <-time.After(3 * time.Second):
					t.Error("helper did not attempt rollback")
				}
				// Both actual cancellation/rollback entry and the queued registry
				// writer are acknowledged; this interval checks absence of publication,
				// not a sleep used to assume either operation has started.
				select {
				case <-publication:
					t.Error("registry published before native cancellation rollback completed")
				case <-time.After(100 * time.Millisecond):
				}
				assert.Equal(t, before, registry.Generation().Number)
				resumeRollback()
				require.ErrorIs(t, <-result, context.Canceled)
				select {
				case <-publication:
				case <-time.After(3 * time.Second):
					t.Error("registry pin leaked after completed rollback")
				}
				assert.Greater(t, registry.Generation().Number, before)
				return nil
			}))
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "cancel-rollback").Count(&count).Error)
			assert.Zero(t, count, "actual native rollback must discard the write")
			require.NoError(t, db.Create(&Option{Key: "after-cancel-rollback"}).Error)
		})
	}
}

func TestCatalogSyncReferenceSchemaAndFailure(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			callback := func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("unsafe schema reached callback")
				return nil
			}
			if engine == "mysql" {
				// The production legacy migration accepts this exact unique layout.
				require.NoError(t, db.Exec("ALTER TABLE options DROP PRIMARY KEY, ADD UNIQUE INDEX legacy_option_key (`key`)").Error)
				require.NoError(t, WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
						pool, err := db.DB()
						require.NoError(t, err)
						catalogAssertWriterBlocked(t, pool, engine, "INSERT INTO options (`key`, value) VALUES ('legacy-gap', '')")
						return tx.Create(&Option{Key: "legacy-index"}).Error
					})
				}))
				require.NoError(t, db.Exec("ALTER TABLE options DROP INDEX legacy_option_key").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, registry, callback))
				require.NoError(t, db.Exec("ALTER TABLE options ADD UNIQUE INDEX legacy_option_key (`key`)").Error)
				require.NoError(t, db.Exec("ALTER TABLE options ENGINE=MyISAM").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, registry, callback))
				require.NoError(t, db.Exec("ALTER TABLE options ENGINE=InnoDB").Error)
			}
			// A late missing relation cannot be treated as an empty reference set.
			require.NoError(t, db.Migrator().DropTable(&Task{}))
			require.Error(t, catalogReferenceTransaction(context.Background(), db, registry, callback))
			require.NoError(t, db.AutoMigrate(&Task{}))
			// Cancellation after a real scan obtained rows must release partial fences.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			seen := false
			if engine == "postgres" {
				require.NoError(t, db.Callback().Raw().After("gorm:raw").Register("cancel-fence", func(tx *gorm.DB) {
					if strings.HasPrefix(tx.Statement.SQL.String(), "LOCK TABLE") && strings.Contains(tx.Statement.SQL.String(), "channels") {
						seen = true
						cancel()
					}
				}))
			} else {
				require.NoError(t, db.Callback().Row().After("gorm:row").Register("cancel-fence", func(tx *gorm.DB) {
					if strings.HasPrefix(tx.Statement.SQL.String(), "SELECT `id` FROM `channels`") {
						seen = true
						cancel()
					}
				}))
			}
			err := WithCatalogWriteBarrier(func() error { return catalogReferenceTransaction(ctx, db, registry, callback) })
			require.Error(t, err)
			assert.True(t, seen, "cancellation must happen after actual partial fence acquisition")
			require.NoError(t, db.Callback().Raw().Remove("cancel-fence"))
			require.NoError(t, db.Callback().Row().Remove("cancel-fence"))
			require.NoError(t, db.Create(&Channel{Key: "after-cancel"}).Error)
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					return tx.Create(&Option{Key: "after-failure"}).Error
				})
			}))
		})
	}
}

// Poll actual server lock wait state; a goroutine start or elapsed sleep cannot
// prove the fence acquisition reached the database before the other commit.
func catalogWaitForDBLock(t *testing.T, db *gorm.DB, engine string, outcomes ...<-chan error) {
	t.Helper()
	query := "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'"
	interval := 10 * time.Millisecond
	if engine == "mysql" {
		var version string
		require.NoError(t, db.Raw("SELECT VERSION()").Scan(&version).Error)
		query = "SELECT count(*) FROM performance_schema.data_lock_waits"
		if strings.HasPrefix(version, "5.7.") {
			query = "SELECT count(*) FROM information_schema.innodb_lock_waits"
			// 5.7 caches InnoDB transaction metadata while queries arrive less
			// than 0.1s apart; faster polling can keep an empty snapshot forever.
			interval = 150 * time.Millisecond
		}
	}
	require.Eventually(t, func() bool {
		for _, outcome := range outcomes {
			select {
			case err := <-outcome:
				t.Fatalf("root returned before database lock wait: %v", err)
			default:
			}
		}
		var count int64
		err := db.Raw(query).Scan(&count).Error
		return err == nil && count > 0
	}, 5*time.Second, interval, "competing SQL must actually reach a lock wait")
}

func TestCatalogSyncReferenceWaitedSnapshot(t *testing.T) {
	for _, engine := range []string{"mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			require.NoError(t, db.Create(&Channel{Id: 10, Key: "fixture", Status: 2}).Error)
			// An opposite pool default catches accidental default-isolation use.
			pool, err := db.DB()
			require.NoError(t, err)
			conn, err := pool.Conn(context.Background())
			require.NoError(t, err)
			writer, err := conn.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			defer conn.Close()
			defer writer.Rollback()
			_, err = writer.Exec("UPDATE channels SET status = 1 WHERE id = 10")
			require.NoError(t, err)
			finished := make(chan error, 1)
			go func() {
				finished <- WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
						var status int
						if err := tx.Model(&Channel{}).Select("status").Where("id = ?", 10).Scan(&status).Error; err != nil {
							return err
						}
						if status != 1 {
							return fmt.Errorf("stale pre-wait reference snapshot: status=%d", status)
						}
						return nil
					})
				})
			}()
			catalogWaitForDBLock(t, db, engine, finished)
			require.NoError(t, writer.Commit())
			require.NoError(t, <-finished)
			// Now hold the same writer until the root deadline; all earlier fences
			// and the anchor must roll back without entering business code.
			writer, err = conn.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			_, err = writer.Exec("UPDATE channels SET status = 2 WHERE id = 10")
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
			defer cancel()
			go func() {
				finished <- WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(ctx, db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
						return errors.New("unexpected callback")
					})
				})
			}()
			catalogWaitForDBLock(t, db, engine)
			require.Error(t, <-finished)
			require.NoError(t, writer.Rollback())
			require.NoError(t, db.Create(&Ability{Group: "g", Model: "after-timeout", ChannelId: 10}).Error)
		})
	}
}

func TestCatalogSyncReferenceMySQLUnsafeGaps(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_UNSAFE_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_UNSAFE_DSN is not set")
	}
	t.Setenv("TEST_MYSQL_DSN", dsn)
	db := catalogFenceTestDB(t, "mysql")
	var unsafe int
	require.NoError(t, db.Raw("SELECT @@global.innodb_locks_unsafe_for_binlog").Scan(&unsafe).Error)
	require.Equal(t, 1, unsafe)
	err := catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
		t.Error("unsafe gap configuration admitted")
		return nil
	})
	require.ErrorContains(t, err, "gap locking")
}

func TestCatalogSyncReferenceMySQLTemporaryShadow(t *testing.T) {
	db := catalogFenceTestDB(t, "mysql")
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	require.NoError(t, db.Exec("CREATE TEMPORARY TABLE channels (id bigint PRIMARY KEY)").Error)
	err = catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
		t.Error("temporary shadow hid persistent references")
		return nil
	})
	require.Error(t, err)
	require.NoError(t, db.Exec("DROP TEMPORARY TABLE channels").Error)
}

func TestCatalogSyncReferenceMySQLAnchorEngineRace(t *testing.T) {
	db := catalogFenceTestDB(t, "mysql")
	var before marketplaceOrderLock
	require.NoError(t, db.First(&before).Error)
	raced := false
	swap := func(tx *gorm.DB) {
		query := tx.Statement.SQL.String()
		if !raced && (strings.HasPrefix(query, "UPDATE `marketplace_order_locks`") || strings.HasPrefix(query, "SELECT `name` FROM `marketplace_order_locks`")) {
			raced = true
			require.NoError(t, db.Exec("ALTER TABLE marketplace_order_locks ENGINE=MyISAM").Error)
		}
	}
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("swap-anchor-engine", swap))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("swap-anchor-engine", swap))
	err := catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
		t.Error("engine race reached callback")
		return nil
	})
	require.Error(t, err)
	require.NoError(t, db.Callback().Raw().Remove("swap-anchor-engine"))
	require.NoError(t, db.Callback().Row().Remove("swap-anchor-engine"))
	assert.True(t, raced)
	var after marketplaceOrderLock
	require.NoError(t, db.First(&after).Error)
	assert.Equal(t, before.Version, after.Version, "unsupported engine must not leave a nontransactional anchor write")
}

func TestCatalogSyncReferenceRangesAndSession(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			pool, err := db.DB()
			require.NoError(t, err)
			// Seed only one pooled connection, configure a contrary isolation or
			// long busy handler, then let root lease that exact connection first.
			pool.SetMaxIdleConns(1)
			switch engine {
			case "mysql":
				require.NoError(t, db.Exec("SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED").Error)
			case "postgres":
				require.NoError(t, db.Exec("SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL REPEATABLE READ").Error)
			case "sqlite":
				require.NoError(t, db.Exec("PRAGMA busy_timeout = 60000").Error)
			}
			require.NoError(t, db.Create(&[]Channel{{Id: 10, Key: "fixture"}, {Id: 30, Key: "fixture"}}).Error)
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					if engine == "postgres" {
						var isolation string
						require.NoError(t, tx.Raw("SHOW transaction_isolation").Scan(&isolation).Error)
						assert.Equal(t, "read committed", isolation)
					}
					for _, id := range []int{1, 20, 40} {
						catalogAssertWriterBlocked(t, pool, engine, fmt.Sprintf("INSERT INTO channels (id, %s, molii_grok_management_access_token) VALUES (%d, 'fixture', '')", commonKeyCol, id))
					}
					catalogAssertWriterBlocked(t, pool, engine, "INSERT INTO channels ("+commonKeyCol+", molii_grok_management_access_token) VALUES ('auto', '')")
					// SQLite reset must be checked on this exact leased connection.
					if engine == "sqlite" {
						var busy int
						require.NoError(t, tx.Raw("PRAGMA busy_timeout").Scan(&busy).Error)
						assert.Zero(t, busy)
					}
					return nil
				})
			}))
			// Closing spare idle connections above lets the last-returning root
			// replace any writer connection; query exact remaining session value.
			if engine == "sqlite" {
				// Reader above used a real file-backed DEFERRED connection. Normal
				// post-commit writes still work after the temporary timeout override.
				require.NoError(t, db.Create(&Channel{Key: "after-ranges"}).Error)
			}
		})
	}
}

func TestCatalogSyncReferenceTrustedNaming(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			base := catalogFenceTestDB(t, engine)
			pool, err := base.DB()
			require.NoError(t, err)
			cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NamingStrategy: schema.NamingStrategy{TablePrefix: "fence space_"}}
			var dialector gorm.Dialector
			switch engine {
			case "sqlite":
				dialector = sqlite.Dialector{Conn: pool}
			case "mysql":
				dialector = mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{Conn: pool}}}
			case "postgres":
				dialector = postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{Conn: pool, PreferSimpleProtocol: true}}}
			}
			db, err := gorm.Open(dialector, cfg)
			require.NoError(t, err)
			// Use actual production migrations, then rename those physical tables.
			// Custom-prefix migration support is separate from fence resolution.
			for _, model := range []any{&Model{}, &Vendor{}, &Option{}, &Channel{}, &Ability{}, &Task{}, &TaskPlugin{}, &Midjourney{}, &SystemTask{}, &CatalogSyncState{}, &CatalogSyncPlan{}, &CatalogSyncBaseline{}, &CatalogSyncOperation{}} {
				old, renamed := &gorm.Statement{DB: base}, &gorm.Statement{DB: db}
				require.NoError(t, old.Parse(model))
				require.NoError(t, renamed.Parse(model))
				require.NoError(t, base.Migrator().RenameTable(old.Schema.Table, renamed.Schema.Table))
			}
			require.NoError(t, WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					stmt := &gorm.Statement{DB: db}
					require.NoError(t, stmt.Parse(&Channel{}))
					query := "INSERT INTO " + stmt.Quote(stmt.Schema.Table) + " (" + commonKeyCol + ", molii_grok_management_access_token) VALUES ('naming', '')"
					catalogAssertWriterBlocked(t, pool, engine, query)
					return tx.Create(&Channel{Key: "naming"}).Error
				})
			}))
		})
	}
}

func TestCatalogSyncReferenceSessionReset(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			pool.SetMaxIdleConns(1)
			query, expected := "PRAGMA busy_timeout", "60000"
			switch engine {
			case "sqlite":
				require.NoError(t, db.Exec("PRAGMA busy_timeout = 60000").Error)
			case "mysql":
				require.NoError(t, db.Exec("SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED").Error)
				var version string
				require.NoError(t, db.Raw("SELECT VERSION()").Scan(&version).Error)
				query, expected = "SELECT @@session.transaction_isolation", "READ-COMMITTED"
				if strings.HasPrefix(version, "5.7.") {
					query = "SELECT @@session.tx_isolation"
				}
			case "postgres":
				require.NoError(t, db.Exec("SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL REPEATABLE READ").Error)
				query, expected = "SHOW transaction_isolation", "repeatable read"
			}
			for _, failure := range []bool{false, true} {
				err = WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
						if failure {
							return errors.New("rollback")
						}
						return nil
					})
				})
				if failure {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				var actual string
				require.NoError(t, db.Raw(query).Scan(&actual).Error)
				assert.Equal(t, expected, actual, "session configuration leaked across root transaction")
			}
		})
	}
}

func TestCatalogSyncReferencePostgresPrivileges(t *testing.T) {
	db := catalogFenceTestDB(t, "postgres")
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	role := fmt.Sprintf("catalog_fence_role_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE ROLE "+role+" NOLOGIN").Error)
	t.Cleanup(func() {
		require.NoError(t, db.Exec("RESET ROLE").Error)
		require.NoError(t, db.Exec("DROP OWNED BY "+role).Error)
		require.NoError(t, db.Exec("DROP ROLE "+role).Error)
	})
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO "+role).Error)
	require.NoError(t, db.Exec("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO "+role).Error)
	require.NoError(t, db.Exec("SET ROLE "+role).Error)
	run := func() error {
		return WithCatalogWriteBarrier(func() error {
			return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
				return tx.Create(&Option{Key: "ordinary-role"}).Error
			})
		})
	}
	require.NoError(t, run(), "ordinary application table permissions must suffice")
	require.NoError(t, db.Exec("RESET ROLE").Error)
	require.NoError(t, db.Exec("REVOKE UPDATE, DELETE ON channels FROM "+role).Error)
	require.NoError(t, db.Exec("SET ROLE "+role).Error)
	err = run()
	require.ErrorContains(t, err, "permission denied", "SELECT-only channels cannot establish a DML fence")
}

func TestCatalogSyncReferenceDeadlockRollback(t *testing.T) {
	for _, engine := range []string{"mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			require.NoError(t, db.Create(&Channel{Id: 10, Key: "fixture", Status: 2}).Error)
			writer := db.Begin()
			require.NoError(t, writer.Error)
			defer writer.Rollback()
			require.NoError(t, writer.Exec("UPDATE channels SET status = 1 WHERE id = 10").Error)
			finished := make(chan error, 1)
			go func() {
				finished <- WithCatalogWriteBarrier(func() error {
					return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
						return tx.Create(&Option{Key: "deadlock-winner"}).Error
					})
				})
			}()
			catalogWaitForDBLock(t, db, engine, finished)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			writerErr := writer.WithContext(ctx).Exec("UPDATE marketplace_order_locks SET version = version + 1 WHERE name = ?", marketplaceOrderLockName).Error
			_ = writer.Rollback().Error
			rootErr := <-finished
			combined := errors.Join(writerErr, rootErr)
			require.Error(t, combined)
			require.Contains(t, strings.ToLower(combined.Error()), "deadlock", "exercise an actual cross-table deadlock, not only a timeout")
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "deadlock-winner").Count(&count).Error)
			if rootErr != nil {
				assert.Zero(t, count, "deadlock victim must fully roll back")
			} else {
				assert.Equal(t, int64(1), count)
			}
			require.NoError(t, db.Create(&Channel{Key: "after-deadlock"}).Error)
		})
	}
}

func TestCatalogSyncReferenceRealCommitFailure(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			var reader *gorm.DB
			if engine == "sqlite" {
				// A real SHARED read lock prevents the RESERVED writer's COMMIT
				// upgrade in rollback-journal mode, even after the callback succeeds.
				reader = db.Begin()
				require.NoError(t, reader.Error)
				defer reader.Rollback()
				var count int64
				require.NoError(t, reader.Model(&Option{}).Count(&count).Error)
			}
			if engine == "postgres" {
				require.NoError(t, db.Exec("CREATE TABLE fence_parent (id bigint PRIMARY KEY)").Error)
				require.NoError(t, db.Exec("CREATE TABLE fence_child (id bigint PRIMARY KEY, parent_id bigint REFERENCES fence_parent(id) DEFERRABLE INITIALLY DEFERRED)").Error)
			}
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
					if err := tx.Create(&Option{Key: "failed-commit"}).Error; err != nil {
						return err
					}
					if engine == "postgres" {
						return tx.Exec("INSERT INTO fence_child (id, parent_id) VALUES (1, 999)").Error
					}
					if engine == "mysql" {
						var id int64
						require.NoError(t, tx.Raw("SELECT CONNECTION_ID()").Scan(&id).Error)
						return db.Exec(fmt.Sprintf("KILL CONNECTION %d", id)).Error
					}
					return nil
				})
			})
			require.ErrorIs(t, err, ErrCatalogCommitUncertain)
			if reader != nil {
				require.NoError(t, reader.Rollback().Error)
			}
			var count int64
			require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" = ?", "failed-commit").Count(&count).Error)
			assert.Zero(t, count)
			registry.SetEnabled(false) // no registry lease may survive COMMIT error
			require.NoError(t, db.Create(&Option{Key: "after-commit-error"}).Error)
		})
	}
}

func TestCatalogSyncReferenceRegistryBusy(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			registry := jsplugin.NewRegistry()
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			go func() {
				done <- registry.SetGenerationPreparer(func(candidate, current *jsplugin.RoutingGeneration) (jsplugin.PreparedRoutingGeneration, error) {
					close(entered)
					<-release
					return jsplugin.PreparedRoutingGeneration{}, errors.New("test publication stopped")
				})
			}()
			<-entered
			err := WithCatalogWriteBarrier(func() error {
				return catalogReferenceTransaction(context.Background(), db, registry, func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
					t.Error("busy registry admitted business callback")
					return nil
				})
			})
			close(release)
			require.Error(t, <-done)
			require.ErrorIs(t, err, jsplugin.ErrGenerationBusy)
			require.NoError(t, db.Create(&Option{Key: "after-busy"}).Error)
		})
	}
}

func TestCatalogSyncReferenceMySQLVersionGate(t *testing.T) {
	for _, version := range []string{"5.7.8", "5.7.43", "8.0.45", "8.4.10", "8.2.0", "9.0.0", "8.4.11-MariaDB", "8.4.bad"} {
		require.Error(t, catalogMySQLFenceVersion(version, "MySQL Community Server - GPL"), version)
	}
	for _, variant := range []string{"MariaDB Server", "TiDB", "Percona Server", "unknown"} {
		require.Error(t, catalogMySQLFenceVersion("8.4.11", variant), variant)
	}
}

func TestCatalogSyncReferencePostgresSearchPath(t *testing.T) {
	db := catalogFenceTestDB(t, "postgres")
	require.NoError(t, db.Exec(`CREATE SCHEMA "fence schema"`).Error)
	for _, model := range []any{&marketplaceOrderLock{}, &Model{}, &Vendor{}, &Option{}, &Channel{}, &Ability{}, &Task{}, &TaskPlugin{}, &Midjourney{}, &SystemTask{}, &CatalogSyncState{}, &CatalogSyncPlan{}, &CatalogSyncBaseline{}, &CatalogSyncOperation{}} {
		stmt := &gorm.Statement{DB: db}
		require.NoError(t, stmt.Parse(model))
		require.NoError(t, db.Exec("ALTER TABLE "+stmt.Quote(stmt.Schema.Table)+` SET SCHEMA "fence schema"`).Error)
	}
	require.NoError(t, db.Exec(`SET search_path TO "fence schema"`).Error)
	pool, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
			catalogAssertWriterBlocked(t, pool, "postgres", `INSERT INTO "fence schema".channels (key, molii_grok_management_access_token) VALUES ('fixture', '')`)
			return tx.Create(&Option{Key: "schema-bound"}).Error
		})
	}))
	var count int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM "fence schema".options WHERE key = 'schema-bound'`).Scan(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCatalogSyncReferenceSQLiteBusyBound(t *testing.T) {
	db := catalogFenceTestDB(t, "sqlite")
	pool, err := db.DB()
	require.NoError(t, err)
	writer := db.Begin()
	require.NoError(t, writer.Error)
	defer writer.Rollback()
	require.NoError(t, writer.Exec("UPDATE marketplace_order_locks SET version = version + 1").Error)
	conn, err := pool.Conn(context.Background())
	require.NoError(t, err)
	_, err = conn.ExecContext(context.Background(), "PRAGMA busy_timeout = 60000")
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- catalogReferenceTransaction(ctx, db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
			return errors.New("unexpected callback")
		})
	}()
	select {
	case err := <-finished:
		require.ErrorContains(t, err, "locked")
	case <-time.After(2 * time.Second):
		t.Fatal("root inherited the unbounded busy wait")
	}
	require.NoError(t, writer.Rollback().Error)
	require.NoError(t, db.Create(&Option{Key: "after-sqlite-busy"}).Error)
}

func TestCatalogSyncReferencePostgresSearchPathShadow(t *testing.T) {
	db := catalogFenceTestDB(t, "postgres")
	require.NoError(t, db.Create(&Option{Key: "proof", Value: "fenced"}).Error)
	require.NoError(t, db.Exec("CREATE SCHEMA fence_shadow").Error)
	require.NoError(t, db.Exec("SET search_path TO fence_shadow, public").Error)
	created := false
	require.NoError(t, db.Callback().Raw().After("gorm:raw").Register("shadow-search-path", func(tx *gorm.DB) {
		query := tx.Statement.SQL.String()
		if !created && strings.HasPrefix(query, "LOCK TABLE") && strings.Contains(query, "options") {
			created = true
			require.NoError(t, db.Exec("CREATE TABLE fence_shadow.options (key text PRIMARY KEY, value text)").Error)
			require.NoError(t, db.Exec("INSERT INTO fence_shadow.options (key, value) VALUES ('proof', 'unfenced')").Error)
		}
	}))
	defer db.Callback().Raw().Remove("shadow-search-path")
	err := WithCatalogWriteBarrier(func() error {
		return catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(tx *gorm.DB, _ *CatalogSyncState, _ *jsplugin.GenerationPin) error {
			var option Option
			if err := tx.First(&option, "key = ?", "proof").Error; err != nil {
				return err
			}
			if option.Value != "fenced" {
				return errors.New("plain read followed an unfenced search_path shadow")
			}
			return nil
		})
	})
	require.NoError(t, err)
	assert.True(t, created)
}

func TestCatalogSyncReferencePostgresUnsupportedNamespaces(t *testing.T) {
	for _, layout := range []string{"temporary", "mixed"} {
		t.Run(layout, func(t *testing.T) {
			db := catalogFenceTestDB(t, "postgres")
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			if layout == "temporary" {
				require.NoError(t, db.Exec("CREATE TEMPORARY TABLE options (key text PRIMARY KEY, value text)").Error)
			} else {
				require.NoError(t, db.Exec("CREATE SCHEMA mixed_fence").Error)
				require.NoError(t, db.Exec("ALTER TABLE options SET SCHEMA mixed_fence").Error)
				require.NoError(t, db.Exec("SET search_path TO mixed_fence, public").Error)
			}
			err = catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("unsupported namespace admitted")
				return nil
			})
			require.Error(t, err)
			if layout == "mixed" {
				require.ErrorContains(t, err, "mixed unqualified")
			}
		})
	}
}

func TestCatalogSyncReferenceUnsupportedTableLayouts(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			switch engine {
			case "postgres":
				require.NoError(t, db.Exec("ALTER TABLE channels ENABLE ROW LEVEL SECURITY").Error)
				require.NoError(t, db.Exec("CREATE POLICY hidden_catalog_references ON channels USING (false)").Error)
			case "mysql":
				require.NoError(t, db.Exec("ALTER TABLE channels PARTITION BY HASH(id) PARTITIONS 2").Error)
			case "sqlite":
				require.NoError(t, db.Migrator().DropTable(&Channel{}))
				require.NoError(t, db.Exec("CREATE VIRTUAL TABLE channels USING fts5(id)").Error)
			}
			callback := func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("unsupported physical layout reached callback")
				return nil
			}
			require.Error(t, catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), callback))
			if engine == "postgres" {
				require.NoError(t, db.Exec("ALTER TABLE channels DISABLE ROW LEVEL SECURITY").Error)
				require.NoError(t, db.Exec("ALTER TABLE channels FORCE ROW LEVEL SECURITY").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), callback))
				require.NoError(t, db.Migrator().DropTable(&Channel{}))
				require.NoError(t, db.Exec("CREATE TABLE channels (id bigint) PARTITION BY HASH(id)").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), callback))
				require.NoError(t, db.Migrator().DropTable(&Channel{}))
				require.NoError(t, db.Exec("CREATE TABLE channel_family (id bigint) PARTITION BY HASH(id)").Error)
				require.NoError(t, db.Exec("CREATE TABLE channels PARTITION OF channel_family FOR VALUES WITH (MODULUS 1, REMAINDER 0)").Error)
				require.Error(t, catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), callback))
			}
		})
	}
}

func TestCatalogSyncReferenceSQLiteAttachedAndTemporary(t *testing.T) {
	for _, layout := range []string{"attached", "temporary"} {
		t.Run(layout, func(t *testing.T) {
			db := catalogFenceTestDB(t, "sqlite")
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			if layout == "attached" {
				require.NoError(t, db.Exec("ATTACH DATABASE ? AS extra", filepath.Join(t.TempDir(), "extra.db")).Error)
				require.NoError(t, db.Migrator().DropTable(&Channel{}))
				require.NoError(t, db.Exec("CREATE TABLE extra.channels (id integer PRIMARY KEY)").Error)
			} else {
				require.NoError(t, db.Exec("CREATE TEMPORARY TABLE channels (id integer PRIMARY KEY)").Error)
			}
			err = catalogReferenceTransaction(context.Background(), db, jsplugin.NewRegistry(), func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error {
				t.Error("references resolved outside the fenced main database")
				return nil
			})
			require.Error(t, err)
		})
	}
}
