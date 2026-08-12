package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupDefaultTokenRegistrationTestDB(t *testing.T) {
	t.Helper()
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}))
}

func requireSingleDefaultToken(t *testing.T, userID int) *model.Token {
	t.Helper()
	var tokens []model.Token
	require.NoError(t, model.DB.Where("user_id = ?", userID).Find(&tokens).Error)
	require.Len(t, tokens, 1)
	require.True(t, tokens[0].IsDefault)
	return &tokens[0]
}

func TestPasswordRegistrationCreatesDefaultToken(t *testing.T) {
	setupDefaultTokenRegistrationTestDB(t)
	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
	})

	router := gin.New()
	router.POST("/api/user/register", Register)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{"username":"default_pw_user","password":"password1"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	require.Contains(t, recorder.Body.String(), `"success":true`)

	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "default_pw_user").First(&user).Error)
	requireSingleDefaultToken(t, user.Id)
}

func TestOAuthRegistrationCreatesDefaultToken(t *testing.T) {
	setupDefaultTokenRegistrationTestDB(t)
	previousRegisterEnabled := common.RegisterEnabled
	common.RegisterEnabled = true
	t.Cleanup(func() { common.RegisterEnabled = previousRegisterEnabled })

	provider := &authFlowTestOAuthProvider{}
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	user, err := findOrCreateOAuthUser(context, provider, &oauth.OAuthUser{ProviderUserID: "default-oauth-user", Username: "default-oauth-user"}, "")
	require.NoError(t, err)
	require.NotZero(t, user.Id)
	requireSingleDefaultToken(t, user.Id)
}

func TestWeChatRegistrationCreatesDefaultToken(t *testing.T) {
	setupDefaultTokenRegistrationTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/wechat/user", request.URL.Path)
		_, _ = writer.Write([]byte(`{"success":true,"data":"test-wechat-user"}`))
	}))
	t.Cleanup(server.Close)

	previousRegisterEnabled := common.RegisterEnabled
	previousWeChatAuthEnabled := common.WeChatAuthEnabled
	previousWeChatServerAddress := common.WeChatServerAddress
	previousWeChatServerToken := common.WeChatServerToken
	common.RegisterEnabled = true
	common.WeChatAuthEnabled = true
	common.WeChatServerAddress = server.URL
	common.WeChatServerToken = "test-token"
	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.WeChatAuthEnabled = previousWeChatAuthEnabled
		common.WeChatServerAddress = previousWeChatServerAddress
		common.WeChatServerToken = previousWeChatServerToken
	})

	router := gin.New()
	router.GET("/api/oauth/wechat", WeChatAuth)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/oauth/wechat?code=test-code", nil))

	var user model.User
	require.NoError(t, model.DB.Where("wechat_id = ?", "test-wechat-user").First(&user).Error)
	requireSingleDefaultToken(t, user.Id)
}
