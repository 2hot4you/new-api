package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPersistGrokImageResultsPreservesMetadataAndSignsRemainingLifetime(t *testing.T) {
	createdAt := time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC)
	now := createdAt.Add(2 * time.Hour)
	var persisted []GrokResultStoreRequest
	var signedTTL []time.Duration
	deps := grokImageResultPersistenceDeps{
		persist: func(_ context.Context, request GrokResultStoreRequest) (*StoredObject, bool, error) {
			persisted = append(persisted, request)
			return &StoredObject{ObjectKey: "grok/image/one.jpg", MIMEType: request.MIMEType, ExpiresAt: createdAt.Add(24 * time.Hour).Unix()}, true, nil
		},
		sign: func(_ context.Context, objectKey string, ttl time.Duration) (string, error) {
			require.Equal(t, "grok/image/one.jpg", objectKey)
			signedTTL = append(signedTTL, ttl)
			return "https://cos.example/signed-one", nil
		},
		rollback: func(context.Context, string) error { return nil },
		now:      func() time.Time { return now },
	}

	results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_public_123",
		CreatedAt: createdAt,
		Images: []GrokImageSource{{
			URL:           "https://upstream.invalid/secret.jpg",
			MIMEType:      "image/jpeg",
			RevisedPrompt: "a revised prompt",
		}},
	}, deps)

	require.NoError(t, err)
	require.Equal(t, []GrokResultStoreRequest{{
		SourceURL:      "https://upstream.invalid/secret.jpg",
		UserID:         42,
		MediaType:      "image",
		MIMEType:       "image/jpeg",
		IdempotencyKey: "req_public_123:image:0",
		CreatedAt:      createdAt,
	}}, persisted)
	require.Equal(t, []time.Duration{22 * time.Hour}, signedTTL)
	require.Equal(t, []GrokImagePersistedResult{{
		URL:           "https://cos.example/signed-one",
		MIMEType:      "image/jpeg",
		RevisedPrompt: "a revised prompt",
	}}, results)
}

func TestPersistGrokImageResultsRollsBackOnlyNewObjectsWhenLaterCopyFails(t *testing.T) {
	createdAt := time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC)
	copyCount := 0
	var rolledBack []string
	deps := grokImageResultPersistenceDeps{
		persist: func(_ context.Context, request GrokResultStoreRequest) (*StoredObject, bool, error) {
			copyCount++
			if copyCount == 3 {
				return nil, false, errors.New("copy failed")
			}
			return &StoredObject{
				ObjectKey: request.IdempotencyKey,
				MIMEType:  request.MIMEType,
				ExpiresAt: createdAt.Add(24 * time.Hour).Unix(),
			}, copyCount == 1, nil
		},
		sign: func(_ context.Context, objectKey string, _ time.Duration) (string, error) {
			return "https://cos.example/" + objectKey, nil
		},
		rollback: func(_ context.Context, objectKey string) error {
			rolledBack = append(rolledBack, objectKey)
			return nil
		},
		now: func() time.Time { return createdAt },
	}

	results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_multi",
		CreatedAt: createdAt,
		Images: []GrokImageSource{
			{URL: "https://upstream.invalid/one.jpg", MIMEType: "image/jpeg"},
			{URL: "https://upstream.invalid/two.jpg", MIMEType: "image/jpeg"},
			{URL: "https://upstream.invalid/three.jpg", MIMEType: "image/jpeg"},
		},
	}, deps)

	require.ErrorContains(t, err, "copy failed")
	require.Nil(t, results)
	require.Equal(t, []string{"req_multi:image:0"}, rolledBack, "a reused object must never be rolled back")
}

