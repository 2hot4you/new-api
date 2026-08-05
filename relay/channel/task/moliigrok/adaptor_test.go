package moliigrok

import (
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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			body: `{"model":"grok-imagine-video-1.5","prompt":" rainy cat ","duration":5,"aspect_ratio":"16:9","resolution":"480p"}`,
			want: `{"model":"grok-imagine-video-1.5","prompt":"rainy cat","duration":5,"aspect_ratio":"16:9","resolution":"480p"}`,
		},
		{
			name: "defaults",
			body: `{"model":"grok-imagine-video-1.5","prompt":"rainy cat"}`,
			want: `{"model":"grok-imagine-video-1.5","prompt":"rainy cat","duration":5,"aspect_ratio":"16:9","resolution":"480p"}`,
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
		`{"model":"grok-imagine-video-1.5","prompt":"` + strings.Repeat("猫", 10001) + `","duration":5}`,
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
			body := `{"status":"` + tt.status + `","progress":` + strconv.Itoa(tt.progress) + `,"video":{"url":"https://videos.example/result.mp4"},"usage":{"cost_in_usd_ticks":4000000000}}`
			result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(body))
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantProgress, result.Progress)
			if tt.wantURL {
				assert.Equal(t, "https://videos.example/result.mp4", result.Url)
			} else {
				assert.Empty(t, result.Url)
			}
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
