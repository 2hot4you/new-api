package starai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	"github.com/QuantumNous/new-api/setting/ratio_setting"

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
	ToolUsage        struct {
		WebSearch int `json:"web_search"`
	} `json:"tool_usage"`
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
	ID         any               `json:"id"`
	Message    string            `json:"message"`
	Status     string            `json:"status"`
	FailReason string            `json:"fail_reason"`
	ResultURL  string            `json:"result_url"`
	Progress   any               `json:"progress"`
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
	ID         any         `json:"id"`
	Status     string      `json:"status"`
	ResultURL  string      `json:"result_url"`
	Progress   any         `json:"progress"`
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
	payload, err := a.convertToRequestPayload(c, &req, info)
	if err != nil {
		if errors.Is(err, service.ErrStarAIAssetExpired) {
			return service.TaskErrorWrapperLocal(err, "temporary_asset_expired", http.StatusBadRequest)
		}
		if errors.Is(err, service.ErrStarAIAssetVerify) {
			return service.TaskErrorWrapperLocal(err, "temporary_asset_verification_failed", http.StatusBadGateway)
		}
		if errors.Is(err, service.ErrStarAIAssetNotReady) {
			return service.TaskErrorWrapperLocal(err, "temporary_asset_not_ready", http.StatusBadRequest)
		}
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := validateDuration(payload.Duration); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_seconds", http.StatusBadRequest)
	}
	if err := validatePayload(payload); err != nil {
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
	payload, err := a.convertToRequestPayload(c, &req, info)
	if err != nil {
		return nil, err
	}
	if err := validateDuration(payload.Duration); err != nil {
		return nil, err
	}
	if err := validatePayload(payload); err != nil {
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
	if err := decodeStarAIResponse(responseBody, &starResp); err != nil {
		contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
		return "", nil, service.TaskErrorWrapper(
			fmt.Errorf("invalid StarAI response (status=%d, content-type=%q, bytes=%d)", resp.StatusCode, contentType, len(responseBody)),
			"unmarshal_response_body_failed", http.StatusBadGateway,
		)
	}
	if starResp.Code != nil && !isSuccessCode(starResp.Code) {
		message := a.safeErrorMessage(responseMessage(starResp))
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("%s", message), "starai_api_error", upstreamErrorStatus(resp.StatusCode))
	}

	upstreamID := firstNonEmpty(starResp.Data.TaskID, stringValue(starResp.Data.ID), starResp.TaskID, stringValue(starResp.ID))
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

func decodeStarAIResponse(body []byte, target any) error {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf}))
	if err := common.Unmarshal(trimmed, target); err == nil {
		return nil
	}

	text := strings.TrimSpace(string(trimmed))
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			text = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
			if err := common.Unmarshal([]byte(text), target); err == nil {
				return nil
			}
		}
	}

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		if err := common.Unmarshal([]byte(payload), target); err == nil {
			return nil
		}
	}
	return fmt.Errorf("response is not valid JSON")
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
	if starResp.Code != nil && !isSuccessCode(starResp.Code) {
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
		TaskID:           firstNonEmpty(starResp.Data.TaskID, stringValue(starResp.Data.ID), starResp.TaskID, stringValue(starResp.ID)),
		Status:           mapStatus(status),
		Reason:           reason,
		Url:              resultURL,
		Progress:         firstNonEmpty(stringValue(starResp.Data.Progress), stringValue(starResp.Progress), defaultProgress(status)),
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
		video.SetMetadata("url", service.BuildSignedVideoProxyURL(originTask.TaskID, originTask.UserId))
	}
	resultUsage := firstUsage(starResp)
	if resultUsage != nil {
		video.Usage = &dto.OpenAIVideoUsage{
			CompletionTokens: resultUsage.CompletionTokens,
			TotalTokens:      resultUsage.TotalTokens,
			ToolUsage: dto.OpenAIVideoToolUsage{
				WebSearch: resultUsage.ToolUsage.WebSearch,
			},
		}
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

// EstimateBilling applies StarAI's Seedance price tier for the requested
// output resolution and whether the request contains reference video input.
// Duration is intentionally not multiplied here: StarAI's returned
// total_tokens already reflects the generated video quantity.
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	payload, err := a.convertToRequestPayload(c, &req, info)
	if err != nil {
		return nil
	}
	hasVideo := false
	for _, item := range payload.Content {
		if strings.EqualFold(strings.TrimSpace(item.Type), "video_url") {
			hasVideo = true
			break
		}
	}
	price, ok := ratio_setting.GetStarAIVideoPrice(info.OriginModelName, payload.Resolution, hasVideo)
	if !ok {
		return nil
	}
	modelRatio, ok, _ := ratio_setting.GetModelRatio(info.OriginModelName)
	if !ok {
		modelRatio, ok = ratio_setting.GetDefaultModelRatioMap()[info.OriginModelName]
	}
	if !ok || modelRatio <= 0 {
		return nil
	}
	// A model ratio of 1 represents 2 platform-currency units per 1M tokens.
	// Reverse the configured absolute price into an OtherRatio so the value in
	// the StarAI form remains authoritative even if the generic ratio changes.
	priceRatio := price / (2 * modelRatio)
	width, height := seedanceDimensions(payload.Resolution, payload.Ratio)
	seconds := 5
	if payload.Duration != nil && *payload.Duration > 0 {
		seconds = *payload.Duration
	}
	const fps = 24
	estimatedTokens := int(math.Ceil(float64(width*height*(fps*seconds+1)) / 1024.0))
	info.EstimatedVideoTokens = estimatedTokens
	info.EstimatedVideoPrice = float64(estimatedTokens) * price / 1_000_000
	info.EstimatedVideoWidth = width
	info.EstimatedVideoHeight = height
	info.EstimatedVideoFPS = fps
	info.EstimatedVideoSeconds = seconds
	info.EstimatedVideoResolution = payload.Resolution
	info.EstimatedVideoRatio = payload.Ratio
	info.EstimatedVideoHasInput = hasVideo
	info.EstimatedVideoUnitPrice = price
	inputTier := "no-video-input"
	if hasVideo {
		inputTier = "video-input"
	}
	return map[string]float64{
		fmt.Sprintf("seedance-%s-%s", strings.ToLower(payload.Resolution), inputTier): priceRatio,
	}
}

