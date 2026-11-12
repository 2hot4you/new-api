package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// A healthy unchanged reload must not turn pricing-cache contention into a
// durable outage. Observe the actual durable transition, not just its postimage.
func TestCatalogPublicationNoopReloadDoesNotRepublish(t *testing.T) {
	db := catalogReloadTestDB(t)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`}))
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	pendingWrites := 0
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("liveness-noop-pending", func(tx *gorm.DB) {
		if tx.Statement.Table != "catalog_sync_states" {
			return
		}
		if values, ok := tx.Statement.Dest.(map[string]any); ok && values["publication_state"] == "committed_pending_publish" {
			pendingWrites++
		}
	}))
	defer db.Callback().Update().Remove("liveness-noop-pending")
	updatePricingLock.Lock()
	err := loadOptionsFromDatabase()
	updatePricingLock.Unlock()
	assert.NoError(t, err, "a no-op reload must not attempt a pricing-cache installation")
	assert.Zero(t, pendingWrites, "unchanged authoritative inputs must never close the durable ready gate")
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.Equal(t, before, after)
	assert.NoError(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
		assert.Equal(t, float64(2), ratio_setting.GetModelPriceMap()["liveness"])
		return nil
	}))
}

func catalogRetiredOptionsTestDB(t *testing.T, engine string, deadline, theme bool) *gorm.DB {
	t.Helper()
	db := catalogFenceTestDB(t, engine)
	initializeOrdinaryCatalogTest(t, db)
	policy, retireAt := CurrentRequestPolicy(), LegacyAccessTokenRetireAt()
	passkey, server := *system_setting.GetPasskeySettings(), system_setting.ServerAddress
	t.Cleanup(func() {
		legacyAccessTokenRetireAt.Store(retireAt)
		for key, value := range policy.Options {
			require.NoError(t, updateOptionMap(key, value))
		}
		requestPolicySnapshot.Store(policy)
		*system_setting.GetPasskeySettings(), system_setting.ServerAddress = passkey, server
	})
	if deadline {
		require.NoError(t, EnsureLegacyAccessTokenRetireAt(1700000000))
		require.NoError(t, EnsureLegacyAccessTokenRetireAt(1700000100), "repeat startup must retain the first deadline")
		require.Equal(t, int64(1702592000), LegacyAccessTokenRetireAt())
	}
	if theme {
		require.NoError(t, normalizeRetiredThemeOption())
	}
	require.NoError(t, db.Create(&Option{Key: "ModelPrice", Value: `{"liveness":2}`}).Error)
	require.NoError(t, db.Create(&Option{Key: "RetryTimes", Value: "3"}).Error)
	require.NoError(t, InitOptionMapBootstrap(context.Background()))
	require.NoError(t, CompleteOptionMapInitialization(context.Background()))
	return db
}

func TestCatalogPublicationRetiredOptionsKeepNoopReadOnly(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			for _, fixture := range []struct {
				name     string
				deadline bool
				theme    bool
			}{
				{"none", false, false}, {"deadline", true, false},
				{"theme", false, true}, {"both", true, true},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					db := catalogRetiredOptionsTestDB(t, engine, fixture.deadline, fixture.theme)
					var before CatalogSyncState
					require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
					var stored []Option
					require.NoError(t, db.Order(commonKeyCol).Find(&stored).Error)
					if fixture.deadline {
						assert.Contains(t, stored, Option{Key: "LegacyAccessTokenRetireAt", Value: "1702592000"})
					}
					if fixture.theme {
						assert.Contains(t, stored, Option{Key: "theme.frontend", Value: "default"})
					}
					stage := runtimeStageForTest(t, db)
					stage.optionsOnly = true
					var writes atomic.Int64
					observeWrite := func(tx *gorm.DB) {
						if tx.Statement.Table == "options" || tx.Statement.Table == "catalog_sync_states" {
							writes.Add(1)
						}
					}
					require.NoError(t, db.Callback().Create().Before("gorm:create").Register("retired-noop-create", observeWrite))
					require.NoError(t, db.Callback().Update().Before("gorm:update").Register("retired-noop-update", observeWrite))
					require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("retired-noop-delete", observeWrite))
					t.Cleanup(func() {
						db.Callback().Create().Remove("retired-noop-create")
						db.Callback().Update().Remove("retired-noop-update")
						db.Callback().Delete().Remove("retired-noop-delete")
					})
					for range 3 {
						err := tryWithCatalogCaptureBarrier(context.Background(), func() error {
							return loadOptionsFromDatabaseGuarded(context.Background(), stage)
						})
						assert.NoError(t, err, "persisted-but-unpublished startup metadata is not an effective option change")
					}
					assert.Zero(t, writes.Load(), "no-op preflight must not write any option or catalog state")
					var after CatalogSyncState
					require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
					assert.Equal(t, before, after)
					var actual []Option
					require.NoError(t, db.Order(commonKeyCol).Find(&actual).Error)
					assert.Equal(t, stored, actual, "server-managed metadata must remain stored verbatim")
					common.OptionMapRWMutex.RLock()
					assert.NotContains(t, common.OptionMap, "LegacyAccessTokenRetireAt")
					assert.NotContains(t, common.OptionMap, "theme.frontend")
					common.OptionMapRWMutex.RUnlock()
					if fixture.deadline {
						assert.Equal(t, int64(1702592000), LegacyAccessTokenRetireAt())
					}
					assert.NoError(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
						assert.Equal(t, float64(2), ratio_setting.GetModelPriceMap()["liveness"])
						return nil
					}))
				})
			}
			t.Run("effective-changes", func(t *testing.T) {
				db := catalogRetiredOptionsTestDB(t, engine, true, true)
				for _, change := range []string{"changed", "uncached", "near-match"} {
					t.Run(change, func(t *testing.T) {
						key, value := "RetryTimes", "4"
						switch change {
						case "changed":
							require.NoError(t, db.Save(&Option{Key: key, Value: value}).Error)
						case "uncached":
							common.OptionMapRWMutex.Lock()
							delete(common.OptionMap, key)
							common.OptionMapRWMutex.Unlock()
						case "near-match":
							key, value = "theme.frontend.extra", "retained-effective-value"
							require.NoError(t, db.Create(&Option{Key: key, Value: value}).Error)
						}
						stage := runtimeStageForTest(t, db)
						stage.optionsOnly = true
						before := CurrentRequestPolicy()
						err := tryWithCatalogCaptureBarrier(context.Background(), func() error {
							return loadOptionsFromDatabaseGuarded(context.Background(), stage)
						})
						require.ErrorIs(t, err, errCatalogOptionsNeedWriter, "effective changes must still require writer exclusion")
						assert.Same(t, before, CurrentRequestPolicy(), "preflight must not partially publish policy")
						require.NoError(t, loadOptionsFromDatabase())
						common.OptionMapRWMutex.RLock()
						assert.Equal(t, value, common.OptionMap[key])
						common.OptionMapRWMutex.RUnlock()
						assert.Equal(t, 4, CurrentRequestPolicy().RetryTimes)
					})
				}
			})
		})
	}
}

func TestCatalogPublicationRetiredOptionsKeepReadersAvailable(t *testing.T) {
	catalogRetiredOptionsTestDB(t, "postgres", true, true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	catalogBarrier.RLock()
	var release sync.Once
	done, preflight := make(chan error, 1), make(chan error, 1)
	joined := false
	defer func() {
		cancel()
		release.Do(catalogBarrier.RUnlock)
		if !joined {
			<-done
		}
	}()
	var writerCalls atomic.Int64
	var firstPreflight sync.Once
	go func() {
		done <- recoverCatalogSyncRuntime(ctx, func(ctx context.Context, stage *catalogRuntimeStage) error {
			if !stage.optionsOnly {
				writerCalls.Add(1)
			}
			err := loadOptionsFromDatabaseGuarded(ctx, stage)
			if stage.optionsOnly {
				// A retried busy attempt must not strand cleanup on an unread
				// second signal. The actual loader's locks are released here.
				firstPreflight.Do(func() { preflight <- err })
			}
			return err
		})
	}()
	var preflightErr error
	select {
	case preflightErr = <-preflight:
	case <-ctx.Done():
		t.Fatal("reload never reached the actual unchanged option preflight")
	}
	assert.NoError(t, preflightErr, "retained startup metadata must not request a catalog writer")
	if errors.Is(preflightErr, errCatalogOptionsNeedWriter) {
		// The held R makes the buggy escalation persist, rather than relying
		// on sampling a brief intent window or scheduling a coincidental read.
		require.Eventually(t, func() bool { return catalogWriterIntent.Load() > 0 }, time.Second, time.Millisecond)
	}
	assert.NoError(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
		assert.Equal(t, float64(2), ratio_setting.GetModelPriceMap()["liveness"])
		return nil
	}), "a regular unchanged refresh must keep new pricing capture available")
	if preflightErr != nil {
		cancel() // bound the old-source RED without releasing the original R
	}
	select {
	case err := <-done:
		joined = true
		if preflightErr == nil {
			assert.NoError(t, err, "no-op recovery must finish while the original reader remains held")
		}
	case <-ctx.Done():
		if preflightErr == nil {
			t.Error("no-op recovery needed the original reader to release")
		}
	}
	assert.Zero(t, writerCalls.Load())
	release.Do(catalogBarrier.RUnlock)
	if !joined {
		<-done
		joined = true
	}
	assert.Zero(t, catalogWriterIntent.Load(), "no writer admission reservation may leak after the reload")
	assertRuntimeLocksReleased(t)
}

// Use the existing diagnostic sink only to rendezvous after the actual
// scheduler cycle has returned its committed-reconciliation failure. Correctness
// below is asserted against SQL COMMIT, durable state and real pricing reads.
type catalogPeriodicFailureWriter struct {
	writer  io.Writer
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *catalogPeriodicFailureWriter) Write(data []byte) (int, error) {
	if strings.Contains(string(data), "catalog refresh source=periodic phase=channels result=failed") {
		w.once.Do(func() { close(w.entered); <-w.release })
	}
	return w.writer.Write(data)
}

func TestCatalogPublicationSchedulerRecoversCommittedReconciliation(t *testing.T) {
	db := catalogReloadTestDB(t)
	previousMemory := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemory })
	const modelName = "review-scheduler-missing"
	channel := Channel{Key: "scheduler-fixture", Models: modelName, Status: common.ChannelStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&Ability{ChannelId: channel.Id, Model: modelName, Group: "default", Enabled: true}).Error)
	// Load the route but deliberately leave metadata missing. The first
	// scheduler recovery is therefore no-op; its real reconciliation creates it.
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	var creates atomic.Int64
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("liveness-periodic-create", func(tx *gorm.DB) {
		if tx.Statement.Table == "models" {
			creates.Add(1)
		}
	}))
	defer db.Callback().Create().Remove("liveness-periodic-create")
	committed := make(chan error, 1)
	var ackOnce sync.Once
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("liveness-periodic-ack", func(tx *gorm.DB) {
		values, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table != "catalog_sync_states" || !ok || values["publication_state"] != "ready" {
			return
		}
		ackOnce.Do(func() {
			go func() {
				// An independent SELECT FOR UPDATE cannot pass the ACK's row
				// lock until its actual COMMIT. A callback seeing UPDATE alone
				// would not prove durable publication.
				lookup, stop := context.WithTimeout(context.Background(), 5*time.Second)
				defer stop()
				var state CatalogSyncState
				err := lockForUpdate(db.WithContext(lookup)).First(&state, CatalogSyncStateID).Error
				if err == nil && state.PublicationState != "ready" {
					err = errors.New("ACK commit observer did not find durable ready state")
				}
				committed <- err
			}()
		})
	}))
	defer db.Callback().Update().Remove("liveness-periodic-ack")
	common.LogWriterMu.Lock()
	priorWriter := gin.DefaultErrorWriter
	observer := &catalogPeriodicFailureWriter{writer: priorWriter, entered: make(chan struct{}), release: make(chan struct{})}
	gin.DefaultErrorWriter = observer
	common.LogWriterMu.Unlock()
	var resume sync.Once
	defer func() {
		resume.Do(func() { close(observer.release) })
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = priorWriter
		common.LogWriterMu.Unlock()
	}()
	updatePricingLock.Lock()
	var releasePricing sync.Once
	defer releasePricing.Do(updatePricingLock.Unlock)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { SyncCatalogCaches(ctx, 5); close(done) }()
	defer func() {
		cancel()
		resume.Do(func() { close(observer.release) })
		releasePricing.Do(updatePricingLock.Unlock)
		<-done
	}()
	select {
	case <-observer.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("scheduler did not reach actual committed reconciliation failure")
	}
	var pending CatalogSyncState
	require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
	require.Equal(t, "committed_pending_publish", pending.PublicationState)
	require.Equal(t, int64(1), creates.Load(), "the real metadata SQL closure must have committed once")
	releasePricing.Do(updatePricingLock.Unlock)
	resume.Do(func() { close(observer.release) })
	select {
	case err := <-committed:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Error("committed reconciliation stayed pending until the full five-second polling period instead of waking short recovery")
	}
	cancel()
	<-done
	var ready CatalogSyncState
	require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
	assert.Equal(t, pending.Revision, ready.Revision, "recovery must not replay the business mutation")
	assert.Equal(t, ready.Revision, ready.RuntimeRevision)
	assert.Equal(t, "ready", ready.PublicationState)
	assert.Equal(t, int64(1), creates.Load())
	assertRuntimeLocksReleased(t)
	assert.NoError(t, WithCatalogPricingRead(context.Background(), modelName, func() error { return nil }))
}

func TestCatalogPublicationOptionLockCancellation(t *testing.T) {
	for _, lockName := range []string{"request-policy", "passkey"} {
		t.Run(lockName, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`}))
			require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"liveness":9}`}).Error)
			lock := &requestPolicyOptionMutex
			if lockName == "passkey" {
				lock = &passkeyOptionMutex
			}
			lock.Lock()
			var released sync.Once
			defer released.Do(lock.Unlock)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			entered := make(chan struct{})
			var enteredOnce sync.Once
			done := make(chan error, 1)
			go func() {
				done <- recoverCatalogSyncRuntime(ctx, func(ctx context.Context, stage *catalogRuntimeStage) error {
					// Existing option-loader seam: the real publisher owns catalog
					// W and all installation locks here, before the option lock.
					enteredOnce.Do(func() { close(entered) })
					return loadOptionsFromDatabaseGuarded(ctx, stage)
				})
			}()
			select {
			case <-entered:
			case err := <-done:
				t.Fatalf("recovery ended before option-lock acquisition: %v", err)
			case <-time.After(5 * time.Second):
				released.Do(lock.Unlock)
				cancel()
				<-done
				t.Fatal("recovery did not reach actual option loader")
			}
			cancel()
			select {
			case err := <-done:
				assert.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				// Failure-only release bounds the RED: an uncancelable Lock must
				// not strand the publisher or poison later package tests.
				released.Do(lock.Unlock)
				assert.ErrorIs(t, <-done, context.Canceled)
				t.Error("canceled recovery retained catalog/cache locks while waiting for " + lockName)
			}
			released.Do(lock.Unlock)
			assertRuntimeLocksReleased(t)
			for _, optionLock := range []*sync.Mutex{&requestPolicyOptionMutex, &passkeyOptionMutex} {
				if assert.True(t, optionLock.TryLock(), "option lock leaked after cancellation") {
					optionLock.Unlock()
				}
			}
			var pending CatalogSyncState
			require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
			assert.Equal(t, "committed_pending_publish", pending.PublicationState)
			assert.Equal(t, float64(2), ratio_setting.GetModelPriceMap()["liveness"], "canceled option loading must precede every projection swap")
			require.NoError(t, recoverCatalogSyncRuntime(context.Background(), loadOptionsFromDatabaseGuarded))
		})
	}
}

func TestCatalogPublicationUnknownCaptureWithReaderFailsClosed(t *testing.T) {
	db := catalogReloadTestDB(t)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`}))
	failure := errors.New("authoritative capture unavailable")
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("liveness-unknown-capture", func(tx *gorm.DB) {
		// Fail the actual reference transaction before its capture callback,
		// not a malformed captured value that is known to require pending.
		if tx.Statement.Table == "catalog_sync_states" && tx.Statement.ConnPool != db.Statement.ConnPool {
			tx.AddError(failure)
		}
	}))
	catalogBarrier.RLock()
	err := RecoverCatalogSyncRuntime(context.Background())
	catalogBarrier.RUnlock()
	require.NoError(t, db.Callback().Query().Remove("liveness-unknown-capture"))
	require.ErrorIs(t, err, failure)
	var state CatalogSyncState
	require.NoError(t, db.First(&state, CatalogSyncStateID).Error)
	assert.Equal(t, "ready", state.PublicationState, "an unknown capture must not fabricate a durable mutation")
	assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
		t.Error("removing the transient SQL failure must not revive an unproven local catalog")
		return nil
	}), ErrCatalogPublicationPending)
	assertRuntimeLocksReleased(t)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	require.NoError(t, WithCatalogPricingRead(context.Background(), "liveness", func() error { return nil }))
}

