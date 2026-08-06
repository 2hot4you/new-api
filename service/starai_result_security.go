package service

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
)

const (
	starAIPrivateTOSHost = "ark-acg-cn-beijing.tos-cn-beijing.volces.com"
	redactedValue        = "[REDACTED]"
)

var starAISignatureQueryKeys = map[string]struct{}{
	"x-tos-signature": {},
	"x-amz-signature": {},
	"signature":       {},
}

var starAIUpstreamBrandPattern = regexp.MustCompile(`(?i)\bstar[\s_-]*ai\b`)

// IsUnsignedStarAIPrivateTOSURL reports whether rawURL targets StarAI's exact
// private Ark TOS host without a non-empty supported signature parameter.
func IsUnsignedStarAIPrivateTOSURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) ||
		!strings.EqualFold(parsed.Hostname(), starAIPrivateTOSHost) {
		return false
	}
	return !hasStarAIURLSignature(parsed)
}

// IsSignedStarAIPrivateTOSURL permits a narrowly scoped SSRF exception for
// StarAI's exact HTTPS result host. The signature requirement prevents the
// proxy from becoming an unrestricted fetcher for that host.
func IsSignedStarAIPrivateTOSURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	return err == nil && strings.EqualFold(parsed.Scheme, "https") &&
		strings.EqualFold(parsed.Hostname(), starAIPrivateTOSHost) &&
		hasStarAIURLSignature(parsed)
}

func hasStarAIURLSignature(parsed *url.URL) bool {
	if parsed == nil {
		return false
	}
	for key, values := range parsed.Query() {
		if _, ok := starAISignatureQueryKeys[strings.ToLower(key)]; !ok {
			continue
		}
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				return true
			}
		}
	}
	return false
}

// SanitizeStarAIResponseBody returns a safe JSON copy of an upstream StarAI
// response. A malformed response is replaced with a fixed value so callers
// never persist or log unparsed upstream bytes.
func SanitizeStarAIResponseBody(body []byte, publicTaskID string) []byte {
	var value any
	if err := common.Unmarshal(body, &value); err != nil {
		return []byte(`{"error":{"message":"upstream response unavailable"}}`)
	}

	upstreamIDs, secrets := collectStarAIRedactionTargets(value)
	value = sanitizeStarAIValue(value, publicTaskID, upstreamIDs, secrets)
	sanitized, err := common.Marshal(value)
	if err != nil {
		return []byte(`{"error":{"message":"upstream response unavailable"}}`)
	}
	return sanitized
}

// RewriteStarAIVideoResponseURLs replaces every public result/video URL in a
// sanitized StarAI response with the same Molii playback URL. This prevents
// callers from accidentally using a copied or redacted upstream TOS URL.
func RewriteStarAIVideoResponseURLs(body []byte, playbackURL string) []byte {
	if len(body) == 0 || strings.TrimSpace(playbackURL) == "" {
		return body
	}
	var value any
	if err := common.Unmarshal(body, &value); err != nil {
		return body
	}
	value = rewriteStarAIVideoURLs(value, playbackURL)
	rewritten, err := common.Marshal(value)
	if err != nil {
		return body
	}
	return rewritten
}

func rewriteStarAIVideoURLs(value any, playbackURL string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalizedKey := normalizeSensitiveKey(key)
			if normalizedKey == "resulturl" || normalizedKey == "videourl" {
				if raw, ok := child.(string); ok && strings.TrimSpace(raw) != "" {
					typed[key] = playbackURL
					continue
				}
			}
			typed[key] = rewriteStarAIVideoURLs(child, playbackURL)
		}
		return typed
	case []any:
		for index, child := range typed {
			typed[index] = rewriteStarAIVideoURLs(child, playbackURL)
		}
		return typed
	default:
		return value
	}
}

func sanitizeStarAIText(text string, responseBody []byte, publicTaskID string) string {
	var value any
	if err := common.Unmarshal(responseBody, &value); err != nil {
		return "Upstream task failed"
	}
	upstreamIDs, secrets := collectStarAIRedactionTargets(value)
	sanitized, ok := sanitizeStarAIValue(text, publicTaskID, upstreamIDs, secrets).(string)
	if !ok {
		return "Upstream task failed"
	}
	return sanitized
}

