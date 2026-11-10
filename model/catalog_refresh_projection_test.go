package model

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCatalogRefreshChangedPolicyRequiresWriter(t *testing.T) {
	previousSelfUse := operation_setting.SelfUseModeEnabled
	t.Cleanup(func() { operation_setting.SelfUseModeEnabled = previousSelfUse })
	db := catalogReloadTestDB(t)
	require.NoError(t, UpdateOption("SelfUseModeEnabled", "false"))
	require.NoError(t, db.Save(&Option{Key: "SelfUseModeEnabled", Value: "true"}).Error)
	loaderEntered := make(chan struct{})
	var once sync.Once
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("changed-policy-loader", func(tx *gorm.DB) {
		if tx.Statement.Table == "options" && tx.Statement.ConnPool == db.Statement.ConnPool {
			once.Do(func() { close(loaderEntered) })
		}
	}))
	defer db.Callback().Query().Remove("changed-policy-loader")
	require.NoError(t, WithCatalogPricingReads(context.Background(), []string{"policy-missing-a", "policy-missing-b"}, func() error {
		_, firstAllowed, _ := ratio_setting.GetModelRatio("policy-missing-a")
		require.False(t, firstAllowed)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- recoverCatalogSyncRuntime(ctx, loadOptionsFromDatabaseGuarded) }()
		select {
		case <-loaderEntered:
		case err := <-done:
			t.Fatalf("reload ended before capturing the changed policy: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("real option loader did not capture the changed persisted policy")
		}
		// The five-second budget is only a watchdog. Rendezvous at the actual
		// writer wait so SQL/fixture speed cannot consume the tested interval.
		require.Eventually(t, func() bool { return catalogWriterIntent.Load() > 0 }, time.Second, time.Millisecond)
		cancel()
		// Keep this genuine pricing capture open through cancellation of W.
		// No setter may change fallback admissibility inside this reader.
		assert.ErrorIs(t, <-done, context.Canceled)
		_, secondAllowed, _ := ratio_setting.GetModelRatio("policy-missing-b")
		assert.Equal(t, firstAllowed, secondAllowed, "one pricing capture must retain one fallback policy")
		return nil
	}))
	require.NoError(t, loadOptionsFromDatabase())
	require.NoError(t, WithCatalogPricingRead(context.Background(), "policy-missing-after", func() error {
		_, allowed, _ := ratio_setting.GetModelRatio("policy-missing-after")
		assert.True(t, allowed, "the changed policy must publish after the old reader releases")
		return nil
	}))
}

func TestCatalogRefreshPublicRecoveryPublishesCompleteMoneyEpoch(t *testing.T) {
	for _, scenario := range []string{"explicit-raw", "effective-default"} {
		t.Run(scenario, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			require.NoError(t, db.Create(&Model{ModelName: "primitive-money", BillingCurrency: "CNY", NameRule: NameRuleExact}).Error)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"primitive-money":2}`, "USDExchangeRate": "7"}))
			require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"primitive-money":9}`}).Error)
			expectedFX, expectedRaw := float64(14), "14.000"
			if scenario == "explicit-raw" {
				require.NoError(t, db.Save(&Option{Key: "USDExchangeRate", Value: expectedRaw}).Error)
			} else {
				expectedFX, expectedRaw = 7.3, "7.3"
				require.NoError(t, db.Where(map[string]any{"key": "USDExchangeRate"}).Delete(&Option{}).Error)
			}
			// This public primitive deliberately has no option-loader callback.
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			require.NoError(t, WithCatalogPricingRead(context.Background(), "primitive-money", func() error {
				money, found, err := ResolvePublishedCatalogMoneyContext("primitive-money")
				require.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["primitive-money"])
				assert.Equal(t, expectedFX, money.CNYPerUSD)
				assert.Equal(t, expectedFX, operation_setting.USDExchangeRate, "legacy FX must belong to the same installed epoch")
				common.OptionMapRWMutex.RLock()
				raw := common.OptionMap["USDExchangeRate"]
				common.OptionMapRWMutex.RUnlock()
				assert.Equal(t, expectedRaw, raw, "keep exact configured raw FX, or the validated effective default when absent")
				return nil
			}))
		})
	}
}

