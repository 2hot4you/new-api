package bytedanceseedance

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/seedanceprotocol"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// TaskAdaptor talks to Molii's public video API. It never stores or exposes
// diagnostics from the provider behind that API.
type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

var _ channel.TaskAdaptor = (*TaskAdaptor)(nil)
var _ service.PrivateTaskPollingAdaptor = (*TaskAdaptor)(nil)

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info == nil {
		return
	}
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func (a *TaskAdaptor) GetModelList() []string              { return seedanceprotocol.SupportedModels() }
func (a *TaskAdaptor) GetChannelName() string              { return ChannelName }
func (a *TaskAdaptor) AllowAutomaticTaskSubmitRetry() bool { return false }

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequestAllowEmptyPrompt(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	request, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	payload, err := a.payload(c, &request, info)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := seedanceprotocol.ValidateDuration(payload.Model, payload.Duration); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_seconds", http.StatusBadRequest)
	}
	if err := seedanceprotocol.ValidatePayload(payload); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if info != nil {
		for _, item := range payload.Content {
			switch item.Type {
			case "image_url":
				info.VideoInputImageCount++
			case "video_url":
				info.VideoInputVideoCount++
			case "audio_url":
				info.VideoInputAudioCount++
			}
		}
		info.VideoInputMediaCountsAvailable = true
	}
	return nil
}

func (a *TaskAdaptor) payload(c *gin.Context, req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*seedanceprotocol.Payload, error) {
	generateAudio, watermark, duration := true, false, 5
	payload := &seedanceprotocol.Payload{
		Model: req.Model, Content: []seedanceprotocol.ContentItem{},
		GenerateAudio: &generateAudio, Resolution: "720p", Ratio: "adaptive",
		Duration: &duration, Watermark: &watermark,
	}
	metadata := make(map[string]any, len(req.Metadata))
	for key, value := range req.Metadata {
		if !strings.EqualFold(key, "model") {
			metadata[key] = value
		}
	}
	if err := taskcommon.UnmarshalMetadata(metadata, payload); err != nil {
		return nil, err
	}
	if c != nil && c.Request != nil && strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		var topLevel struct {
			Content       *[]seedanceprotocol.ContentItem `json:"content"`
			GenerateAudio *bool                           `json:"generate_audio"`
			Resolution    *string                         `json:"resolution"`
			Ratio         *string                         `json:"ratio"`
			Duration      *int                            `json:"duration"`
			Watermark     *bool                           `json:"watermark"`
			Tools         *[]seedanceprotocol.Tool        `json:"tools"`
		}
		if err := common.UnmarshalBodyReusable(c, &topLevel); err != nil {
			return nil, fmt.Errorf("invalid request fields: %w", err)
		}
		if topLevel.Content != nil {
			payload.Content = *topLevel.Content
		}
		if topLevel.GenerateAudio != nil {
			payload.GenerateAudio = topLevel.GenerateAudio
		}
		if topLevel.Resolution != nil {
			payload.Resolution = *topLevel.Resolution
		}
		if topLevel.Ratio != nil {
			payload.Ratio = *topLevel.Ratio
		}
		if topLevel.Duration != nil {
			payload.Duration = topLevel.Duration
		}
		if topLevel.Watermark != nil {
			payload.Watermark = topLevel.Watermark
		}
		if topLevel.Tools != nil {
			payload.Tools = *topLevel.Tools
		}
	}
	for _, image := range req.Images {
		if strings.TrimSpace(image) != "" {
			payload.Content = append(payload.Content, seedanceprotocol.ContentItem{Type: "image_url", ImageURL: &seedanceprotocol.MediaURL{URL: image}})
		}
	}
	if req.Seconds != "" {
		seconds, err := strconv.Atoi(req.Seconds)
		if err != nil {
			return nil, fmt.Errorf("invalid seconds: %w", err)
		}
		payload.Duration = &seconds
	} else if req.Duration != 0 {
		payload.Duration = &req.Duration
	}
	if info != nil && info.UpstreamModelName != "" {
		payload.Model = info.UpstreamModelName
	}
	if payload.Model == "" {
		payload.Model = ModelList[0]
	}
	if !isSupportedModel(payload.Model) {
		return nil, errors.New("unsupported Seedance model")
	}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		payload.Content = append(payload.Content, seedanceprotocol.ContentItem{Type: "text", Text: prompt})
	}
	return payload, nil
}

