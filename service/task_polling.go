package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"
)

// TaskPollingAdaptor 定义轮询所需的最小适配器接口，避免 service -> relay 的循环依赖
type TaskPollingAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	FetchTask(baseURL string, key string, body map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error)
	// AdjustBillingOnComplete 在任务到达终态（成功/失败）时由轮询循环调用。
	// 返回正数触发差额结算（补扣/退还），返回 0 保持预扣费金额不变。
	AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int
}

type perCallTaskCompletionAdjuster interface {
	AllowPerCallCompletionAdjustment() bool
}

// PrivateTaskPollingAdaptor is implemented by providers whose polling payload
// contains private upstream identifiers, result URLs, costs, or credentials.
// The polling loop then persists only adaptor-produced safe data and never
// includes the upstream task ID or raw body in normal logs and errors.
type PrivateTaskPollingAdaptor interface {
	IsPrivateTaskPolling() bool
	IsTaskPollingStatusAccepted(statusCode int) bool
	SafePollingData(taskResult *relaycommon.TaskInfo) []byte
	SafePollingError(statusCode int) error
}

func privateTaskPollingAdaptor(adaptor TaskPollingAdaptor) (PrivateTaskPollingAdaptor, bool) {
	privacy, ok := adaptor.(PrivateTaskPollingAdaptor)
	return privacy, ok && privacy.IsPrivateTaskPolling()
}

// GetTaskAdaptorFunc 由 main 包注入，用于获取指定平台的任务适配器。
// 打破 service -> relay -> relay/channel -> service 的循环依赖。
var GetTaskAdaptorFunc func(platform constant.TaskPlatform) TaskPollingAdaptor

func pollingUpstreamTaskID(task *model.Task) string {
	if task == nil {
		return ""
	}
	if task.PrivateData.UpstreamTaskID != "" {
		return task.PrivateData.UpstreamTaskID
	}
	// Before the public/private ID split, TaskID itself was the upstream ID.
	// Keep that compatibility only for explicitly legacy rows; modern rows must
	// never send the public task ID to an upstream provider.
	if task.SubmitTime < model.TaskRefundLegacyCutoff {
		return task.TaskID
	}
	return ""
}

func pollingTaskMapKey(channelID int, upstreamID string) string {
	return fmt.Sprintf("%d:%d:%s", channelID, len(upstreamID), upstreamID)
}

func addPollingTask(taskM map[string]*model.Task, channelID int, upstreamID string, task *model.Task) {
	taskM[pollingTaskMapKey(channelID, upstreamID)] = task
}

func getPollingTask(taskM map[string]*model.Task, channelID int, upstreamID string) *model.Task {
	if task := taskM[pollingTaskMapKey(channelID, upstreamID)]; task != nil {
		return task
	}
	// Compatibility for existing direct callers that provide a single-channel
	// flat map. RunTaskPollingOnce itself only constructs channel-scoped keys.
	return taskM[upstreamID]
}

func finalizePolledTaskWithBilling(ctx context.Context, task *model.Task, fromStatus model.TaskStatus, adaptor TaskPollingAdaptor, result *relaycommon.TaskInfo) (bool, error) {
	job := BuildTerminalTaskBillingJob(ctx, adaptor, task, result)
	if job == nil {
		return false, errors.New("terminal task billing intent is required")
	}
	return model.FinalizeTaskAndEnqueueBillingWithContext(ctx, task, fromStatus, job)
}

// sweepTimedOutTasks 在主轮询之前独立清理超时任务。
// 每次最多处理 100 条，剩余的下个周期继续处理。
// 使用 per-task CAS (UpdateWithStatus) 防止覆盖被正常轮询已推进的任务。
func sweepTimedOutTasks(ctx context.Context) {
	if err := sweepTimedOutTasksWithError(ctx); err != nil {
		logger.LogError(ctx, fmt.Sprintf("sweepTimedOutTasks query error: %v", err))
	}
}

