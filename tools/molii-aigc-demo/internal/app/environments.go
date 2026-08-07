package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"molii-aigc-demo/internal/store"
	"molii-aigc-demo/internal/upstream"
)

type createEnvironmentRequest struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type updateEnvironmentRequest struct {
	Name    *string `json:"name,omitempty"`
	BaseURL *string `json:"base_url,omitempty"`
	APIKey  *string `json:"api_key,omitempty"`
}

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	if _, _, err := s.ensureSession(w, r); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_session", "UI session is unavailable")
		return
	}
	environments, err := s.store.ListEnvironments(r.Context())
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": environments})
}

func (s *Server) handleCreateEnvironment(w http.ResponseWriter, r *http.Request, _ store.UISession) {
	var input createEnvironmentRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	environment, err := s.store.CreateEnvironment(r.Context(), store.CreateEnvironmentParams{
		Name: input.Name, BaseURL: input.BaseURL, APIKey: input.APIKey,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"environment": environment})
}

func (s *Server) handleUpdateEnvironment(w http.ResponseWriter, r *http.Request, _ store.UISession) {
	var input updateEnvironmentRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	environment, err := s.store.UpdateEnvironment(r.Context(), r.PathValue("id"), store.UpdateEnvironmentParams{
		Name: input.Name, BaseURL: input.BaseURL, APIKey: input.APIKey,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"environment": environment})
}

func (s *Server) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request, session store.UISession) {
	id := strings.TrimSpace(r.PathValue("id"))
	if err := s.store.DeleteEnvironment(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	if session.SelectedEnvironmentID != nil && *session.SelectedEnvironmentID == id {
		_ = s.store.SelectEnvironment(r.Context(), session.ID, nil)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSelectEnvironment(w http.ResponseWriter, r *http.Request, session store.UISession) {
	id := strings.TrimSpace(r.PathValue("id"))
	if _, err := s.store.GetEnvironment(r.Context(), id); err != nil {
		writeStoreError(w, err)
		return
	}
	if err := s.store.SelectEnvironment(r.Context(), session.ID, &id); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"selected_environment_id": id})
}

type connectivityResult struct {
	Name       string          `json:"name"`
	Path       string          `json:"path"`
	StatusCode int             `json:"status_code,omitempty"`
	DurationMS int64           `json:"duration_ms"`
	RequestID  string          `json:"request_id,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
	Error      string          `json:"error,omitempty"`
}

func (s *Server) handleTestEnvironment(w http.ResponseWriter, r *http.Request, _ store.UISession) {
	credentials, err := s.store.GetEnvironmentCredentials(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	tests := []struct {
		name string
		path string
	}{
		{name: "status", path: "/api/status"},
		{name: "models", path: "/v1/models"},
	}
	results := make([]connectivityResult, 0, len(tests))
	ok := true
	for _, test := range tests {
		result, requestErr := s.client.Do(r.Context(), credentials.BaseURL, credentials.APIKey, upstream.PreparedRequest{
			Operation: "environment.test." + test.name,
			Method:    http.MethodGet,
			Path:      test.path,
		})
		entry := connectivityResult{
			Name: test.name, Path: test.path, StatusCode: result.StatusCode,
			DurationMS: result.Duration.Milliseconds(), RequestID: result.RequestID,
		}
		if len(result.LogBody) != 0 && json.Valid(result.LogBody) {
			entry.Body = append(json.RawMessage(nil), result.LogBody...)
		}
		if requestErr != nil {
			entry.Error = requestErr.Error()
			ok = false
		}
		results = append(results, entry)
	}
	status := http.StatusOK
	if !ok {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]any{
		"ok": ok, "environment": store.EnvironmentCredentials{ID: credentials.ID, Name: credentials.Name, BaseURL: credentials.BaseURL},
		"checks": results, "message": "Only status and model-list endpoints were called; no paid generation request was sent.",
	})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Requested record was not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, store.ErrInvalidInput), errors.Is(err, store.ErrSecretRequired):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "database_error", "SQLite operation failed")
	}
}
