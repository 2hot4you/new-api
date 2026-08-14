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

func TestResolveModelsDevModelSupportsTrustedExactSuffixAndCaseInsensitiveIDs(t *testing.T) {
	catalog, err := decodeModelsDevCatalog(strings.NewReader(`{
		"alibaba-cn":{"id":"alibaba-cn","models":{"qwen3.8-max":{"id":"qwen3.8-max","description":"qwen"}}},
		"moonshotai-cn":{"id":"moonshotai-cn","models":{"moonshotai/kimi-k3":{"id":"moonshotai/kimi-k3","description":"kimi"}}},
		"minimax-cn":{"id":"minimax-cn","models":{"MiniMax-M3":{"id":"MiniMax-M3","description":"minimax"}}}
	}`), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	tests := []struct {
		name, localModel, vendor, wantDescription string
	}{
		{"exact", "qwen3.8-max", "阿里巴巴", "qwen"},
		{"provider suffix", "kimi-k3", "Moonshot", "kimi"},
		{"case insensitive", "minimax-m3", "MiniMax", "minimax"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := resolveModelsDevModel(catalog, tt.localModel, tt.vendor)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if resolved.Entry.Description != tt.wantDescription {
				t.Fatalf("description = %q, want %q", resolved.Entry.Description, tt.wantDescription)
			}
		})
	}
}