func sweepTimedOutTasksWithError(ctx context.Context) error {
	if constant.TaskTimeoutMinutes <= 0 {
		return nil
	}
	cutoff := time.Now().Unix() - int64(constant.TaskTimeoutMinutes)*60
	tasks, err := model.GetTimedOutUnfinishedTasksWithError(cutoff, 100)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}

	reason := fmt.Sprintf("任务超时（%d分钟）", constant.TaskTimeoutMinutes)
	legacyReason := "任务超时（旧系统遗留任务，不进行退款，请联系管理员）"
	now := time.Now().Unix()
	timedOutCount := 0

	for _, task := range tasks {
		isLegacy := task.SubmitTime > 0 && task.SubmitTime < model.TaskRefundLegacyCutoff

		oldStatus := task.Status
		task.Status = model.TaskStatusFailure
		task.Progress = "100%"
		task.FinishTime = now
		if isLegacy {
			task.FailReason = legacyReason
			// 旧系统任务明确不退款，随终态 CAS 一并清掉 quota，
			// 避免留下可再次退款的计费状态。
			task.Quota = 0
		} else {
			task.FailReason = reason
		}

		var won bool
		var err error
		if isLegacy {
			won, err = task.UpdateWithStatus(oldStatus)
		} else {
			won, err = finalizePolledTaskWithBilling(ctx, task, oldStatus, nil, nil)
		}
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("sweepTimedOutTasks terminal update error for task %s: %v", task.TaskID, err))
			continue
		}
		if !won {
			logger.LogInfo(ctx, fmt.Sprintf("sweepTimedOutTasks: task %s already transitioned, skip", task.TaskID))
			continue
		}
		timedOutCount++
	}

	if timedOutCount > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("sweepTimedOutTasks: timed out %d tasks", timedOutCount))
	}
	return nil
}

// TaskPollSummary is the result recorded on an async_task_poll system task row,
// summarizing one polling pass.
type TaskPollSummary struct {
	UnfinishedTasks  int `json:"unfinished_tasks"`
	PlatformsScanned int `json:"platforms_scanned"`
	NullTasksFailed  int `json:"null_tasks_failed"`
}

// RunTaskPollingOnce performs one async-task (Suno/video) polling pass
// synchronously. It honors ctx cancellation (the system-task runner cancels it
// when the lease is lost) and, when report is non-nil, reports progress as
// (processedPlatforms, totalPlatforms). It returns immediately if the task
// adaptor factory has not been wired yet, to avoid a nil call during startup.
// RunTaskPollingOnce preserves the historical summary-only API. New system
// task callers use RunTaskPollingOnceWithError so database failures are
// persisted as failed runs; legacy callers still compile and receive a safe
// empty/partial summary while the error is logged.
func RunTaskPollingOnce(ctx context.Context, report func(processed, total int)) TaskPollSummary {
	summary, err := RunTaskPollingOnceWithError(ctx, report)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("task polling pass failed: %v", err))
	}
	return summary
}

