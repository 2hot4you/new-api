package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var (
	ErrGPTImage2PreviewUnavailable = errors.New("GPT Image 2 preview is unavailable")
	ErrGPTImage2PreviewNotFound    = errors.New("GPT Image 2 preview was not found")
)

func gptImage2PreviewKey(userID int, requestID string) string {
	payload := fmt.Sprintf("gpt-image-2-preview:v1:%d:%s", userID, strings.TrimSpace(requestID))
	return "gpt:image:2:preview:" + common.GenerateHMACWithKey([]byte(common.SessionSecret), payload)
}

func RegisterGPTImage2Preview(userID int, requestID string, objects []GPTImage2PreviewObject) error {
	return registerGPTImage2Preview(userID, requestID, objects, time.Now())
}

func registerGPTImage2Preview(userID int, requestID string, objects []GPTImage2PreviewObject, now time.Time) error {
	if userID <= 0 || strings.TrimSpace(requestID) == "" || len(objects) < 1 || len(objects) > gptImage2MaximumImages || !common.RedisEnabled || common.RDB == nil {
		return ErrGPTImage2PreviewUnavailable
	}
	earliestExpiry := int64(0)
	for _, object := range objects {
		if !IsOwnedGPTImage2ResultObject(userID, object.ObjectKey) || !isGPTImage2PreviewMIME(object.MIMEType) || object.ExpiresAt <= now.Unix() {
			return ErrGPTImage2PreviewUnavailable
		}
		if earliestExpiry == 0 || object.ExpiresAt < earliestExpiry {
			earliestExpiry = object.ExpiresAt
		}
	}
	ttl := time.Unix(earliestExpiry, 0).Sub(now)
	if ttl <= 0 || ttl > gptImage2ResultRetention {
		return ErrGPTImage2PreviewUnavailable
	}
	body, err := common.Marshal(objects)
	if err != nil {
		return ErrGPTImage2PreviewUnavailable
	}
	client := newGrokImagePreviewRedisClient(common.RDB)
	if client == nil {
		return ErrGPTImage2PreviewUnavailable
	}
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), grokImagePreviewRedisTimeout)
	defer cancel()
	if err := client.Set(ctx, gptImage2PreviewKey(userID, requestID), body, ttl).Err(); err != nil {
		return ErrGPTImage2PreviewUnavailable
	}
	return nil
}

func GetGPTImage2Preview(userID int, requestID string) ([]string, error) {
	return getGPTImage2Preview(userID, requestID, time.Now(), SignCOSObjectURL)
}

func GetGPTImage2PreviewObject(userID int, requestID string, index int) (GPTImage2PreviewObject, error) {
	return getGPTImage2PreviewObject(userID, requestID, index, time.Now())
}

func getGPTImage2Preview(
	userID int,
	requestID string,
	now time.Time,
	sign func(context.Context, string, time.Duration) (string, error),
) ([]string, error) {
	if sign == nil {
		return nil, ErrGPTImage2PreviewNotFound
	}
	objects, err := getGPTImage2PreviewObjects(userID, requestID, now)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), grokImagePreviewRedisTimeout)
	defer cancel()
	urls := make([]string, 0, len(objects))
	for _, object := range objects {
		remaining := time.Unix(object.ExpiresAt, 0).Sub(now)
		if remaining > objectStorageMaximumSignTTL {
			remaining = objectStorageMaximumSignTTL
		}
		signedURL, err := sign(ctx, object.ObjectKey, remaining)
		if err != nil || strings.TrimSpace(signedURL) == "" {
			return nil, ErrGPTImage2PreviewNotFound
		}
		urls = append(urls, signedURL)
	}
	return urls, nil
}

func getGPTImage2PreviewObject(userID int, requestID string, index int, now time.Time) (GPTImage2PreviewObject, error) {
	if index < 0 {
		return GPTImage2PreviewObject{}, ErrGPTImage2PreviewNotFound
	}
	objects, err := getGPTImage2PreviewObjects(userID, requestID, now)
	if err != nil || index >= len(objects) {
		return GPTImage2PreviewObject{}, ErrGPTImage2PreviewNotFound
	}
	return objects[index], nil
}

func getGPTImage2PreviewObjects(userID int, requestID string, now time.Time) ([]GPTImage2PreviewObject, error) {
	if userID <= 0 || strings.TrimSpace(requestID) == "" || !common.RedisEnabled || common.RDB == nil {
		return nil, ErrGPTImage2PreviewNotFound
	}
	client := newGrokImagePreviewRedisClient(common.RDB)
	if client == nil {
		return nil, ErrGPTImage2PreviewNotFound
	}
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), grokImagePreviewRedisTimeout)
	defer cancel()
	body, err := client.Get(ctx, gptImage2PreviewKey(userID, requestID)).Bytes()
	if err != nil {
		return nil, ErrGPTImage2PreviewNotFound
	}
	var objects []GPTImage2PreviewObject
	if err := common.Unmarshal(body, &objects); err != nil || len(objects) < 1 || len(objects) > gptImage2MaximumImages {
		return nil, ErrGPTImage2PreviewNotFound
	}
	for _, object := range objects {
		if !IsOwnedGPTImage2ResultObject(userID, object.ObjectKey) || !isGPTImage2PreviewMIME(object.MIMEType) {
			return nil, ErrGPTImage2PreviewNotFound
		}
		remaining := time.Unix(object.ExpiresAt, 0).Sub(now)
		if remaining <= 0 {
			return nil, ErrGPTImage2PreviewNotFound
		}
	}
	return objects, nil
}

func isGPTImage2PreviewMIME(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png", "image/jpeg", "image/webp":
		return true
	default:
		return false
	}
}
