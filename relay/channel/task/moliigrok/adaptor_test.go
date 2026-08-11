package moliigrok

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubVideoProber struct {
	result *service.MediaProbeResult
	err    error
}

func (p stubVideoProber) ProbeUserVideo(_ context.Context, _ service.MediaSource) (*service.MediaProbeResult, error) {
	return p.result, p.err
}

func fixtureVideoAdaptor(duration float64) *TaskAdaptor {
	return &TaskAdaptor{videoProber: stubVideoProber{result: &service.MediaProbeResult{
		DurationSeconds: duration, Width: 640, Height: 480, ResolutionTier: "480p", MIMEType: "video/mp4",
	}}}
}

func taskContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(common.KeyRequestBody, []byte(body))
	t.Cleanup(func() { common.CleanupBodyStorage(c) })
	return c, recorder
}

func taskInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: VideoModel,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public_123"},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeMoliiGrokAIGC,
			ChannelBaseUrl:    "https://provider.invalid/xai",
			ApiKey:            "secret-key",
			UpstreamModelName: VideoModel,
		},
	}
}

func TestVideoRequestUsesDirectFieldsAndDefaults(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "explicit",
			body: `{"model":"grok-imagine-video-1.5","prompt":" rainy cat ","image":"https://images.example/cat.png","duration":5,"aspect_ratio":"16:9","resolution":"480p"}`,
			want: `{"model":"grok-imagine-video-1.5","prompt":"rainy cat","image":{"url":"https://images.example/cat.png"},"duration":5,"aspect_ratio":"16:9","resolution":"480p"}`,
		},
		{
			name: "defaults",
			body: `{"model":"grok-imagine-video-1.5","prompt":"rainy cat","image":"https://images.example/cat.png"}`,
			want: `{"model":"grok-imagine-video-1.5","prompt":"rainy cat","image":{"url":"https://images.example/cat.png"},"duration":5,"aspect_ratio":"16:9","resolution":"480p"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := taskContext(t, tt.body)
			info := taskInfo()
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
			body, err := adaptor.BuildRequestBody(c, info)
			require.NoError(t, err)
			encoded, err := io.ReadAll(body)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(encoded))
			assert.Equal(t, 5, info.EstimatedVideoSeconds)
			assert.Equal(t, "16:9", info.EstimatedVideoRatio)
			assert.Equal(t, "480p", info.EstimatedVideoResolution)
		})
	}
}

func TestVideoRequestRejectsUnsupportedModelAndLongPrompt(t *testing.T) {
	tests := []string{
		`{"model":"grok-imagine-image","prompt":"cat","duration":5}`,
		`{"model":"grok-imagine-video-1.5-preview","prompt":"cat","image":"https://images.example/cat.png","duration":5}`,
		`{"model":"grok-imagine-video-1.5","prompt":"` + strings.Repeat("猫", 10001) + `","image":"https://images.example/cat.png","duration":5}`,
	}
	for _, body := range tests {
		c, _ := taskContext(t, body)
		info := taskInfo()
		adaptor := &TaskAdaptor{}
		adaptor.Init(info)
		require.NotNil(t, adaptor.ValidateRequestAndSetAction(c, info))
	}
}

func TestVideoBuildURLAndHeaders(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := taskInfo()
	adaptor.Init(info)
	url, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://provider.invalid/xai/v1/videos/generations", url)

	req := httptest.NewRequest(http.MethodPost, url, nil)
	c, _ := taskContext(t, `{}`)
	require.NoError(t, adaptor.BuildRequestHeader(c, req, info))
	assert.Equal(t, "Bearer secret-key", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
}

func TestGrokImagineVideoGenerationSupportsImageObject(t *testing.T) {
	c, _ := taskContext(t, `{"model":"grok-imagine-video","prompt":"animate","image":{"url":"https://images.example/cat.png"},"duration":6,"aspect_ratio":"9:16","resolution":"720p"}`)
	info := taskInfo()
	info.OriginModelName = LegacyVideoModel
	info.ChannelMeta.UpstreamModelName = LegacyVideoModel
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
	adaptor.EstimateBilling(c, info)
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"grok-imagine-video","prompt":"animate","duration":6,"aspect_ratio":"9:16","resolution":"720p","image":{"url":"https://images.example/cat.png"}}`, string(encoded))
	assert.InDelta(t, 0.422, info.EstimatedVideoPrice, 0.000001)
}

