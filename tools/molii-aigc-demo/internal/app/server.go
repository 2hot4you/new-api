package app

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	demoassets "molii-aigc-demo"
	"molii-aigc-demo/internal/catalog"
	"molii-aigc-demo/internal/jobs"
	"molii-aigc-demo/internal/store"
	"molii-aigc-demo/internal/upstream"
)

type Config struct {
	Store             *store.Store
	Client            *upstream.Client
	SessionSecret     []byte
	SessionTTL        time.Duration
	CookieSecure      bool
	AllowedHosts      []string
	PollInterval      time.Duration
	PollMaxAttempts   int
	BillingSyncPeriod time.Duration
	Logger            *slog.Logger
}

type Server struct {
	store             *store.Store
	client            *upstream.Client
	sessionSecret     []byte
	sessionTTL        time.Duration
	cookieSecure      bool
	allowedHosts      map[string]struct{}
	pollInterval      time.Duration
	billingSyncPeriod time.Duration
	logger            *slog.Logger
	instanceID        string
	handler           http.Handler
	engine            *jobs.Engine
}

func New(config Config) (*Server, error) {
	if config.Store == nil || config.Client == nil {
		return nil, errors.New("store and upstream client are required")
	}
	if len(config.SessionSecret) < 32 {
		return nil, errors.New("session secret must contain at least 32 bytes")
	}
	if config.SessionTTL <= 0 {
		config.SessionTTL = 24 * time.Hour
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 4 * time.Second
	}
	if config.BillingSyncPeriod <= 0 {
		config.BillingSyncPeriod = 5 * time.Second
	}
	if config.PollMaxAttempts <= 0 {
		config.PollMaxAttempts = 60
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	server := &Server{
		store: config.Store, client: config.Client,
		sessionSecret: append([]byte(nil), config.SessionSecret...),
		sessionTTL:    config.SessionTTL, cookieSecure: config.CookieSecure,
		allowedHosts: make(map[string]struct{}), pollInterval: config.PollInterval,
		billingSyncPeriod: config.BillingSyncPeriod,
		logger:            config.Logger,
		instanceID:        uuid.NewString(),
	}
	for _, host := range config.AllowedHosts {
		if host = strings.ToLower(strings.TrimSpace(host)); host != "" {
			server.allowedHosts[host] = struct{}{}
		}
	}
	server.allowedHosts["127.0.0.1"] = struct{}{}
	server.allowedHosts["localhost"] = struct{}{}
	server.allowedHosts["[::1]"] = struct{}{}

	repository := &pollRepository{server: server}
	poller := &videoPoller{server: server}
	engine, err := jobs.New(repository, poller, jobs.Config{
		ScanInterval: time.Second,
		InitialDelay: config.PollInterval,
		MaxDelay:     30 * time.Second,
		MaxAttempts:  config.PollMaxAttempts,
		BatchSize:    32,
		PollTimeout:  20 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	server.engine = engine
	server.handler = server.routes()
	return server, nil
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) Start(ctx context.Context) {
	if err := s.recoverPendingRuns(ctx); err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Error("recover interrupted submissions", "error", err)
	}
	go func() {
		if err := s.engine.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Error("polling engine stopped", "error", err)
		}
	}()
	go s.runBillingSync(ctx)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("GET /api/bootstrap", s.handleBootstrap)
	mux.HandleFunc("GET /api/environments", s.handleListEnvironments)
	mux.HandleFunc("POST /api/environments", s.withWriteSession(s.handleCreateEnvironment))
	mux.HandleFunc("PUT /api/environments/{id}", s.withWriteSession(s.handleUpdateEnvironment))
	mux.HandleFunc("DELETE /api/environments/{id}", s.withWriteSession(s.handleDeleteEnvironment))
	mux.HandleFunc("POST /api/environments/{id}/select", s.withWriteSession(s.handleSelectEnvironment))
	mux.HandleFunc("POST /api/environments/{id}/test", s.withWriteSession(s.handleTestEnvironment))
	mux.HandleFunc("POST /api/preview", s.withWriteSession(s.handlePreview))
	mux.HandleFunc("POST /api/runs", s.withWriteSession(s.handleCreateRun))
	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("POST /api/runs/{id}/cancel", s.withWriteSession(s.handleCancelRun))
	mux.HandleFunc("GET /api/runs/{id}/media", s.handleRunMedia)

	staticFS, err := demoassets.FS()
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", noStoreFiles(http.FileServer(http.FS(staticFS)))))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		content, readErr := fs.ReadFile(staticFS, "index.html")
		if readErr != nil {
			http.Error(w, "UI unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	})
	return s.securityHeaders(mux)
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"instance_id": s.instanceID})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "SQLite database is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	session, csrf, err := s.ensureSession(w, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_failed", err.Error())
		return
	}
	environments, err := s.store.ListEnvironments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}
	runs, err := s.store.ListRuns(r.Context(), store.RunFilter{UISessionID: &session.ID, Limit: 100})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"csrf_token": csrf, "selected_environment_id": session.SelectedEnvironmentID,
		"environments": environments, "models": catalog.Models(), "runs": runs,
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob: https: http:; media-src 'self' blob: https: http:; style-src 'self'; script-src 'self'; connect-src 'self'")
		if !s.hostAllowed(r.Host) {
			writeError(w, http.StatusForbidden, "invalid_host", "Host is not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func noStoreFiles(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
