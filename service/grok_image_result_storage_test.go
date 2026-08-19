package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
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
		now: func() time.Time { return now },
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

func TestPersistGrokImageResultsLeavesFailedBatchObjectsForBoundedCleanup(t *testing.T) {
	createdAt := time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC)
	copyCount := 0
	copyErr := errors.New("copy failed with https://upstream.invalid/result?token=secret")
	deps := grokImageResultPersistenceDeps{
		persist: func(_ context.Context, request GrokResultStoreRequest) (*StoredObject, bool, error) {
			copyCount++
			if copyCount == 3 {
				return nil, false, copyErr
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

	require.ErrorIs(t, err, copyErr)
	stage, category, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, grokImageStageRemoteFetch, stage)
	require.Equal(t, "persistence_failed", category)
	require.Equal(t, "upstream.invalid", sourceHost)
	require.NotContains(t, err.Error(), "token=secret")
	require.Nil(t, results)
	require.Equal(t, 3, copyCount, "already persisted objects remain indexed for the 24-hour cleanup worker")
}

func TestPersistGrokImageResultsReturnsFailureWithoutDeletingWhenSigningFails(t *testing.T) {
	createdAt := time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC)
	signErr := errors.New("sign failed with COS secret")
	deps := grokImageResultPersistenceDeps{
		persist: func(_ context.Context, request GrokResultStoreRequest) (*StoredObject, bool, error) {
			return &StoredObject{ObjectKey: request.IdempotencyKey, MIMEType: request.MIMEType, ExpiresAt: createdAt.Add(24 * time.Hour).Unix()}, true, nil
		},
		sign: func(context.Context, string, time.Duration) (string, error) {
			return "", signErr
		},
		now: func() time.Time { return createdAt },
	}

	results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_sign",
		CreatedAt: createdAt,
		Images:    []GrokImageSource{{URL: "https://upstream.invalid/one.png", MIMEType: "image/png"}},
	}, deps)

	require.ErrorIs(t, err, signErr)
	stage, category, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, grokImageStageCOSSign, stage)
	require.Equal(t, "sign_failed", category)
	require.Equal(t, "upstream.invalid", sourceHost)
	require.NotContains(t, err.Error(), "COS secret")
	require.Nil(t, results)
}

func TestGrokImagePersistenceLockPreventsPeerReuseUntilOwnerFinishes(t *testing.T) {
	useStarAIAssetRedis(t)
	request := GrokImagePersistenceRequest{UserID: 42, RequestID: "req_shared_lock"}
	ownerStarted := make(chan struct{})
	releaseOwner := make(chan struct{})
	ownerDone := make(chan error, 1)
	go func() {
		ownerDone <- withGrokImagePersistenceLock(context.Background(), request, func(_ context.Context) error {
			close(ownerStarted)
			<-releaseOwner
			return nil
		})
	}()
	<-ownerStarted

	var peerEntered atomic.Bool
	peerErr := withGrokImagePersistenceLock(context.Background(), request, func(context.Context) error {
		peerEntered.Store(true)
		return nil
	})
	require.Error(t, peerErr)
	require.False(t, peerEntered.Load())
	close(releaseOwner)
	require.NoError(t, <-ownerDone)

	require.NoError(t, withGrokImagePersistenceLock(context.Background(), request, func(_ context.Context) error {
		peerEntered.Store(true)
		return nil
	}))
	require.True(t, peerEntered.Load())
}

func TestPersistGrokImageResultsRejectsInvalidPersistedMetadata(t *testing.T) {
	createdAt := time.Date(2026, time.August, 11, 8, 0, 0, 0, time.UTC)
	deps := grokImageResultPersistenceDeps{
		persist: func(_ context.Context, request GrokResultStoreRequest) (*StoredObject, bool, error) {
			return &StoredObject{ObjectKey: request.IdempotencyKey, MIMEType: request.MIMEType}, true, nil
		},
		sign: func(context.Context, string, time.Duration) (string, error) {
			t.Fatal("invalid persisted metadata must not be signed")
			return "", nil
		},
		now: func() time.Time { return createdAt },
	}

	results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_invalid_metadata",
		CreatedAt: createdAt,
		Images:    []GrokImageSource{{URL: "https://upstream.invalid/one.png", MIMEType: "image/png"}},
	}, deps)

	stage, category, _, ok := GrokImagePersistenceErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, grokImageStageCOSHead, stage)
	require.Equal(t, "invalid_stored_metadata", category)
	require.Nil(t, results)
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
				sign: func(context.Context, string, time.Duration) (string, error) { return "", nil },
				now:  time.Now,
			}

			results, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
				UserID:    42,
				RequestID: "req_bad_mime",
				CreatedAt: time.Now(),
				Images:    []GrokImageSource{{URL: "https://upstream.invalid/image", MIMEType: mimeType}},
			}, deps)

			require.Error(t, err)
			stage, category, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
			require.True(t, ok)
			require.Equal(t, grokImageStageValidateSourceMIME, stage)
			require.Equal(t, "invalid_source_mime", category)
			require.Equal(t, "upstream.invalid", sourceHost)
			require.Nil(t, results)
			require.False(t, persistCalled)
		})
	}
}

