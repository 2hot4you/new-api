package starai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTaskContext(t *testing.T, request relaycommon.TaskSubmitReq) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	requestBody, err := common.Marshal(request)
	require.NoError(t, err)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(string(requestBody)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeStarAI,
			ChannelBaseUrl: "https://ai-api.lfxqai.com",
			ApiKey:         "starai-secret",
		},
	}
	return ctx, info
}

func TestChannelRegistrationConstants(t *testing.T) {
	assert.Equal(t, 61, constant.ChannelTypeStarAI)
	assert.Equal(t, "Molii Volcengine Imagine API", constant.GetChannelTypeName(constant.ChannelTypeStarAI))
	require.Greater(t, len(constant.ChannelBaseURLs), constant.ChannelTypeStarAI)
	assert.Equal(t, "https://ai-api.lfxqai.com", constant.ChannelBaseURLs[constant.ChannelTypeStarAI])
}

func TestBuildRequestPreservesMetadataCapabilitiesAndMappedModel(t *testing.T) {
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{
		Prompt: "cinematic tea ad",
		Model:  ModelList[0],
		Metadata: map[string]any{
			"content": []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/reference.png"}, "role": "reference_image"},
				map[string]any{"type": "video_url", "video_url": map[string]any{"url": "https://example.com/reference.mp4"}, "role": "reference_video"},
				map[string]any{"type": "audio_url", "audio_url": map[string]any{"url": "https://example.com/reference.mp3"}, "role": "reference_audio"},
			},
			"generate_audio": false,
			"watermark":      false,
			"duration":       4,
			"resolution":     "720p",
			"ratio":          "16:9",
			"tools":          []any{map[string]any{"type": "web_search"}},
		},
	})
	info.OriginModelName = ModelList[0]
	info.UpstreamModelName = ModelList[1]
	info.IsModelMapped = true

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:   "cinematic tea ad",
		Model:    ModelList[0],
		Metadata: map[string]any{},
	})
	var original relaycommon.TaskSubmitReq
	require.NoError(t, common.UnmarshalBodyReusable(ctx, &original))
	ctx.Set("task_request", original)

	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload requestPayload
	require.NoError(t, common.Unmarshal(body, &payload))

	assert.Equal(t, ModelList[1], payload.Model)
	require.NotNil(t, payload.GenerateAudio)
	assert.False(t, *payload.GenerateAudio)
	require.NotNil(t, payload.Watermark)
	assert.False(t, *payload.Watermark)
	require.NotNil(t, payload.Duration)
	assert.Equal(t, 4, *payload.Duration)
	assert.Equal(t, "720p", payload.Resolution)
	assert.Equal(t, "16:9", payload.Ratio)
	require.Len(t, payload.Tools, 1)
	assert.Equal(t, "web_search", payload.Tools[0].Type)
	require.Len(t, payload.Content, 4)
	assert.Equal(t, "reference_image", payload.Content[0].Role)
	assert.Equal(t, "reference_video", payload.Content[1].Role)
	assert.Equal(t, "reference_audio", payload.Content[2].Role)
	assert.Equal(t, contentItem{Type: "text", Text: "cinematic tea ad"}, payload.Content[3])
}