func TestResolveModelsDevModelRejectsAmbiguousSuffixes(t *testing.T) {
	catalog, err := decodeModelsDevCatalog(strings.NewReader(`{
		"moonshotai-cn":{"id":"moonshotai-cn","models":{
			"moonshotai/kimi-k3":{"id":"moonshotai/kimi-k3"},
			"legacy/kimi-k3":{"id":"legacy/kimi-k3"}
		}}
	}`), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if _, err := resolveModelsDevModel(catalog, "kimi-k3", "Moonshot"); err == nil {
		t.Fatal("ambiguous suffixes must be rejected")
	}
}

func TestParseModelMetadataSyncMode(t *testing.T) {
	tests := []struct {
		input   string
		want    ModelMetadataSyncMode
		wantErr bool
	}{
		{"", ModelMetadataSyncModeLocalFirst, false},
		{"local_first", ModelMetadataSyncModeLocalFirst, false},
		{"models_dev_first", ModelMetadataSyncModeModelsDevFirst, false},
		{"unknown", "", true},
	}
	for _, tt := range tests {
		got, err := ParseModelMetadataSyncMode(tt.input)
		if (err != nil) != tt.wantErr {
			t.Fatalf("ParseModelMetadataSyncMode(%q) error = %v, wantErr=%v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Fatalf("ParseModelMetadataSyncMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPreviewEnabledModelMetadataFromModelsDevDoesNotWrite(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "qwen-next-auto")
	entry := Model{ModelName: "qwen-next-auto", Status: 1, SyncOfficial: 1}
	if err := entry.Insert(); err != nil {
		t.Fatalf("insert model: %v", err)
	}
	catalog, err := decodeModelsDevCatalog(strings.NewReader(modelsDevFixture("qwen-next-auto", "alibaba-cn")), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	preview, err := PreviewEnabledModelMetadataFromModelsDev(context.Background(), catalog, "https://models.dev/api.json", "2026-08-14")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.WillUpdate) != 1 || preview.WillUpdate[0].ModelName != "qwen-next-auto" {
		t.Fatalf("preview = %+v", preview)
	}
	var stored Model
	if err := DB.First(&stored, entry.Id).Error; err != nil {
		t.Fatalf("read model: %v", err)
	}
	if stored.ContextLength != 0 || stored.Description != "" {
		t.Fatalf("preview wrote model: %+v", stored)
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

func TestSyncEnabledModelMetadataFromModelsDevWithModeOverwritesOnlyModelsDevMetadata(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "qwen-next-auto")

	customVendor := Vendor{Name: "Custom Vendor", Description: "admin vendor", Icon: "Custom", Status: 1}
	if err := customVendor.Insert(); err != nil {
		t.Fatalf("insert custom vendor: %v", err)
	}
	entry := Model{
		ModelName:          "qwen-next-auto",
		Description:        "admin description",
		Icon:               "AdminIcon",
		Tags:               "admin-tag",
		VendorID:           customVendor.Id,
		Endpoints:          "admin-endpoint",
		Status:             0,
		SyncOfficial:       1,
		NameRule:           NameRulePrefix,
		ContextLength:      123,
		MaxOutputTokens:    456,
		KnowledgeCutoff:    "admin-knowledge",
		ReleaseDate:        "2025-01-01",
		InputModalities:    []string{"audio"},
		OutputModalities:   []string{"audio"},
		Capabilities:       []string{"audio_generation"},
		MetadataSource:     "admin",
		MetadataVerifiedAt: "2025-01-02",
	}
	if err := entry.Insert(); err != nil {
		t.Fatalf("insert model: %v", err)
	}
	catalog, err := decodeModelsDevCatalog(strings.NewReader(modelsDevFixture("qwen-next-auto", "alibaba-cn")), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	summary, err := SyncEnabledModelMetadataFromModelsDevWithMode(
		context.Background(), catalog, "2026-08-14", ModelMetadataSyncModeModelsDevFirst,
	)
	if err != nil {
		t.Fatalf("sync metadata: %v", err)
	}
	if summary.UpdatedModels != 1 {
		t.Fatalf("summary = %+v", summary)
	}

	var stored Model
	if err := DB.First(&stored, entry.Id).Error; err != nil {
		t.Fatalf("read model: %v", err)
	}
	if stored.Description != "external description" || stored.Icon != getDefaultVendorIcon("阿里巴巴") {
		t.Fatalf("models.dev description/icon not applied: %+v", stored)
	}
	if stored.VendorID == customVendor.Id || stored.ContextLength != 1_000_000 || stored.MaxOutputTokens != 65_536 {
		t.Fatalf("models.dev vendor/limits not applied: %+v", stored)
	}
	if stored.KnowledgeCutoff != "2025-06" || stored.ReleaseDate != "2026-08-01" {
		t.Fatalf("models.dev dates not applied: %+v", stored)
	}
	if got := strings.Join(stored.InputModalities, ","); got != "text,image,file" {
		t.Fatalf("input modalities = %q", got)
	}
	if got := strings.Join(stored.OutputModalities, ","); got != "text" {
		t.Fatalf("output modalities = %q", got)
	}
	if got := strings.Join(stored.Capabilities, ","); got != "reasoning,tools,structured_output,vision" {
		t.Fatalf("capabilities = %q", got)
	}
	if stored.MetadataSource != "models.dev" || stored.MetadataVerifiedAt != "2026-08-14" {
		t.Fatalf("metadata provenance not applied: %+v", stored)
	}
	if stored.Status != 0 || stored.SyncOfficial != 1 || stored.NameRule != NameRulePrefix ||
		stored.Tags != "admin-tag" || stored.Endpoints != "admin-endpoint" {
		t.Fatalf("local routing/configuration was overwritten: %+v", stored)
	}
	var ability Ability
	if err := DB.Where("model = ?", "qwen-next-auto").First(&ability).Error; err != nil {
		t.Fatalf("read ability: %v", err)
	}
	if ability.Group != "default" || !ability.Enabled {
		t.Fatalf("channel ability was changed: %+v", ability)
	}
}

func TestSyncEnabledModelMetadataFromModelsDevWithModeReplacesKnownLegacyCatalogDescription(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "kimi-k3")
	entry := Model{
		ModelName:    "kimi-k3",
		Description:  "面向通用对话、内容生成与复杂问答的文本模型；支持通过 OpenAI 兼容 Chat Completions API 调用。",
		Status:       1,
		SyncOfficial: 1,
	}
	if err := entry.Insert(); err != nil {
		t.Fatalf("insert model: %v", err)
	}
	catalog, err := decodeModelsDevCatalog(strings.NewReader(`{
		"moonshotai-cn":{"id":"moonshotai-cn","models":{"moonshotai/kimi-k3":{
			"id":"moonshotai/kimi-k3",
			"description":"Multimodal Kimi model with 1M context and toggleable max-effort thinking for long-horizon agent work",
			"modalities":{"input":["text","image"],"output":["text"]},
			"limit":{"context":1048576,"output":131072}
		}}}
	}`), 1<<20)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	if _, err := SyncEnabledModelMetadataFromModelsDevWithMode(
		context.Background(), catalog, "2026-08-14", ModelMetadataSyncModeModelsDevFirst,
	); err != nil {
		t.Fatalf("sync metadata: %v", err)
	}
	var stored Model
	if err := DB.First(&stored, entry.Id).Error; err != nil {
		t.Fatalf("read model: %v", err)
	}
	want := "Multimodal Kimi model with 1M context and toggleable max-effort thinking for long-horizon agent work"
	if stored.Description != want {
		t.Fatalf("description = %q, want %q", stored.Description, want)
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
