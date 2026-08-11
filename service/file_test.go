package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func validPNGFixture(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	require.NoError(t, png.Encode(&buffer, img))
	return buffer.Bytes()
}

func setupUserFileServiceDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "files-service.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MoliiFile{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
}

func TestValidateMoliiFileUploadAcceptsSupportedImageAndRejectsInvalidInputs(t *testing.T) {
	pngData := validPNGFixture(t, 320, 180)
	metadata, err := validateMoliiFileUpload("photo.png", "vision", pngData)
	require.NoError(t, err)
	assert.Equal(t, "image/png", metadata.MIMEType)
	assert.Equal(t, model.MoliiFileMediaTypeImage, metadata.MediaType)
	assert.Equal(t, ".png", metadata.Extension)
	assert.Equal(t, 320, metadata.Width)
	assert.Equal(t, 180, metadata.Height)

	_, err = validateMoliiFileUpload("photo.png", "purpose with spaces", pngData)
	assert.ErrorIs(t, err, ErrMoliiFileInvalidPurpose)
	_, err = validateMoliiFileUpload("payload.txt", "vision", []byte("not media"))
	assert.ErrorIs(t, err, ErrMoliiFileUnsupportedMedia)
	_, err = validateMoliiFileUpload("empty.png", "vision", nil)
	assert.ErrorIs(t, err, ErrMoliiFileEmpty)
}

func TestValidateMoliiFileUploadPersistsAuthoritativeVideoProbeMetadata(t *testing.T) {
	video := fixtureMP4(t, 99_000, videoFixtureTrack("avc1", 1280, 720, 8_700))
	metadata, err := validateMoliiFileUpload("clip.mp4", "video", video)
	require.NoError(t, err)
	assert.Equal(t, 1280, metadata.Width)
	assert.Equal(t, 720, metadata.Height)
	assert.InDelta(t, 8.7, metadata.DurationSeconds, 0.0001)
}

func TestBuildMoliiFileObjectKeyUsesDedicatedUserPrefix(t *testing.T) {
	createdAt := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	key, err := BuildMoliiFileObjectKey(2205, "file_abcdefghijklmnop", createdAt, ".webp")
	require.NoError(t, err)
	assert.Contains(t, key, "grok-files/2205/2026/08/")
	assert.True(t, len(key) > len("grok-files/2205/2026/08/.webp"))
}

func TestResolveUserFileEnforcesOwnershipExpiryAndMediaType(t *testing.T) {
	setupUserFileServiceDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.CreateMoliiFile(context.Background(), &model.MoliiFile{
		FileID: "file_image", UserID: 10, ObjectKey: "prefix/grok-files/10/2026/08/file_image.png",
		Filename: "image.png", Purpose: "vision", Bytes: 20, MIMEType: "image/png",
		MediaType: model.MoliiFileMediaTypeImage, Status: model.MoliiFileStatusActive,
		CreatedAt: now, UpdatedAt: now, ExpiresAt: now + 3600,
	}))

	file, signedURL, err := resolveUserFileWithSigner(context.Background(), 10, "file_image", model.MoliiFileMediaTypeImage, now, func(_ context.Context, key string, ttl time.Duration) (string, error) {
		assert.Equal(t, "prefix/grok-files/10/2026/08/file_image.png", key)
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, time.Hour)
		return "https://cos.example/signed-image", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "file_image", file.FileID)
	assert.Equal(t, "https://cos.example/signed-image", signedURL)

	_, _, err = resolveUserFileWithSigner(context.Background(), 11, "file_image", model.MoliiFileMediaTypeImage, now, nil)
	assert.ErrorIs(t, err, model.ErrMoliiFileNotFound)
	_, _, err = resolveUserFileWithSigner(context.Background(), 10, "file_image", model.MoliiFileMediaTypeVideo, now, nil)
	assert.ErrorIs(t, err, ErrMoliiFileTypeMismatch)
	_, _, err = resolveUserFileWithSigner(context.Background(), 10, "file_image", model.MoliiFileMediaTypeImage, now+3600, nil)
	assert.ErrorIs(t, err, model.ErrMoliiFileExpired)
}

func TestCreateUserFilePersistsExact24HourMetadataAfterUpload(t *testing.T) {
	setupUserFileServiceDB(t)
	createdAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	png := validPNGFixture(t, 640, 360)
	var uploaded ObjectKeySpec
	file, err := createUserFileWithDependencies(context.Background(), 9, "photo.png", "vision", bytes.NewReader(png), createdAt, userFileDependencies{
		upload: func(_ context.Context, spec ObjectKeySpec, data []byte) error {
			uploaded = spec
			assert.Equal(t, png, data)
			return nil
		},
		registerCleanup: func(key string, expiresAt int64) error {
			assert.NotEmpty(t, key)
			assert.Equal(t, createdAt.Add(24*time.Hour).Unix(), expiresAt)
			return nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, createdAt.Add(24*time.Hour).Unix(), file.ExpiresAt)
	assert.Equal(t, file.ExpiresAt, uploaded.ExpiresAt)
	assert.Equal(t, "image", uploaded.MediaType)
	assert.Equal(t, 640, file.Width)
	assert.Equal(t, 360, file.Height)

	stored, err := model.GetMoliiFileForUser(context.Background(), 9, file.FileID, createdAt.Unix())
	require.NoError(t, err)
	assert.Equal(t, file.ObjectKey, stored.ObjectKey)
}

func TestCreateUserFileDoesNotPersistMetadataWhenUploadFails(t *testing.T) {
	setupUserFileServiceDB(t)
	png := validPNGFixture(t, 2, 2)
	_, err := createUserFileWithDependencies(context.Background(), 9, "photo.png", "vision", bytes.NewReader(png), time.Now(), userFileDependencies{
		upload:          func(context.Context, ObjectKeySpec, []byte) error { return errors.New("upload failed") },
		registerCleanup: func(string, int64) error { return nil },
	})
	require.Error(t, err)
	var count int64
	require.NoError(t, model.DB.Model(&model.MoliiFile{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestCreateUserFileCleansUpUploadedObjectWhenMetadataInsertFails(t *testing.T) {
	setupUserFileServiceDB(t)
	pngData := validPNGFixture(t, 2, 2)
	var deletedKey string
	_, err := createUserFileWithDependencies(context.Background(), 9, "photo.png", "vision", bytes.NewReader(pngData), time.Now(), userFileDependencies{
		upload:          func(context.Context, ObjectKeySpec, []byte) error { return nil },
		registerCleanup: func(string, int64) error { return nil },
		createMetadata:  func(context.Context, *model.MoliiFile) error { return errors.New("database unavailable") },
		deleteObject: func(_ context.Context, key string) error {
			deletedKey = key
			return nil
		},
	})
	require.Error(t, err)
	assert.Contains(t, deletedKey, "/grok-files/9/")
}
