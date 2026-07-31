package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type middlewareApplicationErrorResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeMiddlewareApplicationError(t *testing.T, recorder *httptest.ResponseRecorder) middlewareApplicationErrorResponse {
	t.Helper()
	var response middlewareApplicationErrorResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReadCloser) Close() error             { return nil }

func TestAnonymousRequestBodyLimitErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousLimitKB := constant.AnonymousRequestBodyLimitKB
	constant.AnonymousRequestBodyLimitKB = 1
	t.Cleanup(func() { constant.AnonymousRequestBodyLimitKB = previousLimitKB })

	tests := []struct {
		name       string
		body       io.ReadCloser
		wantStatus int
		wantCode   string
	}{
		{
			name:       "too large",
			body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 1025))),
			wantStatus: http.StatusRequestEntityTooLarge,
			wantCode:   "REQUEST_BODY_TOO_LARGE",
		},
		{
			name:       "read failure",
			body:       failingReadCloser{},
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_REQUEST_BODY",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/user/login", nil)
			c.Request.Body = test.body

			AnonymousRequestBodyLimit()(c)

			assert.Equal(t, test.wantStatus, recorder.Code)
			response := decodeMiddlewareApplicationError(t, recorder)
			assert.False(t, response.Success)
			assert.Equal(t, test.wantCode, response.Code)
			assert.NotEmpty(t, response.Message)
			assert.True(t, c.IsAborted())
		})
	}
}

func TestTurnstileCheckErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousEnabled := common.TurnstileCheckEnabled
	previousSecret := common.TurnstileSecretKey
	previousURL := turnstileVerifyURL
	common.TurnstileCheckEnabled = true
	common.TurnstileSecretKey = "test-secret"
	t.Cleanup(func() {
		common.TurnstileCheckEnabled = previousEnabled
		common.TurnstileSecretKey = previousSecret
		turnstileVerifyURL = previousURL
	})

	tests := []struct {
		name     string
		query    string
		response string
		status   int
		wantCode string
	}{
		{name: "missing token", wantCode: "TURNSTILE_REQUIRED"},
		{name: "invalid token", query: "?turnstile=bad", response: `{"success":false}`, status: http.StatusOK, wantCode: "TURNSTILE_INVALID"},
		{name: "malformed upstream response", query: "?turnstile=test", response: `{`, status: http.StatusOK, wantCode: "TURNSTILE_UNAVAILABLE"},
		{name: "upstream HTTP failure", query: "?turnstile=test", response: `unavailable`, status: http.StatusServiceUnavailable, wantCode: "TURNSTILE_UNAVAILABLE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.query != "" {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(test.status)
					_, _ = w.Write([]byte(test.response))
				}))
				defer server.Close()
				turnstileVerifyURL = server.URL
			}

			router := gin.New()
			router.GET("/check", TurnstileCheck(), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/check"+test.query, nil)
			router.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusOK, recorder.Code)
			response := decodeMiddlewareApplicationError(t, recorder)
			assert.False(t, response.Success)
			assert.Equal(t, test.wantCode, response.Code)
			assert.NotEmpty(t, response.Message)
		})
	}
}