func TestCatalogPublicationNoopReloadKeepsSelectionsDuringCapture(t *testing.T) {
	db := catalogReloadTestDB(t)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`, "RetryTimes": "3"}))
	require.NoError(t, db.Save(&Option{Key: "RetryTimes", Value: "4"}).Error)
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	observed := 0
	armed := false
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("liveness-noop-selection", func(tx *gorm.DB) {
		if tx.Statement.Table != "options" || !armed {
			return
		}
		armed = false
		observed++
		assert.NoError(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
			assert.Equal(t, float64(2), ratio_setting.GetModelPriceMap()["liveness"])
			return nil
		}), "an unchanged capture must not make existing pricing selection unavailable")
	}))
	defer db.Callback().Query().Remove("liveness-noop-selection")
	for range 3 {
		armed = true
		require.NoError(t, loadOptionsFromDatabase())
	}
	require.Equal(t, 3, observed, "must inspect the first real reload capture in each round, not the final short installation boundary")
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.Equal(t, before, after)
	assert.Equal(t, 4, CurrentRequestPolicy().RetryTimes, "noncatalog values must still refresh")
}

// Use a real externally committed price change and failed installation to
// create pending, rather than inventing coordination-state rows in the fixture.
func catalogLivenessPending(t *testing.T) *gorm.DB {
	t.Helper()
	db := catalogReloadTestDB(t)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`}))
	require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"liveness":9}`}).Error)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	updatePricingLock.Lock()
	err := RecoverCatalogSyncRuntime(ctx)
	updatePricingLock.Unlock()
	require.Error(t, err)
	var pending CatalogSyncState
	require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
	require.Equal(t, "committed_pending_publish", pending.PublicationState)
	return db
}

func TestCatalogPublicationRecoveryReaderDeadline(t *testing.T) {
	db := catalogLivenessPending(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	catalogBarrier.RLock()
	err := RecoverCatalogSyncRuntime(ctx)
	catalogBarrier.RUnlock()
	assert.ErrorIs(t, err, context.DeadlineExceeded, "transient reader contention must be retried until the caller's deadline, not returned immediately as busy")
	assertRuntimeLocksReleased(t)
	var pending CatalogSyncState
	require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
	assert.Equal(t, "committed_pending_publish", pending.PublicationState)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()), "a later recovery must succeed after the reader releases")
	require.NoError(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
		assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["liveness"])
		return nil
	}))
}

func TestCatalogPublicationRecoveryReaderCancellation(t *testing.T) {
	db := catalogLivenessPending(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, proceed := make(chan struct{}), make(chan struct{})
	var once sync.Once
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("liveness-cancel-reader", func(tx *gorm.DB) {
		if tx.Statement.Table == "catalog_sync_states" && tx.Statement.ConnPool == db.Statement.ConnPool {
			once.Do(func() { close(entered); <-proceed })
		}
	}))
	defer db.Callback().Query().Remove("liveness-cancel-reader")
	catalogBarrier.RLock()
	var release sync.Once
	defer release.Do(catalogBarrier.RUnlock)
	done := make(chan error, 1)
	go func() { done <- RecoverCatalogSyncRuntime(ctx) }()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("recovery ended before initial state capture: %v", err)
	case <-time.After(5 * time.Second):
		close(proceed)
		t.Fatal("recovery did not reach initial state capture")
	}
	cancel()
	close(proceed)
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		release.Do(catalogBarrier.RUnlock)
		<-done
		t.Error("canceled recovery remained blocked behind a catalog reader")
	}
	release.Do(catalogBarrier.RUnlock)
	assertRuntimeLocksReleased(t)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
}

func TestCatalogPublicationInvalidSameRevisionDriftGatesReads(t *testing.T) {
	for _, drift := range []string{"price", "currency", "fx"} {
		t.Run(drift, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			require.NoError(t, db.Create(&Model{ModelName: "liveness", NameRule: NameRuleExact, BillingCurrency: "USD"}).Error)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`, "USDExchangeRate": "7.25"}))
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			switch drift {
			case "price":
				require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"liveness":`}).Error)
			case "currency":
				require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "liveness").Update("billing_currency", "EUR").Error)
			case "fx":
				require.NoError(t, db.Save(&Option{Key: "USDExchangeRate", Value: "not-a-number"}).Error)
			}
			require.Error(t, loadOptionsFromDatabase(), "a same-revision drift must not be mistaken for an unchanged published input")
			var after CatalogSyncState
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Equal(t, before.Revision, after.Revision)
			assert.Equal(t, "committed_pending_publish", after.PublicationState, "detected invalid committed drift must gate other processes too")
			assert.Equal(t, float64(2), ratio_setting.GetModelPriceMap()["liveness"])
			assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
				t.Error("invalid drift allowed a stale pricing selection")
				return nil
			}), ErrCatalogPublicationPending)
		})
	}
}

func TestCatalogPublicationPreinstallConflictKeepsProjection(t *testing.T) {
	db := catalogReloadTestDB(t)
	vendor := Vendor{Name: "liveness-vendor", Status: 1}
	require.NoError(t, db.Create(&vendor).Error)
	metadata := completePublishedLLM()
	metadata.ModelName, metadata.BillingCurrency, metadata.VendorID = "liveness", "USD", vendor.Id
	require.NoError(t, db.Create(&metadata).Error)
	channel := Channel{Key: "liveness-fixture", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: "liveness"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&Ability{ChannelId: channel.Id, Model: "liveness", Group: "default", Enabled: true}).Error)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`}))
	beforePrices := maps.Clone(ratio_setting.GetModelPriceMap())
	beforePricing := GetPricing()
	beforeVendors := GetVendors()
	require.Len(t, beforePricing, 1)
	require.Len(t, beforeVendors, 1)
	require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"liveness":9}`}).Error)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	modelSupportEndpointsLock.Lock()
	err := RecoverCatalogSyncRuntime(ctx)
	modelSupportEndpointsLock.Unlock()
	require.Error(t, err, "the held projection lock must prevent installation")
	assert.Equal(t, beforePrices, ratio_setting.GetModelPriceMap(), "a later projection-lock failure must not swap typed prices first")
	assert.Equal(t, beforePricing, GetPricing(), "failed installation must preserve the public pricing projection")
	assert.Equal(t, beforeVendors, GetVendors())
	assertRuntimeLocksReleased(t)
	assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "liveness", func() error { return nil }), ErrCatalogPublicationPending)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["liveness"])
}

func TestCatalogPublicationOptionMapReservationCancellation(t *testing.T) {
	for _, scenario := range []string{"publication", "loader", "options-only"} {
		t.Run(scenario, func(t *testing.T) {
			db := catalogReloadTestDB(t)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`, "USDExchangeRate": "7", "RetryTimes": "3"}))
			prices, pricing, vendors := maps.Clone(ratio_setting.GetModelPriceMap()), GetPricing(), GetVendors()
			common.OptionMapRWMutex.RLock()
			raw := maps.Clone(common.OptionMap)
			common.OptionMapRWMutex.RUnlock()
			if scenario != "options-only" {
				require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"liveness":9}`}).Error)
				require.NoError(t, db.Save(&Option{Key: "USDExchangeRate", Value: "14"}).Error)
			}
			common.OptionMapRWMutex.RLock()
			var release sync.Once
			defer release.Do(common.OptionMapRWMutex.RUnlock)
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if scenario == "publication" {
					done <- RecoverCatalogSyncRuntime(ctx)
				} else {
					done <- recoverCatalogSyncRuntime(ctx, loadOptionsFromDatabaseGuarded)
				}
			}()
			select {
			case err := <-done:
				assert.ErrorIs(t, err, context.DeadlineExceeded)
			case <-time.After(time.Second):
				release.Do(common.OptionMapRWMutex.RUnlock)
				<-done
				t.Error("OptionMap contention ignored cancellation and retained publication locks")
			}
			release.Do(common.OptionMapRWMutex.RUnlock)
			assert.Equal(t, prices, ratio_setting.GetModelPriceMap())
			assert.Equal(t, pricing, GetPricing())
			assert.Equal(t, vendors, GetVendors())
			common.OptionMapRWMutex.RLock()
			assert.Equal(t, raw, common.OptionMap, "cancellation must precede every raw/typed option exchange")
			common.OptionMapRWMutex.RUnlock()
			assertRuntimeLocksReleased(t)
			for name, lock := range map[string]interface {
				TryLock() bool
				Unlock()
			}{
				"option_map": &common.OptionMapRWMutex, "policy": &requestPolicyOptionMutex, "passkey": &passkeyOptionMutex,
				"channels": &channelSyncLock, "aliases": &taskAliasRebuildMu, "groups": &modelEnableGroupsLock,
			} {
				if assert.True(t, lock.TryLock(), "%s reservation leaked", name) {
					lock.Unlock()
				}
			}
			require.NoError(t, recoverCatalogSyncRuntime(context.Background(), loadOptionsFromDatabaseGuarded))
		})
	}
}

func TestCatalogPublicationCanceledWriterPreservesOtherIntent(t *testing.T) {
	catalogBarrier.RLock()
	var release sync.Once
	defer release.Do(catalogBarrier.RUnlock)
	first, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	second, cancelSecond := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelSecond()
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	var calls atomic.Int64
	go func() { firstDone <- withCatalogRecoveryWriter(first, func() error { calls.Add(100); return nil }) }()
	go func() { secondDone <- withCatalogRecoveryWriter(second, func() error { calls.Add(1); return nil }) }()
	// Rendezvous only: observable assertions below protect the admission and
	// exactly-once callback contracts when one of two pending writers cancels.
	require.Eventually(t, func() bool { return catalogWriterIntent.Load() == 2 }, time.Second, time.Millisecond)
	cancelFirst()
	require.ErrorIs(t, <-firstDone, context.Canceled)
	assert.ErrorIs(t, tryWithCatalogCaptureBarrier(context.Background(), func() error { t.Error("cancellation erased another writer's reservation"); return nil }), ErrCatalogWriterBusy)
	release.Do(catalogBarrier.RUnlock)
	require.NoError(t, <-secondDone)
	assert.Equal(t, int64(1), calls.Load())
	require.NoError(t, WithCatalogReadBarrier(func() error { return nil }))
}

func TestCatalogPublicationPendingWriterBarsNewReaders(t *testing.T) {
	db := catalogLivenessPending(t)
	catalogBarrier.RLock()
	var release sync.Once
	defer release.Do(catalogBarrier.RUnlock)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- RecoverCatalogSyncRuntime(ctx) }()
	// Individual readers finish, but the original reader overlaps every new
	// arrival. Recovery must close admission rather than require a vacant instant.
	denied := assert.Eventually(t, func() bool {
		return errors.Is(tryWithCatalogCaptureBarrier(context.Background(), func() error { return nil }), ErrCatalogWriterBusy)
	}, time.Second, time.Millisecond, "pending recovery must bar new readers while waiting for existing readers")
	var blockingRead chan float64
	if denied {
		assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "liveness", func() error { t.Error("new pricing reader admitted"); return nil }), ErrCatalogWriterBusy)
		_, err := ExportManagedCatalog(context.Background(), "liveness")
		assert.ErrorIs(t, err, ErrCatalogWriterBusy)
		readCtx, stop := context.WithTimeout(context.Background(), 30*time.Millisecond)
		assert.ErrorIs(t, WithCatalogReadSnapshot(readCtx, func(*gorm.DB) error { t.Error("context reader bypassed writer admission"); return nil }), context.DeadlineExceeded)
		stop()
		blockingRead = make(chan float64, 1)
		go func() {
			_ = WithCatalogReadBarrier(func() error {
				blockingRead <- ratio_setting.GetModelPriceMap()["liveness"]
				return nil
			})
		}()
	}
	release.Do(catalogBarrier.RUnlock)
	require.NoError(t, <-done)
	if blockingRead != nil {
		assert.Equal(t, float64(9), <-blockingRead, "blocking readers wait until the pending writer publishes")
	}
	var ready CatalogSyncState
	require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
	assert.Equal(t, "ready", ready.PublicationState)
	assert.Equal(t, ready.Revision, ready.RuntimeRevision)
	require.NoError(t, WithCatalogReadBarrier(func() error {
		assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["liveness"])
		return nil
	}))
	assertRuntimeLocksReleased(t)
}

func TestCatalogPublicationStaleExpectedRevisionKeepsHealthyEpoch(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			initializeOrdinaryCatalogTest(t, db)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			var old CatalogSyncState
			require.NoError(t, db.First(&old, CatalogSyncStateID).Error)
			require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":9}`}))
			var healthy CatalogSyncState
			require.NoError(t, db.First(&healthy, CatalogSyncStateID).Error)
			require.Greater(t, healthy.Revision, old.Revision)
			require.Equal(t, healthy.Revision, healthy.RuntimeRevision)
			require.ErrorIs(t, PublishCatalogSyncRevision(context.Background(), old.Revision), ErrCatalogSyncPlanStale)
			assert.NotSame(t, DB, catalogRuntimeInvalid.Load(), "an obsolete target is not failed authoritative capture")
			require.NoError(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
				assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["liveness"])
				return nil
			}))
			var after CatalogSyncState
			require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
			assert.Equal(t, healthy, after)
			catalogRuntimeInvalid.Store(DB)
			require.ErrorIs(t, PublishCatalogSyncRevision(context.Background(), old.Revision), ErrCatalogSyncPlanStale)
			assert.Same(t, DB, catalogRuntimeInvalid.Load(), "stale targets cannot clear preexisting invalidation either")
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			captureFailure := errors.New("authoritative capture cleanup unavailable")
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("stale-target-joined-failure", func(tx *gorm.DB) {
				if tx.Statement.Table == "catalog_sync_states" && tx.Statement.ConnPool != db.Statement.ConnPool {
					tx.AddError(errors.Join(ErrCatalogSyncPlanStale, captureFailure))
				}
			}))
			err := PublishCatalogSyncRevision(context.Background(), old.Revision)
			require.NoError(t, db.Callback().Query().Remove("stale-target-joined-failure"))
			require.ErrorIs(t, err, captureFailure)
			require.ErrorIs(t, err, ErrCatalogSyncPlanStale)
			assert.Same(t, DB, catalogRuntimeInvalid.Load(), "stale sentinel cannot whitelist an unknown capture/cleanup failure")
			require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "liveness", func() error { t.Error("joined capture failure admitted pricing"); return nil }), ErrCatalogPublicationPending)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
		})
	}
}