func TestGrokImagineVideoRejectsFileIDBeforeBillingAndSubmit(t *testing.T) {
	tests := []struct {
		name  string
		model string
		path  string
		body  string
	}{
		{name: "generation image object", model: VideoModel, path: "/v1/videos", body: `{"model":"grok-imagine-video-1.5","prompt":"animate","image":{"file_id":"file_abc"}}`},
		{name: "generation direct file reference", model: VideoModel, path: "/v1/videos", body: `{"model":"grok-imagine-video-1.5","prompt":"animate","image":"file_abc"}`},
		{name: "generation images object", model: VideoModel, path: "/v1/videos", body: `{"model":"grok-imagine-video-1.5","prompt":"animate","images":[{"file_id":"file_abc"}]}`},
		{name: "generation images direct reference", model: VideoModel, path: "/v1/videos", body: `{"model":"grok-imagine-video-1.5","prompt":"animate","images":["file_abc"]}`},
		{name: "edit video object", model: LegacyVideoModel, path: "/v1/videos/edits", body: `{"model":"grok-imagine-video","prompt":"edit","video":{"file_id":"file_abc"}}`},
		{name: "edit direct file reference", model: LegacyVideoModel, path: "/v1/videos/edits", body: `{"model":"grok-imagine-video","prompt":"edit","video":"file_abc"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := taskContext(t, tt.body)
			c.Request.URL.Path = tt.path
			info := taskInfo()
			info.OriginModelName = tt.model
			info.ChannelMeta.UpstreamModelName = tt.model
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)

			taskErr := adaptor.ValidateRequestAndSetAction(c, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			assert.Equal(t, "file_id_not_supported", taskErr.Code)
			assert.Equal(t, "Molii media file_id is not supported; use a URL instead", taskErr.Message)
			assert.Nil(t, info.Billing, "unsupported media must fail before task precharge")
			assert.Empty(t, adaptor.EstimateBilling(c, info), "unsupported media must fail before estimation")
		})
	}
}

func TestGrokImagineVideoURLObjectsNeverForwardFileID(t *testing.T) {
	tests := []struct {
		model string
		path  string
		body  string
	}{
		{model: VideoModel, path: "/v1/videos", body: `{"model":"grok-imagine-video-1.5","prompt":"animate","image":{"url":"https://images.example/cat.png"}}`},
		{model: VideoModel, path: "/v1/videos", body: `{"model":"grok-imagine-video-1.5","prompt":"animate","images":["https://images.example/cat.png"]}`},
		{model: LegacyVideoModel, path: "/v1/videos/edits", body: `{"model":"grok-imagine-video","prompt":"edit","video":{"url":"https://videos.example/a.mp4"}}`},
	}
	for _, tt := range tests {
		c, _ := taskContext(t, tt.body)
		c.Request.URL.Path = tt.path
		info := taskInfo()
		info.OriginModelName = tt.model
		info.ChannelMeta.UpstreamModelName = tt.model
		adaptor := fixtureVideoAdaptor(6)
		adaptor.Init(info)
		require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
		body, err := adaptor.BuildRequestBody(c, info)
		require.NoError(t, err)
		encoded, err := io.ReadAll(body)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), "file_id")
	}
}

func TestGrokImagineVideoBuildRequestBodyDefensivelyRejectsDecodedFileID(t *testing.T) {
	c, _ := taskContext(t, `{}`)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model:       VideoModel,
		Prompt:      "animate",
		Image:       "file_abc",
		ImageFileID: "file_abc",
		Duration:    5,
		AspectRatio: "16:9",
		Resolution:  "480p",
	})
	info := taskInfo()
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	_, err := adaptor.BuildRequestBody(c, info)
	require.Error(t, err)
	assert.Equal(t, fileIDNotSupportedMessage, err.Error())
}

func TestGrokImagineVideoEditUsesOfficialEndpointAndPayload(t *testing.T) {
	c, _ := taskContext(t, `{"model":"grok-imagine-video","prompt":"add rain","video":{"url":"https://videos.example/source.mp4"}}`)
	c.Request.URL.Path = "/v1/videos/edits"
	info := taskInfo()
	info.OriginModelName = LegacyVideoModel
	info.ChannelMeta.UpstreamModelName = LegacyVideoModel
	adaptor := fixtureVideoAdaptor(6)
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
	adaptor.EstimateBilling(c, info)
	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://provider.invalid/xai/v1/videos/edits", requestURL)
	body, err := adaptor.BuildRequestBody(c, info)
	require.NoError(t, err)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"grok-imagine-video","prompt":"add rain","video":{"url":"https://videos.example/source.mp4"}}`, string(encoded))
	assert.True(t, info.EstimatedVideoHasInput)
	assert.InDelta(t, 0.36, info.EstimatedVideoPrice, 0.000001)
}

func TestGrokImagineVideoEditRejectsNonPositiveOrOver87SecondInputBeforeBilling(t *testing.T) {
	for _, duration := range []float64{0, -1, 8.700001} {
		t.Run(strconv.FormatFloat(duration, 'f', -1, 64), func(t *testing.T) {
			c, _ := taskContext(t, `{"model":"grok-imagine-video","prompt":"edit","video":{"url":"https://videos.example/source.mp4"}}`)
			c.Request.URL.Path = "/v1/videos/edits"
			info := taskInfo()
			info.OriginModelName = LegacyVideoModel
			info.ChannelMeta.UpstreamModelName = LegacyVideoModel
			adaptor := fixtureVideoAdaptor(duration)
			adaptor.Init(info)

			taskErr := adaptor.ValidateRequestAndSetAction(c, info)

			require.NotNil(t, taskErr)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
			assert.Nil(t, info.Billing)
			assert.Nil(t, info.GrokVideoBilling)
		})
	}
}

func TestGrokImagineVideo15RequiresImageAndRejectsOldModel1080p(t *testing.T) {
	tests := []struct {
		body string
		path string
	}{
		{body: `{"model":"grok-imagine-video-1.5","prompt":"cat"}`, path: "/v1/videos"},
		{body: `{"model":"grok-imagine-video","prompt":"cat","resolution":"1080p"}`, path: "/v1/videos"},
		{body: `{"model":"grok-imagine-video-1.5","prompt":"cat","video":{"url":"https://videos.example/a.mp4"}}`, path: "/v1/videos/edits"},
	}
	for _, tt := range tests {
		c, _ := taskContext(t, tt.body)
		c.Request.URL.Path = tt.path
		var modelName string
		if strings.Contains(tt.body, `"grok-imagine-video-1.5"`) {
			modelName = VideoModel
		} else {
			modelName = LegacyVideoModel
		}
		info := taskInfo()
		info.OriginModelName = modelName
		info.ChannelMeta.UpstreamModelName = modelName
		adaptor := &TaskAdaptor{}
		adaptor.Init(info)
		require.NotNil(t, adaptor.ValidateRequestAndSetAction(c, info))
	}
}

func TestGrokImagineVideoValidatesOfficialAspectRatios(t *testing.T) {
	for _, ratio := range []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"} {
		t.Run("accepts_"+strings.ReplaceAll(ratio, ":", "_"), func(t *testing.T) {
			body := `{"model":"grok-imagine-video","prompt":"cat","aspect_ratio":"` + ratio + `"}`
			c, _ := taskContext(t, body)
			info := taskInfo()
			info.OriginModelName = LegacyVideoModel
			info.ChannelMeta.UpstreamModelName = LegacyVideoModel
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)

			require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
			assert.Equal(t, ratio, info.EstimatedVideoRatio)
		})
	}

	c, _ := taskContext(t, `{"model":"grok-imagine-video","prompt":"cat","aspect_ratio":"21:9"}`)
	info := taskInfo()
	info.OriginModelName = LegacyVideoModel
	info.ChannelMeta.UpstreamModelName = LegacyVideoModel
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_aspect_ratio", taskErr.Code)
}

