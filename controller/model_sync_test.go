package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TestSyncUpstreamPreviewUsesModelsDevAndSanitizesFetchErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api.json" {
			t.Fatalf("path = %q, want /api.json", request.URL.Path)
		}
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("upstream-secret-must-not-leak"))
	}))
	defer server.Close()
	t.Setenv("MODEL_METADATA_SYNC_URL", server.URL+"/api.json")

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/models/sync_upstream/preview", nil)
	SyncUpstreamPreview(context)

	var response struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Success || response.Code != "model_metadata_sync_failed" {
		t.Fatalf("response = %+v", response)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "upstream-secret") || strings.Contains(body, "basellm") {
		t.Fatalf("response leaked upstream details: %s", body)
	}
}

func setupModelSyncModeControllerTest(t *testing.T, description string) (*model.Model, *httptest.Server) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	channel := model.Channel{Name: "sync", Key: "test", Status: common.ChannelStatusEnabled, Models: "qwen-next-auto", Group: "default"}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := db.Create(&model.Ability{Group: "default", Model: "qwen-next-auto", ChannelId: channel.Id, Enabled: true}).Error; err != nil {
		t.Fatalf("create ability: %v", err)
	}
	entry := &model.Model{ModelName: "qwen-next-auto", Description: description, Status: 1, SyncOfficial: 1}
	if err := entry.Insert(); err != nil {
		t.Fatalf("insert model: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"alibaba-cn":{"id":"alibaba-cn","name":"Alibaba Cloud (China)","models":{
				"qwen-next-auto":{"id":"qwen-next-auto","description":"models.dev description","reasoning":true,
				"modalities":{"input":["text"],"output":["text"]},"limit":{"context":1000000,"output":65536}}
			}}
		}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("MODEL_METADATA_SYNC_URL", server.URL)
	return entry, server
}

func TestSyncUpstreamModelsDefaultsToLocalFirstMode(t *testing.T) {
	entry, _ := setupModelSyncModeControllerTest(t, "admin description")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", strings.NewReader(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")

	SyncUpstreamModels(context)

	var stored model.Model
	if err := model.DB.First(&stored, entry.Id).Error; err != nil {
		t.Fatalf("read model: %v", err)
	}
	if stored.Description != "admin description" || stored.ContextLength != 1_000_000 {
		t.Fatalf("local-first mode did not preserve/fill metadata: %+v", stored)
	}
}

func TestSyncUpstreamModelsAcceptsModelsDevFirstMode(t *testing.T) {
	entry, _ := setupModelSyncModeControllerTest(t, "admin description")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", strings.NewReader(`{"sync_mode":"models_dev_first"}`))
	context.Request.Header.Set("Content-Type", "application/json")

	SyncUpstreamModels(context)

	var stored model.Model
	if err := model.DB.First(&stored, entry.Id).Error; err != nil {
		t.Fatalf("read model: %v", err)
	}
	if stored.Description != "models.dev description" {
		t.Fatalf("models.dev-first mode did not overwrite metadata: %+v", stored)
	}
}

func TestSyncUpstreamModelsRejectsUnknownModeBeforeFetch(t *testing.T) {
	setupModelListControllerTestDB(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()
	t.Setenv("MODEL_METADATA_SYNC_URL", server.URL)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", strings.NewReader(`{"sync_mode":"unknown"}`))
	context.Request.Header.Set("Content-Type", "application/json")
	SyncUpstreamModels(context)

	var response struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Success || response.Code != "invalid_sync_mode" {
		t.Fatalf("response = %+v body=%s", response, recorder.Body.String())
	}
	if requests.Load() != 0 {
		t.Fatalf("unknown mode contacted models.dev %d time(s)", requests.Load())
	}
}