func TestPersistGrokImageResultsRejectsUnsafeSourceURLWithSafeStage(t *testing.T) {
	persistCalled := false
	deps := grokImageResultPersistenceDeps{
		persist: func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error) {
			persistCalled = true
			return nil, false, nil
		},
		sign: func(context.Context, string, time.Duration) (string, error) { return "", nil },
		now:  time.Now,
	}

	_, err := persistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_bad_url",
		CreatedAt: time.Now(),
		Images: []GrokImageSource{{
			URL:      "https://user:password@images.example/result.png?token=upstream-secret",
			MIMEType: "image/png",
		}},
	}, deps)

	require.False(t, persistCalled)
	stage, category, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, grokImageStageValidateSourceURL, stage)
	require.Equal(t, "invalid_source_url", category)
	require.Equal(t, "images.example", sourceHost)
	require.NotContains(t, err.Error(), "password")
	require.NotContains(t, err.Error(), "upstream-secret")
}

func TestPersistGrokImageResultsReportsSafeRedisLockStage(t *testing.T) {
	previousEnabled := common.RedisEnabled
	previousRDB := common.RDB
	common.RedisEnabled = false
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = previousEnabled
		common.RDB = previousRDB
	})

	_, err := PersistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_lock_stage",
		CreatedAt: time.Now(),
		Images: []GrokImageSource{{
			URL:      "https://images.example/result.png?token=upstream-secret",
			MIMEType: "image/png",
		}},
	})

	stage, category, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, grokImageStageRedisLock, stage)
	require.Equal(t, "storage_unavailable", category)
	require.Equal(t, "images.example", sourceHost)
	require.NotContains(t, err.Error(), "upstream-secret")
}

func TestPersistGrokImageResultsClassifiesInvalidOwnershipBeforeRedisLock(t *testing.T) {
	_, err := PersistGrokImageResults(context.Background(), GrokImagePersistenceRequest{
		RequestID: "req_invalid_owner",
		CreatedAt: time.Now(),
		Images: []GrokImageSource{{
			URL:      "https://images.example/result.png?token=upstream-secret",
			MIMEType: "image/png",
		}},
	})

	stage, category, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, grokImageStageValidateSourceURL, stage)
	require.Equal(t, "invalid_request_metadata", category)
	require.Equal(t, "images.example", sourceHost)
}

func TestGrokImagePersistenceLockClassifiesLeaseLossAsRedisFailure(t *testing.T) {
	useStarAIAssetRedis(t)
	request := GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_lease_loss",
		Images: []GrokImageSource{{
			URL:      "https://images.example/result.png?token=upstream-secret",
			MIMEType: "image/png",
		}},
	}
	callbackStarted := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- withGrokImagePersistenceLockDiagnosticTimings(context.Background(), request, 200*time.Millisecond, 10*time.Millisecond, func(lockedContext context.Context) error {
			close(callbackStarted)
			<-lockedContext.Done()
			return lockedContext.Err()
		})
	}()
	<-callbackStarted
	require.NoError(t, common.RDB.FlushDB(context.Background()).Err())

	err := <-result
	stage, category, sourceHost, ok := GrokImagePersistenceErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, grokImageStageRedisLock, stage)
	require.Equal(t, "lease_lost", category)
	require.Equal(t, "images.example", sourceHost)
	require.NotContains(t, err.Error(), "upstream-secret")
}

func TestGrokImagePersistenceLockLeaseLossDoesNotTurnSuccessIntoFailure(t *testing.T) {
	useStarAIAssetRedis(t)
	request := GrokImagePersistenceRequest{
		UserID:    42,
		RequestID: "req_lease_loss_after_success",
		Images: []GrokImageSource{{
			URL:      "https://images.example/result.png",
			MIMEType: "image/png",
		}},
	}
	callbackStarted := make(chan struct{})
	releaseCallback := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- withGrokImagePersistenceLockDiagnosticTimings(context.Background(), request, 200*time.Millisecond, 10*time.Millisecond, func(context.Context) error {
			close(callbackStarted)
			<-releaseCallback
			return nil
		})
	}()
	<-callbackStarted
	require.NoError(t, common.RDB.FlushDB(context.Background()).Err())
	time.Sleep(30 * time.Millisecond)
	close(releaseCallback)

	require.NoError(t, <-result)
}

func TestGrokImagePersistenceLeaseLossClassificationIgnoresCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.False(t, isGrokImagePersistenceLeaseLoss(ctx, context.Canceled, 0, false))
	require.False(t, isGrokImagePersistenceLeaseLoss(context.Background(), context.Canceled, 0, true))
	require.True(t, isGrokImagePersistenceLeaseLoss(context.Background(), errors.New("redis unavailable"), 0, false))
	require.True(t, isGrokImagePersistenceLeaseLoss(context.Background(), nil, 0, false))
	require.False(t, isGrokImagePersistenceLeaseLoss(context.Background(), nil, 1, false))
}

func TestPersistGrokImageResultsValidatesEveryItemBeforeFirstCopy(t *testing.T) {
	persistCalled := false
	deps := grokImageResultPersistenceDeps{
		persist: func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error) {
			persistCalled = true
			return nil, false, nil
		},
		sign: func(context.Context, string, time.Duration) (string, error) { return "", nil },
		now:  time.Now,
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
