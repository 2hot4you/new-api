package store

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type migration struct {
	version    int
	statements []string
}

var migrations = []migration{{
	version: 1,
	statements: []string{
		`CREATE TABLE IF NOT EXISTS environments (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			base_url TEXT NOT NULL,
			api_key_ciphertext BLOB NOT NULL,
			api_key_nonce BLOB NOT NULL,
			api_key_version INTEGER NOT NULL CHECK (api_key_version >= 0),
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS ui_sessions (
			id TEXT PRIMARY KEY,
			csrf_token_hash BLOB NOT NULL,
			selected_environment_id TEXT NULL REFERENCES environments(id) ON DELETE SET NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			last_seen_at DATETIME NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ui_sessions_expires_at ON ui_sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			ui_session_id TEXT NULL REFERENCES ui_sessions(id) ON DELETE SET NULL,
			environment_id TEXT NULL REFERENCES environments(id) ON DELETE SET NULL,
			environment_name TEXT NOT NULL,
			base_url TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			operation TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending','submitted','polling','succeeded','failed','canceled')),
			request_id TEXT NOT NULL DEFAULT '',
			upstream_task_id TEXT NOT NULL DEFAULT '',
			progress REAL NULL,
			poll_attempts INTEGER NOT NULL DEFAULT 0 CHECK (poll_attempts >= 0),
			cancel_requested INTEGER NOT NULL DEFAULT 0,
			request_json BLOB NOT NULL,
			result_json BLOB NULL,
			error_code TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			estimated_billing_json BLOB NULL,
			estimated_amount TEXT NULL,
			actual_billing_json BLOB NULL,
			actual_amount TEXT NULL,
			delta_billing_json BLOB NULL,
			delta_amount TEXT NULL,
			currency TEXT NOT NULL DEFAULT '',
			submitted_at DATETIME NULL,
			completed_at DATETIME NULL,
			next_poll_at DATETIME NULL,
			last_polled_at DATETIME NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_session_created ON runs(ui_session_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_environment_created ON runs(environment_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_poll_recovery ON runs(status, cancel_requested, next_poll_at)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_request_id ON runs(request_id)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_upstream_task_id ON runs(upstream_task_id)`,
		`CREATE TABLE IF NOT EXISTS exchanges (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
			sequence INTEGER NOT NULL CHECK (sequence > 0),
			kind TEXT NOT NULL,
			method TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			request_headers_json BLOB NULL,
			request_body_json BLOB NULL,
			response_status INTEGER NULL,
			response_headers_json BLOB NULL,
			response_body_json BLOB NULL,
			error TEXT NOT NULL DEFAULT '',
			started_at DATETIME NOT NULL,
			finished_at DATETIME NULL,
			duration_ms INTEGER NULL CHECK (duration_ms IS NULL OR duration_ms >= 0),
			created_at DATETIME NOT NULL,
			UNIQUE(run_id, sequence)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_exchanges_run_sequence ON exchanges(run_id, sequence)`,
	},
}, {
	version: 2,
	statements: []string{
		`CREATE TABLE IF NOT EXISTS run_media_sources (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
			environment_id TEXT NOT NULL,
			position INTEGER NOT NULL CHECK (position >= 0),
			url_ciphertext BLOB NOT NULL,
			url_nonce BLOB NOT NULL,
			url_key_version INTEGER NOT NULL CHECK (url_key_version >= 0),
			created_at DATETIME NOT NULL,
			UNIQUE(run_id, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_run_media_sources_run ON run_media_sources(run_id, position)`,
	},
}}

func migrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL
	)`).Error; err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}
	for _, item := range migrations {
		var count int64
		if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", item.version).Scan(&count).Error; err != nil {
			return fmt.Errorf("check migration %d: %w", item.version, err)
		}
		if count != 0 {
			continue
		}
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, statement := range item.statements {
				if err := tx.Exec(statement).Error; err != nil {
					return err
				}
			}
			return tx.Exec("INSERT INTO schema_migrations(version, applied_at) VALUES (?, CURRENT_TIMESTAMP)", item.version).Error
		}); err != nil {
			return fmt.Errorf("apply migration %d: %w", item.version, err)
		}
	}
	return nil
}
