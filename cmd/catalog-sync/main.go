package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/catalogsync"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type environment func(string) (string, bool)

func main() {
	os.Exit(run(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr))
}

func run(args []string, getenv environment, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	var secrets []string
	trackedEnv := func(key string) (string, bool) {
		value, ok := getenv(key)
		if ok && value != "" {
			secrets = append(secrets, value)
		}
		return value, ok
	}

	ctx := context.Background()
	var err error
	switch args[0] {
	case "export":
		err = runExport(ctx, args[1:], trackedEnv, stdout)
	case "plan":
		err = runPlan(ctx, args[1:], trackedEnv, stdout)
	case "apply":
		err = runApply(ctx, args[1:], trackedEnv, stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
	if err == nil {
		return 0
	}
	fmt.Fprintln(stderr, redactError(err.Error(), secrets))
	var usageError *commandUsageError
	if errors.As(err, &usageError) {
		return 2
	}
	return 1
}

type commandUsageError struct {
	message string
}

func (err *commandUsageError) Error() string { return err.message }

func usageError(format string, values ...any) error {
	return &commandUsageError{message: fmt.Sprintf(format, values...)}
}

func runExport(ctx context.Context, args []string, getenv environment, stdout io.Writer) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dsnEnv := flags.String("dsn-env", "SQL_DSN", "environment variable containing the PostgreSQL DSN")
	output := flags.String("output", "", "snapshot output path")
	if err := flags.Parse(args); err != nil {
		return usageError("export: %v", err)
	}
	if *output == "" {
		return usageError("export: --output is required")
	}
	db, err := openDatabase(ctx, *dsnEnv, getenv)
	if err != nil {
		return err
	}
	snapshot, err := catalogsync.Export(ctx, db)
	if err != nil {
		return fmt.Errorf("export catalog snapshot: %w", err)
	}
	if err := writeAtomic(*output, 0o600, func(writer io.Writer) error {
		return catalogsync.WriteSnapshot(writer, snapshot)
	}); err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"snapshot":        *output,
		"content_digest":  snapshot.ContentDigest,
		"vendors":         len(snapshot.Vendors),
		"models":          len(snapshot.Models),
		"pricing_options": len(snapshot.Options),
	})
}

func runPlan(ctx context.Context, args []string, getenv environment, stdout io.Writer) error {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dsnEnv := flags.String("dsn-env", "SQL_DSN", "environment variable containing the PostgreSQL DSN")
	input := flags.String("input", "", "source snapshot path")
	output := flags.String("output", "", "optional plan output path")
	if err := flags.Parse(args); err != nil {
		return usageError("plan: %v", err)
	}
	if *input == "" {
		return usageError("plan: --input is required")
	}
	snapshot, err := readSnapshotFile(*input)
	if err != nil {
		return err
	}
	db, err := openDatabase(ctx, *dsnEnv, getenv)
	if err != nil {
		return err
	}
	plan, err := catalogsync.BuildPlan(ctx, db, snapshot)
	if err != nil {
		return fmt.Errorf("build catalog sync plan: %w", err)
	}
	if *output != "" {
		if err := writeAtomic(*output, 0o600, func(writer io.Writer) error { return writeJSON(writer, plan) }); err != nil {
			return err
		}
		return writeJSON(stdout, map[string]any{"plan": *output, "confirmation_digest": plan.ConfirmationDigest, "summary": plan.Summary})
	}
	return writeJSON(stdout, plan)
}

func runApply(ctx context.Context, args []string, getenv environment, stdout io.Writer) error {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dsnEnv := flags.String("dsn-env", "SQL_DSN", "environment variable containing the PostgreSQL DSN")
	input := flags.String("input", "", "source snapshot path")
	confirm := flags.String("confirm", "", "confirmation digest returned by plan")
	backupDir := flags.String("backup-dir", "", "directory for the target backup snapshot")
	targetName := flags.String("target-name", "target", "safe target label used in backup filename")
	if err := flags.Parse(args); err != nil {
		return usageError("apply: %v", err)
	}
	if *input == "" {
		return usageError("apply: --input is required")
	}
	if *confirm == "" {
		return usageError("apply: --confirm is required")
	}
	if *backupDir == "" {
		return usageError("apply: --backup-dir is required")
	}
	snapshot, err := readSnapshotFile(*input)
	if err != nil {
		return err
	}
	db, err := openDatabase(ctx, *dsnEnv, getenv)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*backupDir, 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	backupPath := filepath.Join(*backupDir, backupFilename(*targetName))
	backupFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create backup snapshot: %w", err)
	}
	keepBackup := false
	defer func() {
		_ = backupFile.Close()
		if !keepBackup {
			_ = os.Remove(backupPath)
		}
	}()
	result, err := catalogsync.Apply(ctx, db, snapshot, *confirm, backupFile)
	if err != nil {
		return fmt.Errorf("apply catalog sync: %w", err)
	}
	keepBackup = true
	if err := backupFile.Close(); err != nil {
		return fmt.Errorf("close backup snapshot: %w", err)
	}
	return writeJSON(stdout, map[string]any{
		"backup":              backupPath,
		"backup_digest":       result.BackupDigest,
		"confirmation_digest": result.ConfirmationDigest,
		"summary":             result.Summary,
	})
}

func openDatabase(ctx context.Context, envName string, getenv environment) (*gorm.DB, error) {
	dsn, ok := getenv(envName)
	if !ok || strings.TrimSpace(dsn) == "" {
		return nil, usageError("database environment variable %s is not set", envName)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL using %s: failed", envName)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("initialize PostgreSQL using %s: failed", envName)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL using %s: failed", envName)
	}
	return db, nil
}

func readSnapshotFile(path string) (catalogsync.Snapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return catalogsync.Snapshot{}, fmt.Errorf("open snapshot: %w", err)
	}
	defer file.Close()
	snapshot, err := catalogsync.ReadSnapshot(file)
	if err != nil {
		return catalogsync.Snapshot{}, err
	}
	return snapshot, nil
}

func writeAtomic(path string, mode os.FileMode, write func(io.Writer) error) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".catalog-sync-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return fmt.Errorf("set output permissions: %w", err)
	}
	if err := write(temporary); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish output: %w", err)
	}
	keep = true
	return nil
}

func writeJSON(writer io.Writer, value any) error {
	encoded, err := common.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode command output: %w", err)
	}
	indented, err := common.IndentJson(encoded)
	if err != nil {
		return fmt.Errorf("format command output: %w", err)
	}
	if _, err := writer.Write(append(indented, '\n')); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

var unsafeFilenameCharacters = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func backupFilename(targetName string) string {
	targetName = strings.Trim(unsafeFilenameCharacters.ReplaceAllString(targetName, "-"), "-.")
	if targetName == "" {
		targetName = "target"
	}
	return fmt.Sprintf("catalog-backup-%s-%s.json", targetName, time.Now().UTC().Format("20060102T150405Z"))
}

func redactError(message string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return message
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "catalog-sync copies only vendor, model metadata, and model pricing configuration.")
	fmt.Fprintln(writer, "usage:")
	fmt.Fprintln(writer, "  catalog-sync export --dsn-env SQL_DSN --output snapshot.json")
	fmt.Fprintln(writer, "  catalog-sync plan --dsn-env SQL_DSN --input snapshot.json [--output plan.json]")
	fmt.Fprintln(writer, "  catalog-sync apply --dsn-env SQL_DSN --input snapshot.json --confirm sha256:... --backup-dir backups [--target-name name]")
}
