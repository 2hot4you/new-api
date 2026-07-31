package controller

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelBillingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestUpdateChannelStarAIBalance(t *testing.T) {
	db := setupChannelBillingTestDB(t)
	originalPrice := operation_setting.Price
	operation_setting.Price = 7.3
	t.Cleanup(func() { operation_setting.Price = originalPrice })

	const key = "starai-secret-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/usage/balance/", r.URL.Path)
		assert.Equal(t, "Bearer "+key, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":true,"data":{"balance_cny":73,"object":"user_balance"},"message":"ok"}`))
	}))
	t.Cleanup(server.Close)

	baseURL := server.URL + "/"
	channel := &model.Channel{Name: "StarAI", Key: key, BaseURL: &baseURL}
	require.NoError(t, db.Create(channel).Error)

	balance, err := updateChannelStarAIBalance(channel)
	require.NoError(t, err)
	assert.InDelta(t, 10, balance, 1e-12)

	var saved model.Channel
	require.NoError(t, db.First(&saved, channel.Id).Error)
	assert.InDelta(t, 10, saved.Balance, 1e-12)
}

func TestUpdateChannelStarAIBalanceRejectsInvalidResponsesWithoutLeakingKey(t *testing.T) {
	setupChannelBillingTestDB(t)
	originalPrice := operation_setting.Price
	t.Cleanup(func() { operation_setting.Price = originalPrice })

	tests := []struct {
		name  string
		price float64
		body  string
	}{
		{name: "unsuccessful code", price: 7.3, body: `{"code":false,"message":"starai-secret-key"}`},
		{name: "negative balance", price: 7.3, body: `{"code":true,"data":{"balance_cny":-1}}`},
		{name: "non-finite balance", price: 7.3, body: `{"code":true,"data":{"balance_cny":1e999}}`},
		{name: "zero price", price: 0, body: `{"code":true,"data":{"balance_cny":10}}`},
		{name: "non-finite price", price: math.Inf(1), body: `{"code":true,"data":{"balance_cny":10}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operation_setting.Price = tt.price
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			baseURL := server.URL
			channel := &model.Channel{Key: "starai-secret-key", BaseURL: &baseURL}
			_, err := updateChannelStarAIBalance(channel)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), channel.Key)
		})
	}
}
