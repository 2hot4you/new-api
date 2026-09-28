package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingSessionRefundRestoresInitialAndAdditionalWalletReservations(t *testing.T) {
	truncate(t)

	const (
		userID       = 703
		initialQuota = 100_000
		initialHold  = 20_000
		targetHold   = 60_000
	)
	seedUser(t, userID, initialQuota)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId:          userID,
		IsPlayground:    true,
		ForcePreConsume: true,
		Request:         &dto.ImageRequest{},
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}

	session, apiErr := NewBillingSession(ctx, info, initialHold)
	require.Nil(t, apiErr)
	require.Equal(t, initialQuota-initialHold, walletQuota(t, userID))

	require.NoError(t, session.Reserve(targetHold))
	require.Equal(t, initialQuota-targetHold, walletQuota(t, userID))

	refunded := make(chan struct{}, 1)
	const callback = "billing_session_additive_wallet_refund_observed"
	require.NoError(t, model.DB.Callback().Update().After("gorm:commit_or_rollback_transaction").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" && tx.Error == nil {
			select {
			case refunded <- struct{}{}:
			default:
			}
		}
	}))
	t.Cleanup(func() { require.NoError(t, model.DB.Callback().Update().Remove(callback)) })

	session.Refund(ctx)
	session.Refund(ctx)
	select {
	case <-refunded:
	case <-time.After(5 * time.Second):
		t.Fatal("wallet refund did not finish")
	}
	require.Equal(t, initialQuota, walletQuota(t, userID))

	session.Refund(ctx)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, initialQuota, walletQuota(t, userID))
}

func walletQuota(t *testing.T, userID int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").First(&user, userID).Error)
	return user.Quota
}
