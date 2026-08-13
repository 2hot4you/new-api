package controller

import (
	"bytes"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateVendorMetaRefreshesPricingVendorIntroduction(t *testing.T) {
	previousDB := model.DB
	previousMemoryCache := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "vendor-pricing.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCache
		model.InvalidatePricingCache()
	})

	vendor := model.Vendor{Name: "Vendor", Description: "old introduction", Icon: "OpenAI", Status: 1}
	require.NoError(t, vendor.Insert())
	entry := model.Model{ModelName: "vendor-refresh-model", VendorID: vendor.Id, Status: 1, SyncOfficial: 1}
	require.NoError(t, entry.Insert())
	channel := model.Channel{Name: "channel", Key: "test", Status: common.ChannelStatusEnabled, Models: entry.ModelName, Group: "default"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: entry.ModelName, ChannelId: channel.Id, Enabled: true}).Error)

	model.InvalidatePricingCache()
	require.Len(t, model.GetPricing(), 1)
	require.Equal(t, "old introduction", model.GetVendors()[0].Description)

	body := []byte(`{"id":1,"name":"Vendor","description":"new introduction","icon":"OpenAI","status":1}`)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("PUT", "/api/vendor/", bytes.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")

	UpdateVendorMeta(context)

	require.Equal(t, 200, recorder.Code)
	require.Equal(t, "new introduction", model.GetVendors()[0].Description)
}
