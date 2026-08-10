package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStarAITaskSubmitPolicyBlocksRetryableStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI))

	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway} {
		taskErr := &taskdto.TaskError{
			Error:      errors.New("upstream failed"),
			StatusCode: status,
		}
		assert.False(t, shouldRetryTaskRelayForPlatform(ctx, platform, 1, taskErr, 3), "status %d must not retry", status)
	}
}

func TestMoliiGrokTaskSubmitPolicyBlocksAllAutomaticRetries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC))
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusGatewayTimeout} {
		taskErr := &taskdto.TaskError{Error: errors.New("sanitized failure"), StatusCode: status}
		assert.False(t, shouldRetryTaskRelayForPlatform(ctx, platform, 1, taskErr, 3), "status %d must not resubmit a paid task", status)
	}
}

func TestStarAITaskSubmissionRejectsNonUniqueOrMismatchedSelectedChannelBeforeBilling(t *testing.T) {
	tests := []struct {
		name          string
		selectedIndex int
		channels      int
	}{
		{name: "no enabled channel", selectedIndex: 0, channels: 0},
		{name: "selected channel differs from unique enabled channel", selectedIndex: 1, channels: 1},
		{name: "multiple enabled channels", selectedIndex: 0, channels: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupSingleStarAIChannelTestDB(t)
			channels := make([]*model.Channel, 0, 2)
			for i := 0; i < 2; i++ {
				status := common.ChannelStatusManuallyDisabled
				if i < tt.channels {
					status = common.ChannelStatusEnabled
				}
				channels = append(channels, seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, status, fmt.Sprintf("channel-%d", i)))
			}

			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
			common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeStarAI)
			common.SetContextKey(ctx, constant.ContextKeyChannelId, channels[tt.selectedIndex].Id)
			info := &relaycommon.RelayInfo{}

			result, taskErr := relay.RelayTaskSubmit(ctx, info)

			require.Nil(t, result)
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusServiceUnavailable, taskErr.StatusCode)
			assert.Equal(t, "channel_configuration_unavailable", taskErr.Code)
			assert.NotContains(t, taskErr.Message, "StarAI")
			assert.NotContains(t, taskErr.Message, "Molii")
			assert.Nil(t, info.Billing)
			assert.Zero(t, info.PriceData.Quota)
		})
	}
}
