package moliigrok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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
	apiKey             string
	baseURL            string
	videoProber        service.UserVideoProber
	persistVideoResult func(context.Context, service.GrokResultStoreRequest) (*service.StoredObject, bool, error)
	resolveUserFile    func(context.Context, int, string, model.MoliiFileMediaType) (*model.MoliiFile, string, error)
}

const unresolvedFileIDMessage = "Molii file_id could not be resolved"

var supportedVideoAspectRatios = map[string]struct{}{
	"1:1": {}, "16:9": {}, "9:16": {}, "4:3": {}, "3:4": {}, "3:2": {}, "2:3": {},
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

func (a *TaskAdaptor) userFileResolver() func(context.Context, int, string, model.MoliiFileMediaType) (*model.MoliiFile, string, error) {
	if a != nil && a.resolveUserFile != nil {
		return a.resolveUserFile
	}
	return service.ResolveUserFile
}

func (a *TaskAdaptor) PersistTaskResult(ctx context.Context, task *model.Task, result *relaycommon.TaskInfo, completedAt time.Time) (*model.TaskStoredResult, error) {
	if task == nil || result == nil || task.UserId <= 0 || strings.TrimSpace(task.TaskID) == "" || completedAt.IsZero() || strings.TrimSpace(result.Url) == "" {
		return nil, errors.New("Molii Grok Imagine API video result metadata is invalid")
	}
	keyAnchor := completedAt
	if task.CreatedAt > 0 {
		keyAnchor = time.Unix(task.CreatedAt, 0).UTC()
	} else if task.SubmitTime > 0 {
		keyAnchor = time.Unix(task.SubmitTime, 0).UTC()
	}
	persist := service.PersistGrokVideoResultWithStatus
	if a != nil && a.persistVideoResult != nil {
		persist = a.persistVideoResult
	}
	stored, _, err := persist(ctx, service.GrokResultStoreRequest{
		SourceURL:      strings.TrimSpace(result.Url),
		UserID:         task.UserId,
		MediaType:      "video",
		MIMEType:       "video/mp4",
		IdempotencyKey: task.TaskID,
		CreatedAt:      completedAt,
		KeyAnchor:      keyAnchor,
	})
	if err != nil {
		return nil, errors.New("Molii Grok Imagine API video result persistence failed")
	}
	if stored == nil || strings.TrimSpace(stored.ObjectKey) == "" || stored.MIMEType != "video/mp4" || stored.ExpiresAt <= completedAt.Unix() {
		return nil, errors.New("Molii Grok Imagine API video result persistence returned invalid metadata")
	}
	return &model.TaskStoredResult{
		ObjectKey:       stored.ObjectKey,
		MIMEType:        stored.MIMEType,
		Size:            stored.Size,
		ExpiresAt:       stored.ExpiresAt,
		DurationSeconds: result.ActualDurationSeconds,
	}, nil
}

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
	isExtension := strings.HasSuffix(c.Request.URL.Path, "/videos/extensions")
	if taskErr := validateTaskMediaShape(&req, modelName, isEdit, isExtension); taskErr != nil {
		return taskErr
	}
	if taskErr := a.resolveTaskFileIDs(c, info, &req); taskErr != nil {
		return taskErr
	}
	if isExtension {
		if modelName != LegacyVideoModel {
			return service.TaskErrorWrapperLocal(errors.New("video extension requires grok-imagine-video"), "invalid_model", http.StatusBadRequest)
		}
		if strings.TrimSpace(req.Video) == "" {
			return service.TaskErrorWrapperLocal(errors.New("video is required"), "invalid_video", http.StatusBadRequest)
		}
		if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 || len(req.ReferenceImages) > 0 {
			return service.TaskErrorWrapperLocal(errors.New("video extension cannot be combined with image inputs"), "invalid_request", http.StatusBadRequest)
		}
		if strings.TrimSpace(req.AspectRatio) != "" || strings.TrimSpace(req.Resolution) != "" {
			return service.TaskErrorWrapperLocal(errors.New("aspect_ratio and resolution are not supported for video extensions"), "invalid_request", http.StatusBadRequest)
		}
		if req.Duration == 0 {
			req.Duration = 6
		}
		if req.Duration < 2 || req.Duration > 10 {
			return service.TaskErrorWrapperLocal(errors.New("duration must be between 2 and 10 seconds for video extensions"), "invalid_duration", http.StatusBadRequest)
		}
		probe, taskErr := a.probeInputVideo(c, info, req.Video)
		if taskErr != nil {
			return taskErr
		}
		if probe.DurationSeconds < 2 || probe.DurationSeconds > 15 {
			return service.TaskErrorWrapperLocal(errors.New("input video duration must be between 2 and 15 seconds"), "invalid_video_duration", http.StatusBadRequest)
		}
		resolution := strings.ToLower(strings.TrimSpace(probe.ResolutionTier))
		if resolution == "1080p" {
			resolution = "720p"
		}
		if resolution != "480p" && resolution != "720p" {
			return service.TaskErrorWrapperLocal(errors.New("input video resolution is not supported"), "invalid_video_resolution", http.StatusBadRequest)
		}
		c.Set("task_request", req)
		info.Action = videoExtensionAction
		info.InputVideoDurationSeconds = probe.DurationSeconds
		info.InputVideoResolutionTier = resolution
		info.InputVideoResolutionSource = relaycommon.GrokVideoResolutionSourceInputProbeV1
		info.EstimatedVideoSeconds = req.Duration
		info.EstimatedVideoResolution = resolution
		info.EstimatedVideoHasInput = true
		return nil
	}
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
		probe, taskErr := a.probeInputVideo(c, info, req.Video)
		if taskErr != nil {
			return taskErr
		}
		if probe.DurationSeconds <= 0 || probe.DurationSeconds > 8.7 {
			return service.TaskErrorWrapperLocal(errors.New("video duration must be greater than 0 and at most 8.7 seconds"), "invalid_video_duration", http.StatusBadRequest)
		}
		c.Set("task_request", req)
		info.Action = videoEditAction
		info.InputVideoDurationSeconds = probe.DurationSeconds
		info.InputVideoResolutionTier = probe.ResolutionTier
		info.InputVideoResolutionSource = relaycommon.GrokVideoResolutionSourceInputProbeV1
		info.EstimatedVideoSeconds = int(math.Ceil(probe.DurationSeconds))
		info.EstimatedVideoResolution = probe.ResolutionTier
		info.EstimatedVideoHasInput = true
		return nil
	}
	if len(req.ReferenceImages) > 0 {
		if modelName != VideoModel {
			return service.TaskErrorWrapperLocal(errors.New("reference_images require grok-imagine-video-1.5"), "invalid_model", http.StatusBadRequest)
		}
		if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 || strings.TrimSpace(req.Video) != "" {
			return service.TaskErrorWrapperLocal(errors.New("reference_images cannot be combined with image or video"), "invalid_request", http.StatusBadRequest)
		}
		if len(req.ReferenceImages) > 7 {
			return service.TaskErrorWrapperLocal(errors.New("reference_images must contain at most 7 images"), "invalid_reference_images", http.StatusBadRequest)
		}
		for _, reference := range req.ReferenceImages {
			if strings.TrimSpace(reference) == "" {
				return service.TaskErrorWrapperLocal(errors.New("reference_images entries must contain a URL"), "invalid_reference_images", http.StatusBadRequest)
			}
		}
		if req.Duration == 0 {
			req.Duration = 5
		}
		if req.Duration < 1 || req.Duration > 15 {
			return service.TaskErrorWrapperLocal(errors.New("duration must be between 1 and 15 seconds"), "invalid_duration", http.StatusBadRequest)
		}
		if strings.TrimSpace(req.AspectRatio) == "" {
			req.AspectRatio = "16:9"
		} else {
			req.AspectRatio = strings.TrimSpace(req.AspectRatio)
		}
		if strings.TrimSpace(req.Resolution) == "" {
			req.Resolution = "480p"
		} else {
			req.Resolution = strings.ToLower(strings.TrimSpace(req.Resolution))
		}
		if _, ok := supportedVideoAspectRatios[req.AspectRatio]; !ok {
			return service.TaskErrorWrapperLocal(errors.New("aspect_ratio is not supported"), "invalid_aspect_ratio", http.StatusBadRequest)
		}
		if req.Resolution != "480p" && req.Resolution != "720p" {
			return service.TaskErrorWrapperLocal(errors.New("reference_images support at most 720p"), "invalid_resolution", http.StatusBadRequest)
		}
		if _, _, _, ok := ratio_setting.GetMoliiGrokVideoPrices(modelName, req.Resolution); !ok {
			return service.TaskErrorWrapperLocal(errors.New("resolution is not supported by the selected model"), "invalid_resolution", http.StatusBadRequest)
		}
		c.Set("task_request", req)
		info.Action = referenceToVideoAction
		info.EstimatedVideoSeconds = req.Duration
		info.EstimatedVideoRatio = req.AspectRatio
		info.EstimatedVideoResolution = req.Resolution
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
	} else {
		req.AspectRatio = strings.TrimSpace(req.AspectRatio)
	}
	if strings.TrimSpace(req.Resolution) == "" {
		req.Resolution = "480p"
	} else {
		req.Resolution = strings.ToLower(strings.TrimSpace(req.Resolution))
	}
	if _, ok := supportedVideoAspectRatios[req.AspectRatio]; !ok {
		return service.TaskErrorWrapperLocal(errors.New("aspect_ratio is not supported"), "invalid_aspect_ratio", http.StatusBadRequest)
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

func (a *TaskAdaptor) probeInputVideo(c *gin.Context, info *relaycommon.RelayInfo, value string) (*service.MediaProbeResult, *taskdto.TaskError) {
	if info != nil && info.InputVideoResolutionSource == relaycommon.GrokVideoResolutionSourceInputProbeV1 &&
		info.InputVideoDurationSeconds > 0 && strings.TrimSpace(info.InputVideoResolutionTier) != "" {
		return &service.MediaProbeResult{
			DurationSeconds: info.InputVideoDurationSeconds,
			ResolutionTier:  info.InputVideoResolutionTier,
			MIMEType:        "video/mp4",
		}, nil
	}
	videoProber := a.videoProber
	if videoProber == nil {
		videoProber = service.UserVideoProbeFunc(service.ProbeUserVideo)
	}
	probe, err := videoProber.ProbeUserVideo(c.Request.Context(), service.MediaSource{URL: strings.TrimSpace(value)})
	if err != nil || probe == nil {
		return nil, service.TaskErrorWrapperLocal(errors.New("video must be a valid MP4"), "invalid_video", http.StatusBadRequest)
	}
	return probe, nil
}

func validateTaskMediaShape(req *relaycommon.TaskSubmitReq, modelName string, isEdit, isExtension bool) *taskdto.TaskError {
	if req == nil {
		return service.TaskErrorWrapperLocal(errors.New("request is missing"), "invalid_request", http.StatusBadRequest)
	}
	if len(req.Images) > 1 {
		return service.TaskErrorWrapperLocal(errors.New("videos accept at most one image input"), "invalid_images", http.StatusBadRequest)
	}
	if len(req.ReferenceImages) > 7 {
		return service.TaskErrorWrapperLocal(errors.New("reference_images must contain at most 7 images"), "invalid_reference_images", http.StatusBadRequest)
	}
	if len(req.ReferenceImages) > 0 && (strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 || strings.TrimSpace(req.Video) != "") {
		return service.TaskErrorWrapperLocal(errors.New("reference_images cannot be combined with image or video"), "invalid_request", http.StatusBadRequest)
	}
	if isEdit || isExtension {
		if modelName != LegacyVideoModel {
			return service.TaskErrorWrapperLocal(errors.New("video editing and extension require grok-imagine-video"), "invalid_model", http.StatusBadRequest)
		}
		if strings.TrimSpace(req.Video) == "" {
			return service.TaskErrorWrapperLocal(errors.New("video is required"), "invalid_video", http.StatusBadRequest)
		}
		if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 || len(req.ReferenceImages) > 0 {
			return service.TaskErrorWrapperLocal(errors.New("video input cannot be combined with image inputs"), "invalid_request", http.StatusBadRequest)
		}
		if isEdit && (req.Duration != 0 || strings.TrimSpace(req.AspectRatio) != "" || strings.TrimSpace(req.Resolution) != "") {
			return service.TaskErrorWrapperLocal(errors.New("duration, aspect_ratio and resolution are not supported for video edits"), "invalid_request", http.StatusBadRequest)
		}
		if isExtension {
			if strings.TrimSpace(req.AspectRatio) != "" || strings.TrimSpace(req.Resolution) != "" {
				return service.TaskErrorWrapperLocal(errors.New("aspect_ratio and resolution are not supported for video extensions"), "invalid_request", http.StatusBadRequest)
			}
			if req.Duration != 0 && (req.Duration < 2 || req.Duration > 10) {
				return service.TaskErrorWrapperLocal(errors.New("duration must be between 2 and 10 seconds for video extensions"), "invalid_duration", http.StatusBadRequest)
			}
		}
		return nil
	}
	if strings.TrimSpace(req.Video) != "" {
		return service.TaskErrorWrapperLocal(errors.New("video input requires the edits or extensions endpoint"), "invalid_video", http.StatusBadRequest)
	}
	if len(req.ReferenceImages) > 0 && modelName != VideoModel {
		return service.TaskErrorWrapperLocal(errors.New("reference_images require grok-imagine-video-1.5"), "invalid_model", http.StatusBadRequest)
	}
	if modelName == VideoModel && strings.TrimSpace(req.Image) == "" && len(req.Images) == 0 && len(req.ReferenceImages) == 0 {
		return service.TaskErrorWrapperLocal(errors.New("Grok Imagine Video 1.5 models require an image"), "invalid_image", http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		return "", errors.New("Molii Grok Imagine API configuration is incomplete")
	}
	if info != nil && info.Action == videoEditAction {
		return a.baseURL + "/v1/videos/edits", nil
	}
	if info != nil && info.Action == videoExtensionAction {
		return a.baseURL + "/v1/videos/extensions", nil
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
	if taskRequestContainsFileID(req) {
		return nil, errors.New(unresolvedFileIDMessage)
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
			Video:  buildMediaInput(req.Video),
		}
		body, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(body), nil
	}
	if info != nil && info.Action == videoExtensionAction {
		payload := videoExtensionRequestPayload{
			Model:    modelName,
			Prompt:   strings.TrimSpace(req.Prompt),
			Duration: req.Duration,
			Video:    buildMediaInput(req.Video),
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
		media := buildMediaInput(imageURL)
		payload.Image = &media
	}
	if info != nil && info.Action == referenceToVideoAction {
		payload.ReferenceImages = make([]mediaInput, 0, len(req.ReferenceImages))
		for _, reference := range req.ReferenceImages {
			payload.ReferenceImages = append(payload.ReferenceImages, buildMediaInput(reference))
		}
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (a *TaskAdaptor) resolveTaskFileIDs(c *gin.Context, info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) *taskdto.TaskError {
	if c == nil || c.Request == nil || info == nil || req == nil {
		return service.TaskErrorWrapperLocal(errors.New("request context is missing"), "invalid_request", http.StatusBadRequest)
	}
	resolve := a.userFileResolver()
	type resolvedFile struct {
		file *model.MoliiFile
		url  string
	}
	cache := make(map[string]resolvedFile)
	resolveOne := func(fileID string, expected model.MoliiFileMediaType) (*model.MoliiFile, string, *taskdto.TaskError) {
		fileID = strings.TrimSpace(fileID)
		if fileID == "" {
			return nil, "", nil
		}
		cacheKey := string(expected) + ":" + fileID
		if cached, ok := cache[cacheKey]; ok {
			return cached.file, cached.url, nil
		}
		file, signedURL, err := resolve(c.Request.Context(), info.UserId, fileID, expected)
		if err == nil && strings.TrimSpace(signedURL) != "" {
			resolved := resolvedFile{file: file, url: strings.TrimSpace(signedURL)}
			cache[cacheKey] = resolved
			return resolved.file, resolved.url, nil
		}
		switch {
		case errors.Is(err, model.ErrMoliiFileNotFound):
			return nil, "", service.TaskErrorWrapperLocal(errors.New("file not found"), "file_not_found", http.StatusNotFound)
		case errors.Is(err, model.ErrMoliiFileExpired):
			return nil, "", service.TaskErrorWrapperLocal(errors.New("file has expired"), "file_expired", http.StatusGone)
		case errors.Is(err, service.ErrMoliiFileTypeMismatch):
			return nil, "", service.TaskErrorWrapperLocal(errors.New("file media type does not match the requested input"), "file_type_mismatch", http.StatusBadRequest)
		default:
			return nil, "", service.TaskErrorWrapperLocal(errors.New("file service is temporarily unavailable"), "file_service_unavailable", http.StatusServiceUnavailable)
		}
	}

	if req.ImageFileID != "" {
		fileID := req.ImageFileID
		_, resolved, taskErr := resolveOne(fileID, model.MoliiFileMediaTypeImage)
		if taskErr != nil {
			return taskErr
		}
		req.Image, req.ImageFileID = resolved, ""
		for index, value := range req.Images {
			if strings.TrimSpace(value) == strings.TrimSpace(fileID) {
				req.Images[index] = resolved
			}
		}
	}
	for index, fileID := range req.ImageFileIDs {
		if strings.TrimSpace(fileID) == "" {
			continue
		}
		_, resolved, taskErr := resolveOne(fileID, model.MoliiFileMediaTypeImage)
		if taskErr != nil {
			return taskErr
		}
		if index < len(req.Images) {
			req.Images[index] = resolved
		}
		req.ImageFileIDs[index] = ""
	}
	if req.VideoFileID != "" {
		file, resolved, taskErr := resolveOne(req.VideoFileID, model.MoliiFileMediaTypeVideo)
		if taskErr != nil {
			return taskErr
		}
		if file != nil && file.DurationSeconds > 0 && file.Height > 0 {
			resolution := "720p"
			if file.Height <= 480 {
				resolution = "480p"
			}
			info.InputVideoDurationSeconds = file.DurationSeconds
			info.InputVideoResolutionTier = resolution
			info.InputVideoResolutionSource = relaycommon.GrokVideoResolutionSourceInputProbeV1
		}
		req.Video, req.VideoFileID = resolved, ""
	}
	for index, fileID := range req.ReferenceImageFileIDs {
		if strings.TrimSpace(fileID) == "" {
			continue
		}
		_, resolved, taskErr := resolveOne(fileID, model.MoliiFileMediaTypeImage)
		if taskErr != nil {
			return taskErr
		}
		if index < len(req.ReferenceImages) {
			req.ReferenceImages[index] = resolved
		}
		req.ReferenceImageFileIDs[index] = ""
	}
	c.Set("task_request", *req)
	return nil
}

func buildMediaInput(value string) mediaInput {
	return mediaInput{URL: strings.TrimSpace(value)}
}

func requestContainsFileID(c *gin.Context) bool {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return false
	}
	body, err := storage.Bytes()
	if err != nil {
		return false
	}
	var request map[string]json.RawMessage
	if err := common.Unmarshal(body, &request); err != nil {
		return false
	}
	for _, field := range []string{"image", "images", "video", "reference_images"} {
		if mediaContainsFileID(request[field]) {
			return true
		}
	}
	return false
}

func mediaContainsFileID(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var direct string
	if err := common.Unmarshal(raw, &direct); err == nil {
		return strings.HasPrefix(strings.ToLower(strings.TrimSpace(direct)), "file_")
	}
	var items []json.RawMessage
	if err := common.Unmarshal(raw, &items); err == nil {
		for _, item := range items {
			if mediaContainsFileID(item) {
				return true
			}
		}
		return false
	}
	var object map[string]json.RawMessage
	if err := common.Unmarshal(raw, &object); err == nil {
		for key := range object {
			if strings.EqualFold(key, "file_id") {
				return true
			}
		}
	}
	return false
}

func taskRequestContainsFileID(req relaycommon.TaskSubmitReq) bool {
	if strings.TrimSpace(req.ImageFileID) != "" || strings.TrimSpace(req.VideoFileID) != "" {
		return true
	}
	for _, fileID := range req.ImageFileIDs {
		if strings.TrimSpace(fileID) != "" {
			return true
		}
	}
	for _, fileID := range req.ReferenceImageFileIDs {
		if strings.TrimSpace(fileID) != "" {
			return true
		}
	}
	values := append([]string{req.Image, req.Video}, req.Images...)
	values = append(values, req.ReferenceImages...)
	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "file_") {
			return true
		}
	}
	return false
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
		resolution = strings.ToLower(strings.TrimSpace(info.InputVideoResolutionTier))
		seconds = info.InputVideoDurationSeconds
	} else if info.Action == videoExtensionAction {
		resolution = strings.ToLower(strings.TrimSpace(info.EstimatedVideoResolution))
	}
	outputPrice, imageInputPrice, videoInputPrice, ok := ratio_setting.GetMoliiGrokVideoPrices(modelName, resolution)
	if !ok {
		return nil
	}
	cost := seconds * outputPrice
	if info.Action == videoEditAction {
		cost += seconds * videoInputPrice
	} else if info.Action == videoExtensionAction {
		cost = float64(req.Duration)*outputPrice + info.InputVideoDurationSeconds*videoInputPrice
	} else if info.Action == referenceToVideoAction {
		cost += float64(len(req.ReferenceImages)) * imageInputPrice
	} else if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 {
		cost += imageInputPrice
	}
	info.EstimatedVideoPrice = cost
	info.EstimatedVideoUnitPrice = outputPrice
	if info.Action == videoEditAction || info.Action == videoExtensionAction {
		info.EstimatedVideoInputUnitPrice = videoInputPrice
		info.EstimatedVideoOutputUnitPrices = make(map[string]float64, 2)
		for _, candidate := range []string{"480p", "720p"} {
			candidateOutput, _, _, configured := ratio_setting.GetMoliiGrokVideoPrices(modelName, candidate)
			if configured {
				info.EstimatedVideoOutputUnitPrices[candidate] = candidateOutput
			}
		}
	} else if info.Action == referenceToVideoAction || strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 {
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

		if snapshot.Operation == videoEditAction || snapshot.Operation == videoExtensionAction {
			duration = snapshot.RequestedDurationSeconds
			if duration <= 0 {
				return 0
			}
			resolution := strings.ToLower(strings.TrimSpace(snapshot.RequestedResolution))
			if resolution != "480p" && resolution != "720p" || snapshot.ResolutionSource != relaycommon.GrokVideoResolutionSourceInputProbeV1 {
				return 0
			}
			snapshot.ActualDurationSeconds = duration
			if snapshot.Operation == videoEditAction {
				snapshot.VideoInputBilledSeconds = duration
			}
			snapshot.ActualResolution = resolution
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
	if task.Status == model.TaskStatusSuccess && strings.TrimSpace(task.GetResultURL()) != "" && (task.PrivateData.StoredResult == nil || task.HasUnexpiredStoredResult(time.Now())) {
		if task.PrivateData.StoredResult != nil {
			video.SetMetadata("url", service.BuildSignedVideoProxyURLUntil(task.TaskID, task.UserId, time.Unix(task.PrivateData.StoredResult.ExpiresAt, 0)))
		} else {
			video.SetMetadata("url", service.BuildSignedVideoProxyURL(task.TaskID, task.UserId))
		}
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
