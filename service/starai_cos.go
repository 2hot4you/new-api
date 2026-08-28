package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/go-redis/redis/v8"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

const (
	starAICOSUploadIntentPrefix = "starai:cos-upload:"
	starAICOSCleanupIndexKey    = "starai:cos:cleanup"
	starAICOSOrphanRetention    = 2 * time.Hour
	starAICOSCleanupInterval    = 10 * time.Minute
)

var (
	ErrStarAICOSUnavailable    = ErrObjectStorageUnavailable
	ErrStarAICOSUploadNotFound = errors.New("COS upload authorization not found or expired")
	ErrStarAICOSUploadInvalid  = errors.New("COS uploaded object does not match the authorization")
)

type StarAICOSUploadIntent struct {
	ID          string `json:"id"`
	UserID      int    `json:"user_id"`
	ObjectKey   string `json:"object_key"`
	AssetType   string `json:"asset_type"`
	Name        string `json:"name"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	FileSize    int64  `json:"file_size"`
	ExpiresAt   int64  `json:"expires_at"`
}

type StarAICOSUploadAuthorization struct {
	UploadID  string            `json:"upload_id"`
	UploadURL string            `json:"upload_url"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt int64             `json:"expires_at"`
}

func starAICOSFileRules(assetType string) (map[string]struct{}, int64) {
	switch assetType {
	case "image":
		return map[string]struct{}{".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {}, ".bmp": {}, ".tif": {}, ".tiff": {}, ".gif": {}}, 30 * 1024 * 1024
	case "video":
		return map[string]struct{}{".mp4": {}, ".mov": {}}, 50 * 1024 * 1024
	case "audio":
		return map[string]struct{}{".wav": {}, ".mp3": {}}, 15 * 1024 * 1024
	default:
		return nil, 0
	}
}

func ValidateStarAICOSUpload(fileName, contentType, assetType string, fileSize int64) error {
	assetType = strings.ToLower(strings.TrimSpace(assetType))
	extensions, maxSize := starAICOSFileRules(assetType)
	if extensions == nil {
		return errors.New("unsupported asset type")
	}
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	if _, ok := extensions[extension]; !ok {
		return fmt.Errorf("unsupported %s file extension", assetType)
	}
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if contentType == "" || (!strings.HasPrefix(contentType, assetType+"/") && contentType != "application/octet-stream") {
		return fmt.Errorf("file content type does not match %s", assetType)
	}
	if fileSize <= 0 || fileSize > maxSize {
		return fmt.Errorf("%s file size exceeds the allowed limit", assetType)
	}
	return nil
}

func newStarAICOSClient(config operation_setting.COSConfig) (*cos.Client, error) {
	return newObjectStorageCOSClient(config)
}

func BeginStarAICOSUpload(ctx context.Context, userID int, fileName, contentType, assetType, name string, fileSize int64) (*StarAICOSUploadAuthorization, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, ErrStarAICOSUnavailable
	}
	if err := ValidateStarAICOSUpload(fileName, contentType, assetType, fileSize); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return nil, errors.New("name is required and must not exceed 80 characters")
	}
	config := operation_setting.GetCOSConfig()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStarAICOSUnavailable, err)
	}
	client, err := newStarAICOSClient(config)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	objectKey := path.Join(
		config.PathPrefix,
		strconv.Itoa(userID),
		"starai-assets",
		strings.ToLower(assetType),
		now.Format("2006"),
		now.Format("01"),
		strings.ToLower(common.GetRandomString(32))+extension,
	)
	uploadID := "upload-molii-" + strings.ToLower(common.GetRandomString(24))
	expires := time.Duration(config.UploadExpiryMinutes) * time.Minute
	headers := http.Header{}
	headers.Set("Content-Type", contentType)
	uploadURL, err := client.Object.GetPresignedURL(ctx, http.MethodPut, objectKey, config.SecretID, config.SecretKey, expires, &cos.PresignedURLOptions{Header: &headers})
	if err != nil {
		return nil, fmt.Errorf("%w: failed to sign upload", ErrStarAICOSUnavailable)
	}
	intent := StarAICOSUploadIntent{
		ID:          uploadID,
		UserID:      userID,
		ObjectKey:   objectKey,
		AssetType:   strings.ToLower(assetType),
		Name:        name,
		FileName:    filepath.Base(fileName),
		ContentType: contentType,
		FileSize:    fileSize,
		ExpiresAt:   now.Add(expires).Unix(),
	}
	body, err := common.Marshal(intent)
	if err != nil {
		return nil, err
	}
	pipe := common.RDB.TxPipeline()
	pipe.Set(ctx, starAICOSUploadIntentPrefix+uploadID, body, expires)
	pipe.ZAdd(ctx, starAICOSCleanupIndexKey, &redis.Z{Score: float64(now.Add(starAICOSOrphanRetention).Unix()), Member: objectKey})
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	return &StarAICOSUploadAuthorization{
		UploadID:  uploadID,
		UploadURL: uploadURL.String(),
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: intent.ExpiresAt,
	}, nil
}

