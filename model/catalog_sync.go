package model

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

var catalogBarrier sync.RWMutex

var ErrCatalogWriterBusy = errors.New("catalog writer is busy")

// A cancellable acquisition only: fn runs exactly once after acquisition and
// its error is never replayed. No lock is held during the jittered backoff.
func withCatalogBarrierContext(ctx context.Context, write bool, fn func() error) error {
	delay := 20 * time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var acquired bool
		if write {
			acquired = catalogBarrier.TryLock()
		} else {
			acquired = catalogBarrier.TryRLock()
		}
		if acquired {
			if write {
				defer catalogBarrier.Unlock()
			} else {
				defer catalogBarrier.RUnlock()
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return fn()
		}
		timer := time.NewTimer(delay/2 + time.Duration(rand.Int64N(int64(delay/2)+1)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(ctx.Err(), &catalogLockError{name: "catalog_wait"})
		case <-timer.C:
		}
		delay = min(delay*2, 500*time.Millisecond)
	}
}

// Capture keeps the existing channel capture-through-install exclusion while
// allowing request selections. The reference transaction provides the SQL and
// registry proof; no process-global projection is changed in this callback.
func tryWithCatalogCaptureBarrier(ctx context.Context, capture func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !catalogBarrier.TryRLock() {
		return &catalogLockError{name: "catalog_read"}
	}
	defer catalogBarrier.RUnlock()
	return capture()
}

// TryWithCatalogWriteBarrier is the sensitive entrypoint's bounded acquisition
// boundary. Ordinary writers retain their existing blocking API and lock order.
func TryWithCatalogWriteBarrier(ctx context.Context, write func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !catalogBarrier.TryLock() {
		return &catalogLockError{name: "catalog_write"}
	}
	defer catalogBarrier.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return write()
}

func WithCatalogWriteBarrier(write func() error) error {
	catalogBarrier.Lock()
	defer catalogBarrier.Unlock()
	return write()
}

// Copy immutable billing inputs inside this callback; do not hold the guard
// through billing evaluation, plugin execution, or upstream requests.
func WithCatalogReadBarrier(read func() error) error {
	catalogBarrier.RLock()
	defer catalogBarrier.RUnlock()
	return read()
}

// PostgreSQL READ COMMITTED is insufficient for multi-query source exports.
func WithCatalogReadSnapshot(ctx context.Context, read func(*gorm.DB) error) error {
	return WithCatalogReadBarrier(func() error {
		options := &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}
		if DB.Dialector.Name() == "sqlite" {
			options = nil
		}
		return DB.WithContext(ctx).Transaction(read, options)
	})
}

// Acquire before metadata/option/model rows, while holding the process writer.
// Recovery may inspect pending state here; ordinary writers must reject it.
func LockCatalogMutationTx(tx *gorm.DB) (*CatalogSyncState, error) {
	if err := acquireMarketplaceOrderLock(tx); err != nil {
		return nil, err
	}
	var state CatalogSyncState
	if err := lockForUpdate(tx).First(&state, CatalogSyncStateID).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

// The caller owns the write barrier across this transaction AND publication.
// Compare persisted catalog content, not the coordination anchor counter: a
// no-op reconciliation/reload must not invalidate a ten-minute preview.
func catalogMutationTransaction(db *gorm.DB, write func(*gorm.DB) error) error {
	if err := EnsureCatalogMutationLock(db); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		state, err := LockCatalogMutationTx(tx)
		if err != nil {
			return err
		}
		if state.PublicationState != "ready" {
			return ErrCatalogPublicationPending
		}
		before, err := catalogPersistedDigest(tx)
		if err != nil {
			return err
		}
		if err := write(tx); err != nil {
			return err
		}
		after, err := catalogPersistedDigest(tx)
		if err != nil {
			return err
		}
		if before == after {
			return nil
		}
		return tx.Model(&CatalogSyncState{}).Where("id = ?", CatalogSyncStateID).Updates(map[string]any{"revision": state.Revision + 1, "current_digest": after}).Error
	})
}

func invalidateCatalogCaches() {
	InvalidatePricingCache()
	ratio_setting.InvalidateExposedDataCache()
}