func TestGrokVideoEditCompletionUsesImmutableInputProbeRatherThanProviderValues(t *testing.T) {
	task := &model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		GroupRatio: 1, OriginModelName: LegacyVideoModel, EstimatedHasVideo: true,
		GrokVideoBilling: &model.GrokVideoBillingSnapshot{
			Version: 1, Model: LegacyVideoModel, Operation: videoEditAction, InputType: "video",
			RequestedDurationSeconds: 6, EstimatedDurationSeconds: 6,
			RequestedResolution: "480p", EstimatedResolution: "480p", ResolutionSource: relaycommon.GrokVideoResolutionSourceInputProbeV1,
			OutputUnitPrice: 0.05, VideoInputUnitPrice: 0.01,
		},
	}}}

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, &relaycommon.TaskInfo{ActualDurationSeconds: 12, ActualResolution: "720p", ProviderCost: 999999})

	assert.Equal(t, 180000, quota)
	billing := task.PrivateData.BillingContext.GrokVideoBilling
	assert.Equal(t, "480p", billing.ActualResolution)
	assert.Equal(t, relaycommon.GrokVideoResolutionSourceInputProbeV1, billing.ResolutionSource)
	assert.Equal(t, 6.0, billing.ActualDurationSeconds)
	assert.Equal(t, 6.0, billing.VideoInputBilledSeconds)
	assert.Equal(t, 0.05, billing.OutputUnitPrice)
}