func TestTemporaryAssetIsVerifiedBeforeBuildingUpstreamRequest(t *testing.T) {
	previousEnabled, previousRDB := common.RedisEnabled, common.RDB
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	common.RedisEnabled = true
	common.RDB = redisClient
	t.Cleanup(func() {
		_ = redisClient.Close()
		common.RedisEnabled = previousEnabled
		common.RDB = previousRDB
	})

	var verificationRequests atomic.Int32
	var verificationStatus atomic.Int32
	verificationStatus.Store(http.StatusOK)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verificationRequests.Add(1)
		if !strings.HasPrefix(r.URL.Path, "/v1/assets/asset-") || r.Header.Get("Authorization") != "Bearer starai-secret" {
			http.Error(w, "invalid verification request", http.StatusBadRequest)
			return
		}
		if status := int(verificationStatus.Load()); status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
	}))
	t.Cleanup(upstream.Close)

	binding := &service.StarAIAssetBinding{UpstreamID: "asset-upstream-reference", UserID: 42, AssetType: "image", Status: "ACTIVE"}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))
	request := relaycommon.TaskSubmitReq{
		Model: ModelList[0],
		Metadata: map[string]any{"content": []any{
			map[string]any{"type": "text", "text": "use the reference"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "asset://" + binding.ID}, "role": "reference_image"},
		}},
	}
	ctx, info := newTaskContext(t, request)
	ctx.Set("id", 42)
	info.UserId = 42
	info.ChannelBaseUrl = upstream.URL
	info.ApiKey = "starai-secret"
	info.UpstreamModelName = ModelList[0]
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	assert.Equal(t, int32(1), verificationRequests.Load())
	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload requestPayload
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Len(t, payload.Content, 2)
	require.NotNil(t, payload.Content[1].ImageURL)
	assert.Equal(t, "asset://asset-upstream-reference", payload.Content[1].ImageURL.URL)
	assert.Equal(t, int32(1), verificationRequests.Load(), "request build must reuse the verification result from validation")

	verificationStatus.Store(http.StatusNotFound)
	expiredBinding := &service.StarAIAssetBinding{UpstreamID: "asset-expired-reference", UserID: 42, AssetType: "image", Status: "ACTIVE"}
	require.NoError(t, service.SaveStarAIAssetBinding(expiredBinding))
	expiredRequest := relaycommon.TaskSubmitReq{
		Model: ModelList[0],
		Metadata: map[string]any{"content": []any{
			map[string]any{"type": "text", "text": "use the expired reference"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "asset://" + expiredBinding.ID}, "role": "reference_image"},
		}},
	}
	expiredCtx, expiredInfo := newTaskContext(t, expiredRequest)
	expiredCtx.Set("id", 42)
	expiredInfo.UserId = 42
	expiredInfo.ChannelBaseUrl = upstream.URL
	expiredInfo.ApiKey = "starai-secret"
	expiredInfo.UpstreamModelName = ModelList[0]
	expiredAdaptor := &TaskAdaptor{}
	expiredAdaptor.Init(expiredInfo)
	taskErr := expiredAdaptor.ValidateRequestAndSetAction(expiredCtx, expiredInfo)
	require.NotNil(t, taskErr)
	assert.Equal(t, "temporary_asset_expired", taskErr.Code)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	storedExpired, err := service.GetStarAIAssetBinding(expiredBinding.ID, 42)
	require.NoError(t, err)
	assert.Equal(t, "EXPIRED", storedExpired.Status)
}

func TestBuildRequestAcceptsSmartDurationAndRejectsZero(t *testing.T) {
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{
		Prompt:   "smart-duration fixture",
		Model:    ModelList[0],
		Duration: -1,
	})
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(`{"prompt":"smart-duration fixture","model":"`+ModelList[0]+`","duration":-1}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))

	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload requestPayload
	require.NoError(t, common.Unmarshal(body, &payload))
	require.NotNil(t, payload.Duration)
	assert.Equal(t, -1, *payload.Duration)

	zeroCtx, zeroInfo := newTaskContext(t, relaycommon.TaskSubmitReq{Prompt: "invalid", Model: ModelList[0]})
	zeroCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(`{"prompt":"invalid","model":"`+ModelList[0]+`","duration":0}`))
	zeroCtx.Request.Header.Set("Content-Type", "application/json")
	taskErr := adaptor.ValidateRequestAndSetAction(zeroCtx, zeroInfo)
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_seconds", taskErr.Code)
}

func TestBuildTextVideoFiveSecondDefaults(t *testing.T) {
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{
		Prompt:   "a paper boat crossing a rainy street",
		Model:    ModelList[0],
		Duration: 5,
	})
	request := relaycommon.TaskSubmitReq{Prompt: "a paper boat crossing a rainy street", Model: ModelList[0], Duration: 5}
	ctx.Set("task_request", request)
	info.UpstreamModelName = ModelList[0]
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)

	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload requestPayload
	require.NoError(t, common.Unmarshal(body, &payload))

	assert.Equal(t, ModelList[0], payload.Model)
	require.Equal(t, []contentItem{{Type: "text", Text: request.Prompt}}, payload.Content)
	require.NotNil(t, payload.Duration)
	assert.Equal(t, 5, *payload.Duration)
	assert.Equal(t, "720p", payload.Resolution)
	assert.Equal(t, "adaptive", payload.Ratio)
	require.NotNil(t, payload.GenerateAudio)
	assert.True(t, *payload.GenerateAudio)
	require.NotNil(t, payload.Watermark)
	assert.False(t, *payload.Watermark)
}

func TestValidateAllowsMediaOnlyReferenceContent(t *testing.T) {
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{
		Model: ModelList[0],
		Metadata: map[string]any{
			"content": []any{
				map[string]any{
					"type":      "image_url",
					"image_url": map[string]any{"url": "https://example.com/reference.png"},
					"role":      "reference_image",
				},
				map[string]any{
					"type":      "video_url",
					"video_url": map[string]any{"url": "https://example.com/reference.mp4"},
					"role":      "reference_video",
				},
			},
		},
	})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))

	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload requestPayload
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Len(t, payload.Content, 2)
	assert.Equal(t, "https://example.com/reference.png", payload.Content[0].ImageURL.URL)
	assert.Equal(t, "https://example.com/reference.mp4", payload.Content[1].VideoURL.URL)
}

func TestBuildRequestSupportsNativeTopLevelStarAIFields(t *testing.T) {
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{Model: ModelList[0]})
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(`{
		"model":"`+ModelList[0]+`",
		"content":[{"type":"text","text":"native prompt"}],
		"generate_audio":false,
		"resolution":"480p",
		"ratio":"1:1",
		"duration":6,
		"tools":[{"type":"web_search"}]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))

	reader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload requestPayload
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Equal(t, []contentItem{{Type: "text", Text: "native prompt"}}, payload.Content)
	require.NotNil(t, payload.GenerateAudio)
	assert.False(t, *payload.GenerateAudio)
	assert.Equal(t, "480p", payload.Resolution)
	assert.Equal(t, "1:1", payload.Ratio)
	require.NotNil(t, payload.Duration)
	assert.Equal(t, 6, *payload.Duration)
	require.Len(t, payload.Tools, 1)
}