// The existing phase logger supplies a post-helper-return rendezvous, not a
// production test hook. All assertions below inspect real SQL and publication.
type catalogReconcileCompletionWriter struct {
	writer  io.Writer
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *catalogReconcileCompletionWriter) Write(data []byte) (int, error) {
	line := string(data)
	if strings.Contains(line, "source=channel_reconcile ") && strings.Contains(line, "phase=recovery_complete ") && strings.Contains(line, "result=ok") {
		w.once.Do(func() { close(w.entered); <-w.release })
	}
	return w.writer.Write(data)
}

func TestCatalogRefreshReconciliationCacheBindsCompletedOperation(t *testing.T) {
	db := catalogReloadTestDB(t)
	priorCache := catalogReconciledEpoch.Swap(nil)
	t.Cleanup(func() { catalogReconciledEpoch.Store(priorCache) })
	channelA := Channel{Key: "cache-a", Models: "cache-window-a", Group: "default", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channelA).Error)
	require.NoError(t, db.Create(&Ability{ChannelId: channelA.Id, Model: "cache-window-a", Group: "default", Enabled: true}).Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	observer := &catalogReconcileCompletionWriter{writer: previousWriter, entered: make(chan struct{}), release: make(chan struct{})}
	gin.DefaultWriter = observer
	common.LogWriterMu.Unlock()
	var resume sync.Once
	defer func() {
		resume.Do(func() { close(observer.release) })
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		common.LogWriterMu.Unlock()
	}()
	done := make(chan error, 1)
	go func() { _, err := reconcileEnabledModelMetadata(context.Background(), true); done <- err }()
	joined := false
	defer func() {
		resume.Do(func() { close(observer.release) })
		if !joined {
			<-done
		}
	}()
	select {
	case <-observer.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("operation A did not reach its real post-writer completion boundary")
	}
	// B wins after A released its writer but before A's caller records a cache.
	// B publishes a valid route without performing metadata reconciliation.
	channelB := Channel{Key: "cache-b", Models: "cache-window-b", Group: "default", Status: common.ChannelStatusEnabled}
	require.NoError(t, withMarketplaceOrderTransaction(db, func(tx *gorm.DB) error {
		if err := tx.Create(&Vendor{Name: "cache-window-b-publication"}).Error; err != nil {
			return err
		}
		if err := tx.Create(&channelB).Error; err != nil {
			return err
		}
		return tx.Create(&Ability{ChannelId: channelB.Id, Model: "cache-window-b", Group: "default", Enabled: true}).Error
	}))
	var count int64
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "cache-window-b").Count(&count).Error)
	require.Zero(t, count, "B must actually remain unreconciled before the next cycle")
	resume.Do(func() { close(observer.release) })
	err := <-done
	joined = true
	require.NoError(t, err)
	summary, err := reconcileEnabledModelMetadata(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.CreatedModels, "A must not cache B as already reconciled")
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "cache-window-b").Count(&count).Error)
	assert.Equal(t, int64(1), count, "a later verified B epoch still needs its own metadata backfill")
}

