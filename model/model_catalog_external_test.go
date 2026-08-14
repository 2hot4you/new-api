package model

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func modelsDevFixture(modelName string, providers ...string) string {
	entries := make([]string, 0, len(providers))
	for index, provider := range providers {
		contextLength := 42 + index
		if provider == "alibaba-cn" {
			contextLength = 1_000_000
		}
		entries = append(entries, fmt.Sprintf(`%q:{"id":%q,"name":%q,"models":{%q:{"id":%q,"name":"Qwen Next","description":"external description","reasoning":true,"tool_call":true,"structured_output":true,"knowledge":"2025-06","release_date":"2026-08-01","modalities":{"input":["text","image","pdf","unknown"],"output":["text"]},"limit":{"context":%d,"output":65536},"cost":{"input":999,"output":999}}}}`, provider, provider, provider, modelName, modelName, contextLength))
	}
	return "{" + strings.Join(entries, ",") + "}"
}

func TestModelsDevProfilePrefersOfficialVendorAndMapsSupportedFacts(t *testing.T) {
	catalog, err := decodeModelsDevCatalog(strings.NewReader(modelsDevFixture("qwen-next-auto", "random-reseller", "alibaba", "alibaba-cn")), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	profile, ok := buildModelsDevCatalogProfile(catalog, "qwen-next-auto", "阿里巴巴", "2026-08-14")
	if !ok {
		t.Fatal("expected trusted Alibaba catalog match")
	}
	if profile.ContextLength != 1_000_000 || profile.MaxOutputTokens != 65_536 {
		t.Fatalf("official provider limits not selected: %+v", profile)
	}
	if profile.Description != "external description" || profile.KnowledgeCutoff != "2025-06" || profile.ReleaseDate != "2026-08-01" {
		t.Fatalf("public facts not mapped: %+v", profile)
	}
	if got := strings.Join(profile.InputModalities, ","); got != "text,image,file" {
		t.Fatalf("input modalities = %q, want text,image,file", got)
	}
	if got := strings.Join(profile.OutputModalities, ","); got != "text" {
		t.Fatalf("output modalities = %q, want text", got)
	}
	if got := strings.Join(profile.Capabilities, ","); got != "reasoning,tools,structured_output,vision" {
		t.Fatalf("capabilities = %q", got)
	}
	if profile.MetadataSource != "models.dev" || profile.MetadataVerifiedAt != "2026-08-14" {
		t.Fatalf("metadata provenance missing: %+v", profile)
	}
}

func TestModelsDevProfileRejectsUntrustedReseller(t *testing.T) {
	catalog, err := decodeModelsDevCatalog(strings.NewReader(modelsDevFixture("qwen-next-auto", "random-reseller")), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if _, ok := buildModelsDevCatalogProfile(catalog, "qwen-next-auto", "阿里巴巴", "2026-08-14"); ok {
		t.Fatal("untrusted reseller must not become the metadata source")
	}
}

func TestSyncEnabledModelMetadataFromModelsDevOnlyFillsBlanks(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "qwen-next-auto")

	entry := Model{
		ModelName: "qwen-next-auto", Description: "管理员中文简介", Status: 1,
		SyncOfficial: 1, ContextLength: 0, Capabilities: []string{"system_prompt"},
	}
	if err := entry.Insert(); err != nil {
		t.Fatalf("insert model: %v", err)
	}
	catalog, err := decodeModelsDevCatalog(strings.NewReader(modelsDevFixture("qwen-next-auto", "alibaba-cn")), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	first, err := SyncEnabledModelMetadataFromModelsDev(context.Background(), catalog, "2026-08-14")
	if err != nil {
		t.Fatalf("sync metadata: %v", err)
	}
	if first.UpdatedModels != 1 {
		t.Fatalf("first summary = %+v", first)
	}
	var stored Model
	if err := DB.First(&stored, entry.Id).Error; err != nil {
		t.Fatalf("read model: %v", err)
	}
	if stored.Description != "管理员中文简介" || stored.ContextLength != 1_000_000 {
		t.Fatalf("admin value was overwritten or blank not filled: %+v", stored)
	}
	if got := strings.Join(stored.Capabilities, ","); got != "system_prompt" {
		t.Fatalf("existing capabilities overwritten: %q", got)
	}
	if stored.MetadataSource != "models.dev" || stored.MetadataVerifiedAt != "2026-08-14" {
		t.Fatalf("source fields not filled: %+v", stored)
	}

	second, err := SyncEnabledModelMetadataFromModelsDev(context.Background(), catalog, "2026-08-15")
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if second.UpdatedModels != 0 {
		t.Fatalf("second sync must be idempotent: %+v", second)
	}
}

func TestSyncEnabledModelMetadataFromModelsDevRespectsOptOutAndTombstones(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "qwen-opt-out")
	addEnabledCatalogAbility(t, "qwen-deleted")

	optOut := Model{ModelName: "qwen-opt-out", Status: 1, SyncOfficial: 0}
	if err := optOut.Insert(); err != nil {
		t.Fatalf("insert opt-out: %v", err)
	}
	deleted := Model{ModelName: "qwen-deleted", Status: 1, SyncOfficial: 1}
	if err := deleted.Insert(); err != nil {
		t.Fatalf("insert deleted: %v", err)
	}
	if err := deleted.Delete(); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	fixture := `{"alibaba-cn":{"id":"alibaba-cn","name":"Alibaba (China)","models":{` +
		`"qwen-opt-out":{"id":"qwen-opt-out","description":"external","modalities":{"input":["text"],"output":["text"]},"limit":{"context":1000000,"output":65536}},` +
		`"qwen-deleted":{"id":"qwen-deleted","description":"external","modalities":{"input":["text"],"output":["text"]},"limit":{"context":1000000,"output":65536}}}}}`
	catalog, err := decodeModelsDevCatalog(strings.NewReader(fixture), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	summary, err := SyncEnabledModelMetadataFromModelsDev(context.Background(), catalog, "2026-08-14")
	if err != nil {
		t.Fatalf("sync metadata: %v", err)
	}
	if summary.UpdatedModels != 0 {
		t.Fatalf("opt-out/deleted models must not update: %+v", summary)
	}
	var stored Model
	if err := DB.First(&stored, optOut.Id).Error; err != nil {
		t.Fatalf("read opt-out: %v", err)
	}
	if stored.ContextLength != 0 || stored.MetadataSource != "" || stored.VendorID != 0 {
		t.Fatalf("opt-out model was updated: %+v", stored)
	}
}

func TestFetchModelsDevCatalogValidatesStatusAndBodyLimit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(modelsDevFixture("qwen-next-auto", "alibaba-cn")))
		}))
		defer server.Close()

		catalog, err := fetchModelsDevCatalog(context.Background(), server.Client(), server.URL, 1<<20)
		if err != nil || len(catalog) != 1 {
			t.Fatalf("fetch catalog: len=%d err=%v", len(catalog), err)
		}
	})

	t.Run("status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "secret upstream details", http.StatusBadGateway)
		}))
		defer server.Close()
		if _, err := fetchModelsDevCatalog(context.Background(), server.Client(), server.URL, 1<<20); err == nil || strings.Contains(err.Error(), "secret upstream details") {
			t.Fatalf("expected sanitized status error, got %v", err)
		}
	})

	t.Run("body limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", 65)))
		}))
		defer server.Close()
		if _, err := fetchModelsDevCatalog(context.Background(), server.Client(), server.URL, 64); err == nil {
			t.Fatal("oversized catalog must fail")
		}
	})
}