func TestEstimateBillingUsesResolutionAndVideoInputTier(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		body      string
		wantKey   string
		wantRatio float64
	}{
		{
			name:      "standard 720p without video",
			model:     ModelList[0],
			body:      `{"model":"` + ModelList[0] + `","content":[{"type":"text","text":"prompt"}],"resolution":"720p"}`,
			wantKey:   "seedance-720p-no-video-input",
			wantRatio: 1,
		},
		{
			name:      "standard 720p with video",
			model:     ModelList[0],
			body:      `{"model":"` + ModelList[0] + `","content":[{"type":"video_url","video_url":{"url":"https://example.com/reference.mp4"},"role":"reference_video"}],"resolution":"720p"}`,
			wantKey:   "seedance-720p-video-input",
			wantRatio: 28.0 / 46.0,
		},
		{
			name:      "standard 1080p without video",
			model:     ModelList[0],
			body:      `{"model":"` + ModelList[0] + `","content":[{"type":"text","text":"prompt"}],"resolution":"1080p"}`,
			wantKey:   "seedance-1080p-no-video-input",
			wantRatio: 51.0 / 46.0,
		},
		{
			name:      "standard 1080p with video",
			model:     ModelList[0],
			body:      `{"model":"` + ModelList[0] + `","content":[{"type":"video_url","video_url":{"url":"https://example.com/reference.mp4"},"role":"reference_video"}],"resolution":"1080p"}`,
			wantKey:   "seedance-1080p-video-input",
			wantRatio: 31.0 / 46.0,
		},
		{
			name:      "fast 720p with video",
			model:     ModelList[1],
			body:      `{"model":"` + ModelList[1] + `","content":[{"type":"video_url","video_url":{"url":"https://example.com/reference.mp4"},"role":"reference_video"}],"resolution":"720p"}`,
			wantKey:   "seedance-720p-video-input",
			wantRatio: 22.0 / 37.0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{Model: test.model})
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(test.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			info.OriginModelName = test.model
			info.UpstreamModelName = test.model
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))

			ratios := adaptor.EstimateBilling(ctx, info)
			require.Len(t, ratios, 1)
			assert.InDelta(t, test.wantRatio, ratios[test.wantKey], 1e-9)
		})
	}
}

func TestEstimateBillingRecordsSeedanceTokenAndPriceEstimate(t *testing.T) {
	adaptor := &TaskAdaptor{}
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{Model: ModelList[0]})
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(
		`{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"prompt"}],"resolution":"720p","ratio":"16:9","duration":5}`,
	))
	ctx.Request.Header.Set("Content-Type", "application/json")
	info.OriginModelName = ModelList[0]
	info.UpstreamModelName = ModelList[0]
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))

	ratios := adaptor.EstimateBilling(ctx, info)
	require.Len(t, ratios, 1)
	assert.Equal(t, 1280, info.EstimatedVideoWidth)
	assert.Equal(t, 720, info.EstimatedVideoHeight)
	assert.Equal(t, 24, info.EstimatedVideoFPS)
	assert.Equal(t, 5, info.EstimatedVideoSeconds)
	assert.Equal(t, 108900, info.EstimatedVideoTokens)
	assert.InDelta(t, 5.0094, info.EstimatedVideoPrice, 1e-9)
}