func TestCatalogRefreshOrdinaryNoopRejectsUnacknowledgedInput(t *testing.T) {
	db := catalogReloadTestDB(t)
	require.NoError(t, db.Create(&Model{ModelName: "noop-money", BillingCurrency: "CNY", NameRule: NameRuleExact}).Error)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"noop-money":2}`, "USDExchangeRate": "7"}))
	require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"noop-money":9}`}).Error)
	require.NoError(t, db.Save(&Option{Key: "USDExchangeRate", Value: "14"}).Error)
	// Relative to fresh SQL this FX save is a no-op, but that fresh input is
	// not the locally/shared acknowledged epoch. Never publish only its price.
	err := UpdateOptionsBulk(map[string]string{"USDExchangeRate": "14"})
	if err != nil {
		assert.ErrorIs(t, err, ErrCatalogPublicationPending)
	}
	selectionErr := WithCatalogPricingRead(context.Background(), "noop-money", func() error {
		money, _, err := ResolvePublishedCatalogMoneyContext("noop-money")
		require.NoError(t, err)
		price := ratio_setting.GetModelPriceMap()["noop-money"]
		assert.True(t, (price == 2 && money.CNYPerUSD == 7) || (price == 9 && money.CNYPerUSD == 14), "a selection may use a complete old/new epoch, never price=%v with FX=%v", price, money.CNYPerUSD)
		return nil
	})
	if selectionErr != nil {
		assert.ErrorIs(t, selectionErr, ErrCatalogPublicationPending)
	}
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	require.NoError(t, WithCatalogPricingRead(context.Background(), "noop-money", func() error {
		money, _, err := ResolvePublishedCatalogMoneyContext("noop-money")
		require.NoError(t, err)
		assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["noop-money"])
		assert.Equal(t, float64(14), money.CNYPerUSD)
		return nil
	}))
}

func TestCatalogRefreshCanonicalRawNoopDoesNotRepublish(t *testing.T) {
	db := catalogReloadTestDB(t)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"raw-priced":2}`}))
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	const raw = ` { "raw-priced" : 2.0 } `
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": raw}))
	var option Option
	require.NoError(t, db.Where(map[string]any{"key": "ModelPrice"}).First(&option).Error)
	assert.Equal(t, raw, option.Value)
	common.OptionMapRWMutex.RLock()
	published := common.OptionMap["ModelPrice"]
	common.OptionMapRWMutex.RUnlock()
	assert.Equal(t, raw, published, "canonical fingerprints must not rewrite accepted raw representation")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	updatePricingLock.Lock()
	err := recoverCatalogSyncRuntime(ctx, loadOptionsFromDatabaseGuarded)
	updatePricingLock.Unlock()
	require.NoError(t, err, "a later unchanged reload must not compile/reinstall formatting-only prices")
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.Equal(t, before, after, "do not add an ACK/state mutation to conceal a formatting-only republish")
}

func TestCatalogRefreshCanonicalFingerprintPreservesInvalidityAndPrecision(t *testing.T) {
	for _, scenario := range []string{"duplicate", "precision"} {
		t.Run(scenario, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			initial, next := `{"semantic-map":2}`, `{"semantic-map":2,"semantic-map":2}`
			if scenario == "precision" {
				initial, next = `{"semantic-map":9007199254740992}`, `{"semantic-map":9007199254740993}`
			}
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": initial}))
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: next}).Error)
			updatePricingLock.Lock()
			err := PublishCatalogSyncRevision(context.Background(), before.Revision)
			updatePricingLock.Unlock()
			if scenario == "duplicate" {
				require.Error(t, err, "canonicalization must not hide ambiguous price-map identities")
			} else {
				require.ErrorIs(t, err, ErrCatalogWriterBusy, "exact decimal drift must not collapse through a float64 fingerprint")
			}
			var pending CatalogSyncState
			require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
			assert.Equal(t, "committed_pending_publish", pending.PublicationState)
			require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "semantic-map", func() error { t.Error("unverified map escaped"); return nil }), ErrCatalogPublicationPending)
			if scenario == "duplicate" {
				require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: initial}).Error)
			}
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
		})
	}
}

func TestCatalogRefreshMixedPasskeyFXPublishesMoney(t *testing.T) {
	db := catalogReloadTestDB(t)
	require.NoError(t, db.Create(&Model{ModelName: "fx-passkey", BillingCurrency: "CNY", NameRule: NameRuleExact}).Error)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"fx-passkey":2}`, "USDExchangeRate": "7"}))
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	values := map[string]string{"ServerAddress": "https://fx-login.example.com", "USDExchangeRate": "14"}
	_, err := UpdatePasskeyDomainOptions(values, true, "")
	require.NoError(t, err)
	var preview CatalogSyncState
	require.NoError(t, db.First(&preview, CatalogSyncStateID).Error)
	assert.Equal(t, before, preview, "preview must not publish FX or its fingerprint")
	assert.Empty(t, system_setting.ServerAddress)
	require.NoError(t, UpdateOptionsBulk(values))
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.NotEqual(t, before.RuntimeFingerprint, after.RuntimeFingerprint, "a mixed FX-only save must acknowledge a new complete epoch")
	require.NoError(t, WithCatalogPricingRead(context.Background(), "fx-passkey", func() error {
		money, _, err := ResolvePublishedCatalogMoneyContext("fx-passkey")
		require.NoError(t, err)
		assert.Equal(t, float64(14), money.CNYPerUSD)
		return nil
	}))
	assert.Equal(t, "https://fx-login.example.com", system_setting.ServerAddress)
	values["USDExchangeRate"] = "21"
	updatePricingLock.Lock()
	err = UpdateOptionsBulk(values)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy, "committed mixed FX publication failure must retain the real pending gate")
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "fx-passkey", func() error { t.Error("unacknowledged FX escaped"); return nil }), ErrCatalogPublicationPending)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	require.NoError(t, WithCatalogPricingRead(context.Background(), "fx-passkey", func() error {
		money, _, err := ResolvePublishedCatalogMoneyContext("fx-passkey")
		require.NoError(t, err)
		assert.Equal(t, float64(21), money.CNYPerUSD)
		return nil
	}))
}

