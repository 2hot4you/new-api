package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	grokImagePersistenceLockPrefix  = "grok:image:persistence:lock:"
	grokImagePersistenceLockLease   = 2 * time.Minute
	grokImagePersistenceLockRefresh = 30 * time.Second
)

const grokImagePersistenceLockRefreshScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 0
`

const grokImagePersistenceLockReleaseScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`

type GrokImageSource struct {
	URL           string
	MIMEType      string
	RevisedPrompt string
}

type GrokImagePersistenceRequest struct {
	UserID    int
	RequestID string
	CreatedAt time.Time
	Images    []GrokImageSource
}

type GrokImagePersistedResult struct {
	URL           string
	MIMEType      string
	RevisedPrompt string
}

type grokImageResultPersistenceDeps struct {
	persist func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error)
	sign    func(context.Context, string, time.Duration) (string, error)
	now     func() time.Time
}

func PersistGrokImageResults(ctx context.Context, request GrokImagePersistenceRequest) ([]GrokImagePersistedResult, error) {
	var results []GrokImagePersistedResult
	err := withGrokImagePersistenceLockDiagnostic(ctx, request, func(lockedContext context.Context) error {
		var err error
		results, err = persistGrokImageResults(lockedContext, request, grokImageResultPersistenceDeps{
			persist: PersistGrokResultWithStatus,
			sign:    SignCOSObjectURL,
			now:     time.Now,
		})
		return err
	})
	if err != nil {
		if _, _, _, ok := GrokImagePersistenceErrorDetails(err); ok {
			return nil, err
		}
		return nil, newGrokImagePersistenceError(grokImageStageRedisLock, "lock_failed", firstGrokImageSourceURL(request), err)
	}
	return results, nil
}

func persistGrokImageResults(ctx context.Context, request GrokImagePersistenceRequest, deps grokImageResultPersistenceDeps) ([]GrokImagePersistedResult, error) {
	if request.UserID <= 0 || strings.TrimSpace(request.RequestID) == "" || request.CreatedAt.IsZero() {
		return nil, newGrokImagePersistenceError(grokImageStageValidateSourceURL, "invalid_request_metadata", firstGrokImageSourceURL(request), errors.New("Grok image result ownership metadata is invalid"))
	}
	if len(request.Images) < 1 || len(request.Images) > 4 {
		return nil, newGrokImagePersistenceError(grokImageStageParseUpstream, "invalid_image_count", firstGrokImageSourceURL(request), errors.New("Grok image result count is invalid"))
	}
	if deps.persist == nil || deps.sign == nil || deps.now == nil {
		return nil, newGrokImagePersistenceError(grokImageStageCOSPut, "storage_unavailable", firstGrokImageSourceURL(request), ErrObjectStorageUnavailable)
	}
	for _, image := range request.Images {
		if err := validateGrokImageSource(image); err != nil {
			return nil, err
		}
	}

	results := make([]GrokImagePersistedResult, 0, len(request.Images))
	for index, image := range request.Images {
		stored, _, err := deps.persist(ctx, GrokResultStoreRequest{
			SourceURL:      strings.TrimSpace(image.URL),
			UserID:         request.UserID,
			MediaType:      "image",
			MIMEType:       strings.TrimSpace(image.MIMEType),
			IdempotencyKey: fmt.Sprintf("%s:image:%d", strings.TrimSpace(request.RequestID), index),
			CreatedAt:      request.CreatedAt,
		})
		if err != nil {
			if _, _, _, ok := GrokImagePersistenceErrorDetails(err); ok {
				return nil, err
			}
			return nil, newGrokImagePersistenceError(grokImageStageRemoteFetch, "persistence_failed", image.URL, err)
		}
		if stored == nil || stored.ObjectKey == "" {
			return nil, newGrokImagePersistenceError(grokImageStageCOSHead, "invalid_stored_metadata", image.URL, errors.New("persisted Grok image metadata is invalid"))
		}
		if stored.ExpiresAt <= 0 {
			return nil, newGrokImagePersistenceError(grokImageStageCOSHead, "invalid_stored_metadata", image.URL, errors.New("persisted Grok image metadata is invalid"))
		}
		remaining := time.Unix(stored.ExpiresAt, 0).Sub(deps.now())
		if remaining <= 0 {
			return nil, newGrokImagePersistenceError(grokImageStageCOSHead, "stored_object_expired", image.URL, errors.New("persisted Grok image has expired"))
		}
		if remaining > objectStorageMaximumSignTTL {
			remaining = objectStorageMaximumSignTTL
		}
		signedURL, err := deps.sign(ctx, stored.ObjectKey, remaining)
		if err != nil {
			return nil, newGrokImagePersistenceError(grokImageStageCOSSign, "sign_failed", image.URL, err)
		}
		results = append(results, GrokImagePersistedResult{
			URL:           signedURL,
			MIMEType:      stored.MIMEType,
			RevisedPrompt: image.RevisedPrompt,
		})
	}
	return results, nil
}

func withGrokImagePersistenceLock(
	ctx context.Context,
	request GrokImagePersistenceRequest,
	callback func(context.Context) error,
) error {
	return withGrokImagePersistenceLockInternal(ctx, request, grokImagePersistenceLockLease, grokImagePersistenceLockRefresh, false, callback)
}

