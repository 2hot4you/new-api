package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"molii-aigc-demo/internal/billing"
	"molii-aigc-demo/internal/jobs"
	"molii-aigc-demo/internal/store"
	"molii-aigc-demo/internal/upstream"
)

type pollRepository struct {
	server *Server
}

func (r *pollRepository) ListRunnable(ctx context.Context, due time.Time, limit int) ([]jobs.Job, error) {
	runs, err := r.server.store.ListRecoverableRuns(ctx, due, limit)
	if err != nil {
		return nil, err
	}
	result := make([]jobs.Job, 0, len(runs))
	for _, run := range runs {
		environmentID := ""
		if run.EnvironmentID != nil {
			environmentID = *run.EnvironmentID
		}
		nextPollAt := due
		if run.NextPollAt != nil {
			nextPollAt = *run.NextPollAt
		}
		result = append(result, jobs.Job{
			ID: run.ID, EnvironmentID: environmentID, TaskID: run.UpstreamTaskID,
			Operation: run.Operation, Attempt: run.PollAttempts,
			CreatedAt: run.CreatedAt, NextPollAt: nextPollAt,
		})
	}
	return result, nil
}

func (r *pollRepository) SavePoll(ctx context.Context, update jobs.Update) error {
	status := store.RunPolling
	if update.Result.Terminal {
		if update.Result.Success {
			status = store.RunSucceeded
		} else {
			status = store.RunFailed
		}
	}
	if update.Exhausted {
		status = store.RunFailed
	}
	var progress *float64
	if update.Result.Progress > 0 || status == store.RunSucceeded {
		value := float64(update.Result.Progress) / 100
		if status == store.RunSucceeded {
			value = 1
		}
		if value < 0 {
			value = 0
		}
		if value > 1 {
			value = 1
		}
		progress = &value
	}
	errorMessage := strings.TrimSpace(update.Result.Error)
	if errorMessage == "" {
		errorMessage = strings.TrimSpace(update.PollError)
	}
	if update.Exhausted && errorMessage == "" {
		errorMessage = "polling attempt limit reached"
	}
	var completedAt *time.Time
	if status.Terminal() {
		value := update.PolledAt.UTC()
		completedAt = &value
	}
	var nextPollAt *time.Time
	if !status.Terminal() && !update.NextPollAt.IsZero() {
		value := update.NextPollAt.UTC()
		nextPollAt = &value
	}
	return r.server.store.UpdateRunPoll(ctx, update.JobID, store.PollUpdate{
		Status: status, Progress: progress, ResultJSON: logJSON(update.Result.Raw),
		ErrorCode: errorCodeForPoll(update, status), ErrorMessage: errorMessage,
		NextPollAt: nextPollAt, PolledAt: update.PolledAt, CompletedAt: completedAt,
	})
}

func errorCodeForPoll(update jobs.Update, status store.RunStatus) string {
	if status != store.RunFailed {
		return ""
	}
	if update.Exhausted {
		return "polling_exhausted"
	}
	if update.PollError != "" {
		return "poll_request_failed"
	}
	return "generation_failed"
}

func (r *pollRepository) Cancel(ctx context.Context, jobID string) error {
	if err := r.server.store.MarkRunCancelRequested(ctx, jobID); err != nil {
		return err
	}
	return r.server.store.MarkRunCanceled(ctx, jobID, time.Now().UTC())
}

type videoPoller struct {
	server *Server
}

func (p *videoPoller) Poll(ctx context.Context, job jobs.Job) (jobs.PollResult, error) {
	if job.EnvironmentID == "" {
		return jobs.PollResult{}, errors.New("environment was deleted before polling completed")
	}
	credentials, err := p.server.store.GetEnvironmentCredentials(ctx, job.EnvironmentID)
	if err != nil {
		return jobs.PollResult{}, err
	}
	request, err := upstream.BuildResource("video.get", job.TaskID)
	if err != nil {
		return jobs.PollResult{}, err
	}
	result, requestErr := p.server.client.Do(ctx, credentials.BaseURL, credentials.APIKey, request)
	if appendErr := p.server.appendHTTPExchange(ctx, job.ID, "poll", credentials.BaseURL, request, result, requestErr); appendErr != nil {
		p.server.logger.Error("save poll exchange", "run_id", job.ID, "error", appendErr)
	}
	if requestErr != nil {
		return jobs.PollResult{Raw: logJSON(result.LogBody)}, requestErr
	}
	parsed, err := upstream.ParsePollResponse(result.Body)
	if err != nil {
		return jobs.PollResult{Raw: logJSON(result.LogBody)}, err
	}
	return jobs.PollResult{
		Status: parsed.Status, Progress: parsed.Progress, ResultURL: parsed.ResultURL,
		Error: parsed.Error, Terminal: parsed.Terminal, Success: parsed.Success,
		Raw: logJSON(result.LogBody),
	}, nil
}

