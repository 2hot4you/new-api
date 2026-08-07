package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	demoassets "molii-aigc-demo"
	"molii-aigc-demo/internal/secure"
	"molii-aigc-demo/internal/store"
	"molii-aigc-demo/internal/upstream"
)

type demoFixture struct {
	server *Server
	store  *store.Store
	cookie *http.Cookie
	csrf   string
}

func newDemoFixture(t *testing.T, upstreamServer *httptest.Server) *demoFixture {
	t.Helper()
	encodedKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, secure.MasterKeySize))
	keyring, err := secure.NewKeyring(encodedKey, 1)
	require.NoError(t, err)
	database, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "demo.db"), keyring, store.DefaultOptions())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	server, err := New(Config{
		Store: database, Client: upstream.NewClient(3*time.Second, upstream.DefaultResponseCap, nil),
		SessionSecret: bytes.Repeat([]byte{3}, 32), SessionTTL: time.Hour,
		PollInterval: 5 * time.Millisecond, PollMaxAttempts: 10,
		BillingSyncPeriod: time.Hour,
	})
	require.NoError(t, err)
	fixture := &demoFixture{server: server, store: database}
	bootstrap := fixture.request(t, http.MethodGet, "/api/bootstrap", nil, false)
	require.Equal(t, http.StatusOK, bootstrap.Code)
	fixture.cookie = bootstrap.Result().Cookies()[0]
	var payload map[string]any
	require.NoError(t, json.Unmarshal(bootstrap.Body.Bytes(), &payload))
	fixture.csrf, _ = payload["csrf_token"].(string)
	require.NotEmpty(t, fixture.csrf)
	if upstreamServer != nil {
		response := fixture.request(t, http.MethodPost, "/api/environments", map[string]any{
			"name": "local", "base_url": upstreamServer.URL, "api_key": "sk-demo-secret",
		}, true)
		require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
		var created struct {
			Environment store.Environment `json:"environment"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &created))
		selectResponse := fixture.request(t, http.MethodPost, "/api/environments/"+created.Environment.ID+"/select", map[string]any{}, true)
		require.Equal(t, http.StatusOK, selectResponse.Code)
	}
	return fixture
}

func (f *demoFixture) request(t *testing.T, method, path string, body any, write bool) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, "http://127.0.0.1"+path, reader)
	request.Host = "127.0.0.1"
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if f.cookie != nil {
		request.AddCookie(f.cookie)
	}
	if write {
		request.Header.Set("X-CSRF-Token", f.csrf)
	}
	response := httptest.NewRecorder()
	f.server.Handler().ServeHTTP(response, request)
	return response
}

func TestBootstrapRejectsUntrustedHostAndWriteRequiresCSRF(t *testing.T) {
	fixture := newDemoFixture(t, nil)
	blocked := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://evil.invalid/api/bootstrap", nil)
	request.Host = "evil.invalid"
	fixture.server.Handler().ServeHTTP(blocked, request)
	require.Equal(t, http.StatusForbidden, blocked.Code)

	withoutCSRF := fixture.request(t, http.MethodPost, "/api/environments", map[string]any{
		"name": "local", "base_url": "http://127.0.0.1:3000", "api_key": "sk-secret",
	}, false)
	require.Equal(t, http.StatusForbidden, withoutCSRF.Code)
}

func TestPreviewAndSynchronousRunPersistRedactedTimelineAndBilling(t *testing.T) {
	var logCalls atomic.Int32
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-demo-secret", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/pricing":
			writeTestJSON(w, map[string]any{
				"success": true,
				"data": []any{map[string]any{"model_name": "grok-imagine-image", "molii_grok_pricing": map[string]any{
					"kind": "image", "output_unit": "image", "output_prices": map[string]any{"1k": 0.02, "2k": 0.02},
					"image_input_unit": "image", "image_input_price": 0.002,
				}}},
				"group_ratio": map[string]any{"default": 1}, "pricing_version": "test-v1",
			})
		case "/v1/images/generations":
			var requestBody map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
			require.Contains(t, requestBody["prompt"], "signature=signed-secret", "upstream request must retain the original signed URL")
			w.Header().Set("X-Oneapi-Request-Id", "req_image_1")
			writeTestJSON(w, map[string]any{"data": []any{map[string]any{"url": "https://media.invalid/image.png?signature=result-secret"}}})
		case "/api/log/token":
			logCalls.Add(1)
			writeTestJSON(w, map[string]any{"success": true, "data": []any{map[string]any{
				"created_at": 1, "type": 2, "model_name": "grok-imagine-image", "quota": 10000,
				"request_id": "req_image_1", "other": map[string]any{},
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstreamServer.Close()
	fixture := newDemoFixture(t, upstreamServer)
	environmentID := selectedEnvironmentID(t, fixture)
	descriptor := map[string]any{
		"environment_id": environmentID, "model": "grok-imagine-image", "operation": "grok.image.generate",
		"parameters": map[string]any{
			"model":        "grok-imagine-image",
			"prompt":       "orange cat https://media.invalid/input.png?signature=signed-secret",
			"aspect_ratio": "16:9", "resolution": "1k", "n": 1,
		},
	}
	preview := fixture.request(t, http.MethodPost, "/api/preview", descriptor, true)
	require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())
	require.Contains(t, preview.Body.String(), "$MOLII_API_KEY")
	require.NotContains(t, preview.Body.String(), "sk-demo-secret")
	require.Contains(t, preview.Body.String(), `"estimated_amount":"0.02"`)

	created := fixture.request(t, http.MethodPost, "/api/runs", descriptor, true)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var detail store.RunWithExchanges
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &detail))
	require.Equal(t, store.RunSucceeded, detail.Run.Status)
	require.Equal(t, "req_image_1", detail.Run.RequestID)
	require.NotContains(t, string(detail.Run.RequestJSON), "sk-demo-secret")
	require.NotContains(t, string(detail.Run.RequestJSON), "signed-secret")
	require.Contains(t, string(detail.Run.RequestJSON), "REDACTED")
	require.Len(t, detail.Exchanges, 1)
	require.NotContains(t, string(detail.Exchanges[0].RequestHeadersJSON), "sk-demo-secret")
	require.Contains(t, string(detail.Exchanges[0].RequestHeadersJSON), "REDACTED")

	persisted, err := fixture.store.GetRun(context.Background(), detail.Run.ID)
	require.NoError(t, err)
	require.NotContains(t, string(persisted.RequestJSON), "sk-demo-secret")
	require.NotContains(t, string(persisted.RequestJSON), "signed-secret")
	require.NotContains(t, string(persisted.ResultJSON), "result-secret")

	mediaSource, err := fixture.store.GetRunMediaSource(context.Background(), detail.Run.ID, 0)
	require.NoError(t, err)
	require.Equal(t, "https://media.invalid/image.png?signature=result-secret", mediaSource.URL)
	media := fixture.request(t, http.MethodGet, "/api/runs/"+detail.Run.ID+"/media?index=0", nil, false)
	require.Equal(t, http.StatusTemporaryRedirect, media.Code)
	require.Equal(t, mediaSource.URL, media.Header().Get("Location"))
	require.Equal(t, "private, no-store", media.Header().Get("Cache-Control"))

	fixture.server.syncBilling(context.Background())
	settled, err := fixture.store.GetRun(context.Background(), detail.Run.ID)
	require.NoError(t, err)
	require.NotNil(t, settled.ActualAmount)
	require.Equal(t, "0.02", settled.ActualAmount.String())
	require.NotNil(t, settled.DeltaAmount)
	require.True(t, settled.DeltaAmount.IsZero())
	require.EqualValues(t, 1, logCalls.Load())
}

func TestAsyncVideoRunPollsDurablyAndStoresEveryExchange(t *testing.T) {
	var polls atomic.Int32
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pricing":
			writeTestJSON(w, map[string]any{
				"success": true,
				"data": []any{map[string]any{"model_name": "doubao-seedance-2-0-260128", "video_pricing": map[string]any{
					"unit": "per_million_tokens", "fps": 24, "extra_frames": 1,
					"rows": []any{map[string]any{"resolutions": []string{"720p"}, "without_video": 0.5, "with_video": 0.8}},
				}}}, "group_ratio": map[string]any{"default": 1},
			})
		case "/v1/video/generations":
			w.Header().Set("X-Oneapi-Request-Id", "req_video_1")
			writeTestJSON(w, map[string]any{"id": "task_video_1", "status": "queued"})
		case "/v1/videos/task_video_1":
			polls.Add(1)
			writeTestJSON(w, map[string]any{"id": "task_video_1", "status": "completed", "progress": 100, "result_url": "https://media.invalid/video.mp4"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstreamServer.Close()
	fixture := newDemoFixture(t, upstreamServer)
	environmentID := selectedEnvironmentID(t, fixture)
	created := fixture.request(t, http.MethodPost, "/api/runs", map[string]any{
		"environment_id": environmentID, "model": "doubao-seedance-2-0-260128", "operation": "seedance.video.generate",
		"parameters": map[string]any{
			"model": "doubao-seedance-2-0-260128", "prompt": "ocean at sunrise", "generate_audio": true,
			"resolution": "720p", "ratio": "16:9", "duration": 5, "watermark": false,
		},
	}, true)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var detail store.RunWithExchanges
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &detail))
	require.Equal(t, store.RunSubmitted, detail.Run.Status)
	require.Equal(t, "task_video_1", detail.Run.UpstreamTaskID)

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, fixture.server.engine.RunOnce(context.Background()))
	completed, err := fixture.store.GetRunWithExchanges(context.Background(), detail.Run.ID)
	require.NoError(t, err)
	require.Equal(t, store.RunSucceeded, completed.Run.Status)
	require.NotNil(t, completed.Run.Progress)
	require.Equal(t, float64(1), *completed.Run.Progress)
	require.Len(t, completed.Exchanges, 2)
	require.Equal(t, "poll", completed.Exchanges[1].Kind)
	require.EqualValues(t, 1, polls.Load())
}

func TestRunFinalizationSurvivesClientCancellation(t *testing.T) {
	cancelRequest := func() {}
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pricing":
			writeTestJSON(w, map[string]any{
				"success": true,
				"data": []any{map[string]any{"model_name": "grok-imagine-image", "molii_grok_pricing": map[string]any{
					"kind": "image", "output_prices": map[string]any{"1k": 0.02},
				}}},
				"group_ratio": map[string]any{"default": 1},
			})
		case "/v1/images/generations":
			w.Header().Set("X-Oneapi-Request-Id", "req_canceled_client")
			writeTestJSON(w, map[string]any{"data": []any{map[string]any{"url": "https://media.invalid/result.png"}}})
			cancelRequest()
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstreamServer.Close()
	fixture := newDemoFixture(t, upstreamServer)
	descriptor := map[string]any{
		"environment_id": selectedEnvironmentID(t, fixture),
		"model":          "grok-imagine-image",
		"operation":      "grok.image.generate",
		"parameters": map[string]any{
			"model": "grok-imagine-image", "prompt": "cat", "resolution": "1k",
		},
	}
	body, err := json.Marshal(descriptor)
	require.NoError(t, err)
	requestContext, cancel := context.WithCancel(context.Background())
	cancelRequest = cancel
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/runs", bytes.NewReader(body)).WithContext(requestContext)
	request.Host = "127.0.0.1"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", fixture.csrf)
	request.AddCookie(fixture.cookie)
	response := httptest.NewRecorder()
	fixture.server.Handler().ServeHTTP(response, request)

	runs, err := fixture.store.ListRuns(context.Background(), store.RunFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.NotEqual(t, store.RunPending, runs[0].Status)
	exchanges, err := fixture.store.ListExchanges(context.Background(), runs[0].ID)
	require.NoError(t, err)
	require.Len(t, exchanges, 1)
}

func TestPendingSubmissionRecoveryNeverReplaysPaidPost(t *testing.T) {
	fixture := newDemoFixture(t, nil)
	ctx := context.Background()
	asyncRun, err := fixture.store.CreateRun(ctx, store.CreateRunParams{
		EnvironmentName: "recovery", BaseURL: "https://example.test", Provider: "seedance",
		Model: "doubao-seedance-2-0-260128", Operation: "seedance.video.generate",
		RequestJSON: []byte(`{"model":"doubao-seedance-2-0-260128","prompt":"cat"}`),
	})
	require.NoError(t, err)
	status := http.StatusOK
	finished := time.Now().UTC()
	_, err = fixture.store.AppendExchange(ctx, store.Exchange{
		RunID: asyncRun.ID, Kind: "submit", Method: http.MethodPost,
		URL:                 "https://example.test/v1/video/generations",
		ResponseStatus:      &status,
		ResponseHeadersJSON: []byte(`{"X-Oneapi-Request-Id":["req-recovered"]}`),
		ResponseBodyJSON:    []byte(`{"id":"task-recovered","status":"queued"}`),
		StartedAt:           finished.Add(-time.Second), FinishedAt: &finished,
	})
	require.NoError(t, err)
	interrupted, err := fixture.store.CreateRun(ctx, store.CreateRunParams{
		EnvironmentName: "recovery", BaseURL: "https://example.test", Provider: "grok",
		Model: "grok-imagine-image", Operation: "grok.image.generate",
		RequestJSON: []byte(`{"model":"grok-imagine-image","prompt":"cat"}`),
	})
	require.NoError(t, err)

	require.NoError(t, fixture.server.recoverPendingRuns(ctx))
	recovered, err := fixture.store.GetRun(ctx, asyncRun.ID)
	require.NoError(t, err)
	require.Equal(t, store.RunSubmitted, recovered.Status)
	require.Equal(t, "task-recovered", recovered.UpstreamTaskID)
	require.Equal(t, "req-recovered", recovered.RequestID)
	require.NotNil(t, recovered.NextPollAt)

	failed, err := fixture.store.GetRun(ctx, interrupted.ID)
	require.NoError(t, err)
	require.Equal(t, store.RunFailed, failed.Status)
	require.Equal(t, "submission_interrupted", failed.ErrorCode)
	require.Contains(t, failed.ErrorMessage, "not replayed")
}

func selectedEnvironmentID(t *testing.T, fixture *demoFixture) string {
	t.Helper()
	environments, err := fixture.store.ListEnvironments(context.Background())
	require.NoError(t, err)
	require.Len(t, environments, 1)
	return environments[0].ID
}

func writeTestJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func TestStaticUIContainsNoBrowserPersistenceAPIs(t *testing.T) {
	content, err := demoassets.FS()
	require.NoError(t, err)
	appJS, err := fs.ReadFile(content, "app.js")
	require.NoError(t, err)
	for _, forbidden := range []string{"localStorage", "sessionStorage", "indexedDB", "serviceWorker", "caches.open"} {
		require.NotContains(t, string(appJS), forbidden)
	}
	require.False(t, strings.Contains(strings.ToLower(string(appJS)), "authorization: bearer sk-"))
}