func collectStarAIRedactionTargets(value any) (upstreamIDs []string, secrets []string) {
	var visit func(any)
	visit = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				normalizedKey := normalizeSensitiveKey(key)
				switch {
				case normalizedKey == "id" || normalizedKey == "taskid":
					collectStringValues(child, &upstreamIDs)
				case isSensitiveJSONField(normalizedKey):
					collectStringValues(child, &secrets)
				default:
					visit(child)
				}
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(value)
	return upstreamIDs, secrets
}

func collectStringValues(value any, values *[]string) {
	switch typed := value.(type) {
	case string:
		if typed != "" {
			*values = append(*values, typed)
		}
	case map[string]any:
		for _, child := range typed {
			collectStringValues(child, values)
		}
	case []any:
		for _, child := range typed {
			collectStringValues(child, values)
		}
	}
}

func sanitizeStarAIValue(value any, publicTaskID string, upstreamIDs, secrets []string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalizedKey := normalizeSensitiveKey(key)
			switch {
			case normalizedKey == "id" || normalizedKey == "taskid":
				typed[key] = publicTaskID
			case isSensitiveJSONField(normalizedKey):
				typed[key] = redactedValue
			default:
				typed[key] = sanitizeStarAIValue(child, publicTaskID, upstreamIDs, secrets)
			}
		}
		return typed
	case []any:
		for index, child := range typed {
			typed[index] = sanitizeStarAIValue(child, publicTaskID, upstreamIDs, secrets)
		}
		return typed
	case string:
		for _, secret := range secrets {
			typed = strings.ReplaceAll(typed, secret, redactedValue)
		}
		for _, upstreamID := range upstreamIDs {
			typed = strings.ReplaceAll(typed, upstreamID, publicTaskID)
		}
		typed = sanitizeURLQueryValues(typed)
		return starAIUpstreamBrandPattern.ReplaceAllString(typed, "Molii Volcengine Imagine API")
	default:
		return value
	}
}

func normalizeSensitiveKey(key string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, key)
}

func isSensitiveJSONField(normalizedKey string) bool {
	switch normalizedKey {
	case "authorization", "apikey", "key", "token", "accesstoken", "refreshtoken",
		"credential", "credentials", "secret", "secretkey", "clientsecret", "apisecret":
		return true
	}
	return strings.Contains(normalizedKey, "authorization") ||
		strings.Contains(normalizedKey, "apikey") ||
		strings.Contains(normalizedKey, "credential") ||
		strings.Contains(normalizedKey, "signature") ||
		strings.Contains(normalizedKey, "secret") ||
		(strings.Contains(normalizedKey, "token") && !strings.HasSuffix(normalizedKey, "tokens"))
}

func isSensitiveURLQueryKey(key string) bool {
	normalizedKey := normalizeSensitiveKey(key)
	return isSensitiveJSONField(normalizedKey) ||
		strings.Contains(normalizedKey, "token") ||
		strings.Contains(normalizedKey, "apikey")
}

// SanitizeURLForLog removes credentials and sensitive query values while
// retaining enough URL structure for diagnostics.
func SanitizeURLForLog(rawURL string) string {
	return sanitizeURLQueryValues(rawURL)
}

func sanitizeURLQueryValues(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		if strings.Contains(rawURL, "://") {
			return "[REDACTED_URL]"
		}
		return rawURL
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}

	if parsed.User != nil {
		parsed.User = url.User(redactedValue)
	}
	parsed.Fragment = ""
	if parsed.RawQuery == "" {
		return parsed.String()
	}

	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		parsed.RawQuery = "redacted"
		return parsed.String()
	}
	for key, values := range query {
		if !isSensitiveURLQueryKey(key) {
			continue
		}
		for index := range values {
			values[index] = redactedValue
		}
		query[key] = values
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
