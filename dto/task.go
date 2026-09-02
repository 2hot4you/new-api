package dto

import (
	"encoding/json"
)

type TaskError struct {
	Code       string `json:"code"`
	Type       string `json:"type,omitempty"`
	Message    string `json:"message"`
	Data       any    `json:"data"`
	StatusCode int    `json:"-"`
	LocalError bool   `json:"-"`
	Error      error  `json:"-"`
}

type TaskData interface {
	SunoDataResponse | []SunoDataResponse | string | any
}

const TaskSuccessCode = "success"

type TaskResponse[T TaskData] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func (t *TaskResponse[T]) IsSuccess() bool {
	return t.Code == TaskSuccessCode
}

type TaskDto struct {
	ID                   int64               `json:"id"`
	CreatedAt            int64               `json:"created_at"`
	UpdatedAt            int64               `json:"updated_at"`
	TaskID               string              `json:"task_id"`
	Platform             string              `json:"platform"`
	UserId               int                 `json:"user_id"`
	Group                string              `json:"group"`
	ChannelId            int                 `json:"channel_id"`
	Quota                int                 `json:"quota"`
	Action               string              `json:"action"`
	Status               string              `json:"status"`
	FailReason           string              `json:"fail_reason"`
	ResultURL            string              `json:"result_url,omitempty"` // 任务结果 URL（视频地址等）
	SubmitTime           int64               `json:"submit_time"`
	StartTime            int64               `json:"start_time"`
	FinishTime           int64               `json:"finish_time"`
	Progress             string              `json:"progress"`
	Properties           any                 `json:"properties"`
	Username             string              `json:"username,omitempty"`
	Data                 json.RawMessage     `json:"data"`
	VideoParams          *TaskVideoParams    `json:"video_params,omitempty"`
	Billing              *TaskBillingSummary `json:"billing,omitempty"`
	LegacyVideoAvailable bool                `json:"legacy_video_available,omitempty"`
	AdminInfo            *TaskAdminInfo      `json:"admin_info,omitempty"`
	RootInfo             *TaskRootInfo       `json:"root_info,omitempty"`
}

type TaskPluginInfo struct {
	Key     string                `json:"key"`
	Name    string                `json:"name"`
	Version string                `json:"version,omitempty"`
	Author  *TaskPluginAuthorInfo `json:"author,omitempty"`
}
type TaskPluginAuthorInfo struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}
type TaskPluginRuntimeInfo struct {
	Key        string `json:"key"`
	Version    string `json:"version"`
	APIVersion int    `json:"api_version"`
	Generation uint64 `json:"generation"`
}
type TaskAdminInfo struct {
	RequestID   string          `json:"request_id,omitempty"`
	RequestPath string          `json:"request_path,omitempty"`
	TaskPlugin  *TaskPluginInfo `json:"task_plugin,omitempty"`
}
type TaskRootInfo struct {
	TaskPlugin     *TaskPluginRuntimeInfo `json:"task_plugin,omitempty"`
	UpstreamTaskID string                 `json:"upstream_task_id,omitempty"`
	NodeName       string                 `json:"node_name,omitempty"`
}

// TaskVideoParams contains only non-content generation metadata safe for task
// log presentation. It deliberately excludes prompts, input URLs and results.
type TaskVideoParams struct {
	Resolution string `json:"resolution,omitempty"`
	Ratio      string `json:"ratio,omitempty"`
	Seconds    int    `json:"seconds,omitempty"`
	FPS        int    `json:"fps,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	HasVideo   bool   `json:"has_video"`
}

// TaskBillingSummary is the public-safe settlement view used by generation
// records. It deliberately excludes funding identifiers, upstream request
// data, media references, prompts and every other TaskPrivateData field.
type TaskBillingSummary struct {
	State           string                  `json:"state"`
	Mode            string                  `json:"mode"`
	Model           string                  `json:"model,omitempty"`
	FinalCost       float64                 `json:"final_cost"`
	GroupRatio      float64                 `json:"group_ratio"`
	DetailAvailable bool                    `json:"detail_available"`
	Seedance        *TaskSeedanceBilling    `json:"seedance,omitempty"`
	GrokVideo       *TaskGrokVideoBillingV1 `json:"grok_video,omitempty"`
}

type TaskSeedanceBilling struct {
	ActualTokens int     `json:"actual_tokens"`
	Resolution   string  `json:"resolution,omitempty"`
	Ratio        string  `json:"ratio,omitempty"`
	Seconds      int     `json:"seconds,omitempty"`
	HasVideo     bool    `json:"has_video"`
	UnitPrice    float64 `json:"unit_price"`
}

// TaskGrokVideoBillingV1 mirrors the versioned, public-safe task snapshot.
// Keeping the DTO explicit prevents accidental exposure when the internal
// billing context gains new fields.
type TaskGrokVideoBillingV1 struct {
	Version                  int     `json:"version"`
	Model                    string  `json:"model"`
	Operation                string  `json:"operation"`
	InputType                string  `json:"input_type"`
	RequestedDurationSeconds float64 `json:"requested_duration_seconds"`
	EstimatedDurationSeconds float64 `json:"estimated_duration_seconds"`
	ActualDurationSeconds    float64 `json:"actual_duration_seconds"`
	RequestedResolution      string  `json:"requested_resolution"`
	EstimatedResolution      string  `json:"estimated_resolution"`
	ActualResolution         string  `json:"actual_resolution"`
	AspectRatio              string  `json:"aspect_ratio"`
	InputImageCount          int     `json:"input_image_count"`
	VideoInputBilledSeconds  float64 `json:"video_input_billed_seconds"`
	OutputUnitPrice          float64 `json:"output_unit_price"`
	ImageInputUnitPrice      float64 `json:"image_input_unit_price"`
	VideoInputUnitPrice      float64 `json:"video_input_unit_price"`
	OutputCost               float64 `json:"output_cost"`
	ImageInputCost           float64 `json:"image_input_cost"`
	VideoInputCost           float64 `json:"video_input_cost"`
	Subtotal                 float64 `json:"subtotal"`
	GroupRatio               float64 `json:"group_ratio"`
	FinalCost                float64 `json:"final_cost"`
}

type FetchReq struct {
	IDs []string `json:"ids"`
}
