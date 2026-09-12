package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/internal/catalogsync"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var commandTestDatabaseID atomic.Uint64

func testEnv(values map[string]string) environment {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func TestRunHelpListsSafeThreeStepWorkflow(t *testing.T) {
	var stdout bytes.Buffer
	exitCode := run([]string{"help"}, testEnv(nil), &stdout, &bytes.Buffer{})
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, stdout.String(), "catalog-sync export")
	assert.Contains(t, stdout.String(), "catalog-sync plan")
	assert.Contains(t, stdout.String(), "catalog-sync apply")
	assert.Contains(t, stdout.String(), "--confirm")
}

func openCommandTestDB(t *testing.T, dsn string) (*gorm.DB, string) {
	t.Helper()
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schemaName := fmt.Sprintf("catalog_sync_cmd_%d_%d", os.Getpid(), commandTestDatabaseID.Add(1))
	require.NoError(t, admin.Exec(`CREATE SCHEMA "`+schemaName+`"`).Error)
	t.Cleanup(func() {
		require.NoError(t, admin.Exec(`DROP SCHEMA "`+schemaName+`" CASCADE`).Error)
	})
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("search_path", schemaName)
	parsed.RawQuery = query.Encode()
	scopedDSN := parsed.String()
	db, err := gorm.Open(postgres.Open(scopedDSN), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.Vendor{}, &model.Model{}))
	return db, scopedDSN
}

func TestRunRejectsMissingDSNEnvironmentVariable(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"export", "--dsn-env", "MISSING", "--output", filepath.Join(t.TempDir(), "snapshot.json")}, testEnv(nil), &stdout, &stderr)
	assert.Equal(t, 2, exitCode)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "MISSING")
}

func TestRunApplyRequiresConfirmationBeforeConnecting(t *testing.T) {
	input := filepath.Join(t.TempDir(), "snapshot.json")
	require.NoError(t, os.WriteFile(input, []byte(`{"schema_version":1,"vendors":[],"models":[],"options":{}}`), 0o600))
	var stderr bytes.Buffer
	exitCode := run([]string{"apply", "--input", input, "--backup-dir", t.TempDir()}, testEnv(nil), &bytes.Buffer{}, &stderr)
	assert.Equal(t, 2, exitCode)
	assert.Contains(t, stderr.String(), "--confirm")
	assert.NotContains(t, stderr.String(), "SQL_DSN")
}

func TestRunPlanDoesNotModifyTargetDatabase(t *testing.T) {
	dsn := os.Getenv("TEST_CATALOG_SYNC_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_CATALOG_SYNC_POSTGRES_DSN is not configured")
	}
	db, scopedDSN := openCommandTestDB(t, dsn)
	targetVendor := model.Vendor{Name: "TargetOnly", Status: 1}
	require.NoError(t, db.Create(&targetVendor).Error)

	snapshot := catalogsync.Snapshot{
		SchemaVersion: catalogsync.SnapshotSchemaVersion,
		Vendors:       []catalogsync.VendorRecord{{Name: "OpenAI", Status: 1}},
		Models:        []catalogsync.ModelRecord{{ModelName: "gpt-test", Vendor: "OpenAI", BillingCurrency: "USD", Status: 1}},
	}
	input := filepath.Join(t.TempDir(), "snapshot.json")
	inputFile, err := os.OpenFile(input, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	require.NoError(t, err)
	require.NoError(t, catalogsync.WriteSnapshot(inputFile, snapshot))
	require.NoError(t, inputFile.Close())

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"plan", "--dsn-env", "TEST_DSN", "--input", input}, testEnv(map[string]string{"TEST_DSN": scopedDSN}), &stdout, &stderr)
	assert.Equal(t, 0, exitCode, stderr.String())
	assert.Contains(t, stdout.String(), `"confirmation_digest"`)
	assert.Contains(t, stdout.String(), `"vendors_created": 1`)
	var vendors []model.Vendor
	require.NoError(t, db.Find(&vendors).Error)
	require.Len(t, vendors, 1)
	assert.Equal(t, "TargetOnly", vendors[0].Name)
}

func TestRunDatabaseErrorDoesNotExposePassword(t *testing.T) {
	secret := "never-print-this-password"
	var stderr bytes.Buffer
	exitCode := run(
		[]string{"export", "--dsn-env", "BROKEN_DSN", "--output", filepath.Join(t.TempDir(), "snapshot.json")},
		testEnv(map[string]string{"BROKEN_DSN": "postgresql://catalog:" + secret + "@127.0.0.1:1/catalog?sslmode=disable"}),
		&bytes.Buffer{},
		&stderr,
	)
	assert.Equal(t, 1, exitCode)
	assert.NotContains(t, stderr.String(), secret)
}