func isSupportedModel(value string) bool {
	for _, name := range ModelList {
		if value == name {
			return true
		}
	}
	return false
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		return "", errors.New("Molii base URL is required")
	}
	return a.baseURL + "/v1/video/generations", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, request *http.Request, _ *relaycommon.RelayInfo) error {
	request.Header.Set("Authorization", "Bearer "+a.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	payload, err := a.payload(c, &req, info)
	if err != nil {
		return nil, err
	}
	if err := seedanceprotocol.ValidateDuration(payload.Model, payload.Duration); err != nil {
		return nil, err
	}
	if err := seedanceprotocol.ValidatePayload(payload); err != nil {
		return nil, err
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, body)
}

func (a *TaskAdaptor) ParseResponse(_ *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *taskdto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, service.TaskErrorWrapper(errors.New("cannot read Molii video response"), "read_response_body_failed", http.StatusBadGateway)
	}
	_ = resp.Body.Close()
	var envelope seedanceprotocol.ResponseEnvelope
	if err := seedanceprotocol.DecodeResponse(body, &envelope); err != nil {
		return nil, service.TaskErrorWrapper(errors.New("invalid Molii video response"), "invalid_response", http.StatusBadGateway)
	}
	if !successCode(envelope.Code) {
		return nil, service.TaskErrorWrapper(errors.New("Molii video request failed"), "molii_video_api_error", http.StatusBadGateway)
	}
	id := first(envelope.Data.TaskID, stringValue(envelope.Data.ID), envelope.TaskID, stringValue(envelope.ID))
	if !isPublicTaskID(id) {
		return nil, service.TaskErrorWrapper(errors.New("Molii video response omitted a valid public task ID"), "invalid_response", http.StatusBadGateway)
	}
	publicID, originModel := "", ""
	if info != nil {
		publicID, originModel = info.PublicTaskID, info.OriginModelName
	}
	client := dto.NewOpenAIVideo()
	client.ID, client.TaskID, client.Model = publicID, publicID, originModel
	client.CreatedAt = time.Now().Unix()
	safe, _ := common.Marshal(struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Model  string `json:"model,omitempty"`
	}{publicID, "queued", originModel})
	return &channel.TaskSubmitResponse{UpstreamTaskID: id, TaskData: safe, ClientResponse: client}, nil
}

func (a *TaskAdaptor) SanitizeTaskSubmitError(_ []byte) string { return "Molii video request failed" }

