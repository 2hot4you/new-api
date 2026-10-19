package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageResultProxySanitizesUpstreamFailure(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	service.InitHttpClient()
	for _, tc := range []struct {
		name      string
		enabled   bool
		transport bool
	}{
		{"HTTP disabled", false, false}, {"HTTP enabled", true, false},
		{"transport disabled", false, true}, {"transport enabled", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enabled := tc.enabled
			if enabled {
				t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
			} else {
				t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "false")
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":{"type":"upstream_error","message":"failed https://provider.example/image?secret=abc"}}`)
			}))
			defer server.Close()
			if tc.transport {
				server.Close()
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			c.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-image-2.5")
			info := &relaycommon.RelayInfo{
				OriginModelName: "gpt-image-2.5",
				RelayMode:       relayconstant.RelayModeImagesGenerations,
				RequestURLPath:  "/v1/images/generations",
				Request:         &dto.ImageRequest{Model: "gpt-image-2.5", Prompt: "cat"},
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiType: constant.APITypeOpenAI, ChannelType: constant.ChannelTypeOpenAI,
					ChannelBaseUrl: server.URL, UpstreamModelName: "gpt-image-2.5",
				},
			}
			err := ImageHelper(c, info)
			require.NotNil(t, err)
			expectedStatus := http.StatusBadRequest
			if tc.transport {
				expectedStatus = http.StatusInternalServerError
				if enabled {
					expectedStatus = http.StatusBadGateway
				}
			}
			assert.Equal(t, expectedStatus, err.StatusCode)
			if enabled {
				assert.Equal(t, "image generation is unavailable", err.ToOpenAIError().Message)
				assert.NotContains(t, err.ToOpenAIError().Message, "provider.example")
				assert.NotContains(t, err.ToOpenAIError().Message, "secret=abc")
				assert.True(t, types.IsSkipRetryError(err))
			} else if !tc.transport {
				assert.Contains(t, err.ToOpenAIError().Message, "failed https://")
			} else {
				assert.Contains(t, err.ToOpenAIError().Message, "do request failed")
			}
		})
	}
}
