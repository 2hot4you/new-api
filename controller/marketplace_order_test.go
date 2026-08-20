package controller_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/router"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type marketplaceOrderResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupMarketplaceOrderRouter(t *testing.T) (*gorm.DB, *gin.Engine, string, string) {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.Vendor{}, &model.User{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	adminToken := "marketplace-order-admin-token"
	ordinaryToken := "marketplace-order-ordinary-token"
	require.NoError(t, db.Create(&model.User{
		Username:    "marketplace-order-admin",
		AffCode:     "marketplace-order-admin-aff",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AccessToken: &adminToken,
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Username:    "marketplace-order-ordinary",
		AffCode:     "marketplace-order-ordinary-aff",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AccessToken: &ordinaryToken,
	}).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router.SetApiRouter(engine)
	return db, engine, adminToken, ordinaryToken
}

func marketplaceOrderRequest(t *testing.T, engine *gin.Engine, method, target, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func decodeMarketplaceOrderResponse(t *testing.T, recorder *httptest.ResponseRecorder) marketplaceOrderResponse {
	t.Helper()
	var response marketplaceOrderResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func seedMarketplaceOrderFixtures(t *testing.T, db *gorm.DB) ([]int, []int) {
	t.Helper()
	models := []*model.Model{
		{ModelName: "model-one", DisplayOrder: 1},
		{ModelName: "model-two", DisplayOrder: 2},
		{ModelName: "model-three", DisplayOrder: 3},
	}
	for _, entry := range models {
		require.NoError(t, db.Create(entry).Error)
	}
	vendors := []*model.Vendor{
		{Name: "vendor-one", DisplayOrder: 1},
		{Name: "vendor-two", DisplayOrder: 2},
		{Name: "vendor-three", DisplayOrder: 3},
	}
	for _, entry := range vendors {
		require.NoError(t, db.Create(entry).Error)
	}

	return []int{models[0].Id, models[1].Id, models[2].Id}, []int{vendors[0].Id, vendors[1].Id, vendors[2].Id}
}

func TestMarketplaceOrderRequiresAdministrator(t *testing.T) {
	_, engine, _, ordinaryToken := setupMarketplaceOrderRouter(t)

	routes := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "get model order", method: http.MethodGet, target: "/api/models/order"},
		{name: "save model order", method: http.MethodPut, target: "/api/models/order", body: `{}`},
		{name: "get vendor order", method: http.MethodGet, target: "/api/vendors/order"},
		{name: "save vendor order", method: http.MethodPut, target: "/api/vendors/order", body: `{}`},
	}
	credentials := []struct {
		name       string
		token      string
		statusCode int
	}{
		{name: "unauthenticated", statusCode: http.StatusUnauthorized},
		{name: "ordinary user", token: ordinaryToken, statusCode: http.StatusForbidden},
	}

	for _, route := range routes {
		for _, credential := range credentials {
			t.Run(route.name+"/"+credential.name, func(t *testing.T) {
				recorder := marketplaceOrderRequest(t, engine, route.method, route.target, credential.token, route.body)
				assert.Equal(t, credential.statusCode, recorder.Code)
			})
		}
	}
}

func TestMarketplaceOrderReadFailureUsesLoadMessage(t *testing.T) {
	db, engine, adminToken, _ := setupMarketplaceOrderRouter(t)
	require.NoError(t, db.Migrator().DropTable(&model.Model{}))

	recorder := marketplaceOrderRequest(t, engine, http.MethodGet, "/api/models/order", adminToken, "")
	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeMarketplaceOrderResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "Unable to load marketplace order", response.Message)
}

