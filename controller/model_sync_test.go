package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