func seedanceDimensions(resolution, ratio string) (int, int) {
	// StarAI documents exact values for 480p/720p. 1080p uses the
	// corresponding standard canvas. adaptive cannot be known before task
	// completion, so it deliberately falls back to the common 16:9 canvas.
	if ratio == "adaptive" || ratio == "" {
		ratio = "16:9"
	}
	table := map[string]map[string][2]int{
		"480p":  {"16:9": {864, 496}, "4:3": {752, 560}, "1:1": {640, 640}, "3:4": {560, 752}, "9:16": {496, 864}, "21:9": {992, 432}},
		"720p":  {"16:9": {1280, 720}, "4:3": {1112, 834}, "1:1": {960, 960}, "3:4": {834, 1112}, "9:16": {720, 1280}, "21:9": {1470, 630}},
		"1080p": {"16:9": {1920, 1080}, "4:3": {1440, 1080}, "1:1": {1080, 1080}, "3:4": {1080, 1440}, "9:16": {1080, 1920}, "21:9": {2520, 1080}},
		"4k":    {"16:9": {3840, 2160}, "4:3": {2880, 2160}, "1:1": {2160, 2160}, "3:4": {2160, 2880}, "9:16": {2160, 3840}, "21:9": {5040, 2160}},
	}
	byRatio, ok := table[strings.ToLower(strings.TrimSpace(resolution))]
	if !ok {
		return 0, 0
	}
	if size, ok := byRatio[ratio]; ok {
		return size[0], size[1]
	}
	size := byRatio["16:9"]
	return size[0], size[1]
}

