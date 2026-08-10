package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func GetAllTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	// 解析其他查询参数
	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelID:      c.Query("channel_id"),
	}

	items := model.TaskGetAllTasks(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	total := model.TaskCountAllTasks(queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasksToDto(items, true))
	common.ApiSuccess(c, pageInfo)
}

func GetUserTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	userId := c.GetInt("id")

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}

	items := model.TaskGetAllUserTask(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	total := model.TaskCountAllUserTask(userId, queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasksToDto(items, false))
	common.ApiSuccess(c, pageInfo)
}

func tasksToDto(tasks []*model.Task, fillUser bool) []*dto.TaskDto {
	var userIdMap map[int]*model.UserBase
	if fillUser {
		userIdMap = make(map[int]*model.UserBase)
		userIds := types.NewSet[int]()
		for _, task := range tasks {
			userIds.Add(task.UserId)
		}
		for _, userId := range userIds.Items() {
			cacheUser, err := model.GetUserCache(userId)
			if err == nil {
				userIdMap[userId] = cacheUser
			}
		}
	}
	billingJobs := make(map[int64]*model.TaskBillingJob)
	taskIDs := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		if task != nil && task.ID > 0 {
			taskIDs = append(taskIDs, task.ID)
		}
	}
	if len(taskIDs) > 0 {
		var jobs []*model.TaskBillingJob
		if err := model.DB.Where("task_id IN ?", taskIDs).Find(&jobs).Error; err == nil {
			for _, job := range jobs {
				billingJobs[job.TaskID] = job
			}
		}
	}
	result := make([]*dto.TaskDto, len(tasks))
	for i, task := range tasks {
		if fillUser {
			if user, ok := userIdMap[task.UserId]; ok {
				task.Username = user.Username
			}
		}
		item := relay.TaskModel2Dto(task)
		isPrivateVideoPlatform := task.Platform == constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeStarAI)) ||
			task.Platform == constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC))
		if isPrivateVideoPlatform {
			// The URL is generated on demand and points to Molii's signed streaming
			// proxy. No upstream URL or video content is persisted in the log DTO.
			if task.Status == model.TaskStatusSuccess {
				item.ResultURL = service.BuildSignedVideoProxyPath(task.TaskID, task.UserId)
			} else {
				item.ResultURL = ""
			}
			item.Data = nil
			if bc := task.PrivateData.BillingContext; bc != nil {
				item.VideoParams = &dto.TaskVideoParams{
					Resolution: bc.EstimatedResolution,
					Ratio:      bc.EstimatedRatio,
					Seconds:    bc.EstimatedSeconds,
					FPS:        bc.EstimatedFPS,
					Width:      bc.EstimatedWidth,
					Height:     bc.EstimatedHeight,
					HasVideo:   bc.EstimatedHasVideo,
				}
			}
			item.Billing = taskBillingSummary(task, billingJobs[task.ID])
		} else {
			item.ResultURL = ""
		}
		result[i] = item
	}
	return result
}

func taskBillingSummary(task *model.Task, job *model.TaskBillingJob) *dto.TaskBillingSummary {
	if task == nil {
		return nil
	}

	state := service.TaskBillingPublicState(task, job)

	modelName := task.Properties.OriginModelName
	mode := "seedance"
	if task.Platform == constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC)) {
		mode = "grok_video"
	}
	summary := &dto.TaskBillingSummary{
		State: state,
		Mode:  mode,
		Model: modelName,
	}
	if state == "settled" {
		summary.FinalCost = float64(task.Quota) / common.QuotaPerUnit
	}

	bc := task.PrivateData.BillingContext
	if bc == nil {
		return summary
	}
	if bc.OriginModelName != "" {
		summary.Model = bc.OriginModelName
	}
	summary.GroupRatio = bc.GroupRatio
	if state != "settled" {
		return summary
	}

	if mode == "seedance" {
		summary.Seedance = &dto.TaskSeedanceBilling{
			ActualTokens: bc.ActualTokens,
			Resolution:   bc.EstimatedResolution,
			Ratio:        bc.EstimatedRatio,
			Seconds:      bc.EstimatedSeconds,
			HasVideo:     bc.EstimatedHasVideo,
			UnitPrice:    bc.EstimatedUnitPrice,
		}
		summary.DetailAvailable = state == "settled" && bc.ActualTokens > 0 && bc.EstimatedUnitPrice >= 0
		return summary
	}

	if snapshot := bc.GrokVideoBilling; snapshot != nil {
		summary.GrokVideo = &dto.TaskGrokVideoBillingV1{
			Version: snapshot.Version, Model: snapshot.Model,
			Operation: snapshot.Operation, InputType: snapshot.InputType,
			RequestedDurationSeconds: snapshot.RequestedDurationSeconds,
			EstimatedDurationSeconds: snapshot.EstimatedDurationSeconds,
			ActualDurationSeconds:    snapshot.ActualDurationSeconds,
			RequestedResolution:      snapshot.RequestedResolution,
			EstimatedResolution:      snapshot.EstimatedResolution,
			ActualResolution:         snapshot.ActualResolution,
			AspectRatio:              snapshot.AspectRatio, InputImageCount: snapshot.InputImageCount,
			VideoInputBilledSeconds: snapshot.VideoInputBilledSeconds,
			OutputUnitPrice:         snapshot.OutputUnitPrice,
			ImageInputUnitPrice:     snapshot.ImageInputUnitPrice,
			VideoInputUnitPrice:     snapshot.VideoInputUnitPrice,
			OutputCost:              snapshot.OutputCost, ImageInputCost: snapshot.ImageInputCost,
			VideoInputCost: snapshot.VideoInputCost, Subtotal: snapshot.Subtotal,
			GroupRatio: snapshot.GroupRatio, FinalCost: snapshot.FinalCost,
		}
		summary.DetailAvailable = state == "settled" && snapshot.Version == 1 &&
			snapshot.ActualDurationSeconds > 0 && snapshot.ActualResolution != ""
	}
	return summary
}
