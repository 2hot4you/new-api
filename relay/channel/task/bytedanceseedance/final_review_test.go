package bytedanceseedance

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func TestReadablePublicBusinessErrorsStayPrivate(t *testing.T) {
	a := &TaskAdaptor{}
	payload := `{"error":{"code":"invalid_request","message":"Invalid video request","type":"invalid_request_error","diagnostics":{"api_key":"canary-key","url":"https://private.invalid","upstream_id":"cgt-private"}}}`
	mapper, ok := any(a).(channel.TaskSubmitErrorMapper)
	require.True(t, ok, "non-2xx submit errors need structured safe mapping")
	mapped := mapper.MapTaskSubmitError(http.StatusBadRequest, []byte(payload))
	require.Equal(t, "invalid_request", mapped.Code)
	require.Equal(t, "Invalid video request", mapped.Message)
	require.Equal(t, "invalid_request_error", mapped.Type)
	require.Nil(t, mapped.Data)
	numericEnvelope := mapper.MapTaskSubmitError(http.StatusBadRequest, []byte(`{"code":200,"data":`+payload+`}`))
	require.Equal(t, "invalid_request", numericEnvelope.Code)
	require.Equal(t, "invalid_request_error", numericEnvelope.Type)
	_, submitErr := a.ParseResponse(nil, &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"code":"invalid_request","message":"Invalid video request","type":"invalid_request_error"}`))}, nil)
	require.Equal(t, "invalid_request", submitErr.Code)
	result, err := a.ParseTaskResult(nil, nil, []byte(`{"data":{"status":"FAILURE",`+payload[1:]+`}`))
	require.NoError(t, err)
	require.Equal(t, "Invalid video request", result.Reason)
	safe := a.SafePollingData(result)
	require.Contains(t, string(safe), `"code":"invalid_request"`)
	require.Contains(t, string(safe), `"type":"invalid_request_error"`)
	video, err := a.ConvertToOpenAIVideo(&model.Task{TaskID: "task_local", Status: model.TaskStatusFailure, Data: safe})
	require.NoError(t, err)
	require.Contains(t, string(video), `"code":"invalid_request"`)
	require.Contains(t, string(video), `"type":"invalid_request_error"`)
	for _, data := range []string{string(safe), string(video)} {
		for _, secret := range []string{"canary", "private.invalid", "cgt-private", "diagnostics"} {
			require.NotContains(t, data, secret)
		}
	}
	for _, message := range []string{"Invalid video request https://private.invalid?key=canary", "Invalid video request cgt-private", "canary-key", `{\"code\":\"invalid_request\",\"message\":\"canary\"}`} {
		raw, _ := json.Marshal(map[string]any{"error": map[string]any{"code": "invalid_request", "message": message, "type": "Bearer canary"}})
		e := mapper.MapTaskSubmitError(http.StatusBadRequest, raw)
		require.Equal(t, "Invalid video request", e.Message)
		require.Empty(t, e.Type)
	}
}

func TestPollingCapturesOnlySafeTimingAndRequestFacts(t *testing.T) {
	a := &TaskAdaptor{}
	result, err := a.ParseTaskResult(nil, nil, []byte(`{"code":"success","data":{"status":"SUCCESS","submit_time":1700000000,"start_time":1700000002,"finish_time":1700000008,"video_params":{"resolution":"1080p","ratio":"16:9","seconds":6,"input_image_count":2,"input_video_count":1,"input_audio_count":0},"data":{"code":"success","data":{"status":"succeeded","usage":{"total_tokens":40},"duration":6,"resolution":"1080p"}},"api_key":"secret-canary","request":{"prompt":"private-prompt"}}}`))
	require.NoError(t, err)
	var safe map[string]any
	require.NoError(t, json.Unmarshal(a.SafePollingData(result), &safe))
	require.Equal(t, float64(1700000000), safe["submit_time"])
	require.Equal(t, float64(1700000002), safe["start_time"])
	require.Equal(t, float64(1700000008), safe["finish_time"])
	require.Equal(t, float64(6), safe["duration"])
	require.Equal(t, "1080p", safe["resolution"])
	require.Equal(t, "16:9", safe["ratio"])
	require.Equal(t, float64(2), safe["input_image_count"])
	require.Equal(t, float64(1), safe["input_video_count"])
	require.Equal(t, float64(0), safe["input_audio_count"])
	require.NotContains(t, string(a.SafePollingData(result)), "secret")
	require.NotContains(t, string(a.SafePollingData(result)), "prompt")
}

func TestAllResellerOutboundPathsValidateBaseURL(t *testing.T) {
	old := system_setting.ServerAddress
	system_setting.ServerAddress = "https://self.example"
	t.Cleanup(func() { system_setting.ServerAddress = old })
	useResellerAssetRedis(t)
	binding := &service.StarAIAssetBinding{UpstreamID: "asset-bound", UserID: 42, ChannelType: constant.ChannelTypeByteDanceSeedance, Status: "ACTIVE", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))
	for _, base := range []string{"", "relative", "https://self.example", "https://user:password@upstream.example", "https://upstream.example?secret=value", "https://upstream.example#part"} {
		t.Run(base, func(t *testing.T) {
			a := &TaskAdaptor{baseURL: base, apiKey: "one-key"}
			task := &model.Task{PrivateData: model.TaskPrivateData{UpstreamTaskID: "task_upstream"}}
			_, err := a.BuildRequestURL(nil)
			require.Error(t, err)
			_, err = a.FetchTask(base, "one-key", task, "")
			require.Error(t, err)
			_, err = a.BuildContentRequest(task, "video", channel.TaskArtifactClientRequest{Method: http.MethodGet})
			require.Error(t, err)
			_, err = service.ResolveStarAIAssetURI(context.Background(), "asset://asset-bound", 42, service.StarAIAssetVerificationConfig{BaseURL: base, APIKey: "one-key", ChannelType: constant.ChannelTypeByteDanceSeedance})
			require.Error(t, err)
		})
	}
}

func TestDirectStarAIPublicEnvelopeUsage(t *testing.T) {
	a := &TaskAdaptor{}
	result, err := a.ParseTaskResult(nil, nil, []byte(`{"code":"success","data":{"platform":"61","status":"SUCCESS","data":{"code":"success","data":{"status":"succeeded","usage":{"total_tokens":40,"completion_tokens":30}}}}}`))
	require.NoError(t, err)
	require.Equal(t, model.TaskStatusSuccess, result.Status)
	require.Equal(t, 40, result.TotalTokens)
	require.Equal(t, 30, result.CompletionTokens)
}

func TestPollingRejectsInconsistentTokenSiblings(t *testing.T) {
	for _, usage := range []string{`{"total_tokens":10,"completion_tokens":11}`, `{"total_tokens":0,"completion_tokens":1}`} {
		result, err := (&TaskAdaptor{}).ParseTaskResult(nil, nil, []byte(`{"status":"SUCCESS","usage":`+usage+`}`))
		require.NoError(t, err)
		require.Zero(t, result.TotalTokens)
		require.Zero(t, result.CompletionTokens)
	}
}
