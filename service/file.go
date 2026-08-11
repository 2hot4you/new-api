package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	cos "github.com/tencentyun/cos-go-sdk-v5"
	_ "golang.org/x/image/webp"
)

const (
	moliiFileRetention     = 24 * time.Hour
	moliiFileImageMaxBytes = int64(30 * 1024 * 1024)
	moliiFileVideoMaxBytes = int64(mediaProbeMaxBytes)
	moliiFileSignMaxTTL    = time.Hour
)

var (
	ErrMoliiFileInvalidPurpose     = errors.New("file purpose is invalid")
	ErrMoliiFileUnsupportedMedia   = errors.New("file media type is not supported")
	ErrMoliiFileEmpty              = errors.New("file is empty")
	ErrMoliiFileTooLarge           = errors.New("file exceeds the maximum size")
	ErrMoliiFileTypeMismatch       = errors.New("file media type does not match the requested input")
	ErrMoliiFileServiceUnavailable = errors.New("file service is unavailable")
	moliiFilePurposePattern        = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	moliiFileIDPattern             = regexp.MustCompile(`^file_[A-Za-z0-9]{16,64}$`)
)

type moliiFileUploadMetadata struct {
	MIMEType        string
	MediaType       model.MoliiFileMediaType
	Extension       string
	Width           int
	Height          int
	DurationSeconds float64
}

type userFileDependencies struct {
	upload          func(context.Context, ObjectKeySpec, []byte) error
	registerCleanup func(string, int64) error
	createMetadata  func(context.Context, *model.MoliiFile) error
	deleteObject    func(context.Context, string) error
}

func defaultUserFileDependencies() userFileDependencies {
	return userFileDependencies{
		upload:          uploadMoliiFileObject,
		registerCleanup: RegisterPendingGrokObjectCleanup,
		createMetadata:  model.CreateMoliiFile,
		deleteObject:    DeleteGrokResultObject,
	}
}

func validateMoliiFileUpload(filename, purpose string, data []byte) (moliiFileUploadMetadata, error) {
	if !moliiFilePurposePattern.MatchString(strings.TrimSpace(purpose)) {
		return moliiFileUploadMetadata{}, ErrMoliiFileInvalidPurpose
	}
	if len(data) == 0 {
		return moliiFileUploadMetadata{}, ErrMoliiFileEmpty
	}
	detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	if parsed, _, err := mime.ParseMediaType(detected); err == nil {
		detected = strings.ToLower(parsed)
	}
	if detected == "application/octet-stream" && len(data) >= 12 && string(data[4:8]) == "ftyp" {
		detected = "video/mp4"
	}
	var metadata moliiFileUploadMetadata
	switch detected {
	case "image/png":
		metadata = moliiFileUploadMetadata{MIMEType: detected, MediaType: model.MoliiFileMediaTypeImage, Extension: ".png"}
	case "image/jpeg":
		metadata = moliiFileUploadMetadata{MIMEType: detected, MediaType: model.MoliiFileMediaTypeImage, Extension: ".jpg"}
	case "image/webp":
		metadata = moliiFileUploadMetadata{MIMEType: detected, MediaType: model.MoliiFileMediaTypeImage, Extension: ".webp"}
	case "video/mp4":
		metadata = moliiFileUploadMetadata{MIMEType: detected, MediaType: model.MoliiFileMediaTypeVideo, Extension: ".mp4"}
		probe, err := ProbeUserVideo(context.Background(), MediaSource{Data: data, MIMEType: detected})
		if err != nil || probe == nil {
			return moliiFileUploadMetadata{}, ErrMoliiFileUnsupportedMedia
		}
		metadata.Width, metadata.Height, metadata.DurationSeconds = probe.Width, probe.Height, probe.DurationSeconds
	default:
		return moliiFileUploadMetadata{}, ErrMoliiFileUnsupportedMedia
	}
	if metadata.MediaType == model.MoliiFileMediaTypeImage && int64(len(data)) > moliiFileImageMaxBytes {
		return moliiFileUploadMetadata{}, ErrMoliiFileTooLarge
	}
	if metadata.MediaType == model.MoliiFileMediaTypeVideo && int64(len(data)) > moliiFileVideoMaxBytes {
		return moliiFileUploadMetadata{}, ErrMoliiFileTooLarge
	}
	if metadata.MediaType == model.MoliiFileMediaTypeImage {
		config, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width <= 0 || config.Height <= 0 {
			return moliiFileUploadMetadata{}, ErrMoliiFileUnsupportedMedia
		}
		metadata.Width, metadata.Height = config.Width, config.Height
	}
	if strings.TrimSpace(path.Base(filename)) == "." {
		return moliiFileUploadMetadata{}, errors.New("file name is invalid")
	}
	return metadata, nil
}

