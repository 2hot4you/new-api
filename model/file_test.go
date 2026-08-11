package model

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMoliiFileTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "molii-files.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MoliiFile{}))
	DB = db
	t.Cleanup(func() { DB = previousDB })
}

func TestMoliiFileLifecycleIsUserScopedAndExpires(t *testing.T) {
	setupMoliiFileTestDB(t)
	now := time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC).Unix()
	file := &MoliiFile{
		FileID:    "file_owned",
		UserID:    101,
		ObjectKey: "molii/grok-files/101/2026/08/file_owned.png",
		Filename:  "owned.png",
		Purpose:   "vision",
		Bytes:     123,
		MIMEType:  "image/png",
		MediaType: MoliiFileMediaTypeImage,
		Status:    MoliiFileStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: now + 24*60*60,
	}
	require.NoError(t, CreateMoliiFile(context.Background(), file))

	owned, err := GetMoliiFileForUser(context.Background(), 101, file.FileID, now)
	require.NoError(t, err)
	assert.Equal(t, file.ObjectKey, owned.ObjectKey)

	_, err = GetMoliiFileForUser(context.Background(), 202, file.FileID, now)
	assert.ErrorIs(t, err, ErrMoliiFileNotFound)

	_, err = GetMoliiFileForUser(context.Background(), 101, file.FileID, file.ExpiresAt)
	assert.ErrorIs(t, err, ErrMoliiFileExpired)
}

func TestMoliiFileListAndDeleteAreIdempotent(t *testing.T) {
	setupMoliiFileTestDB(t)
	now := time.Now().Unix()
	active := &MoliiFile{FileID: "file_active", UserID: 7, ObjectKey: "p/grok-files/7/file_active.webp", Filename: "active.webp", Purpose: "vision", Bytes: 42, MIMEType: "image/webp", MediaType: MoliiFileMediaTypeImage, Status: MoliiFileStatusActive, CreatedAt: now, UpdatedAt: now, ExpiresAt: now + 3600}
	expired := &MoliiFile{FileID: "file_expired", UserID: 7, ObjectKey: "p/grok-files/7/file_expired.mp4", Filename: "expired.mp4", Purpose: "vision", Bytes: 100, MIMEType: "video/mp4", MediaType: MoliiFileMediaTypeVideo, Status: MoliiFileStatusActive, CreatedAt: now - 7200, UpdatedAt: now - 7200, ExpiresAt: now - 1}
	foreign := &MoliiFile{FileID: "file_foreign", UserID: 8, ObjectKey: "p/grok-files/8/file_foreign.png", Filename: "foreign.png", Purpose: "vision", Bytes: 2, MIMEType: "image/png", MediaType: MoliiFileMediaTypeImage, Status: MoliiFileStatusActive, CreatedAt: now, UpdatedAt: now, ExpiresAt: now + 3600}
	for _, file := range []*MoliiFile{active, expired, foreign} {
		require.NoError(t, CreateMoliiFile(context.Background(), file))
	}

	files, err := ListActiveMoliiFiles(context.Background(), 7, now)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, active.FileID, files[0].FileID)

	deleted, changed, err := MarkMoliiFileDeleted(context.Background(), 7, active.FileID, now)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, MoliiFileStatusDeleted, deleted.Status)
	assert.Empty(t, deleted.Filename)
	assert.Equal(t, int64(0), deleted.Bytes)
	assert.Contains(t, deleted.ObjectKey, "deleted/")

	deleted, changed, err = MarkMoliiFileDeleted(context.Background(), 7, active.FileID, now+1)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, MoliiFileStatusDeleted, deleted.Status)

	_, _, err = MarkMoliiFileDeleted(context.Background(), 8, active.FileID, now)
	assert.True(t, errors.Is(err, ErrMoliiFileNotFound))
}

func TestExpireMoliiFileByObjectKeyScrubsMetadataAndKeepsGoneContract(t *testing.T) {
	setupMoliiFileTestDB(t)
	now := time.Now().Unix()
	file := &MoliiFile{
		FileID: "file_expiring", UserID: 7, ObjectKey: "p/grok-files/7/file_expiring.mp4",
		Filename: "customer-video.mp4", Purpose: "video", Bytes: 1234, MIMEType: "video/mp4",
		MediaType: MoliiFileMediaTypeVideo, Width: 1280, Height: 720, DurationSeconds: 8.7,
		Status: MoliiFileStatusActive, CreatedAt: now - 86400, UpdatedAt: now - 86400, ExpiresAt: now,
	}
	require.NoError(t, CreateMoliiFile(context.Background(), file))
	require.NoError(t, ExpireMoliiFileByObjectKey(context.Background(), file.ObjectKey, now))

	_, err := GetMoliiFileForUser(context.Background(), 7, file.FileID, now)
	assert.ErrorIs(t, err, ErrMoliiFileExpired)
	record, err := GetMoliiFileRecordForUser(context.Background(), 7, file.FileID)
	require.NoError(t, err)
	assert.Equal(t, MoliiFileStatusExpired, record.Status)
	assert.Contains(t, record.ObjectKey, "expired/")
	assert.Empty(t, record.Filename)
	assert.Empty(t, record.Purpose)
	assert.Zero(t, record.Bytes)
	assert.Zero(t, record.Width)
	assert.Zero(t, record.Height)
	assert.Zero(t, record.DurationSeconds)

	deleted, changed, err := MarkMoliiFileDeleted(context.Background(), 7, file.FileID, now+1)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, MoliiFileStatusDeleted, deleted.Status)
}
