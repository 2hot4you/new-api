package controller

import (
	"errors"
	"net/http"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	errModelNameImmutable            = errors.New("model name is immutable")
	errMarketplaceMetadataIncomplete = errors.New("marketplace metadata is incomplete")
)

func writeModelMetaPublicationError(c *gin.Context, code string, message string, missingFields []string) {
	response := gin.H{
		"success": false,
		"code":    code,
		"message": message,
	}
	if missingFields != nil {
		response["missing_fields"] = missingFields
	}
	c.JSON(http.StatusOK, response)
}

// GetAllModelsMeta 获取模型列表（分页）
func GetAllModelsMeta(c *gin.Context) {
	listModelsMeta(c, "", "")
}

// SearchModelsMeta 搜索模型列表
func SearchModelsMeta(c *gin.Context) {
	listModelsMeta(c, c.Query("keyword"), c.Query("vendor"))
}

func listModelsMeta(c *gin.Context, keyword, vendor string) {
	squareState := model.ModelSquareState(c.Query("square_state"))
	switch squareState {
	case "", model.ModelSquareVisible, model.ModelSquareUnavailable, model.ModelSquareHidden, model.ModelSquarePartial:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid model square state"})
		return
	}

	pageInfo := common.GetPageQuery(c)
	if squareState != "" && (pageInfo.GetPage() < 1 || pageInfo.GetPageSize() < 1) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid pagination"})
		return
	}
	offset, limit := pageInfo.GetStartIdx(), pageInfo.GetPageSize()
	if squareState != "" {
		// Visibility depends on live channels and metadata rules. Filter the
		// enriched candidate set before counting and paginating the results.
		offset, limit = 0, -1
	}
	search := model.SearchModels
	if c.Query("include_channel_models") == "true" {
		search = model.SearchModelsWithChannels
	}
	modelsMeta, total, err := search(keyword, vendor, c.Query("status"), c.Query("sync_official"), offset, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := enrichModels(modelsMeta); err != nil {
		common.ApiError(c, err)
		return
	}
	if squareState != "" {
		filtered := make([]*model.Model, 0, len(modelsMeta))
		for _, metadata := range modelsMeta {
			if metadata.SquareState == squareState {
				filtered = append(filtered, metadata)
			}
		}
		total = int64(len(filtered))
		start := len(filtered)
		if pageInfo.GetPage()-1 <= len(filtered)/pageInfo.GetPageSize() {
			start = (pageInfo.GetPage() - 1) * pageInfo.GetPageSize()
		}
		end := min(start+pageInfo.GetPageSize(), len(filtered))
		modelsMeta = filtered[start:end]
	}

	vendorCounts, _ := model.GetVendorModelCounts()
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(modelsMeta)
	common.ApiSuccess(c, gin.H{
		"items":         modelsMeta,
		"total":         total,
		"page":          pageInfo.GetPage(),
		"page_size":     pageInfo.GetPageSize(),
		"vendor_counts": vendorCounts,
	})
}

// GetModelMeta 根据 ID 获取单条模型信息
func GetModelMeta(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var m model.Model
	if err := model.DB.First(&m, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := enrichModels([]*model.Model{&m}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, &m)
}

// CreateModelMeta 新建模型
func CreateModelMeta(c *gin.Context) {
	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		common.ApiError(c, err)
		return
	}
	if m.ModelName == "" {
		common.ApiErrorMsg(c, "模型名称不能为空")
		return
	}
	if err := model.ValidateMetadataValues(model.MetadataValues{Endpoints: m.Endpoints, Status: m.Status, NameRule: m.NameRule}); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := m.NormalizeCatalogMetadata(); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	readiness := m.EvaluateMarketplaceReadiness()
	if m.MarketplaceEnabled && !readiness.Complete {
		writeModelMetaPublicationError(c, "marketplace_metadata_incomplete", errMarketplaceMetadataIncomplete.Error(), readiness.Missing)
		return
	}
	// 名称冲突检查
	if dup, err := model.IsModelNameDuplicated(0, m.ModelName); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "模型名称已存在")
		return
	}

	if err := m.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RefreshPricing()
	enrichModels([]*model.Model{&m})
	m.HasMetadata = m.Id > 0
	common.ApiSuccess(c, &m)
}

