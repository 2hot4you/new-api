package store

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"molii-aigc-demo/internal/secure"
)

type runMediaSourceRecord struct {
	ID            string    `gorm:"column:id;primaryKey"`
	RunID         string    `gorm:"column:run_id"`
	EnvironmentID string    `gorm:"column:environment_id"`
	Position      int       `gorm:"column:position"`
	URLCiphertext []byte    `gorm:"column:url_ciphertext"`
	URLNonce      []byte    `gorm:"column:url_nonce"`
	URLKeyVersion uint32    `gorm:"column:url_key_version"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (runMediaSourceRecord) TableName() string { return "run_media_sources" }

// ReplaceRunMediaSources encrypts result URLs outside of the public run and
// exchange JSON. This preserves signed-media playback without persisting
// bearer-like query strings in plaintext.
func (s *Store) ReplaceRunMediaSources(ctx context.Context, runID, environmentID string, sources []string) error {
	runID = strings.TrimSpace(runID)
	environmentID = strings.TrimSpace(environmentID)
	if runID == "" || environmentID == "" {
		return ErrInvalidInput
	}
	records := make([]runMediaSourceRecord, 0, len(sources))
	for position, source := range sources {
		source = strings.TrimSpace(source)
		if err := validateMediaSourceURL(source); err != nil {
			return err
		}
		sealed, err := s.keyring.Encrypt(environmentID, []byte(source))
		if err != nil {
			return fmt.Errorf("encrypt media source: %w", err)
		}
		records = append(records, runMediaSourceRecord{
			ID: uuid.NewString(), RunID: runID, EnvironmentID: environmentID,
			Position: position, URLCiphertext: sealed.Ciphertext, URLNonce: sealed.Nonce,
			URLKeyVersion: sealed.KeyVersion, CreatedAt: s.now(),
		})
	}
	return normalizeDatabaseError(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if result := tx.Delete(&runMediaSourceRecord{}, "run_id = ?", runID); result.Error != nil {
			return result.Error
		}
		if len(records) == 0 {
			return nil
		}
		return tx.Create(&records).Error
	}))
}

func (s *Store) GetRunMediaSource(ctx context.Context, runID string, position int) (RunMediaSource, error) {
	if strings.TrimSpace(runID) == "" || position < 0 {
		return RunMediaSource{}, ErrInvalidInput
	}
	var record runMediaSourceRecord
	if err := s.db.WithContext(ctx).First(&record, "run_id = ? AND position = ?", runID, position).Error; err != nil {
		return RunMediaSource{}, normalizeDatabaseError(err)
	}
	plaintext, err := s.keyring.Decrypt(record.EnvironmentID, secure.SealedValue{
		Ciphertext: record.URLCiphertext, Nonce: record.URLNonce, KeyVersion: record.URLKeyVersion,
	})
	if err != nil {
		return RunMediaSource{}, fmt.Errorf("decrypt media source: %w", err)
	}
	source := string(plaintext)
	if err := validateMediaSourceURL(source); err != nil {
		return RunMediaSource{}, err
	}
	return RunMediaSource{RunID: record.RunID, EnvironmentID: record.EnvironmentID, Position: record.Position, URL: source}, nil
}

func validateMediaSourceURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Opaque != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("media source must be an absolute http(s) URL: %w", ErrInvalidInput)
	}
	if parsed.User != nil || parsed.Fragment != "" || strings.ContainsAny(raw, "\r\n") {
		return fmt.Errorf("media source contains forbidden URL components: %w", ErrInvalidInput)
	}
	return nil
}
