package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"molii-aigc-demo/internal/billing"
	"molii-aigc-demo/internal/catalog"
	"molii-aigc-demo/internal/store"
	"molii-aigc-demo/internal/upstream"
)

type runRequest struct {
	EnvironmentID string          `json:"environment_id"`
	Model         string          `json:"model"`
	Operation     string          `json:"operation"`
	Parameters    json.RawMessage `json:"parameters"`
}

type preparedRun struct {
	Environment store.EnvironmentCredentials
	Model       catalog.Model
	Request     upstream.PreparedRequest
	Estimate    billing.Estimate
	EstimateRaw []byte
	Curl        string
}

func (s *Server) prepareRun(ctx context.Context, session store.UISession, input runRequest) (preparedRun, error) {
	input.EnvironmentID = strings.TrimSpace(input.EnvironmentID)
	if input.EnvironmentID == "" && session.SelectedEnvironmentID != nil {
		input.EnvironmentID = *session.SelectedEnvironmentID
	}
	if input.EnvironmentID == "" {
		return preparedRun{}, errors.New("an environment must be selected")
	}
	credentials, err := s.store.GetEnvironmentCredentials(ctx, input.EnvironmentID)
	if err != nil {
		return preparedRun{}, err
	}
	model, ok := catalog.FindModel(strings.TrimSpace(input.Model))
	if !ok {
		return preparedRun{}, errors.New("unsupported model")
	}
	operation, ok := findOperation(model, strings.TrimSpace(input.Operation))
	if !ok {
		return preparedRun{}, errors.New("operation is not supported by the selected model")
	}
	parameters := input.Parameters
	if len(parameters) == 0 || string(parameters) == "null" {
		parameters = json.RawMessage(`{}`)
	}
	if !json.Valid(parameters) {
		return preparedRun{}, errors.New("parameters must be valid JSON")
	}
	var request upstream.PreparedRequest
	if operation.Method == http.MethodGet || operation.Method == http.MethodDelete {
		var resource struct {
			ID string `json:"id"`
		}
		if err := decodeRawStrict(parameters, &resource); err != nil {
			return preparedRun{}, fmt.Errorf("invalid resource parameters: %w", err)
		}
		request, err = upstream.BuildResource(operation.ID, resource.ID)
	} else {
		request, err = upstream.Build(operation.ID, parameters)
	}
	if err != nil {
		return preparedRun{}, err
	}
	if request.Generation {
		var payload map[string]any
		if json.Unmarshal(request.Body, &payload) == nil {
			if requestModel, _ := payload["model"].(string); requestModel != model.ID {
				return preparedRun{}, errors.New("parameters.model must match the selected model")
			}
		}
	}
	curl, err := upstream.CurlPreview(credentials.BaseURL, request)
	if err != nil {
		return preparedRun{}, err
	}
	estimate := billing.Estimate{Currency: "CNY", Reason: "this operation does not consume generation quota"}
	if request.Generation {
		priceCatalog, catalogErr := billing.FetchCatalog(ctx, s.client, credentials.BaseURL, credentials.APIKey)
		if catalogErr != nil {
			estimate.Reason = "pricing could not be loaded: " + catalogErr.Error()
		} else {
			estimate = priceCatalog.Estimate(estimateInput(model.ID, operation.ID, request.Body))
		}
	}
	estimateRaw, _ := json.Marshal(estimate)
	return preparedRun{
		Environment: credentials, Model: model, Request: request,
		Estimate: estimate, EstimateRaw: estimateRaw, Curl: curl,
	}, nil
}

func findOperation(model catalog.Model, id string) (catalog.Operation, bool) {
	for _, operation := range model.Operations {
		if operation.ID == id {
			return operation, true
		}
	}
	return catalog.Operation{}, false
}

func decodeRawStrict(raw []byte, target any) error {
	request, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	return decodeJSON(request, target)
}

