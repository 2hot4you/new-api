package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	grokImageStageParseUpstream      = "parse_upstream_response"
	grokImageStageValidateSourceURL  = "validate_source_url"
	grokImageStageValidateSourceMIME = "validate_source_mime"
	grokImageStageRedisLock          = "redis_lock"
	grokImageStageBuildObjectKey     = "build_object_key"
	grokImageStageCOSHead            = "cos_head"
	grokImageStageCleanupEnqueue     = "cleanup_enqueue"
	grokImageStageRemoteFetch        = "remote_fetch"
	grokImageStageRemoteRedirect     = "remote_redirect"
	grokImageStageRemoteContentType  = "remote_content_type"
	grokImageStageRemoteSize         = "remote_size"
	grokImageStageCOSPut             = "cos_put"
	grokImageStageCOSSign            = "cos_sign"
)

type grokImagePersistenceError struct {
	stage      string
	category   string
	sourceHost string
	cause      error
}

func (err *grokImagePersistenceError) Error() string {
	if err == nil {
		return "grok image persistence failed"
	}
	return fmt.Sprintf("grok image persistence failed: stage=%s error_category=%s source_host=%s", err.stage, err.category, err.sourceHost)
}

func (err *grokImagePersistenceError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

func newGrokImagePersistenceError(stage, category, sourceURL string, cause error) error {
	var existing *grokImagePersistenceError
	if errors.As(cause, &existing) {
		return cause
	}
	return &grokImagePersistenceError{
		stage:      stage,
		category:   category,
		sourceHost: grokImagePersistenceSourceHost(sourceURL),
		cause:      cause,
	}
}

func grokImagePersistenceErrorForMedia(mediaType, stage, category, sourceURL string, cause error) error {
	if !strings.EqualFold(strings.TrimSpace(mediaType), "image") {
		return cause
	}
	return newGrokImagePersistenceError(stage, category, sourceURL, cause)
}

func grokImagePersistenceSourceHost(sourceURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
}

// GrokImagePersistenceErrorDetails exposes only fixed, log-safe diagnostic
// fields. It never exposes the upstream URL, query parameters, or wrapped SDK
// error text.
func GrokImagePersistenceErrorDetails(err error) (stage, errorCategory, sourceHost string, ok bool) {
	var persistenceErr *grokImagePersistenceError
	if !errors.As(err, &persistenceErr) || persistenceErr == nil {
		return "", "", "", false
	}
	return persistenceErr.stage, persistenceErr.category, persistenceErr.sourceHost, true
}
