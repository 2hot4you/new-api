package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type authApplicationErrorResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeAuthApplicationError(t *testing.T, recorder *httptest.ResponseRecorder) authApplicationErrorResponse {
	t.Helper()
	var response authApplicationErrorResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestWriteAuthErrorI18nPreservesLegacyStatusAndAddsCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/login", nil)

	writeAuthErrorI18n(c, "AUTH_INVALID_REQUEST", i18n.MsgInvalidParams)

	assert.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAuthApplicationError(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "AUTH_INVALID_REQUEST", response.Code)
	assert.NotEmpty(t, response.Message)
}

func TestPasswordLoginInvalidRequestHasStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousEnabled := common.PasswordLoginEnabled
	common.PasswordLoginEnabled = true
	t.Cleanup(func() { common.PasswordLoginEnabled = previousEnabled })

	router := gin.New()
	router.POST("/api/user/login", Login)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{"username":"","password":""}`))
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAuthApplicationError(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "AUTH_INVALID_REQUEST", response.Code)
	assert.NotEmpty(t, response.Message)
}

func TestPasswordRegistrationInvalidRequestHasStableCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
	})

	router := gin.New()
	router.POST("/api/user/register", Register)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{"username":"   ","password":"password"}`))
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAuthApplicationError(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, "AUTH_INVALID_REQUEST", response.Code)
	assert.NotEmpty(t, response.Message)
}

func TestPasswordAuthFeatureFlagsHaveStableCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		path      string
		handler   gin.HandlerFunc
		configure func()
		wantCode  string
	}{
		{
			name:    "password login disabled",
			path:    "/api/user/login",
			handler: Login,
			configure: func() {
				common.PasswordLoginEnabled = false
			},
			wantCode: "AUTH_PASSWORD_LOGIN_DISABLED",
		},
		{
			name:    "registration disabled",
			path:    "/api/user/register",
			handler: Register,
			configure: func() {
				common.RegisterEnabled = false
			},
			wantCode: "AUTH_REGISTRATION_DISABLED",
		},
		{
			name:    "password registration disabled",
			path:    "/api/user/register",
			handler: Register,
			configure: func() {
				common.RegisterEnabled = true
				common.PasswordRegisterEnabled = false
			},
			wantCode: "AUTH_PASSWORD_REGISTRATION_DISABLED",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previousLoginEnabled := common.PasswordLoginEnabled
			previousRegisterEnabled := common.RegisterEnabled
			previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
			t.Cleanup(func() {
				common.PasswordLoginEnabled = previousLoginEnabled
				common.RegisterEnabled = previousRegisterEnabled
				common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
			})
			test.configure()

			router := gin.New()
			router.POST(test.path, test.handler)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(`{}`))
			router.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, test.wantCode, decodeAuthApplicationError(t, recorder).Code)
		})
	}
}

func TestWriteAuthInternalErrorDoesNotExposeCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/user/login", nil)

	writeAuthInternalErrorI18n(
		context,
		"password login failed",
		errors.New("database password must not reach the client"),
		i18n.MsgDatabaseError,
	)

	response := decodeAuthApplicationError(t, recorder)
	assert.Equal(t, "AUTH_INTERNAL_ERROR", response.Code)
	assert.NotContains(t, response.Message, "database password")
	assert.NotContains(t, recorder.Body.String(), "must not reach the client")
}
