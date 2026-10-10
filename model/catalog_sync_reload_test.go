package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"maps"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func catalogReloadTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	require.NotEmpty(t, os.Getenv("TEST_POSTGRES_DSN"), "this fixture requires task PostgreSQL")
	db := catalogFenceTestDB(t, "postgres")
	initializeOrdinaryCatalogTest(t, db)
	policy := CurrentRequestPolicy()
	metadata := ratio_setting.GroupMetadata2JSONString()
	t.Cleanup(func() {
		for key, value := range policy.Options {
			require.NoError(t, updateOptionMap(key, value))
		}
		requestPolicySnapshot.Store(policy)
		require.NoError(t, ratio_setting.UpdateGroupMetadataByJSONString(metadata))
	})
	require.NoError(t, db.AutoMigrate(&PasskeyCredential{}))
	previous, server := *system_setting.GetPasskeySettings(), system_setting.ServerAddress
	t.Cleanup(func() { *system_setting.GetPasskeySettings(), system_setting.ServerAddress = previous, server })
	*system_setting.GetPasskeySettings() = system_setting.PasskeySettings{}
	system_setting.ServerAddress = ""
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	return db
}

func TestCatalogMixedPasskeySave(t *testing.T) {
	db := catalogReloadTestDB(t)
	values := map[string]string{"ServerAddress": "https://login.example.com", "ModelPrice": `{"mixed":0}`, "SystemName": "mixed save", "RetryTimes": "3"}
	original := maps.Clone(values)
	preview, err := UpdatePasskeyDomainOptions(values, true, "")
	require.NoError(t, err)
	assert.Equal(t, "login.example.com", preview.EffectiveRPID)
	var count int64
	require.NoError(t, db.Model(&Option{}).Count(&count).Error)
	assert.Zero(t, count, "preview must roll back even implicit default rows")
	_, err = UpdatePasskeyDomainOptions(values, false, preview.RemovalConfirmation)
	require.NoError(t, err)
	assert.Equal(t, original, values)
	var state CatalogSyncState
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Equal(t, int64(1), state.Revision)
	assert.Equal(t, state.Revision, state.RuntimeRevision, "mixed saves must acknowledge the complete published catalog")
	require.NoError(t, WithCatalogPricingRead(context.Background(), "mixed", func() error {
		assert.Equal(t, map[string]float64{"mixed": 0}, ratio_setting.GetModelPriceMap())
		return nil
	}))
	assert.Equal(t, "mixed save", common.SystemName)
	assert.Equal(t, "https://login.example.com", system_setting.ServerAddress)
	assert.Equal(t, 3, CurrentRequestPolicy().RetryTimes)
	_, err = UpdatePasskeyDomainOptions(values, false, "")
	require.NoError(t, err)
	var noOp CatalogSyncState
	require.NoError(t, db.First(&noOp, CatalogSyncStateID).Error)
	assert.Equal(t, state, noOp)
	values["ServerAddress"] = "https://new.example.com"
	_, err = UpdatePasskeyDomainOptions(values, false, "")
	require.ErrorIs(t, err, system_setting.ErrPasskeyRPIDInvalid, "a previously stored explicit origin cannot be silently expanded")
	values["passkey.origins"] = "https://login.example.com,https://new.example.com"
	change, err := UpdatePasskeyDomainOptions(values, false, "")
	require.NoError(t, err)
	assert.Equal(t, "login.example.com", change.LegacyRPIDs)
	assert.Equal(t, "https://login.example.com,https://new.example.com", change.Origins)
	require.NoError(t, db.First(&noOp, CatalogSyncStateID).Error)
	assert.Equal(t, state, noOp, "domain-only change with same catalog must retain plan freshness")
}

