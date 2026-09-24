package bytedanceseedance

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/seedanceprotocol"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useResellerAssetRedis(t *testing.T) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	oldEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, client
	t.Cleanup(func() { _ = client.Close(); common.RedisEnabled, common.RDB = oldEnabled, oldRDB })
}

func TestTemporaryAssetResellerGenerationChecksOwnerAndPreservesRawURI(t *testing.T) {
	useResellerAssetRedis(t)
	binding := &service.StarAIAssetBinding{UpstreamID: "asset-owned", UserID: 42, ChannelType: constant.ChannelTypeByteDanceSeedance, AssetType: "image", Status: "ACTIVE"}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))
	var assetCalls, generationCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/assets/asset-owned":
			assetCalls.Add(1)
			_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
		case "/v1/video/generations":
			generationCalls.Add(1)
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), `"url":"asset://asset-owned"`)
			_, _ = w.Write([]byte(`{"id":"task_molii_public"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	request := relaycommon.TaskSubmitReq{Model: "doubao-seedance-2-5-260628", Prompt: "scene", Images: []string{"asset://asset-owned"}}
	adaptor := &TaskAdaptor{}
	ctx, info := testContext(t, request)
	ctx.Set("task_request", request)
	info.UserId, info.ChannelBaseUrl, info.ApiKey = 84, server.URL, "account-key"
	adaptor.Init(info)
	_, err := adaptor.BuildRequestBody(ctx, info)
	require.Error(t, err)
	require.Zero(t, assetCalls.Load())
	require.Zero(t, generationCalls.Load())

	ctx, info = testContext(t, request)
	ctx.Set("task_request", request)
	info.UserId, info.ChannelBaseUrl, info.ApiKey = 42, server.URL, "account-key"
	adaptor.Init(info)
	body, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	resp, err := adaptor.DoRequest(ctx, info, body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, int32(1), assetCalls.Load())
	require.Equal(t, int32(1), generationCalls.Load())
}

func TestTemporaryAssetResellerGenerationRejectsDeletedExpiredAndNonSuccessBindings(t *testing.T) {
	useResellerAssetRedis(t)
	var upstreamCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
	}))
	t.Cleanup(server.Close)
	for _, tc := range []struct {
		name, status string
		deleted      bool
		expired      bool
	}{
		{name: "deleted", status: "ACTIVE", deleted: true},
		{name: "expired", status: "ACTIVE", expired: true},
		{name: "processing", status: "PROCESSING"},
		{name: "failed", status: "FAILED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binding := &service.StarAIAssetBinding{UpstreamID: "asset-" + tc.name, UserID: 42, ChannelType: constant.ChannelTypeByteDanceSeedance, AssetType: "image", Status: tc.status}
			require.NoError(t, service.SaveStarAIAssetBinding(binding))
			if tc.deleted {
				require.NoError(t, service.DeleteStarAIAssetBinding(binding.ID, 42))
			}
			if tc.expired {
				binding.ExpiresAt = time.Now().Add(-time.Second).Unix()
				body, err := common.Marshal(binding)
				require.NoError(t, err)
				require.NoError(t, common.RDB.Set(context.Background(), fmt.Sprintf("starai:asset:user:42:%s", binding.ID), body, time.Hour).Err())
			}
			request := relaycommon.TaskSubmitReq{Model: "doubao-seedance-2-5-260628", Prompt: "scene", Images: []string{"asset://" + binding.ID}}
			ctx, info := testContext(t, request)
			ctx.Set("task_request", request)
			info.UserId, info.ChannelBaseUrl, info.ApiKey = 42, server.URL, "account-key"
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			_, err := adaptor.BuildRequestBody(ctx, info)
			require.Error(t, err)
		})
	}
	require.Zero(t, upstreamCalls.Load())
}

func TestTemporaryAssetResellerVerifiesImageVideoAndAudioReferences(t *testing.T) {
	useResellerAssetRedis(t)
	var assetCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assetCalls.Add(1)
		_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
	}))
	t.Cleanup(server.Close)
	for _, tc := range []struct{ kind, role string }{
		{kind: "image", role: "reference_image"},
		{kind: "video", role: "reference_video"},
		{kind: "audio", role: "reference_audio"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			binding := &service.StarAIAssetBinding{UpstreamID: "asset-" + tc.kind, UserID: 42, ChannelType: constant.ChannelTypeByteDanceSeedance, AssetType: tc.kind, Status: "ACTIVE"}
			require.NoError(t, service.SaveStarAIAssetBinding(binding))
			request := relaycommon.TaskSubmitReq{Model: "doubao-seedance-2-5-260628", Prompt: "scene"}
			newContext := func(userID int) (*gin.Context, *relaycommon.RelayInfo) {
				ctx, info := testContext(t, request)
				ctx.Set("task_request", request)
				body := fmt.Sprintf(`{"model":"doubao-seedance-2-5-260628","content":[{"type":"%s_url","%s_url":{"url":"asset://asset-%s"},"role":"%s"}]}`, tc.kind, tc.kind, tc.kind, tc.role)
				if tc.kind == "audio" {
					body = `{"model":"doubao-seedance-2-5-260628","content":[{"type":"image_url","image_url":{"url":"https://example.com/reference.png"},"role":"reference_image"},{"type":"audio_url","audio_url":{"url":"asset://asset-audio"},"role":"reference_audio"}]}`
				}
				ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
				ctx.Request.Header.Set("Content-Type", "application/json")
				info.UserId, info.ChannelBaseUrl, info.ApiKey = userID, server.URL, "account-key"
				return ctx, info
			}
			adaptor := &TaskAdaptor{}
			ctx, info := newContext(84)
			adaptor.Init(info)
			_, err := adaptor.BuildRequestBody(ctx, info)
			require.Error(t, err)
			require.Zero(t, assetCalls.Load())
			ctx, info = newContext(42)
			adaptor.Init(info)
			body, err := adaptor.BuildRequestBody(ctx, info)
			require.NoError(t, err)
			payload, err := io.ReadAll(body)
			require.NoError(t, err)
			assert.Contains(t, string(payload), "asset://asset-"+tc.kind)
			require.Equal(t, int32(1), assetCalls.Swap(0))
		})
	}
}

func testContext(t *testing.T, request relaycommon.TaskSubmitReq) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	requestBody, err := common.Marshal(request)
	require.NoError(t, err)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(string(requestBody)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_reseller_public"},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeByteDanceSeedance,
			ChannelBaseUrl: "https://molii.example/",
			ApiKey:         "molii-key",
		},
	}
	return ctx, info
}

func TestSubmitStoresOnlyMoliiPublicTaskID(t *testing.T) {
	adaptor := &TaskAdaptor{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"task_molii_public","upstream_id":"cgt-private","data":{"secret":"secret"}}`))}
	parsed, taskErr := adaptor.ParseResponse(ctx, response, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_reseller_public"}, OriginModelName: "doubao-seedance-2-5-260628"})
	require.Nil(t, taskErr)
	require.NotNil(t, parsed)
	assert.Equal(t, "task_molii_public", parsed.UpstreamTaskID)
	assert.NotContains(t, string(parsed.TaskData), "cgt-private")
	assert.NotContains(t, string(parsed.TaskData), "secret")
	assert.NotContains(t, string(parsed.TaskData), "task_molii_public")
}

