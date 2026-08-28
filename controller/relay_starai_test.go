package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
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

func TestOrdinaryTaskSubmitPolicyRetainsAutomaticRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeOpenAI))
	taskErr := &taskdto.TaskError{Error: errors.New("upstream failed"), StatusCode: http.StatusBadGateway}

	assert.True(t, shouldRetryTaskRelayForPlatform(ctx, platform, 1, taskErr, 3))
}

func TestStarAITaskSubmissionDoesNotRejectSelectedChannelWhenSeveralAreEnabled(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "standard")
	selected := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "mini")

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeStarAI)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, selected.Id)
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, selected.Key)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	result, taskErr := relay.RelayTaskSubmit(ctx, info)

	require.Nil(t, result)
	require.NotNil(t, taskErr)
	assert.NotEqual(t, "channel_configuration_unavailable", taskErr.Code)
	assert.Equal(t, selected.Id, info.ChannelId)
}
