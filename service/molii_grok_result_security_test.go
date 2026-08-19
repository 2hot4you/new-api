package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTrustedMoliiGrokVideoURL(t *testing.T) {
	assert.True(t, IsTrustedMoliiGrokVideoURL("https://vidgen.x.ai/results/video.mp4"))
	assert.True(t, IsTrustedMoliiGrokVideoURL("https://files-cdn.x.ai/results/video.mp4?token=signed"))
	assert.True(t, IsTrustedMoliiGrokVideoURL("https://vidgen.x.ai:443/results/video.mp4?token=signed"))
	assert.True(t, IsTrustedMoliiGrokVideoURL("https://VIDGEN.X.AI./results/video.mp4"))

	assert.False(t, IsTrustedMoliiGrokVideoURL("http://vidgen.x.ai/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("https://vidgen.x.ai:8443/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("https://user@vidgen.x.ai/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("https://vidgen.x.ai.evil.example/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("https://other.example/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("//vidgen.x.ai/results/video.mp4"))
	assert.False(t, IsTrustedMoliiGrokVideoURL("not a url"))
	assert.False(t, IsTrustedMoliiGrokVideoURL(""))
}

func TestIsTrustedMoliiGrokImageURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "image host", url: "https://imgen.x.ai/results/image.jpg?token=signed", want: true},
		{name: "shared files host", url: "https://files-cdn.x.ai/results/image.webp?token=signed", want: true},
		{name: "explicit standard TLS port", url: "https://imgen.x.ai:443/results/image.png", want: true},
		{name: "uppercase trailing dot", url: "https://IMGEN.X.AI./results/image.png", want: true},
		{name: "http", url: "http://imgen.x.ai/results/image.jpg"},
		{name: "custom port", url: "https://imgen.x.ai:8443/results/image.jpg"},
		{name: "userinfo", url: "https://user:secret@imgen.x.ai/results/image.jpg"},
		{name: "forged suffix", url: "https://imgen.x.ai.evil.example/results/image.jpg"},
		{name: "subdomain", url: "https://cdn.imgen.x.ai/results/image.jpg"},
		{name: "video-only host", url: "https://vidgen.x.ai/results/image.jpg"},
		{name: "scheme relative", url: "//imgen.x.ai/results/image.jpg"},
		{name: "malformed", url: "not a url"},
		{name: "empty", url: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsTrustedMoliiGrokImageURL(tt.url))
		})
	}
}