func (a *TaskAdaptor) FetchTask(baseURL, key string, task *model.Task, proxy string) (*http.Response, error) {
	if task == nil || strings.TrimSpace(task.GetUpstreamTaskID()) == "" {
		return nil, errors.New("missing Molii task ID")
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/v1/video/generations/" + url.PathEscape(task.GetUpstreamTaskID())
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(request)
}

func (a *TaskAdaptor) ParseTaskResult(_ *model.Task, _ *http.Response, body []byte) (*relaycommon.TaskInfo, error) {
	var envelope seedanceprotocol.ResponseEnvelope
	if err := seedanceprotocol.DecodeResponse(body, &envelope); err != nil {
		return nil, errors.New("invalid Molii video task response")
	}
	if !successCode(envelope.Code) {
		return nil, errors.New("Molii video task query failed")
	}
	status := first(envelope.Data.Status, envelope.Data.Data.Status, envelope.Status)
	if status == "" {
		return nil, errors.New("Molii video task response omitted status")
	}
	mappedStatus := mapStatus(status)
	if mappedStatus == "" {
		return nil, errors.New("Molii video task response has unknown status")
	}
	usage := envelope.Data.Data.Usage
	if usage == nil {
		usage = envelope.Data.Usage
	}
	if usage == nil {
		usage = envelope.Usage
	}
	result := &relaycommon.TaskInfo{
		Status:   mappedStatus,
		Progress: progress(status),
		// Molii may include a private signed result URL. The content proxy
		// must obtain media separately from the authenticated public API.
		ActualDurationSeconds: envelope.Data.Data.Duration,
		ActualResolution:      safeResolution(envelope.Data.Data.Resolution),
	}
	if usage != nil {
		result.CompletionTokens, result.TotalTokens = usage.CompletionTokens, usage.TotalTokens
	}
	if result.Status == model.TaskStatusFailure {
		result.Reason = "Molii video task failed"
	}
	return result, nil
}

func (a *TaskAdaptor) IsPrivateTaskPolling() bool { return true }
func (a *TaskAdaptor) IsTaskPollingStatusAccepted(code int) bool {
	return code == http.StatusOK || code == http.StatusAccepted
}
func (a *TaskAdaptor) SafePollingError(code int) error {
	return fmt.Errorf("Molii video polling failed with status %d", code)
}

func (a *TaskAdaptor) SafePollingData(result *relaycommon.TaskInfo) []byte {
	if result == nil {
		return []byte(`{}`)
	}
	data, _ := common.Marshal(struct {
		Status           string  `json:"status"`
		Progress         string  `json:"progress,omitempty"`
		CompletionTokens int     `json:"completion_tokens,omitempty"`
		TotalTokens      int     `json:"total_tokens,omitempty"`
		Duration         float64 `json:"duration,omitempty"`
		Resolution       string  `json:"resolution,omitempty"`
	}{result.Status, result.Progress, result.CompletionTokens, result.TotalTokens, result.ActualDurationSeconds, result.ActualResolution})
	return data
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	video := dto.NewOpenAIVideo()
	video.ID, video.TaskID = task.TaskID, task.TaskID
	video.Model = task.Properties.OriginModelName
	video.Status = task.Status.ToVideoStatus()
	video.SetProgressStr(task.Progress)
	video.CreatedAt, video.CompletedAt = task.CreatedAt, task.UpdatedAt
	var safeData struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	if len(task.Data) > 0 && common.Unmarshal(task.Data, &safeData) == nil && (safeData.CompletionTokens != 0 || safeData.TotalTokens != 0) {
		video.Usage = &dto.OpenAIVideoUsage{CompletionTokens: safeData.CompletionTokens, TotalTokens: safeData.TotalTokens}
	}
	if task.Status == model.TaskStatusFailure {
		video.Error = &dto.OpenAIVideoError{Code: "molii_video_failed", Message: "Molii video task failed"}
	}
	return common.Marshal(video)
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isPublicTaskID(value string) bool {
	if !strings.HasPrefix(value, "task_") || len(value) <= len("task_") {
		return false
	}
	for _, ch := range value[len("task_"):] {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	default:
		return ""
	}
}

func successCode(code any) bool {
	if code == nil {
		return true
	}
	switch v := code.(type) {
	case string:
		return strings.EqualFold(v, "success") || strings.EqualFold(v, "ok") || v == "0" || v == "200"
	case float64:
		return v == 0 || v == 200
	case bool:
		return v
	default:
		return false
	}
}

func mapStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "NOT_START", "SUBMITTED":
		return model.TaskStatusSubmitted
	case "PENDING", "QUEUED":
		return model.TaskStatusQueued
	case "PROCESSING", "RUNNING", "IN_PROGRESS":
		return model.TaskStatusInProgress
	case "SUCCESS", "SUCCEEDED", "COMPLETED":
		return model.TaskStatusSuccess
	case "FAILURE", "FAILED", "CANCELLED", "EXPIRED":
		return model.TaskStatusFailure
	default:
		return ""
	}
}

func progress(status string) string {
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

func safeResolution(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "480p", "720p", "1080p", "4k":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}
