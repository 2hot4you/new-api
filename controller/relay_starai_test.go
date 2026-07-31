package controller

import (
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
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
