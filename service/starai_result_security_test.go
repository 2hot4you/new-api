package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsUnsignedStarAIPrivateTOSURL(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		unsigned bool
	}{
		{
			name:     "exact host without query",
			rawURL:   "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4",
			unsigned: true,
		},
		{
			name:     "unrelated query is not a signature",
			rawURL:   "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4?response-content-type=video%2Fmp4",
			unsigned: true,
		},
		{
			name:     "empty signature remains unsigned",
			rawURL:   "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4?X-Tos-Signature=",
			unsigned: true,
		},
		{
			name:   "tos signature is case insensitive",
			rawURL: "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4?X-ToS-SiGnAtUrE=signed",
		},
		{
			name:   "amz signature",
			rawURL: "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4?X-Amz-Signature=signed",
		},
		{
			name:   "generic signature",
			rawURL: "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4?SIGNATURE=signed",
		},
		{
			name:   "host suffix does not match",
			rawURL: "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com.evil.example/results/video.mp4",
		},
		{
			name:   "other TOS host does not match",
			rawURL: "https://other.tos-cn-beijing.volces.com/results/video.mp4",
		},
		{
			name:   "relative URL does not match",
			rawURL: "//ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.unsigned, IsUnsignedStarAIPrivateTOSURL(test.rawURL))
		})
	}
}

func TestIsSignedStarAIPrivateTOSURL(t *testing.T) {
	assert.True(t, IsSignedStarAIPrivateTOSURL("https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4?X-Tos-Signature=signed"))
	assert.False(t, IsSignedStarAIPrivateTOSURL("https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4"))
	assert.False(t, IsSignedStarAIPrivateTOSURL("http://ark-acg-cn-beijing.tos-cn-beijing.volces.com/results/video.mp4?X-Tos-Signature=signed"))
	assert.False(t, IsSignedStarAIPrivateTOSURL("https://ark-acg-cn-beijing.tos-cn-beijing.volces.com.evil.example/results/video.mp4?X-Tos-Signature=signed"))
}

func TestSanitizeStarAIResponseBodyRecursivelyRedactsSecretsAndIDs(t *testing.T) {
	const (
		publicTaskID   = "task_public_safe"
		upstreamTaskID = "starai_upstream_secret_id"
		signature      = "signed-query-secret"
		apiKey         = "sk-starai-secret"
	)
	body := []byte(`{
		"id":"top-level-upstream-id",
		"message":"StarAI task failed",
		"authorization":"Bearer top-secret",
		"data":{
			"task_id":"starai_upstream_secret_id",
			"api_key":"sk-starai-secret",
			"x-api-key":"x-api-key-secret",
			"api_token":"api-token-secret",
			"result_url":"https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/starai_upstream_secret_id/video.mp4?X-Amz-Signature=signed-query-secret&X-Amz-Credential=credential-secret&safe=value",
			"debug":"task starai_upstream_secret_id used sk-starai-secret",
			"nested":[{
				"ID":"nested-upstream-id",
				"access_token":"access-secret",
				"client_secret":"client-secret"
			}]
		},
		"usage":{"completion_tokens":731025,"total_tokens":731025}
	}`)

	sanitized := SanitizeStarAIResponseBody(body, publicTaskID)
	assert.NotContains(t, string(sanitized), upstreamTaskID)
	assert.NotContains(t, string(sanitized), signature)
	assert.NotContains(t, string(sanitized), apiKey)
	assert.NotContains(t, string(sanitized), "x-api-key-secret")
	assert.NotContains(t, string(sanitized), "api-token-secret")
	assert.NotContains(t, string(sanitized), "top-secret")
	assert.NotContains(t, string(sanitized), "access-secret")
	assert.NotContains(t, string(sanitized), "client-secret")
	assert.NotContains(t, string(sanitized), "StarAI")

	var decoded map[string]any
	require.NoError(t, common.Unmarshal(sanitized, &decoded))
	assert.Equal(t, publicTaskID, decoded["id"])
	assert.Equal(t, "Molii Volcengine Imagine API task failed", decoded["message"])
	assert.Equal(t, "[REDACTED]", decoded["authorization"])

	data := decoded["data"].(map[string]any)
	assert.Equal(t, publicTaskID, data["task_id"])
	assert.Equal(t, "[REDACTED]", data["api_key"])
	assert.Equal(t, "[REDACTED]", data["x-api-key"])
	assert.Equal(t, "[REDACTED]", data["api_token"])
	assert.Equal(t, "task task_public_safe used [REDACTED]", data["debug"])
	assert.Contains(t, data["result_url"], "safe=value")
	assert.Contains(t, data["result_url"], publicTaskID+"/video.mp4")
	assert.Contains(t, data["result_url"], "%5BREDACTED%5D")

	nested := data["nested"].([]any)[0].(map[string]any)
	assert.Equal(t, publicTaskID, nested["ID"])
	assert.Equal(t, "[REDACTED]", nested["access_token"])
	assert.Equal(t, "[REDACTED]", nested["client_secret"])

	usage := decoded["usage"].(map[string]any)
	assert.EqualValues(t, 731025, usage["completion_tokens"])
	assert.EqualValues(t, 731025, usage["total_tokens"])

	sanitizedReason := sanitizeStarAIText(
		"StarAI task "+upstreamTaskID+" failed while using "+apiKey,
		body,
		publicTaskID,
	)
	assert.Equal(t, "Molii Volcengine Imagine API task task_public_safe failed while using [REDACTED]", sanitizedReason)
}

func TestSanitizeStarAIResponseBodyFailsClosed(t *testing.T) {
	malformed := []byte(`{"task_id":"real-id","authorization":"Bearer secret"`)
	sanitized := SanitizeStarAIResponseBody(malformed, "task_public")

	assert.JSONEq(t, `{"error":{"message":"upstream response unavailable"}}`, string(sanitized))
	assert.NotContains(t, string(sanitized), "real-id")
	assert.NotContains(t, string(sanitized), "secret")
}

func TestSanitizeURLForLogRedactsCredentialsAndSignedQuery(t *testing.T) {
	rawURL := "https://user:password@example.com/video.mp4?X-Tos-Signature=signed&token=secret&safe=value#credential-fragment"
	sanitized := SanitizeURLForLog(rawURL)

	assert.NotContains(t, sanitized, "password")
	assert.NotContains(t, sanitized, "signed")
	assert.NotContains(t, sanitized, "secret")
	assert.NotContains(t, sanitized, "credential-fragment")
	assert.Contains(t, sanitized, "safe=value")
	assert.Contains(t, sanitized, "%5BREDACTED%5D")
}