func TestModelMetadataSyncConfigDefaultsAndBounds(t *testing.T) {
	t.Setenv("MODEL_METADATA_AUTO_SYNC_ENABLED", "")
	t.Setenv("MODEL_METADATA_SYNC_URL", "")
	t.Setenv("MODEL_METADATA_SYNC_INTERVAL_HOURS", "0")
	t.Setenv("MODEL_METADATA_SYNC_TIMEOUT_SECONDS", "-1")
	t.Setenv("MODEL_METADATA_SYNC_MAX_MB", "0")

	config := loadModelMetadataSyncConfig()
	if !config.Enabled || config.URL != "https://models.dev/api.json" {
		t.Fatalf("unexpected defaults: %+v", config)
	}
	if config.Interval <= 0 || config.Timeout <= 0 || config.MaxBodyBytes <= 0 {
		t.Fatalf("invalid bounded durations/body: %+v", config)
	}

	overflowing := strconv.FormatInt(math.MaxInt64, 10)
	t.Setenv("MODEL_METADATA_SYNC_INTERVAL_HOURS", overflowing)
	t.Setenv("MODEL_METADATA_SYNC_TIMEOUT_SECONDS", overflowing)
	t.Setenv("MODEL_METADATA_SYNC_MAX_MB", overflowing)
	config = loadModelMetadataSyncConfig()
	if config.Interval != defaultModelMetadataSyncIntervalHours*time.Hour ||
		config.Timeout != defaultModelMetadataSyncTimeoutSeconds*time.Second ||
		config.MaxBodyBytes != int64(defaultModelMetadataSyncMaxMB)*1024*1024 {
		t.Fatalf("oversized configuration must fall back safely: %+v", config)
	}
}