func estimateInput(model, operation string, body []byte) billing.EstimateInput {
	input := billing.EstimateInput{Model: model, Operation: operation, OutputCount: 1, Group: "default"}
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return input
	}
	input.Resolution = stringValue(value["resolution"])
	input.Quality = stringValue(value["quality"])
	input.Ratio = stringValue(value["ratio"])
	if input.Ratio == "" {
		input.Ratio = stringValue(value["aspect_ratio"])
	}
	input.Duration = intValue(value["duration"])
	if count := intValue(value["n"]); count > 0 {
		input.OutputCount = count
	}
	if value["image"] != nil {
		input.InputImageCount++
	}
	if images, ok := value["images"].([]any); ok {
		input.InputImageCount += len(images)
	}
	if value["video"] != nil {
		input.HasVideoInput = true
	}
	if content, ok := value["content"].([]any); ok {
		for _, rawItem := range content {
			item, _ := rawItem.(map[string]any)
			switch stringValue(item["type"]) {
			case "image_url":
				input.InputImageCount++
			case "video_url":
				input.HasVideoInput = true
			}
		}
	}
	return input
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func intValue(value any) int {
	switch typed := value.(type) {
	case json.Number:
		result, _ := strconv.Atoi(typed.String())
		return result
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request, session store.UISession) {
	var input runRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	prepared, err := s.prepareRun(r.Context(), session, input)
	if err != nil {
		writePreparationError(w, err)
		return
	}
	requestJSON := json.RawMessage(prepared.Request.Body)
	if len(requestJSON) == 0 {
		requestJSON = json.RawMessage(`{}`)
	}
	response := map[string]any{
		"method": prepared.Request.Method, "path": prepared.Request.Path,
		"request_json": requestJSON, "curl": prepared.Curl,
		"estimate": prepared.Estimate, "currency": prepared.Estimate.Currency,
		"formula": prepared.Estimate.Formula,
	}
	if prepared.Estimate.Available {
		response["estimated_amount"] = prepared.Estimate.Amount
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request, session store.UISession) {
	var input runRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	prepared, err := s.prepareRun(r.Context(), session, input)
	if err != nil {
		writePreparationError(w, err)
		return
	}
	// Persist only the log-safe view. The prepared body itself remains
	// untouched and is still used for the actual upstream request and preview.
	requestJSON := upstream.RedactBody(prepared.Request.Body, prepared.Environment.APIKey)
	if len(requestJSON) == 0 {
		requestJSON = []byte(`{}`)
	}
	var estimatedAmount *decimal.Decimal
	if prepared.Estimate.Available {
		amount := prepared.Estimate.Amount
		estimatedAmount = &amount
	}
	environmentID := prepared.Environment.ID
	sessionID := session.ID
	run, err := s.store.CreateRun(r.Context(), store.CreateRunParams{
		UISessionID: &sessionID, EnvironmentID: &environmentID,
		EnvironmentName: prepared.Environment.Name, BaseURL: prepared.Environment.BaseURL,
		Provider: prepared.Model.Provider, Model: prepared.Model.ID, Operation: prepared.Request.Operation,
		RequestJSON: requestJSON, EstimatedBillingJSON: prepared.EstimateRaw,
		EstimatedAmount: estimatedAmount, Currency: "CNY",
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	result, requestErr := s.client.Do(r.Context(), prepared.Environment.BaseURL, prepared.Environment.APIKey, prepared.Request)
	// Once the paid request has been attempted, persist its outcome even if the
	// browser disconnects. This context is bounded and is never used to retry or
	// extend the upstream generation request itself.
	persistCtx, cancelPersist := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Second)
	defer cancelPersist()
	if appendErr := s.appendHTTPExchange(persistCtx, run.ID, "submit", prepared.Environment.BaseURL, prepared.Request, result, requestErr); appendErr != nil {
		s.logger.Error("save submit exchange", "run_id", run.ID, "error", appendErr)
	}
	now := time.Now().UTC()
	status := store.RunSucceeded
	taskID := ""
	errorCode, errorMessage := "", ""
	if requestErr != nil {
		status = store.RunFailed
		errorCode, errorMessage = "upstream_request_failed", requestErr.Error()
	} else if prepared.Request.Async {
		taskID = extractTaskID(result.Body)
		if taskID == "" {
			status = store.RunFailed
			errorCode, errorMessage = "missing_task_id", "upstream response did not contain a task ID"
		} else {
			status = store.RunSubmitted
		}
	}
	var nextPollAt *time.Time
	if status == store.RunSubmitted {
		value := now.Add(s.pollInterval)
		nextPollAt = &value
	}
	if status == store.RunSucceeded && strings.HasPrefix(prepared.Request.Operation, "grok.image.") {
		if sources := extractImageMediaURLs(result.Body); len(sources) != 0 {
			if mediaErr := s.store.ReplaceRunMediaSources(persistCtx, run.ID, prepared.Environment.ID, sources); mediaErr != nil {
				s.logger.Error("save encrypted image media", "run_id", run.ID, "error", mediaErr)
			}
		}
	}
	resultJSON := logJSON(result.LogBody)
	if len(resultJSON) == 0 {
		resultJSON = []byte(`{}`)
	}
	if err := s.store.MarkRunSubmitted(persistCtx, run.ID, store.SubmissionUpdate{
		RequestID: result.RequestID, UpstreamTaskID: taskID, Status: status,
		ResultJSON: resultJSON, NextPollAt: nextPollAt, SubmittedAt: now,
	}); err != nil {
		writeStoreError(w, err)
		return
	}
	if status == store.RunFailed {
		_ = s.store.UpdateRunPoll(persistCtx, run.ID, store.PollUpdate{
			Status: store.RunFailed, ResultJSON: resultJSON,
			ErrorCode: errorCode, ErrorMessage: errorMessage, PolledAt: now, CompletedAt: &now,
		})
	}
	detail, detailErr := s.store.GetRunWithExchanges(persistCtx, run.ID)
	if detailErr != nil {
		writeStoreError(w, detailErr)
		return
	}
	if requestErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": map[string]any{"code": errorCode, "message": errorMessage, "run_id": run.ID},
			"run":   detail.Run, "exchanges": detail.Exchanges,
		})
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

func writePreparationError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeStoreError(w, err)
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
}

func extractTaskID(body []byte) string {
	var value map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return ""
	}
	for _, candidate := range []map[string]any{value, mapValue(value["data"])} {
		for _, key := range []string{"task_id", "id"} {
			if id := stringValue(candidate[key]); id != "" {
				return id
			}
		}
	}
	return ""
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func extractImageMediaURLs(body []byte) []string {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, 4)
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if (key == "url" || key == "result_url" || key == "image_url") && isHTTPMediaURL(child) {
					source := strings.TrimSpace(child.(string))
					if _, ok := seen[source]; !ok {
						seen[source] = struct{}{}
						result = append(result, source)
					}
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return result
}

func isHTTPMediaURL(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "https://") || strings.HasPrefix(text, "http://")
}