func TestGrokVideoEditCompletionWithoutTrustworthyResolutionIsIndeterminate(t *testing.T) {
	for _, resolution := range []string{"", "1080p", "unknown"} {
		t.Run(resolution, func(t *testing.T) {
			task := &model.Task{Platform: constant.TaskPlatform("62"), PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
				GroupRatio: 1, OriginModelName: LegacyVideoModel, EstimatedHasVideo: true,
				EstimatedOutputUnitPrices: map[string]float64{"480p": 0.05, "720p": 0.07},
				GrokVideoBilling: &model.GrokVideoBillingSnapshot{
					Version: 1, Model: LegacyVideoModel, Operation: videoEditAction, InputType: "video",
					EstimatedDurationSeconds: 8.7, EstimatedResolution: "720p",
					OutputUnitPrice: 0.07, VideoInputUnitPrice: 0.01,
				},
			}}}

			quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, &relaycommon.TaskInfo{ActualDurationSeconds: 6, ActualResolution: resolution, ProviderCost: 999999})
			assert.Zero(t, quota)
			billing := task.PrivateData.BillingContext.GrokVideoBilling
			assert.Empty(t, billing.ActualResolution)
			assert.Empty(t, billing.ResolutionSource)
		})
	}
}

func TestLegacyGrokVideoEditContextNeverInfersResolutionFromProviderCost(t *testing.T) {
	task := &model.Task{Platform: constant.TaskPlatform("62"), PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		GroupRatio: 1, OriginModelName: LegacyVideoModel, EstimatedHasVideo: true,
		EstimatedSeconds: 9, EstimatedResolution: "720p", EstimatedInputUnitPrice: 0.01,
		EstimatedOutputUnitPrices: map[string]float64{"480p": 0.05, "720p": 0.07},
	}}}

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, &relaycommon.TaskInfo{ActualDurationSeconds: 6, ProviderCost: 0.36})

	assert.Zero(t, quota)
	bc := task.PrivateData.BillingContext
	assert.True(t, bc.FinalUsageLogOnly)
	require.NotNil(t, bc.GrokVideoBilling)
	assert.Empty(t, bc.GrokVideoBilling.ActualResolution)
	assert.Empty(t, bc.GrokVideoBilling.ResolutionSource)
}

