package openai

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const imageProxySource = "https://8.8.8.8/image.png?secret=source-credential"

func setupImageResultProxy(t *testing.T) {
	t.Helper()
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
	t.Setenv("IMAGE_RESULT_PROXY_BASE_URL", "https://images.example.com")
	t.Setenv("CRYPTO_SECRET", "image-result-proxy-test-shared-secret")
	oldSecret, oldTimeout := common.CryptoSecret, constant.StreamingTimeout
	common.CryptoSecret = "image-result-proxy-test-shared-secret"
	constant.StreamingTimeout = 30
	t.Cleanup(func() { common.CryptoSecret = oldSecret; constant.StreamingTimeout = oldTimeout })
}

func TestImageResultProxyJSON(t *testing.T) {
	setupImageResultProxy(t)
	body := `{"created":1710000000,"data":[{"url":"` + imageProxySource + `","revised_prompt":"draw a cat","unknown":42},{"b64_json":"encoded"},{"url":"data:image/png;base64,abc"}],"usage": {"input_tokens":3,"output_tokens":4,"total_tokens":7,"custom":12},"custom":"keep me"}`
	for _, tc := range []struct {
		name, original, mapped string
		enabled, rewritten     bool
	}{
		{"original", "gpt-image-2", "alias", true, true},
		{"mapped", "alias", "gpt-image-2.5", true, true},
		{"disabled", "gpt-image-2", "", false, false},
		{"other model", "gpt-image-1", "gpt-image-1", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("IMAGE_RESULT_PROXY_ENABLED", fmt.Sprint(tc.enabled))
			c, recorder, resp, info := newImageTestContext(t, body, "application/json", false)
			info.OriginModelName, info.UpstreamModelName = tc.original, tc.mapped
			info.PriceData.UsePrice = true
			info.PriceData.AddOtherRatio("n", 4)
			usage, apiErr := OpenaiImageHandler(c, info, resp)
			require.Nil(t, apiErr)
			require.Equal(t, 7, usage.TotalTokens)
			require.Equal(t, 3, usage.PromptTokens)
			require.Equal(t, 2.0, info.PriceData.OtherRatios()["n"], "production counts the larger URL/base64 payload group")
			actual := recorder.Body.String()
			if !tc.rewritten {
				require.Equal(t, body, actual)
				return
			}
			proxyURL := gjson.Get(actual, "data.0.url").String()
			require.True(t, strings.HasPrefix(proxyURL, "https://images.example.com/v1/images/proxy/"), proxyURL)
			require.NotContains(t, proxyURL, "8.8.8.8")
			require.NotContains(t, proxyURL, "source-credential")
			restored, err := sjson.Set(actual, "data.0.url", imageProxySource)
			require.NoError(t, err)
			require.Equal(t, body, restored, "all fields and usage formatting must survive")
		})
	}
}

func TestImageResultProxyStreams(t *testing.T) {
	setupImageResultProxy(t)
	for _, realSSE := range []bool{true, false} {
		t.Run(fmt.Sprintf("realSSE=%t", realSSE), func(t *testing.T) {
			usageJSON := `{"input_tokens":3,"output_tokens":4,"total_tokens":7,"custom":12}`
			body := `{"created":1710000000,"data":[{"url":"` + imageProxySource + `","revised_prompt":"draw a cat","b64_json":"encoded"}],"usage":` + usageJSON + `}`
			contentType := "application/json"
			if realSSE {
				body = "data: " + `{"type":"image_edit.completed","url":"` + imageProxySource + `","revised_prompt":"draw a cat","b64_json":"encoded","usage":` + usageJSON + `}` + "\n\ndata: [DONE]\n\n"
				contentType = "text/event-stream"
			}
			c, recorder, resp, info := newImageTestContext(t, body, contentType, true)
			info.OriginModelName = "gpt-image-2"
			info.PriceData.UsePrice = true
			info.PriceData.AddOtherRatio("n", 3)
			usage, apiErr := OpenaiImageStreamHandler(c, info, resp)
			require.Nil(t, apiErr)
			require.Equal(t, 7, usage.TotalTokens)
			require.Equal(t, 1.0, info.PriceData.OtherRatios()["n"])
			actual := recorder.Body.String()
			require.Contains(t, actual, `"url":"https://images.example.com/v1/images/proxy/`)
			require.NotContains(t, actual, "8.8.8.8")
			require.NotContains(t, actual, "source-credential")
			require.Contains(t, actual, `"revised_prompt":"draw a cat"`)
			require.Contains(t, actual, `"b64_json":"encoded"`)
			require.Contains(t, actual, "data: [DONE]")
			if realSSE {
				require.Contains(t, actual, `"usage":`+usageJSON)
			}
		})
	}
}

