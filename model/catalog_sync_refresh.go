package model

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Only detached reconstruction is retryable. In particular this must never
// wrap a caller's business SQL closure or an uncertain COMMIT.
func retryCatalogRefresh(ctx context.Context, source string, refresh func(context.Context) error) (result error) {
	return retryCatalogRefreshUntil(ctx, source, func(ctx context.Context) (bool, error) { return true, refresh(ctx) })
}

// retryable must become false as soon as a business SQL closure has started,
// even if the eventual error is a normally retryable cache-lock sentinel.
func retryCatalogRefreshUntil(ctx context.Context, source string, refresh func(context.Context) (retryable bool, err error)) (result error) {
	ctx = catalogSyncContext(ctx, source)
	defer func() { recordCatalogSyncPhase(ctx, "recovery_complete", nil, result) }()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	delay := 20 * time.Millisecond
	started := time.Now()
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		ctx.Value(catalogSyncTraceKey{}).(*catalogSyncTrace).attempt = attempt
		retryable, err := refresh(ctx)
		// Driver cleanup may replace cancellation with a connection/rollback
		// error. Preserve both the lifecycle cause and native uncertainty; never
		// retry or obscure an unknown COMMIT merely because the budget expired.
		if lifecycle := ctx.Err(); lifecycle != nil {
			return errors.Join(lifecycle, err)
		}
		if err == nil {
			return nil
		}
		if !retryable || (!errors.Is(err, ErrCatalogWriterBusy) && !errors.Is(err, ErrCatalogSyncPlanStale)) {
			return err
		}
		if attempt == 1 || attempt%16 == 0 {
			common.SysLog(fmt.Sprintf("catalog refresh source=%s phase=retry attempt=%d elapsed_ms=%d pending_age=unknown result=contention", source, attempt, time.Since(started).Milliseconds()))
		}
		// refresh has returned: all catalog/cache locks, transactions and
		// registry pins have ended before the cancellable jittered backoff.
		wait := delay/2 + time.Duration(rand.Int64N(int64(delay/2)+1))
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ctx.Err(), err)
		case <-timer.C:
		}
		delay = min(delay*2, 500*time.Millisecond)
	}
}

var catalogRecoveryPermit = make(chan struct{}, 1)

// One local loop replaces the two phase-aligned option/channel loops. Recovery
// precedes ordinary channel reconciliation; a failed recovery is retried on a
// short bounded cadence instead of waiting for another full sync period.
func SyncCatalogCaches(ctx context.Context, frequency int) {
	interval := time.Duration(max(frequency, 1)) * time.Second
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := recoverCatalogSyncRuntime(catalogSyncContext(ctx, "periodic"), loadOptionsFromDatabaseGuarded); err != nil {
			common.SysError("catalog refresh source=periodic phase=recovery pending_age=unknown result=failed")
			timer.Reset(time.Second)
			continue
		}
		if err := refreshPeriodicChannelCache(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			common.SysError("catalog refresh source=periodic phase=channels result=failed")
			// Reconciliation may already have committed its metadata mutation.
			// Wake the next authoritative recovery soon; never replay the failed
			// business closure or wait another full phase-aligned sync period.
			timer.Reset(time.Second)
			continue
		}
		timer.Reset(interval)
	}
}