func BuildMoliiFileObjectKey(userID int, fileID string, createdAt time.Time, extension string) (string, error) {
	fileID = strings.TrimSpace(fileID)
	if userID <= 0 || !moliiFileIDPattern.MatchString(fileID) || createdAt.IsZero() {
		return "", errors.New("Molii file object metadata is invalid")
	}
	switch extension {
	case ".png", ".jpg", ".webp", ".mp4":
	default:
		return "", errors.New("Molii file extension is invalid")
	}
	createdAt = createdAt.UTC()
	return path.Join(
		operation_setting.GetCOSConfig().PathPrefix,
		"grok-files",
		strconv.Itoa(userID),
		createdAt.Format("2006"),
		createdAt.Format("01"),
		fileID+extension,
	), nil
}

func CreateUserFile(ctx context.Context, userID int, filename, purpose string, content io.Reader) (*model.MoliiFile, error) {
	return createUserFileWithDependencies(ctx, userID, filename, purpose, content, time.Now().UTC(), defaultUserFileDependencies())
}

func createUserFileWithDependencies(ctx context.Context, userID int, filename, purpose string, content io.Reader, createdAt time.Time, dependencies userFileDependencies) (*model.MoliiFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if dependencies.createMetadata == nil {
		dependencies.createMetadata = model.CreateMoliiFile
	}
	if dependencies.deleteObject == nil {
		dependencies.deleteObject = DeleteGrokResultObject
	}
	if userID <= 0 || content == nil || dependencies.upload == nil || dependencies.registerCleanup == nil || dependencies.createMetadata == nil || dependencies.deleteObject == nil {
		return nil, errors.New("valid file upload context is required")
	}
	data, err := io.ReadAll(io.LimitReader(&objectStorageContextReader{ctx: ctx, reader: content}, moliiFileVideoMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read file upload: %w", err)
	}
	if int64(len(data)) > moliiFileVideoMaxBytes {
		return nil, ErrMoliiFileTooLarge
	}
	metadata, err := validateMoliiFileUpload(filename, purpose, data)
	if err != nil {
		return nil, err
	}
	fileID := "file_" + common.GetUUID()
	objectKey, err := BuildMoliiFileObjectKey(userID, fileID, createdAt, metadata.Extension)
	if err != nil {
		return nil, err
	}
	expiresAt := createdAt.Add(moliiFileRetention).Unix()
	if err := dependencies.registerCleanup(objectKey, expiresAt); err != nil {
		return nil, fmt.Errorf("register file cleanup: %w", err)
	}
	spec := ObjectKeySpec{ObjectKey: objectKey, MediaType: string(metadata.MediaType), MIMEType: metadata.MIMEType, MaxBytes: int64(len(data)), ExpiresAt: expiresAt}
	if err := dependencies.upload(ctx, spec, data); err != nil {
		return nil, err
	}
	baseName := path.Base(strings.TrimSpace(filename))
	if len(baseName) > 255 {
		baseName = baseName[:255]
	}
	file := &model.MoliiFile{
		FileID: fileID, UserID: userID, ObjectKey: objectKey, Filename: baseName,
		Purpose: strings.TrimSpace(purpose), Bytes: int64(len(data)), MIMEType: metadata.MIMEType,
		MediaType: metadata.MediaType, Width: metadata.Width, Height: metadata.Height, DurationSeconds: metadata.DurationSeconds,
		Status:    model.MoliiFileStatusActive,
		CreatedAt: createdAt.Unix(), UpdatedAt: createdAt.Unix(), ExpiresAt: expiresAt,
	}
	if err := dependencies.createMetadata(ctx, file); err != nil {
		_ = dependencies.deleteObject(ctx, objectKey)
		return nil, fmt.Errorf("persist file metadata: %w", err)
	}
	return file, nil
}

func uploadMoliiFileObject(ctx context.Context, spec ObjectKeySpec, data []byte) error {
	store, err := newObjectStorageCOSForPersistence(operation_setting.GetCOSConfig())
	if err != nil {
		return err
	}
	if err := validateObjectKeySpec(spec); err != nil {
		return err
	}
	metadata := http.Header{}
	metadata.Set("x-cos-meta-expires-at", strconv.FormatInt(spec.ExpiresAt, 10))
	conditional := http.Header{}
	conditional.Set("x-cos-forbid-overwrite", "true")
	response, err := store.client.Object.Put(ctx, spec.ObjectKey, bytes.NewReader(data), &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: spec.MIMEType, ContentLength: int64(len(data)), XCosMetaXXX: &metadata, XOptionHeader: &conditional,
		},
	})
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("upload file to COS: %w", err)
	}
	return nil
}