func RunTaskPollingOnceWithError(ctx context.Context, report func(processed, total int)) (TaskPollSummary, error) {
	summary := TaskPollSummary{}
	if GetTaskAdaptorFunc == nil {
		return summary, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	common.SysLog("任务进度轮询开始")
	if err := sweepTimedOutTasksWithError(ctx); err != nil {
		return summary, err
	}
	allTasks, err := model.GetAllUnFinishSyncTasksWithError(constant.TaskQueryLimit)
	if err != nil {
		return summary, err
	}
	summary.UnfinishedTasks = len(allTasks)
	platformTask := make(map[constant.TaskPlatform][]*model.Task)
	for _, t := range allTasks {
		platformTask[t.Platform] = append(platformTask[t.Platform], t)
	}

	totalPlatforms := len(platformTask)
	processedPlatforms := 0
	for platform, tasks := range platformTask {
		if ctx.Err() != nil {
			break
		}
		if report != nil {
			report(processedPlatforms, totalPlatforms)
		}
		processedPlatforms++
		if len(tasks) == 0 {
			continue
		}
		summary.PlatformsScanned++
		taskChannelM := make(map[int][]string)
		taskM := make(map[string]*model.Task)
		for _, task := range tasks {
			upstreamID := pollingUpstreamTaskID(task)
			if upstreamID == "" {
				oldStatus := task.Status
				task.Status = model.TaskStatusFailure
				task.Progress = "100%"
				task.FinishTime = time.Now().Unix()
				task.FailReason = "缺少上游任务 ID"
				won, err := finalizePolledTaskWithBilling(ctx, task, oldStatus, nil, nil)
				if err != nil {
					logger.LogError(ctx, fmt.Sprintf("Fail task without upstream ID %d: %v", task.ID, err))
				} else if won {
					summary.NullTasksFailed++
				}
				continue
			}
			addPollingTask(taskM, task.ChannelId, upstreamID, task)
			taskChannelM[task.ChannelId] = append(taskChannelM[task.ChannelId], upstreamID)
		}
		if len(taskChannelM) == 0 {
			continue
		}

		DispatchPlatformUpdate(ctx, platform, taskChannelM, taskM)
	}
	if report != nil && ctx.Err() == nil {
		report(totalPlatforms, totalPlatforms)
	}
	common.SysLog("任务进度轮询完成")
	return summary, nil
}

// DispatchPlatformUpdate 按平台分发轮询更新
func DispatchPlatformUpdate(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) {
	if ctx == nil {
		ctx = context.Background()
	}
	switch platform {
	case constant.TaskPlatformMidjourney:
		// MJ 轮询由其自身处理，这里预留入口
	case constant.TaskPlatformSuno:
		_ = UpdateSunoTasks(ctx, taskChannelM, taskM)
	default:
		if err := UpdateVideoTasks(ctx, platform, taskChannelM, taskM); err != nil {
			common.SysLog(fmt.Sprintf("UpdateVideoTasks fail: %s", err))
		}
	}
}

// UpdateSunoTasks 按渠道更新所有 Suno 任务
func UpdateSunoTasks(ctx context.Context, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelId, taskIds := range taskChannelM {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := updateSunoTasks(ctx, channelId, taskIds, taskM)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("渠道 #%d 更新异步任务失败: %s", channelId, err.Error()))
		}
	}
	return nil
}

