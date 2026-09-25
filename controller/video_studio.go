package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/task/seedanceprotocol"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type videoStudioTokenOption struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	MaskedKey       string   `json:"masked_key"`
	Group           string   `json:"group"`
	UnlimitedQuota  bool     `json:"unlimited_quota"`
	RemainQuota     int      `json:"remain_quota,omitempty"`
	AvailableModels []string `json:"available_models"`
}

type videoStudioOptions struct {
	Tokens       []videoStudioTokenOption                      `json:"tokens"`
	Capabilities map[string]seedanceprotocol.ModelCapabilities `json:"capabilities"`
}

type videoStudioEstimateResponse struct {
	Model             string             `json:"model"`
	UpstreamModel     string             `json:"upstream_model,omitempty"`
	BillingModel      string             `json:"billing_model,omitempty"`
	Quota             int                `json:"quota"`
	EstimatedCost     float64            `json:"estimated_cost"`
	EstimatedTokens   int                `json:"estimated_tokens,omitempty"`
	GroupRatio        float64            `json:"group_ratio"`
	GroupSpecialRatio float64            `json:"group_special_ratio,omitempty"`
	OtherRatios       map[string]float64 `json:"other_ratios,omitempty"`
	Estimated         bool               `json:"estimated"`
}

func GetVideoStudioOptions(c *gin.Context) {
	userID := c.GetInt("id")
	userGroup, err := model.GetUserGroup(userID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to load user group"})
		return
	}
	tokens, err := model.GetAllUserTokens(userID, 0, 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to load API keys"})
		return
	}

	abilities, err := model.GetAllEnableAbilityWithChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to load Seedance routes"})
		return
	}
	availableByGroup := make(map[string]map[string]struct{})
	for _, ability := range abilities {
		if ability.ChannelType != constant.ChannelTypeStarAI && ability.ChannelType != constant.ChannelTypeByteDanceSeedance {
			continue
		}
		if _, ok := availableByGroup[ability.Group]; !ok {
			availableByGroup[ability.Group] = make(map[string]struct{})
		}
		availableByGroup[ability.Group][ability.Model] = struct{}{}
	}

	supported := seedanceprotocol.SupportedModels()
	options := make([]videoStudioTokenOption, 0, len(tokens))
	for _, token := range tokens {
		if !videoStudioTokenCurrentlyUsable(token) {
			continue
		}
		groups := videoStudioTokenGroups(token, userGroup)
		allowedModels := token.GetModelLimitsMap()
		models := make([]string, 0, len(supported))
		for _, modelName := range supported {
			if token.ModelLimitsEnabled && !allowedModels[modelName] {
				continue
			}
			if videoStudioModelAvailable(groups, modelName, availableByGroup) {
				models = append(models, modelName)
			}
		}
		if len(models) == 0 {
			continue
		}
		options = append(options, videoStudioTokenOption{
			ID: token.Id, Name: token.Name, MaskedKey: videoStudioMaskedKey(token.Key), Group: token.Group,
			UnlimitedQuota: token.UnlimitedQuota, RemainQuota: token.RemainQuota, AvailableModels: models,
		})
	}

	capabilities := make(map[string]seedanceprotocol.ModelCapabilities, len(supported))
	for _, modelName := range supported {
		if value, ok := seedanceprotocol.CapabilitiesForModel(modelName); ok {
			capabilities[modelName] = value
		}
	}
	common.ApiSuccess(c, videoStudioOptions{Tokens: options, Capabilities: capabilities})
}

// EstimateVideoStudioTask validates and prices the selected request without
// reserving quota, creating a task, or contacting the upstream provider.
func EstimateVideoStudioTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, relaytypes.RelayFormatTask, nil, nil)
	if err != nil {
		respondTaskSubmissionError(c, service.TaskErrorWrapperLocal(err, "gen_relay_info_failed", http.StatusInternalServerError))
		return
	}
	if taskErr := cacheVideoStudioEstimateAssets(c, relayInfo, service.GetStarAIAssetBinding); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	if action := c.GetString("task_action"); action != "" {
		relayInfo.Action = action
	}
	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	if taskErr := relay.ApplyOriginTaskAffinity(c, relayInfo); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	estimate, taskErr := relay.EstimateTaskSubmit(c, relayInfo)
	if taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	otherRatios := estimate.PriceData.OtherRatios()
	if otherRatios == nil {
		otherRatios = map[string]float64{}
	}
	common.ApiSuccess(c, videoStudioEstimateResponse{
		Model:             estimate.ModelName,
		UpstreamModel:     estimate.UpstreamModelName,
		BillingModel:      estimate.BillingModelName,
		Quota:             estimate.Quota,
		EstimatedCost:     float64(estimate.Quota) / common.QuotaPerUnit,
		EstimatedTokens:   estimate.EstimatedTokens,
		GroupRatio:        estimate.PriceData.GroupRatioInfo.GroupRatio,
		GroupSpecialRatio: estimate.PriceData.GroupRatioInfo.GroupSpecialRatio,
		OtherRatios:       otherRatios,
		Estimated:         true,
	})
}

