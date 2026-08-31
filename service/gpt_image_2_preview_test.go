package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGPTImage2PreviewStoresOwnedObjectMetadataAndSignsRemainingLifetime(t *testing.T) {
	server := useGrokImagePreviewRedis(t)
	useStarAICOSTestConfig(t)
	now := time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC)
	object := GPTImage2PreviewObject{
		ObjectKey: "users/gpt-image-2-results/42/2026/08/result.png",
		MIMEType:  "image/png",
		ExpiresAt: now.Add(24 * time.Hour).Unix(),
	}

	require.NoError(t, registerGPTImage2Preview(42, "req-preview", []GPTImage2PreviewObject{object}, now))
	key := gptImage2PreviewKey(42, "req-preview")
	assert.NotContains(t, key, "req-preview")
	assert.Equal(t, 24*time.Hour, server.TTL(key))
	stored, err := common.RDB.Get(context.Background(), key).Result()
	require.NoError(t, err)
	assert.Contains(t, stored, object.ObjectKey)
	assert.NotContains(t, stored, "https://")

	var signedTTL time.Duration
	urls, err := getGPTImage2Preview(42, "req-preview", now.Add(2*time.Hour), func(_ context.Context, objectKey string, ttl time.Duration) (string, error) {
		assert.Equal(t, object.ObjectKey, objectKey)
		signedTTL = ttl
		return "https://cos.example/signed-result", nil
	})
	require.NoError(t, err)
	assert.Equal(t, 22*time.Hour, signedTTL)
	assert.Equal(t, []string{"https://cos.example/signed-result"}, urls)
}

func TestGPTImage2PreviewRejectsCrossUserExpiredAndMalformedObjects(t *testing.T) {
	useGrokImagePreviewRedis(t)
	useStarAICOSTestConfig(t)
	now := time.Now().UTC()
	valid := GPTImage2PreviewObject{
		ObjectKey: "users/gpt-image-2-results/42/2026/08/result.png",
		MIMEType:  "image/png",
		ExpiresAt: now.Add(time.Hour).Unix(),
	}

	for name, objects := range map[string][]GPTImage2PreviewObject{
		"cross-user": {{ObjectKey: "users/gpt-image-2-results/7/2026/08/result.png", MIMEType: "image/png", ExpiresAt: valid.ExpiresAt}},
		"expired":    {{ObjectKey: valid.ObjectKey, MIMEType: "image/png", ExpiresAt: now.Add(-time.Second).Unix()}},
		"too-many":   make([]GPTImage2PreviewObject, 11),
	} {
		t.Run(name, func(t *testing.T) {
			err := registerGPTImage2Preview(42, "req-"+name, objects, now)
			require.ErrorIs(t, err, ErrGPTImage2PreviewUnavailable)
		})
	}

	require.NoError(t, registerGPTImage2Preview(42, "req-expire-after-store", []GPTImage2PreviewObject{valid}, now))
	_, err := getGPTImage2Preview(42, "req-expire-after-store", now.Add(2*time.Hour), func(context.Context, string, time.Duration) (string, error) {
		t.Fatal("expired preview must not be signed")
		return "", nil
	})
	require.ErrorIs(t, err, ErrGPTImage2PreviewNotFound)
}

func TestGPTImage2PreviewCollapsesSigningAndRedisFailures(t *testing.T) {
	useGrokImagePreviewRedis(t)
	useStarAICOSTestConfig(t)
	now := time.Now().UTC()
	object := GPTImage2PreviewObject{
		ObjectKey: "users/gpt-image-2-results/42/2026/08/result.png",
		MIMEType:  "image/png",
		ExpiresAt: now.Add(time.Hour).Unix(),
	}
	require.NoError(t, registerGPTImage2Preview(42, "req-sign-error", []GPTImage2PreviewObject{object}, now))

	_, err := getGPTImage2Preview(42, "req-sign-error", now, func(context.Context, string, time.Duration) (string, error) {
		return "", errors.New("secret cos signing failure")
	})
	require.ErrorIs(t, err, ErrGPTImage2PreviewNotFound)
	assert.NotContains(t, err.Error(), "secret")

	common.RedisEnabled = false
	_, err = GetGPTImage2Preview(42, "req-sign-error")
	require.ErrorIs(t, err, ErrGPTImage2PreviewNotFound)
}
