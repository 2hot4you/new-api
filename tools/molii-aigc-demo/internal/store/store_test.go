package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"molii-aigc-demo/internal/secure"
)

var fixedNow = time.Date(2026, time.August, 7, 9, 30, 0, 0, time.UTC)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, secure.MasterKeySize))
	keyring, err := secure.NewKeyring(key, 3)
	require.NoError(t, err)
	directory := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(directory, "demo.sqlite")
	options := DefaultOptions()
	options.MaxOpenConns = 1
	options.MaxIdleConns = 1
	store, err := Open(context.Background(), path, keyring, options)
	require.NoError(t, err)
	store.now = func() time.Time { return fixedNow }
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store, path
}

func TestOpenConfiguresSQLiteAndPermissions(t *testing.T) {
	store, path := newTestStore(t)

	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
	directoryInfo, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), directoryInfo.Mode().Perm())

	var journalMode string
	require.NoError(t, store.db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error)
	require.Equal(t, "wal", journalMode)
	var foreignKeys int
	require.NoError(t, store.db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error)
	require.Equal(t, 1, foreignKeys)
	var busyTimeout int
	require.NoError(t, store.db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error)
	require.Equal(t, 5000, busyTimeout)
	var migrationCount int
	require.NoError(t, store.db.Raw("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount).Error)
	require.Equal(t, len(migrations), migrationCount)
}

func TestEnvironmentCRUDEncryptsSecretsAndReturnsMaskedDTOs(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	environment, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{
		Name: "Local", BaseURL: "https://new-api.test/", APIKey: "sk-super-secret",
	})
	require.NoError(t, err)
	require.Equal(t, "https://new-api.test", environment.BaseURL)
	require.Equal(t, maskedAPIKey, environment.KeyMasked)
	require.True(t, environment.KeyConfigured)

	serialized, err := json.Marshal(environment)
	require.NoError(t, err)
	require.NotContains(t, string(serialized), "super-secret")
	require.NotContains(t, string(serialized), "ciphertext")

	var record environmentRecord
	require.NoError(t, store.db.First(&record, "id = ?", environment.ID).Error)
	require.NotContains(t, string(record.APIKeyCiphertext), "sk-super-secret")
	require.Len(t, record.APIKeyNonce, secure.NonceSize)
	require.Equal(t, uint32(3), record.APIKeyVersion)

	credentials, err := store.GetEnvironmentCredentials(ctx, environment.ID)
	require.NoError(t, err)
	require.Equal(t, "sk-super-secret", credentials.APIKey)
	credentialJSON, err := json.Marshal(credentials)
	require.NoError(t, err)
	require.NotContains(t, string(credentialJSON), credentials.APIKey)

	newName, newKey := "Production", "sk-replaced"
	updated, err := store.UpdateEnvironment(ctx, environment.ID, UpdateEnvironmentParams{Name: &newName, APIKey: &newKey})
	require.NoError(t, err)
	require.Equal(t, newName, updated.Name)
	credentials, err = store.GetEnvironmentCredentials(ctx, environment.ID)
	require.NoError(t, err)
	require.Equal(t, newKey, credentials.APIKey)

	environments, err := store.ListEnvironments(ctx)
	require.NoError(t, err)
	require.Len(t, environments, 1)
	require.NoError(t, store.DeleteEnvironment(ctx, environment.ID))
	_, err = store.GetEnvironment(ctx, environment.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestEnvironmentValidationAndUniqueness(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "bad", BaseURL: "ftp://example.test", APIKey: "key"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "bad", BaseURL: "https://user:pass@example.test", APIKey: "key"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "bad", BaseURL: "https://example.test", APIKey: ""})
	require.ErrorIs(t, err, ErrSecretRequired)
	_, err = store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "bad", BaseURL: "http://example.test", APIKey: "key"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "bad", BaseURL: "http://192.168.1.2", APIKey: "key"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "bad", BaseURL: "https://example.test/api", APIKey: "key"})
	require.ErrorIs(t, err, ErrInvalidInput)
	_, err = store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "bad", BaseURL: "https://example.test?", APIKey: "key"})
	require.ErrorIs(t, err, ErrInvalidInput)

	local, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "local", BaseURL: "http://localhost:3000/", APIKey: "key"})
	require.NoError(t, err)
	require.Equal(t, "http://localhost:3000", local.BaseURL)
	loopback4, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "loopback4", BaseURL: "http://127.42.3.1:8080", APIKey: "key"})
	require.NoError(t, err)
	require.Equal(t, "http://127.42.3.1:8080", loopback4.BaseURL)
	loopback6, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "loopback6", BaseURL: "http://[::1]:8080", APIKey: "key"})
	require.NoError(t, err)
	require.Equal(t, "http://[::1]:8080", loopback6.BaseURL)

	_, err = store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "same", BaseURL: "https://one.test", APIKey: "key"})
	require.NoError(t, err)
	_, err = store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "same", BaseURL: "https://two.test", APIKey: "key"})
	require.ErrorIs(t, err, ErrConflict)
}