func ResolveUserFile(ctx context.Context, userID int, fileID string, expectedMediaType model.MoliiFileMediaType) (*model.MoliiFile, string, error) {
	return resolveUserFileWithSigner(ctx, userID, fileID, expectedMediaType, time.Now().Unix(), SignCOSObjectURL)
}

func resolveUserFileWithSigner(ctx context.Context, userID int, fileID string, expectedMediaType model.MoliiFileMediaType, now int64, signer func(context.Context, string, time.Duration) (string, error)) (*model.MoliiFile, string, error) {
	file, err := model.GetMoliiFileForUser(ctx, userID, fileID, now)
	if err != nil {
		if !errors.Is(err, model.ErrMoliiFileNotFound) && !errors.Is(err, model.ErrMoliiFileExpired) {
			return nil, "", fmt.Errorf("%w: file metadata lookup failed", ErrMoliiFileServiceUnavailable)
		}
		return nil, "", err
	}
	if expectedMediaType != "" && file.MediaType != expectedMediaType {
		return nil, "", ErrMoliiFileTypeMismatch
	}
	if signer == nil {
		return nil, "", ErrObjectStorageUnavailable
	}
	ttl := time.Duration(file.ExpiresAt-now) * time.Second
	if ttl > moliiFileSignMaxTTL {
		ttl = moliiFileSignMaxTTL
	}
	if ttl <= 0 {
		return nil, "", model.ErrMoliiFileExpired
	}
	signedURL, err := signer(ctx, file.ObjectKey, ttl)
	if err != nil {
		return nil, "", fmt.Errorf("%w: file URL signing failed", ErrMoliiFileServiceUnavailable)
	}
	return file, signedURL, nil
}

func ListUserFiles(ctx context.Context, userID int) ([]*model.MoliiFile, error) {
	return model.ListActiveMoliiFiles(ctx, userID, time.Now().Unix())
}

func DeleteUserFile(ctx context.Context, userID int, fileID string) (*model.MoliiFile, error) {
	file, err := model.GetMoliiFileRecordForUser(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	if file.Status == model.MoliiFileStatusDeleted {
		return file, nil
	}
	if file.Status != model.MoliiFileStatusExpired {
		if err := DeleteCOSObject(ctx, file.ObjectKey); err != nil {
			return nil, err
		}
	}
	deleted, _, err := model.MarkMoliiFileDeleted(ctx, userID, fileID, time.Now().Unix())
	return deleted, err
}
