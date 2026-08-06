package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTrustedMoliiGrokVideoURL(t *testing.T) {
	assert.True(t, IsTrustedMoliiGrokVideoURL("https://vidgen.x.ai/results/video.mp4"))
	assert.True(t, IsTrustedMoliiGrokVideoURL("https://vidgen.x.ai:443/results/video.mp4?token=signed"))

	assert.False(t, IsTrustedMoliiGrokVideoURL("http://vidgen.x.ai/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("https://vidgen.x.ai:8443/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("https://user@vidgen.x.ai/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("https://vidgen.x.ai.evil.example/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("https://other.example/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("not a url"))
}
