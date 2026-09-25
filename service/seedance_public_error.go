package service

import (
	"encoding/json"
	"strings"
)

type SeedancePublicError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

// Upstream free-form prose cannot be proved free of credentials or account
// identifiers. Recognized public business codes get readable, fixed wording;
// exact approved wording may pass through. Unknown diagnostics never do.
var seedanceBusinessMessages = map[string]string{
	"invalid_request":          "Invalid video request",
	"invalid_parameter":        "Invalid request parameter",
	"invalid_seconds":          "Invalid video duration",
	"content_policy_violation": "Content violates the safety policy",
	"content_filter":           "Content violates the safety policy",
	"insufficient_quota":       "Insufficient quota",
	"rate_limit_exceeded":      "Too many requests; please retry later",
	"model_not_found":          "Requested model is not available",
	"asset_not_found":          "Temporary asset was not found",
	"asset_expired":            "Temporary asset has expired",
	"asset_not_ready":          "Temporary asset is not ready",
	"unsupported_media_type":   "Unsupported media format",
	"temporary_asset_failed":   "temporary asset processing failed",
	"molii_video_failed":       "Molii video task failed",
}

func SafeSeedanceBusinessFields(code, message, kind, fallbackCode, fallbackMessage string) SeedancePublicError {
	result := SeedancePublicError{Code: fallbackCode, Message: fallbackMessage}
	code = strings.ToLower(strings.TrimSpace(code))
	if canonical, ok := seedanceBusinessMessages[code]; ok {
		result.Code, result.Message = code, canonical
		if trimmed := strings.TrimSpace(message); strings.EqualFold(trimmed, canonical) {
			result.Message = trimmed
		}
	}
	switch kind = strings.ToLower(strings.TrimSpace(kind)); kind {
	case "invalid_request_error", "server_error", "rate_limit_error", "authentication_error", "permission_error", "not_found_error", "content_policy_error":
		result.Type = kind
	}
	return result
}

func SeedanceBusinessError(body []byte, fallbackCode, fallbackMessage string) SeedancePublicError {
	fallback := SeedancePublicError{Code: fallbackCode, Message: fallbackMessage}
	if len(body) > 1<<20 {
		return fallback
	}
	for depth := 0; depth <= 4; depth++ {
		var node struct {
			Code    json.RawMessage `json:"code"`
			Message string          `json:"message"`
			Type    string          `json:"type"`
			Error   json.RawMessage `json:"error"`
			Data    json.RawMessage `json:"data"`
		}
		if json.Unmarshal(body, &node) != nil {
			return fallback
		}
		if len(node.Error) > 0 && string(node.Error) != "null" {
			var encoded string
			if json.Unmarshal(node.Error, &encoded) == nil && len(encoded) <= 4096 {
				node.Error = []byte(encoded)
			}
			var business struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Type    string `json:"type"`
			}
			if json.Unmarshal(node.Error, &business) == nil {
				return SafeSeedanceBusinessFields(business.Code, business.Message, business.Type, fallbackCode, fallbackMessage)
			}
			return fallback
		}
		var code string
		_ = json.Unmarshal(node.Code, &code)
		if _, ok := seedanceBusinessMessages[strings.ToLower(strings.TrimSpace(code))]; ok {
			return SafeSeedanceBusinessFields(code, node.Message, node.Type, fallbackCode, fallbackMessage)
		}
		body = node.Data
	}
	return fallback
}
