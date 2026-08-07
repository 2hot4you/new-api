package upstream

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type PollResult struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	Progress  int    `json:"progress"`
	ResultURL string `json:"result_url,omitempty"`
	Error     string `json:"error,omitempty"`
	Terminal  bool   `json:"terminal"`
	Success   bool   `json:"success"`
}

// ParsePollResponse accepts both the OpenAI video view and Seedance's generic
// {code,data} view.
func ParsePollResponse(body []byte) (PollResult, error) {
	var envelope map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&envelope); err != nil {
		return PollResult{}, fmt.Errorf("invalid poll response: %w", err)
	}
	data := envelope
	if nested, ok := envelope["data"].(map[string]any); ok {
		data = nested
	}
	result := PollResult{
		TaskID:    firstString(data, "task_id", "id"),
		Status:    strings.ToLower(firstString(data, "status")),
		ResultURL: firstString(data, "result_url"),
		Error:     firstString(data, "fail_reason", "message"),
	}
	if metadata, ok := data["metadata"].(map[string]any); ok && result.ResultURL == "" {
		result.ResultURL = firstString(metadata, "url")
	}
	if errValue, ok := data["error"].(map[string]any); ok && result.Error == "" {
		result.Error = firstString(errValue, "message", "code")
	}
	result.Progress = parseProgress(data["progress"])
	switch strings.ToUpper(result.Status) {
	case "SUCCESS", "SUCCEEDED", "COMPLETED":
		result.Status, result.Terminal, result.Success = "completed", true, true
		if result.Progress == 0 {
			result.Progress = 100
		}
	case "FAILURE", "FAILED", "CANCELLED", "EXPIRED":
		result.Status, result.Terminal = "failed", true
	case "SUBMITTED", "NOT_START", "QUEUED", "PENDING":
		result.Status = "queued"
	case "PROCESSING", "RUNNING", "IN_PROGRESS", "UNKNOWN":
		result.Status = "in_progress"
	default:
		return PollResult{}, errors.New("poll response did not contain a recognized status")
	}
	return result, nil
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case json.Number:
			return value.String()
		}
	}
	return ""
}

func parseProgress(value any) int {
	switch typed := value.(type) {
	case json.Number:
		v, _ := strconv.Atoi(typed.String())
		return v
	case float64:
		return int(typed)
	case string:
		v, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(typed), "%"))
		return v
	default:
		return 0
	}
}
