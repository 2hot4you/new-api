package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type MoliiFileStatus string
type MoliiFileMediaType string

const (
	MoliiFileStatusActive  MoliiFileStatus = "active"
	MoliiFileStatusDeleted MoliiFileStatus = "deleted"
	MoliiFileStatusExpired MoliiFileStatus = "expired"

	MoliiFileMediaTypeImage MoliiFileMediaType = "image"
	MoliiFileMediaTypeVideo MoliiFileMediaType = "video"
)

var (
	ErrMoliiFileNotFound = errors.New("file not found")
	ErrMoliiFileExpired  = errors.New("file expired")
)

type MoliiFile struct {
	ID              int64              `json:"-" gorm:"primaryKey"`
	FileID          string             `json:"id" gorm:"type:varchar(191);uniqueIndex"`
	UserID          int                `json:"-" gorm:"index:idx_molii_files_user_status_expiry,priority:1"`
	ObjectKey       string             `json:"-" gorm:"type:varchar(512);uniqueIndex"`
	Filename        string             `json:"filename" gorm:"type:varchar(255)"`
	Purpose         string             `json:"purpose" gorm:"type:varchar(64)"`
	Bytes           int64              `json:"bytes"`
	MIMEType        string             `json:"mime_type" gorm:"type:varchar(127)"`
	MediaType       MoliiFileMediaType `json:"media_type" gorm:"type:varchar(16)"`
	Width           int                `json:"width,omitempty"`
	Height          int                `json:"height,omitempty"`
	DurationSeconds float64            `json:"duration_seconds,omitempty"`
	Status          MoliiFileStatus    `json:"status" gorm:"type:varchar(20);index:idx_molii_files_user_status_expiry,priority:2"`
	CreatedAt       int64              `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64              `json:"-" gorm:"bigint"`
	ExpiresAt       int64              `json:"expires_at" gorm:"bigint;index:idx_molii_files_user_status_expiry,priority:3"`
}

func (MoliiFile) TableName() string { return "molii_files" }

func (file *MoliiFile) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	file.FileID = strings.TrimSpace(file.FileID)
	file.ObjectKey = strings.TrimSpace(file.ObjectKey)
	file.Filename = strings.TrimSpace(file.Filename)
	file.Purpose = strings.TrimSpace(file.Purpose)
	file.MIMEType = strings.ToLower(strings.TrimSpace(file.MIMEType))
	if file.Status == "" {
		file.Status = MoliiFileStatusActive
	}
	if file.CreatedAt == 0 {
		file.CreatedAt = now
	}
	if file.UpdatedAt == 0 {
		file.UpdatedAt = file.CreatedAt
	}
	return nil
}

func CreateMoliiFile(ctx context.Context, file *MoliiFile) error {
	if file == nil || file.UserID <= 0 || strings.TrimSpace(file.FileID) == "" || strings.TrimSpace(file.ObjectKey) == "" || file.ExpiresAt <= 0 {
		return errors.New("valid Molii file metadata is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if DB == nil {
		return errors.New("database is unavailable")
	}
	return DB.WithContext(ctx).Create(file).Error
}

func GetMoliiFileForUser(ctx context.Context, userID int, fileID string, now int64) (*MoliiFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if DB == nil {
		return nil, errors.New("database is unavailable")
	}
	var file MoliiFile
	err := DB.WithContext(ctx).Where("user_id = ? AND file_id = ?", userID, strings.TrimSpace(fileID)).First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || file.Status == MoliiFileStatusDeleted {
		return nil, ErrMoliiFileNotFound
	}
	if err != nil {
		return nil, err
	}
	if file.Status == MoliiFileStatusExpired || now >= file.ExpiresAt {
		return nil, ErrMoliiFileExpired
	}
	return &file, nil
}

func ListActiveMoliiFiles(ctx context.Context, userID int, now int64) ([]*MoliiFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if DB == nil {
		return nil, errors.New("database is unavailable")
	}
	files := make([]*MoliiFile, 0)
	err := DB.WithContext(ctx).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, MoliiFileStatusActive, now).
		Order("id DESC").Find(&files).Error
	return files, err
}

func GetMoliiFileRecordForUser(ctx context.Context, userID int, fileID string) (*MoliiFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if DB == nil {
		return nil, errors.New("database is unavailable")
	}
	var file MoliiFile
	err := DB.WithContext(ctx).Where("user_id = ? AND file_id = ?", userID, strings.TrimSpace(fileID)).First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMoliiFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func MarkMoliiFileDeleted(ctx context.Context, userID int, fileID string, now int64) (*MoliiFile, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if DB == nil {
		return nil, false, errors.New("database is unavailable")
	}
	var file MoliiFile
	err := DB.WithContext(ctx).Where("user_id = ? AND file_id = ?", userID, strings.TrimSpace(fileID)).First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, ErrMoliiFileNotFound
	}
	if err != nil {
		return nil, false, err
	}
	if file.Status == MoliiFileStatusDeleted {
		return &file, false, nil
	}
	updates := scrubMoliiFileMetadata(file.FileID, MoliiFileStatusDeleted, now)
	result := DB.WithContext(ctx).Model(&MoliiFile{}).
		Where("id = ? AND status IN ?", file.ID, []MoliiFileStatus{MoliiFileStatusActive, MoliiFileStatusExpired}).
		Updates(updates)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return MarkMoliiFileDeleted(ctx, userID, fileID, now)
	}
	applyMoliiFileScrub(&file, MoliiFileStatusDeleted, now)
	return &file, true, nil
}

func ExpireMoliiFileByObjectKey(ctx context.Context, objectKey string, now int64) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if DB == nil {
		return errors.New("database is unavailable")
	}
	var file MoliiFile
	err := DB.WithContext(ctx).Where("object_key = ? AND status = ?", strings.TrimSpace(objectKey), MoliiFileStatusActive).First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return DB.WithContext(ctx).Model(&MoliiFile{}).Where("id = ? AND status = ?", file.ID, MoliiFileStatusActive).
		Updates(scrubMoliiFileMetadata(file.FileID, MoliiFileStatusExpired, now)).Error
}

func scrubMoliiFileMetadata(fileID string, status MoliiFileStatus, now int64) map[string]any {
	return map[string]any{
		"object_key": fmt.Sprintf("%s/%s", status, strings.TrimSpace(fileID)),
		"filename":   "", "purpose": "", "bytes": int64(0), "mime_type": "", "media_type": "",
		"width": 0, "height": 0, "duration_seconds": float64(0), "status": status, "updated_at": now,
	}
}

func applyMoliiFileScrub(file *MoliiFile, status MoliiFileStatus, now int64) {
	if file == nil {
		return
	}
	file.ObjectKey = fmt.Sprintf("%s/%s", status, strings.TrimSpace(file.FileID))
	file.Filename, file.Purpose, file.MIMEType, file.MediaType = "", "", "", ""
	file.Bytes, file.Width, file.Height, file.DurationSeconds = 0, 0, 0, 0
	file.Status, file.UpdatedAt = status, now
}