func TestCatalogReloadConcurrentOrdinary(t *testing.T) {
	for _, writer := range []string{"pricing-currency", "metadata", "generic"} {
		t.Run(writer, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			require.NoError(t, db.Create(&Model{ModelName: "concurrent", NameRule: NameRuleExact, BillingCurrency: "USD"}).Error)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"billing_setting.billing_mode": `{"concurrent":"tiered_expr"}`, "billing_setting.billing_expr": `{"concurrent":"p * 2"}`}))
			snapshot, err := GetModelPricingSnapshot([]string{"concurrent"})
			require.NoError(t, err)
			finished := make(chan error, 1)
			var launched atomic.Bool
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("reload-overlap", func(tx *gorm.DB) {
				if tx.Statement.Table != "options" || tx.Statement.ConnPool != db.Statement.ConnPool || !launched.CompareAndSwap(false, true) {
					return
				}
				var state CatalogSyncState
				tx.AddError(db.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, "committed_pending_publish", state.PublicationState)
				assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "concurrent", func() error { t.Error("reload selection escaped"); return nil }), ErrCatalogWriterBusy)
				pinReleased := make(chan struct{})
				go func() { jsplugin.DefaultRegistry.SetEnabled(true); close(pinReleased) }()
				select {
				case <-pinReleased:
				case <-time.After(3 * time.Second):
					t.Error("reload retained registry pin")
				}
				started := make(chan struct{})
				go func() {
					close(started)
					switch writer {
					case "pricing-currency":
						finished <- UpdateModelPricing([]ModelPricingChange{{ModelName: "concurrent", ExpectedVersion: snapshot.Entries[0].Version, BillingCurrency: "CNY", Pricing: PricingValues{"billing_setting.billing_mode": "tiered_expr", "billing_setting.billing_expr": "p * 9"}}})
					case "metadata":
						vendor := Vendor{Name: "after-reload"}
						finished <- vendor.Insert()
					case "generic":
						finished <- UpdateOptionsBulk(map[string]string{"ModelPrice": `{"new":0}`})
					}
				}()
				<-started
			}))
			reloadErr := loadOptionsFromDatabase()
			require.True(t, launched.Load())
			select {
			case err = <-finished:
			case <-time.After(10 * time.Second):
				t.Fatal("ordinary writer did not finish after reload")
			}
			require.NoError(t, db.Callback().Query().Remove("reload-overlap"))
			require.NoError(t, reloadErr)
			require.NoError(t, err)
			require.NoError(t, WithCatalogPricingRead(context.Background(), "concurrent", func() error {
				switch writer {
				case "pricing-currency":
					money, _, err := ResolveBillingMoneyContext(db, "concurrent")
					require.NoError(t, err)
					assert.Equal(t, "CNY", string(money.SourceCurrency))
					expression, _ := billing_setting.GetBillingExpr("concurrent")
					assert.Equal(t, "p * 9", expression)
				case "metadata":
					var count int64
					require.NoError(t, db.Model(&Vendor{}).Where("name = ?", "after-reload").Count(&count).Error)
					assert.Equal(t, int64(1), count)
				case "generic":
					assert.Equal(t, map[string]float64{"new": 0}, ratio_setting.GetModelPriceMap())
				}
				return nil
			}))
		})
	}
}

