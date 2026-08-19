package service

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokImagePersistenceErrorDetailsAreStableAndErrorIsRedacted(t *testing.T) {
	err := newGrokImagePersistenceError(
		"remote_fetch",
		"request_failed",
		"https://user:password@Images.Example.com/result.png?token=upstream-secret",
		errors.New("sdk secret: cos-secret-key"),
	)

	stage, reason, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, "remote_fetch", stage)
	require.Equal(t, "request_failed", reason)
	require.Equal(t, "images.example.com", sourceHost)
	require.Equal(t, "grok image persistence failed: stage=remote_fetch error_category=request_failed source_host=images.example.com", err.Error())
	require.NotContains(t, err.Error(), "password")
	require.NotContains(t, err.Error(), "upstream-secret")
	require.NotContains(t, err.Error(), "cos-secret-key")
	require.ErrorIs(t, err, errors.Unwrap(err))
}

func TestGrokImagePersistenceErrorDetailsRejectsUntypedErrors(t *testing.T) {
	stage, reason, sourceHost, ok := GrokImagePersistenceErrorDetails(errors.New("plain"))
	require.False(t, ok)
	require.Empty(t, stage)
	require.Empty(t, reason)
	require.Empty(t, sourceHost)
}

func TestGrokImagePersistenceRemoteStatusIsExtractedWithoutChangingSafeError(t *testing.T) {
	for _, status := range []int{401, 403, 404, 410, 429, 502} {
		t.Run(fmt.Sprintf("status_%d", status), func(t *testing.T) {
			cause := errors.New("response body contains api-key=do-not-log")
			err := newGrokImagePersistenceErrorWithRemoteStatus(
				grokImageStageRemoteFetch,
				"non_success_status",
				"https://imgen.x.ai/private/result.png?signature=do-not-log",
				status,
				cause,
			)

			stage, category, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
			require.True(t, ok)
			require.Equal(t, grokImageStageRemoteFetch, stage)
			require.Equal(t, "non_success_status", category)
			require.Equal(t, "imgen.x.ai", sourceHost)
			require.Equal(t, status, GrokImagePersistenceRemoteStatus(err))
			require.Equal(t, status, GrokImagePersistenceRemoteStatus(fmt.Errorf("wrapped: %w", err)))
			require.NotContains(t, err.Error(), "/private/result.png")
			require.NotContains(t, err.Error(), "signature")
			require.NotContains(t, err.Error(), "api-key")
		})
	}
}

func TestGrokImagePersistenceRemoteStatusDefaultsToZero(t *testing.T) {
	typed := newGrokImagePersistenceError(
		grokImageStageCOSPut,
		"put_failed",
		"https://imgen.x.ai/result.png?signature=do-not-log",
		errors.New("cos secret do-not-log"),
	)

	require.Zero(t, GrokImagePersistenceRemoteStatus(typed))
	require.Zero(t, GrokImagePersistenceRemoteStatus(errors.New("plain")))
}

func TestGrokImagePersistenceRemoteStatusConstructorKeepsExistingSafeDiagnostic(t *testing.T) {
	original := newGrokImagePersistenceErrorWithRemoteStatus(
		grokImageStageRemoteFetch,
		"non_success_status",
		"https://imgen.x.ai/result.png?signature=first-secret",
		http.StatusTooManyRequests,
		errors.New("provider body first-secret"),
	)
	wrapped := fmt.Errorf("authorization=Bearer-second-secret: %w", original)

	rewrapped := newGrokImagePersistenceErrorWithRemoteStatus(
		grokImageStageRemoteFetch,
		"non_success_status",
		"https://imgen.x.ai/result.png?signature=second-secret",
		http.StatusBadGateway,
		wrapped,
	)

	require.Equal(t, http.StatusTooManyRequests, GrokImagePersistenceRemoteStatus(rewrapped))
	require.NotContains(t, rewrapped.Error(), "Bearer-second-secret")
	require.NotContains(t, rewrapped.Error(), "first-secret")
	require.NotContains(t, rewrapped.Error(), "second-secret")
}

func TestGrokImagePersistenceGenericMediaWrapperKeepsLegacyOuterError(t *testing.T) {
	original := newGrokImagePersistenceError(
		grokImageStageCOSHead,
		"head_failed",
		"https://imgen.x.ai/result.png",
		errors.New("head failed"),
	)
	wrapped := fmt.Errorf("legacy outer context: %w", original)

	result := grokImagePersistenceErrorForMedia(
		"image",
		grokImageStageCOSPut,
		"put_failed",
		"https://imgen.x.ai/result.png",
		wrapped,
	)

	require.Equal(t, wrapped, result)
}
