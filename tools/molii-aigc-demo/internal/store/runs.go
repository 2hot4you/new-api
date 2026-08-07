package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Store) CreateRun(ctx context.Context, params CreateRunParams) (Run, error) {
	if strings.TrimSpace(params.EnvironmentName) == "" || strings.TrimSpace(params.BaseURL) == "" ||
		strings.TrimSpace(params.Provider) == "" || strings.TrimSpace(params.Model) == "" || strings.TrimSpace(params.Operation) == "" {
		return Run{}, fmt.Errorf("run identity fields are required: %w", ErrInvalidInput)
	}
	if err := requireJSON("request", params.RequestJSON); err != nil {
		return Run{}, err
	}
	if err := optionalJSON("estimated billing", params.EstimatedBillingJSON); err != nil {
		return Run{}, err
	}
	id := params.ID
	if id == "" {
		id = uuid.NewString()
	}
	now := s.now()
	run := Run{
		ID: id, UISessionID: params.UISessionID, EnvironmentID: params.EnvironmentID,
		EnvironmentName: params.EnvironmentName, BaseURL: params.BaseURL,
		Provider: params.Provider, Model: params.Model, Operation: params.Operation,
		Status: RunPending, RequestJSON: cloneBytes(params.RequestJSON),
		EstimatedBillingJSON: cloneBytes(params.EstimatedBillingJSON), EstimatedAmount: params.EstimatedAmount,
		Currency: params.Currency, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(&run).Error; err != nil {
		return Run{}, normalizeDatabaseError(err)
	}
	return run, nil
}

func (s *Store) GetRun(ctx context.Context, id string) (Run, error) {
	var run Run
	if err := s.db.WithContext(ctx).First(&run, "id = ?", id).Error; err != nil {
		return Run{}, normalizeDatabaseError(err)
	}
	return run, nil
}

func (s *Store) FindRunByRequestID(ctx context.Context, requestID string) (Run, error) {
	var run Run
	if err := s.db.WithContext(ctx).Where("request_id = ?", requestID).Order("created_at DESC").First(&run).Error; err != nil {
		return Run{}, normalizeDatabaseError(err)
	}
	return run, nil
}

func (s *Store) FindRunByUpstreamTaskID(ctx context.Context, taskID string) (Run, error) {
	var run Run
	if err := s.db.WithContext(ctx).Where("upstream_task_id = ?", taskID).Order("created_at DESC").First(&run).Error; err != nil {
		return Run{}, normalizeDatabaseError(err)
	}
	return run, nil
}

func (s *Store) ListRuns(ctx context.Context, filter RunFilter) ([]Run, error) {
	query := s.db.WithContext(ctx).Model(&Run{})
	if filter.UISessionID != nil {
		query = query.Where("ui_session_id = ?", *filter.UISessionID)
	}
	if filter.EnvironmentID != nil {
		query = query.Where("environment_id = ?", *filter.EnvironmentID)
	}
	if len(filter.Statuses) != 0 {
		values := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			if !validRunStatus(status) {
				return nil, fmt.Errorf("unknown run status %q: %w", status, ErrInvalidInput)
			}
			values = append(values, string(status))
		}
		query = query.Where("status IN ?", values)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if filter.Offset < 0 {
		return nil, ErrInvalidInput
	}
	var runs []Run
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(filter.Offset).Find(&runs).Error; err != nil {
		return nil, err
	}
	return runs, nil
}

// ListRecoverableRuns returns async work due at or before the supplied time.
// Submitted rows with no next-poll time are included for crash recovery.
func (s *Store) ListRecoverableRuns(ctx context.Context, due time.Time, limit int) ([]Run, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var runs []Run
	err := s.db.WithContext(ctx).
		Where("status IN ? AND cancel_requested = ? AND (next_poll_at IS NULL OR next_poll_at <= ?)", []string{string(RunSubmitted), string(RunPolling)}, false, due.UTC()).
		Order("COALESCE(next_poll_at, created_at) ASC, id ASC").Limit(limit).Find(&runs).Error
	return runs, err
}

// ListUnreconciledRuns returns terminal work whose actual charge has not yet
// been found. Failed jobs are included because their terminal error log is the
// authoritative zero-charge/refund record. Canceled jobs remain eligible
// because cancellation stops only Demo polling, not the upstream task.
func (s *Store) ListUnreconciledRuns(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var runs []Run
	err := s.db.WithContext(ctx).
		Where("status IN ? AND actual_amount IS NULL", []RunStatus{RunSucceeded, RunFailed, RunCanceled}).
		Order("updated_at ASC, id ASC").Limit(limit).Find(&runs).Error
	return runs, err
}

func (s *Store) MarkRunSubmitted(ctx context.Context, id string, update SubmissionUpdate) error {
	status := update.Status
	if status == "" {
		status = RunSubmitted
	}
	if !validRunStatus(status) || status == RunPending {
		return fmt.Errorf("invalid submitted status %q: %w", status, ErrInvalidInput)
	}
	if err := optionalJSON("result", update.ResultJSON); err != nil {
		return err
	}
	submittedAt := update.SubmittedAt
	if submittedAt.IsZero() {
		submittedAt = s.now()
	}
	updates := map[string]any{
		"request_id": update.RequestID, "upstream_task_id": update.UpstreamTaskID,
		"status": status, "result_json": nullableBytes(update.ResultJSON),
		"next_poll_at": update.NextPollAt, "submitted_at": submittedAt.UTC(), "updated_at": s.now(),
	}
	if status.Terminal() {
		updates["completed_at"] = submittedAt.UTC()
		updates["next_poll_at"] = nil
	}
	return s.updateRun(ctx, id, updates)
}

// UpdateRunPoll records one poll result and increments PollAttempts atomically.
func (s *Store) UpdateRunPoll(ctx context.Context, id string, update PollUpdate) error {
	if !validRunStatus(update.Status) || update.Status == RunPending || update.Status == RunSubmitted {
		return fmt.Errorf("invalid poll status %q: %w", update.Status, ErrInvalidInput)
	}
	if update.Progress != nil && (*update.Progress < 0 || *update.Progress > 1) {
		return fmt.Errorf("progress must be between 0 and 1: %w", ErrInvalidInput)
	}
	if err := optionalJSON("result", update.ResultJSON); err != nil {
		return err
	}
	polledAt := update.PolledAt
	if polledAt.IsZero() {
		polledAt = s.now()
	}
	updates := map[string]any{
		"status": update.Status, "progress": update.Progress,
		"result_json": nullableBytes(update.ResultJSON), "error_code": update.ErrorCode, "error_message": update.ErrorMessage,
		"next_poll_at": update.NextPollAt, "last_polled_at": polledAt.UTC(),
		"poll_attempts": gorm.Expr("poll_attempts + 1"), "updated_at": s.now(),
	}
	if update.Status.Terminal() {
		completedAt := update.CompletedAt
		if completedAt == nil {
			value := polledAt.UTC()
			completedAt = &value
		}
		updates["completed_at"] = completedAt
		updates["next_poll_at"] = nil
	}
	return s.updateRun(ctx, id, updates)
}

func (s *Store) UpdateRunBilling(ctx context.Context, id string, update BillingUpdate) error {
	for label, value := range map[string][]byte{
		"estimated billing": update.EstimatedJSON,
		"actual billing":    update.ActualJSON,
		"delta billing":     update.DeltaJSON,
	} {
		if err := optionalJSON(label, value); err != nil {
			return err
		}
	}
	updates := map[string]any{"updated_at": s.now()}
	if len(update.EstimatedJSON) != 0 {
		updates["estimated_billing_json"] = cloneBytes(update.EstimatedJSON)
	}
	if update.EstimatedAmount != nil {
		updates["estimated_amount"] = update.EstimatedAmount
	}
	if len(update.ActualJSON) != 0 {
		updates["actual_billing_json"] = cloneBytes(update.ActualJSON)
	}
	if update.ActualAmount != nil {
		updates["actual_amount"] = update.ActualAmount
	}
	if len(update.DeltaJSON) != 0 {
		updates["delta_billing_json"] = cloneBytes(update.DeltaJSON)
	}
	if update.DeltaAmount != nil {
		updates["delta_amount"] = update.DeltaAmount
	}
	if update.Currency != "" {
		updates["currency"] = update.Currency
	}
	return s.updateRun(ctx, id, updates)
}

func (s *Store) MarkRunCancelRequested(ctx context.Context, id string) error {
	return s.updateRun(ctx, id, map[string]any{"cancel_requested": true, "updated_at": s.now()})
}

func (s *Store) MarkRunCanceled(ctx context.Context, id string, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = s.now()
	}
	return s.updateRun(ctx, id, map[string]any{
		"status": RunCanceled, "cancel_requested": true, "completed_at": completedAt.UTC(),
		"next_poll_at": nil, "updated_at": s.now(),
	})
}

func (s *Store) updateRun(ctx context.Context, id string, updates map[string]any) error {
	result := s.db.WithContext(ctx).Model(&Run{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return normalizeDatabaseError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func validRunStatus(status RunStatus) bool {
	switch status {
	case RunPending, RunSubmitted, RunPolling, RunSucceeded, RunFailed, RunCanceled:
		return true
	default:
		return false
	}
}

func requireJSON(label string, value []byte) error {
	if len(value) == 0 || !json.Valid(value) {
		return fmt.Errorf("%s must be valid JSON: %w", label, ErrInvalidInput)
	}
	return nil
}

func optionalJSON(label string, value []byte) error {
	if len(value) != 0 && !json.Valid(value) {
		return fmt.Errorf("%s must be valid JSON: %w", label, ErrInvalidInput)
	}
	return nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return cloneBytes(value)
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	result := make([]byte, len(value))
	copy(result, value)
	return result
}
