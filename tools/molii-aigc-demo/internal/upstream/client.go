package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultTimeout     = 45 * time.Second
	DefaultResponseCap = int64(4 << 20)
	maxRedirects       = 3
)

var sensitiveHeaders = map[string]bool{
	"authorization": true, "proxy-authorization": true, "cookie": true, "set-cookie": true,
	"x-api-key": true, "api-key": true,
}

type Client struct {
	httpClient  *http.Client
	responseCap int64
}

type Result struct {
	StatusCode       int           `json:"status_code"`
	Header           http.Header   `json:"-"`
	Body             []byte        `json:"-"`
	LogHeader        http.Header   `json:"log_headers"`
	LogBody          []byte        `json:"log_body"`
	RequestLogHeader http.Header   `json:"request_log_headers"`
	RequestLogBody   []byte        `json:"request_log_body,omitempty"`
	Duration         time.Duration `json:"duration"`
	RequestID        string        `json:"request_id,omitempty"`
	Attempts         int           `json:"attempts"`
	ResponseTooLarge bool          `json:"response_too_large,omitempty"`
}

type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string { return fmt.Sprintf("upstream returned HTTP %d", e.StatusCode) }

func NewClient(timeout time.Duration, responseCap int64, transport http.RoundTripper) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if responseCap <= 0 {
		responseCap = DefaultResponseCap
	}
	if transport == nil {
		transport = http.DefaultTransport
	}
	c := &Client{responseCap: responseCap}
	c.httpClient = &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			previous := via[len(via)-1]
			if !sameOrigin(previous.URL, next.URL) {
				return errors.New("cross-origin redirect refused")
			}
			if previous.URL.Scheme == "https" && next.URL.Scheme != "https" {
				return errors.New("HTTPS downgrade redirect refused")
			}
			return nil
		},
	}
	return c
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("base URL must be an absolute HTTP or HTTPS URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("base URL must not contain credentials, query, or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/"
	return parsed, nil
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

// Do executes exactly one initial request. In particular, generation POSTs
// are never retried after a timeout or transport failure.
func (c *Client) Do(ctx context.Context, baseURL, apiKey string, prepared PreparedRequest) (Result, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return Result{}, errors.New("API key is required")
	}
	target := base.ResolveReference(&url.URL{Path: prepared.Path})
	var body io.Reader
	if len(prepared.Body) != 0 {
		body = bytes.NewReader(prepared.Body)
	}
	req, err := http.NewRequestWithContext(ctx, prepared.Method, target.String(), body)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	if len(prepared.Body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	requestLogHeader := redactHeaders(req.Header, apiKey)
	requestLogBody := RedactBody(prepared.Body, apiKey)
	started := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{Duration: time.Since(started), Attempts: 1, RequestLogHeader: requestLogHeader, RequestLogBody: requestLogBody}, redactError(err, apiKey)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, c.responseCap+1)
	responseBody, readErr := io.ReadAll(limited)
	result := Result{
		StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: responseBody,
		LogHeader: redactHeaders(resp.Header, apiKey), RequestLogHeader: requestLogHeader,
		RequestLogBody: requestLogBody, Duration: time.Since(started), Attempts: 1,
		RequestID: firstHeader(resp.Header, "X-Oneapi-Request-Id", "X-Request-Id", "X-Request-ID", "Request-Id"),
	}
	if readErr != nil {
		return result, redactError(readErr, apiKey)
	}
	if int64(len(responseBody)) > c.responseCap {
		result.ResponseTooLarge = true
		result.Body = nil
		result.LogBody = []byte(`{"error":"response exceeded configured size limit"}`)
		return result, errors.New("upstream response exceeded configured size limit")
	}
	result.LogBody = RedactBody(responseBody, apiKey)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, &HTTPError{StatusCode: resp.StatusCode, Body: result.LogBody}
	}
	return result, nil
}

func firstHeader(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func RedactHeaders(input http.Header) http.Header {
	return redactHeaders(input, "")
}

func redactHeaders(input http.Header, apiKey string) http.Header {
	output := make(http.Header, len(input))
	for key, values := range input {
		if sensitiveHeaders[strings.ToLower(key)] {
			output[key] = []string{"[REDACTED]"}
			continue
		}
		for _, value := range values {
			output.Add(key, RedactText(value, apiKey))
		}
	}
	return output
}

// Stream opens a video-content response without applying the JSON response
// cap. It intentionally accepts only the prepared video.content GET operation,
// and forwards only conditional range headers. The caller must close the body.
func (c *Client) Stream(ctx context.Context, baseURL, apiKey string, prepared PreparedRequest, requestHeaders http.Header) (*http.Response, error) {
	if prepared.Operation != "video.content" || prepared.Method != http.MethodGet || len(prepared.Body) != 0 {
		return nil, errors.New("streaming is only allowed for video.content GET requests")
	}
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("API key is required")
	}
	target := base.ResolveReference(&url.URL{Path: prepared.Path})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "*/*")
	for _, name := range []string{"Range", "If-Range"} {
		if value := requestHeaders.Get(name); value != "" {
			req.Header.Set(name, value)
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, redactError(err, apiKey)
	}
	return resp, nil
}

func redactError(err error, apiKey string) error {
	if err == nil {
		return nil
	}
	return errors.New(RedactText(err.Error(), apiKey))
}

func RedactBody(body []byte, apiKey string) []byte {
	var value any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return []byte(RedactText(string(body), apiKey))
	}
	redactJSON(&value, apiKey)
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte("[REDACTED]")
	}
	return encoded
}

func redactJSON(value *any, apiKey string) {
	switch typed := (*value).(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if sensitiveHeaders[lower] || sensitiveQueryKeys[lower] || strings.Contains(lower, "api_key") || strings.Contains(lower, "authorization") || strings.Contains(lower, "cookie") || strings.Contains(lower, "secret") {
				typed[key] = "[REDACTED]"
				continue
			}
			redactJSON(&child, apiKey)
			typed[key] = child
		}
	case []any:
		for i := range typed {
			redactJSON(&typed[i], apiKey)
		}
	case string:
		*value = RedactText(typed, apiKey)
	}
}

var sensitiveQueryKeys = map[string]bool{
	"signature": true, "x-tos-signature": true, "x-amz-signature": true,
	"x-amz-credential": true, "credential": true, "access_token": true,
	"token": true, "api_key": true, "key": true,
}

func RedactText(text, apiKey string) string {
	if apiKey != "" {
		text = strings.ReplaceAll(text, apiKey, "[REDACTED]")
	}
	words := strings.Fields(text)
	for i, word := range words {
		candidate := strings.Trim(word, "\"'(),")
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Scheme == "" || parsed.RawQuery == "" {
			continue
		}
		query := parsed.Query()
		changed := false
		for key := range query {
			if sensitiveQueryKeys[strings.ToLower(key)] {
				query.Set(key, "[REDACTED]")
				changed = true
			}
		}
		if changed {
			parsed.RawQuery = query.Encode()
			words[i] = strings.Replace(word, candidate, parsed.String(), 1)
		}
	}
	return strings.Join(words, " ")
}
