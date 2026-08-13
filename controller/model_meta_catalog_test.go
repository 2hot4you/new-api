package controller

import (
	"bytes"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelMetaCatalogControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "model-meta-catalog.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.Vendor{}, &model.Channel{}, &model.Ability{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.InvalidatePricingCache()
	})
	return db
}

func modelMetaRequestContext(method string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, "/api/models/", bytes.NewBufferString(body))
	context.Request.Header.Set("Content-Type", "application/json")
	return context, recorder
}

func TestCreateModelMetaPersistsNormalizedCatalogFields(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	context, recorder := modelMetaRequestContext("POST", `{
      "model_name":"catalog-controller-model","status":1,"sync_official":1,
      "context_length":1000000,"max_output_tokens":65536,
      "release_date":"2026-02-16","metadata_verified_at":"2026-08-13",
      "input_modalities":[" text ","image","text"],"output_modalities":["text"],
      "capabilities":[" tools ","streaming","tools"],"metadata_source":" models.dev "
    }`)

	CreateModelMeta(context)

	assert.Equal(t, 200, recorder.Code)
	var entry model.Model
	require.NoError(t, db.Where("model_name = ?", "catalog-controller-model").First(&entry).Error)
	assert.Equal(t, []string{"text", "image"}, entry.InputModalities)
	assert.Equal(t, []string{"tools", "streaming"}, entry.Capabilities)
	assert.Equal(t, "models.dev", entry.MetadataSource)
}

func TestCreateModelMetaRejectsInvalidCatalogFields(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	context, recorder := modelMetaRequestContext("POST", `{
      "model_name":"invalid-catalog-model","status":1,"context_length":-1,
      "release_date":"2026/02/16"
    }`)

	CreateModelMeta(context)

	var count int64
	require.NoError(t, db.Model(&model.Model{}).Count(&count).Error)
	assert.Zero(t, count)
	assert.True(t, strings.Contains(recorder.Body.String(), "context_length") || strings.Contains(recorder.Body.String(), "release_date"))
}

func TestUpdateModelMetaRejectsInvalidCatalogFieldsWithoutChangingRow(t *testing.T) {
	db := setupModelMetaCatalogControllerTest(t)
	entry := model.Model{ModelName: "existing-catalog-model", Description: "original", Status: 1, SyncOfficial: 1, ContextLength: 128_000}
	require.NoError(t, entry.Insert())
	context, recorder := modelMetaRequestContext("PUT", `{
      "id":1,"model_name":"existing-catalog-model","description":"changed","status":1,
      "context_length":-1,"release_date":"invalid"
    }`)

	UpdateModelMeta(context)

	var stored model.Model
	require.NoError(t, db.First(&stored, entry.Id).Error)
	assert.Equal(t, "original", stored.Description)
	assert.Equal(t, 128_000, stored.ContextLength)
	assert.Contains(t, recorder.Body.String(), "context_length")
}
