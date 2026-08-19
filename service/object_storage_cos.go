package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

const (
	objectStorageDefaultImageMaxBytes = int64(30 * 1024 * 1024)
	objectStorageDefaultVideoMaxBytes = int64(200 * 1024 * 1024)
	objectStorageMaximumSignTTL       = 24 * time.Hour
)

var ErrObjectStorageUnavailable = errors.New("COS object storage is unavailable")

// StoredObject contains only Molii-owned object metadata. In particular, it
// never contains the temporary upstream URL used to populate the object.
type StoredObject struct {
	ObjectKey string `json:"object_key"`
	MIMEType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	ExpiresAt int64  `json:"expires_at"`
}

// ObjectKeySpec describes the destination and the limits applied while
// copying a remote object. MIMEType is optional; when set, the upstream
// response must match it exactly after MIME normalization.
type ObjectKeySpec struct {
	ObjectKey string
	MediaType string
	MIMEType  string
	MaxBytes  int64
	ExpiresAt int64
}

type objectStorageCOS struct {
	client      *cos.Client
	fetchClient *http.Client
	config      operation_setting.COSConfig
}

type objectStorageRedirectError struct {
	cause error
}

func (err *objectStorageRedirectError) Error() string {
	if err == nil || err.cause == nil {
		return "remote object redirect rejected"
	}
	return err.cause.Error()
}

func (err *objectStorageRedirectError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

func newObjectStorageFetchClient() *http.Client {
	client := newProtectedFetchHTTPClientWithoutProxy(nil, nil, nil)
	protectedRedirect := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if request == nil || request.URL == nil || request.URL.Scheme != "https" {
			return &objectStorageRedirectError{cause: errors.New("remote object redirects must use HTTPS")}
		}
		if err := protectedRedirect(request, via); err != nil {
			return &objectStorageRedirectError{cause: err}
		}
		return nil
	}
	return client
}

func buildObjectStorageCOSClient(config operation_setting.COSConfig) (*cos.Client, error) {
	if err := config.ValidateCredentials(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrObjectStorageUnavailable, err)
	}
	endpoint := config.CustomDomain
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region)
	}
	bucketURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid COS endpoint", ErrObjectStorageUnavailable)
	}
	return cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.SecretID,
			SecretKey: config.SecretKey,
		},
	}), nil
}

func newObjectStorageCOSClient(config operation_setting.COSConfig) (*cos.Client, error) {
	return buildObjectStorageCOSClient(config)
}

func newObjectStorageCOS(config operation_setting.COSConfig) (*objectStorageCOS, error) {
	client, err := newObjectStorageCOSClient(config)
	if err != nil {
		return nil, err
	}
	return &objectStorageCOS{client: client, fetchClient: newObjectStorageFetchClient(), config: config}, nil
}

func newObjectStorageCOSForPersistence(config operation_setting.COSConfig) (*objectStorageCOS, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrObjectStorageUnavailable, err)
	}
	return newObjectStorageCOS(config)
}

// CopyRemoteObjectToCOS fetches one HTTPS object through the protected direct
// fetch path, validates and bounds it on disk, then uploads it to COS. A HEAD
// hit reuses an object previously written under the deterministic key.
func CopyRemoteObjectToCOS(ctx context.Context, sourceURL string, key ObjectKeySpec) (*StoredObject, error) {
	store, err := newObjectStorageCOSForPersistence(operation_setting.GetCOSConfig())
	if err != nil {
		return nil, err
	}
	return store.copyRemoteObjectToCOS(ctx, sourceURL, key)
}

func (store *objectStorageCOS) copyRemoteObjectToCOS(ctx context.Context, sourceURL string, key ObjectKeySpec) (*StoredObject, error) {
	stored, _, err := store.copyRemoteObjectToCOSWithStatus(ctx, sourceURL, key)
	return stored, err
}

