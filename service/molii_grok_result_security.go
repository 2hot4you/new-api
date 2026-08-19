package service

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

var (
	moliiGrokImageResultHosts = map[string]struct{}{
		"imgen.x.ai":     {},
		"files-cdn.x.ai": {},
	}
	moliiGrokVideoResultHosts = map[string]struct{}{
		"vidgen.x.ai":    {},
		"files-cdn.x.ai": {},
	}
)

// IsTrustedMoliiGrokImageURL accepts only the exact HTTPS hosts used for Grok
// image results. Validation is syntactic and never performs a network lookup.
func IsTrustedMoliiGrokImageURL(rawURL string) bool {
	return isTrustedMoliiGrokResultURL(rawURL, moliiGrokImageResultHosts)
}

// IsTrustedMoliiGrokVideoURL permits a narrowly scoped SSRF exception for the
// exact HTTPS host used by Grok Imagine video results. This is needed when a
// local proxy's fake-IP DNS maps the public host into 198.18.0.0/15.
func IsTrustedMoliiGrokVideoURL(rawURL string) bool {
	return isTrustedMoliiGrokResultURL(rawURL, moliiGrokVideoResultHosts)
}

// IsMoliiGrokTaskRoutingConsistent requires the persisted task platform and
// the current channel type to agree whenever either side identifies Grok.
func IsMoliiGrokTaskRoutingConsistent(platform constant.TaskPlatform, channelType int) bool {
	isGrokPlatform := platform == constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMoliiGrokAIGC))
	isGrokChannel := channelType == constant.ChannelTypeMoliiGrokAIGC
	return isGrokPlatform == isGrokChannel
}

func isTrustedMoliiGrokResultURL(rawURL string, allowedHosts map[string]struct{}) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.User != nil || parsed.Host == "" {
		return false
	}
	hostname := strings.ToLower(parsed.Hostname())
	hostname = strings.TrimSuffix(hostname, ".")
	if _, ok := allowedHosts[hostname]; !ok {
		return false
	}
	port := parsed.Port()
	return port == "" || port == "443"
}