func TestDecodeStarAIResponseAcceptsCommonWrappers(t *testing.T) {
	tests := [][]byte{
		[]byte("\xef\xbb\xbf" + `{"code":"success","data":{"task_id":"upstream-task"}}`),
		[]byte("```json\n" + `{"code":"success","data":{"task_id":"upstream-task"}}` + "\n```"),
		[]byte("event: message\ndata: " + `{"code":"success","data":{"task_id":"upstream-task"}}` + "\n\n"),
	}
	for _, body := range tests {
		var response responseEnvelope
		require.NoError(t, decodeStarAIResponse(body, &response))
		assert.Equal(t, "upstream-task", response.Data.TaskID)
	}
}

func TestDecodeStarAICreateResponseAcceptsNumericProgress(t *testing.T) {
	body := []byte(`{"id":"task_upstream","task_id":"task_upstream","object":"video","model":"doubao-seedance-2-0-fast-260128","status":"queued","progress":0,"created_at":1785563120}`)
	var response responseEnvelope
	require.NoError(t, decodeStarAIResponse(body, &response))
	assert.Equal(t, "task_upstream", response.TaskID)
	assert.Equal(t, "0", stringValue(response.Progress))
}

func TestParseTaskResultAcceptsNumericEnvelopeID(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{"code":"success","data":{"id":316061,"task_id":"task_upstream","status":"IN_PROGRESS","progress":"50%","data":{"status":"running"}}}`)
	result, err := adaptor.ParseTaskResult(body)
	require.NoError(t, err)
	assert.Equal(t, "task_upstream", result.TaskID)
	assert.Equal(t, "50%", result.Progress)
}

func TestDoResponseAcceptsSuccessWithoutCode(t *testing.T) {
	adaptor := &TaskAdaptor{}
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{Model: ModelList[1]})
	info.PublicTaskID = "task_public"
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`{"id":"task_upstream","task_id":"task_upstream","object":"video","model":"doubao-seedance-2-0-fast-260128","status":"queued","progress":0,"created_at":1785563120}`,
		)),
	}

	taskID, _, taskErr := adaptor.DoResponse(ctx, response, info)
	require.Nil(t, taskErr)
	assert.Equal(t, "task_upstream", taskID)
}

func TestValidateRejectsEmptyOrAudioOnlyContent(t *testing.T) {
	tests := []struct {
		name    string
		request relaycommon.TaskSubmitReq
	}{
		{name: "empty", request: relaycommon.TaskSubmitReq{Model: ModelList[0]}},
		{
			name: "audio only",
			request: relaycommon.TaskSubmitReq{
				Model: ModelList[0],
				Metadata: map[string]any{
					"content": []any{map[string]any{
						"type":      "audio_url",
						"audio_url": map[string]any{"url": "https://example.com/reference.mp3"},
					}},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, info := newTaskContext(t, test.request)
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, "invalid_request", taskErr.Code)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		})
	}
}

func TestBuildRequestURLAndHeaders(t *testing.T) {
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{Prompt: "prompt", Model: ModelList[0]})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://ai-api.lfxqai.com/v1/video/generations", requestURL)

	req := httptest.NewRequest(http.MethodPost, requestURL, nil)
	require.NoError(t, adaptor.BuildRequestHeader(ctx, req, info))
	assert.Equal(t, "Bearer starai-secret", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
	assert.False(t, adaptor.AllowAutomaticTaskSubmitRetry())
}

func TestDoResponseExtractsTaskIDWithoutExposingIt(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "data task id", body: `{"code":"success","data":{"task_id":"up-data-task","id":"up-data-id"},"task_id":"up-top-task","id":"up-top-id"}`, want: "up-data-task"},
		{name: "data id", body: `{"code":"success","data":{"id":"up-data-id"},"task_id":"up-top-task","id":"up-top-id"}`, want: "up-data-id"},
		{name: "top task id", body: `{"code":"success","task_id":"up-top-task","id":"up-top-id"}`, want: "up-top-task"},
		{name: "top id", body: `{"code":"success","id":"up-top-id"}`, want: "up-top-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tt.body))}
			info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"}, OriginModelName: ModelList[0]}
			adaptor := &TaskAdaptor{}

			upstreamID, taskData, taskErr := adaptor.DoResponse(ctx, resp, info)
			require.Nil(t, taskErr)
			assert.Equal(t, tt.want, upstreamID)
			assert.NotContains(t, recorder.Body.String(), tt.want)
			assert.Contains(t, recorder.Body.String(), "task_public")
			assert.NotContains(t, string(taskData), tt.want)
			assert.Contains(t, string(taskData), "task_public")
		})
	}
}

func TestDoResponseRejectsFailureAndMissingID(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{name: "failure code", body: `{"code":"failed","message":"upstream rejected request"}`, wantCode: "starai_api_error"},
		{name: "missing id", body: `{"code":"success","data":{}}`, wantCode: "invalid_response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tt.body))}
			_, _, taskErr := (&TaskAdaptor{}).DoResponse(ctx, resp, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"}})
			require.NotNil(t, taskErr)
			assert.Equal(t, tt.wantCode, taskErr.Code)
		})
	}
}

func TestSanitizeTaskSubmitErrorDoesNotExposeSecrets(t *testing.T) {
	adaptor := &TaskAdaptor{apiKey: "starai-secret-key"}
	message := adaptor.SanitizeTaskSubmitError([]byte(`{
		"code":"failed",
		"message":"StarAI request with starai-secret-key failed: https://example.com/video?X-Tos-Signature=signed-secret"
	}`))
	assert.NotContains(t, message, "starai-secret-key")
	assert.NotContains(t, message, "signed-secret")
	assert.NotContains(t, message, "StarAI")
	assert.Contains(t, message, "Molii Volcengine Imagine API")
	assert.Contains(t, message, "X-Tos-Signature=***")
	assert.Equal(t, "Molii Volcengine Imagine API request failed", adaptor.SanitizeTaskSubmitError([]byte(`not-json starai-secret-key`)))
}

func TestFetchTaskUsesEscapedUpstreamIDAndAuth(t *testing.T) {
	const upstreamID = "task/upstream value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/video/generations/task%2Fupstream%20value", r.URL.EscapedPath())
		assert.Equal(t, "Bearer fetch-secret", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"success","data":{"status":"PENDING"}}`))
	}))
	t.Cleanup(server.Close)

	resp, err := (&TaskAdaptor{}).FetchTask(server.URL, "fetch-secret", map[string]any{"task_id": upstreamID}, "")
	require.NoError(t, err)
	require.NotNil(t, resp)
	_ = resp.Body.Close()
}