func TestImageResultProxyFailsClosed(t *testing.T) {
	setupImageResultProxy(t)
	for _, target := range []string{"http://8.8.8.8/image", "https://user:secret@8.8.8.8/image", "https://127.0.0.1/image", "/relative/image", "https://%broken", "javascript:bad", "42"} {
		for _, mode := range []string{"json", "sse", "fallback"} {
			t.Run(mode+"/"+target, func(t *testing.T) {
				value := fmt.Sprintf("%q", target)
				if target == "42" {
					value = target
				}
				body := `{"data":[{"url":` + value + `}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`
				contentType := "application/json"
				if mode == "sse" {
					contentType = "text/event-stream"
					body = "data: " + `{"type":"image_generation.completed","url":` + value + `,"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}` + "\n\ndata: [DONE]\n\n"
				}
				c, recorder, resp, info := newImageTestContext(t, body, contentType, mode != "json")
				info.OriginModelName = "gpt-image-2"
				var apiErr *types.NewAPIError
				if mode == "json" {
					_, apiErr = OpenaiImageHandler(c, info, resp)
				} else {
					_, apiErr = OpenaiImageStreamHandler(c, info, resp)
				}
				require.NotNil(t, apiErr)
				require.True(t, types.IsSkipRetryError(apiErr))
				require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
				require.Equal(t, "image result is unavailable", apiErr.ToOpenAIError().Message)
				require.Empty(t, recorder.Body.String(), "failed URL must never be forwarded")
			})
		}
	}
}

func TestImageResultProxyStreamScope(t *testing.T) {
	setupImageResultProxy(t)
	for _, realSSE := range []bool{true, false} {
		for _, tc := range []struct {
			name, model string
			enabled     bool
		}{
			{"disabled", "gpt-image-2", false}, {"other model", "gpt-image-1", true},
		} {
			t.Run(fmt.Sprintf("realSSE=%t/%s", realSSE, tc.name), func(t *testing.T) {
				t.Setenv("IMAGE_RESULT_PROXY_ENABLED", fmt.Sprint(tc.enabled))
				body := `{"created":1710000000,"data":[{"url":"` + imageProxySource + `"}]}`
				contentType := "application/json"
				if realSSE {
					body = "data: " + `{"type":"image_generation.completed","url":"` + imageProxySource + `"}` + "\n\ndata: [DONE]\n\n"
					contentType = "text/event-stream"
				}
				c, recorder, resp, info := newImageTestContext(t, body, contentType, true)
				info.OriginModelName = tc.model
				_, apiErr := OpenaiImageStreamHandler(c, info, resp)
				require.Nil(t, apiErr)
				require.Contains(t, recorder.Body.String(), imageProxySource)
				require.NotContains(t, recorder.Body.String(), "/v1/images/proxy/")
			})
		}
	}
}

func TestImageResultProxyStreamStopsAfterInvalidURL(t *testing.T) {
	setupImageResultProxy(t)
	body := "data: " + `{"type":"image_generation.partial_image","b64_json":"partial"}` + "\n\ndata: " + `{"type":"image_generation.completed","url":"http://8.8.8.8/private","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}` + "\n\ndata: " + `{"type":"image_generation.completed","url":"` + imageProxySource + `"}` + "\n\ndata: [DONE]\n\n"
	c, recorder, resp, info := newImageTestContext(t, body, "text/event-stream", true)
	info.UpstreamModelName = "gpt-image-2.5"
	usage, apiErr := OpenaiImageStreamHandler(c, info, resp)
	require.NotNil(t, apiErr)
	require.True(t, types.IsSkipRetryError(apiErr))
	require.Equal(t, 7, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), `"b64_json":"partial"`)
	require.NotContains(t, recorder.Body.String(), "8.8.8.8")
	require.NotContains(t, recorder.Body.String(), "[DONE]")
	require.NotContains(t, recorder.Body.String(), "image_generation.completed")
}