func (s *Server) appendHTTPExchange(ctx context.Context, runID, kind, baseURL string, request upstream.PreparedRequest, result upstream.Result, requestErr error) error {
	finished := time.Now().UTC()
	started := finished.Add(-result.Duration)
	status := result.StatusCode
	duration := result.Duration.Milliseconds()
	requestHeaders, _ := json.Marshal(result.RequestLogHeader)
	responseHeaders, _ := json.Marshal(result.LogHeader)
	exchange := store.Exchange{
		RunID: runID, Kind: kind, Method: request.Method,
		URL:                strings.TrimRight(baseURL, "/") + request.Path,
		RequestHeadersJSON: requestHeaders, RequestBodyJSON: logJSON(result.RequestLogBody),
		ResponseHeadersJSON: responseHeaders, ResponseBodyJSON: logJSON(result.LogBody),
		StartedAt: started, FinishedAt: &finished, DurationMS: &duration,
	}
	if result.StatusCode != 0 {
		exchange.ResponseStatus = &status
	}
	if requestErr != nil {
		exchange.Error = requestErr.Error()
	}
	returnError := error(nil)
	_, returnError = s.store.AppendExchange(ctx, exchange)
	return returnError
}

func logJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	if json.Valid(raw) {
		return append([]byte(nil), raw...)
	}
	wrapped, _ := json.Marshal(map[string]string{"text": string(raw)})
	return wrapped
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	session, _, err := s.ensureSession(w, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_session", "UI session is unavailable")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	filter := store.RunFilter{UISessionID: &session.ID, Limit: limit}
	if environmentID := strings.TrimSpace(r.URL.Query().Get("environment_id")); environmentID != "" {
		filter.EnvironmentID = &environmentID
	}
	runs, err := s.store.ListRuns(r.Context(), filter)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	session, _, err := s.ensureSession(w, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_session", "UI session is unavailable")
		return
	}
	detail, err := s.store.GetRunWithExchanges(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if detail.Run.UISessionID == nil || *detail.Run.UISessionID != session.ID {
		writeError(w, http.StatusNotFound, "not_found", "Run was not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request, session store.UISession) {
	run, err := s.store.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if run.UISessionID == nil || *run.UISessionID != session.ID {
		writeError(w, http.StatusNotFound, "not_found", "Run was not found")
		return
	}
	if !run.Status.Terminal() {
		if err := s.engine.Cancel(r.Context(), run.ID); err != nil {
			writeStoreError(w, err)
			return
		}
	}
	updated, err := s.store.GetRun(r.Context(), run.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRunMedia(w http.ResponseWriter, r *http.Request) {
	session, _, err := s.ensureSession(w, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_session", "UI session is unavailable")
		return
	}
	run, err := s.store.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if run.UISessionID == nil || *run.UISessionID != session.ID {
		writeError(w, http.StatusNotFound, "not_found", "Run was not found")
		return
	}
	if run.Status != store.RunSucceeded {
		writeError(w, http.StatusConflict, "media_unavailable", "Media content is not available for this run")
		return
	}
	if strings.HasPrefix(run.Operation, "grok.image.") {
		position, parseErr := strconv.Atoi(r.URL.Query().Get("index"))
		if r.URL.Query().Get("index") == "" {
			position = 0
		} else if parseErr != nil || position < 0 {
			writeError(w, http.StatusBadRequest, "invalid_media_index", "Media index must be a non-negative integer")
			return
		}
		source, sourceErr := s.store.GetRunMediaSource(r.Context(), run.ID, position)
		if sourceErr != nil {
			writeStoreError(w, sourceErr)
			return
		}
		// Redirecting the user's browser avoids server-side fetching of an
		// upstream-controlled URL (and therefore avoids SSRF). The signed URL is
		// encrypted at rest and never included in logs or run JSON.
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		http.Redirect(w, r, source.URL, http.StatusTemporaryRedirect)
		return
	}
	if run.UpstreamTaskID == "" || run.EnvironmentID == nil {
		writeError(w, http.StatusConflict, "media_unavailable", "Video content is not available for this run")
		return
	}
	credentials, err := s.store.GetEnvironmentCredentials(r.Context(), *run.EnvironmentID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	request, err := upstream.BuildResource("video.content", run.UpstreamTaskID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_task_id", err.Error())
		return
	}
	response, err := s.client.Stream(r.Context(), credentials.BaseURL, credentials.APIKey, request, r.Header)
	if err != nil {
		writeError(w, http.StatusBadGateway, "media_fetch_failed", err.Error())
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified", "Content-Disposition"} {
		if value := response.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	if r.URL.Query().Get("download") == "1" && w.Header().Get("Content-Disposition") == "" {
		w.Header().Set("Content-Disposition", `attachment; filename="molii-result.mp4"`)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}