func TestCatalogReloadDeletedPrice(t *testing.T) {
	db := catalogReloadTestDB(t)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"removed":7}`}))
	require.NoError(t, db.Delete(&Option{}, commonKeyCol+" = ?", "ModelPrice").Error)
	require.NoError(t, loadOptionsFromDatabase())
	assert.NotContains(t, ratio_setting.GetModelPriceMap(), "removed", "authoritative reload must not resurrect deleted leaves from typed caches")
}

func TestCatalogReloadSQLCancellation(t *testing.T) {
	for _, site := range []string{"query", "normalization-write"} {
		t.Run(site, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			const key = "group_ratio_setting.group_metadata"
			const legacy = `[{"name":"vip","icon":"DeepSeek.Color","recommendation":4}]`
			require.NoError(t, db.Create(&Option{Key: key, Value: legacy}).Error)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type blockedSQL struct {
				blocker *gorm.DB
				ctx     context.Context
			}
			started := make(chan blockedSQL, 1)
			sqlResult := make(chan error, 1)
			var armed atomic.Bool
			armed.Store(true)
			before := func(tx *gorm.DB) {
				matches := tx.Statement.Table == "options" && tx.Statement.ConnPool == db.Statement.ConnPool
				if site == "normalization-write" {
					row, ok := tx.Statement.Dest.(*Option)
					matches = ok && row.Key == key
				}
				if !matches || !armed.CompareAndSwap(true, false) {
					return
				}
				// The final reference transaction has ended. This independent
				// connection blocks the actual new SQL site, not a mock query.
				blocker := db.Begin()
				tx.AddError(blocker.Exec("LOCK TABLE options IN ACCESS EXCLUSIVE MODE").Error)
				tx.Statement.Settings.Store("reload-cancel-observed", true)
				started <- blockedSQL{blocker, tx.Statement.Context}
			}
			after := func(tx *gorm.DB) {
				if _, observed := tx.Statement.Settings.Load("reload-cancel-observed"); observed {
					sqlResult <- tx.Error
				}
			}
			if site == "query" {
				require.NoError(t, db.Callback().Query().Before("gorm:query").Register("reload-cancel-before", before))
				require.NoError(t, db.Callback().Query().After("gorm:query").Register("reload-cancel-after", after))
			} else {
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("reload-cancel-before", before))
				require.NoError(t, db.Callback().Update().After("gorm:update").Before("gorm:commit_or_rollback_transaction").Register("reload-cancel-after", after))
			}
			finished := make(chan error, 1)
			go func() { finished <- recoverCatalogSyncRuntime(ctx, loadOptionsFromDatabaseGuarded) }()
			var blocked blockedSQL
			select {
			case blocked = <-started:
			case err := <-finished:
				t.Fatalf("reload exited before SQL site: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("reload did not reach SQL site")
			}
			defer blocked.blocker.Rollback()
			catalogWaitForDBLock(t, db, "postgres", finished)
			cancel()
			select {
			case err := <-sqlResult:
				assert.ErrorIs(t, err, context.Canceled, "actual blocked query/write must return cancellation")
			case <-time.After(2 * time.Second):
				t.Error("publisher cancellation did not release the blocked option SQL")
				// Bound the RED failure and allow its lifecycle to release locks.
				require.NoError(t, blocked.blocker.Rollback().Error)
				assert.ErrorIs(t, <-sqlResult, context.Canceled)
			}
			assert.ErrorIs(t, blocked.ctx.Err(), context.Canceled, "the new SQL must inherit the publisher context")
			require.ErrorIs(t, <-finished, context.Canceled)
			_ = blocked.blocker.Rollback().Error
			if site == "query" {
				require.NoError(t, db.Callback().Query().Remove("reload-cancel-before"))
				require.NoError(t, db.Callback().Query().Remove("reload-cancel-after"))
			} else {
				require.NoError(t, db.Callback().Update().Remove("reload-cancel-before"))
				require.NoError(t, db.Callback().Update().Remove("reload-cancel-after"))
			}
			for _, lock := range []struct {
				name    string
				acquire func() bool
				release func()
			}{
				{"common", catalogBarrier.TryLock, catalogBarrier.Unlock},
				{"policy", requestPolicyOptionMutex.TryLock, requestPolicyOptionMutex.Unlock},
				{"passkey", passkeyOptionMutex.TryLock, passkeyOptionMutex.Unlock},
			} {
				if assert.True(t, lock.acquire(), "%s lock must be released", lock.name) {
					lock.release()
				}
			}
			var pending CatalogSyncState
			require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
			assert.Equal(t, "committed_pending_publish", pending.PublicationState)
			var row Option
			require.NoError(t, db.First(&row, commonKeyCol+" = ?", key).Error)
			assert.Equal(t, legacy, row.Value, "canceled normalization must not persist")
			require.NoError(t, loadOptionsFromDatabase())
			var ready CatalogSyncState
			require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
			assert.Equal(t, pending.Revision, ready.Revision)
			assert.Equal(t, "ready", ready.PublicationState)
			require.NoError(t, db.First(&row, commonKeyCol+" = ?", key).Error)
			assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, row.Value)
		})
	}
}

func TestCatalogReloadErrorsAndFreshOptions(t *testing.T) {
	db := catalogReloadTestDB(t)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"kept":4}`}))
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"bad":`}).Error)
	require.Error(t, loadOptionsFromDatabase())
	assert.Equal(t, map[string]float64{"kept": 4}, ratio_setting.GetModelPriceMap(), "decode failure must not publish a partial candidate")
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "kept", func() error { t.Error("failed reload escaped"); return nil }), ErrCatalogPublicationPending)
	require.ErrorIs(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{}`}), ErrCatalogPublicationPending)
	for key, value := range map[string]string{
		"ModelPrice": `{}`, "USDExchangeRate": "7.125", "RetryTimes": "4",
		"ServerAddress": "https://reload.example.com", "passkey.rp_id": "reload.example.com",
		"group_ratio_setting.group_metadata": `[{"name":"vip","icon":"DeepSeek.Color","recommendation":4}]`,
	} {
		require.NoError(t, db.Save(&Option{Key: key, Value: value}).Error)
	}
	require.NoError(t, loadOptionsFromDatabase())
	assert.Empty(t, ratio_setting.GetModelPriceMap())
	assert.Equal(t, 7.125, operation_setting.USDExchangeRate)
	assert.Equal(t, 4, CurrentRequestPolicy().RetryTimes)
	assert.Equal(t, "reload.example.com", system_setting.GetPasskeySettings().RPID)
	assert.Equal(t, "https://reload.example.com", system_setting.ServerAddress)
	var normalized Option
	require.NoError(t, db.First(&normalized, commonKeyCol+" = ?", "group_ratio_setting.group_metadata").Error)
	assert.JSONEq(t, `[{"name":"vip","icon":"DeepSeek.Color"}]`, normalized.Value)
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.Equal(t, before.Revision, after.Revision, "reload does not invent a business mutation")
	assert.Equal(t, after.Revision, after.RuntimeRevision)
	require.NoError(t, loadOptionsFromDatabase())
	var noOp CatalogSyncState
	require.NoError(t, db.First(&noOp, CatalogSyncStateID).Error)
	assert.Equal(t, after, noOp)
	// A callback query error is visible and must not acknowledge success.
	failure := errors.New("reload option query unavailable")
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("reload-query-error", func(tx *gorm.DB) {
		if tx.Statement.Table == "options" && tx.Statement.ConnPool == db.Statement.ConnPool {
			tx.AddError(failure)
		}
	}))
	err := loadOptionsFromDatabase()
	require.NoError(t, db.Callback().Query().Remove("reload-query-error"))
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "kept", func() error { t.Error("query failure escaped"); return nil }), ErrCatalogPublicationPending)
	require.NoError(t, loadOptionsFromDatabase())
	require.NoError(t, db.Save(&Option{Key: "group_ratio_setting.group_metadata", Value: `[{"name":""}]`}).Error)
	require.Error(t, loadOptionsFromDatabase(), "normalization failure must be returned, not skipped")
	require.NoError(t, db.Save(&Option{Key: "group_ratio_setting.group_metadata", Value: `[]`}).Error)
	require.NoError(t, loadOptionsFromDatabase())
}