func TestUISessionLifecycleAndEnvironmentDeletion(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	environment, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "env", BaseURL: "https://example.test", APIKey: "key"})
	require.NoError(t, err)
	hash := sha256.Sum256([]byte("csrf-token"))
	session, err := store.CreateUISession(ctx, UISession{
		ID: "session-1", CSRFTokenHash: hash[:], ExpiresAt: fixedNow.Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, store.SelectEnvironment(ctx, session.ID, &environment.ID))

	got, err := store.GetUISession(ctx, session.ID)
	require.NoError(t, err)
	require.Equal(t, environment.ID, *got.SelectedEnvironmentID)
	require.NoError(t, store.TouchUISession(ctx, session.ID, fixedNow.Add(time.Minute), fixedNow.Add(2*time.Hour)))

	require.NoError(t, store.DeleteEnvironment(ctx, environment.ID))
	got, err = store.GetUISession(ctx, session.ID)
	require.NoError(t, err)
	require.Nil(t, got.SelectedEnvironmentID)

	count, err := store.DeleteExpiredUISessions(ctx, fixedNow.Add(3*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	_, err = store.GetUISession(ctx, session.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRunTimelinePollingRecoveryAndBilling(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	environment, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{Name: "env", BaseURL: "https://example.test", APIKey: "key"})
	require.NoError(t, err)
	hash := sha256.Sum256([]byte("csrf"))
	session, err := store.CreateUISession(ctx, UISession{ID: "s1", CSRFTokenHash: hash[:], ExpiresAt: fixedNow.Add(time.Hour)})
	require.NoError(t, err)
	estimated := decimal.RequireFromString("0.125")
	run, err := store.CreateRun(ctx, CreateRunParams{
		ID: "run-1", UISessionID: &session.ID, EnvironmentID: &environment.ID,
		EnvironmentName: environment.Name, BaseURL: environment.BaseURL,
		Provider: "volcengine", Model: "seedance-1-5-pro", Operation: "video",
		RequestJSON:          []byte(`{"model":"seedance-1-5-pro","prompt":"hello"}`),
		EstimatedBillingJSON: []byte(`{"source":"pricing"}`), EstimatedAmount: &estimated, Currency: "USD",
	})
	require.NoError(t, err)
	require.Equal(t, RunPending, run.Status)

	nextPoll := fixedNow.Add(10 * time.Second)
	require.NoError(t, store.MarkRunSubmitted(ctx, run.ID, SubmissionUpdate{
		RequestID: "req-1", UpstreamTaskID: "task-1", NextPollAt: &nextPoll, SubmittedAt: fixedNow,
	}))
	recoverable, err := store.ListRecoverableRuns(ctx, fixedNow.Add(5*time.Second), 10)
	require.NoError(t, err)
	require.Empty(t, recoverable)
	recoverable, err = store.ListRecoverableRuns(ctx, nextPoll, 10)
	require.NoError(t, err)
	require.Len(t, recoverable, 1)

	status := 202
	finished := fixedNow.Add(250 * time.Millisecond)
	exchange1, err := store.AppendExchange(ctx, Exchange{
		RunID: run.ID, Kind: "submission", Method: "POST", URL: "https://example.test/v1/videos",
		RequestHeadersJSON: []byte(`{"Content-Type":"application/json"}`), RequestBodyJSON: run.RequestJSON,
		ResponseStatus: &status, ResponseBodyJSON: []byte(`{"id":"task-1"}`), StartedAt: fixedNow, FinishedAt: &finished,
	})
	require.NoError(t, err)
	require.Equal(t, 1, exchange1.Sequence)
	require.Equal(t, int64(250), *exchange1.DurationMS)

	progress := 0.5
	nextPoll = fixedNow.Add(20 * time.Second)
	require.NoError(t, store.UpdateRunPoll(ctx, run.ID, PollUpdate{
		Status: RunPolling, Progress: &progress, ResultJSON: []byte(`{"progress":50}`),
		NextPollAt: &nextPoll, PolledAt: fixedNow.Add(10 * time.Second),
	}))
	completed := fixedNow.Add(30 * time.Second)
	progress = 1
	require.NoError(t, store.UpdateRunPoll(ctx, run.ID, PollUpdate{
		Status: RunSucceeded, Progress: &progress, ResultJSON: []byte(`{"video_url":"https://cdn.test/v.mp4"}`),
		PolledAt: completed, CompletedAt: &completed,
	}))
	unreconciled, err := store.ListUnreconciledRuns(ctx, 10)
	require.NoError(t, err)
	require.Len(t, unreconciled, 1)
	require.Equal(t, run.ID, unreconciled[0].ID)

	actual := decimal.RequireFromString("0.150")
	delta := actual.Sub(estimated)
	require.NoError(t, store.UpdateRunBilling(ctx, run.ID, BillingUpdate{
		EstimatedJSON: []byte(`{"amount":"0.125"}`), EstimatedAmount: &estimated,
		ActualJSON: []byte(`{"amount":"0.150"}`), ActualAmount: &actual,
		DeltaJSON: []byte(`{"amount":"0.025"}`), DeltaAmount: &delta, Currency: "USD",
	}))

	complete, err := store.GetRunWithExchanges(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, RunSucceeded, complete.Run.Status)
	require.Equal(t, 2, complete.Run.PollAttempts)
	require.Nil(t, complete.Run.NextPollAt)
	require.True(t, complete.Run.ActualAmount.Equal(actual))
	require.True(t, complete.Run.DeltaAmount.Equal(delta))
	require.Len(t, complete.Exchanges, 1)
	require.Equal(t, "submission", complete.Exchanges[0].Kind)
	unreconciled, err = store.ListUnreconciledRuns(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, unreconciled)

	found, err := store.FindRunByRequestID(ctx, "req-1")
	require.NoError(t, err)
	require.Equal(t, run.ID, found.ID)
	found, err = store.FindRunByUpstreamTaskID(ctx, "task-1")
	require.NoError(t, err)
	require.Equal(t, run.ID, found.ID)

	runs, err := store.ListRuns(ctx, RunFilter{UISessionID: &session.ID, Statuses: []RunStatus{RunSucceeded}})
	require.NoError(t, err)
	require.Len(t, runs, 1)
}

func TestRunRejectsInvalidJSONAndEnforcesForeignKeys(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	missing := "missing"
	_, err := store.CreateRun(ctx, CreateRunParams{
		EnvironmentID: &missing, EnvironmentName: "missing", BaseURL: "https://example.test",
		Provider: "grok", Model: "grok-imagine", Operation: "image", RequestJSON: []byte(`{"ok":true}`),
	})
	require.ErrorIs(t, err, ErrConflict)
	_, err = store.CreateRun(ctx, CreateRunParams{
		EnvironmentName: "env", BaseURL: "https://example.test", Provider: "grok", Model: "grok-imagine",
		Operation: "image", RequestJSON: []byte(`{bad`),
	})
	require.ErrorIs(t, err, ErrInvalidInput)

	_, err = store.GetRun(ctx, "missing")
	require.True(t, errors.Is(err, ErrNotFound))
}

func TestCanceledRunRemainsEligibleForBillingReconciliation(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	environment, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{
		Name: "env", BaseURL: "https://example.test", APIKey: "key",
	})
	require.NoError(t, err)
	run, err := store.CreateRun(ctx, CreateRunParams{
		EnvironmentID: &environment.ID, EnvironmentName: environment.Name, BaseURL: environment.BaseURL,
		Provider: "grok", Model: "grok-imagine-video", Operation: "grok.video.generate",
		RequestJSON: []byte(`{"model":"grok-imagine-video","prompt":"hello"}`),
	})
	require.NoError(t, err)
	require.NoError(t, store.MarkRunSubmitted(ctx, run.ID, SubmissionUpdate{
		UpstreamTaskID: "task-canceled", Status: RunSubmitted, SubmittedAt: fixedNow,
	}))
	require.NoError(t, store.MarkRunCanceled(ctx, run.ID, fixedNow.Add(time.Minute)))

	unreconciled, err := store.ListUnreconciledRuns(ctx, 10)
	require.NoError(t, err)
	require.Len(t, unreconciled, 1)
	require.Equal(t, RunCanceled, unreconciled[0].Status)
	require.Equal(t, "task-canceled", unreconciled[0].UpstreamTaskID)
}

func TestRunMediaSourcesAreEncryptedAndOrdered(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	environment, err := store.CreateEnvironment(ctx, CreateEnvironmentParams{
		Name: "media-env", BaseURL: "https://example.test", APIKey: "key",
	})
	require.NoError(t, err)
	run, err := store.CreateRun(ctx, CreateRunParams{
		EnvironmentID: &environment.ID, EnvironmentName: environment.Name, BaseURL: environment.BaseURL,
		Provider: "grok", Model: "grok-imagine-image", Operation: "grok.image.generate",
		RequestJSON: []byte(`{"model":"grok-imagine-image","prompt":"cat"}`),
	})
	require.NoError(t, err)
	sources := []string{
		"https://cdn.example.test/one.png?signature=secret-one",
		"https://cdn.example.test/two.png?token=secret-two",
	}
	require.NoError(t, store.ReplaceRunMediaSources(ctx, run.ID, environment.ID, sources))

	var records []runMediaSourceRecord
	require.NoError(t, store.db.Order("position ASC").Find(&records, "run_id = ?", run.ID).Error)
	require.Len(t, records, 2)
	for index, record := range records {
		require.NotContains(t, string(record.URLCiphertext), "secret-")
		require.Equal(t, index, record.Position)
		decrypted, err := store.GetRunMediaSource(ctx, run.ID, index)
		require.NoError(t, err)
		require.Equal(t, sources[index], decrypted.URL)
	}

	require.ErrorIs(t, store.ReplaceRunMediaSources(ctx, run.ID, environment.ID, []string{"file:///tmp/private"}), ErrInvalidInput)
}