const videoStudioResolvedAssetCacheKey = "starai_resolved_asset_uris"

// cacheVideoStudioEstimateAssets supplies the StarAI adaptor with locally
// authorized asset IDs so its request validation remains side-effect-free.
// A real submission does not receive this cache and still verifies the asset
// against the selected upstream account before reserving quota.
func cacheVideoStudioEstimateAssets(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	lookup func(string, int) (*service.StarAIAssetBinding, error),
) *dto.TaskError {
	if info == nil || common.GetContextKeyInt(c, constant.ContextKeyChannelType) != constant.ChannelTypeStarAI {
		return nil
	}
	snapshot := service.VideoStudioRequestSnapshotFromContext(c)
	if snapshot == nil || len(snapshot.Media) == 0 {
		return nil
	}

	resolved := make(map[string]string, len(snapshot.Media))
	now := time.Now().Unix()
	for _, media := range snapshot.Media {
		if media.Source != "asset" || strings.TrimSpace(media.AssetID) == "" {
			continue
		}
		binding, err := lookup(media.AssetID, info.UserId)
		if err != nil || binding == nil {
			if err == nil {
				err = service.ErrStarAIAssetNotFound
			}
			return service.TaskErrorWrapperLocal(err, "temporary_asset_not_ready", http.StatusBadRequest)
		}
		providerType := binding.ChannelType
		if providerType == 0 {
			providerType = constant.ChannelTypeStarAI
		}
		if providerType != constant.ChannelTypeStarAI || strings.TrimSpace(binding.UpstreamID) == "" {
			return service.TaskErrorWrapperLocal(service.ErrStarAIAssetVerify, "temporary_asset_verification_failed", http.StatusBadRequest)
		}
		if binding.ExpiresAt > 0 && binding.ExpiresAt <= now {
			return service.TaskErrorWrapperLocal(service.ErrStarAIAssetExpired, "temporary_asset_expired", http.StatusBadRequest)
		}
		switch service.NormalizeTemporaryAssetStatus(providerType, binding.Status) {
		case "ACTIVE", "SUCCESS":
		case "EXPIRED":
			return service.TaskErrorWrapperLocal(service.ErrStarAIAssetExpired, "temporary_asset_expired", http.StatusBadRequest)
		case "FAILED":
			return service.TaskErrorWrapperLocal(service.ErrStarAIAssetVerify, "temporary_asset_verification_failed", http.StatusBadRequest)
		default:
			return service.TaskErrorWrapperLocal(service.ErrStarAIAssetNotReady, "temporary_asset_not_ready", http.StatusBadRequest)
		}
		resolved["asset://"+media.AssetID] = "asset://" + binding.UpstreamID
	}
	if len(resolved) > 0 {
		c.Set(videoStudioResolvedAssetCacheKey, resolved)
	}
	return nil
}

func videoStudioMaskedKey(key string) string {
	key = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(key), "sk-"))
	if len(key) <= 8 {
		return "sk-xxxx"
	}
	return "sk-" + key[:4] + "xxxx" + key[len(key)-4:]
}

func videoStudioTokenCurrentlyUsable(token *model.Token) bool {
	if token == nil || token.Status != common.TokenStatusEnabled {
		return false
	}
	if token.ExpiredTime != -1 && token.ExpiredTime < common.GetTimestamp() {
		return false
	}
	return token.UnlimitedQuota || token.RemainQuota > 0
}

