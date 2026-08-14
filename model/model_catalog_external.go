package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultModelMetadataSyncURL            = "https://models.dev/api.json"
	defaultModelMetadataSyncIntervalHours  = 6
	defaultModelMetadataSyncTimeoutSeconds = 10
	defaultModelMetadataSyncMaxMB          = 32
)

type modelsDevCatalog map[string]modelsDevProvider

type modelsDevProvider struct {
	ID     string                    `json:"id"`
	Name   string                    `json:"name"`
	Models map[string]modelsDevModel `json:"models"`
}

type modelsDevModel struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	Attachment       bool                `json:"attachment"`
	Reasoning        bool                `json:"reasoning"`
	ToolCall         bool                `json:"tool_call"`
	StructuredOutput bool                `json:"structured_output"`
	Knowledge        string              `json:"knowledge"`
	ReleaseDate      string              `json:"release_date"`
	Modalities       modelsDevModalities `json:"modalities"`
	Limit            modelsDevLimit      `json:"limit"`
}

type modelsDevModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type modelsDevLimit struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

type ModelMetadataExternalSyncSummary struct {
	UpdatedModels int `json:"updated_models"`
}

type modelMetadataSyncConfig struct {
	Enabled      bool
	URL          string
	Interval     time.Duration
	Timeout      time.Duration
	MaxBodyBytes int64
}

var modelsDevProviderPreferences = map[string][]string{
	"OpenAI":    {"openai"},
	"Anthropic": {"anthropic"},
	"Google":    {"google"},
	"DeepSeek":  {"deepseek"},
	"阿里巴巴":      {"alibaba-cn", "alibaba"},
	"Moonshot":  {"moonshotai-cn", "moonshotai"},
	"智谱":        {"zhipuai", "zai"},
	"MiniMax":   {"minimax-cn", "minimax"},
	"xAI":       {"xai"},
	"Meta":      {"meta"},
	"Mistral":   {"mistral"},
	"Cohere":    {"cohere"},
}

var (
	modelMetadataSyncNotifications = make(chan struct{}, 1)
	modelMetadataSyncStartOnce     sync.Once
)

func decodeModelsDevCatalog(reader io.Reader, maxBodyBytes int64) (modelsDevCatalog, error) {
	if maxBodyBytes <= 0 {
		return nil, errors.New("model metadata catalog body limit must be positive")
	}
	body, err := io.ReadAll(io.LimitReader(reader, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read model metadata catalog: %w", err)
	}
	if int64(len(body)) > maxBodyBytes {
		return nil, errors.New("model metadata catalog exceeds configured body limit")
	}
	var catalog modelsDevCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("decode model metadata catalog: %w", err)
	}
	if len(catalog) == 0 {
		return nil, errors.New("model metadata catalog is empty")
	}
	return catalog, nil
}

func fetchModelsDevCatalog(ctx context.Context, client *http.Client, url string, maxBodyBytes int64) (modelsDevCatalog, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create model metadata request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch model metadata catalog: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("model metadata catalog returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxBodyBytes {
		return nil, errors.New("model metadata catalog exceeds configured body limit")
	}
	return decodeModelsDevCatalog(response.Body, maxBodyBytes)
}

func normalizeModelsDevModalities(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "pdf" {
			value = "file"
		}
		if _, allowed := allowedModelModalities[value]; !allowed {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func modelsDevCapabilities(entry modelsDevModel, inputs []string) []string {
	capabilities := make([]string, 0, 4)
	if entry.Reasoning {
		capabilities = append(capabilities, "reasoning")
	}
	if entry.ToolCall {
		capabilities = append(capabilities, "tools")
	}
	if entry.StructuredOutput {
		capabilities = append(capabilities, "structured_output")
	}
	for _, input := range inputs {
		if input == "image" {
			capabilities = append(capabilities, "vision")
			break
		}
	}
	return capabilities
}

func buildModelsDevCatalogProfile(catalog modelsDevCatalog, modelName string, vendorName string, verifiedAt string) (CatalogModelProfile, bool) {
	preferences := modelsDevProviderPreferences[vendorName]
	for _, providerID := range preferences {
		provider, exists := catalog[providerID]
		if !exists {
			continue
		}
		entry, exists := provider.Models[modelName]
		if !exists || strings.TrimSpace(entry.ID) != modelName {
			continue
		}
		inputs := normalizeModelsDevModalities(entry.Modalities.Input)
		outputs := normalizeModelsDevModalities(entry.Modalities.Output)
		return CatalogModelProfile{
			ModelName:          modelName,
			VendorName:         vendorName,
			Description:        strings.TrimSpace(entry.Description),
			Icon:               getDefaultVendorIcon(vendorName),
			ContextLength:      entry.Limit.Context,
			MaxOutputTokens:    entry.Limit.Output,
			KnowledgeCutoff:    strings.TrimSpace(entry.Knowledge),
			ReleaseDate:        strings.TrimSpace(entry.ReleaseDate),
			InputModalities:    inputs,
			OutputModalities:   outputs,
			Capabilities:       modelsDevCapabilities(entry, inputs),
			MetadataSource:     "models.dev",
			MetadataVerifiedAt: verifiedAt,
		}, true
	}
	return CatalogModelProfile{}, false
}

func SyncEnabledModelMetadataFromModelsDev(ctx context.Context, catalog modelsDevCatalog, verifiedAt string) (ModelMetadataExternalSyncSummary, error) {
	summary := ModelMetadataExternalSyncSummary{}
	if _, err := ReconcileEnabledModelMetadata(); err != nil {
		return summary, err
	}
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		modelNames, err := enabledCatalogModelNames(tx)
		if err != nil {
			return err
		}
		sort.Strings(modelNames)
		for _, rawModelName := range modelNames {
			modelName := strings.TrimSpace(rawModelName)
			if modelName == "" || modelName == "grok-imagine-video-1.5-preview" {
				continue
			}
			vendorName := InferCatalogVendorName(modelName)
			if localProfile, known := GetCatalogModelProfile(modelName); known && localProfile.VendorName != "" {
				vendorName = localProfile.VendorName
			}
			profile, ok := buildModelsDevCatalogProfile(catalog, modelName, vendorName, verifiedAt)
			if !ok {
				continue
			}

			var entry Model
			err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("model_name = ?", modelName).First(&entry).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if entry.DeletedAt.Valid || entry.SyncOfficial == 0 {
				continue
			}
			updates := fillCatalogModelBlanks(&entry, 0, profile, true)
			if len(updates) == 0 {
				continue
			}
			updates["updated_time"] = common.GetTimestamp()
			if err := tx.Model(&Model{}).Where("id = ?", entry.Id).Updates(updates).Error; err != nil {
				return err
			}
			summary.UpdatedModels++
		}
		return nil
	})
	if err != nil {
		return ModelMetadataExternalSyncSummary{}, err
	}
	if summary.UpdatedModels > 0 {
		InvalidatePricingCache()
	}
	return summary, nil
}

