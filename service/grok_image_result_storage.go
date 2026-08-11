package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

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
	persist  func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error)
	sign     func(context.Context, string, time.Duration) (string, error)
	rollback func(context.Context, string) error
	now      func() time.Time
}

func PersistGrokImageResults(ctx context.Context, request GrokImagePersistenceRequest) ([]GrokImagePersistedResult, error) {
	return persistGrokImageResults(ctx, request, grokImageResultPersistenceDeps{
		persist:  PersistGrokResultWithStatus,
		sign:     SignCOSObjectURL,
		rollback: DeleteGrokResultObject,
		now:      time.Now,
	})
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
