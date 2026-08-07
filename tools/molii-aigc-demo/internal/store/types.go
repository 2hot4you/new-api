package store

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrNotFound       = errors.New("record not found")
	ErrConflict       = errors.New("record conflicts with existing data")
	ErrInvalidInput   = errors.New("invalid input")
	ErrSecretRequired = errors.New("API key is required")
)

// Environment is safe to serialize as an API DTO. It intentionally contains
// neither API-key plaintext nor encrypted database fields.
type Environment struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	BaseURL       string    `json:"base_url"`
	KeyConfigured bool      `json:"key_configured"`
	KeyMasked     string    `json:"key_masked"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateEnvironmentParams struct {
	Name    string
	BaseURL string
	APIKey  string
}

// UpdateEnvironmentParams uses pointers to distinguish an omitted field from
// an explicitly empty value. An empty APIKey is rejected rather than erasing it.
type UpdateEnvironmentParams struct {
	Name    *string
	BaseURL *string
	APIKey  *string
}

// EnvironmentCredentials is for server-side upstream calls only. APIKey is
// explicitly excluded from JSON serialization.
type EnvironmentCredentials struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"-"`
}

// UISession is persisted server-side. Cookie and CSRF plaintext are never
// stored in this record; callers supply only cryptographic hashes.
type UISession struct {
	ID                    string     `json:"-" gorm:"column:id;primaryKey"`
	CSRFTokenHash         []byte     `json:"-" gorm:"column:csrf_token_hash"`
	SelectedEnvironmentID *string    `json:"selected_environment_id,omitempty" gorm:"column:selected_environment_id"`
	ExpiresAt             time.Time  `json:"expires_at" gorm:"column:expires_at"`
	CreatedAt             time.Time  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt             time.Time  `json:"updated_at" gorm:"column:updated_at"`
	LastSeenAt            *time.Time `json:"last_seen_at,omitempty" gorm:"column:last_seen_at"`
}

func (UISession) TableName() string { return "ui_sessions" }

type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunSubmitted RunStatus = "submitted"
	RunPolling   RunStatus = "polling"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCanceled  RunStatus = "canceled"
)

func (s RunStatus) Terminal() bool {
	return s == RunSucceeded || s == RunFailed || s == RunCanceled
}

// Run stores the complete durable state of one generation request. JSON fields
// contain already-redacted payloads suitable for history display.
type Run struct {
	ID              string    `json:"id" gorm:"column:id;primaryKey"`
	UISessionID     *string   `json:"-" gorm:"column:ui_session_id"`
	EnvironmentID   *string   `json:"environment_id,omitempty" gorm:"column:environment_id"`
	EnvironmentName string    `json:"environment_name" gorm:"column:environment_name"`
	BaseURL         string    `json:"base_url" gorm:"column:base_url"`
	Provider        string    `json:"provider" gorm:"column:provider"`
	Model           string    `json:"model" gorm:"column:model"`
	Operation       string    `json:"operation" gorm:"column:operation"`
	Status          RunStatus `json:"status" gorm:"column:status"`

	RequestID       string   `json:"request_id,omitempty" gorm:"column:request_id"`
	UpstreamTaskID  string   `json:"upstream_task_id,omitempty" gorm:"column:upstream_task_id"`
	Progress        *float64 `json:"progress,omitempty" gorm:"column:progress"`
	PollAttempts    int      `json:"poll_attempts" gorm:"column:poll_attempts"`
	CancelRequested bool     `json:"cancel_requested" gorm:"column:cancel_requested"`

	RequestJSON  []byte `json:"request_json" gorm:"column:request_json"`
	ResultJSON   []byte `json:"result_json,omitempty" gorm:"column:result_json"`
	ErrorCode    string `json:"error_code,omitempty" gorm:"column:error_code"`
	ErrorMessage string `json:"error_message,omitempty" gorm:"column:error_message"`

	EstimatedBillingJSON []byte           `json:"estimated_billing,omitempty" gorm:"column:estimated_billing_json"`
	EstimatedAmount      *decimal.Decimal `json:"estimated_amount,omitempty" gorm:"column:estimated_amount"`
	ActualBillingJSON    []byte           `json:"actual_billing,omitempty" gorm:"column:actual_billing_json"`
	ActualAmount         *decimal.Decimal `json:"actual_amount,omitempty" gorm:"column:actual_amount"`
	DeltaBillingJSON     []byte           `json:"delta_billing,omitempty" gorm:"column:delta_billing_json"`
	DeltaAmount          *decimal.Decimal `json:"delta_amount,omitempty" gorm:"column:delta_amount"`
	Currency             string           `json:"currency,omitempty" gorm:"column:currency"`

	SubmittedAt  *time.Time `json:"submitted_at,omitempty" gorm:"column:submitted_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty" gorm:"column:completed_at"`
	NextPollAt   *time.Time `json:"next_poll_at,omitempty" gorm:"column:next_poll_at"`
	LastPolledAt *time.Time `json:"last_polled_at,omitempty" gorm:"column:last_polled_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"column:updated_at"`
}