func TestCatalogRefreshSchedulerCancelsChannelSQL(t *testing.T) {
	db := catalogReloadTestDB(t)
	previous := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = previous })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type blocked struct {
		transaction  *gorm.DB
		queryContext context.Context
	}
	entered := make(chan blocked, 1)
	queryResult := make(chan error, 1)
	var once sync.Once
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("scheduler-channel-before", func(tx *gorm.DB) {
		if tx.Statement.Table != "channels" || tx.Statement.ConnPool != db.Statement.ConnPool {
			return
		}
		once.Do(func() {
			blocker := db.Begin()
			tx.AddError(blocker.Exec("LOCK TABLE channels IN ACCESS EXCLUSIVE MODE").Error)
			tx.Statement.Settings.Store("scheduler-channel-blocked", true)
			entered <- blocked{blocker, tx.Statement.Context}
		})
	}))
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("scheduler-channel-after", func(tx *gorm.DB) {
		if _, ok := tx.Statement.Settings.Load("scheduler-channel-blocked"); ok {
			queryResult <- tx.Error
		}
	}))
	defer db.Callback().Query().Remove("scheduler-channel-before")
	defer db.Callback().Query().Remove("scheduler-channel-after")
	done := make(chan struct{})
	go func() { SyncCatalogCaches(ctx, 1); close(done) }()
	var blockedSQL blocked
	select {
	case blockedSQL = <-entered:
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("scheduler did not enter ordinary channel SQL")
	}
	defer blockedSQL.transaction.Rollback()
	cancel()
	select {
	case err := <-queryResult:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Error("scheduler cancellation did not cancel the actual channel SQL")
		require.NoError(t, blockedSQL.transaction.Rollback().Error)
		<-queryResult
	}
	assert.ErrorIs(t, blockedSQL.queryContext.Err(), context.Canceled)
	_ = blockedSQL.transaction.Rollback().Error
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler leaked after cancellation")
	}
	assertRuntimeLocksReleased(t)
}