func TestCatalogPublicationUpstreamObservationsKeepEpoch(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			initializeOrdinaryCatalogTest(t, db)
			channel := Channel{Key: "observation-local-only", Models: "liveness", Group: "default", Status: common.ChannelStatusEnabled}
			require.NoError(t, db.Create(&channel).Error)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			var before CatalogSyncState
			require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
			pendingWrites := 0
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("liveness-observation-pending", func(tx *gorm.DB) {
				if values, ok := tx.Statement.Dest.(map[string]any); ok && tx.Statement.Table == "catalog_sync_states" && values["publication_state"] == "committed_pending_publish" {
					pendingWrites++
				}
			}))
			defer db.Callback().Update().Remove("liveness-observation-pending")
			for _, raw := range []string{`{"upstream_model_update_last_check_time":100}`, `{"upstream_model_update_last_check_time":101,"upstream_model_update_last_detected_models":["new"],"upstream_model_update_last_removed_models":["old"]}`, `null`, `{}`} {
				require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Update("settings", raw).Error)
				require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
				var after CatalogSyncState
				require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
				assert.Equal(t, before, after, "pure observations must not change the acknowledged epoch: %s", raw)
				var stored Channel
				require.NoError(t, db.First(&stored, channel.Id).Error)
				assert.Equal(t, raw, stored.OtherSettings, "routing normalization must not rewrite stored observations")
			}
			assert.Zero(t, pendingWrites)
		})
	}
}

