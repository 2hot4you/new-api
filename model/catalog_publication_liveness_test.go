package model

import (
	"context"
	"errors"
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