func (s *Server) runBillingSync(ctx context.Context) {
	s.syncBilling(ctx)
	ticker := time.NewTicker(s.billingSyncPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncBilling(ctx)
		}
	}
}

// recoverPendingRuns closes the only durable crash window around a paid POST.
// Generation POSTs are never replayed: a saved successful submit exchange is
// promoted to submitted/succeeded, while a pending row without a durable
// response is marked interrupted for manual inspection.
func (s *Server) recoverPendingRuns(ctx context.Context) error {
	for offset := 0; ; offset += 500 {
		runs, err := s.store.ListRuns(ctx, store.RunFilter{Statuses: []store.RunStatus{store.RunPending}, Limit: 500, Offset: offset})
		if err != nil {
			return err
		}
		for _, run := range runs {
			exchanges, exchangeErr := s.store.ListExchanges(ctx, run.ID)
			if exchangeErr != nil {
				return exchangeErr
			}
			var submit *store.Exchange
			for index := range exchanges {
				if exchanges[index].Kind == "submit" {
					submit = &exchanges[index]
				}
			}
			if err := s.recoverPendingRun(ctx, run, submit); err != nil {
				return err
			}
		}
		if len(runs) < 500 {
			return nil
		}
		// Every processed row leaves pending status, so restart from offset zero.
		offset = -500
	}
}

func (s *Server) recoverPendingRun(ctx context.Context, run store.Run, submit *store.Exchange) error {
	now := time.Now().UTC()
	resultJSON := []byte(`{"error":"Demo process stopped before the upstream response was durably recorded; the paid request was not replayed"}`)
	status := store.RunFailed
	errorCode := "submission_interrupted"
	errorMessage := "Demo process stopped before the upstream response was durably recorded; request was not replayed"
	requestID, taskID := "", ""
	var nextPollAt *time.Time
	submittedAt := run.CreatedAt
	if submit != nil {
		if len(submit.ResponseBodyJSON) != 0 {
			resultJSON = submit.ResponseBodyJSON
		}
		if submit.FinishedAt != nil {
			submittedAt = *submit.FinishedAt
		} else {
			submittedAt = submit.StartedAt
		}
		requestID = requestIDFromExchange(*submit)
		success := submit.Error == "" && submit.ResponseStatus != nil && *submit.ResponseStatus >= 200 && *submit.ResponseStatus < 300
		if success {
			if isAsyncOperation(run.Operation) {
				taskID = extractTaskID(submit.ResponseBodyJSON)
				if taskID != "" {
					status, errorCode, errorMessage = store.RunSubmitted, "", ""
					value := now.Add(s.pollInterval)
					nextPollAt = &value
				} else {
					errorCode, errorMessage = "missing_task_id", "saved upstream response did not contain a task ID"
				}
			} else {
				status, errorCode, errorMessage = store.RunSucceeded, "", ""
			}
		} else {
			errorCode, errorMessage = "upstream_request_failed", strings.TrimSpace(submit.Error)
			if errorMessage == "" {
				errorMessage = "saved upstream response reported a failed request"
			}
		}
	}
	if err := s.store.MarkRunSubmitted(ctx, run.ID, store.SubmissionUpdate{
		RequestID: requestID, UpstreamTaskID: taskID, Status: status,
		ResultJSON: resultJSON, NextPollAt: nextPollAt, SubmittedAt: submittedAt,
	}); err != nil {
		return err
	}
	if status == store.RunFailed {
		return s.store.UpdateRunPoll(ctx, run.ID, store.PollUpdate{
			Status: store.RunFailed, ResultJSON: resultJSON,
			ErrorCode: errorCode, ErrorMessage: errorMessage,
			PolledAt: now, CompletedAt: &now,
		})
	}
	return nil
}

