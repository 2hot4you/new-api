package model

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
)

type PricingPluginVariant struct {
	PluginKey            string                               `json:"plugin_key"`
	PluginName           string                               `json:"plugin_name"`
	Icon                 string                               `json:"icon,omitempty"`
	BillingExpr          string                               `json:"billing_expr"`
	BillingMode          string                               `json:"billing_mode"`
	BillingUsageSchema   map[string]jsplugin.UsageFieldSchema `json:"billing_usage_schema"`
	BillingUsageExamples []jsplugin.UsageExample              `json:"billing_usage_examples,omitempty"`
}

type Pricing struct {
	BillingPluginVariants  []PricingPluginVariant                 `json:"billing_plugin_variants,omitempty"`
	ModelName              string                                 `json:"model_name"`
	DisplayName            string                                 `json:"display_name,omitempty"`
	Description            string                                 `json:"description,omitempty"`
	DescriptionEN          string                                 `json:"description_en,omitempty"`
	Icon                   string                                 `json:"icon,omitempty"`
	Tags                   string                                 `json:"tags,omitempty"`
	VendorID               int                                    `json:"vendor_id,omitempty"`
	DisplayOrder           int                                    `json:"display_order"`
	QuotaType              int                                    `json:"quota_type"`
	ModelRatio             float64                                `json:"model_ratio"`
	ModelPrice             float64                                `json:"model_price"`
	OwnerBy                string                                 `json:"owner_by"`
	CompletionRatio        float64                                `json:"completion_ratio"`
	CacheRatio             *float64                               `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64                               `json:"create_cache_ratio,omitempty"`
	ImageRatio             *float64                               `json:"image_ratio,omitempty"`
	AudioRatio             *float64                               `json:"audio_ratio,omitempty"`
	AudioCompletionRatio   *float64                               `json:"audio_completion_ratio,omitempty"`
	EnableGroup            []string                               `json:"enable_groups"`
	SupportedEndpointTypes []constant.EndpointType                `json:"supported_endpoint_types"`
	BillingMode            string                                 `json:"billing_mode,omitempty"`
	BillingExpr            string                                 `json:"billing_expr,omitempty"`
	BillingUsageSchema     map[string]jsplugin.UsageFieldSchema   `json:"billing_usage_schema,omitempty"`
	BillingUsageExamples   []jsplugin.UsageExample                `json:"billing_usage_examples,omitempty"`
	PricingVersion         string                                 `json:"pricing_version,omitempty"`
	VideoPricing           *ratio_setting.StarAIVideoPricing      `json:"video_pricing,omitempty"`
	MoliiGrokPricing       *ratio_setting.MoliiGrokCatalogPricing `json:"molii_grok_pricing,omitempty"`
	ContextLength          int                                    `json:"context_length,omitempty"`
	MaxOutputTokens        int                                    `json:"max_output_tokens,omitempty"`
	KnowledgeCutoff        string                                 `json:"knowledge_cutoff,omitempty"`
	ReleaseDate            string                                 `json:"release_date,omitempty"`
	InputModalities        []string                               `json:"input_modalities,omitempty"`
	OutputModalities       []string                               `json:"output_modalities,omitempty"`
	Capabilities           []string                               `json:"capabilities,omitempty"`
	SupportedParameters    []string                               `json:"supported_parameters,omitempty"`
	SupportedResolutions   []string                               `json:"supported_resolutions,omitempty"`
	SupportedAspectRatios  []string                               `json:"supported_aspect_ratios,omitempty"`
	MaxInputImages         int                                    `json:"max_input_images,omitempty"`
	OutputFormats          []string                               `json:"output_formats,omitempty"`
	MinDuration            int                                    `json:"min_duration,omitempty"`
	MaxDuration            int                                    `json:"max_duration,omitempty"`
	ReferenceModalities    []string                               `json:"reference_modalities,omitempty"`
	MetadataUpdatedTime    int64                                  `json:"metadata_updated_time,omitempty"`
	BillingCurrency        string                                 `json:"billing_currency,omitempty"`
}

type PricingVendor struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	DisplayOrder int    `json:"display_order"`
	Description  string `json:"description,omitempty"`
	Icon         string `json:"icon,omitempty"`
}

var (
	pricingMap           []Pricing
	vendorsList          []PricingVendor
	supportedEndpointMap map[string]common.EndpointInfo
	lastGetPricingTime   time.Time
	updatePricingLock    sync.Mutex

	// 缓存映射：模型名 -> 启用分组 / 计费类型
	modelEnableGroups     = make(map[string][]string)
	modelQuotaTypeMap     = make(map[string]int)
	modelEnableGroupsLock = sync.RWMutex{}
)

var (
	modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	modelSupportEndpointsLock = sync.RWMutex{}
)

func GetPricing() []Pricing {
	if time.Since(lastGetPricingTime) > time.Minute*1 || len(pricingMap) == 0 {
		updatePricingLock.Lock()
		defer updatePricingLock.Unlock()
		// Double check after acquiring the lock
		if time.Since(lastGetPricingTime) > time.Minute*1 || len(pricingMap) == 0 {
			modelSupportEndpointsLock.Lock()
			defer modelSupportEndpointsLock.Unlock()
			updatePricing()
		}
	}
	return pricingMap
}

// IsModelPricingConfigured checks explicit pricing settings without accepting
// the operational self-use fallback ratio or mutating any cache.
func IsModelPricingConfigured(modelName string) bool {
	if _, configured := ratio_setting.GetModelPrice(modelName, false); configured {
		return true
	}

	normalizedName := ratio_setting.FormatMatchingModelName(modelName)
	modelRatios := ratio_setting.GetModelRatioCopy()
	if _, configured := modelRatios[normalizedName]; configured {
		return true
	}
	if strings.HasSuffix(normalizedName, ratio_setting.CompactModelSuffix) {
		if _, configured := modelRatios[ratio_setting.CompactWildcardModelKey]; configured {
			return true
		}
	}

	expression, configured := billing_setting.GetBillingExpr(modelName)
	return configured && strings.TrimSpace(expression) != ""
}

func InvalidatePricingCache() {
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()

	pricingMap = nil
	vendorsList = nil
	lastGetPricingTime = time.Time{}
}

// GetVendors 返回当前定价接口使用到的供应商信息
func GetVendors() []PricingVendor {
	if time.Since(lastGetPricingTime) > time.Minute*1 || len(pricingMap) == 0 {
		// 保证先刷新一次
		GetPricing()
	}
	return vendorsList
}

func GetModelSupportEndpointTypes(model string) []constant.EndpointType {
	if model == "" {
		return make([]constant.EndpointType, 0)
	}
	modelSupportEndpointsLock.RLock()
	defer modelSupportEndpointsLock.RUnlock()
	if endpoints, ok := modelSupportEndpointTypes[model]; ok {
		return endpoints
	}
	return make([]constant.EndpointType, 0)
}

func getPricingEndpointTypesForAbility(ability AbilityWithChannel, advancedCustomConfigs map[int]*dto.AdvancedCustomConfig) []constant.EndpointType {
	if ability.ChannelType != constant.ChannelTypeAdvancedCustom {
		return common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
	}
	if config := advancedCustomConfigs[ability.ChannelId]; config != nil {
		return config.SupportedEndpointTypesForModel(ability.Model)
	}
	return common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
}

// loadPricingAdvancedCustomConfigs runs inside updatePricing while
// updatePricingLock is held, and nests channelSyncLock.RLock. This defines the
// global lock order updatePricingLock -> channelSyncLock: any code path holding
// channelSyncLock must release it before touching the pricing cache (see
// InitChannelCache / CacheUpdateChannel), otherwise it deadlocks.
// The returned configs are pointers shared with the channel cache; they are
// replaced wholesale on update and never mutated in place, so reading them after
// RUnlock is safe.
func loadPricingAdvancedCustomConfigs(enableAbilities []AbilityWithChannel) map[int]*dto.AdvancedCustomConfig {
	channelIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, ability := range enableAbilities {
		if ability.ChannelType != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if _, exists := seen[ability.ChannelId]; exists {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	if len(channelIDs) == 0 {
		return nil
	}

	configs := make(map[int]*dto.AdvancedCustomConfig, len(channelIDs))
	if common.MemoryCacheEnabled {
		channelSyncLock.RLock()
		defer channelSyncLock.RUnlock()
		for _, channelID := range channelIDs {
			if config := channel2advancedCustomConfig[channelID]; config != nil {
				configs[channelID] = config
			}
		}
		return configs
	}

	for _, channelID := range channelIDs {
		channel, err := CacheGetChannel(channelID)
		if err != nil {
			common.SysLog(fmt.Sprintf("load advanced custom channel settings error: channel_id=%d, error=%v", channelID, err))
			continue
		}
		if channel.Type != constant.ChannelTypeAdvancedCustom {
			continue
		}
		if config := channel.GetOtherSettings().AdvancedCustom; config != nil {
			configs[channelID] = config
		}
	}
	return configs
}

func appendPricingEndpoint(endpoints []string, endpoint string) []string {
	if endpoint == "" || common.StringsContains(endpoints, endpoint) {
		return endpoints
	}
	return append(endpoints, endpoint)
}

func pricingReleaseDate(releaseDate string) (time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(releaseDate))
	return parsed, err == nil
}

func sortPricingByVendorAndReleaseDate(pricing []Pricing, vendorMap map[int]*Vendor) {
	sort.Slice(pricing, func(i, j int) bool {
		left := pricing[i]
		right := pricing[j]

		if left.VendorID != right.VendorID {
			leftVendor, leftExists := vendorMap[left.VendorID]
			rightVendor, rightExists := vendorMap[right.VendorID]
			if leftExists != rightExists {
				return leftExists
			}
			if leftExists {
				if leftVendor.DisplayOrder != rightVendor.DisplayOrder {
					return leftVendor.DisplayOrder < rightVendor.DisplayOrder
				}
				if leftVendor.Name != rightVendor.Name {
					return leftVendor.Name < rightVendor.Name
				}
			}
			return left.VendorID < right.VendorID
		}

		leftDate, leftDated := pricingReleaseDate(left.ReleaseDate)
		rightDate, rightDated := pricingReleaseDate(right.ReleaseDate)
		if leftDated != rightDated {
			return leftDated
		}
		if leftDated && !leftDate.Equal(rightDate) {
			return leftDate.After(rightDate)
		}
		if left.DisplayOrder != right.DisplayOrder {
			return left.DisplayOrder < right.DisplayOrder
		}
		return left.ModelName < right.ModelName
	})
}

func updatePricing() {
	//modelRatios := common.GetModelRatios()
	enableAbilities, err := GetAllEnableAbilityWithChannels()
	if err != nil {
		common.SysLog(fmt.Sprintf("GetAllEnableAbilityWithChannels error: %v", err))
		return
	}
	// 预加载模型元数据与供应商一次，避免循环查询
	var allMeta []Model
	_ = DB.Find(&allMeta).Error
	names := make([]string, 0, len(enableAbilities))
	for _, ability := range enableAbilities {
		names = append(names, ability.Model)
	}
	metaMap := resolveModelMetadata(allMeta, names)

	// 预加载供应商
	var vendors []Vendor
	_ = DB.Find(&vendors).Error
	vendorMap := make(map[int]*Vendor)
	for i := range vendors {
		vendorMap[vendors[i].Id] = &vendors[i]
	}

	modelGroupsMap := make(map[string]*types.Set[string])

	for _, ability := range enableAbilities {
		groups, ok := modelGroupsMap[ability.Model]
		if !ok {
			groups = types.NewSet[string]()
			modelGroupsMap[ability.Model] = groups
		}
		groups.Add(ability.Group)
	}

	//这里使用切片而不是Set，因为一个模型可能支持多个端点类型，并且第一个端点是优先使用端点
	modelSupportEndpointsStr := make(map[string][]string)
	advancedCustomConfigs := loadPricingAdvancedCustomConfigs(enableAbilities)

	// 先根据已有能力填充原生端点
	for _, ability := range enableAbilities {
		endpoints := modelSupportEndpointsStr[ability.Model]
		channelTypes := getPricingEndpointTypesForAbility(ability, advancedCustomConfigs)
		for _, channelType := range channelTypes {
			if !common.StringsContains(endpoints, string(channelType)) {
				endpoints = append(endpoints, string(channelType))
			}
		}
		modelSupportEndpointsStr[ability.Model] = endpoints
	}

	// 再补充模型自定义端点：若配置有效则追加到已有推断，不再裁剪渠道真实能力
	for modelName, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]any
		if err := common.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			endpoints := modelSupportEndpointsStr[modelName]
			for k, v := range raw {
				switch v.(type) {
				case string, map[string]any:
					endpoints = appendPricingEndpoint(endpoints, k)
				}
			}
			if len(endpoints) > 0 {
				modelSupportEndpointsStr[modelName] = endpoints
			}
		}
	}

	modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	for model, endpoints := range modelSupportEndpointsStr {
		supportedEndpoints := make([]constant.EndpointType, 0)
		for _, endpointStr := range endpoints {
			endpointType := constant.EndpointType(endpointStr)
			supportedEndpoints = append(supportedEndpoints, endpointType)
		}
		modelSupportEndpointTypes[model] = supportedEndpoints
	}

	// 构建全局 supportedEndpointMap（默认 + 自定义覆盖）
	supportedEndpointMap = make(map[string]common.EndpointInfo)
	// 1. 默认端点
	for _, endpoints := range modelSupportEndpointTypes {
		for _, et := range endpoints {
			if info, ok := common.GetDefaultEndpointInfo(et); ok {
				if _, exists := supportedEndpointMap[string(et)]; !exists {
					supportedEndpointMap[string(et)] = info
				}
			}
		}
	}
	// 2. 自定义端点（models 表）覆盖默认
	for _, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]any
		if err := common.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			for k, v := range raw {
				switch val := v.(type) {
				case string:
					supportedEndpointMap[k] = common.EndpointInfo{Path: val, Method: "POST"}
				case map[string]any:
					ep := common.EndpointInfo{Method: "POST"}
					if p, ok := val["path"].(string); ok {
						ep.Path = p
					}
					if m, ok := val["method"].(string); ok {
						ep.Method = strings.ToUpper(m)
					}
					supportedEndpointMap[k] = ep
				default:
					// ignore unsupported types
				}
			}
		}
	}

	pricingMap = make([]Pricing, 0)
	referencedVendorIDs := make(map[int]struct{})
	pluginGeneration := jsplugin.DefaultRegistry.Generation()
	for model, groups := range modelGroupsMap {
		_, hasTaskPlugin := pluginGeneration.GetByModel(model)
		if !hasTaskPlugin {
			_, hasTaskPlugin = ResolveTaskModelAlias(pluginGeneration, model)
		}
		meta, ok := metaMap[model]
		if !ok && !hasTaskPlugin {
			continue
		}
		if ok && !hasTaskPlugin && (meta.Status != 1 || !meta.MarketplaceEnabled || !meta.EvaluateMarketplaceReadiness().Complete) {
			continue
		}
		if meta == nil {
			meta = &Model{ModelName: model}
		}
		vendor, exists := vendorMap[meta.VendorID]
		if !hasTaskPlugin && (!exists || vendor.Status != 1) {
			continue
		}

		modelPrice, hasModelPrice := ratio_setting.GetModelPrice(model, false)
		modelRatio, hasModelRatio, _ := ratio_setting.GetModelRatio(model)
		billingExpr, hasBillingExpr := billing_setting.GetBillingExpr(model)
		hasBillingExpr = hasBillingExpr && strings.TrimSpace(billingExpr) != ""
		if (!IsModelPricingConfigured(model) && !hasTaskPlugin) || len(groups.Items()) == 0 || len(modelSupportEndpointTypes[model]) == 0 {
			continue
		}

		pricing := Pricing{
			ModelName:              model,
			EnableGroup:            groups.Items(),
			SupportedEndpointTypes: modelSupportEndpointTypes[model],
		}
		if exists && vendor.Status == 1 {
			referencedVendorIDs[meta.VendorID] = struct{}{}
		}
		pricing.DisplayName = meta.DisplayName
		pricing.Description = meta.Description
		pricing.DescriptionEN = meta.DescriptionEN
		pricing.Icon = meta.Icon
		pricing.Tags = meta.Tags
		pricing.VendorID = meta.VendorID
		pricing.BillingCurrency = meta.BillingCurrency
		if pricing.BillingCurrency == "" {
			pricing.BillingCurrency = "USD"
		}
		pricing.DisplayOrder = meta.DisplayOrder
		pricing.ContextLength = meta.ContextLength
		pricing.MaxOutputTokens = meta.MaxOutputTokens
		pricing.KnowledgeCutoff = meta.KnowledgeCutoff
		pricing.ReleaseDate = meta.ReleaseDate
		pricing.InputModalities = append([]string(nil), meta.InputModalities...)
		pricing.OutputModalities = append([]string(nil), meta.OutputModalities...)
		pricing.Capabilities = append([]string(nil), meta.Capabilities...)
		pricing.SupportedParameters = append([]string(nil), meta.SupportedParameters...)
		pricing.SupportedResolutions = append([]string(nil), meta.SupportedResolutions...)
		pricing.SupportedAspectRatios = append([]string(nil), meta.SupportedAspectRatios...)
		pricing.MaxInputImages = meta.MaxInputImages
		pricing.OutputFormats = append([]string(nil), meta.OutputFormats...)
		pricing.MinDuration = meta.MinDuration
		pricing.MaxDuration = meta.MaxDuration
		pricing.ReferenceModalities = append([]string(nil), meta.ReferenceModalities...)
		pricing.MetadataUpdatedTime = meta.UpdatedTime
		if hasModelPrice {
			pricing.ModelPrice = modelPrice
			pricing.QuotaType = 1
		} else if hasModelRatio || hasBillingExpr {
			pricing.ModelRatio = modelRatio
			pricing.CompletionRatio = ratio_setting.GetCompletionRatio(model)
			pricing.QuotaType = 0
		}
		if cacheRatio, ok := ratio_setting.GetCacheRatio(model); ok {
			pricing.CacheRatio = &cacheRatio
		}
		if createCacheRatio, ok := ratio_setting.GetCreateCacheRatio(model); ok {
			pricing.CreateCacheRatio = &createCacheRatio
		}
		if imageRatio, ok := ratio_setting.GetImageRatio(model); ok {
			pricing.ImageRatio = &imageRatio
		}
		if ratio_setting.ContainsAudioRatio(model) {
			audioRatio := ratio_setting.GetAudioRatio(model)
			pricing.AudioRatio = &audioRatio
		}
		if ratio_setting.ContainsAudioCompletionRatio(model) {
			audioCompletionRatio := ratio_setting.GetAudioCompletionRatio(model)
			pricing.AudioCompletionRatio = &audioCompletionRatio
		}
		if billingMode := billing_setting.GetBillingMode(model); billingMode == "tiered_expr" {
			if hasBillingExpr {
				pricing.BillingMode = billingMode
				pricing.BillingExpr = billingExpr
			}
		} else if target, resolved := ResolveTaskModelAlias(pluginGeneration, model); resolved && target.Declared != "" {
			if tailMode := billing_setting.GetBillingMode(target.Declared); tailMode == "tiered_expr" {
				if expr, ok := billing_setting.GetBillingExpr(target.Declared); ok && strings.TrimSpace(expr) != "" {
					pricing.BillingMode = tailMode
					pricing.BillingExpr = expr
				}
			}
		}
		usageModel := model
		plugin, ok := pluginGeneration.GetByModel(model)
		if !ok {
			if target, resolved := ResolveTaskModelAlias(pluginGeneration, model); resolved {
				plugin, ok = pluginGeneration.Get(target.PluginKey)
				usageModel = target.Declared
			}
		}
		if ok && plugin != nil {
			usageSchema, usageExamples := plugin.Meta.UsageForModel(usageModel)
			pricing.BillingUsageSchema = jsplugin.CloneUsageSchema(usageSchema)
			pricing.BillingUsageExamples = jsplugin.CloneUsageExamples(usageExamples)
		}
		providers := pluginGeneration.PluginsByModel(model)
		hasProviderOverride := false
		for _, provider := range providers {
			if _, configured := billing_setting.GetPluginBillingExpr(provider.Meta.Key, model); configured {
				hasProviderOverride = true
				break
			}
		}
		if hasProviderOverride || (len(providers) >= 2 && pricing.BillingMode == billing_setting.BillingModeTieredExpr) {
			for _, provider := range providers {
				schema, examples := provider.Meta.UsageForModel(model)
				if schema == nil {
					schema = map[string]jsplugin.UsageFieldSchema{}
				}
				expression, hasExpression := billing_setting.ResolveTaskBillingExpr(provider.Meta.Key, model, "")
				mode := billing_setting.BillingModeRatio
				if hasExpression || billing_setting.GetBillingMode(model) == billing_setting.BillingModeTieredExpr {
					mode = billing_setting.BillingModeTieredExpr
				}
				if mode == billing_setting.BillingModeTieredExpr && !billing_setting.TaskExprCompatible(expression, schema) {
					expression = ""
				}
				pricing.BillingPluginVariants = append(pricing.BillingPluginVariants, PricingPluginVariant{
					PluginKey: provider.Meta.Key, PluginName: provider.Meta.Name, Icon: provider.Meta.Icon,
					BillingExpr: expression, BillingMode: mode,
					BillingUsageSchema: jsplugin.CloneUsageSchema(schema), BillingUsageExamples: jsplugin.CloneUsageExamples(examples),
				})
			}
		}
		if videoPricing, ok := ratio_setting.GetStarAIVideoPricing(model); ok {
			pricing.VideoPricing = videoPricing
		}
		if grokPricing, ok := ratio_setting.GetMoliiGrokCatalogPricing(model); ok {
			pricing.MoliiGrokPricing = grokPricing
		}
		pricingMap = append(pricingMap, pricing)
	}
	sortPricingByVendorAndReleaseDate(pricingMap, vendorMap)

	vendorsList = make([]PricingVendor, 0, len(referencedVendorIDs))
	for vendorID := range referencedVendorIDs {
		vendor := vendorMap[vendorID]
		vendorsList = append(vendorsList, PricingVendor{
			ID: vendor.Id, Name: vendor.Name, DisplayOrder: vendor.DisplayOrder, Description: vendor.Description, Icon: vendor.Icon,
		})
	}
	sort.Slice(vendorsList, func(i, j int) bool {
		if vendorsList[i].DisplayOrder != vendorsList[j].DisplayOrder {
			return vendorsList[i].DisplayOrder < vendorsList[j].DisplayOrder
		}
		return vendorsList[i].Name < vendorsList[j].Name
	})

	// 防止大更新后数据不通用
	if len(pricingMap) > 0 {
		pricingMap[0].PricingVersion = "5a90f2b86c08bd983a9a2e6d66c255f4eaef9c4bc934386d2b6ae84ef0ff1f1f"
	}

	// 刷新缓存映射，供高并发快速查询
	modelEnableGroupsLock.Lock()
	modelEnableGroups = make(map[string][]string)
	modelQuotaTypeMap = make(map[string]int)
	for _, p := range pricingMap {
		modelEnableGroups[p.ModelName] = p.EnableGroup
		modelQuotaTypeMap[p.ModelName] = p.QuotaType
	}
	modelEnableGroupsLock.Unlock()

	lastGetPricingTime = time.Now()
}

// GetSupportedEndpointMap 返回全局端点到路径的映射
func GetSupportedEndpointMap() map[string]common.EndpointInfo {
	return supportedEndpointMap
}
