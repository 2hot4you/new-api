package model

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// This existing diagnostic sink supplies a safe scheduler rendezvous only.
// The eleventh recovery has finished, so ten prior channel/reconcile cycles
// have returned. Durable state, real reads and routing are asserted separately.
type catalogReadTrafficCycleWriter struct {
	writer  io.Writer
	cycles  atomic.Int64
	entered chan struct{}
	release chan struct{}
}

func (w *catalogReadTrafficCycleWriter) Write(data []byte) (int, error) {
	line := string(data)
	if strings.Contains(line, "source=periodic ") && strings.Contains(line, "phase=recovery_complete ") && strings.Contains(line, "result=ok") {
		if w.cycles.Add(1) == 11 {
			close(w.entered)
			<-w.release
		}
	}
	return w.writer.Write(data)
}

func TestCatalogPublicationSchedulerUnderReadTraffic(t *testing.T) {
	db := catalogReloadTestDB(t)
	previousMemory, previousAliases := common.MemoryCacheEnabled, taskAliasViewPtr.Load()
	channelSyncLock.Lock()
	previousChannels, previousGroups, previousAdvanced := channelsIDM, group2model2channels, channel2advancedCustomConfig
	channelSyncLock.Unlock()
	previousReconciled := catalogReconciledEpoch.Swap(nil)
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemory
		taskAliasViewPtr.Store(previousAliases)
		catalogReconciledEpoch.Store(previousReconciled)
		channelSyncLock.Lock()
		channelsIDM, group2model2channels, channel2advancedCustomConfig = previousChannels, previousGroups, previousAdvanced
		channelSyncLock.Unlock()
	})
	common.MemoryCacheEnabled = true
	const modelName = "traffic-scheduler-model"
	vendor := Vendor{Name: "traffic-scheduler-vendor", Status: 1}
	require.NoError(t, db.Create(&vendor).Error)
	metadata := completePublishedLLM()
	metadata.ModelName, metadata.BillingCurrency, metadata.VendorID = modelName, "CNY", vendor.Id
	require.NoError(t, db.Create(&metadata).Error)
	channel := Channel{Key: "traffic-fixture", Type: constant.ChannelTypeOpenAI, Models: modelName, Group: "default", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&Ability{ChannelId: channel.Id, Model: modelName, Group: "default", Enabled: true}).Error)
	require.NoError(t, UpdateOptionsBulk(map[string]string{"ModelPrice": `{"traffic-scheduler-model":2}`, "USDExchangeRate": "7"}))
	// Complete real channel installation and metadata reconciliation before
	// measuring unchanged periodic cycles; startup work is not no-op traffic.
	require.NoError(t, refreshPeriodicChannelCache(context.Background()))
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	var before CatalogSyncState
	require.NoError(t, db.First(&before, CatalogSyncStateID).Error)
	require.Equal(t, "ready", before.PublicationState)
	require.Equal(t, before.Revision, before.RuntimeRevision)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var firstTrafficRead atomic.Bool
	readerAtFX, loaderAtFX := make(chan struct{}), make(chan struct{})
	capture := func() error {
		price, configured := ratio_setting.GetModelPrice(modelName, false)
		if !configured || price != 2 {
			return fmt.Errorf("published price = %v (configured %v), want 2", price, configured)
		}
		money, found, err := ResolvePublishedCatalogMoneyContext(modelName)
		if err != nil {
			return err
		}
		if !found || string(money.SourceCurrency) != "CNY" || money.CNYPerUSD != 7 {
			return fmt.Errorf("published money = %s/%v (metadata %v), want CNY/7", money.SourceCurrency, money.CNYPerUSD, found)
		}
		if firstTrafficRead.CompareAndSwap(true, false) {
			// Keep this real catalog-R selection past its final database and
			// typed-price operations. Rendezvous before the actual loader's FX
			// update so incidental pool reuse cannot serialize away the legacy
			// read/write race. The read and loader write remain unordered.
			close(readerAtFX)
			select {
			case <-loaderAtFX:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		// Existing legacy consumers also read typed FX under this same catalog
		// R boundary. A no-op loader sharing R must not write it, even with an
		// identical numeric value: value equality does not synchronize access.
		if legacyFX := operation_setting.USDExchangeRate; legacyFX != 7 {
			return fmt.Errorf("catalog-protected legacy FX = %v, want 7", legacyFX)
		}
		return nil
	}
	require.NoError(t, WithCatalogPricingRead(context.Background(), modelName, capture))
	var pendingUpdates atomic.Int64
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("scheduler-traffic-pending", func(tx *gorm.DB) {
		values, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "catalog_sync_states" && ok && values["publication_state"] == "committed_pending_publish" {
			pendingUpdates.Add(1)
		}
	}))
	defer db.Callback().Update().Remove("scheduler-traffic-pending")
	var loaderOnce sync.Once
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("scheduler-traffic-fx-window", func(tx *gorm.DB) {
		if tx.Statement.Table == "options" && tx.Statement.ConnPool == db.Statement.ConnPool {
			loaderOnce.Do(func() {
				select {
				case <-readerAtFX:
					close(loaderAtFX)
				case <-ctx.Done():
				}
			})
		}
	}))
	defer db.Callback().Query().Remove("scheduler-traffic-fx-window")
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	observer := &catalogReadTrafficCycleWriter{writer: previousWriter, entered: make(chan struct{}), release: make(chan struct{})}
	gin.DefaultWriter = observer
	common.LogWriterMu.Unlock()
	var resume sync.Once
	defer func() {
		resume.Do(func() { close(observer.release) })
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		common.LogWriterMu.Unlock()
	}()
	trafficCtx, stopTraffic := context.WithCancel(context.Background())
	defer stopTraffic()
	var attempts, captures, successful int
	var failures []error
	trafficDone := make(chan struct{})
	firstTrafficRead.Store(true)
	go func() {
		defer close(trafficDone)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-trafficCtx.Done():
				return
			case <-ticker.C:
				attempts++
				// Stopping traffic prevents future requests; it does not cancel
				// an in-flight read and manufacture a shutdown failure.
				err := WithCatalogPricingRead(context.Background(), modelName, func() error {
					captures++
					return capture()
				})
				if err != nil {
					failures = append(failures, err)
				} else {
					successful++
				}
			}
		}
	}()
	schedulerDone := make(chan struct{})
	go func() { SyncCatalogCaches(ctx, 1); close(schedulerDone) }()
	defer func() {
		stopTraffic()
		cancel()
		resume.Do(func() { close(observer.release) })
		<-trafficDone
		<-schedulerDone
	}()
	select {
	case <-observer.entered:
	case <-ctx.Done():
		t.Fatal("scheduler did not complete ten actual channel/reconcile cycles within the bounded deadline")
	}
	// Stop at the existing recovery-return boundary, not during arbitrary SQL.
	stopTraffic()
	<-trafficDone
	cancel()
	resume.Do(func() { close(observer.release) })
	<-schedulerDone
	assert.GreaterOrEqual(t, successful, 50, "real concurrent pricing traffic was not exercised")
	assert.Equal(t, attempts, captures, "every real request must reach a pricing capture")
	assert.Empty(t, failures, "unchanged background synchronization must keep pricing continuously available and coherent")
	assert.Zero(t, pendingUpdates.Load(), "unchanged periodic work must never close the durable gate")
	var after CatalogSyncState
	require.NoError(t, db.First(&after, CatalogSyncStateID).Error)
	assert.Equal(t, before, after, "ten unchanged complete cycles preserve the entire published epoch")
	require.NoError(t, WithCatalogPricingRead(context.Background(), modelName, capture))
	route, err := GetRandomSatisfiedChannel("default", modelName, 0, nil)
	require.NoError(t, err)
	require.NotNil(t, route)
	assert.Equal(t, channel.Id, route.Id)
	cached, err := CacheGetChannel(channel.Id)
	require.NoError(t, err)
	assert.Equal(t, modelName, cached.Models)
	assertRuntimeLocksReleased(t)
	for _, lock := range []interface {
		TryLock() bool
		Unlock()
	}{&channelSyncLock, &taskAliasRebuildMu, &modelEnableGroupsLock, &requestPolicyOptionMutex, &passkeyOptionMutex} {
		if assert.True(t, lock.TryLock(), "background synchronization leaked a lock") {
			lock.Unlock()
		}
	}
	t.Logf("completed periodic cycles >= 10; pricing attempts=%d captures=%d successful=%d failures=%d", attempts, captures, successful, len(failures))
}