func GetStarAICOSUploadIntent(ctx context.Context, uploadID string, userID int) (*StarAICOSUploadIntent, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, ErrStarAICOSUnavailable
	}
	body, err := common.RDB.Get(ctx, starAICOSUploadIntentPrefix+uploadID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrStarAICOSUploadNotFound
	}
	if err != nil {
		return nil, err
	}
	var intent StarAICOSUploadIntent
	if err := common.Unmarshal(body, &intent); err != nil {
		return nil, err
	}
	if intent.UserID != userID {
		return nil, ErrStarAIAssetForbidden
	}
	return &intent, nil
}

func VerifyStarAICOSUpload(ctx context.Context, uploadID string, userID int) (*StarAICOSUploadIntent, string, error) {
	intent, err := GetStarAICOSUploadIntent(ctx, uploadID, userID)
	if err != nil {
		return nil, "", err
	}
	config := operation_setting.GetCOSConfig()
	client, err := newStarAICOSClient(config)
	if err != nil {
		return nil, "", err
	}
	response, err := client.Object.Head(ctx, intent.ObjectKey, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w: uploaded object not found", ErrStarAICOSUploadInvalid)
	}
	if response.Body != nil {
		defer response.Body.Close()
	}
	if response.ContentLength != intent.FileSize {
		_ = DeleteStarAICOSObject(ctx, intent.ObjectKey)
		return nil, "", fmt.Errorf("%w: uploaded object size mismatch", ErrStarAICOSUploadInvalid)
	}
	readURL, err := signStarAICOSObjectURL(ctx, client, config, intent.ObjectKey)
	if err != nil {
		return nil, "", err
	}
	return intent, readURL, nil
}

func signStarAICOSObjectURL(ctx context.Context, client *cos.Client, config operation_setting.COSConfig, objectKey string) (string, error) {
	return signCOSObjectURLWithClient(ctx, client, config, objectKey, time.Duration(config.ReadExpiryMinutes)*time.Minute)
}

func GetStarAICOSPreviewURL(ctx context.Context, objectKey string) (string, error) {
	if strings.TrimSpace(objectKey) == "" {
		return "", nil
	}
	config := operation_setting.GetCOSConfig()
	client, err := newStarAICOSClient(config)
	if err != nil {
		return "", err
	}
	return signStarAICOSObjectURL(ctx, client, config, objectKey)
}

func FinishStarAICOSUpload(ctx context.Context, intent *StarAICOSUploadIntent, assetExpiresAt int64) error {
	if intent == nil || !common.RedisEnabled || common.RDB == nil {
		return ErrStarAICOSUnavailable
	}
	pipe := common.RDB.TxPipeline()
	pipe.Del(ctx, starAICOSUploadIntentPrefix+intent.ID)
	pipe.ZAdd(ctx, starAICOSCleanupIndexKey, &redis.Z{Score: float64(assetExpiresAt), Member: intent.ObjectKey})
	_, err := pipe.Exec(ctx)
	return err
}

func DeleteStarAICOSObject(ctx context.Context, objectKey string) error {
	if strings.TrimSpace(objectKey) == "" {
		return nil
	}
	err := DeleteCOSObject(ctx, objectKey)
	if err == nil && common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.ZRem(ctx, starAICOSCleanupIndexKey, objectKey).Err()
	}
	return err
}

func TestStarAICOSConnection(ctx context.Context) error {
	client, err := newStarAICOSClient(operation_setting.GetCOSConfig())
	if err != nil {
		return err
	}
	response, err := client.Bucket.Head(ctx)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	return err
}

func cleanupExpiredStarAICOSObjects(ctx context.Context) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	objects, err := common.RDB.ZRangeByScore(ctx, starAICOSCleanupIndexKey, &redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(time.Now().Unix(), 10), Offset: 0, Count: 100}).Result()
	if err != nil {
		common.SysError("failed to list expired COS objects: " + err.Error())
		return
	}
	for _, objectKey := range objects {
		if err := DeleteStarAICOSObject(ctx, objectKey); err != nil {
			common.SysError("failed to delete expired COS object: " + err.Error())
		}
	}
}

func StartStarAICOSObjectCleanup() {
	if !common.IsMasterNode || !common.RedisEnabled || common.RDB == nil {
		return
	}
	go func() {
		cleanupExpiredStarAICOSObjects(context.Background())
		ticker := time.NewTicker(starAICOSCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredStarAICOSObjects(context.Background())
		}
	}()
}