func TestPersistGrokImageResultsRollsBackNewObjectWhenSigningFails(t *testing.T) {
	createdAt := time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC)
	var rolledBack []string
	deps := grokImageResultPersistenceDeps{
		persist: func(_ context.Context, request GrokResultStoreRequest) (*StoredObject, bool, error) {
			return &StoredObject{ObjectKey: request.IdempotencyKey, MIMEType: request.MIMEType, ExpiresAt: createdAt.Add(24 * time.Hour).Unix()}, true, nil
		},
		sign: func(context.Context, string, time.Duration) (string, error) {
			return "", errors.New("sign failed")
		},
		rollback: func(_ context.Context, objectKey string) error {
			rolledBack = append(rolledBack, objectKey)
			return nil
		},
		now: func() time.Time { return createdAt },
	}

	results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_sign",
		CreatedAt: createdAt,
		Images:    []GrokImageSource{{URL: "https://upstream.invalid/one.png", MIMEType: "image/png"}},
	}, deps)

	require.ErrorContains(t, err, "sign failed")
	require.Nil(t, results)
	require.Equal(t, []string{"req_sign:image:0"}, rolledBack)
}

func TestPersistGrokImageResultsRollsBackCreatedObjectWithInvalidMetadata(t *testing.T) {
	createdAt := time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC)
	var rolledBack []string
	deps := grokImageResultPersistenceDeps{
		persist: func(_ context.Context, request GrokResultStoreRequest) (*StoredObject, bool, error) {
			return &StoredObject{ObjectKey: request.IdempotencyKey, MIMEType: request.MIMEType}, true, nil
		},
		sign: func(context.Context, string, time.Duration) (string, error) {
			t.Fatal("invalid persisted metadata must not be signed")
			return "", nil
		},
		rollback: func(_ context.Context, objectKey string) error {
			rolledBack = append(rolledBack, objectKey)
			return nil
		},
		now: func() time.Time { return createdAt },
	}

	results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_invalid_metadata",
		CreatedAt: createdAt,
		Images:    []GrokImageSource{{URL: "https://upstream.invalid/one.png", MIMEType: "image/png"}},
	}, deps)

	require.ErrorContains(t, err, "metadata is invalid")
	require.Nil(t, results)
	require.Equal(t, []string{"req_invalid_metadata:image:0"}, rolledBack)
}

func TestPersistGrokImageResultsRejectsMissingOrUnsupportedMIMEBeforeCopy(t *testing.T) {
	for _, mimeType := range []string{"", "application/octet-stream"} {
		t.Run(mimeType, func(t *testing.T) {
			persistCalled := false
			deps := grokImageResultPersistenceDeps{
				persist: func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error) {
					persistCalled = true
					return nil, false, nil
				},
				sign:     func(context.Context, string, time.Duration) (string, error) { return "", nil },
				rollback: func(context.Context, string) error { return nil },
				now:      time.Now,
			}

			results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
				UserID:    42,
				RequestID: "req_bad_mime",
				CreatedAt: time.Now(),
				Images:    []GrokImageSource{{URL: "https://upstream.invalid/image", MIMEType: mimeType}},
			}, deps)

			require.Error(t, err)
			require.Nil(t, results)
			require.False(t, persistCalled)
		})
	}
}

func TestPersistGrokImageResultsValidatesEveryItemBeforeFirstCopy(t *testing.T) {
	persistCalled := false
	deps := grokImageResultPersistenceDeps{
		persist: func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error) {
			persistCalled = true
			return nil, false, nil
		},
		sign:     func(context.Context, string, time.Duration) (string, error) { return "", nil },
		rollback: func(context.Context, string) error { return nil },
		now:      time.Now,
	}

	results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_validate_all",
		CreatedAt: time.Now(),
		Images: []GrokImageSource{
			{URL: "https://upstream.invalid/valid.jpg", MIMEType: "image/jpeg"},
			{URL: "https://upstream.invalid/invalid.bin", MIMEType: "application/octet-stream"},
		},
	}, deps)

	require.Error(t, err)
	require.Nil(t, results)
	require.False(t, persistCalled, "no image may be copied until the whole upstream result is valid")
}
