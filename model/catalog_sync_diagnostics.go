package model

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type catalogSyncTraceKey struct{}
type catalogSyncTrace struct {
	owner           uint64
	source          string
	started         time.Time
	last            time.Time
	attempt         int
	localRevision   int64
	poolWaitCount   int64
	poolWaitTime    time.Duration
	pendingEntered  time.Time
	pendingObserved time.Time
	targetRevision  int64
	dbRevision      int64
}

var catalogPendingObservation struct {
	sync.Mutex
	state    CatalogSyncState
	db       *gorm.DB
	entered  time.Time
	observed time.Time
}

// Process-local diagnostic provenance only, never a readiness proof. A known
// transition is timed only after its real SQL COMMIT; inherited pending rows
// have an explicitly unknown start and a separate first-observed clock.
func observeCatalogPending(ctx context.Context, input *catalogRuntimeInput, transitioned bool) {
	if input == nil || input.state.PublicationState != "committed_pending_publish" {
		return
	}
	now := time.Now()
	catalogPendingObservation.Lock()
	if catalogPendingObservation.db != DB || catalogPendingObservation.state != input.state || transitioned {
		catalogPendingObservation.db, catalogPendingObservation.state = DB, input.state
		catalogPendingObservation.observed, catalogPendingObservation.entered = now, time.Time{}
	}
	if transitioned {
		catalogPendingObservation.entered = now
	}
	if trace, ok := ctx.Value(catalogSyncTraceKey{}).(*catalogSyncTrace); ok {
		trace.pendingEntered, trace.pendingObserved = catalogPendingObservation.entered, catalogPendingObservation.observed
	}
	catalogPendingObservation.Unlock()
}

var catalogSyncOwner atomic.Uint64

type catalogLockError struct{ name string }

func (e *catalogLockError) Error() string {
	return "catalog phase=acquire lock=" + e.name + ": " + ErrCatalogWriterBusy.Error()
}
func (e *catalogLockError) Unwrap() error { return ErrCatalogWriterBusy }

func catalogSyncContext(ctx context.Context, source string) context.Context {
	if _, exists := ctx.Value(catalogSyncTraceKey{}).(*catalogSyncTrace); exists {
		return ctx
	}
	now := time.Now()
	trace := &catalogSyncTrace{owner: catalogSyncOwner.Add(1), source: source, started: now, last: now, attempt: 1, localRevision: -1, targetRevision: -1, dbRevision: -1}
	if DB != nil {
		if pool, err := DB.DB(); err == nil {
			stats := pool.Stats()
			trace.poolWaitCount, trace.poolWaitTime = stats.WaitCount, stats.WaitDuration
		}
	}
	return context.WithValue(ctx, catalogSyncTraceKey{}, trace)
}

func recordCatalogSyncPhase(ctx context.Context, phase string, input *catalogRuntimeInput, err error) {
	trace, ok := ctx.Value(catalogSyncTraceKey{}).(*catalogSyncTrace)
	if !ok {
		return
	}
	result := "ok"
	lockName := "none"
	var lockErr *catalogLockError
	if errors.As(err, &lockErr) {
		lockName = lockErr.name
	}
	switch {
	case errors.Is(err, context.Canceled):
		result = "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		result = "deadline"
	case errors.Is(err, ErrCatalogWriterBusy):
		result = "busy"
	case errors.Is(err, ErrCatalogSyncPlanStale):
		result = "stale"
	case errors.Is(err, ErrCatalogCommitUncertain):
		result = "commit_unknown"
	case err != nil:
		result = "failed"
	}
	target, durable := int64(-1), int64(-1)
	if input != nil {
		target, durable = input.state.Revision, input.state.RuntimeRevision
		trace.targetRevision, trace.dbRevision = target, durable
	} else {
		target, durable = trace.targetRevision, trace.dbRevision
	}
	now := time.Now()
	pendingAge := "unknown"
	if !trace.pendingEntered.IsZero() {
		pendingAge = fmt.Sprint(now.Sub(trace.pendingEntered).Milliseconds())
	}
	observedAge := "unknown"
	if !trace.pendingObserved.IsZero() {
		observedAge = fmt.Sprint(now.Sub(trace.pendingObserved).Milliseconds())
	}
	var waits int64
	var waitMS int64
	if DB != nil {
		if pool, err := DB.DB(); err == nil {
			stats := pool.Stats()
			waits, waitMS = stats.WaitCount-trace.poolWaitCount, (stats.WaitDuration - trace.poolWaitTime).Milliseconds()
		}
	}
	common.SysLog(fmt.Sprintf("catalog sync source=%s instance=%q owner=%d phase=%s lock=%s attempt=%d local_revision=%d target_revision=%d db_runtime_revision=%d pending_age_ms=%s first_observed_age_ms=%s phase_ms=%d elapsed_ms=%d pool_wait_count_delta=%d pool_wait_ms_delta=%d result=%s", trace.source, common.GetNodeIdentity().Name, trace.owner, phase, lockName, trace.attempt, trace.localRevision, target, durable, pendingAge, observedAge, now.Sub(trace.last).Milliseconds(), now.Sub(trace.started).Milliseconds(), waits, waitMS, result))
	trace.last = now
}

// Cache lock failures retain their sentinel and identify the actual lock, not
// an inferred goroutine owner. No configuration, channel keys or SQL is logged.
func acquireCatalogInstallLocks(ctx context.Context) (func(), error) {
	locks := []struct {
		name string
		lock interface {
			TryLock() bool
			Unlock()
		}
	}{
		{"pricing", &updatePricingLock},
		{"endpoints", &modelSupportEndpointsLock},
		{"channels", &channelSyncLock},
		{"aliases", &taskAliasRebuildMu},
		{"groups", &modelEnableGroupsLock},
	}
	acquired := 0
	release := func() {
		for acquired > 0 {
			acquired--
			locks[acquired].lock.Unlock()
		}
	}
	for i, lock := range locks {
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		if !lock.lock.TryLock() {
			release()
			return nil, &catalogLockError{name: lock.name}
		}
		acquired = i + 1
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, errors.Join(err, ErrCatalogWriterBusy)
	}
	return release, nil
}