func TestCatalogPublicationObservationChangesDuringProofs(t *testing.T) {
	db := catalogReloadTestDB(t)
	channel := Channel{Key: "observation-local", Models: "liveness", Group: "default", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"liveness":2}`}))
	require.NoError(t, db.Save(&Option{Key: "ModelPrice", Value: `{"liveness":9}`}).Error)
	updatePricingLock.Lock()
	err := PublishCatalogSyncRevision(context.Background(), catalogRuntime.revision)
	updatePricingLock.Unlock()
	require.ErrorIs(t, err, ErrCatalogWriterBusy)
	stage := runtimeStageForTest(t, db)
	proofs := 0
	var lastRaw string
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("observation-during-proofs", func(tx *gorm.DB) {
		if _, routing := tx.Statement.Dest.(*[]*Channel); tx.Statement.Table != "channels" || !routing {
			return
		}
		proofs++
		lastRaw = fmt.Sprintf(`{"upstream_model_update_last_check_time":%d,"upstream_model_update_last_detected_models":["new"],"upstream_model_update_last_removed_models":["old"]}`, proofs)
		// Same real SQL transaction, immediately before each final capture.
		tx.AddError(tx.Session(&gorm.Session{NewDB: true}).Model(&Channel{}).Where("id = ?", channel.Id).Update("settings", lastRaw).Error)
	}))
	err = TryWithCatalogWriteBarrier(context.Background(), func() error { return publishCatalogRuntimeGuarded(context.Background(), stage) })
	require.NoError(t, db.Callback().Query().Remove("observation-during-proofs"))
	require.NoError(t, err)
	assert.Equal(t, 3, proofs, "exercise recheck, preinstall and real ACK captures")
	var actual Channel
	require.NoError(t, db.First(&actual, channel.Id).Error)
	assert.Equal(t, lastRaw, actual.OtherSettings)
	assert.Empty(t, stage.input.channels[0].OtherSettings, "routing normalization must not mutate the captured original")
	require.NoError(t, WithCatalogPricingRead(context.Background(), "liveness", func() error {
		assert.Equal(t, float64(9), ratio_setting.GetModelPriceMap()["liveness"])
		return nil
	}))
}

func TestCatalogPublicationRoutingSettingsRemainBound(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			initializeOrdinaryCatalogTest(t, db)
			testCatalogPublicationRoutingSettingsRemainBound(t, db)
		})
	}
}

func testCatalogPublicationRoutingSettingsRemainBound(t *testing.T, db *gorm.DB) {
	t.Helper()
	channel := Channel{Key: "routing-local", Models: "liveness", Group: "default", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	for _, raw := range []string{
		`{"claude_beta_query":true}`,
		`{"upstream_model_update_ignored_models":["ignored"]}`,
		`{"unknown":{"precise":9007199254740992}}`,
		`{"unknown":{"precise":9007199254740993}}`,
	} {
		var before CatalogSyncState
		require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
		require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Update("settings", raw).Error)
		require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
		var after CatalogSyncState
		require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
		assert.NotEqual(t, before.RuntimeFingerprint, after.RuntimeFingerprint, "real settings remain bound: %s", raw)
		assert.Equal(t, raw, catalogRuntime.input.channels[0].OtherSettings)
	}
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	// Canonically equivalent unknown high-precision data keeps the epoch.
	require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Update("settings", ` {"unknown": {"precise":9007199254740993.0},"upstream_model_update_last_check_time":5} `).Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.Equal(t, before, after)
	require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Update("models", "changed-model").Error)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.NotEqual(t, before.RuntimeFingerprint, after.RuntimeFingerprint)
	for _, raw := range []string{`{"broken":`, `[]`, `true`, `{"unknown":1,"unknown":2}`, `{"unknown":{"same":1,"same":2}}`} {
		require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Update("settings", raw).Error)
		require.Error(t, RecoverCatalogSyncRuntime(context.Background()), "invalid or ambiguous settings cannot ACK: %s", raw)
		var pending CatalogSyncState
		require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
		assert.Equal(t, "committed_pending_publish", pending.PublicationState)
		assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "liveness", func() error { t.Error("invalid settings admitted pricing"); return nil }), ErrCatalogPublicationPending)
	}
	for _, fixture := range []struct {
		name   string
		status int
		raw    string
	}{
		{"enabled-malformed", common.ChannelStatusEnabled, `{"advanced_custom":`},
		{"disabled-malformed", common.ChannelStatusManuallyDisabled, `{"advanced_custom":`},
		{"enabled-typed-invalid", common.ChannelStatusEnabled, `{"advanced_custom":false}`},
		{"disabled-typed-invalid", common.ChannelStatusManuallyDisabled, `{"advanced_custom":false}`},
		{"disabled-ambiguous", common.ChannelStatusManuallyDisabled, `{"unknown":1,"unknown":2}`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
				"type": constant.ChannelTypeAdvancedCustom, "status": fixture.status, "settings": "{}",
			}).Error)
			require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
			var ready CatalogSyncState
			require.NoError(t, db.First(&ready, CatalogSyncStateID).Error)
			require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Update("settings", fixture.raw).Error)
			var before Channel
			require.NoError(t, db.First(&before, channel.Id).Error)
			writes := 0
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("routing-advanced-no-repair", func(tx *gorm.DB) {
				if tx.Statement.Table == "channels" {
					writes++
				}
			}))
			err := RecoverCatalogSyncRuntime(context.Background())
			require.NoError(t, db.Callback().Update().Remove("routing-advanced-no-repair"))
			require.ErrorContains(t, err, "decode advanced custom channel settings")
			assert.NotContains(t, err.Error(), before.Key, "errors must not disclose channel credentials")
			assert.NotContains(t, err.Error(), fixture.raw, "errors must not disclose the raw settings document")
			var after Channel
			require.NoError(t, db.First(&after, channel.Id).Error)
			assert.Equal(t, before, after, "failed recovery must preserve the complete stored channel row")
			assert.Zero(t, writes, "lifecycle validation must never repair channels")
			var pending CatalogSyncState
			require.NoError(t, db.First(&pending, CatalogSyncStateID).Error)
			assert.Equal(t, "committed_pending_publish", pending.PublicationState)
			assert.Equal(t, ready.RuntimeRevision, pending.RuntimeRevision)
			assert.Equal(t, ready.RuntimeFingerprint, pending.RuntimeFingerprint, "invalid advanced settings cannot ACK a new fingerprint")
			assert.ErrorIs(t, WithCatalogPricingRead(context.Background(), "liveness", func() error { t.Error("invalid advanced settings admitted pricing"); return nil }), ErrCatalogPublicationPending)
		})
	}
}
