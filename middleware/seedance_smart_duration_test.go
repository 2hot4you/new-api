package middleware

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/bytedanceseedance"
	taskplugin "github.com/QuantumNous/new-api/relay/channel/task/jsplugin"
	"github.com/QuantumNous/new-api/relay/channel/task/seedanceprotocol"
	"github.com/QuantumNous/new-api/relay/channel/task/starai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedanceSmartDurationSharedEndpoint(t *testing.T) {
	previousRegistry := jsplugin.DefaultRegistry
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	t.Cleanup(func() { jsplugin.DefaultRegistry = previousRegistry })
	source, err := builtinplugins.Source("doubao")
	require.NoError(t, err)
	// Model the historical uploaded driver this compatibility test protects.
	// The current built-in declares duration-auto@1 and can now reserve a
	// positive maximum usage while forwarding -1; older drivers cannot opt in.
	const autoDurationDeclaration = `requiredCapabilities: ["duration-auto@1"],`
	require.Contains(t, source, autoDurationDeclaration)
	source = strings.Replace(source, autoDurationDeclaration, "", 1)
	plugin, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
	require.NoError(t, err)
	require.False(t, plugin.Meta.AcceptsAutoDuration())

	for _, channelType := range []int{constant.ChannelTypeStarAI, constant.ChannelTypeByteDanceSeedance, constant.ChannelTypeDoubaoVideo, constant.ChannelTypeVolcEngine} {
		for _, path := range []string{"/v1/videos", "/api/video-studio/estimate"} {
			t.Run(fmt.Sprintf("%d%s", channelType, path), func(t *testing.T) {
				const modelName = "doubao-seedance-2-0-260128"
				router := gin.New()
				handlers := []gin.HandlerFunc{}
				if path == "/api/video-studio/estimate" {
					handlers = append(handlers, PrepareVideoStudioRequest())
				}
				reachedChannel := false
				handlers = append(handlers, PinTaskPluginEndpoint(), PrepareTaskPluginEndpoint(), func(c *gin.Context) {
					reachedChannel = true
					selected := &model.Channel{Id: 1, Type: channelType, Key: "test-key", Models: modelName}
					allowed, _ := model.ChannelSatisfiesFilters(selected, modelName, service.GetChannelConstraints(c).Filters)
					require.True(t, allowed, "the shared precheck must retain the existing channel identity rules")
					require.Nil(t, SetupContextForSelectedChannel(c, selected, modelName))
					info := &relaycommon.RelayInfo{OriginModelName: modelName, TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
					info.InitChannelMeta(c)
					info.UpstreamModelName = modelName
					var adaptor channel.TaskAdaptor
					switch channelType {
					case constant.ChannelTypeStarAI:
						adaptor = &starai.TaskAdaptor{}
					case constant.ChannelTypeByteDanceSeedance:
						adaptor = &bytedanceseedance.TaskAdaptor{}
					default:
						adaptor = taskplugin.New(plugin)
					}
					adaptor.Init(info)
					taskErr := adaptor.ValidateRequestAndSetAction(c, info)
					if channelType == constant.ChannelTypeDoubaoVideo || channelType == constant.ChannelTypeVolcEngine {
						require.NotNil(t, taskErr, "legacy JS drivers must still reject a negative billing quantity")
						assert.Equal(t, http.StatusBadRequest, taskErr.StatusCode)
						assert.Equal(t, "plugin_usage_invalid", taskErr.Code)
						assert.Nil(t, info.Billing, "rejection precedes pre-consumption")
						c.Status(taskErr.StatusCode)
						return
					}
					require.Nil(t, taskErr)
					body, err := adaptor.BuildRequestBody(c, info)
					require.NoError(t, err)
					raw, err := io.ReadAll(body)
					require.NoError(t, err)
					var payload seedanceprotocol.Payload
					require.NoError(t, common.Unmarshal(raw, &payload))
					require.NotNil(t, payload.Duration)
					assert.Equal(t, -1, *payload.Duration)
					require.NotNil(t, payload.Watermark)
					assert.False(t, *payload.Watermark, "explicit false must survive")
					assert.Equal(t, "123", payload.Content[0].Text)
					if meter, ok := adaptor.(channel.TaskValidatedUsageFactsProvider); ok {
						facts, err := meter.ExtractUsageFactsValidated(c, info)
						require.NoError(t, err)
						tokens, ok := facts["tokens"].(float64)
						require.True(t, ok)
						assert.Greater(t, tokens, float64(0))
						assert.False(t, math.IsNaN(tokens) || math.IsInf(tokens, 0))
						capabilities, ok := seedanceprotocol.CapabilitiesForModel(modelName)
						require.True(t, ok)
						assert.Equal(t, capabilities.MaxDuration, info.EstimatedVideoSeconds)
					}
					assert.Nil(t, info.Billing, "validation and estimation must not reserve quota")
					c.Status(http.StatusNoContent)
				})
				router.POST(path, handlers...)
				request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"123"}],"generate_audio":true,"resolution":"720p","ratio":"adaptive","duration":-1,"watermark":false}`))
				request.Header.Set("Content-Type", "application/json")
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				require.True(t, reachedChannel, recorder.Body.String())
				wantStatus := http.StatusNoContent
				if channelType == constant.ChannelTypeDoubaoVideo || channelType == constant.ChannelTypeVolcEngine {
					wantStatus = http.StatusBadRequest
				}
				assert.Equal(t, wantStatus, recorder.Code, recorder.Body.String())
			})
		}
	}
}

func TestSeedanceSharedDurationPrecheckBounds(t *testing.T) {
	previousRegistry := jsplugin.DefaultRegistry
	jsplugin.DefaultRegistry = jsplugin.NewRegistry()
	t.Cleanup(func() { jsplugin.DefaultRegistry = previousRegistry })
	source, err := builtinplugins.Source("doubao")
	require.NoError(t, err)
	plugin, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
	require.NoError(t, err)
	require.True(t, plugin.Meta.AcceptsAutoDuration(), "the current driver retains its bounded auto-duration capability")
	const standard = "doubao-seedance-2-0-260128"
	tests := []struct {
		name, model, fields string
		wantStatus          int
	}{
		{"standard auto", standard, `"duration":-1`, 204},
		{"fast auto", "doubao-seedance-2-0-fast-260128", `"duration":-1`, 204},
		{"mini auto", "doubao-seedance-2-0-mini-260615", `"duration":-1`, 204},
		{"2.5 auto", "doubao-seedance-2-5-260628", `"duration":-1`, 204},
		{"seconds string auto", standard, `"seconds":"-1"`, 204},
		{"positive seconds wins", standard, `"seconds":"5","duration":-1`, 204},
		{"invalid seconds wins", standard, `"seconds":"-2","duration":-1`, 400},
		{"other negative", standard, `"duration":-2`, 400},
		{"zero", standard, `"duration":0`, 400},
		{"upper bound", standard, `"duration":3600`, 204},
		{"over upper bound", standard, `"duration":3601`, 400},
		{"non finite", standard, `"seconds":"NaN"`, 400},
		{"unbridged model", "doubao-seedance-1-0-pro-250528", `"duration":-1`, 400},
	}
	for _, tc := range tests {
		for _, form := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/multipart=%t", tc.name, form), func(t *testing.T) {
				raw := fmt.Sprintf(`{"model":%q,"prompt":"123",%s}`, tc.model, tc.fields)
				var request *http.Request
				if form {
					var values map[string]any
					require.NoError(t, common.Unmarshal([]byte(raw), &values))
					var body bytes.Buffer
					writer := multipart.NewWriter(&body)
					for name, value := range values {
						require.NoError(t, writer.WriteField(name, fmt.Sprint(value)))
					}
					require.NoError(t, writer.Close())
					request = httptest.NewRequest(http.MethodPost, "/v1/videos", &body)
					request.Header.Set("Content-Type", writer.FormDataContentType())
				} else {
					request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(raw))
					request.Header.Set("Content-Type", "application/json")
				}
				router := gin.New()
				router.POST("/v1/videos", PinTaskPluginEndpoint(), PrepareTaskPluginEndpoint(), func(c *gin.Context) {
					if !form {
						var preserved map[string]any
						require.NoError(t, common.UnmarshalBodyReusable(c, &preserved))
						actual, err := common.Marshal(preserved)
						require.NoError(t, err)
						assert.JSONEq(t, raw, string(actual), "shared precheck must not rewrite original native fields")
					}
					c.Status(http.StatusNoContent)
				})
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				assert.Equal(t, tc.wantStatus, recorder.Code, recorder.Body.String())
			})
		}
	}
}