func updateSunoTasks(ctx context.Context, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("渠道 #%d 未完成的任务有: %d", channelId, len(taskIds)))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(taskIds) == 0 {
		return nil
	}
	ch, err := model.CacheGetChannel(channelId)
	if err != nil {
		common.SysLog(fmt.Sprintf("CacheGetChannel: %v", err))
		reason := fmt.Sprintf("获取渠道信息失败，请联系管理员，渠道ID：%d", channelId)
		for _, upstreamID := range taskIds {
			if t := getPollingTask(taskM, channelId, upstreamID); t != nil {
				oldStatus := t.Status
				t.Status = model.TaskStatusFailure
				t.Progress = "100%"
				t.FinishTime = time.Now().Unix()
				t.FailReason = reason
				if _, finalizeErr := finalizePolledTaskWithBilling(ctx, t, oldStatus, nil, nil); finalizeErr != nil {
					common.SysLog(fmt.Sprintf("UpdateSunoTask terminal error: %v", finalizeErr))
				}
			}
		}
		return err
	}
	adaptor := GetTaskAdaptorFunc(constant.TaskPlatformSuno)
	if adaptor == nil {
		return errors.New("adaptor not found")
	}
	proxy := ch.GetSetting().Proxy
	resp, err := adaptor.FetchTask(*ch.BaseURL, ch.Key, map[string]any{
		"ids": taskIds,
	}, proxy)
	if err != nil {
		common.SysLog(fmt.Sprintf("Get Task Do req error: %v", err))
		return err
	}
	if resp.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("Get Task status code: %d", resp.StatusCode))
		return fmt.Errorf("Get Task status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		common.SysLog(fmt.Sprintf("Get Suno Task parse body error: %v", err))
		return err
	}
	var responseItems taskdto.TaskResponse[[]taskdto.SunoDataResponse]
	err = common.Unmarshal(responseBody, &responseItems)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Get Suno Task parse body error2: %v, body: %s", err, string(responseBody)))
		return err
	}
	if !responseItems.IsSuccess() {
		common.SysLog(fmt.Sprintf("渠道 #%d 未完成的任务有: %d, 成功获取到任务数: %s", channelId, len(taskIds), string(responseBody)))
		return err
	}

	for _, responseItem := range responseItems.Data {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		task := getPollingTask(taskM, channelId, responseItem.TaskID)
		if task == nil {
			logger.LogWarn(ctx, fmt.Sprintf("Suno task response ignored: unknown task_id=%s", responseItem.TaskID))
			continue
		}
		if !taskNeedsUpdate(task, responseItem) {
			continue
		}

		snap := task.Snapshot()
		prevStatus := task.Status
		task.Status = lo.If(model.TaskStatus(responseItem.Status) != "", model.TaskStatus(responseItem.Status)).Else(task.Status)
		task.FailReason = lo.If(responseItem.FailReason != "", responseItem.FailReason).Else(task.FailReason)
		task.SubmitTime = lo.If(responseItem.SubmitTime != 0, responseItem.SubmitTime).Else(task.SubmitTime)
		task.StartTime = lo.If(responseItem.StartTime != 0, responseItem.StartTime).Else(task.StartTime)
		task.FinishTime = lo.If(responseItem.FinishTime != 0, responseItem.FinishTime).Else(task.FinishTime)
		isFailure := responseItem.FailReason != "" || task.Status == model.TaskStatusFailure
		if isFailure {
			logger.LogInfo(ctx, task.TaskID+" 构建失败，"+task.FailReason)
			task.Status = model.TaskStatusFailure
			task.Progress = "100%"
		}
		if responseItem.Status == model.TaskStatusSuccess {
			task.Progress = "100%"
		}
		task.Data = responseItem.Data

		isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
		if isDone && prevStatus != task.Status {
			result := &relaycommon.TaskInfo{
				TaskID: task.TaskID, Status: string(task.Status), Progress: task.Progress, Reason: task.FailReason,
			}
			won, err := finalizePolledTaskWithBilling(ctx, task, prevStatus, adaptor, result)
			if err != nil {
				logger.LogError(ctx, fmt.Sprintf("UpdateSunoTask task %s terminal error: %v", task.TaskID, err))
			} else if !won {
				logger.LogWarn(ctx, fmt.Sprintf("Task %s CAS lost or no-op update, skip billing", task.TaskID))
			}
		} else if !snap.Equal(task.Snapshot()) {
			if _, err := task.UpdateWithStatus(prevStatus); err != nil {
				logger.LogError(ctx, fmt.Sprintf("UpdateSunoTask task %s error: %v", task.TaskID, err))
			}
		}
	}
	return nil
}