func videoStudioTokenGroups(token *model.Token, userGroup string) []string {
	if token.Group == "auto" {
		groups, err := token.GetAutoGroups()
		if err == nil && len(groups) > 0 {
			return service.FilterUserTokenAutoGroups(userGroup, groups)
		}
		return service.GetUserAutoGroup(userGroup)
	}
	if token.Group != "" {
		if _, allowed := service.GetUserUsableGroups(userGroup)[token.Group]; !allowed {
			return nil
		}
		return []string{token.Group}
	}
	return []string{userGroup}
}

func videoStudioModelAvailable(groups []string, modelName string, availability map[string]map[string]struct{}) bool {
	for _, group := range groups {
		if _, ok := availability[group][modelName]; ok {
			return true
		}
	}
	return false
}

type videoStudioTaskResponse struct {
	Task                *dto.TaskDto                      `json:"task"`
	Request             *model.VideoStudioRequestSnapshot `json:"request,omitempty"`
	UnavailableAssetIDs []string                          `json:"unavailable_asset_ids,omitempty"`
}

func GetVideoStudioTasks(c *gin.Context) {
	page := common.GetPageQuery(c)
	userID := c.GetInt("id")
	tasks, total, err := paginateSeedanceTasks(
		page.GetStartIdx(),
		page.GetPageSize(),
		func(offset, limit int) ([]*model.Task, error) {
			return model.GetUserTasksForPlatformsPage(userID, videoStudioPlatforms(), offset, limit)
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to load video tasks"})
		return
	}
	page.SetTotal(total)
	page.SetItems(buildVideoStudioTaskResponses(userID, tasks))
	common.ApiSuccess(c, page)
}

type videoStudioTaskBatchFetcher func(offset, limit int) ([]*model.Task, error)

func paginateSeedanceTasks(start, pageSize int, fetch videoStudioTaskBatchFetcher) ([]*model.Task, int, error) {
	const batchSize = 200
	if start < 0 {
		start = 0
	}
	if pageSize < 1 {
		pageSize = 20
	}
	selected := make([]*model.Task, 0, pageSize)
	total := 0
	for offset := 0; ; offset += batchSize {
		batch, err := fetch(offset, batchSize)
		if err != nil {
			return nil, 0, err
		}
		for _, task := range filterSeedanceTasks(batch) {
			if total >= start && len(selected) < pageSize {
				selected = append(selected, task)
			}
			total++
		}
		if len(batch) < batchSize {
			break
		}
	}
	return selected, total, nil
}

func GetVideoStudioTask(c *gin.Context) {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), strings.TrimSpace(c.Param("task_id")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to load video task"})
		return
	}
	if !exists || task == nil || len(filterSeedanceTasks([]*model.Task{task})) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "video task not found"})
		return
	}
	common.ApiSuccess(c, buildVideoStudioTaskResponses(c.GetInt("id"), []*model.Task{task})[0])
}

func videoStudioPlatforms() []constant.TaskPlatform {
	return []constant.TaskPlatform{
		constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI)),
		constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeByteDanceSeedance)),
	}
}

func filterSeedanceTasks(tasks []*model.Task) []*model.Task {
	supported := make(map[string]struct{})
	for _, modelName := range seedanceprotocol.SupportedModels() {
		supported[modelName] = struct{}{}
	}
	filtered := make([]*model.Task, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		modelName := task.Properties.OriginModelName
		if modelName == "" && task.PrivateData.BillingContext != nil {
			modelName = task.PrivateData.BillingContext.OriginModelName
		}
		if _, ok := supported[modelName]; ok {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func buildVideoStudioTaskResponses(userID int, tasks []*model.Task) []videoStudioTaskResponse {
	views := tasksToDto(tasks, false, common.RoleCommonUser)
	responses := make([]videoStudioTaskResponse, 0, len(tasks))
	for index, task := range tasks {
		response := videoStudioTaskResponse{Task: views[index], Request: task.PrivateData.VideoStudioRequest}
		if response.Request != nil {
			for _, media := range response.Request.Media {
				if media.Source != "asset" || media.AssetID == "" {
					continue
				}
				binding, err := service.GetStarAIAssetBinding(media.AssetID, userID)
				if err != nil || binding == nil || binding.ExpiresAt > 0 && binding.ExpiresAt <= time.Now().Unix() {
					response.UnavailableAssetIDs = append(response.UnavailableAssetIDs, media.AssetID)
				}
			}
		}
		responses = append(responses, response)
	}
	return responses
}
