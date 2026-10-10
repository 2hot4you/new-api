package model

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCatalogPricingIdentitiesPending(t *testing.T) {
	db, actor := catalogStartupInstanceFixture(t)
	require.NoError(t, RecoverCatalogSyncRuntime(context.Background()))
	source := catalogSyncTestSource(t)
	require.NoError(t, appendManagedEntry(&source, catalogmanifest.KindModel, "upstream-money", catalogmanifest.ModelValue{ModelName: "upstream-money", BillingCurrency: "CNY"}))
	var err error
	source.Digest, err = catalogmanifest.SnapshotDigest(source)
	require.NoError(t, err)
	plan, err := CreateCatalogSyncPlan(context.Background(), source, actor, time.Now())
	require.NoError(t, err)
	pendingReads := 0
	injected := errors.New("publication recheck unavailable")
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("pricing-identities-pending", func(tx *gorm.DB) {
		if state, ok := tx.Statement.Dest.(*CatalogSyncState); ok && state.PendingOperationID == "identities-operation" {
			pendingReads++
			if pendingReads == 2 {
				tx.AddError(injected)
			}
		}
	}))
	_, err = ApplyCatalogSyncPlan(context.Background(), plan.ID, plan.Digest, "identities-operation", actor)
	require.NoError(t, db.Callback().Query().Remove("pricing-identities-pending"))
	require.ErrorIs(t, err, injected)
	called := 0
	capture := func() error { called++; return nil }
	require.ErrorIs(t, WithCatalogPricingReads(context.Background(), []string{"client-name", "upstream-money"}, capture), ErrCatalogPublicationPending)
	assert.Zero(t, called)
	require.NoError(t, WithCatalogPricingReads(context.Background(), []string{"client-name", "other-money", "client-name"}, capture))
	assert.Equal(t, 1, called)
	require.ErrorIs(t, WithCatalogPricingRead(context.Background(), "upstream-money", capture), ErrCatalogPublicationPending)
	require.NoError(t, WithCatalogPricingRead(context.Background(), "client-name", capture))
	assert.Equal(t, 2, called)
	for _, names := range [][]string{nil, {}, {""}, {"client-name", " "}} {
		require.ErrorIs(t, WithCatalogPricingReads(context.Background(), names, capture), ErrCatalogPublicationPending)
	}
	assert.Equal(t, 2, called)
	require.NoError(t, WithCatalogPricingReads(context.Background(), []string{"client-name"}, func() error {
		assert.ErrorIs(t, TryWithCatalogWriteBarrier(context.Background(), func() error { t.Error("writer entered during capture"); return nil }), ErrCatalogWriterBusy)
		return nil
	}))
	require.NoError(t, TryWithCatalogWriteBarrier(context.Background(), func() error { return nil }))
}
