// Package store implements durable SQLite persistence for the demo.
package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"molii-aigc-demo/internal/secure"
)

const maskedAPIKey = "••••••••"

type Options struct {
	BusyTimeout  time.Duration
	MaxOpenConns int
	MaxIdleConns int
	LogLevel     logger.LogLevel
}

func DefaultOptions() Options {
	return Options{
		BusyTimeout:  5 * time.Second,
		MaxOpenConns: 4,
		MaxIdleConns: 4,
		LogLevel:     logger.Silent,
	}
}

type Store struct {
	db      *gorm.DB
	keyring *secure.Keyring
	path    string
	now     func() time.Time
}

// Open creates or opens a private SQLite database, configures WAL and
// connection-level safety pragmas, and applies explicit versioned migrations.
func Open(ctx context.Context, path string, keyring *secure.Keyring, options Options) (*Store, error) {
	if keyring == nil {
		return nil, fmt.Errorf("keyring is required: %w", ErrInvalidInput)
	}
	if strings.TrimSpace(path) == "" || path == ":memory:" {
		return nil, fmt.Errorf("a file-backed database path is required: %w", ErrInvalidInput)
	}
	if options.BusyTimeout <= 0 {
		options.BusyTimeout = DefaultOptions().BusyTimeout
	}
	if options.MaxOpenConns <= 0 {
		options.MaxOpenConns = DefaultOptions().MaxOpenConns
	}
	if options.MaxIdleConns < 0 {
		options.MaxIdleConns = 0
	} else if options.MaxIdleConns == 0 {
		options.MaxIdleConns = DefaultOptions().MaxIdleConns
	}
	if options.LogLevel < logger.Silent || options.LogLevel > logger.Info {
		options.LogLevel = logger.Silent
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	directory := filepath.Dir(absPath)
	_, directoryErr := os.Stat(directory)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	// Tighten permissions only when this call created the database directory.
	// An existing directory may be a shared application directory that Open must
	// not unexpectedly make inaccessible to other files.
	if errors.Is(directoryErr, os.ErrNotExist) {
		if err := os.Chmod(directory, 0o700); err != nil {
			return nil, fmt.Errorf("secure database directory: %w", err)
		}
	} else if directoryErr != nil {
		return nil, fmt.Errorf("inspect database directory: %w", directoryErr)
	}
	file, err := os.OpenFile(absPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure database file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close database file: %w", err)
	}

	dsn := sqliteDSN(absPath, options.BusyTimeout)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:  logger.Default.LogMode(options.LogLevel),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access sqlite connection: %w", err)
	}
	cleanup := func(openErr error) (*Store, error) {
		_ = sqlDB.Close()
		return nil, openErr
	}
	sqlDB.SetMaxOpenConns(options.MaxOpenConns)
	sqlDB.SetMaxIdleConns(options.MaxIdleConns)

	if err := sqlDB.PingContext(ctx); err != nil {
		return cleanup(fmt.Errorf("ping sqlite: %w", err))
	}
	var journalMode string
	if err := db.WithContext(ctx).Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		return cleanup(fmt.Errorf("read journal mode: %w", err))
	}
	if !strings.EqualFold(journalMode, "wal") {
		return cleanup(fmt.Errorf("sqlite journal mode is %q, want WAL", journalMode))
	}
	if err := migrate(ctx, db); err != nil {
		return cleanup(err)
	}
	if err := secureSQLiteFiles(absPath); err != nil {
		return cleanup(err)
	}
	return &Store{db: db, keyring: keyring, path: absPath, now: func() time.Time { return time.Now().UTC() }}, nil
}

func sqliteDSN(path string, busyTimeout time.Duration) string {
	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := u.Query()
	query.Add("_pragma", "busy_timeout("+fmt.Sprint(busyTimeout.Milliseconds())+")")
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "journal_mode(WAL)")
	u.RawQuery = query.Encode()
	return u.String()
}

func secureSQLiteFiles(path string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		candidate := path + suffix
		if err := os.Chmod(candidate, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("secure sqlite file %s: %w", filepath.Base(candidate), err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := secureSQLiteFiles(s.path); err != nil {
		return err
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("access sqlite connection: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close sqlite: %w", err)
	}
	return secureSQLiteFiles(s.path)
}

func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func normalizeDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unique constraint failed") || strings.Contains(message, "foreign key constraint failed") {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	return err
}
