package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"

	"molii-aigc-demo/internal/upstream"
)

const quotaPerCurrencyUnit = int64(500_000)

type Log struct {
	CreatedAt int64           `json:"created_at"`
	Type      int             `json:"type"`
	ModelName string          `json:"model_name"`
	Quota     json.Number     `json:"quota"`
	RequestID string          `json:"request_id"`
	Other     json.RawMessage `json:"other"`
}

type logEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    []Log  `json:"data"`
}

type Reference struct {
	Kind      string // image or video
	RequestID string
	TaskID    string
}

type Actual struct {
	Found     bool            `json:"found"`
	Pending   bool            `json:"pending"`
	Amount    decimal.Decimal `json:"amount"`
	Currency  string          `json:"currency"`
	Model     string          `json:"model,omitempty"`
	LogType   int             `json:"log_type,omitempty"`
	CreatedAt int64           `json:"created_at,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	TaskID    string          `json:"task_id,omitempty"`
	Reason    string          `json:"reason,omitempty"`
}

func FetchActual(ctx context.Context, client *upstream.Client, baseURL, apiKey string, ref Reference) (Actual, error) {
	if client == nil {
		return Actual{}, errors.New("upstream client is required")
	}
	result, err := client.Do(ctx, baseURL, apiKey, upstream.PreparedRequest{Operation: "billing.logs", Method: http.MethodGet, Path: "/api/log/token"})
	if err != nil {
		return Actual{}, fmt.Errorf("fetch token logs: %w", err)
	}
	var envelope logEnvelope
	dec := json.NewDecoder(bytes.NewReader(result.Body))
	dec.UseNumber()
	if err := dec.Decode(&envelope); err != nil {
		return Actual{}, fmt.Errorf("decode token logs: %w", err)
	}
	if !envelope.Success {
		return Actual{}, fmt.Errorf("token log API failed: %s", strings.TrimSpace(envelope.Message))
	}
	return Reconcile(envelope.Data, ref), nil
}

func Reconcile(logs []Log, ref Reference) Actual {
	actual := Actual{Pending: true, Currency: "CNY", RequestID: ref.RequestID, TaskID: ref.TaskID}
	for _, item := range logs {
		other := parseOther(item.Other)
		matches := false
		if ref.Kind == "image" && ref.RequestID != "" {
			matches = item.RequestID == ref.RequestID
		} else if ref.Kind == "video" && ref.TaskID != "" {
			matches = stringFromAny(other["task_id"]) == ref.TaskID
		}
		if !matches {
			continue
		}
		actual.Found, actual.Pending = true, false
		actual.Model, actual.LogType, actual.CreatedAt = item.ModelName, item.Type, item.CreatedAt
		if item.Type == 5 { // error log; failed async generation is fully refunded
			actual.Amount = decimal.Zero
			actual.Reason = "generation failed; terminal error log reports no charge"
			return actual
		}
		if cost, ok := nestedFinalCost(other, "grok_image_billing"); ok {
			actual.Amount = cost
			return actual
		}
		if cost, ok := nestedFinalCost(other, "grok_video_billing"); ok {
			actual.Amount = cost
			return actual
		}
		quota, err := decimal.NewFromString(item.Quota.String())
		if err == nil {
			actual.Amount = quota.Div(decimal.NewFromInt(quotaPerCurrencyUnit))
			return actual
		}
		actual.Reason = "matched log has an invalid quota"
		return actual
	}
	actual.Reason = "settlement log has not synchronized yet"
	return actual
}

func parseOther(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		raw = []byte(text)
	}
	var output map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	_ = dec.Decode(&output)
	return output
}

func nestedFinalCost(other map[string]any, key string) (decimal.Decimal, bool) {
	value, ok := other[key].(map[string]any)
	if !ok {
		return decimal.Zero, false
	}
	text := stringFromAny(value["final_cost"])
	if text == "" {
		return decimal.Zero, false
	}
	cost, err := decimal.NewFromString(text)
	return cost, err == nil
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return decimal.NewFromFloat(typed).String()
	default:
		return ""
	}
}
