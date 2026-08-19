package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useGrokImagePreviewControllerRedis(t *testing.T) {
	t.Helper()
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	server := miniredis.RunT(t)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled = previousEnabled
		common.RDB = previousRDB
	})
}

func previewRequest(t *testing.T, userID, role, targetUserID int, requestID string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{
		{Key: "user_id", Value: strconv.Itoa(targetUserID)},
		{Key: "request_id", Value: requestID},
	}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log/grok-image-preview/"+strconv.Itoa(targetUserID)+"/"+url.PathEscape(requestID), nil)
	ctx.Set("id", userID)
	ctx.Set("role", role)
	GetGrokImagePreview(ctx)
	return recorder
}

func TestGetGrokImagePreviewAllowsOwnerAndAdminButHidesCrossUserResults(t *testing.T) {
	useGrokImagePreviewControllerRedis(t)
	url := "https://imgen.x.ai/result.webp?token=private"
	require.NoError(t, service.RegisterGrokImagePreview(42, "req_owner", []string{url}))

	owner := previewRequest(t, 42, common.RoleCommonUser, 42, "req_owner")
	require.Equal(t, http.StatusOK, owner.Code)
	assert.Contains(t, owner.Body.String(), url)

	admin := previewRequest(t, 7, common.RoleAdminUser, 42, "req_owner")
	require.Equal(t, http.StatusOK, admin.Code)
	assert.Contains(t, admin.Body.String(), url)

	other := previewRequest(t, 8, common.RoleCommonUser, 42, "req_owner")
	require.Equal(t, http.StatusNotFound, other.Code)
	assert.NotContains(t, other.Body.String(), url)
}

func TestGetGrokImagePreviewReturnsNotFoundForMissingOrMalformedRequestID(t *testing.T) {
	useGrokImagePreviewControllerRedis(t)
	for _, requestID := range []string{"", "   ", "missing"} {
		response := previewRequest(t, 42, common.RoleCommonUser, 42, requestID)
		assert.Equal(t, http.StatusNotFound, response.Code)
	}
}
