package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useGPTImage2PreviewCOSConfig(t *testing.T) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{
		"COSEnabled":      "true",
		"COSBucket":       "molii-assets-1250000000",
		"COSRegion":       "ap-guangzhou",
		"COSSecretID":     "AKIDTEST",
		"COSSecretKey":    "SECRETTEST",
		"COSPathPrefix":   "users",
		"COSCustomDomain": "https://assets.example.com",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func gptImage2PreviewRequest(userID, role, targetUserID int, requestID string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "user_id", Value: strconv.Itoa(targetUserID)}, {Key: "request_id", Value: requestID}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/gpt-image-2-preview", nil)
	ctx.Set("id", userID)
	ctx.Set("role", role)
	GetGPTImage2Preview(ctx)
	return recorder
}

func TestGetGPTImage2PreviewAllowsOwnerAndAdminButHidesCrossUser(t *testing.T) {
	useGrokImagePreviewControllerRedis(t)
	useGPTImage2PreviewCOSConfig(t)
	requestID := "req-image-2"
	require.NoError(t, service.RegisterGPTImage2Preview(42, requestID, []service.GPTImage2PreviewObject{{
		ObjectKey: "users/gpt-image-2-results/42/2026/08/result.png",
		MIMEType:  "image/png",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}}))

	owner := gptImage2PreviewRequest(42, common.RoleCommonUser, 42, requestID)
	require.Equal(t, http.StatusOK, owner.Code)
	assert.Contains(t, owner.Body.String(), "assets.example.com")

	admin := gptImage2PreviewRequest(7, common.RoleAdminUser, 42, requestID)
	require.Equal(t, http.StatusOK, admin.Code)

	other := gptImage2PreviewRequest(8, common.RoleCommonUser, 42, requestID)
	assert.Equal(t, http.StatusNotFound, other.Code)
}

func TestGetGPTImage2PreviewReturnsNotFoundForMissingRequest(t *testing.T) {
	useGrokImagePreviewControllerRedis(t)
	useGPTImage2PreviewCOSConfig(t)
	assert.Equal(t, http.StatusNotFound, gptImage2PreviewRequest(42, common.RoleCommonUser, 42, "missing").Code)
}
