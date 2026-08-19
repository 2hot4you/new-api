package service

import (
	"errors"
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