// taskNeedsUpdate 检查 Suno 任务是否需要更新
func taskNeedsUpdate(oldTask *model.Task, newTask taskdto.SunoDataResponse) bool {
	if oldTask.SubmitTime != newTask.SubmitTime {
		return true
	}
	if oldTask.StartTime != newTask.StartTime {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if string(oldTask.Status) != newTask.Status {
		return true
	}
	if oldTask.FailReason != newTask.FailReason {
		return true
	}

	if (oldTask.Status == model.TaskStatusFailure || oldTask.Status == model.TaskStatusSuccess) && oldTask.Progress != "100%" {
		return true
	}

	oldData, _ := common.Marshal(oldTask.Data)
	newData, _ := common.Marshal(newTask.Data)

	sort.Slice(oldData, func(i, j int) bool {
		return oldData[i] < oldData[j]
	})
	sort.Slice(newData, func(i, j int) bool {
		return newData[i] < newData[j]
	})

	if string(oldData) != string(newData) {
		return true
	}
	return false
}

// UpdateVideoTasks 按渠道更新所有视频任务
func UpdateVideoTasks(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	channelIDs := make([]int, 0, len(taskChannelM))
	for channelID := range taskChannelM {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Ints(channelIDs)

	var wg sync.WaitGroup
	for _, channelId := range channelIDs {
		taskIds := taskChannelM[channelId]
		if len(taskIds) == 0 {
			continue
		}
		taskIds = append([]string(nil), taskIds...)

		wg.Add(1)
		gopool.Go(func() {
			defer wg.Done()
			if err := updateVideoTasks(ctx, platform, channelId, taskIds, taskM); err != nil {
				adaptor := GetTaskAdaptorFunc(platform)
				_, privatePolling := privateTaskPollingAdaptor(adaptor)
				if platform == constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeStarAI)) {
					logger.LogError(ctx, fmt.Sprintf("Channel #%d failed to update Molii Volcengine Imagine API async tasks", channelId))
				} else if privatePolling {
					logger.LogError(ctx, fmt.Sprintf("Channel #%d failed to update private async tasks", channelId))
				} else {
					logger.LogError(ctx, fmt.Sprintf("Channel #%d failed to update video async tasks: %s", channelId, err.Error()))
				}
			}
		})
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func updateVideoTasks(ctx context.Context, platform constant.TaskPlatform, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("Channel #%d pending video tasks: %d", channelId, len(taskIds)))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(taskIds) == 0 {
		return nil
	}
	cacheGetChannel, err := model.CacheGetChannel(channelId)
	if err != nil {
		reason := fmt.Sprintf("Failed to get channel info, channel ID: %d", channelId)
		for _, upstreamID := range taskIds {
			if t := getPollingTask(taskM, channelId, upstreamID); t != nil {
				oldStatus := t.Status
				t.Status = model.TaskStatusFailure
				t.Progress = taskcommon.ProgressComplete
				t.FinishTime = time.Now().Unix()
				t.FailReason = reason
				if _, finalizeErr := finalizePolledTaskWithBilling(ctx, t, oldStatus, nil, nil); finalizeErr != nil {
					common.SysLog(fmt.Sprintf("UpdateVideoTask terminal error: %v", finalizeErr))
				}
			}
		}
		return fmt.Errorf("CacheGetChannel failed: %w", err)
	}
	if !IsMoliiGrokTaskRoutingConsistent(platform, cacheGetChannel.Type) {
		return errors.New("Molii Grok Imagine API task platform/channel mismatch")
	}
	adaptor := GetTaskAdaptorFunc(platform)
	if adaptor == nil {
		return fmt.Errorf("video adaptor not found")
	}
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelBaseUrl: cacheGetChannel.GetBaseURL(),
	}
	info.ApiKey = cacheGetChannel.Key
	adaptor.Init(info)
	disablePollingSleep := cacheGetChannel.GetOtherSettings().DisableTaskPollingSleep
	_, privatePolling := privateTaskPollingAdaptor(adaptor)
	for i, taskId := range taskIds {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := updateVideoSingleTask(ctx, adaptor, cacheGetChannel, taskId, taskM); err != nil {
			if cacheGetChannel.Type == constant.ChannelTypeStarAI {
				publicTaskID := "unknown"
				if task := getPollingTask(taskM, channelId, taskId); task != nil {
					publicTaskID = task.TaskID
				}
				logger.LogError(ctx, fmt.Sprintf("Failed to update Molii Volcengine Imagine API public task %s", publicTaskID))
			} else if privatePolling {
				publicTaskID := "unknown"
				if task := getPollingTask(taskM, channelId, taskId); task != nil {
					publicTaskID = task.TaskID
				}
				logger.LogError(ctx, fmt.Sprintf("Failed to update private public task %s", publicTaskID))
			} else {
				logger.LogError(ctx, fmt.Sprintf("Failed to update video task %s: %s", taskId, err.Error()))
			}
		}
		if disablePollingSleep || i == len(taskIds)-1 {
			continue
		}

		// sleep 1 second between tasks for this channel only.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return nil
}

func updateVideoSingleTask(ctx context.Context, adaptor TaskPollingAdaptor, ch *model.Channel, taskId string, taskM map[string]*model.Task) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	baseURL := constant.ChannelBaseURLs[ch.Type]
	if ch.GetBaseURL() != "" {
		baseURL = ch.GetBaseURL()
	}
	proxy := ch.GetSetting().Proxy

	privacy, privatePolling := privateTaskPollingAdaptor(adaptor)
	task := getPollingTask(taskM, ch.Id, taskId)
	if task == nil {
		if ch.Type == constant.ChannelTypeStarAI {
			logger.LogError(ctx, "Video polling response did not match a pending task")
			return errors.New("video polling response did not match a pending task")
		}
		if privatePolling {
			logger.LogError(ctx, "Private video polling response did not match a pending task")
			return errors.New("private video polling response did not match a pending task")
		}
		logger.LogError(ctx, fmt.Sprintf("Task %s not found in taskM", taskId))
		return fmt.Errorf("task %s not found", taskId)
	}
	if !IsMoliiGrokTaskRoutingConsistent(task.Platform, ch.Type) {
		return errors.New("Molii Grok Imagine API task platform/channel mismatch")
	}
	publicTaskID := task.TaskID
	isStarAI := ch.Type == constant.ChannelTypeStarAI
	key := ch.Key

	privateData := task.PrivateData
	if privateData.Key != "" {
		key = privateData.Key
	}
	resp, err := adaptor.FetchTask(baseURL, key, map[string]any{
		"task_id": taskId,
		"action":  task.Action,
	}, proxy)
	if err != nil {
		if isStarAI {
			return fmt.Errorf("fetchTask failed for public task %s", publicTaskID)
		}
		if privatePolling {
			return fmt.Errorf("fetchTask failed for public task %s", publicTaskID)
		}
		return fmt.Errorf("fetchTask failed for task %s: %w", taskId, err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if isStarAI {
			return fmt.Errorf("readAll failed for public task %s", publicTaskID)
		}
		if privatePolling {
			return fmt.Errorf("readAll failed for public task %s", publicTaskID)
		}
		return fmt.Errorf("readAll failed for task %s: %w", taskId, err)
	}
	if privatePolling && !privacy.IsTaskPollingStatusAccepted(resp.StatusCode) {
		return privacy.SafePollingError(resp.StatusCode)
	}

	snap := task.Snapshot()

	taskResult := &relaycommon.TaskInfo{}
	// try parse as New API response format
	var responseItems taskdto.TaskResponse[model.Task]
	if !isStarAI && !privatePolling {
		err = common.Unmarshal(responseBody, &responseItems)
	}
	if !isStarAI && !privatePolling && err == nil && responseItems.IsSuccess() {
		logger.LogDebug(ctx, "updateVideoSingleTask parsed as new api response format: %+v", responseItems)
		t := responseItems.Data
		taskResult.TaskID = t.TaskID
		taskResult.Status = string(t.Status)
		taskResult.Url = t.GetResultURL()
		taskResult.Progress = t.Progress
		taskResult.Reason = t.FailReason
		task.Data = t.Data
	} else if taskResult, err = adaptor.ParseTaskResult(responseBody); err != nil {
		if isStarAI {
			return fmt.Errorf("parseTaskResult failed for public task %s", publicTaskID)
		}
		if privatePolling {
			return fmt.Errorf("parseTaskResult failed for public task %s", publicTaskID)
		}
		return fmt.Errorf("parseTaskResult failed for task %s: %w", taskId, err)
	}

	if isStarAI {
		task.Data = SanitizeStarAIResponseBody(responseBody, publicTaskID)
	} else if privatePolling {
		task.Data = privacy.SafePollingData(taskResult)
	} else {
		task.Data = redactVideoResponseBody(responseBody)
	}
	if isStarAI {
		logger.LogDebug(ctx, "updateVideoSingleTask safe response: %s", task.Data)
		logger.LogDebug(ctx, "updateVideoSingleTask public task %s parsed with status %s", publicTaskID, taskResult.Status)
	} else if privatePolling {
		logger.LogDebug(ctx, "updateVideoSingleTask public task %s parsed with status %s", publicTaskID, taskResult.Status)
	} else {
		logger.LogDebug(ctx, "updateVideoSingleTask response: %s", responseBody)
		logger.LogDebug(ctx, "updateVideoSingleTask taskResult: %+v", taskResult)
	}

	now := time.Now().Unix()
	if taskResult.Status == "" {
		//taskResult = relaycommon.FailTaskInfo("upstream returned empty status")
		errorResult := &dto.GeneralErrorResponse{}
		if err = common.Unmarshal(responseBody, &errorResult); err == nil {
			openaiError := errorResult.TryToOpenAIError()
			if openaiError != nil {
				// 返回规范的 OpenAI 错误格式，提取错误信息，判断错误是否为任务失败
				if openaiError.Code == "429" {
					// 429 错误通常表示请求过多或速率限制，暂时不认为是任务失败，保持原状态等待下一轮轮询
					return nil
				}

				// 其他错误认为是任务失败，记录错误信息并更新任务状态
				taskResult = relaycommon.FailTaskInfo("upstream returned error")
			} else {
				if isStarAI {
					// Only the already-sanitized StarAI response may be logged.
					logger.LogError(ctx, fmt.Sprintf("Public task %s returned empty status with unrecognized error format, safe response: %s", publicTaskID, string(task.Data)))
				} else if privatePolling {
					logger.LogError(ctx, fmt.Sprintf("Public task %s returned empty status", publicTaskID))
				} else {
					logger.LogError(ctx, fmt.Sprintf("Task %s returned empty status with unrecognized error format, response: %s", taskId, string(responseBody)))
				}
				taskResult = relaycommon.FailTaskInfo("upstream returned unrecognized message")
			}
		}
	}

	grokResultURL := ""
	if taskResult.Status == model.TaskStatusSuccess && ch.Type == constant.ChannelTypeMoliiGrokAIGC {
		grokResultURL = strings.TrimSpace(taskResult.Url)
		if !IsTrustedMoliiGrokVideoURL(grokResultURL) {
			return fmt.Errorf("Molii Grok Imagine API returned an invalid result URL for public task %s", publicTaskID)
		}
		// Private polling logs only status and adaptor-produced SafePollingData;
		// the URL is persisted exclusively in TaskPrivateData below.
	}

	task.Status = model.TaskStatus(taskResult.Status)
	switch taskResult.Status {
	case model.TaskStatusSubmitted:
		task.Progress = taskcommon.ProgressSubmitted
	case model.TaskStatusQueued:
		task.Progress = taskcommon.ProgressQueued
	case model.TaskStatusInProgress:
		task.Progress = taskcommon.ProgressInProgress
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case model.TaskStatusSuccess:
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if ch.Type == constant.ChannelTypeMoliiGrokAIGC {
			task.PrivateData.ResultURL = grokResultURL
		} else if strings.HasPrefix(taskResult.Url, "data:") {
			// data: URI (e.g. Vertex base64 encoded video) — keep in Data, not in ResultURL
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		} else if taskResult.Url != "" {
			// Direct upstream URL (e.g. Kling, Ali, Doubao, etc.)
			task.PrivateData.ResultURL = taskResult.Url
		} else {
			// No URL from adaptor — construct proxy URL using public task ID
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		}
	case model.TaskStatusFailure:
		if isStarAI {
			logger.LogInfo(ctx, fmt.Sprintf("Molii Volcengine Imagine API public task %s reached failure status", publicTaskID))
		} else if privatePolling {
			logger.LogInfo(ctx, fmt.Sprintf("Private public task %s reached failure status", publicTaskID))
		} else {
			logger.LogJson(ctx, fmt.Sprintf("Task %s failed", taskId), task)
		}
		task.Status = model.TaskStatusFailure
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Reason
		if isStarAI {
			task.FailReason = sanitizeStarAIText(taskResult.Reason, responseBody, publicTaskID)
		} else if privatePolling {
			task.FailReason = "Molii Grok Imagine API task failed"
		}
		if isStarAI {
			logger.LogInfo(ctx, fmt.Sprintf("Molii Volcengine Imagine API public task %s failed", task.TaskID))
		} else if privatePolling {
			logger.LogInfo(ctx, fmt.Sprintf("Private public task %s failed", task.TaskID))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("Task %s failed: %s", task.TaskID, task.FailReason))
		}
		taskResult.Progress = taskcommon.ProgressComplete
	default:
		return fmt.Errorf("unknown task status %s for task %s", taskResult.Status, task.TaskID)
	}
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}

	isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
	if isDone && snap.Status != task.Status {
		won, err := finalizePolledTaskWithBilling(ctx, task, snap.Status, adaptor, taskResult)
		if err != nil {
			return fmt.Errorf("terminal task persistence failed for task %s: %w", task.TaskID, err)
		} else if !won {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s CAS lost or no-op update, skip billing", task.TaskID))
		}
	} else if !snap.Equal(task.Snapshot()) {
		if _, err := task.UpdateWithStatus(snap.Status); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Failed to update task %s: %s", task.TaskID, err.Error()))
		}
	} else {
		// No changes, skip update
		logger.LogDebug(ctx, "No update needed for task %s", task.TaskID)
	}

	return nil
}

