package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/go-redis/redis/v8"
)

const (
	grokCOSCleanupIndexKey = "grok:cos:cleanup"
	grokResultRetention    = 24 * time.Hour
	grokCOSCleanupInterval = starAICOSCleanupInterval
)

type GrokResultStoreRequest struct {
	SourceURL      string
	UserID         int
	MediaType      string
	MIMEType       string
	IdempotencyKey string
	CreatedAt      time.Time
}

type grokResultStore struct {
	objectStorage  *objectStorageCOS
	enqueueCleanup func(string, int64) error
}

func BuildGrokResultObjectKey(userID int, mediaType, idempotencyKey string, createdAt time.Time, mimeType string) (string, error) {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if userID <= 0 {
		return "", errors.New("Grok result user ID is invalid")
	}
	if mediaType != "image" && mediaType != "video" {
		return "", errors.New("Grok result media type must be image or video")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return "", errors.New("Grok result idempotency key is required")
	}
	if createdAt.IsZero() {
		return "", errors.New("Grok result creation time is required")
	}
	extension, err := grokResultExtension(mediaType, mimeType)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(idempotencyKey))
	fileName := hex.EncodeToString(digest[:16]) + extension
	createdAt = createdAt.UTC()
	return path.Join(
		operation_setting.GetCOSConfig().PathPrefix,
		"grok-results",
		strconv.Itoa(userID),
		mediaType,
		createdAt.Format("2006"),
		createdAt.Format("01"),
		fileName,
	), nil
}

func grokResultExtension(mediaType, rawMIMEType string) (string, error) {
	mimeType, _, err := mime.ParseMediaType(strings.TrimSpace(rawMIMEType))
	if err != nil {
		return "", errors.New("Grok result MIME type is invalid")
	}
	mimeType = strings.ToLower(mimeType)
	if mediaType == "video" {
		if mimeType != "video/mp4" {
			return "", errors.New("Grok video results must be MP4")
		}
		return ".mp4", nil
	}
	switch mimeType {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	case "image/gif":
		return ".gif", nil
	case "image/avif":
		return ".avif", nil
	default:
		return "", fmt.Errorf("unsupported Grok image MIME type %q", mimeType)
	}
}

func PersistGrokResult(ctx context.Context, request GrokResultStoreRequest) (*StoredObject, error) {
	objectStorage, err := newObjectStorageCOSForPersistence(operation_setting.GetCOSConfig())
	if err != nil {
		return nil, err
	}
	store := &grokResultStore{objectStorage: objectStorage, enqueueCleanup: EnqueueGrokObjectCleanup}
	return store.persist(ctx, request)
}

func (store *grokResultStore) persist(ctx context.Context, request GrokResultStoreRequest) (*StoredObject, error) {
	if store == nil || store.objectStorage == nil || store.enqueueCleanup == nil {
		return nil, ErrObjectStorageUnavailable
	}
	objectKey, err := BuildGrokResultObjectKey(request.UserID, request.MediaType, request.IdempotencyKey, request.CreatedAt, request.MIMEType)
	if err != nil {
		return nil, err
	}
	expiresAt := request.CreatedAt.Add(grokResultRetention).Unix()
	keySpec := ObjectKeySpec{
		ObjectKey: objectKey,
		MediaType: strings.ToLower(strings.TrimSpace(request.MediaType)),
		MIMEType:  request.MIMEType,
		ExpiresAt: expiresAt,
	}
	if existing, found, err := headStoredCOSObject(ctx, store.objectStorage.client, keySpec); err != nil {
		return nil, err
	} else if found {
		if err := store.enqueueCleanup(existing.ObjectKey, existing.ExpiresAt); err != nil {
			return nil, err
		}
		return existing, nil
	}
	// Queue the deterministic key before the first COS write. A process crash,
	// ambiguous PUT result, or another node's concurrent write can therefore
	// never produce an object without a durable cleanup record.
	if err := store.enqueueCleanup(objectKey, expiresAt); err != nil {
		return nil, err
	}
	stored, _, err := store.objectStorage.copyRemoteObjectToCOSWithStatus(ctx, request.SourceURL, keySpec)
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func EnqueueGrokObjectCleanup(objectKey string, expiresAt int64) error {
	if strings.TrimSpace(objectKey) == "" || expiresAt <= 0 {
		return errors.New("Grok cleanup object key and expiry are required")
	}
	if !common.RedisEnabled || common.RDB == nil {
		return ErrObjectStorageUnavailable
	}
	const enqueueWithoutExtension = `
local current = redis.call('ZSCORE', KEYS[1], ARGV[2])
if (not current) or (tonumber(ARGV[1]) < tonumber(current)) then
  return redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
end
return 0
`
	return common.RDB.Eval(context.Background(), enqueueWithoutExtension, []string{grokCOSCleanupIndexKey}, expiresAt, objectKey).Err()
}

func cleanupExpiredGrokObjects(ctx context.Context) {
	cleanupExpiredGrokObjectsWithDelete(ctx, DeleteCOSObject)
}

func cleanupExpiredGrokObjectsWithDelete(ctx context.Context, deleteObject func(context.Context, string) error) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	if deleteObject == nil {
		return
	}
	objects, err := common.RDB.ZRangeByScore(ctx, grokCOSCleanupIndexKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(time.Now().Unix(), 10),
		Offset: 0,
		Count:  100,
	}).Result()
	if err != nil {
		common.SysError("failed to list expired Grok COS objects: " + err.Error())
		return
	}
	for _, objectKey := range objects {
		if err := deleteObject(ctx, objectKey); err != nil {
			common.SysError("failed to delete expired Grok COS object: " + err.Error())
			continue
		}
		if err := common.RDB.ZRem(ctx, grokCOSCleanupIndexKey, objectKey).Err(); err != nil {
			common.SysError("failed to remove expired Grok COS cleanup entry: " + err.Error())
		}
	}
}

func StartGrokCOSObjectCleanup() {
	if !common.IsMasterNode || !common.RedisEnabled || common.RDB == nil {
		return
	}
	go func() {
		cleanupExpiredGrokObjects(context.Background())
		ticker := time.NewTicker(grokCOSCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredGrokObjects(context.Background())
		}
	}()
}