func TestParseTaskResultStatusAndFields(t *testing.T) {
	statusCases := map[string]string{
		"NOT_START":   model.TaskStatusSubmitted,
		"PENDING":     model.TaskStatusQueued,
		"SUBMITTED":   model.TaskStatusSubmitted,
		"QUEUED":      model.TaskStatusQueued,
		"IN_PROGRESS": model.TaskStatusInProgress,
		"running":     model.TaskStatusInProgress,
		"SUCCESS":     model.TaskStatusSuccess,
		"succeeded":   model.TaskStatusSuccess,
		"FAILURE":     model.TaskStatusFailure,
		"cancelled":   model.TaskStatusFailure,
		"mystery":     model.TaskStatusInProgress,
	}
	for status, expected := range statusCases {
		t.Run(status, func(t *testing.T) {
			body := []byte(`{"code":"success","data":{"status":"` + status + `"}}`)
			result, err := (&TaskAdaptor{}).ParseTaskResult(body)
			require.NoError(t, err)
			assert.Equal(t, expected, result.Status)
		})
	}

	body := []byte(`{
		"code":"success",
		"data":{
			"task_id":"upstream",
			"status":"SUCCESS",
			"progress":"87%",
			"fail_reason":"outer reason",
			"result_url":"https://example.com/preferred.mp4",
			"data":{
				"status":"succeeded",
				"content":{"video_url":"https://example.com/nested.mp4"},
				"usage":{"completion_tokens":731025,"total_tokens":731026},
				"error":{"message":"nested reason"}
			}
		}
	}`)
	result, err := (&TaskAdaptor{}).ParseTaskResult(body)
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "87%", result.Progress)
	assert.Equal(t, "https://example.com/preferred.mp4", result.Url)
	assert.Equal(t, "outer reason", result.Reason)
	assert.Equal(t, 731025, result.CompletionTokens)
	assert.Equal(t, 731026, result.TotalTokens)

	nestedZero := []byte(`{"code":"success","usage":{"completion_tokens":90,"total_tokens":100},"data":{"status":"SUCCESS","usage":{"completion_tokens":70,"total_tokens":80},"data":{"usage":{"completion_tokens":0,"total_tokens":0}}}}`)
	result, err = (&TaskAdaptor{}).ParseTaskResult(nestedZero)
	require.NoError(t, err)
	assert.Zero(t, result.CompletionTokens)
	assert.Zero(t, result.TotalTokens)

	nestedOnly := []byte(`{"code":"success","data":{"status":"FAILED","data":{"content":{"video_url":"https://example.com/nested-only.mp4"},"error":{"message":"nested failure"}}}}`)
	result, err = (&TaskAdaptor{}).ParseTaskResult(nestedOnly)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/nested-only.mp4", result.Url)
	assert.Equal(t, "nested failure", result.Reason)
}

