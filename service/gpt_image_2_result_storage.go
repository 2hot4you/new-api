package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	gptImage2ResultRetention = 24 * time.Hour
	gptImage2MaximumImages   = 10
)

type GPTImage2PersistenceRequest struct {
	UserID       int
	RequestID    string
	CreatedAt    time.Time
	OutputFormat string
	Images       []string
}

type GPTImage2PreviewObject struct {
	ObjectKey string `json:"object_key"`
	MIMEType  string `json:"mime_type"`
	ExpiresAt int64  `json:"expires_at"`
}

type gptImage2ResultPersistenceDeps struct {
	put             func(context.Context, io.Reader, ObjectKeySpec) (*StoredObject, bool, error)
	registerCleanup func(string, int64) error
}

func PersistGPTImage2Results(ctx context.Context, request GPTImage2PersistenceRequest) ([]GPTImage2PreviewObject, error) {
	store, err := newObjectStorageCOSForPersistence(operation_setting.GetCOSConfig())
	if err != nil {
		return nil, err
	}
	var results []GPTImage2PreviewObject
	err = withGrokImagePersistenceLock(ctx, GrokImagePersistenceRequest{
		UserID:    request.UserID,
		RequestID: "gpt-image-2:" + request.RequestID,
	}, func(lockedContext context.Context) error {
		var persistErr error
		results, persistErr = persistGPTImage2Results(lockedContext, request, gptImage2ResultPersistenceDeps{
			put:             store.putReaderObjectToCOSWithStatus,
			registerCleanup: RegisterPendingGrokObjectCleanup,
		})
		return persistErr
	})
	return results, err
}

func persistGPTImage2Results(ctx context.Context, request GPTImage2PersistenceRequest, deps gptImage2ResultPersistenceDeps) ([]GPTImage2PreviewObject, error) {
	if request.UserID <= 0 || strings.TrimSpace(request.RequestID) == "" || request.CreatedAt.IsZero() {
		return nil, errors.New("GPT Image 2 result ownership metadata is invalid")
	}
	if len(request.Images) < 1 || len(request.Images) > gptImage2MaximumImages {
		return nil, errors.New("GPT Image 2 result image count is invalid")
	}
	if deps.put == nil || deps.registerCleanup == nil {
		return nil, ErrObjectStorageUnavailable
	}
	mimeType, extension, err := gptImage2OutputType(request.OutputFormat)
	if err != nil {
		return nil, err
	}
	maxBytes := objectStorageDefaultImageMaxBytes
	for _, image := range request.Images {
		if err := validateGPTImage2Base64(image, maxBytes); err != nil {
			return nil, err
		}
	}

	expiresAt := request.CreatedAt.Add(gptImage2ResultRetention).Unix()
	results := make([]GPTImage2PreviewObject, 0, len(request.Images))
	for index, encoded := range request.Images {
		objectKey, err := BuildGPTImage2ResultObjectKey(request.UserID, request.RequestID, index, request.CreatedAt, extension)
		if err != nil {
			return nil, err
		}
		if err := deps.registerCleanup(objectKey, expiresAt); err != nil {
			return nil, err
		}
		decoder := base64.NewDecoder(base64.StdEncoding.Strict(), strings.NewReader(strings.TrimSpace(encoded)))
		stored, _, err := deps.put(ctx, decoder, ObjectKeySpec{
			ObjectKey: objectKey,
			MediaType: "image",
			MIMEType:  mimeType,
			MaxBytes:  maxBytes,
			ExpiresAt: expiresAt,
		})
		if err != nil {
			return nil, err
		}
		if stored == nil || stored.ObjectKey == "" || stored.ExpiresAt != expiresAt {
			return nil, errors.New("persisted GPT Image 2 metadata is invalid")
		}
		results = append(results, GPTImage2PreviewObject{
			ObjectKey: stored.ObjectKey,
			MIMEType:  stored.MIMEType,
			ExpiresAt: stored.ExpiresAt,
		})
	}
	return results, nil
}

func validateGPTImage2Base64(encoded string, maxBytes int64) error {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return errors.New("GPT Image 2 base64 result is empty")
	}
	if int64(len(encoded)) > int64(base64.StdEncoding.EncodedLen(int(maxBytes))) {
		return errors.New("GPT Image 2 base64 result exceeds maximum size")
	}
	decoded, err := io.Copy(io.Discard, base64.NewDecoder(base64.StdEncoding.Strict(), strings.NewReader(encoded)))
	if err != nil {
		return errors.New("GPT Image 2 result contains invalid base64")
	}
	if decoded <= 0 || decoded > maxBytes {
		return errors.New("GPT Image 2 base64 result exceeds maximum size")
	}
	return nil
}

func gptImage2OutputType(outputFormat string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(outputFormat)) {
	case "", "png":
		return "image/png", ".png", nil
	case "jpeg", "jpg":
		return "image/jpeg", ".jpg", nil
	case "webp":
		return "image/webp", ".webp", nil
	default:
		return "", "", errors.New("GPT Image 2 output format is unsupported")
	}
}

func BuildGPTImage2ResultObjectKey(userID int, requestID string, index int, createdAt time.Time, extension string) (string, error) {
	if userID <= 0 || strings.TrimSpace(requestID) == "" || index < 0 || createdAt.IsZero() {
		return "", errors.New("GPT Image 2 object key metadata is invalid")
	}
	if extension != ".png" && extension != ".jpg" && extension != ".webp" {
		return "", errors.New("GPT Image 2 object extension is invalid")
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:image:%d", strings.TrimSpace(requestID), index)))
	createdAt = createdAt.UTC()
	return path.Join(
		operation_setting.GetCOSConfig().PathPrefix,
		"gpt-image-2-results",
		strconv.Itoa(userID),
		createdAt.Format("2006"),
		createdAt.Format("01"),
		hex.EncodeToString(digest[:16])+extension,
	), nil
}

func IsOwnedGPTImage2ResultObject(userID int, objectKey string) bool {
	objectKey = strings.TrimSpace(objectKey)
	if userID <= 0 || objectKey == "" || path.Clean(objectKey) != objectKey {
		return false
	}
	prefix := path.Join(operation_setting.GetCOSConfig().PathPrefix, "gpt-image-2-results", strconv.Itoa(userID)) + "/"
	return strings.HasPrefix(objectKey, prefix)
}
