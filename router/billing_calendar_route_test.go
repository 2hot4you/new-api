package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingCalendarRouteRequiresAdminWithoutPricingModuleGate(t *testing.T) {
	setupGrokImagePreviewRouteTest(t)
	user := createGrokImagePreviewRouteUser(t, "calendar-user", "calendar-user-token", common.RoleCommonUser)
	admin := createGrokImagePreviewRouteUser(t, "calendar-admin", "calendar-admin-token", common.RoleAdminUser)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	perform := func(accessToken string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/billing-calendar", nil)
		if accessToken != "" {
			request.Header.Set("Authorization", "Bearer "+accessToken)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		return response
	}

	assert.Equal(t, http.StatusUnauthorized, perform("").Code)
	assert.Equal(t, http.StatusForbidden, perform(*user.AccessToken).Code)
	adminResponse := perform(*admin.AccessToken)
	require.Equal(t, http.StatusOK, adminResponse.Code)
	assert.Contains(t, adminResponse.Body.String(), `"success":true`)
}
