package starai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type mediaURL struct {
	URL string `json:"url,omitempty"`
}

type contentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *mediaURL `json:"image_url,omitempty"`
	VideoURL *mediaURL `json:"video_url,omitempty"`
	AudioURL *mediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type tool struct {
	Type string `json:"type,omitempty"`
}

type requestPayload struct {
	Model         string        `json:"model"`
	Content       []contentItem `json:"content"`
	GenerateAudio *bool         `json:"generate_audio,omitempty"`
	Resolution    string        `json:"resolution,omitempty"`
	Ratio         string        `json:"ratio,omitempty"`
	Duration      *int          `json:"duration,omitempty"`
	Watermark     *bool         `json:"watermark,omitempty"`
	Tools         []tool        `json:"tools,omitempty"`
}

type usage struct {
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type nestedTaskPayload struct {
	Status     string `json:"status"`
	Message    string `json:"message"`
	FailReason string `json:"fail_reason"`
	Content    struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Usage *usage `json:"usage"`
	Error any    `json:"error"`
}

type taskPayload struct {
	TaskID     string            `json:"task_id"`
	ID         string            `json:"id"`
	Message    string            `json:"message"`
	Status     string            `json:"status"`
	FailReason string            `json:"fail_reason"`
	ResultURL  string            `json:"result_url"`
	Progress   string            `json:"progress"`
	Usage      *usage            `json:"usage"`
	Error      any               `json:"error"`
	Data       nestedTaskPayload `json:"data"`
}

type responseEnvelope struct {
	Code       any         `json:"code"`
	Message    string      `json:"message"`
	Error      any         `json:"error"`
	FailReason string      `json:"fail_reason"`
	TaskID     string      `json:"task_id"`
	ID         string      `json:"id"`
	Status     string      `json:"status"`
	ResultURL  string      `json:"result_url"`
	Progress   string      `json:"progress"`
	Usage      *usage      `json:"usage"`
	Data       taskPayload `json:"data"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func (a *TaskAdaptor) AllowAutomaticTaskSubmitRetry() bool {
	return false
}

func (a *TaskAdaptor) SanitizeTaskSubmitError(responseBody []byte) string {
	var starResp responseEnvelope
	if err := common.Unmarshal(responseBody, &starResp); err != nil {
		return "StarAI request failed"
	}
	return a.safeErrorMessage(responseMessage(starResp))
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequestAllowEmptyPrompt(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	payload, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := applyExplicitTopLevelDuration(c, req, payload); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_seconds", http.StatusBadRequest)
	}
	if err := validateDuration(payload.Duration); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_seconds", http.StatusBadRequest)
	}
	if err := validateContent(payload.Content); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/video/generations", nil
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
	payload, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil, err
	}
	if err := applyExplicitTopLevelDuration(c, req, payload); err != nil {
		return nil, err
	}
	if err := validateDuration(payload.Duration); err != nil {
		return nil, err
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var starResp responseEnvelope
	if err := common.Unmarshal(responseBody, &starResp); err != nil {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("invalid StarAI response"), "unmarshal_response_body_failed", http.StatusBadGateway)
	}
	if !isSuccessCode(starResp.Code) {
		message := a.safeErrorMessage(responseMessage(starResp))
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("%s", message), "starai_api_error", upstreamErrorStatus(resp.StatusCode))
	}

	upstreamID := firstNonEmpty(starResp.Data.TaskID, starResp.Data.ID, starResp.TaskID, starResp.ID)
	if upstreamID == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("StarAI response did not include a task ID"), "invalid_response", http.StatusBadGateway)
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = info.PublicTaskID
	openAIVideo.TaskID = info.PublicTaskID
	openAIVideo.Model = info.OriginModelName
	openAIVideo.CreatedAt = time.Now().Unix()
	c.JSON(http.StatusOK, openAIVideo)

	return upstreamID, service.SanitizeStarAIResponseBody(responseBody, info.PublicTaskID), nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	requestURL := strings.TrimRight(baseURL, "/") + "/v1/video/generations/" + url.PathEscape(taskID)
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var starResp responseEnvelope
	if err := common.Unmarshal(respBody, &starResp); err != nil {
		return nil, fmt.Errorf("unmarshal StarAI task result failed: %w", err)
	}
	if !isSuccessCode(starResp.Code) {
		return nil, fmt.Errorf("StarAI task query failed: %s", a.safeErrorMessage(responseMessage(starResp)))
	}

	status := firstNonEmpty(starResp.Data.Status, starResp.Data.Data.Status, starResp.Status)
	resultURL := firstNonEmpty(starResp.Data.ResultURL, starResp.Data.Data.Content.VideoURL, starResp.ResultURL)
	reason := firstNonEmpty(starResp.Data.FailReason, messageFromAny(starResp.Data.Data.Error), starResp.Data.Data.FailReason, messageFromAny(starResp.Data.Error), starResp.FailReason)
	resultUsage := usage{}
	if starResp.Data.Data.Usage != nil {
		resultUsage = *starResp.Data.Data.Usage
	} else if starResp.Data.Usage != nil {
		resultUsage = *starResp.Data.Usage
	} else if starResp.Usage != nil {
		resultUsage = *starResp.Usage
	}

	result := &relaycommon.TaskInfo{
		Code:             0,
		TaskID:           firstNonEmpty(starResp.Data.TaskID, starResp.Data.ID, starResp.TaskID, starResp.ID),
		Status:           mapStatus(status),
		Reason:           reason,
		Url:              resultURL,
		Progress:         firstNonEmpty(starResp.Data.Progress, starResp.Progress, defaultProgress(status)),
		CompletionTokens: resultUsage.CompletionTokens,
		TotalTokens:      resultUsage.TotalTokens,
	}
	return result, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var starResp responseEnvelope
	if len(originTask.Data) > 0 {
		if err := common.Unmarshal(originTask.Data, &starResp); err != nil {
			return nil, fmt.Errorf("unmarshal StarAI task data failed: %w", err)
		}
	}

	video := dto.NewOpenAIVideo()
	video.ID = originTask.TaskID
	video.TaskID = originTask.TaskID
	video.Model = originTask.Properties.OriginModelName
	video.Status = originTask.Status.ToVideoStatus()
	video.SetProgressStr(originTask.Progress)
	video.CreatedAt = originTask.CreatedAt
	video.CompletedAt = originTask.UpdatedAt
	resultAvailable := firstNonEmpty(starResp.Data.ResultURL, starResp.Data.Data.Content.VideoURL, starResp.ResultURL, originTask.GetResultURL()) != ""
	if resultAvailable {
		video.SetMetadata("url", taskcommon.BuildProxyURL(originTask.TaskID))
	}
	if originTask.Status == model.TaskStatusFailure {
		reason := firstNonEmpty(originTask.FailReason, starResp.Data.FailReason, starResp.Data.Data.FailReason, messageFromAny(starResp.Data.Data.Error), responseMessage(starResp))
		video.Error = &dto.OpenAIVideoError{Code: "starai_task_failed", Message: a.safeErrorMessage(reason)}
	}
	return common.Marshal(video)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*requestPayload, error) {
	generateAudio := false
	watermark := false
	duration := 5
	payload := &requestPayload{
		Model:         req.Model,
		Content:       make([]contentItem, 0),
		GenerateAudio: &generateAudio,
		Resolution:    "720p",
		Ratio:         "16:9",
		Duration:      &duration,
		Watermark:     &watermark,
	}
	metadata := make(map[string]any, len(req.Metadata))
	for key, value := range req.Metadata {
		if !strings.EqualFold(key, "model") {
			metadata[key] = value
		}
	}
	if err := taskcommon.UnmarshalMetadata(metadata, payload); err != nil {
		return nil, fmt.Errorf("unmarshal metadata failed: %w", err)
	}
	content := make([]contentItem, 0, len(req.Images)+len(payload.Content)+1)
	for _, imageURL := range req.Images {
		if strings.TrimSpace(imageURL) == "" {
			continue
		}
		content = append(content, contentItem{
			Type:     "image_url",
			ImageURL: &mediaURL{URL: imageURL},
		})
	}
	content = append(content, payload.Content...)
	payload.Content = content
	if req.Seconds != "" {
		seconds, err := strconv.Atoi(req.Seconds)
		if err != nil {
			return nil, fmt.Errorf("invalid seconds: %w", err)
		}
		payload.Duration = &seconds
	} else if req.Duration != 0 {
		duration := req.Duration
		payload.Duration = &duration
	}
	if info != nil && info.UpstreamModelName != "" {
		payload.Model = info.UpstreamModelName
	}
	if payload.Model == "" {
		payload.Model = ModelList[0]
	}

	content = payload.Content[:0]
	for _, item := range payload.Content {
		if !strings.EqualFold(item.Type, "text") {
			content = append(content, item)
		}
	}
	payload.Content = content
	if strings.TrimSpace(req.Prompt) != "" {
		payload.Content = append(payload.Content, contentItem{Type: "text", Text: req.Prompt})
	}
	return payload, nil
}

func validateContent(content []contentItem) error {
	for _, item := range content {
		switch strings.ToLower(strings.TrimSpace(item.Type)) {
		case "text":
			if strings.TrimSpace(item.Text) != "" {
				return nil
			}
		case "image_url":
			if item.ImageURL != nil && strings.TrimSpace(item.ImageURL.URL) != "" {
				return nil
			}
		case "video_url":
			if item.VideoURL != nil && strings.TrimSpace(item.VideoURL.URL) != "" {
				return nil
			}
		}
	}
	return fmt.Errorf("prompt or reference image/video is required")
}

func validateDuration(duration *int) error {
	if duration == nil {
		return nil
	}
	if *duration < 0 || *duration > relaycommon.MaxTaskDurationSeconds {
		return fmt.Errorf("duration must be between 0 and %d", relaycommon.MaxTaskDurationSeconds)
	}
	return nil
}

func applyExplicitTopLevelDuration(c *gin.Context, req relaycommon.TaskSubmitReq, payload *requestPayload) error {
	if req.Seconds != "" || !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		return nil
	}
	var rawRequest map[string]any
	if err := common.UnmarshalBodyReusable(c, &rawRequest); err != nil {
		return fmt.Errorf("read explicit duration: %w", err)
	}
	rawDuration, exists := rawRequest["duration"]
	if !exists || rawDuration == nil {
		return nil
	}

	var duration int
	switch value := rawDuration.(type) {
	case float64:
		if value < 0 || value > relaycommon.MaxTaskDurationSeconds || value != float64(int(value)) {
			return fmt.Errorf("duration must be an integer between 0 and %d", relaycommon.MaxTaskDurationSeconds)
		}
		duration = int(value)
	case string:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		duration = parsed
	default:
		return fmt.Errorf("duration must be an integer")
	}
	payload.Duration = &duration
	return nil
}

func isSuccessCode(code any) bool {
	switch value := code.(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "success", "ok", "0", "200", "true":
			return true
		}
	case float64:
		return value == 0 || value == 200
	case int:
		return value == 0 || value == 200
	}
	return false
}

func mapStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "NOT_START", "SUBMITTED":
		return model.TaskStatusSubmitted
	case "PENDING", "QUEUED":
		return model.TaskStatusQueued
	case "PROCESSING", "RUNNING", "IN_PROGRESS":
		return model.TaskStatusInProgress
	case "SUCCESS", "SUCCEEDED":
		return model.TaskStatusSuccess
	case "FAILURE", "FAILED", "CANCELLED", "EXPIRED":
		return model.TaskStatusFailure
	default:
		return model.TaskStatusInProgress
	}
}

func defaultProgress(status string) string {
	switch mapStatus(status) {
	case model.TaskStatusSubmitted:
		return taskcommon.ProgressSubmitted
	case model.TaskStatusQueued:
		return taskcommon.ProgressQueued
	case model.TaskStatusSuccess, model.TaskStatusFailure:
		return taskcommon.ProgressComplete
	default:
		return taskcommon.ProgressInProgress
	}
}

func responseMessage(resp responseEnvelope) string {
	return firstNonEmpty(resp.Message, messageFromAny(resp.Error), resp.FailReason, resp.Data.Message, resp.Data.FailReason, messageFromAny(resp.Data.Error), resp.Data.Data.Message, resp.Data.Data.FailReason, messageFromAny(resp.Data.Data.Error), "StarAI request failed")
}

func messageFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		for _, key := range []string{"message", "error", "fail_reason", "detail"} {
			if message := messageFromAny(typed[key]); message != "" {
				return message
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var sensitiveQueryPattern = regexp.MustCompile(`(?i)(x-tos-signature|x-amz-signature|signature|x-amz-credential|credential|access_token|token|api_key)=([^&\s]+)`)

func (a *TaskAdaptor) safeErrorMessage(message string) string {
	message = sensitiveQueryPattern.ReplaceAllString(message, "$1=***")
	if a.apiKey != "" {
		message = strings.ReplaceAll(message, a.apiKey, "***")
	}
	return common.MaskSensitiveInfo(message)
}

func upstreamErrorStatus(status int) int {
	if status >= 400 {
		return status
	}
	return http.StatusBadGateway
}