func TestImageResultProxyFallbackValidatesAllBeforeWrite(t *testing.T) {
	setupImageResultProxy(t)
	body := `{"data":[{"url":"` + imageProxySource + `"},{"url":"http://8.8.8.8/invalid"}],"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`
	c, recorder, resp, info := newImageTestContext(t, body, "application/json", true)
	info.OriginModelName = "gpt-image-2"
	usage, apiErr := OpenaiImageStreamHandler(c, info, resp)
	require.NotNil(t, apiErr)
	require.True(t, types.IsSkipRetryError(apiErr))
	require.Equal(t, 7, usage.TotalTokens)
	require.Empty(t, recorder.Body.String())
}

func TestImageResultProxyResponseHeaders(t *testing.T) {
	setupImageResultProxy(t)
	for _, enabled := range []bool{true, false} {
		for _, mode := range []string{"json", "sse", "fallback"} {
			t.Run(fmt.Sprintf("enabled=%t/%s", enabled, mode), func(t *testing.T) {
				t.Setenv("IMAGE_RESULT_PROXY_ENABLED", fmt.Sprint(enabled))
				body := `{"data":[{"url":"` + imageProxySource + `"}]}`
				contentType := "application/json"
				if mode == "sse" {
					body = "data: " + `{"type":"image_generation.completed","url":"` + imageProxySource + `"}` + "\n\ndata: [DONE]\n\n"
					contentType = "text/event-stream"
				}
				c, recorder, resp, info := newImageTestContext(t, body, contentType, mode != "json")
				info.OriginModelName = "gpt-image-2"
				c.Header(common.RequestIdKey, "local-request-id")
				resp.Header.Set(common.RequestIdKey, "upstream-request-id")
				for _, name := range []string{"Location", "Content-Location", "Link", "Server", "Set-Cookie", "X-Provider-Info", "X-Codex-Turn-State", "X-Reasoning-Included"} {
					resp.Header.Set(name, imageProxySource)
				}
				if mode == "json" {
					resp.Header.Set("Content-Type", "application/json; provider="+imageProxySource)
				}
				if mode == "json" {
					_, apiErr := OpenaiImageHandler(c, info, resp)
					require.Nil(t, apiErr)
				} else {
					_, apiErr := OpenaiImageStreamHandler(c, info, resp)
					require.Nil(t, apiErr)
				}
				require.Equal(t, "local-request-id", recorder.Header().Get(common.RequestIdKey))
				if enabled || mode == "fallback" {
					for name, values := range recorder.Header() {
						for _, value := range values {
							require.NotContains(t, value, imageProxySource, name)
						}
					}
					expectedContentType := "text/event-stream"
					if mode == "json" {
						expectedContentType = "application/json"
					}
					require.Equal(t, expectedContentType, recorder.Header().Get("Content-Type"))
				} else {
					require.Equal(t, imageProxySource, recorder.Header().Get("X-Codex-Turn-State"))
					if mode == "json" {
						require.Equal(t, imageProxySource, recorder.Header().Get("Location"))
						require.Equal(t, "upstream-request-id", c.GetString(common.UpstreamRequestIdKey))
					}
				}
				if enabled && mode != "fallback" {
					require.Equal(t, "upstream-request-id", c.GetString(common.UpstreamRequestIdKey))
				}
			})
		}
	}
}

