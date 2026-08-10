package upstream

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSupportedGenerationOperations(t *testing.T) {
	tests := []struct {
		op   string
		body string
		path string
	}{
		{"seedance.video.generate", `{"model":"doubao-seedance-2-0-260128","prompt":"cat"}`, "/v1/video/generations"},
		{"grok.image.generate", `{"model":"grok-imagine-image","prompt":"cat"}`, "/v1/images/generations"},
		{"grok.image.edit", `{"model":"grok-imagine-image-quality","prompt":"edit","images":["https://example.test/a.png"]}`, "/v1/images/edits"},
		{"grok.video.generate", `{"model":"grok-imagine-video-1.5","prompt":"animate","image":{"file_id":"file_1"}}`, "/v1/videos"},
		{"grok.video.edit", `{"model":"grok-imagine-video","prompt":"rain","video":"https://example.test/a.mp4"}`, "/v1/videos/edits"},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			request, err := Build(tt.op, []byte(tt.body))
			require.NoError(t, err)
			require.True(t, request.Generation)
			require.Equal(t, tt.path, request.Path)
		})
	}
}

func TestBuildAssetAndPollingOperations(t *testing.T) {
	asset, err := Build("seedance.asset.create", []byte(`{"url":"https://example.test/input.mp4","asset_type":"video","name":"reference"}`))
	require.NoError(t, err)
	require.Equal(t, "/v1/assets", asset.Path)
	require.False(t, asset.Generation)

	resources := map[string]string{
		"seedance.asset.get":    "/v1/assets/asset-molii-1",
		"seedance.asset.delete": "/v1/assets/asset-molii-1",
		"video.get":             "/v1/videos/asset-molii-1",
		"video.content":         "/v1/videos/asset-molii-1/content",
	}
	for operation, path := range resources {
		request, err := BuildResource(operation, "asset-molii-1")
		require.NoError(t, err)
		require.Equal(t, path, request.Path)
	}
}

func TestBuildRejectsContractViolations(t *testing.T) {
	tests := []struct{ op, body string }{
		{"seedance.video.generate", `{"model":"doubao-seedance-2-0-fast-260128","prompt":"cat","resolution":"1080p"}`},
		{"seedance.video.generate", `{"model":"doubao-seedance-2-0-260128","content":[{"type":"audio_url","audio_url":{"url":"https://example.test/a.mp3"},"role":"reference_audio"}]}`},
		{"grok.image.generate", `{"model":"grok-imagine-image","prompt":"cat","size":"1024x1024"}`},
		{"grok.image.generate", `{"model":"grok-imagine-image","prompt":"cat","n":0}`},
		{"grok.image.edit", `{"model":"grok-imagine-image","prompt":"cat"}`},
		{"grok.video.generate", `{"model":"grok-imagine-video-1.5","prompt":"cat"}`},
		{"grok.video.generate", `{"model":"grok-imagine-video","prompt":""}`},
		{"grok.video.edit", `{"model":"grok-imagine-video-1.5","video":"https://example.test/a.mp4"}`},
		{"grok.video.edit", `{"model":"grok-imagine-video","prompt":"","video":"https://example.test/a.mp4"}`},
	}
	for _, tt := range tests {
		_, err := Build(tt.op, []byte(tt.body))
		require.Error(t, err, tt.body)
	}
}

func TestBuildRejectsRetiredGrokVideoAlias(t *testing.T) {
	retiredModel := "grok-imagine-video-1.5-" + "pre" + "view"
	body := fmt.Sprintf(`{"model":%q,"prompt":"animate","image":{"file_id":"file_1"}}`, retiredModel)
	_, err := Build("grok.video.generate", []byte(body))
	require.Error(t, err)
}

func TestCurlPreviewUsesEnvironmentVariableOnly(t *testing.T) {
	request, err := Build("grok.image.generate", []byte(`{"model":"grok-imagine-image","prompt":"cat"}`))
	require.NoError(t, err)
	preview, err := CurlPreview("https://api.example.test", request)
	require.NoError(t, err)
	require.Contains(t, preview, "Bearer $MOLII_API_KEY")
	require.NotContains(t, preview, "sk-secret")
	require.True(t, strings.Contains(preview, "/v1/images/generations"))
}

func TestParsePollResponseViews(t *testing.T) {
	openAI, err := ParsePollResponse([]byte(`{"id":"task_1","status":"completed","progress":100,"metadata":{"url":"https://example.test/video?signature=x"}}`))
	require.NoError(t, err)
	require.True(t, openAI.Terminal)
	require.True(t, openAI.Success)

	generic, err := ParsePollResponse([]byte(`{"code":"success","data":{"task_id":"task_2","status":"IN_PROGRESS","progress":"42%"}}`))
	require.NoError(t, err)
	require.Equal(t, 42, generic.Progress)
	require.False(t, generic.Terminal)
}