func TestGrokVideoGenerationCompletionUsesSubmittedUnitPrices(t *testing.T) {
	task := &model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		GroupRatio: 1.5, OriginModelName: VideoModel,
		GrokVideoBilling: &model.GrokVideoBillingSnapshot{
			Version: 1, Model: VideoModel, Operation: "image_to_video", InputType: "image",
			RequestedDurationSeconds: 5, EstimatedDurationSeconds: 5,
			RequestedResolution: "1080p", EstimatedResolution: "1080p", InputImageCount: 1,
			OutputUnitPrice: 0, ImageInputUnitPrice: 0,
		},
	}}}

	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, &relaycommon.TaskInfo{ActualDurationSeconds: 4})
	assert.Zero(t, quota)
	billing := task.PrivateData.BillingContext.GrokVideoBilling
	assert.Equal(t, "1080p", billing.ActualResolution)
	assert.Equal(t, 4.0, billing.ActualDurationSeconds)
	assert.Zero(t, billing.OutputUnitPrice, "explicit snapshotted zero must not fall back to current admin pricing")
	assert.Zero(t, billing.Subtotal)
}

func TestGrokVideoV1CompletionUsesHalfAwayRoundingWithGroupRatio(t *testing.T) {
	task := &model.Task{PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{
		GroupRatio: 1.5, OriginModelName: LegacyVideoModel,
		GrokVideoBilling: &model.GrokVideoBillingSnapshot{
			Version: 1, Model: LegacyVideoModel, Operation: "text_to_video", InputType: "text",
			RequestedDurationSeconds: 1, EstimatedDurationSeconds: 1,
			RequestedResolution: "480p", EstimatedResolution: "480p",
			OutputUnitPrice: 0.000001,
		},
	}}}

	// 0.000001 * 1 second * 500000 quota/unit * 1.5 = 0.75,
	// which rounds half-away-from-zero to one quota unit (not truncation to zero).
	quota := (&TaskAdaptor{}).AdjustBillingOnComplete(task, &relaycommon.TaskInfo{ActualDurationSeconds: 1})
	assert.Equal(t, 1, quota)
	assert.Equal(t, 1.5, task.PrivateData.BillingContext.GrokVideoBilling.GroupRatio)
}

func TestVideoSubmitReturnsOnlyPublicTaskID(t *testing.T) {
	c, recorder := taskContext(t, `{}`)
	info := taskInfo()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"upstream-secret-id"}`)),
	}

	upstreamID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, info)
	require.Nil(t, taskErr)
	assert.Equal(t, "upstream-secret-id", upstreamID)
	assert.NotContains(t, string(taskData), "upstream-secret-id")
	assert.Contains(t, recorder.Body.String(), "task_public_123")
	assert.NotContains(t, recorder.Body.String(), "upstream-secret-id")
}

func TestParseVideoTaskStatusesAndClampProgress(t *testing.T) {
	tests := []struct {
		status       string
		progress     int
		wantStatus   string
		wantProgress string
		wantURL      bool
	}{
		{status: "pending", progress: -10, wantStatus: model.TaskStatusInProgress, wantProgress: "0%"},
		{status: "processing", progress: 45, wantStatus: model.TaskStatusInProgress, wantProgress: "45%"},
		{status: "done", progress: 110, wantStatus: model.TaskStatusSuccess, wantProgress: "100%", wantURL: true},
		{status: "failed", progress: 60, wantStatus: model.TaskStatusFailure, wantProgress: "100%"},
		{status: "expired", progress: 20, wantStatus: model.TaskStatusFailure, wantProgress: "100%"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			body := `{"status":"` + tt.status + `","progress":` + strconv.Itoa(tt.progress) + `,"video":{"url":"https://videos.example/result.mp4","duration":5},"usage":{"cost_in_usd_ticks":4000000000}}`
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(body))
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantProgress, result.Progress)
			if tt.wantURL {
				assert.Equal(t, "https://videos.example/result.mp4", result.Url)
				assert.Equal(t, 5.0, result.ActualDurationSeconds)
				assert.Equal(t, 0.4, result.ProviderCost)
			} else {
				assert.Empty(t, result.Url)
			}
		})
	}
}

func TestParseVideoTaskResultNormalizesExplicitResolution(t *testing.T) {
	for _, tt := range []struct {
		resolution string
		want       string
	}{
		{resolution: " 480P ", want: "480p"},
		{resolution: "720p", want: "720p"},
		{resolution: "1080P", want: "1080p"},
		{resolution: "4k", want: ""},
		{resolution: "", want: ""},
	} {
		t.Run(tt.resolution, func(t *testing.T) {
			body := `{"status":"done","video":{"url":"https://videos.example/result.mp4","duration":5,"resolution":` + strconv.Quote(tt.resolution) + `}}`
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(body))
			require.NoError(t, err)
			assert.Equal(t, tt.want, result.ActualResolution)
		})
	}
}

func TestParseTaskResultRejectsNonHTTPSVideoURL(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"status":"done",
		"progress":100,
		"video":{"url":"http://videos.example/result.mp4"}
	}`))
	require.Error(t, err)
	assert.Nil(t, result)
	assert.NotContains(t, err.Error(), "videos.example")
}