func (store *objectStorageCOS) copyRemoteObjectToCOSWithStatus(ctx context.Context, sourceURL string, key ObjectKeySpec) (*StoredObject, bool, error) {
	if err := validateObjectKeySpec(key); err != nil {
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageBuildObjectKey, "invalid_object_spec", sourceURL, err)
	}
	parsed, err := url.Parse(strings.TrimSpace(sourceURL))
	if err != nil || parsed == nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		cause := errors.New("remote object URL must be an HTTPS URL without credentials")
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageValidateSourceURL, "invalid_source_url", sourceURL, cause)
	}

	if store == nil || store.client == nil {
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageCOSHead, "storage_unavailable", sourceURL, ErrObjectStorageUnavailable)
	}
	if existing, found, err := headStoredCOSObject(ctx, store.client, key); err != nil {
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageCOSHead, "head_failed", sourceURL, err)
	} else if found {
		return existing, false, nil
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		cause := errors.New("remote object URL is invalid")
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageValidateSourceURL, "invalid_source_url", sourceURL, cause)
	}
	fetchClient := store.fetchClient
	if fetchClient == nil {
		cause := errors.New("protected remote object client is unavailable")
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteFetch, "client_unavailable", sourceURL, cause)
	}
	response, err := fetchClient.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteFetch, "request_cancelled", sourceURL, ctxErr)
		}
		var redirectErr *objectStorageRedirectError
		if errors.As(err, &redirectErr) {
			if strings.EqualFold(strings.TrimSpace(key.MediaType), "image") {
				return nil, false, newGrokImagePersistenceError(grokImageStageRemoteRedirect, "redirect_rejected", sourceURL, err)
			}
			return nil, false, errors.New("remote object fetch failed")
		}
		cause := errors.New("remote object fetch failed")
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteFetch, "request_failed", sourceURL, cause)
	}
	if response == nil || response.Body == nil {
		cause := errors.New("remote object fetch returned no body")
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteFetch, "missing_response_body", sourceURL, cause)
	}
	defer response.Body.Close()
	if response.Request != nil && response.Request.URL != nil && response.Request.URL.Scheme != "https" {
		cause := errors.New("remote object redirects must use HTTPS")
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteRedirect, "insecure_redirect", sourceURL, cause)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		cause := fmt.Errorf("remote object fetch failed with status %d", response.StatusCode)
		return nil, false, grokImagePersistenceErrorForMediaWithRemoteStatus(key.MediaType, grokImageStageRemoteFetch, "non_success_status", sourceURL, response.StatusCode, cause)
	}
	mimeType, err := validateRemoteObjectMIME(response.Header.Get("Content-Type"), key)
	if err != nil {
		category := "invalid_content_type"
		if strings.Contains(err.Error(), "does not match") {
			category = "content_type_mismatch"
		}
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteContentType, category, sourceURL, err)
	}
	maxBytes := objectStorageMaxBytes(key)
	if response.ContentLength > maxBytes {
		cause := fmt.Errorf("remote object exceeds maximum size of %d bytes", maxBytes)
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteSize, "size_exceeded", sourceURL, cause)
	}

	temporary, err := os.CreateTemp("", "molii-cos-copy-*")
	if err != nil {
		cause := fmt.Errorf("prepare bounded remote object copy: %w", err)
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteFetch, "buffer_failed", sourceURL, cause)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()

	size, err := io.Copy(temporary, io.LimitReader(&objectStorageContextReader{ctx: ctx, reader: response.Body}, maxBytes+1))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteFetch, "request_cancelled", sourceURL, ctxErr)
		}
		cause := fmt.Errorf("copy remote object: %w", err)
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteFetch, "body_read_failed", sourceURL, cause)
	}
	if size <= 0 {
		cause := errors.New("remote object is empty")
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteSize, "empty_body", sourceURL, cause)
	}
	if size > maxBytes {
		cause := fmt.Errorf("remote object exceeds maximum size of %d bytes", maxBytes)
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageRemoteSize, "size_exceeded", sourceURL, cause)
	}
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		cause := fmt.Errorf("prepare COS upload: %w", err)
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageCOSPut, "buffer_seek_failed", sourceURL, cause)
	}
	metadata := http.Header{}
	metadata.Set("x-cos-meta-expires-at", strconv.FormatInt(key.ExpiresAt, 10))
	conditionalHeaders := http.Header{}
	// COS atomically rejects a concurrent writer for the same key on
	// unversioned buckets. This makes the returned created bit an ownership
	// guarantee instead of a racy HEAD-then-PUT observation.
	conditionalHeaders.Set("x-cos-forbid-overwrite", "true")
	putResponse, err := store.client.Object.Put(ctx, key.ObjectKey, temporary, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType:   mimeType,
			ContentLength: size,
			XCosMetaXXX:   &metadata,
			XOptionHeader: &conditionalHeaders,
		},
	})
	if putResponse != nil && putResponse.Body != nil {
		defer putResponse.Body.Close()
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageCOSPut, "request_cancelled", sourceURL, ctxErr)
		}
		if isCOSObjectAlreadyExists(err) {
			existing, found, headErr := headStoredCOSObject(ctx, store.client, key)
			if headErr != nil {
				return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageCOSHead, "head_after_conflict_failed", sourceURL, headErr)
			}
			if found {
				return existing, false, nil
			}
		}
		cause := fmt.Errorf("upload remote object to COS: %w", err)
		return nil, false, grokImagePersistenceErrorForMedia(key.MediaType, grokImageStageCOSPut, "put_failed", sourceURL, cause)
	}
	return &StoredObject{ObjectKey: key.ObjectKey, MIMEType: mimeType, Size: size, ExpiresAt: key.ExpiresAt}, true, nil
}