func TestMarketplaceOrderListsAndSavesModelOrder(t *testing.T) {
	db, engine, adminToken, _ := setupMarketplaceOrderRouter(t)
	modelIDs, _ := seedMarketplaceOrderFixtures(t, db)

	listRecorder := marketplaceOrderRequest(t, engine, http.MethodGet, "/api/models/order", adminToken, "")
	require.Equal(t, http.StatusOK, listRecorder.Code)
	listResponse := decodeMarketplaceOrderResponse(t, listRecorder)
	require.True(t, listResponse.Success)
	var listed []model.Model
	require.NoError(t, json.Unmarshal(listResponse.Data, &listed))
	require.Equal(t, modelIDs, []int{listed[0].Id, listed[1].Id, listed[2].Id})

	orderedIDs := []int{modelIDs[2], modelIDs[0], modelIDs[1]}
	saveRecorder := marketplaceOrderRequest(t, engine, http.MethodPut, "/api/models/order", adminToken,
		fmt.Sprintf(`{"ordered_ids":[%d,%d,%d]}`, orderedIDs[0], orderedIDs[1], orderedIDs[2]))
	require.Equal(t, http.StatusOK, saveRecorder.Code)
	assert.True(t, decodeMarketplaceOrderResponse(t, saveRecorder).Success)

	listRecorder = marketplaceOrderRequest(t, engine, http.MethodGet, "/api/models/order", adminToken, "")
	listResponse = decodeMarketplaceOrderResponse(t, listRecorder)
	require.True(t, listResponse.Success)
	require.NoError(t, json.Unmarshal(listResponse.Data, &listed))
	assert.Equal(t, orderedIDs, []int{listed[0].Id, listed[1].Id, listed[2].Id})
}

func TestMarketplaceOrderListsAndSavesVendorOrder(t *testing.T) {
	db, engine, adminToken, _ := setupMarketplaceOrderRouter(t)
	_, vendorIDs := seedMarketplaceOrderFixtures(t, db)
	orderedIDs := []int{vendorIDs[1], vendorIDs[2], vendorIDs[0]}

	saveRecorder := marketplaceOrderRequest(t, engine, http.MethodPut, "/api/vendors/order", adminToken,
		fmt.Sprintf(`{"ordered_ids":[%d,%d,%d]}`, orderedIDs[0], orderedIDs[1], orderedIDs[2]))
	require.Equal(t, http.StatusOK, saveRecorder.Code)
	assert.True(t, decodeMarketplaceOrderResponse(t, saveRecorder).Success)

	listRecorder := marketplaceOrderRequest(t, engine, http.MethodGet, "/api/vendors/order", adminToken, "")
	listResponse := decodeMarketplaceOrderResponse(t, listRecorder)
	require.True(t, listResponse.Success)
	var listed []model.Vendor
	require.NoError(t, json.Unmarshal(listResponse.Data, &listed))
	assert.Equal(t, orderedIDs, []int{listed[0].Id, listed[1].Id, listed[2].Id})
}

func TestMarketplaceOrderRejectsInvalidOrConflictingRequestsSafely(t *testing.T) {
	db, engine, adminToken, _ := setupMarketplaceOrderRouter(t)
	modelIDs, _ := seedMarketplaceOrderFixtures(t, db)

	overflowIDs := make([]string, 10001)
	for index := range overflowIDs {
		overflowIDs[index] = strconv.Itoa(index + 1)
	}

	for _, body := range []string{
		fmt.Sprintf(`{"ordered_ids":[%d,%d,%d]}`, modelIDs[0], modelIDs[0], modelIDs[2]),
		fmt.Sprintf(`{"ordered_ids":[%d,%d]}`, modelIDs[0], modelIDs[1]),
		`{"ordered_ids":[]}`,
		`{"ordered_ids":[0]}`,
		`{}`,
		`{"ordered_ids":[` + strings.Join(overflowIDs, ",") + "]}",
	} {
		recorder := marketplaceOrderRequest(t, engine, http.MethodPut, "/api/models/order", adminToken, body)
		require.Equal(t, http.StatusOK, recorder.Code)
		response := decodeMarketplaceOrderResponse(t, recorder)
		assert.False(t, response.Success)
		assert.NotEmpty(t, response.Message)
		assert.NotContains(t, strings.ToLower(response.Message), "sql")
		assert.NotContains(t, strings.ToLower(response.Message), "database")
	}

	conflictRecorder := marketplaceOrderRequest(t, engine, http.MethodPut, "/api/models/order", adminToken,
		fmt.Sprintf(`{"ordered_ids":[%d,%d]}`, modelIDs[0], modelIDs[1]))
	conflictResponse := decodeMarketplaceOrderResponse(t, conflictRecorder)
	assert.False(t, conflictResponse.Success)
	assert.Contains(t, strings.ToLower(conflictResponse.Message), "refresh")
}
