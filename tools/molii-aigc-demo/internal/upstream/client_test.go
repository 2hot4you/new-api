package upstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerationPostIsNeverRetriedAndLogsAreRedacted(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "Bearer sk-secret", r.Header.Get("Authorization"))
		w.Header().Set("Set-Cookie", "token=secret")
		w.Header().Set("X-Echo", "sk-secret")
		w.Header().Set("X-Request-Id", "req_1")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"error":{"message":"sk-secret https://cdn.test/v?a=1&signature=abc"}}`)
	}))
	defer server.Close()

	client := NewClient(time.Second, 1024, nil)
	prepared := PreparedRequest{Method: http.MethodPost, Path: "/v1/images/generations", Body: []byte(`{}`), Generation: true}
	result, err := client.Do(context.Background(), server.URL, "sk-secret", prepared)
	require.Error(t, err)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, 1, result.Attempts)
	require.Equal(t, "req_1", result.RequestID)
	require.Equal(t, "[REDACTED]", result.LogHeader.Get("Set-Cookie"))
	require.Equal(t, "[REDACTED]", result.RequestLogHeader.Get("Authorization"))
	require.NotContains(t, result.LogHeader.Get("X-Echo"), "sk-secret")
	require.NotContains(t, string(result.LogBody), "sk-secret")
	require.NotContains(t, string(result.LogBody), "signature=abc")
}

func TestClientCapsResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", 65))
	}))
	defer server.Close()
	result, err := NewClient(time.Second, 64, nil).Do(context.Background(), server.URL, "key", PreparedRequest{Method: http.MethodGet, Path: "/large"})
	require.Error(t, err)
	require.True(t, result.ResponseTooLarge)
	require.Nil(t, result.Body)
}

func TestClientRejectsCrossOriginRedirect(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("cross-origin destination must not be called")
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer source.Close()
	_, err := NewClient(time.Second, 1024, nil).Do(context.Background(), source.URL, "secret", PreparedRequest{Method: http.MethodGet, Path: "/redirect"})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret")
}

func TestStreamOnlyAllowsVideoContentAndForwardsRangeHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer stream-secret", r.Header.Get("Authorization"))
		require.Equal(t, "bytes=10-", r.Header.Get("Range"))
		require.Equal(t, `"etag"`, r.Header.Get("If-Range"))
		require.Empty(t, r.Header.Get("Cookie"))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, strings.Repeat("v", 128))
	}))
	defer server.Close()
	client := NewClient(time.Second, 8, nil)
	request, err := BuildResource("video.content", "task_1")
	require.NoError(t, err)
	headers := http.Header{"Range": {"bytes=10-"}, "If-Range": {`"etag"`}, "Cookie": {"do-not-forward"}}
	resp, err := client.Stream(context.Background(), server.URL, "stream-secret", request, headers)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusPartialContent, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Len(t, body, 128, "stream must not use the JSON response cap")

	_, err = client.Stream(context.Background(), server.URL, "stream-secret", PreparedRequest{Operation: "pricing.fetch", Method: http.MethodGet, Path: "/api/pricing"}, nil)
	require.Error(t, err)
}