func isCOSObjectAlreadyExists(err error) bool {
	response, ok := cos.IsCOSError(err)
	if !ok || response == nil || response.Response == nil || response.Response.StatusCode != http.StatusConflict {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(response.Code), "FileAlreadyExists")
}

func validateObjectKeySpec(key ObjectKeySpec) error {
	trimmedKey := strings.TrimSpace(key.ObjectKey)
	if trimmedKey == "" || strings.HasPrefix(trimmedKey, "/") || path.Clean(trimmedKey) != trimmedKey || strings.HasPrefix(trimmedKey, "..") {
		return errors.New("COS object key is invalid")
	}
	mediaType := strings.ToLower(strings.TrimSpace(key.MediaType))
	if mediaType != "image" && mediaType != "video" {
		return errors.New("COS object media type must be image or video")
	}
	if key.ExpiresAt <= 0 {
		return errors.New("COS object expiry is required")
	}
	if key.MaxBytes < 0 {
		return errors.New("COS object maximum size is invalid")
	}
	return nil
}

func objectStorageMaxBytes(key ObjectKeySpec) int64 {
	if key.MaxBytes > 0 {
		return key.MaxBytes
	}
	if strings.EqualFold(strings.TrimSpace(key.MediaType), "image") {
		return objectStorageDefaultImageMaxBytes
	}
	return objectStorageDefaultVideoMaxBytes
}

func validateRemoteObjectMIME(raw string, key ObjectKeySpec) (string, error) {
	mimeType, _, err := mime.ParseMediaType(strings.TrimSpace(raw))
	if err != nil || mimeType == "" {
		return "", errors.New("remote object content type is invalid")
	}
	mimeType = strings.ToLower(mimeType)
	mediaType := strings.ToLower(strings.TrimSpace(key.MediaType))
	if !strings.HasPrefix(mimeType, mediaType+"/") {
		return "", fmt.Errorf("remote object content type %q does not match %s", mimeType, mediaType)
	}
	if expected := strings.TrimSpace(key.MIMEType); expected != "" {
		normalizedExpected, _, expectedErr := mime.ParseMediaType(expected)
		if expectedErr != nil || !strings.EqualFold(mimeType, normalizedExpected) {
			return "", fmt.Errorf("remote object content type %q does not match %q", mimeType, expected)
		}
	}
	return mimeType, nil
}

