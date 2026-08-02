package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteStarAIVideoResponseURLs(t *testing.T) {
	body := []byte(`{
		"result_url":"https://private.example/result.mp4?signature=secret",
		"data":{"content":{"video_url":"https://private.example/video.mp4"}},
		"unrelated_url":"https://docs.example/help"
	}`)
	playbackURL := "https://molii.example/v1/videos/task_public/content?signature=public"
	rewritten := RewriteStarAIVideoResponseURLs(body, playbackURL)

	var result map[string]any
	require.NoError(t, common.Unmarshal(rewritten, &result))
	assert.Equal(t, playbackURL, result["result_url"])
	data := result["data"].(map[string]any)
	content := data["content"].(map[string]any)
	assert.Equal(t, playbackURL, content["video_url"])
	assert.Equal(t, "https://docs.example/help", result["unrelated_url"])
}
