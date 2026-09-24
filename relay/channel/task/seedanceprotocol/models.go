package seedanceprotocol

import "strings"

var supportedModels = []string{
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
	"doubao-seedance-2-0-mini-260615",
	"doubao-seedance-2-5-260628",
}

var modelSet = map[string]struct{}{
	"doubao-seedance-2-0-260128":      {},
	"doubao-seedance-2-0-fast-260128": {},
	"doubao-seedance-2-0-mini-260615": {},
	"doubao-seedance-2-5-260628":      {},
}

func SupportedModels() []string {
	return append([]string(nil), supportedModels...)
}

func FilterSupportedModels(models []string) []string {
	result := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, raw := range models {
		model := strings.TrimSpace(raw)
		if _, supported := modelSet[model]; !supported {
			continue
		}
		if _, duplicate := seen[model]; duplicate {
			continue
		}
		seen[model] = struct{}{}
		result = append(result, model)
	}
	return result
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type ContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type Tool struct {
	Type string `json:"type,omitempty"`
}

type Payload struct {
	Model         string        `json:"model"`
	Content       []ContentItem `json:"content"`
	GenerateAudio *bool         `json:"generate_audio,omitempty"`
	Resolution    string        `json:"resolution,omitempty"`
	Ratio         string        `json:"ratio,omitempty"`
	Duration      *int          `json:"duration,omitempty"`
	Watermark     *bool         `json:"watermark,omitempty"`
	Tools         []Tool        `json:"tools,omitempty"`
}

type Usage struct {
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	ToolUsage        struct {
		WebSearch int `json:"web_search"`
	} `json:"tool_usage"`
}

type NestedTaskPayload struct {
	Status          string  `json:"status"`
	Message         string  `json:"message"`
	FailReason      string  `json:"fail_reason"`
	Duration        float64 `json:"duration"`
	Resolution      string  `json:"resolution"`
	Ratio           string  `json:"ratio"`
	FramesPerSecond int     `json:"framespersecond"`
	Seed            int64   `json:"seed"`
	Content         struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Usage *Usage `json:"usage"`
	Error any    `json:"error"`
}

type TaskPayload struct {
	TaskID     string            `json:"task_id"`
	ID         any               `json:"id"`
	Message    string            `json:"message"`
	Status     string            `json:"status"`
	FailReason string            `json:"fail_reason"`
	ResultURL  string            `json:"result_url"`
	Progress   any               `json:"progress"`
	Usage      *Usage            `json:"usage"`
	Error      any               `json:"error"`
	Data       NestedTaskPayload `json:"data"`
}

type ResponseEnvelope struct {
	Code       any         `json:"code"`
	Message    string      `json:"message"`
	Error      any         `json:"error"`
	FailReason string      `json:"fail_reason"`
	TaskID     string      `json:"task_id"`
	ID         any         `json:"id"`
	Status     string      `json:"status"`
	ResultURL  string      `json:"result_url"`
	Progress   any         `json:"progress"`
	Usage      *Usage      `json:"usage"`
	Data       TaskPayload `json:"data"`
}