func TestValidateRejectsMetadataDurationOutsideBillingBoundary(t *testing.T) {
	ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{
		Prompt: "prompt",
		Model:  ModelList[0],
		Metadata: map[string]any{
			"duration": maxDurationSeconds + 1,
		},
	})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
	require.NotNil(t, taskErr)
	assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
	assert.Equal(t, "invalid_seconds", taskErr.Code)
}

func TestValidateEnforcesDocumentedMediaAndModelRules(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		metadata map[string]any
	}{
		{
			name:  "frame and reference modes cannot mix",
			model: ModelList[0],
			metadata: map[string]any{"content": []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/first.png"}, "role": "first_frame"},
				map[string]any{"type": "video_url", "video_url": map[string]any{"url": "https://example.com/reference.mp4"}, "role": "reference_video"},
			}},
		},
		{
			name:  "audio requires media",
			model: ModelList[0],
			metadata: map[string]any{"content": []any{
				map[string]any{"type": "text", "text": "prompt"},
				map[string]any{"type": "audio_url", "audio_url": map[string]any{"url": "https://example.com/reference.mp3"}, "role": "reference_audio"},
			}},
		},
		{
			name:  "video role is required",
			model: ModelList[0],
			metadata: map[string]any{"content": []any{
				map[string]any{"type": "video_url", "video_url": map[string]any{"url": "https://example.com/reference.mp4"}},
			}},
		},
		{
			name:  "fast model rejects 1080p",
			model: ModelList[1],
			metadata: map[string]any{
				"resolution": "1080p",
				"content":    []any{map[string]any{"type": "text", "text": "prompt"}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, info := newTaskContext(t, relaycommon.TaskSubmitReq{Model: test.model, Metadata: test.metadata})
			info.UpstreamModelName = test.model
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			taskErr := adaptor.ValidateRequestAndSetAction(ctx, info)
			require.NotNil(t, taskErr)
			assert.Equal(t, "invalid_request", taskErr.Code)
			assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
		})
	}
}

func TestConvertToOpenAIVideoUsesPublicID(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		UserId:     42,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		CreatedAt:  10,
		UpdatedAt:  20,
		Properties: model.Properties{OriginModelName: ModelList[0]},
		Data:       []byte(`{"code":"success","data":{"task_id":"task_public","status":"SUCCESS","result_url":"https://example.com/video.mp4","usage":{"completion_tokens":8,"total_tokens":9,"tool_usage":{"web_search":2}}}}`),
	}
	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(body, &video))
	assert.Equal(t, "task_public", video.ID)
	assert.Equal(t, "task_public", video.TaskID)
	assert.Equal(t, dto.VideoStatusCompleted, video.Status)
	playbackURL, err := url.Parse(video.Metadata["url"].(string))
	require.NoError(t, err)
	assert.Equal(t, "/v1/videos/task_public/content", playbackURL.Path)
	_, err = service.VerifyVideoPlaybackSignature("task_public", playbackURL.Query().Get("user_id"), playbackURL.Query().Get("expires"), playbackURL.Query().Get("signature"), time.Now())
	require.NoError(t, err)
	require.NotNil(t, video.Usage)
	assert.Equal(t, 8, video.Usage.CompletionTokens)
	assert.Equal(t, 9, video.Usage.TotalTokens)
	assert.Equal(t, 2, video.Usage.ToolUsage.WebSearch)
}
