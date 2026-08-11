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
	persist     func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error)
	sign        func(context.Context, string, time.Duration) (string, error)
	rollback    func(context.Context, string) error
	canRollback func() bool
	now         func() time.Time
}

func PersistGrokImageResults(ctx context.Context, request GrokImagePersistenceRequest) ([]GrokImagePersistedResult, error) {
	var results []GrokImagePersistedResult
	err := withGrokImagePersistenceLock(ctx, request, func(lockedContext context.Context, owns func() bool) error {
		var err error
		results, err = persistGrokImageResults(lockedContext, request, grokImageResultPersistenceDeps{
			persist:     PersistGrokResultWithStatus,
			sign:        SignCOSObjectURL,
			rollback:    DeleteGrokResultObject,
			canRollback: owns,
			now:         time.Now,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func persistGrokImageResults(ctx context.Context, request GrokImagePersistenceRequest, deps grokImageResultPersistenceDeps) ([]GrokImagePersistedResult, error) {
	if request.UserID <= 0 || strings.TrimSpace(request.RequestID) == "" || request.CreatedAt.IsZero() {
		return nil, errors.New("Grok image result ownership metadata is invalid")
	}
	if len(request.Images) < 1 || len(request.Images) > 4 {
		return nil, errors.New("Grok image result count is invalid")
	}
	if deps.persist == nil || deps.sign == nil || deps.rollback == nil || deps.now == nil {
		return nil, ErrObjectStorageUnavailable
	}
	for _, image := range request.Images {
		if err := validateGrokImageSource(image); err != nil {
			return nil, err
		}
	}

	type createdObject struct {
		key string
	}
	created := make([]createdObject, 0, len(request.Images))
	rollback := func() {
		if deps.canRollback != nil && !deps.canRollback() {
			return
		}
		for index := len(created) - 1; index >= 0; index-- {
			rollbackContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = deps.rollback(rollbackContext, created[index].key)
			cancel()
		}
	}
	results := make([]GrokImagePersistedResult, 0, len(request.Images))
	for index, image := range request.Images {
		stored, wasCreated, err := deps.persist(ctx, GrokResultStoreRequest{
			SourceURL:      strings.TrimSpace(image.URL),
			UserID:         request.UserID,
			MediaType:      "image",
			MIMEType:       strings.TrimSpace(image.MIMEType),
			IdempotencyKey: fmt.Sprintf("%s:image:%d", strings.TrimSpace(request.RequestID), index),
			CreatedAt:      request.CreatedAt,
		})
		if err != nil {
			rollback()
			return nil, err
		}
		if stored == nil || stored.ObjectKey == "" {
			rollback()
			return nil, errors.New("persisted Grok image metadata is invalid")
		}
		if wasCreated {
			created = append(created, createdObject{key: stored.ObjectKey})
		}
		if stored.ExpiresAt <= 0 {
			rollback()
			return nil, errors.New("persisted Grok image metadata is invalid")
		}
		remaining := time.Unix(stored.ExpiresAt, 0).Sub(deps.now())
		if remaining <= 0 {
			rollback()
			return nil, errors.New("persisted Grok image has expired")
		}
		if remaining > objectStorageMaximumSignTTL {
			remaining = objectStorageMaximumSignTTL
		}
		signedURL, err := deps.sign(ctx, stored.ObjectKey, remaining)
		if err != nil {
			rollback()
			return nil, err
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
	callback func(context.Context, func() bool) error,
) error {
	if request.UserID <= 0 || strings.TrimSpace(request.RequestID) == "" || callback == nil {
		return errors.New("Grok image result ownership metadata is invalid")
	}
	if !common.RedisEnabled || common.RDB == nil {
		return ErrObjectStorageUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	token, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		return errors.New("Grok image result persistence lock is unavailable")
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", request.UserID, strings.TrimSpace(request.RequestID))))
	lockKey := grokImagePersistenceLockPrefix + hex.EncodeToString(digest[:])
	acquired, err := common.RDB.SetNX(ctx, lockKey, token, grokImagePersistenceLockLease).Result()
	if err != nil {
		return errors.New("Grok image result persistence lock is unavailable")
	}
	if !acquired {
		return errors.New("Grok image result persistence is already in progress")
	}

	lockedContext, cancel := context.WithCancel(ctx)
	defer cancel()
	var owned atomic.Bool
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
			grokImagePersistenceLockLease.Milliseconds(),
		).Int64()
		if refreshErr != nil || result != 1 {
			owned.Store(false)
			cancel()
			return false
		}
		return owned.Load()
	}

	renewDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(grokImagePersistenceLockRefresh)
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

	callbackErr := callback(lockedContext, func() bool {
		// Refreshing here is a fencing step for destructive rollback: a peer
		// cannot acquire the request lock while the bounded COS delete runs.
		ownershipContext, ownershipCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ownershipCancel()
		return refreshLease(ownershipContext)
	})
	close(renewDone)
	owned.Store(false)
	releaseContext, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer releaseCancel()
	_, _ = common.RDB.Eval(
		releaseContext,
		grokImagePersistenceLockReleaseScript,
		[]string{lockKey},
		token,
	).Result()
	return callbackErr
}

func validateGrokImageSource(image GrokImageSource) error {
	parsed, err := url.Parse(strings.TrimSpace(image.URL))
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return errors.New("Grok image result URL is invalid")
	}
	if _, err := grokResultExtension("image", image.MIMEType); err != nil {
		return err
	}
	return nil
}