func withGrokImagePersistenceLockDiagnostic(
	ctx context.Context,
	request GrokImagePersistenceRequest,
	callback func(context.Context) error,
) error {
	return withGrokImagePersistenceLockInternal(ctx, request, grokImagePersistenceLockLease, grokImagePersistenceLockRefresh, true, callback)
}

func withGrokImagePersistenceLockDiagnosticTimings(
	ctx context.Context,
	request GrokImagePersistenceRequest,
	lease time.Duration,
	refreshInterval time.Duration,
	callback func(context.Context) error,
) error {
	return withGrokImagePersistenceLockInternal(ctx, request, lease, refreshInterval, true, callback)
}

func withGrokImagePersistenceLockInternal(
	ctx context.Context,
	request GrokImagePersistenceRequest,
	lease time.Duration,
	refreshInterval time.Duration,
	diagnostics bool,
	callback func(context.Context) error,
) error {
	lockError := func(category string, cause error) error {
		if !diagnostics {
			return cause
		}
		return newGrokImagePersistenceError(grokImageStageRedisLock, category, firstGrokImageSourceURL(request), cause)
	}
	if request.UserID <= 0 || strings.TrimSpace(request.RequestID) == "" || callback == nil {
		cause := errors.New("Grok image result ownership metadata is invalid")
		if !diagnostics {
			return cause
		}
		return newGrokImagePersistenceError(grokImageStageValidateSourceURL, "invalid_request_metadata", firstGrokImageSourceURL(request), cause)
	}
	if !common.RedisEnabled || common.RDB == nil {
		return lockError("storage_unavailable", ErrObjectStorageUnavailable)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	token, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		return lockError("token_generation_failed", errors.New("Grok image result persistence lock is unavailable"))
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", request.UserID, strings.TrimSpace(request.RequestID))))
	lockKey := grokImagePersistenceLockPrefix + hex.EncodeToString(digest[:])
	acquired, err := common.RDB.SetNX(ctx, lockKey, token, lease).Result()
	if err != nil {
		return lockError("acquire_failed", errors.New("Grok image result persistence lock is unavailable"))
	}
	if !acquired {
		return lockError("already_in_progress", errors.New("Grok image result persistence is already in progress"))
	}

	lockedContext, cancel := context.WithCancel(ctx)
	defer cancel()
	var owned atomic.Bool
	var leaseLost atomic.Bool
	var stopping atomic.Bool
	owned.Store(true)
	refreshLease := func(refreshContext context.Context) bool {
		if !owned.Load() {
			return false
		}
		result, refreshErr := common.RDB.Eval(
			refreshContext,
			grokImagePersistenceLockRefreshScript,
			[]string{lockKey},
			token,
			lease.Milliseconds(),
		).Int64()
		if isGrokImagePersistenceLeaseLoss(refreshContext, refreshErr, result, stopping.Load()) {
			leaseLost.Store(true)
			owned.Store(false)
			cancel()
			return false
		}
		if refreshErr != nil || result != 1 {
			return false
		}
		return owned.Load()
	}

	renewDone := make(chan struct{})
	renewExited := make(chan struct{})
	go func() {
		defer close(renewExited)
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-renewDone:
				return
			case <-lockedContext.Done():
				return
			case <-ticker.C:
				if !refreshLease(lockedContext) {
					return
				}
			}
		}
	}()

	callbackErr := callback(lockedContext)
	if diagnostics {
		stopping.Store(true)
		cancel()
	}
	close(renewDone)
	if diagnostics {
		<-renewExited
	}
	wasLeaseLost := diagnostics && leaseLost.Load()
	owned.Store(false)
	releaseContext, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer releaseCancel()
	_, _ = common.RDB.Eval(
		releaseContext,
		grokImagePersistenceLockReleaseScript,
		[]string{lockKey},
		token,
	).Result()
	if wasLeaseLost && callbackErr != nil {
		return &grokImagePersistenceError{
			stage:      grokImageStageRedisLock,
			category:   "lease_lost",
			sourceHost: grokImagePersistenceSourceHost(firstGrokImageSourceURL(request)),
			cause:      callbackErr,
		}
	}
	return callbackErr
}

func isGrokImagePersistenceLeaseLoss(refreshContext context.Context, refreshErr error, result int64, stopping bool) bool {
	if stopping || (refreshContext != nil && refreshContext.Err() != nil) {
		return false
	}
	return refreshErr != nil || result != 1
}

func validateGrokImageSource(image GrokImageSource) error {
	parsed, err := url.Parse(strings.TrimSpace(image.URL))
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return newGrokImagePersistenceError(grokImageStageValidateSourceURL, "invalid_source_url", image.URL, errors.New("Grok image result URL is invalid"))
	}
	if _, err := grokResultExtension("image", image.MIMEType); err != nil {
		return newGrokImagePersistenceError(grokImageStageValidateSourceMIME, "invalid_source_mime", image.URL, err)
	}
	return nil
}

func firstGrokImageSourceURL(request GrokImagePersistenceRequest) string {
	if len(request.Images) == 0 {
		return ""
	}
	return request.Images[0].URL
}