func (a *TaskAdaptor) convertToRequestPayload(c *gin.Context, req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*requestPayload, error) {
	generateAudio := true
	watermark := false
	duration := 5
	payload := &requestPayload{
		Model:         req.Model,
		Content:       make([]contentItem, 0),
		GenerateAudio: &generateAudio,
		Resolution:    "720p",
		Ratio:         "adaptive",
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
	if err := applyTopLevelFields(c, payload); err != nil {
		return nil, err
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

	if strings.TrimSpace(req.Prompt) != "" {
		payload.Content = append(payload.Content, contentItem{Type: "text", Text: req.Prompt})
	}
	if err := a.resolveTemporaryAssets(c, payload, info); err != nil {
		return nil, err
	}
	return payload, nil
}

const starAIResolvedAssetCacheKey = "starai_resolved_asset_uris"

func (a *TaskAdaptor) resolveTemporaryAssets(c *gin.Context, payload *requestPayload, info *relaycommon.RelayInfo) error {
	resolvedAssets := make(map[string]string)
	if cached, ok := c.Get(starAIResolvedAssetCacheKey); ok {
		if typed, valid := cached.(map[string]string); valid {
			resolvedAssets = typed
		}
	}
	userID := c.GetInt("id")
	proxy := ""
	if info != nil {
		userID = info.UserId
		proxy = info.ChannelSetting.Proxy
	}
	for i := range payload.Content {
		item := &payload.Content[i]
		for _, media := range []*mediaURL{item.ImageURL, item.VideoURL, item.AudioURL} {
			if media == nil || !strings.HasPrefix(media.URL, "asset://") {
				continue
			}
			if resolved, ok := resolvedAssets[media.URL]; ok {
				media.URL = resolved
				continue
			}
			originalURI := media.URL
			resolved, err := service.ResolveStarAIAssetURI(c.Request.Context(), originalURI, userID, service.StarAIAssetVerificationConfig{
				BaseURL: a.baseURL,
				APIKey:  a.apiKey,
				Proxy:   proxy,
			})
			if err != nil {
				return fmt.Errorf("invalid temporary asset: %w", err)
			}
			resolvedAssets[originalURI] = resolved
			media.URL = resolved
		}
	}
	c.Set(starAIResolvedAssetCacheKey, resolvedAssets)
	return nil
}

func validateDuration(duration *int) error {
	if duration == nil {
		return nil
	}
	if *duration != -1 && (*duration < 4 || *duration > maxDurationSeconds) {
		return fmt.Errorf("duration must be -1 or between 4 and %d", maxDurationSeconds)
	}
	return nil
}

func applyTopLevelFields(c *gin.Context, payload *requestPayload) error {
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		return nil
	}
	var topLevel struct {
		Content       *[]contentItem `json:"content"`
		GenerateAudio *bool          `json:"generate_audio"`
		Resolution    *string        `json:"resolution"`
		Ratio         *string        `json:"ratio"`
		Duration      *int           `json:"duration"`
		Watermark     *bool          `json:"watermark"`
		Tools         *[]tool        `json:"tools"`
	}
	if err := common.UnmarshalBodyReusable(c, &topLevel); err != nil {
		return fmt.Errorf("read StarAI request fields: %w", err)
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
	return nil
}

func validatePayload(payload *requestPayload) error {
	if payload.Resolution != "480p" && payload.Resolution != "720p" && payload.Resolution != "1080p" && payload.Resolution != "4k" {
		return fmt.Errorf("resolution must be one of 480p, 720p, 1080p, or 4k")
	}
	if payload.Model == ModelList[1] && (payload.Resolution == "1080p" || payload.Resolution == "4k") {
		return fmt.Errorf("%s is not supported by %s", payload.Resolution, ModelList[1])
	}
	validRatios := map[string]struct{}{"16:9": {}, "4:3": {}, "1:1": {}, "3:4": {}, "9:16": {}, "21:9": {}, "adaptive": {}}
	if _, ok := validRatios[payload.Ratio]; !ok {
		return fmt.Errorf("unsupported ratio %q", payload.Ratio)
	}
	for _, item := range payload.Tools {
		if item.Type != "web_search" {
			return fmt.Errorf("unsupported tool type %q", item.Type)
		}
	}

	var textCount, imageCount, videoCount, audioCount int
	var firstFrameCount, lastFrameCount, referenceImageCount int
	for _, item := range payload.Content {
		switch strings.ToLower(strings.TrimSpace(item.Type)) {
		case "text":
			if strings.TrimSpace(item.Text) == "" {
				return fmt.Errorf("text content must not be empty")
			}
			textCount++
		case "image_url":
			if item.ImageURL == nil || strings.TrimSpace(item.ImageURL.URL) == "" {
				return fmt.Errorf("image_url content requires a URL")
			}
			imageCount++
			switch item.Role {
			case "", "first_frame":
				firstFrameCount++
			case "last_frame":
				lastFrameCount++
			case "reference_image":
				referenceImageCount++
			default:
				return fmt.Errorf("unsupported image role %q", item.Role)
			}
		case "video_url":
			if item.VideoURL == nil || strings.TrimSpace(item.VideoURL.URL) == "" {
				return fmt.Errorf("video_url content requires a URL")
			}
			if item.Role != "reference_video" {
				return fmt.Errorf("video role must be reference_video")
			}
			videoCount++
		case "audio_url":
			if item.AudioURL == nil || strings.TrimSpace(item.AudioURL.URL) == "" {
				return fmt.Errorf("audio_url content requires a URL")
			}
			if item.Role != "reference_audio" {
				return fmt.Errorf("audio role must be reference_audio")
			}
			audioCount++
		default:
			return fmt.Errorf("unsupported content type %q", item.Type)
		}
	}
	if textCount+imageCount+videoCount+audioCount == 0 || (imageCount+videoCount == 0 && textCount == 0) {
		return fmt.Errorf("prompt or reference image/video is required")
	}
	if imageCount > 9 || videoCount > 3 || audioCount > 3 {
		return fmt.Errorf("at most 9 images, 3 videos, and 3 audio files are supported")
	}
	frameScene := firstFrameCount > 0 || lastFrameCount > 0
	referenceScene := referenceImageCount > 0 || videoCount > 0 || audioCount > 0
	if frameScene && referenceScene {
		return fmt.Errorf("frame-based and multimodal reference content cannot be mixed")
	}
	if frameScene {
		if firstFrameCount != 1 || lastFrameCount > 1 || imageCount > 2 {
			return fmt.Errorf("frame-based generation requires one first frame and at most one last frame")
		}
	}
	if referenceScene {
		if imageCount != referenceImageCount {
			return fmt.Errorf("multimodal images must use role reference_image")
		}
		if audioCount > 0 && imageCount+videoCount == 0 {
			return fmt.Errorf("audio requires at least one reference image or video")
		}
	}
	return nil
}

func firstUsage(resp responseEnvelope) *usage {
	if resp.Data.Data.Usage != nil {
		return resp.Data.Data.Usage
	}
	if resp.Data.Usage != nil {
		return resp.Data.Usage
	}
	return resp.Usage
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

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
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