// UpdateModelMeta 更新模型
func UpdateModelMeta(c *gin.Context) {
	statusOnly := c.Query("status_only") == "true"

	var m model.Model
	if err := c.ShouldBindJSON(&m); err != nil {
		common.ApiError(c, err)
		return
	}
	if m.Id == 0 {
		common.ApiErrorMsg(c, "缺少模型 ID")
		return
	}

	if err := model.ValidateMetadataValues(model.MetadataValues{Endpoints: m.Endpoints, Status: m.Status, NameRule: m.NameRule}); err != nil {
		common.ApiError(c, err)
		return
	}
	missingFields := []string(nil)
	withdrawn := false
	err := model.WithModelMetadataTransaction(func(tx *gorm.DB) error {
		persisted, err := model.GetModelByIDForUpdate(tx, m.Id)
		if err != nil {
			return err
		}
		if m.ModelName != "" && m.ModelName != persisted.ModelName {
			return errModelNameImmutable
		}

		if statusOnly {
			if err := tx.Model(&model.Model{}).Where("id = ?", persisted.Id).Update("status", m.Status).Error; err != nil {
				return err
			}
			persisted.Status = m.Status
			m = *persisted
			return nil
		}

		if m.ModelName == "" {
			return errModelNameImmutable
		}
		if err := m.NormalizeCatalogMetadata(); err != nil {
			return err
		}

		readiness := m.EvaluateMarketplaceReadiness()
		if !persisted.MarketplaceEnabled && m.MarketplaceEnabled && !readiness.Complete {
			missingFields = readiness.Missing
			return errMarketplaceMetadataIncomplete
		}
		if persisted.MarketplaceEnabled && !readiness.Complete {
			m.MarketplaceEnabled = false
			withdrawn = true
		}
		m.Id = persisted.Id
		return m.UpdateTx(tx)
	})
	if errors.Is(err, errModelNameImmutable) {
		writeModelMetaPublicationError(c, "model_name_immutable", err.Error(), nil)
		return
	}
	if errors.Is(err, errMarketplaceMetadataIncomplete) {
		writeModelMetaPublicationError(c, "marketplace_metadata_incomplete", err.Error(), missingFields)
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RefreshPricing()
	m.MarketplaceWithdrawn = withdrawn
	enrichModels([]*model.Model{&m})
	m.HasMetadata = m.Id > 0
	common.ApiSuccess(c, &m)
}

// DeleteModelMeta 删除模型
func DeleteModelMeta(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	removeFromChannels, err := strconv.ParseBool(c.DefaultQuery("remove_from_channels", "false"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	removePricing, err := strconv.ParseBool(c.DefaultQuery("remove_pricing", "false"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if removePricing && c.GetInt("role") != common.RoleRootUser {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Model pricing is managed by a super administrator."})
		return
	}
	result, err := model.DeleteModelMetadata([]int{id}, removeFromChannels, removePricing)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "model.delete", map[string]any{"model_ids": []int{id}, "remove_from_channels": removeFromChannels, "remove_pricing": removePricing, "updated_channels": result.UpdatedChannels})
	common.ApiSuccess(c, result)
}

func BatchDeleteModelMeta(c *gin.Context) {
	var request struct {
		ModelIDs           []int `json:"model_ids"`
		RemoveFromChannels bool  `json:"remove_from_channels"`
		RemovePricing      bool  `json:"remove_pricing"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.RemovePricing && c.GetInt("role") != common.RoleRootUser {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Model pricing is managed by a super administrator."})
		return
	}
	result, err := model.DeleteModelMetadata(request.ModelIDs, request.RemoveFromChannels, request.RemovePricing)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "model.delete_batch", map[string]any{"model_ids": request.ModelIDs, "remove_from_channels": request.RemoveFromChannels, "remove_pricing": request.RemovePricing, "updated_channels": result.UpdatedChannels})
	common.ApiSuccess(c, result)
}

// enrichModels keeps configured endpoints intact and derives connections from
// enabled routes, including hidden or unpriced models absent from the catalog.
func enrichModels(models []*model.Model) error {
	if len(models) == 0 {
		return nil
	}
	// Refresh runtime endpoint inference even for unpublished catalog entries.
	model.GetPricing()
	configured, err := model.GetConfiguredModelChannels()
	if err != nil {
		return err
	}
	for _, metadata := range models {
		if metadata == nil {
			continue
		}
		metadata.HasMetadata = metadata.Id > 0
		channelIDs := make(map[int]struct{})
		for name, ids := range configured {
			if metadata.MatchesName(name) {
				for _, id := range ids {
					channelIDs[id] = struct{}{}
				}
			}
		}
		metadata.ConfiguredChannelCount = len(channelIDs)
	}
	connections, err := model.GetModelConnections()
	if err != nil {
		return err
	}
	if err := model.FillModelSquareStates(models, configured, connections); err != nil {
		return err
	}
	for _, metadata := range models {
		if metadata == nil {
			continue
		}
		channels := make(map[int]model.BoundChannel)
		groups := make(map[string]bool)
		names := make(map[string]bool)
		endpoints := make(map[string]bool)
		quotas := make(map[int]bool)
		for _, connection := range connections {
			name := connection.Model
			if !metadata.MatchesName(name) {
				continue
			}
			names[name] = true
			groups[connection.Group] = true
			channels[connection.ChannelId] = model.BoundChannel{Name: connection.ChannelName, Type: connection.ChannelType}
			for _, endpoint := range model.GetModelSupportEndpointTypes(name) {
				endpoints[string(endpoint)] = true
			}
			for _, quota := range model.GetModelQuotaTypes(name) {
				quotas[quota] = true
			}
		}
		metadata.BoundChannels = nil
		metadata.EnableGroups = nil
		metadata.SupportedEndpoints = nil
		metadata.QuotaTypes = nil
		metadata.MatchedModels = nil
		for _, channel := range channels {
			metadata.BoundChannels = append(metadata.BoundChannels, channel)
		}
		sort.Slice(metadata.BoundChannels, func(i, j int) bool {
			a, b := metadata.BoundChannels[i], metadata.BoundChannels[j]
			if a.Name == b.Name {
				return a.Type < b.Type
			}
			return a.Name < b.Name
		})
		for group := range groups {
			metadata.EnableGroups = append(metadata.EnableGroups, group)
		}
		for endpoint := range endpoints {
			metadata.SupportedEndpoints = append(metadata.SupportedEndpoints, endpoint)
		}
		for quota := range quotas {
			metadata.QuotaTypes = append(metadata.QuotaTypes, quota)
		}
		sort.Strings(metadata.EnableGroups)
		sort.Strings(metadata.SupportedEndpoints)
		sort.Ints(metadata.QuotaTypes)
		if metadata.NameRule != model.NameRuleExact {
			for name := range names {
				metadata.MatchedModels = append(metadata.MatchedModels, name)
			}
			sort.Strings(metadata.MatchedModels)
			metadata.MatchedCount = len(names)
		}
	}
	runtimeEndpointCounts := make([]int, len(models))
	for i, metadata := range models {
		if metadata != nil {
			runtimeEndpointCounts[i] = len(metadata.SupportedEndpoints)
		}
	}
	enrichMarketplaceStates(models, runtimeEndpointCounts)
	return nil
}

func enrichMarketplaceStates(models []*model.Model, runtimeEndpointCounts []int) {
	vendorIDs := make([]int, 0, len(models))
	seenVendorIDs := make(map[int]struct{}, len(models))
	for _, entry := range models {
		if entry == nil || entry.VendorID <= 0 {
			continue
		}
		if _, seen := seenVendorIDs[entry.VendorID]; seen {
			continue
		}
		seenVendorIDs[entry.VendorID] = struct{}{}
		vendorIDs = append(vendorIDs, entry.VendorID)
	}

	enabledVendors := make(map[int]bool, len(vendorIDs))
	if len(vendorIDs) > 0 {
		var vendors []model.Vendor
		if err := model.DB.Select("id", "status").Where("id IN ?", vendorIDs).Find(&vendors).Error; err == nil {
			for _, vendor := range vendors {
				enabledVendors[vendor.Id] = vendor.Status == 1
			}
		}
	}

	for index, entry := range models {
		if entry == nil {
			continue
		}
		readiness := entry.EvaluateMarketplaceReadiness()
		entry.MarketplaceCategory = string(readiness.Category)
		entry.MarketplaceComplete = readiness.Complete
		entry.MarketplaceMissingFields = readiness.Missing

		pricingConfigured := model.IsModelPricingConfigured(entry.ModelName)
		if !pricingConfigured {
			for _, matchedName := range entry.MatchedModels {
				if model.IsModelPricingConfigured(matchedName) {
					pricingConfigured = true
					break
				}
			}
		}

		entry.MarketplaceBlockers = model.EvaluateMarketplaceBlockers(
			enabledVendors[entry.VendorID],
			pricingConfigured,
			len(entry.EnableGroups),
			runtimeEndpointCounts[index],
		)
		entry.MarketplaceVisible = entry.Status == 1 &&
			entry.MarketplaceEnabled &&
			readiness.Complete &&
			len(entry.MarketplaceBlockers) == 0
	}

}