func TestImageResultProxyStreamDeliveredImageRetainsBillingOnFailure(t *testing.T) {
	setupImageResultProxy(t)
	body := "data: " + `{"type":"image_generation.completed","url":"` + imageProxySource + `","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}` + "\n\ndata: " + `{"type":"image_generation.completed","url":"http://8.8.8.8/invalid","usage":{"input_tokens":5,"output_tokens":6,"total_tokens":11}}` + "\n\ndata: [DONE]\n\n"
	c, recorder, resp, info := newImageTestContext(t, body, "text/event-stream", true)
	info.OriginModelName = "gpt-image-2"
	info.PriceData.UsePrice = true
	info.PriceData.AddOtherRatio("n", 3)
	usage, apiErr := OpenaiImageStreamHandler(c, info, resp)
	require.Nil(t, apiErr, "a delivered image must not trigger whole-request refund")
	require.Equal(t, 11, usage.TotalTokens)
	require.Equal(t, 3.0, info.PriceData.OtherRatios()["n"], "interrupted delivery must not lower the requested count")
	require.Contains(t, recorder.Body.String(), `"url":"https://images.example.com/v1/images/proxy/`)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: image_generation.completed"))
	require.Contains(t, recorder.Body.String(), "event: error")
	require.Contains(t, recorder.Body.String(), "image result is unavailable")
	require.NotContains(t, recorder.Body.String(), "8.8.8.8")
	require.NotContains(t, recorder.Body.String(), "[DONE]")
	require.True(t, info.StreamStatus.HasErrors())
}

func TestImageResultProxySanitizesUpstreamErrors(t *testing.T) {
	setupImageResultProxy(t)
	for _, enabled := range []bool{true, false} {
		for _, mode := range []string{"json", "sse", "fallback"} {
			t.Run(fmt.Sprintf("enabled=%t/%s", enabled, mode), func(t *testing.T) {
				t.Setenv("IMAGE_RESULT_PROXY_ENABLED", fmt.Sprint(enabled))
				payload := `{"type":"upstream_error","message":"failed ` + imageProxySource + `","error":{"type":"upstream_error","message":"failed ` + imageProxySource + `","param":"` + imageProxySource + `"},"provider_details":"` + imageProxySource + `","usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}}`
				body, contentType := payload, "application/json"
				if mode == "sse" {
					body = "data: " + payload + "\n\ndata: [DONE]\n\n"
					contentType = "text/event-stream"
				}
				c, recorder, resp, info := newImageTestContext(t, body, contentType, mode != "json")
				info.OriginModelName = "gpt-image-2"
				if mode == "sse" {
					usage, apiErr := OpenaiImageStreamHandler(c, info, resp)
					require.Nil(t, apiErr)
					require.Equal(t, 7, usage.TotalTokens)
					require.True(t, info.StreamStatus.HasErrors())
					if enabled {
						require.NotContains(t, recorder.Body.String(), imageProxySource)
						require.NotContains(t, recorder.Body.String(), "provider_details")
						require.Contains(t, recorder.Body.String(), "image result is unavailable")
						require.NotContains(t, info.StreamStatus.Errors[0].Message, imageProxySource)
					} else {
						require.Contains(t, recorder.Body.String(), payload)
					}
				} else {
					var apiErr *types.NewAPIError
					if mode == "json" {
						_, apiErr = OpenaiImageHandler(c, info, resp)
					} else {
						_, apiErr = OpenaiImageStreamHandler(c, info, resp)
					}
					require.NotNil(t, apiErr)
					require.Empty(t, recorder.Body.String())
					if enabled {
						require.Equal(t, "image result is unavailable", apiErr.ToOpenAIError().Message)
						require.True(t, types.IsSkipRetryError(apiErr))
					} else {
						require.True(t, strings.HasPrefix(apiErr.ToOpenAIError().Message, "failed "))
						require.Equal(t, "upstream_error", apiErr.ToOpenAIError().Type)
						require.False(t, types.IsSkipRetryError(apiErr))
					}
				}
			})
		}
	}
}

func TestImageResultProxySanitizesUntypedJSONError(t *testing.T) {
	setupImageResultProxy(t)
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			body := `{"error":{"message":"failed ` + imageProxySource + `"}}`
			c, recorder, resp, info := newImageTestContext(t, body, "application/json", stream)
			info.OriginModelName = "gpt-image-2"
			var apiErr *types.NewAPIError
			if stream {
				_, apiErr = OpenaiImageStreamHandler(c, info, resp)
			} else {
				_, apiErr = OpenaiImageHandler(c, info, resp)
			}
			require.NotNil(t, apiErr)
			require.Equal(t, "image result is unavailable", apiErr.ToOpenAIError().Message)
			require.Empty(t, recorder.Body.String())
		})
	}
}
