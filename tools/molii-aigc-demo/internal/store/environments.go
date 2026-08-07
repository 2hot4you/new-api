package store

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"molii-aigc-demo/internal/secure"
)

type environmentRecord struct {
	ID               string    `gorm:"column:id;primaryKey"`
	Name             string    `gorm:"column:name"`
	BaseURL          string    `gorm:"column:base_url"`
	APIKeyCiphertext []byte    `gorm:"column:api_key_ciphertext"`
	APIKeyNonce      []byte    `gorm:"column:api_key_nonce"`
	APIKeyVersion    uint32    `gorm:"column:api_key_version"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (environmentRecord) TableName() string { return "environments" }

func (s *Store) CreateEnvironment(ctx context.Context, params CreateEnvironmentParams) (Environment, error) {
	name, baseURL, apiKey, err := validateEnvironment(params.Name, params.BaseURL, params.APIKey)
	if err != nil {
		return Environment{}, err
	}
	id := uuid.NewString()
	sealed, err := s.keyring.Encrypt(id, []byte(apiKey))
	if err != nil {
		return Environment{}, fmt.Errorf("encrypt API key: %w", err)
	}
	now := s.now()
	record := environmentRecord{
		ID: id, Name: name, BaseURL: baseURL,
		APIKeyCiphertext: sealed.Ciphertext, APIKeyNonce: sealed.Nonce, APIKeyVersion: sealed.KeyVersion,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return Environment{}, normalizeDatabaseError(err)
	}
	return environmentDTO(record), nil
}

func (s *Store) UpdateEnvironment(ctx context.Context, id string, params UpdateEnvironmentParams) (Environment, error) {
	if strings.TrimSpace(id) == "" {
		return Environment{}, fmt.Errorf("environment ID is required: %w", ErrInvalidInput)
	}
	var result environmentRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&result, "id = ?", id).Error; err != nil {
			return err
		}
		updates := map[string]any{"updated_at": s.now()}
		if params.Name != nil {
			name := strings.TrimSpace(*params.Name)
			if name == "" {
				return fmt.Errorf("environment name is required: %w", ErrInvalidInput)
			}
			updates["name"] = name
		}
		if params.BaseURL != nil {
			baseURL, err := normalizeBaseURL(*params.BaseURL)
			if err != nil {
				return err
			}
			updates["base_url"] = baseURL
		}
		if params.APIKey != nil {
			apiKey := strings.TrimSpace(*params.APIKey)
			if apiKey == "" {
				return ErrSecretRequired
			}
			sealed, err := s.keyring.Encrypt(id, []byte(apiKey))
			if err != nil {
				return fmt.Errorf("encrypt API key: %w", err)
			}
			updates["api_key_ciphertext"] = sealed.Ciphertext
			updates["api_key_nonce"] = sealed.Nonce
			updates["api_key_version"] = sealed.KeyVersion
		}
		if err := tx.Model(&environmentRecord{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&result, "id = ?", id).Error
	})
	if err != nil {
		return Environment{}, normalizeDatabaseError(err)
	}
	return environmentDTO(result), nil
}

func (s *Store) GetEnvironment(ctx context.Context, id string) (Environment, error) {
	var record environmentRecord
	if err := s.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		return Environment{}, normalizeDatabaseError(err)
	}
	return environmentDTO(record), nil
}

func (s *Store) ListEnvironments(ctx context.Context) ([]Environment, error) {
	var records []environmentRecord
	if err := s.db.WithContext(ctx).Order("created_at ASC, id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]Environment, 0, len(records))
	for _, record := range records {
		result = append(result, environmentDTO(record))
	}
	return result, nil
}

// GetEnvironmentCredentials decrypts an environment for an immediate
// server-side upstream request. Never marshal or log the returned value.
func (s *Store) GetEnvironmentCredentials(ctx context.Context, id string) (EnvironmentCredentials, error) {
	var record environmentRecord
	if err := s.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		return EnvironmentCredentials{}, normalizeDatabaseError(err)
	}
	plaintext, err := s.keyring.Decrypt(record.ID, secure.SealedValue{
		Ciphertext: record.APIKeyCiphertext,
		Nonce:      record.APIKeyNonce,
		KeyVersion: record.APIKeyVersion,
	})
	if err != nil {
		return EnvironmentCredentials{}, fmt.Errorf("decrypt environment API key: %w", err)
	}
	return EnvironmentCredentials{ID: record.ID, Name: record.Name, BaseURL: record.BaseURL, APIKey: string(plaintext)}, nil
}

func (s *Store) DeleteEnvironment(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Delete(&environmentRecord{}, "id = ?", id)
	if result.Error != nil {
		return normalizeDatabaseError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func validateEnvironment(name, rawBaseURL, apiKey string) (string, string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", "", fmt.Errorf("environment name is required: %w", ErrInvalidInput)
	}
	baseURL, err := normalizeBaseURL(rawBaseURL)
	if err != nil {
		return "", "", "", err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", "", "", ErrSecretRequired
	}
	return name, baseURL, apiKey, nil
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Opaque != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("base URL must be an absolute http(s) URL: %w", ErrInvalidInput)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("base URL must not contain credentials, path, query, or fragment: %w", ErrInvalidInput)
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return "", fmt.Errorf("non-loopback base URLs require HTTPS: %w", ErrInvalidInput)
	}
	parsed.Path = ""
	return parsed.String(), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func environmentDTO(record environmentRecord) Environment {
	return Environment{
		ID: record.ID, Name: record.Name, BaseURL: record.BaseURL,
		KeyConfigured: len(record.APIKeyCiphertext) != 0, KeyMasked: maskedAPIKey,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}