func (Run) TableName() string { return "runs" }

type CreateRunParams struct {
	ID                   string
	UISessionID          *string
	EnvironmentID        *string
	EnvironmentName      string
	BaseURL              string
	Provider             string
	Model                string
	Operation            string
	RequestJSON          []byte
	EstimatedBillingJSON []byte
	EstimatedAmount      *decimal.Decimal
	Currency             string
}

type RunFilter struct {
	UISessionID   *string
	EnvironmentID *string
	Statuses      []RunStatus
	Limit         int
	Offset        int
}

type SubmissionUpdate struct {
	RequestID      string
	UpstreamTaskID string
	Status         RunStatus
	ResultJSON     []byte
	NextPollAt     *time.Time
	SubmittedAt    time.Time
}

type PollUpdate struct {
	Status       RunStatus
	Progress     *float64
	ResultJSON   []byte
	ErrorCode    string
	ErrorMessage string
	NextPollAt   *time.Time
	PolledAt     time.Time
	CompletedAt  *time.Time
}

type BillingUpdate struct {
	EstimatedJSON   []byte
	EstimatedAmount *decimal.Decimal
	ActualJSON      []byte
	ActualAmount    *decimal.Decimal
	DeltaJSON       []byte
	DeltaAmount     *decimal.Decimal
	Currency        string
}

// Exchange is one chronological HTTP interaction in a run. Headers and URLs
// supplied here must already be redacted by the upstream layer.
type Exchange struct {
	ID                  string     `json:"id" gorm:"column:id;primaryKey"`
	RunID               string     `json:"run_id" gorm:"column:run_id"`
	Sequence            int        `json:"sequence" gorm:"column:sequence"`
	Kind                string     `json:"kind" gorm:"column:kind"`
	Method              string     `json:"method,omitempty" gorm:"column:method"`
	URL                 string     `json:"url,omitempty" gorm:"column:url"`
	RequestHeadersJSON  []byte     `json:"request_headers,omitempty" gorm:"column:request_headers_json"`
	RequestBodyJSON     []byte     `json:"request_body,omitempty" gorm:"column:request_body_json"`
	ResponseStatus      *int       `json:"response_status,omitempty" gorm:"column:response_status"`
	ResponseHeadersJSON []byte     `json:"response_headers,omitempty" gorm:"column:response_headers_json"`
	ResponseBodyJSON    []byte     `json:"response_body,omitempty" gorm:"column:response_body_json"`
	Error               string     `json:"error,omitempty" gorm:"column:error"`
	StartedAt           time.Time  `json:"started_at" gorm:"column:started_at"`
	FinishedAt          *time.Time `json:"finished_at,omitempty" gorm:"column:finished_at"`
	DurationMS          *int64     `json:"duration_ms,omitempty" gorm:"column:duration_ms"`
	CreatedAt           time.Time  `json:"created_at" gorm:"column:created_at"`
}

func (Exchange) TableName() string { return "exchanges" }

type RunWithExchanges struct {
	Run       Run        `json:"run"`
	Exchanges []Exchange `json:"exchanges"`
}

// RunMediaSource is decrypted only for the short-lived redirect response. Its
// URL is never JSON serialized or included in request/response history.
type RunMediaSource struct {
	RunID         string `json:"run_id"`
	EnvironmentID string `json:"environment_id"`
	Position      int    `json:"position"`
	URL           string `json:"-"`
}