func headStoredCOSObject(ctx context.Context, client *cos.Client, key ObjectKeySpec) (*StoredObject, bool, error) {
	response, err := client.Object.Head(ctx, key.ObjectKey, nil)
	if err != nil {
		if cos.IsNotFoundError(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("check COS object: %w", err)
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if response == nil {
		return nil, false, errors.New("COS object HEAD returned no response")
	}
	mimeType := ""
	mimeType, err = validateRemoteObjectMIME(response.Header.Get("Content-Type"), key)
	if err != nil {
		return nil, false, fmt.Errorf("existing COS object metadata is invalid: %w", err)
	}
	persistedExpiry := strings.TrimSpace(response.Header.Get("x-cos-meta-expires-at"))
	if persistedExpiry == "" {
		return nil, false, errors.New("existing COS object expiry metadata is missing")
	}
	expiresAt, parseErr := strconv.ParseInt(persistedExpiry, 10, 64)
	if parseErr != nil || expiresAt <= 0 {
		return nil, false, errors.New("existing COS object expiry metadata is invalid")
	}
	return &StoredObject{
		ObjectKey: key.ObjectKey,
		MIMEType:  mimeType,
		Size:      response.ContentLength,
		ExpiresAt: expiresAt,
	}, true, nil
}

func SignCOSObjectURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	config := operation_setting.GetCOSConfig()
	store, err := newObjectStorageCOS(config)
	if err != nil {
		return "", err
	}
	return store.signObjectURL(ctx, objectKey, ttl)
}

// FetchCOSObject opens a private COS object with Molii's server-side
// credentials. Callers may relay byte ranges without exposing COS credentials
// or a provider-signed URL to the client.
func FetchCOSObject(ctx context.Context, objectKey, rangeHeader, ifRangeHeader string) (*http.Response, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" || strings.HasPrefix(objectKey, "/") || path.Clean(objectKey) != objectKey || strings.HasPrefix(objectKey, "..") {
		return nil, errors.New("COS object key is invalid")
	}
	store, err := newObjectStorageCOS(operation_setting.GetCOSConfig())
	if err != nil {
		return nil, err
	}
	return fetchCOSObjectWithClient(ctx, store.client, objectKey, rangeHeader, ifRangeHeader)
}

func fetchCOSObjectWithClient(ctx context.Context, client *cos.Client, objectKey, rangeHeader, ifRangeHeader string) (*http.Response, error) {
	if client == nil {
		return nil, ErrObjectStorageUnavailable
	}
	options := &cos.ObjectGetOptions{Range: strings.TrimSpace(rangeHeader)}
	if ifRange := strings.TrimSpace(ifRangeHeader); ifRange != "" {
		headers := make(http.Header)
		headers.Set("If-Range", ifRange)
		options.XOptionHeader = &headers
	}
	response, err := client.Object.Get(ctx, objectKey, options)
	if err != nil {
		if response != nil && response.Response != nil && response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			if response.Body == nil {
				response.Body = http.NoBody
			}
			return response.Response, nil
		}
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, fmt.Errorf("fetch COS object: %w", err)
	}
	if response == nil || response.Response == nil || response.Body == nil {
		return nil, errors.New("fetch COS object returned no response")
	}
	return response.Response, nil
}

func (store *objectStorageCOS) signObjectURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	if store == nil || store.client == nil {
		return "", ErrObjectStorageUnavailable
	}
	return signCOSObjectURLWithClient(ctx, store.client, store.config, objectKey, ttl)
}

func signCOSObjectURLWithClient(ctx context.Context, client *cos.Client, config operation_setting.COSConfig, objectKey string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(objectKey) == "" {
		return "", errors.New("COS object key is required")
	}
	if ttl <= 0 || ttl > objectStorageMaximumSignTTL {
		return "", fmt.Errorf("COS object signing TTL must be greater than zero and at most %s", objectStorageMaximumSignTTL)
	}
	readURL, err := client.Object.GetPresignedURL(ctx, http.MethodGet, objectKey, config.SecretID, config.SecretKey, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("%w: failed to sign object read", ErrObjectStorageUnavailable)
	}
	return readURL.String(), nil
}

func DeleteCOSObject(ctx context.Context, objectKey string) error {
	if strings.TrimSpace(objectKey) == "" {
		return nil
	}
	store, err := newObjectStorageCOS(operation_setting.GetCOSConfig())
	if err != nil {
		return err
	}
	return store.deleteObject(ctx, objectKey)
}

func (store *objectStorageCOS) deleteObject(ctx context.Context, objectKey string) error {
	if strings.TrimSpace(objectKey) == "" {
		return nil
	}
	if store == nil || store.client == nil {
		return ErrObjectStorageUnavailable
	}
	response, err := store.client.Object.Delete(ctx, objectKey)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if cos.IsNotFoundError(err) {
		return nil
	}
	return err
}

type objectStorageContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *objectStorageContextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	read, err := reader.reader.Read(buffer)
	if contextErr := reader.ctx.Err(); contextErr != nil {
		return read, contextErr
	}
	return read, err
}