func isAsyncOperation(operation string) bool {
	return operation == "seedance.video.generate" || strings.HasPrefix(operation, "grok.video.")
}

func requestIDFromExchange(exchange store.Exchange) string {
	var headers map[string][]string
	if json.Unmarshal(exchange.ResponseHeadersJSON, &headers) != nil {
		return ""
	}
	for key, values := range headers {
		if strings.EqualFold(key, "X-Oneapi-Request-Id") || strings.EqualFold(key, "X-Request-Id") || strings.EqualFold(key, "Request-Id") {
			if len(values) != 0 {
				return strings.TrimSpace(values[0])
			}
		}
	}
	return ""
}

func (s *Server) syncBilling(ctx context.Context) {
	runs, err := s.store.ListUnreconciledRuns(ctx, 64)
	if err != nil {
		s.logger.Error("list unsettled runs", "error", err)
		return
	}
	for _, run := range runs {
		if ctx.Err() != nil {
			return
		}
		if !isGenerationOperation(run.Operation) {
			zero := billing.Actual{Found: true, Amount: billingZero(), Currency: "CNY", Reason: "non-generation API call has no generation charge"}
			s.persistActual(ctx, run, zero)
			continue
		}
		if run.EnvironmentID == nil {
			continue
		}
		credentials, credentialErr := s.store.GetEnvironmentCredentials(ctx, *run.EnvironmentID)
		if credentialErr != nil {
			continue
		}
		ref := billing.Reference{Kind: "video", TaskID: run.UpstreamTaskID}
		if strings.HasPrefix(run.Operation, "grok.image.") {
			ref = billing.Reference{Kind: "image", RequestID: run.RequestID}
		}
		if (ref.Kind == "image" && ref.RequestID == "") || (ref.Kind == "video" && ref.TaskID == "") {
			continue
		}
		actual, actualErr := billing.FetchActual(ctx, s.client, credentials.BaseURL, credentials.APIKey, ref)
		if actualErr != nil {
			s.logger.Debug("billing not available", "run_id", run.ID, "error", actualErr)
			continue
		}
		if !actual.Found || actual.Pending {
			continue
		}
		s.persistActual(ctx, run, actual)
	}
}

func billingZero() decimal.Decimal {
	return decimal.Zero
}

func isGenerationOperation(operation string) bool {
	return operation == "seedance.video.generate" || strings.HasPrefix(operation, "grok.image.") || strings.HasPrefix(operation, "grok.video.")
}

func (s *Server) persistActual(ctx context.Context, run store.Run, actual billing.Actual) {
	actualAmount := actual.Amount
	delta := actualAmount
	if run.EstimatedAmount != nil {
		delta = actualAmount.Sub(*run.EstimatedAmount)
	}
	formula := "actual settlement from /api/log/token"
	var estimate billing.Estimate
	if json.Unmarshal(run.EstimatedBillingJSON, &estimate) == nil && estimate.Formula != "" {
		formula = estimate.Formula
	}
	actualDocument := map[string]any{
		"found": actual.Found, "pending": actual.Pending, "amount": actual.Amount,
		"currency": actual.Currency, "model": actual.Model, "log_type": actual.LogType,
		"created_at": actual.CreatedAt, "request_id": actual.RequestID, "task_id": actual.TaskID,
		"reason": actual.Reason, "formula": formula,
	}
	actualRaw, _ := json.Marshal(actualDocument)
	deltaRaw, _ := json.Marshal(map[string]any{
		"estimated_amount": run.EstimatedAmount, "actual_amount": actual.Amount,
		"delta_amount": delta, "currency": actual.Currency,
	})
	if err := s.store.UpdateRunBilling(ctx, run.ID, store.BillingUpdate{
		ActualJSON: actualRaw, ActualAmount: &actualAmount,
		DeltaJSON: deltaRaw, DeltaAmount: &delta, Currency: actual.Currency,
	}); err != nil {
		s.logger.Error("save actual billing", "run_id", run.ID, "error", err)
	}
}
