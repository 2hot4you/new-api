package service

import (
	"net/url"
	"strings"
)

const moliiGrokVideoResultHost = "vidgen.x.ai"

// IsTrustedMoliiGrokVideoURL permits a narrowly scoped SSRF exception for the
// exact HTTPS host used by Grok Imagine video results. This is needed when a
// local proxy's fake-IP DNS maps the public host into 198.18.0.0/15.
func IsTrustedMoliiGrokVideoURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.User != nil ||
		!strings.EqualFold(parsed.Hostname(), moliiGrokVideoResultHost) {
		return false
	}
	port := parsed.Port()
	return port == "" || port == "443"
}
