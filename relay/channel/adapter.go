package channel

import (
	"io"
	"net/http"

	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor interface {
	// Init IsStream bool
	Init(info *relaycommon.RelayInfo)
	GetRequestURL(info *relaycommon.RelayInfo) (string, error)
	SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error
	ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error)
	ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error)
	ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error)
	ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error)
	ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error)
	ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error)
	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error)
	DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError)
	GetModelList() []string
	GetChannelName() string
	ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error)
	ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error)
}

type TaskAdaptor interface {
	Init(info *relaycommon.RelayInfo)

	ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError

	// ── Billing ──────────────────────────────────────────────────────

	// EstimateBilling returns OtherRatios for pre-charge based on user request.
	// Called after ValidateRequestAndSetAction, before price calculation.
	// Adaptors should extract duration, resolution, etc. from the parsed request
	// and return them as ratio multipliers (e.g. {"seconds": 5, "size": 1.666}).
	// Return nil to use the base model price without extra ratios.
	EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64

	// AdjustBillingOnSubmit returns adjusted OtherRatios from the upstream
	// submit response. Called after a successful DoResponse.
	// If the upstream returned actual parameters that differ from the estimate
	// (e.g. actual seconds), return updated ratios so the caller can recalculate
	// the quota and settle the delta with the pre-charge.
	// Return nil if no adjustment is needed.
	AdjustBillingOnSubmit(info *relaycommon.RelayInfo, taskData []byte) map[string]float64

	// AdjustBillingOnComplete returns the actual quota when a task reaches a
	// terminal state (success/failure) during polling.
	// Called by the polling loop after ParseTaskResult.
	// Return a positive value to trigger delta settlement (supplement / refund).
	// Return 0 to keep the pre-charged amount unchanged.
	AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int

	// ── Request / Response ───────────────────────────────────────────

	BuildRequestURL(info *relaycommon.RelayInfo) (string, error)
	BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error
	BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error)

	DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error)
	ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*TaskSubmitResponse, *taskdto.TaskError)

	GetModelList() []string
	GetChannelName() string

	// ── Polling ──────────────────────────────────────────────────────

	FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error)
}

// TaskSubmitResponse is parsed without writing to the client. The controller
// sends ClientResponse only after the task and its billing state are durable.
type TaskSubmitResponse struct {
	UpstreamTaskID string
	TaskData       []byte
	ClientResponse any
	Immediate      *relaycommon.TaskInfo
}

type OpenAIVideoConverter interface {
	ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error)
}

type TaskArtifact = hosttypes.TaskArtifact

type TaskArtifactClientRequest struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
}

type TaskArtifactProvider interface {
	ListArtifacts(task *model.Task) ([]TaskArtifact, error)
}

type TaskContentRequest struct {
	URL            string
	Method         string
	Headers        map[string]string
	Body           []byte
	Credentialless bool
}

type TaskContentRequestProvider interface {
	BuildContentRequest(task *model.Task, artifactKey string, clientRequest TaskArtifactClientRequest) (*TaskContentRequest, error)
}

type TaskUsageFactsProvider interface {
	ExtractUsageFacts(c *gin.Context, info *relaycommon.RelayInfo) map[string]any
}

type TaskValidatedBillingProvider interface {
	EstimateBillingValidated(c *gin.Context, info *relaycommon.RelayInfo) (map[string]float64, error)
}

type TaskValidatedUsageFactsProvider interface {
	ExtractUsageFactsValidated(c *gin.Context, info *relaycommon.RelayInfo) (map[string]any, error)
}

// TaskRetryPolicy optionally controls whether a task submit may be retried on
// another channel after an upstream failure. Adaptors that do not implement it
// retain the existing automatic retry behavior.
type TaskRetryPolicy interface {
	AllowAutomaticTaskSubmitRetry() bool
}

// TaskSubmitErrorSanitizer optionally prevents raw non-2xx upstream response
// bodies from being copied into task errors.
type TaskSubmitErrorSanitizer interface {
	SanitizeTaskSubmitError(responseBody []byte) string
}

// TaskSubmitErrorMapper optionally supplies a fully sanitized task error with
// a stable public code and type for non-2xx submit responses.
type TaskSubmitErrorMapper interface {
	MapTaskSubmitError(statusCode int, responseBody []byte) *taskdto.TaskError
}

// ImageRequestErrorSanitizer optionally maps provider-specific image request
// validation errors to a safe API error with the correct HTTP status.
type ImageRequestErrorSanitizer interface {
	SanitizeImageRequestError(err error) *types.NewAPIError
}

// ImageErrorSanitizer optionally replaces raw non-2xx image response bodies
// with a provider-safe error before the generic relay error parser runs.
type ImageErrorSanitizer interface {
	SanitizeImageError(statusCode int, responseBody []byte) *types.NewAPIError
}

type ImageTransportErrorSanitizer interface {
	SanitizeImageTransportError(err error) *types.NewAPIError
}

type ImageRequestLoggingPolicy interface {
	AllowImageRequestBodyLog() bool
}

// ImageBillingEstimator lets a provider replace generic image count ratios
// with a direct media-cost ratio before pre-consumption.
type ImageBillingEstimator interface {
	EstimateImageBilling(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (map[string]float64, error)
}

type TaskTransportErrorMapper interface {
	MapTaskTransportError(err error) *taskdto.TaskError
}

// PerCallTaskCompletionAdjuster opts a fixed-price task into completion-time
// delta settlement. Most per-call tasks remain immutable after submit.
type PerCallTaskCompletionAdjuster interface {
	AllowPerCallCompletionAdjustment() bool
}