func TestSubmitRejectsNonPublicTaskIDs(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"private root ID", `{"id":"cgt-private"}`},
		{"private nested ID", `{"code":"success","data":{"task_id":"cgt-private"}}`},
		{"empty suffix", `{"id":"task_"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.body))}
			parsed, taskErr := (&TaskAdaptor{}).ParseResponse(ctx, resp, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_reseller_public"}})
			assert.Nil(t, parsed)
			require.NotNil(t, taskErr)
			assert.Equal(t, "invalid_response", taskErr.Code)
			assert.NotContains(t, taskErr.Message, "cgt-private")
		})
	}
}

func TestPollDropsNestedStarAIPrivateDiagnostics(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := []byte(`{"code":"success","data":{"task_id":"task_molii_public","upstream_id":"cgt-private","status":"SUCCESS","data":{"private":"secret"},"usage":{"total_tokens":288625}}}`)
	result, err := adaptor.ParseTaskResult(&model.Task{TaskID: "task_reseller_public"}, nil, body)
	require.NoError(t, err)
	assert.Equal(t, 288625, result.TotalTokens)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	safeData := adaptor.SafePollingData(result)
	assert.NotContains(t, string(safeData), "cgt-private")
	assert.NotContains(t, string(safeData), "secret")
	assert.NotContains(t, string(safeData), "task_molii_public")
	assert.True(t, adaptor.IsPrivateTaskPolling())
	var privacy service.PrivateTaskPollingAdaptor = adaptor
	assert.True(t, privacy.IsTaskPollingStatusAccepted(http.StatusOK))
}

func TestOpenAIVideoPollPreservesUsageWithoutPrivateFields(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult(nil, nil, []byte(`{"id":"task_molii_public","status":"completed","usage":{"completion_tokens":120,"total_tokens":150},"result_url":"https://molii.example/video.mp4","upstream_id":"cgt-private"}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, 150, result.TotalTokens)
	assert.Empty(t, result.Url, "polling must not persist Molii's possibly signed result URL")
	safeData := adaptor.SafePollingData(result)
	task := &model.Task{TaskID: "task_reseller_public", Status: model.TaskStatusSuccess, Data: safeData, PrivateData: model.TaskPrivateData{UpstreamTaskID: "task_molii_public"}}
	clientBody, err := adaptor.ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	assert.Contains(t, string(clientBody), `"total_tokens":150`)
	assert.NotContains(t, string(clientBody), "task_molii_public")
	assert.NotContains(t, string(clientBody), "cgt-private")
}

