package moliigrok

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func (a *TaskAdaptor) AllowAutomaticTaskSubmitRetry() bool { return false }

func (a *TaskAdaptor) AllowPerCallCompletionAdjustment() bool { return true }

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(info.OriginModelName)
	}
	if modelName != VideoModel && modelName != LegacyVideoModel {
		return service.TaskErrorWrapperLocal(errors.New("model is not supported by Molii Grok Imagine API"), "invalid_model", http.StatusBadRequest)
	}
	if info.UpstreamModelName != "" && info.UpstreamModelName != VideoModel && info.UpstreamModelName != LegacyVideoModel {
		return service.TaskErrorWrapperLocal(errors.New("mapped model is not supported by Molii Grok Imagine API"), "invalid_model", http.StatusBadRequest)
	}
	prompt := strings.TrimSpace(req.Prompt)
	if utf8.RuneCountInString(prompt) > 10000 {
		return service.TaskErrorWrapperLocal(errors.New("prompt must not exceed 10000 characters"), "invalid_prompt", http.StatusBadRequest)
	}
	req.Model = modelName
	req.Prompt = prompt
	isEdit := strings.HasSuffix(c.Request.URL.Path, "/videos/edits")
	if isEdit {
		if modelName != LegacyVideoModel {
			return service.TaskErrorWrapperLocal(errors.New("video editing requires grok-imagine-video"), "invalid_model", http.StatusBadRequest)
		}
		if strings.TrimSpace(req.Video) == "" {
			return service.TaskErrorWrapperLocal(errors.New("video is required"), "invalid_video", http.StatusBadRequest)
		}
		if req.Duration != 0 || strings.TrimSpace(req.AspectRatio) != "" || strings.TrimSpace(req.Resolution) != "" {
			return service.TaskErrorWrapperLocal(errors.New("duration, aspect_ratio and resolution are not supported for video edits"), "invalid_request", http.StatusBadRequest)
		}
		c.Set("task_request", req)
		info.Action = videoEditAction
		info.EstimatedVideoSeconds = 9
		info.EstimatedVideoResolution = "720p"
		info.EstimatedVideoHasInput = true
		return nil
	}
	if modelName == VideoModel && strings.TrimSpace(req.Image) == "" && len(req.Images) == 0 {
		return service.TaskErrorWrapperLocal(errors.New("Grok Imagine Video 1.5 models require an image"), "invalid_image", http.StatusBadRequest)
	}
	if req.Duration == 0 {
		req.Duration = 5
	}
	if req.Duration < 1 || req.Duration > 15 {
		return service.TaskErrorWrapperLocal(errors.New("duration must be between 1 and 15 seconds"), "invalid_duration", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.AspectRatio) == "" {
		req.AspectRatio = "16:9"
	}
	if strings.TrimSpace(req.Resolution) == "" {
		req.Resolution = "480p"
	}
	if len(req.AspectRatio) > 32 || len(req.Resolution) > 32 {
		return service.TaskErrorWrapperLocal(errors.New("invalid video parameters"), "invalid_request", http.StatusBadRequest)
	}
	if _, _, _, ok := ratio_setting.GetMoliiGrokVideoPrices(modelName, req.Resolution); !ok {
		return service.TaskErrorWrapperLocal(errors.New("resolution is not supported by the selected model"), "invalid_resolution", http.StatusBadRequest)
	}
	c.Set("task_request", req)
	info.Action = constant.TaskActionGenerate
	info.EstimatedVideoSeconds = req.Duration
	info.EstimatedVideoRatio = req.AspectRatio
	info.EstimatedVideoResolution = req.Resolution
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		return "", errors.New("Molii Grok Imagine API configuration is incomplete")
	}
	if info != nil && info.Action == videoEditAction {
		return a.baseURL + "/v1/videos/edits", nil
	}
	return a.baseURL + "/v1/videos/generations", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = VideoModel
	}
	if info != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		modelName = strings.TrimSpace(info.UpstreamModelName)
	}
	if info != nil && info.Action == videoEditAction {
		payload := videoEditRequestPayload{
			Model:  modelName,
			Prompt: strings.TrimSpace(req.Prompt),
			Video:  buildMediaInput(req.Video, req.VideoFileID),
		}
		body, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(body), nil
	}
	payload := videoRequestPayload{
		Model:       modelName,
		Prompt:      strings.TrimSpace(req.Prompt),
		Duration:    req.Duration,
		AspectRatio: strings.TrimSpace(req.AspectRatio),
		Resolution:  strings.TrimSpace(req.Resolution),
	}
	imageURL := strings.TrimSpace(req.Image)
	if imageURL == "" && len(req.Images) > 0 {
		imageURL = strings.TrimSpace(req.Images[0])
	}
	if imageURL != "" {
		media := buildMediaInput(imageURL, req.ImageFileID)
		payload.Image = &media
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func buildMediaInput(value, fileID string) mediaInput {
	if strings.TrimSpace(fileID) != "" {
		return mediaInput{FileID: strings.TrimSpace(fileID)}
	}
	return mediaInput{URL: strings.TrimSpace(value)}
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if info == nil {
		return nil
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(info.OriginModelName)
	}
	basePrice, ok := ratio_setting.GetModelPrice(modelName, false)
	if !ok {
		basePrice, ok = ratio_setting.GetDefaultModelPriceMap()[modelName]
	}
	if !ok || basePrice <= 0 {
		return nil
	}
	resolution := strings.ToLower(strings.TrimSpace(req.Resolution))
	seconds := float64(req.Duration)
	if info.Action == videoEditAction {
		resolution = "720p"
		seconds = 8.7
	}
	outputPrice, imageInputPrice, videoInputPrice, ok := ratio_setting.GetMoliiGrokVideoPrices(modelName, resolution)
	if !ok {
		return nil
	}
	cost := seconds * outputPrice
	if info.Action == videoEditAction {
		cost += seconds * videoInputPrice
	} else if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 {
		cost += imageInputPrice
	}
	info.EstimatedVideoPrice = cost
	info.EstimatedVideoUnitPrice = outputPrice
	if info.Action == videoEditAction {
		info.EstimatedVideoInputUnitPrice = videoInputPrice
		info.EstimatedVideoOutputUnitPrices = make(map[string]float64, 2)
		for _, candidate := range []string{"480p", "720p"} {
			candidateOutput, _, _, configured := ratio_setting.GetMoliiGrokVideoPrices(modelName, candidate)
			if configured {
				info.EstimatedVideoOutputUnitPrices[candidate] = candidateOutput
			}
		}
	} else if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 {
		info.EstimatedVideoInputUnitPrice = imageInputPrice
	}
	return map[string]float64{"molii_grok_direct_cost": cost / basePrice}
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil || taskResult == nil || task.PrivateData.BillingContext == nil {
		return 0
	}
	bc := task.PrivateData.BillingContext
	if snapshot := bc.GrokVideoBilling; snapshot != nil && snapshot.Version == 1 {
		duration := taskResult.ActualDurationSeconds
		if duration <= 0 {
			duration = snapshot.RequestedDurationSeconds
		}
		if duration <= 0 {
			duration = snapshot.EstimatedDurationSeconds
		}
		if duration < 0 {
			return 0
		}

		if snapshot.Operation == videoEditAction {
			snapshot.ActualDurationSeconds = duration
			snapshot.VideoInputBilledSeconds = duration
			snapshot.ActualResolution = ""
			snapshot.ResolutionSource = ""
			resolution := normalizePollingResolution(taskResult.ActualResolution)
			outputPrice, ok := bc.EstimatedOutputUnitPrices[resolution]
			if !ok || outputPrice < 0 || math.IsNaN(outputPrice) || math.IsInf(outputPrice, 0) {
				return 0
			}
			billedModel := strings.TrimSpace(snapshot.BilledModel)
			if billedModel == "" {
				billedModel = strings.TrimSpace(snapshot.Model)
			}
			if _, _, _, configured := ratio_setting.GetMoliiGrokVideoPrices(billedModel, resolution); !configured {
				return 0
			}
			snapshot.ActualResolution = resolution
			snapshot.ResolutionSource = model.GrokVideoResolutionSourceProviderPollV1
			snapshot.OutputUnitPrice = outputPrice
		} else {
			resolution := snapshot.RequestedResolution
			if resolution == "" {
				resolution = snapshot.EstimatedResolution
			}
			snapshot.ActualResolution = resolution
		}

		snapshot.ActualDurationSeconds = duration
		snapshot.OutputCost = snapshot.OutputUnitPrice * duration
		snapshot.ImageInputCost = snapshot.ImageInputUnitPrice * float64(snapshot.InputImageCount)
		snapshot.VideoInputCost = snapshot.VideoInputUnitPrice * snapshot.VideoInputBilledSeconds
		snapshot.Subtotal = snapshot.OutputCost + snapshot.ImageInputCost + snapshot.VideoInputCost
		snapshot.GroupRatio = bc.GroupRatio
		quota, _ := common.QuotaRoundChecked(snapshot.Subtotal * common.QuotaPerUnit * bc.GroupRatio)
		return quota
	}

	if !bc.EstimatedHasVideo || bc.OriginModelName != LegacyVideoModel {
		return 0
	}
	// Historical edit contexts do not carry an authoritative final resolution.
	// Mark them for terminal-only review, but never invent a 480p/720p tier.
	bc.FinalUsageLogOnly = true
	if bc.GrokVideoBilling == nil {
		bc.GrokVideoBilling = &model.GrokVideoBillingSnapshot{
			Version:                  1,
			Model:                    LegacyVideoModel,
			Operation:                videoEditAction,
			InputType:                "video",
			EstimatedDurationSeconds: float64(bc.EstimatedSeconds),
			EstimatedResolution:      strings.ToLower(strings.TrimSpace(bc.EstimatedResolution)),
			VideoInputUnitPrice:      bc.EstimatedInputUnitPrice,
			GroupRatio:               bc.GroupRatio,
		}
	}
	bc.GrokVideoBilling.ActualDurationSeconds = taskResult.ActualDurationSeconds
	return 0
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *taskdto.TaskError) {
	if resp == nil || resp.Body == nil {
		return "", nil, service.TaskErrorWrapper(errors.New("Molii Grok Imagine API request failed"), "invalid_response", http.StatusBadGateway)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return "", nil, service.TaskErrorWrapper(errors.New("Molii Grok Imagine API request failed"), "read_response_body_failed", http.StatusBadGateway)
	}
	var upstream videoSubmitResponse
	if err := common.Unmarshal(body, &upstream); err != nil || strings.TrimSpace(upstream.RequestID) == "" {
		return "", nil, service.TaskErrorWrapper(errors.New("Molii Grok Imagine API request failed"), "invalid_response", http.StatusBadGateway)
	}

	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.Model = info.OriginModelName
	c.JSON(http.StatusOK, video)
	safeData, _ := common.Marshal(safeTaskData{Status: "submitted", Progress: 0})
	return upstream.RequestID, safeData, nil
}

func (a *TaskAdaptor) GetModelList() []string { return ModelList }

func (a *TaskAdaptor) GetChannelName() string { return ChannelName }

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	upstreamID, _ := body["task_id"].(string)
	upstreamID = strings.TrimSpace(upstreamID)
	if upstreamID == "" {
		return nil, errors.New("task identifier is missing")
	}
	requestURL := strings.TrimRight(baseURL, "/") + "/v1/videos/" + url.PathEscape(upstreamID)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, errors.New("create Molii Grok Imagine API polling request failed")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	client := service.GetHttpClient()
	if strings.TrimSpace(proxy) != "" {
		client, err = service.GetHttpClientWithProxy(proxy)
		if err != nil {
			return nil, errors.New("create Molii Grok Imagine API polling client failed")
		}
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var upstream videoPollResponse
	if err := common.Unmarshal(body, &upstream); err != nil {
		return nil, errors.New("Molii Grok Imagine API request failed")
	}
	status := strings.ToLower(strings.TrimSpace(upstream.Status))
	if status == "" {
		return nil, errors.New("Molii Grok Imagine API request failed")
	}
	progress := upstream.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	result := &relaycommon.TaskInfo{Progress: strconv.Itoa(progress) + "%"}
	switch status {
	case "done":
		result.Status = model.TaskStatusSuccess
		result.Progress = "100%"
		result.Url = strings.TrimSpace(upstream.Video.URL)
		result.ActualDurationSeconds = upstream.Video.Duration
		result.ActualResolution = normalizePollingResolution(upstream.Video.Resolution)
		result.ProviderCost = float64(upstream.Usage.CostInUSDTicks) / 10_000_000_000
		parsedURL, err := url.Parse(result.Url)
		if err != nil || !strings.EqualFold(parsedURL.Scheme, "https") || strings.TrimSpace(parsedURL.Host) == "" {
			return nil, errors.New("Molii Grok Imagine API returned an invalid video result")
		}
	case "failed", "expired":
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = "Molii Grok Imagine API task failed"
	default:
		result.Status = model.TaskStatusInProgress
	}
	return result, nil
}

func normalizePollingResolution(value string) string {
	resolution := strings.ToLower(strings.TrimSpace(value))
	switch resolution {
	case "480p", "720p", "1080p":
		return resolution
	default:
		return ""
	}
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	if task == nil {
		return nil, errors.New("task is missing")
	}
	video := dto.NewOpenAIVideo()
	video.ID = task.TaskID
	video.TaskID = task.TaskID
	video.Model = task.Properties.OriginModelName
	video.Status = task.Status.ToVideoStatus()
	video.SetProgressStr(task.Progress)
	video.CreatedAt = task.CreatedAt
	video.CompletedAt = task.UpdatedAt
	if task.Status == model.TaskStatusSuccess && strings.TrimSpace(task.GetResultURL()) != "" {
		video.SetMetadata("url", service.BuildSignedVideoProxyURL(task.TaskID, task.UserId))
	}
	if task.Status == model.TaskStatusFailure {
		video.Error = &dto.OpenAIVideoError{Code: "molii_grok_task_failed", Message: "Molii Grok Imagine API task failed"}
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) SanitizeTaskSubmitError(body []byte) string {
	var upstream videoSubmitResponse
	if err := common.Unmarshal(body, &upstream); err == nil && upstream.Error != nil && upstream.Error.Code == "task_pricing_not_configured" {
		return "Molii Grok Imagine API 渠道计费未配置，请联系管理员"
	}
	return "Molii Grok Imagine API request failed"
}

func (a *TaskAdaptor) MapTaskSubmitError(statusCode int, body []byte) *taskdto.TaskError {
	message := a.SanitizeTaskSubmitError(body)
	code := a.SanitizedTaskSubmitCode(body)
	errorType := "provider_error"
	if code == "task_pricing_not_configured" {
		errorType = "provider_configuration_error"
		if statusCode < http.StatusBadRequest || statusCode >= http.StatusInternalServerError {
			statusCode = http.StatusBadRequest
		}
	} else {
		switch statusCode {
		case http.StatusBadRequest:
			code = "invalid_request"
			errorType = "invalid_request_error"
		case http.StatusUnauthorized, http.StatusForbidden:
			code = "invalid_channel_key"
			errorType = "authentication_error"
		case http.StatusTooManyRequests:
			code = "rate_limit_exceeded"
			errorType = "rate_limit_error"
		default:
			if statusCode >= http.StatusInternalServerError {
				code = "provider_unavailable"
			}
		}
	}
	if statusCode < http.StatusBadRequest || statusCode > http.StatusNetworkAuthenticationRequired {
		statusCode = http.StatusBadGateway
	}
	err := errors.New(message)
	return &taskdto.TaskError{
		Code:       code,
		Type:       errorType,
		Message:    message,
		StatusCode: statusCode,
		LocalError: code == "task_pricing_not_configured",
		Error:      err,
	}
}

func (a *TaskAdaptor) MapTaskTransportError(err error) *taskdto.TaskError {
	statusCode := http.StatusBadGateway
	code := "molii_grok_request_failed"
	message := "Molii Grok Imagine API request failed"
	var networkErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkErr) && networkErr.Timeout()) {
		statusCode = http.StatusGatewayTimeout
		code = "task_submit_outcome_unknown"
		message = "Molii Grok Imagine API request failed; submit result is unknown"
	}
	safeErr := errors.New(message)
	return &taskdto.TaskError{
		Code:       code,
		Type:       "provider_error",
		Message:    message,
		StatusCode: statusCode,
		Error:      safeErr,
	}
}

func (a *TaskAdaptor) SanitizedTaskSubmitCode(body []byte) string {
	var upstream videoSubmitResponse
	if err := common.Unmarshal(body, &upstream); err == nil && upstream.Error != nil {
		if upstream.Error.Code == "task_pricing_not_configured" {
			return upstream.Error.Code
		}
	}
	return "molii_grok_request_failed"
}

func (a *TaskAdaptor) SafePollingData(taskResult *relaycommon.TaskInfo) []byte {
	progress := 0
	if taskResult != nil {
		progress, _ = strconv.Atoi(strings.TrimSuffix(taskResult.Progress, "%"))
	}
	status := "in_progress"
	if taskResult != nil {
		status = taskResult.Status
	}
	body, _ := common.Marshal(safeTaskData{Status: status, Progress: progress})
	return body
}

func (a *TaskAdaptor) IsTaskPollingStatusAccepted(statusCode int) bool {
	return statusCode == http.StatusOK || statusCode == http.StatusAccepted
}

func (a *TaskAdaptor) IsPrivateTaskPolling() bool { return true }

func (a *TaskAdaptor) SafePollingError(statusCode int) error {
	return fmt.Errorf("Molii Grok Imagine API polling failed with status %d", statusCode)
}