func TestConvertToOpenAIVideoUsesPublicContentURL(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public_123",
		UserId:     9,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		CreatedAt:  100,
		UpdatedAt:  200,
		Properties: model.Properties{OriginModelName: VideoModel},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "upstream-secret-id",
			ResultURL:      "https://videos.example/result.mp4",
		},
	}
	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "task_public_123")
	assert.Contains(t, text, "/v1/videos/task_public_123/content")
	assert.NotContains(t, text, "upstream-secret-id")
	assert.NotContains(t, text, "videos.example")
}

func TestTaskSubmitErrorSanitizerHandlesPricingWithoutRequestID(t *testing.T) {
	adaptor := &TaskAdaptor{}
	message := adaptor.SanitizeTaskSubmitError([]byte(`{"error":{"message":"任务计费未配置，请联系管理员 (Request-ID: USA-xxx)","code":"task_pricing_not_configured"}}`))
	assert.Equal(t, "Molii Grok Imagine API 渠道计费未配置，请联系管理员", message)
	assert.NotContains(t, message, "USA-xxx")
	assert.False(t, adaptor.AllowAutomaticTaskSubmitRetry())
	mapped := adaptor.MapTaskSubmitError(http.StatusBadRequest, []byte(`{"error":{"message":"任务计费未配置 (Request-ID: USA-xxx)","code":"task_pricing_not_configured"}}`))
	assert.Equal(t, "task_pricing_not_configured", mapped.Code)
	assert.Equal(t, "provider_configuration_error", mapped.Type)
	assert.Equal(t, http.StatusBadRequest, mapped.StatusCode)
	assert.True(t, mapped.LocalError)
	assert.NotContains(t, mapped.Message, "USA-xxx")
	transport := adaptor.MapTaskTransportError(assert.AnError)
	assert.Equal(t, "molii_grok_request_failed", transport.Code)
	assert.Equal(t, "Molii Grok Imagine API request failed", transport.Message)
	assert.Equal(t, http.StatusBadGateway, transport.StatusCode)
	statusCases := []struct {
		status int
		code   string
		type_  string
	}{
		{http.StatusBadRequest, "invalid_request", "invalid_request_error"},
		{http.StatusUnauthorized, "invalid_channel_key", "authentication_error"},
		{http.StatusForbidden, "invalid_channel_key", "authentication_error"},
		{http.StatusTooManyRequests, "rate_limit_exceeded", "rate_limit_error"},
		{http.StatusBadGateway, "provider_unavailable", "provider_error"},
	}
	for _, tt := range statusCases {
		mapped := adaptor.MapTaskSubmitError(tt.status, []byte(`{"error":{"message":"raw Request-ID: hidden"}}`))
		assert.Equal(t, tt.code, mapped.Code)
		assert.Equal(t, tt.type_, mapped.Type)
		assert.Equal(t, "Molii Grok Imagine API request failed", mapped.Message)
		assert.NotContains(t, mapped.Message, "hidden")
	}
}
