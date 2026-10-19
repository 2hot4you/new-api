package openai

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const imageResultProxyStreamError = `{"type":"error","error":{"type":"upstream_error","message":"image result is unavailable"}}`

func imageResultProxyError() *types.NewAPIError {
	// Upstream has already generated the image. Retrying would generate and
	// charge for another image; never include a source URL or signing error.
	return types.NewOpenAIError(errors.New("image result is unavailable"), types.ErrorCodeBadResponseBody, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
}

// rewriteImageResultURLs changes only image URL values, preserving unknown
// fields, usage, and large b64_json payloads without decoding them into structs.
func rewriteImageResultURLs(body []byte, info *relaycommon.RelayInfo) ([]byte, *types.NewAPIError) {
	if !imageResultProxyEnabled(info) {
		return body, nil
	}
	if !gjson.ValidBytes(body) {
		return nil, imageResultProxyError()
	}
	paths := []string{"url"}
	data := gjson.GetBytes(body, "data")
	if data.IsObject() {
		paths = append(paths, "data.url")
	} else if data.IsArray() {
		for i := range data.Array() {
			paths = append(paths, "data."+strconv.Itoa(i)+".url")
		}
	}
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() {
			continue
		}
		if value.Type != gjson.String {
			return nil, imageResultProxyError()
		}
		source := value.String()
		if source == "" || strings.HasPrefix(strings.ToLower(source), "data:image/") {
			continue
		}
		proxyURL, err := service.BuildImageResultProxyURL(source)
		if err != nil {
			return nil, imageResultProxyError()
		}
		body, err = sjson.SetBytes(body, path, proxyURL)
		if err != nil {
			return nil, imageResultProxyError()
		}
	}
	return body, nil
}

func imageResultProxyEnabled(info *relaycommon.RelayInfo) bool {
	return service.ImageResultProxyEnabledForModel(info.GetOriginModelName()) ||
		service.ImageResultProxyEnabledForModel(info.GetUpstreamModelName())
}

// Do not forward provider headers alongside concealed image URLs. Keep the
// local response headers and retain the upstream request ID only for logging.
func imageResultProxyResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, contentType string) *http.Response {
	if !imageResultProxyEnabled(info) || resp == nil {
		return resp
	}
	for name, values := range resp.Header {
		if strings.EqualFold(name, common.RequestIdKey) {
			service.ShouldCopyUpstreamHeader(c, name, values)
		}
	}
	filtered := *resp
	filtered.Header = http.Header{"Content-Type": []string{contentType}}
	return &filtered
}
