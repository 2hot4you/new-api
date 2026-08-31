package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGrokImagePreviewRouteTest(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	previousRateLimit := common.DisableAllRateLimit
	previousMainDBType, previousLogDBType := common.MainDatabaseType(), common.LogDatabaseType()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled = true
	common.RDB = client
	common.DisableAllRateLimit = true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		_ = client.Close()
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled, common.RDB = previousEnabled, previousRDB
		common.DisableAllRateLimit = previousRateLimit
		common.SetDatabaseTypes(previousMainDBType, previousLogDBType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func createGrokImagePreviewRouteUser(t *testing.T, username, accessToken string, role int) *model.User {
	t.Helper()
	user := &model.User{
		Username: username, Password: "password", Role: role, Status: common.UserStatusEnabled,
		Group: "default", AccessToken: &accessToken, AuthVersion: 1, AffCode: "aff-" + username,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func assertGrokImagePreviewNoStoreHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	cacheControl := response.Header().Get("Cache-Control")
	assert.Contains(t, cacheControl, "private")
	assert.Contains(t, cacheControl, "no-store")
	assert.Equal(t, "no-cache", response.Header().Get("Pragma"))
	assert.Equal(t, "0", response.Header().Get("Expires"))
}

func TestGrokImagePreviewRouteUsesAuthOwnershipAndNoStoreHeaders(t *testing.T) {
	setupGrokImagePreviewRouteTest(t)
	owner := createGrokImagePreviewRouteUser(t, "preview-owner", "preview-owner-token", common.RoleCommonUser)
	other := createGrokImagePreviewRouteUser(t, "preview-other", "preview-other-token", common.RoleCommonUser)
	admin := createGrokImagePreviewRouteUser(t, "preview-admin", "preview-admin-token", common.RoleAdminUser)
	requestID := "req preview encoded"
	resultURL := "https://imgen.x.ai/route-preview.webp?token=private"
	require.NoError(t, service.RegisterGrokImagePreview(owner.Id, requestID, []string{resultURL}))

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	path := "/api/log/grok-image-preview/" + fmt.Sprintf("%d", owner.Id) + "/" + url.PathEscape(requestID)
	perform := func(accessToken string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if accessToken != "" {
			request.Header.Set("Authorization", "Bearer "+accessToken)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		return response
	}

	unauthenticated := perform("")
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	assertGrokImagePreviewNoStoreHeaders(t, unauthenticated)

	ownerResponse := perform(*owner.AccessToken)
	require.Equal(t, http.StatusOK, ownerResponse.Code)
	assert.Contains(t, ownerResponse.Body.String(), resultURL)
	assertGrokImagePreviewNoStoreHeaders(t, ownerResponse)

	otherResponse := perform(*other.AccessToken)
	assert.Equal(t, http.StatusNotFound, otherResponse.Code)
	assert.NotContains(t, otherResponse.Body.String(), resultURL)
	assertGrokImagePreviewNoStoreHeaders(t, otherResponse)

	adminResponse := perform(*admin.AccessToken)
	require.Equal(t, http.StatusOK, adminResponse.Code)
	assert.Contains(t, adminResponse.Body.String(), resultURL)
	assertGrokImagePreviewNoStoreHeaders(t, adminResponse)
}

func TestGPTImage2PreviewRouteRequiresAuthentication(t *testing.T) {
	setupGrokImagePreviewRouteTest(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)
	request := httptest.NewRequest(http.MethodGet, "/api/log/gpt-image-2-preview/42/request-id", nil)
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assertGrokImagePreviewNoStoreHeaders(t, response)
}