func TestPollDiscardsNestedAndTopLevelResultURLs(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult(nil, nil, []byte(`{"code":"success","result_url":"https://private.example/root?signature=secret","data":{"status":"SUCCESS","result_url":"https://private.example/data?signature=secret","data":{"content":{"video_url":"https://private.example/nested?signature=secret"}}}}`))
	require.NoError(t, err)
	assert.Empty(t, result.Url)
	assert.NotContains(t, string(adaptor.SafePollingData(result)), "private.example")
}

func TestFailureReasonCannotStoreNestedDiagnostic(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult(nil, nil, []byte(`{"code":"success","data":{"status":"FAILURE","fail_reason":"cgt-private secret","data":{"error":"secret"}}}`))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusFailure, result.Status)
	assert.NotContains(t, result.Reason, "cgt-private")
	assert.NotContains(t, result.Reason, "secret")
}

func TestPollRejectsResponseWithoutStatus(t *testing.T) {
	_, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, []byte(`{"code":"success","data":{"upstream_id":"cgt-private","data":{"private":"secret"}}}`))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "cgt-private")
	assert.NotContains(t, err.Error(), "secret")
}

func TestPollRejectsUnknownStatus(t *testing.T) {
	_, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, []byte(`{"code":"success","data":{"status":"MYSTERY","upstream_id":"cgt-private"}}`))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "cgt-private")
}

func TestPollRejectsPrivateResolutionDiagnostic(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult(nil, nil, []byte(`{"code":"success","data":{"status":"SUCCESS","data":{"resolution":"cgt-private"}}}`))
	require.NoError(t, err)
	assert.NotContains(t, string(adaptor.SafePollingData(result)), "cgt-private")
}

func TestBuildRequestAndFetchUseMoliiPublicEndpoints(t *testing.T) {
	ctx, info := testContext(t, relaycommon.TaskSubmitReq{Prompt: "a sunrise", Model: "doubao-seedance-2-5-260628"})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	url, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://molii.example/v1/video/generations", url)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "a sunrise", Model: "doubao-seedance-2-5-260628"})
	bodyReader, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	body, err := io.ReadAll(bodyReader)
	require.NoError(t, err)
	var payload seedanceprotocol.Payload
	require.NoError(t, common.Unmarshal(body, &payload))
	assert.Equal(t, "doubao-seedance-2-5-260628", payload.Model)
	assert.Equal(t, "a sunrise", payload.Content[0].Text)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/video/generations/task_molii_public", r.URL.EscapedPath())
		assert.Equal(t, "Bearer molii-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	resp, err := adaptor.FetchTask(server.URL, "molii-key", &model.Task{PrivateData: model.TaskPrivateData{UpstreamTaskID: "task_molii_public"}}, "")
	require.NoError(t, err)
	_ = resp.Body.Close()
}

func TestSeedanceModelValidationLimits(t *testing.T) {
	for _, modelName := range seedanceprotocol.SupportedModels() {
		t.Run(modelName, func(t *testing.T) {
			ctx, info := testContext(t, relaycommon.TaskSubmitReq{Prompt: "a sunrise", Model: modelName})
			adaptor := &TaskAdaptor{}
			adaptor.Init(info)
			require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
		})
	}
	tooManyImages := make([]string, 31)
	for i := range tooManyImages {
		tooManyImages[i] = "https://example.com/image.png"
	}
	ctx, info := testContext(t, relaycommon.TaskSubmitReq{Prompt: "a sunrise", Model: "doubao-seedance-2-5-260628", Images: tooManyImages})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	assert.NotNil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
}

func TestSeedance25AllowsThirtyImagesTenVideosAndTenAudio(t *testing.T) {
	content := make([]any, 0, 51)
	for i := 0; i < 30; i++ {
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/image.png"}, "role": "reference_image"})
	}
	for i := 0; i < 10; i++ {
		content = append(content, map[string]any{"type": "video_url", "video_url": map[string]any{"url": "https://example.com/video.mp4"}, "role": "reference_video"})
		content = append(content, map[string]any{"type": "audio_url", "audio_url": map[string]any{"url": "https://example.com/audio.mp3"}, "role": "reference_audio"})
	}
	ctx, info := testContext(t, relaycommon.TaskSubmitReq{Model: "doubao-seedance-2-5-260628", Metadata: map[string]any{"content": content}})
	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	require.Nil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
	assert.Equal(t, 30, info.VideoInputImageCount)
	assert.Equal(t, 10, info.VideoInputVideoCount)
	assert.Equal(t, 10, info.VideoInputAudioCount)
	ctx, info = testContext(t, relaycommon.TaskSubmitReq{Model: "doubao-seedance-2-0-260128", Metadata: map[string]any{"content": content[:10]}})
	adaptor.Init(info)
	assert.NotNil(t, adaptor.ValidateRequestAndSetAction(ctx, info))
}