func TestCatalogRefreshReconcileCancelsWriterWait(t *testing.T) {
	db := catalogReloadTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	catalogBarrier.RLock()
	_, err := reconcileEnabledModelMetadata(ctx, true)
	catalogBarrier.RUnlock()
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assertRuntimeLocksReleased(t)
	_, err = reconcileEnabledModelMetadata(context.Background(), true)
	require.NoError(t, err)
	var count int64
	require.NoError(t, db.Model(&CatalogSyncState{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestCatalogRefreshReconcileNeverReplaysCommittedSQL(t *testing.T) {
	db := catalogReloadTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	attempts := 0
	updatePricingLock.Lock()
	err := withMarketplaceOrderTransactionContext(ctx, db, func(tx *gorm.DB) error {
		attempts++
		return tx.Create(&Vendor{Name: "reconcile-once"}).Error
	})
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	assert.NoError(t, ctx.Err(), "must return after the SQL attempt, not retry until deadline")
	assert.Equal(t, 1, attempts)
	var count int64
	require.NoError(t, db.Model(&Vendor{}).Where("name = ?", "reconcile-once").Count(&count).Error)
	assert.Equal(t, int64(1), count)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
}

func TestCatalogRefreshUnchangedReconciliationDoesNotRepublish(t *testing.T) {
	db := catalogReloadTestDB(t)
	_, err := reconcileEnabledModelMetadata(context.Background(), true)
	require.NoError(t, err)
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	common.OptionMapRWMutex.Lock()
	var release sync.Once
	defer release.Do(common.OptionMapRWMutex.Unlock)
	done := make(chan error, 1)
	go func() { _, err := reconcileEnabledModelMetadata(ctx, true); done <- err }()
	select {
	case err := <-done:
		require.NoError(t, err, "complete unchanged proof must skip typed publication and recompilation")
	case <-ctx.Done():
		t.Error("unchanged reconciliation blocked on global configuration publication")
		release.Do(common.OptionMapRWMutex.Unlock)
		<-done
	}
	release.Do(common.OptionMapRWMutex.Unlock)
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.Equal(t, before, after)
}

func TestCatalogRefreshOperationalChannelStatsDoNotInvalidateEpoch(t *testing.T) {
	db := catalogReloadTestDB(t)
	channel := Channel{Key: "local-fixture", Models: "whole", Group: "default", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{"used_quota": 123, "response_time": 456, "test_time": 789, "balance": 50, "balance_updated_time": 1000}).Error)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	updatePricingLock.Lock()
	err := RecoverCatalogSyncRuntime(ctx)
	updatePricingLock.Unlock()
	require.NoError(t, err, "ordinary traffic/accounting counters must not require pricing republish")
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.Equal(t, before, after)
	// This is a different progress guarantee: accounting that commits AFTER a
	// pending epoch was captured cannot make every final proof perpetually stale.
	require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"whole":9}`}).Error)
	updatePricingLock.Lock()
	err = PublishCatalogSyncRevision(context.Background(), before.Revision)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	stage := runtimeStageForTest(t, db)
	require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{"used_quota": 999, "response_time": 1001, "test_time": 1002, "balance": 45, "balance_updated_time": 1003}).Error)
	require.NoError(t, TryWithCatalogWriteBarrier(context.Background(), func() error { return publishCatalogRuntimeGuarded(context.Background(), stage) }))
	require.NoError(t, WithCatalogPricingRead(context.Background(), "whole", func() error {
		assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["whole"])
		return nil
	}))
}

func TestCatalogRefreshRetryReleasesReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	first := make(chan struct{})
	done := make(chan error, 1)
	catalogBarrier.RLock()
	var release sync.Once
	defer release.Do(catalogBarrier.RUnlock)
	attempts := 0
	go func() {
		done <- retryCatalogRefresh(ctx, "test-reader", func(ctx context.Context) error {
			attempts++
			err := TryWithCatalogWriteBarrier(ctx, func() error { return nil })
			if attempts == 1 {
				close(first)
			}
			return err
		})
	}()
	select {
	case <-first:
	case <-ctx.Done():
		t.Fatal("did not attempt recovery")
	}
	release.Do(catalogBarrier.RUnlock)
	require.NoError(t, <-done)
	assert.GreaterOrEqual(t, attempts, 2)
	assertRuntimeLocksReleased(t)
}

func TestCatalogRefreshAllInstallLocksKeepOldProjection(t *testing.T) {
	for _, site := range []string{"pricing", "endpoints", "channels", "aliases", "groups", "option_map"} {
		t.Run(site, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"whole":2}`}))
			require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"whole":9}`}).Error)
			beforePrices := ratio_setting.GetModelPriceMap()
			beforeDisplay, beforeVendors := pricingMap, vendorsList
			beforeEndpoints, beforeSupported := modelSupportEndpointTypes, supportedEndpointMap
			beforeGroups, beforeQuota := modelEnableGroups, modelQuotaTypeMap
			beforeAliases := taskAliasViewPtr.Load()
			locks := map[string]interface {
				Lock()
				Unlock()
			}{
				"pricing": &updatePricingLock, "endpoints": &modelSupportEndpointsLock,
				"channels": &channelSyncLock, "aliases": &taskAliasRebuildMu, "groups": &modelEnableGroupsLock,
				"option_map": &common.OptionMapRWMutex,
			}
			lock := locks[site]
			lock.Lock()
			err := PublishCatalogSyncRevision(context.Background(), catalogRuntime.revision)
			lock.Unlock()
			require.ErrorIs(t, err, ErrCatalogWriterBusy)
			assert.Contains(t, err.Error(), "lock="+site)
			assert.Equal(t, beforePrices, ratio_setting.GetModelPriceMap())
			assert.Equal(t, beforeDisplay, pricingMap)
			assert.Equal(t, beforeVendors, vendorsList)
			assert.Equal(t, beforeEndpoints, modelSupportEndpointTypes)
			assert.Equal(t, beforeSupported, supportedEndpointMap)
			assert.Equal(t, beforeGroups, modelEnableGroups)
			assert.Equal(t, beforeQuota, modelQuotaTypeMap)
			assert.Same(t, beforeAliases, taskAliasViewPtr.Load())
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["whole"])
		})
	}
}

func TestCatalogRefreshMediaProjectionUsesCandidate(t *testing.T) {
	preserveCatalogCandidate(t)
	candidate, err := stageCommittedCatalogPricing(nil, catalogPricingDependencies{})
	require.NoError(t, err)
	candidate.video.Seedance25720p = 123
	candidate.grok.Image20Low1K = 0.55
	prices := catalogProjectionPrices{candidate: candidate}
	video, ok := prices.video("doubao-seedance-2-5-260628")
	require.True(t, ok)
	assert.Equal(t, float64(123), video.Rows[0].WithoutVideo)
	assert.Equal(t, "cny_per_million_tokens", video.Unit)
	grok, ok := prices.grok("grok-imagine-image-2.0")
	require.True(t, ok)
	assert.Equal(t, map[string]float64{"low/1k": 0.55, "low/2k": 0.06, "medium/1k": 0.06, "medium/2k": 0.08}, grok.OutputPrices)
	assert.Equal(t, float64(0.01), grok.ImageInputPrice)
	// Both public and detached callers share the same derivation after install.
	candidate.publishGuarded()
	for _, name := range ratio_setting.StarAIVideoPricingModels() {
		local, localOK := prices.video(name)
		global, globalOK := ratio_setting.GetStarAIVideoPricing(name)
		assert.Equal(t, localOK, globalOK)
		assert.Equal(t, local, global)
	}
	for _, name := range []string{"grok-imagine-image", "grok-imagine-image-quality", "grok-imagine-image-2.0", "grok-imagine-video", "grok-imagine-video-1.5", "unknown"} {
		local, localOK := prices.grok(name)
		global, globalOK := ratio_setting.GetMoliiGrokCatalogPricing(name)
		assert.Equal(t, localOK, globalOK)
		assert.Equal(t, local, global)
	}
}
