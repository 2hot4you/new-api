package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Store) CreateUISession(ctx context.Context, session UISession) (UISession, error) {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	if len(session.CSRFTokenHash) < 16 || session.ExpiresAt.IsZero() {
		return UISession{}, fmt.Errorf("session hash and expiry are required: %w", ErrInvalidInput)
	}
	now := s.now()
	session.CreatedAt = now
	session.UpdatedAt = now
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return UISession{}, normalizeDatabaseError(err)
	}
	return session, nil
}

func (s *Store) GetUISession(ctx context.Context, id string) (UISession, error) {
	var session UISession
	if err := s.db.WithContext(ctx).First(&session, "id = ?", id).Error; err != nil {
		return UISession{}, normalizeDatabaseError(err)
	}
	return session, nil
}

func (s *Store) TouchUISession(ctx context.Context, id string, lastSeenAt, expiresAt time.Time) error {
	if strings.TrimSpace(id) == "" || lastSeenAt.IsZero() || expiresAt.IsZero() {
		return ErrInvalidInput
	}
	result := s.db.WithContext(ctx).Model(&UISession{}).Where("id = ?", id).Updates(map[string]any{
		"last_seen_at": lastSeenAt.UTC(), "expires_at": expiresAt.UTC(), "updated_at": s.now(),
	})
	if result.Error != nil {
		return normalizeDatabaseError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SelectEnvironment(ctx context.Context, sessionID string, environmentID *string) error {
	if strings.TrimSpace(sessionID) == "" {
		return ErrInvalidInput
	}
	if environmentID != nil && strings.TrimSpace(*environmentID) == "" {
		return ErrInvalidInput
	}
	result := s.db.WithContext(ctx).Model(&UISession{}).Where("id = ?", sessionID).Updates(map[string]any{
		"selected_environment_id": environmentID, "updated_at": s.now(),
	})
	if result.Error != nil {
		return normalizeDatabaseError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteUISession(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Delete(&UISession{}, "id = ?", id)
	if result.Error != nil {
		return normalizeDatabaseError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteExpiredUISessions(ctx context.Context, before time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Where("expires_at <= ?", before.UTC()).Delete(&UISession{})
	return result.RowsAffected, normalizeDatabaseError(result.Error)
}