func TestCatalogReloadRejectsObsoleteOptionsAndFX(t *testing.T) {
	for _, drift := range []string{"ModelPrice", "USDExchangeRate"} {
		t.Run(drift, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"fresh":2}`, "USDExchangeRate": "7.25"}))
			changed := false
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("reload-obsolete", func(tx *gorm.DB) {
				if changed || tx.Statement.Table != "options" || tx.Statement.ConnPool != db.Statement.ConnPool {
					return
				}
				changed = true
				value := `{"fresh":9}`
				if drift == "USDExchangeRate" {
					value = "8.5"
				}
				tx.AddError(db.Save(&Option{Key: drift, Value: value}).Error)
			}))
			err := loadOptionsFromDatabase()
			require.NoError(t, db.Callback().Query().Remove("reload-obsolete"))
			require.True(t, changed)
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			assert.Equal(t, float64(2), ratio_setting.GetModelPriceMap()["fresh"])
			assert.Equal(t, 7.25, operation_setting.USDExchangeRate)
			require.NoError(t, loadOptionsFromDatabase())
			if drift == "ModelPrice" {
				assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["fresh"])
			} else {
				assert.Equal(t, 8.5, operation_setting.USDExchangeRate)
			}
		})
	}
}

func TestCatalogMixedPasskeyNativeAcknowledgements(t *testing.T) {
	for _, lost := range []string{"committed_pending_publish", "ready"} {
		t.Run(lost, func(t *testing.T) {
			db := catalogReloadTestDB(t)
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
				writes := 0
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("mixed-native-loss", func(tx *gorm.DB) {
					if row, ok := tx.Statement.Dest.(*Option); ok && row.Key == "SystemName" {
						writes++
					}
					if tx.Statement.Table == "catalog_sync_states" {
						if values, ok := tx.Statement.Dest.(map[string]any); ok && values["publication_state"] == lost {
							connection.armed.Store(true)
						}
					}
				}))
				defer db.Callback().Update().Remove("mixed-native-loss")
				_, err := UpdatePasskeyDomainOptions(map[string]string{"ServerAddress": "https://native.example.com", "SystemName": "native", "ModelPrice": `{"native":8}`}, false, "")
				if lost == "committed_pending_publish" {
					require.ErrorIs(t, err, ErrCatalogCommitUncertain)
				} else {
					require.NoError(t, err)
				}
				assert.Equal(t, 1, writes, "native COMMIT uncertainty must never replay the supplied writes")
				assert.Equal(t, int64(1), connection.dropped.Load())
				var state CatalogSyncState
				require.NoError(t, root.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, int64(1), state.Revision)
				if lost == "committed_pending_publish" {
					assert.Equal(t, lost, state.PublicationState)
					assert.Empty(t, system_setting.ServerAddress)
					require.NoError(t, loadOptionsFromDatabase())
				}
				require.NoError(t, root.First(&state, CatalogSyncStateID).Error)
				assert.Equal(t, state.Revision, state.RuntimeRevision)
				assert.Equal(t, "https://native.example.com", system_setting.ServerAddress)
				assert.Equal(t, "native", common.SystemName)
				assert.Equal(t, float64(8), ratio_setting.GetModelPriceMap()["native"])
				return nil
			}))
		})
	}
}

func TestCatalogMixedPasskeyConfirmationAndRollback(t *testing.T) {
	db := catalogReloadTestDB(t)
	_, err := UpdatePasskeyDomainOptions(map[string]string{
		"passkey.rp_id": "login.example.com", "passkey.origins": "https://login.example.com,https://old.example.com",
		"passkey.legacy_rp_ids": "old.example.com",
	}, false, "")
	require.NoError(t, err)
	rp := "old.example.com"
	require.NoError(t, db.Create(&PasskeyCredential{UserID: 1, CredentialID: "known", PublicKey: "test", RPID: &rp}).Error)
	values := map[string]string{"passkey.legacy_rp_ids": "", "ModelPrice": `{"confirmed":2}`, "SystemName": "confirmed", "zz_mixed_failure": "last write"}
	preview, err := UpdatePasskeyDomainOptions(values, true, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"old.example.com"}, preview.RemovedRPIDs)
	assert.Equal(t, int64(1), preview.AffectedCredentials)
	assert.True(t, preview.ConfirmationRequired)
	_, err = UpdatePasskeyDomainOptions(values, false, "")
	require.ErrorIs(t, err, ErrPasskeyDomainRemovalConfirmation)
	// The count is bound to the signed preview, so a newly stored credential
	// invalidates it even though the domain option values have not changed.
	require.NoError(t, db.Create(&PasskeyCredential{UserID: 2, CredentialID: "unknown", PublicKey: "test"}).Error)
	fresh, err := UpdatePasskeyDomainOptions(values, false, preview.RemovalConfirmation)
	require.ErrorIs(t, err, ErrPasskeyDomainRemovalConfirmation)
	assert.Equal(t, int64(1), fresh.UnknownCredentials)
	assert.NotEqual(t, preview.RemovalConfirmation, fresh.RemovalConfirmation)
	var count int64
	require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" IN ?", []string{"ModelPrice", "SystemName"}).Count(&count).Error)
	assert.Zero(t, count)
	assert.Equal(t, "old.example.com", system_setting.GetPasskeySettings().LegacyRPIDs)
	// A late unrelated SQL error must roll back both catalog and domain rows.
	failure := errors.New("forced unrelated write failure")
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("mixed-rollback", func(tx *gorm.DB) {
		if row, ok := tx.Statement.Dest.(*Option); ok && row.Key == "zz_mixed_failure" {
			tx.AddError(failure)
		}
	}))
	_, err = UpdatePasskeyDomainOptions(values, false, fresh.RemovalConfirmation)
	require.NoError(t, db.Callback().Update().Remove("mixed-rollback"))
	require.ErrorIs(t, err, failure)
	require.NoError(t, db.Model(&Option{}).Where(commonKeyCol+" IN ?", []string{"ModelPrice", "SystemName"}).Count(&count).Error)
	assert.Zero(t, count)
	assert.Equal(t, "old.example.com", system_setting.GetPasskeySettings().LegacyRPIDs)
	var persisted Option
	require.NoError(t, db.First(&persisted, commonKeyCol+" = ?", "passkey.legacy_rp_ids").Error)
	assert.Equal(t, "old.example.com", persisted.Value, "late unrelated error must roll back already-written domain rows")
	_, err = UpdatePasskeyDomainOptions(values, false, fresh.RemovalConfirmation)
	require.NoError(t, err)
	assert.Empty(t, system_setting.GetPasskeySettings().LegacyRPIDs)
	assert.Equal(t, float64(2), ratio_setting.GetModelPriceMap()["confirmed"])
	assert.Equal(t, "confirmed", common.SystemName)
}

func TestCatalogMixedPasskeyInvalidAndPending(t *testing.T) {
	db := catalogReloadTestDB(t)
	values := map[string]string{"ServerAddress": "https://login.example.com", "ModelPrice": `{"invalid":-2}`}
	_, err := UpdatePasskeyDomainOptions(values, false, "")
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&Option{}).Count(&count).Error)
	assert.Zero(t, count, "catalog validation must precede every supplied SQL write")
	values["ModelPrice"] = `{"committed":3}`
	updatePricingLock.Lock()
	_, err = UpdatePasskeyDomainOptions(values, false, "")
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	var state CatalogSyncState
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Equal(t, "committed_pending_publish", state.PublicationState)
	assert.Equal(t, int64(1), state.Revision)
	assert.Equal(t, "https://login.example.com", system_setting.ServerAddress)
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "committed", func() error { t.Error("pending selection"); return nil }), ErrCatalogPublicationPending)
	_, err = UpdatePasskeyDomainOptions(values, false, "")
	require.ErrorIs(t, err, ErrCatalogPublicationPending)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Equal(t, int64(1), state.Revision)
	assert.Equal(t, state.Revision, state.RuntimeRevision)
}