func boundedModelMetadataEnv(name string, fallback int, maximum int) int {
	value := common.GetEnvOrDefault(name, fallback)
	if value <= 0 || value > maximum {
		return fallback
	}
	return value
}

func loadModelMetadataSyncConfig() modelMetadataSyncConfig {
	url := strings.TrimSpace(common.GetEnvOrDefaultString("MODEL_METADATA_SYNC_URL", defaultModelMetadataSyncURL))
	if url == "" {
		url = defaultModelMetadataSyncURL
	}
	intervalHours := boundedModelMetadataEnv("MODEL_METADATA_SYNC_INTERVAL_HOURS", defaultModelMetadataSyncIntervalHours, 24*365)
	timeoutSeconds := boundedModelMetadataEnv("MODEL_METADATA_SYNC_TIMEOUT_SECONDS", defaultModelMetadataSyncTimeoutSeconds, 300)
	maxBodyMB := boundedModelMetadataEnv("MODEL_METADATA_SYNC_MAX_MB", defaultModelMetadataSyncMaxMB, 256)
	return modelMetadataSyncConfig{
		Enabled:      common.GetEnvOrDefaultBool("MODEL_METADATA_AUTO_SYNC_ENABLED", true),
		URL:          url,
		Interval:     time.Duration(intervalHours) * time.Hour,
		Timeout:      time.Duration(timeoutSeconds) * time.Second,
		MaxBodyBytes: int64(maxBodyMB) * 1024 * 1024,
	}
}

// NotifyModelMetadataAutoSync coalesces repeated channel refreshes and never
// waits for external I/O. The actual worker is started only by the application.
func NotifyModelMetadataAutoSync() {
	select {
	case modelMetadataSyncNotifications <- struct{}{}:
	default:
	}
}

func runModelMetadataAutoSync(ctx context.Context, client *http.Client, config modelMetadataSyncConfig) {
	catalog, err := fetchModelsDevCatalog(ctx, client, config.URL, config.MaxBodyBytes)
	if err != nil {
		common.SysLog(fmt.Sprintf("warning: automatic model metadata sync failed: %v; existing metadata remains active", err))
		return
	}
	summary, err := SyncEnabledModelMetadataFromModelsDev(ctx, catalog, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		common.SysLog(fmt.Sprintf("warning: automatic model metadata reconciliation failed: %v; existing metadata remains active", err))
		return
	}
	if summary.UpdatedModels > 0 {
		common.SysLog(fmt.Sprintf("automatic model metadata synced: models_updated=%d source=models.dev", summary.UpdatedModels))
	}
}

// StartModelMetadataAutoSync starts one master-only background worker. Channel
// refreshes trigger immediate work and the ticker provides eventual retry.
func StartModelMetadataAutoSync(ctx context.Context) {
	config := loadModelMetadataSyncConfig()
	if !config.Enabled || !common.IsMasterNode {
		return
	}
	modelMetadataSyncStartOnce.Do(func() {
		client := &http.Client{Timeout: config.Timeout}
		go func() {
			ticker := time.NewTicker(config.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-modelMetadataSyncNotifications:
					runModelMetadataAutoSync(ctx, client, config)
				case <-ticker.C:
					runModelMetadataAutoSync(ctx, client, config)
				}
			}
		}()
	})
}