func redactVideoResponseBody(body []byte) []byte {
	var m map[string]any
	if err := common.Unmarshal(body, &m); err != nil {
		return body
	}
	resp, _ := m["response"].(map[string]any)
	if resp != nil {
		delete(resp, "bytesBase64Encoded")
		if v, ok := resp["video"].(string); ok {
			resp["video"] = truncateBase64(v)
		}
		if vs, ok := resp["videos"].([]any); ok {
			for i := range vs {
				if vm, ok := vs[i].(map[string]any); ok {
					delete(vm, "bytesBase64Encoded")
				}
			}
		}
	}
	b, err := common.Marshal(m)
	if err != nil {
		return body
	}
	return b
}

func truncateBase64(s string) string {
	const maxKeep = 256
	if len(s) <= maxKeep {
		return s
	}
	return s[:maxKeep] + "..."
}

// settleTaskBillingOnComplete 任务完成时的统一计费调整。
// 优先级：1. adaptor.AdjustBillingOnComplete 返回正数 → 使用 adaptor 计算的额度
//
//  2. taskResult.TotalTokens > 0 → 按 token 重算
//  3. 都不满足 → 保持预扣额度不变
func settleTaskBillingOnComplete(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo) bool {
	// 0. 按次计费的任务不做差额结算
	if bc := task.PrivateData.BillingContext; bc != nil && bc.PerCallBilling {
		adjuster, ok := adaptor.(perCallTaskCompletionAdjuster)
		if !ok || !adjuster.AllowPerCallCompletionAdjustment() {
			logger.LogInfo(ctx, fmt.Sprintf("任务 %s 按次计费，跳过差额结算", task.TaskID))
			return true
		}
	}
	// 1. 优先让 adaptor 决定最终额度
	actualQuota := adaptor.AdjustBillingOnComplete(task, taskResult)
	if isMoliiGrokFinalUsageTask(task) {
		if !validFinalizedGrokVideoBilling(task.PrivateData.BillingContext.GrokVideoBilling) {
			logger.LogError(ctx, fmt.Sprintf("Grok video billing snapshot finalization failed for task %s", task.TaskID))
			return false
		}
		return RecalculateTaskQuota(ctx, task, actualQuota, "adaptor计费调整")
	}
	if actualQuota > 0 {
		return RecalculateTaskQuota(ctx, task, actualQuota, "adaptor计费调整")
	}
	// 2. 回退到 token 重算
	if taskResult.TotalTokens > 0 {
		return RecalculateTaskQuotaByTokens(ctx, task, taskResult.TotalTokens)
	}
	// 3. 无调整，保持预扣额度
	return true
}
