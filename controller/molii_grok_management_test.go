package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setMoliiGrokManagementConfigForTest(t *testing.T, baseURL, accessToken string, userID int) {
	t.Helper()
	originalBaseURL := constant.MoliiGrokNewAPIBaseURL
	originalAccessToken := constant.MoliiGrokNewAPIAccessToken
	originalUserID := constant.MoliiGrokNewAPIUserID
	constant.MoliiGrokNewAPIBaseURL = baseURL
	constant.MoliiGrokNewAPIAccessToken = accessToken
	constant.MoliiGrokNewAPIUserID = userID
	t.Cleanup(func() {
		constant.MoliiGrokNewAPIBaseURL = originalBaseURL
		constant.MoliiGrokNewAPIAccessToken = originalAccessToken
		constant.MoliiGrokNewAPIUserID = originalUserID
	})
}

func TestUpdateMoliiGrokBalanceUsesManagementStatusAndSelf(t *testing.T) {
	db := setupChannelBillingTestDB(t)
	const (
		accessToken = "system-access-token-placeholder"
		channelKey  = "channel-key-placeholder"
		userID      = 2205
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			assert.Empty(t, r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
		case "/api/user/self":
			assert.Equal(t, "Bearer "+accessToken, r.Header.Get("Authorization"))
			assert.Equal(t, "2205", r.Header.Get("User-id"))
			assert.NotContains(t, r.Header.Get("Authorization"), channelKey)
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":6250000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	setMoliiGrokManagementConfigForTest(t, server.URL+"/", accessToken, userID)

	channel := &model.Channel{Type: constant.ChannelTypeMoliiGrokAIGC, Name: "Molii Grok", Key: channelKey}
	require.NoError(t, db.Create(channel).Error)

	balance, err := updateChannelMoliiGrokBalance(channel)
	require.NoError(t, err)
	assert.InDelta(t, 12.5, balance, 1e-12)

	var saved model.Channel
	require.NoError(t, db.First(&saved, channel.Id).Error)
	assert.InDelta(t, 12.5, saved.Balance, 1e-12)
}

func TestUpdateMoliiGrokBalancePrefersCompleteChannelManagementCredentials(t *testing.T) {
	db := setupChannelBillingTestDB(t)
	const (
		channelAccessToken = "channel-management-token-placeholder"
		environmentToken   = "environment-management-token-placeholder"
		channelUserID      = 2205
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
		case "/api/user/self":
			assert.Equal(t, "Bearer "+channelAccessToken, r.Header.Get("Authorization"))
			assert.Equal(t, "2205", r.Header.Get("User-id"))
			assert.NotContains(t, r.Header.Get("Authorization"), environmentToken)
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":1000000}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	setMoliiGrokManagementConfigForTest(t, server.URL, environmentToken, 9999)

	channel := &model.Channel{
		Type:                           constant.ChannelTypeMoliiGrokAIGC,
		Name:                           "Molii Grok",
		Key:                            "channel-key-placeholder",
		MoliiGrokManagementAccessToken: channelAccessToken,
		MoliiGrokManagementUserID:      channelUserID,
	}
	require.NoError(t, db.Create(channel).Error)

	balance, err := updateChannelMoliiGrokBalance(channel)
	require.NoError(t, err)
	assert.InDelta(t, 2, balance, 1e-12)
}

func TestMoliiGrokManagementConfigRejectsPartialChannelCredentialsWithoutMixingEnvironment(t *testing.T) {
	setMoliiGrokManagementConfigForTest(t, "https://management.example.invalid", "environment-token-placeholder", 9999)

	_, _, _, err := moliiGrokManagementConfig(&model.Channel{
		Type:                           constant.ChannelTypeMoliiGrokAIGC,
		MoliiGrokManagementAccessToken: "channel-token-placeholder",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "渠道表单")
	assert.NotContains(t, err.Error(), "channel-token-placeholder")
	assert.NotContains(t, err.Error(), "environment-token-placeholder")
}

func TestMoliiGrokManagementAccessTokenNeverSerializes(t *testing.T) {
	const token = "channel-management-token-must-not-leak"
	channel := &model.Channel{
		Type:                           constant.ChannelTypeMoliiGrokAIGC,
		MoliiGrokManagementAccessToken: token,
		MoliiGrokManagementUserID:      2205,
	}
	clearChannelInfo(channel)

	payload, err := json.Marshal(channel)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), token)
	assert.Contains(t, string(payload), `"molii_grok_management_access_token_configured":true`)
	assert.Contains(t, string(payload), `"molii_grok_management_user_id":2205`)
}

func TestUpdateMoliiGrokBalanceRequiresManagementCredentials(t *testing.T) {
	setupChannelBillingTestDB(t)
	setMoliiGrokManagementConfigForTest(t, "https://management.example.invalid", "", 0)

	_, err := updateChannelMoliiGrokBalance(&model.Channel{Key: "channel-key-placeholder"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MOLII_GROK_NEW_API_ACCESS_TOKEN")
	assert.Contains(t, err.Error(), "MOLII_GROK_NEW_API_USER_ID")
	assert.NotContains(t, err.Error(), "channel-key-placeholder")
}

func TestMoliiGrokManagementConfigRejectsUnsafeBaseURLWithoutLeakingURLSecrets(t *testing.T) {
	const urlSecret = "url-secret-placeholder"
	tests := []struct {
		name    string
		baseURL string
	}{
		{name: "unsupported scheme", baseURL: "ftp://management.example.invalid/path/" + urlSecret},
		{name: "missing hostname", baseURL: "https:///" + urlSecret},
		{name: "userinfo", baseURL: "https://user:" + urlSecret + "@management.example.invalid"},
		{name: "query", baseURL: "https://management.example.invalid?token=" + urlSecret},
		{name: "generation path", baseURL: "https://management.example.invalid/xai/" + urlSecret},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setMoliiGrokManagementConfigForTest(t, tt.baseURL, "system-access-token-placeholder", 2205)

			_, _, _, err := moliiGrokManagementConfig(nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "MOLII_GROK_NEW_API_BASE_URL")
			assert.NotContains(t, err.Error(), urlSecret)
			assert.NotContains(t, err.Error(), "user:")
			assert.NotContains(t, err.Error(), "token=")
		})
	}
}

func TestUpdateMoliiGrokBalanceRejectsUnsafeResponsesWithoutLeakingSecrets(t *testing.T) {
	db := setupChannelBillingTestDB(t)
	const (
		accessToken = "system-access-token-placeholder"
		channelKey  = "channel-key-placeholder"
		bodySecret  = "upstream-body-secret-placeholder"
	)

	tests := []struct {
		name       string
		statusCode int
		statusBody string
		selfCode   int
		selfBody   string
		wantError  string
	}{
		{name: "status HTTP failure", statusCode: http.StatusBadGateway, statusBody: bodySecret},
		{name: "status unsuccessful", statusCode: http.StatusOK, statusBody: `{"success":false,"message":"` + bodySecret + `"}`},
		{name: "status malformed JSON", statusCode: http.StatusOK, statusBody: `{"success":true,"data":` + bodySecret},
		{name: "status overflowing number", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":1e999},"message":"` + bodySecret + `"}`},
		{name: "status missing data", statusCode: http.StatusOK, statusBody: `{"success":true,"message":"` + bodySecret + `"}`, wantError: "Molii Grok 管理状态响应缺少 data.quota_per_unit"},
		{name: "status null data", statusCode: http.StatusOK, statusBody: `{"success":true,"data":null,"message":"` + bodySecret + `"}`, wantError: "Molii Grok 管理状态响应缺少 data.quota_per_unit"},
		{name: "status missing quota per unit", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{},"message":"` + bodySecret + `"}`, wantError: "Molii Grok 管理状态响应缺少 data.quota_per_unit"},
		{name: "status null quota per unit", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":null},"message":"` + bodySecret + `"}`, wantError: "Molii Grok 管理状态响应缺少 data.quota_per_unit"},
		{name: "invalid unit", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":0}}`},
		{name: "self HTTP failure", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, selfCode: http.StatusUnauthorized, selfBody: bodySecret},
		{name: "self malformed JSON", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, selfCode: http.StatusOK, selfBody: `{"success":true,"data":` + bodySecret},
		{name: "self overflowing number", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, selfCode: http.StatusOK, selfBody: `{"success":true,"data":{"quota":1e999},"message":"` + bodySecret + `"}`},
		{name: "self missing data", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, selfCode: http.StatusOK, selfBody: `{"success":true,"message":"` + bodySecret + `"}`, wantError: "Molii Grok 用户余额响应缺少 data.quota"},
		{name: "self null data", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, selfCode: http.StatusOK, selfBody: `{"success":true,"data":null,"message":"` + bodySecret + `"}`, wantError: "Molii Grok 用户余额响应缺少 data.quota"},
		{name: "self missing quota", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, selfCode: http.StatusOK, selfBody: `{"success":true,"data":{},"message":"` + bodySecret + `"}`, wantError: "Molii Grok 用户余额响应缺少 data.quota"},
		{name: "self null quota", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, selfCode: http.StatusOK, selfBody: `{"success":true,"data":{"quota":null},"message":"` + bodySecret + `"}`, wantError: "Molii Grok 用户余额响应缺少 data.quota"},
		{name: "negative quota", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`, selfCode: http.StatusOK, selfBody: `{"success":true,"data":{"quota":-1}}`},
		{name: "non-finite converted balance", statusCode: http.StatusOK, statusBody: `{"success":true,"data":{"quota_per_unit":5e-324}}`, selfCode: http.StatusOK, selfBody: `{"success":true,"data":{"quota":1.7976931348623157e308}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/status" {
					w.WriteHeader(tt.statusCode)
					_, _ = w.Write([]byte(tt.statusBody))
					return
				}
				status := tt.selfCode
				if status == 0 {
					status = http.StatusOK
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(tt.selfBody))
			}))
			t.Cleanup(server.Close)
			setMoliiGrokManagementConfigForTest(t, server.URL, accessToken, 2205)

			channel := &model.Channel{Type: constant.ChannelTypeMoliiGrokAIGC, Key: channelKey, Balance: 37.5}
			require.NoError(t, db.Create(channel).Error)
			_, err := updateChannelMoliiGrokBalance(channel)
			require.Error(t, err)
			if tt.wantError != "" {
				require.EqualError(t, err, tt.wantError)
			}
			assert.NotContains(t, err.Error(), accessToken)
			assert.NotContains(t, err.Error(), channelKey)
			assert.NotContains(t, err.Error(), bodySecret)

			var saved model.Channel
			require.NoError(t, db.First(&saved, channel.Id).Error)
			assert.InDelta(t, 37.5, saved.Balance, 1e-12)
		})
	}
}

func TestUpdateMoliiGrokBalanceAcceptsZeroQuota(t *testing.T) {
	db := setupChannelBillingTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status" {
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"quota":0}}`))
	}))
	t.Cleanup(server.Close)
	setMoliiGrokManagementConfigForTest(t, server.URL, "system-access-token-placeholder", 2205)

	channel := &model.Channel{Type: constant.ChannelTypeMoliiGrokAIGC, Key: "channel-key-placeholder", Balance: 37.5}
	require.NoError(t, db.Create(channel).Error)
	balance, err := updateChannelMoliiGrokBalance(channel)
	require.NoError(t, err)
	assert.Zero(t, balance)

	var saved model.Channel
	require.NoError(t, db.First(&saved, channel.Id).Error)
	assert.Zero(t, saved.Balance)
}

func TestUpdateMoliiGrokBalanceRequestsHaveOverallTimeout(t *testing.T) {
	originalTimeout := moliiGrokManagementRequestTimeout
	moliiGrokManagementRequestTimeout = 25 * time.Millisecond
	t.Cleanup(func() { moliiGrokManagementRequestTimeout = originalTimeout })

	for _, hangingPath := range []string{"/api/status", "/api/user/self"} {
		t.Run(hangingPath, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/status" && hangingPath != r.URL.Path {
					_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
					return
				}
				<-r.Context().Done()
			}))
			t.Cleanup(server.Close)
			setMoliiGrokManagementConfigForTest(t, server.URL, "system-access-token-placeholder", 2205)

			started := time.Now()
			_, err := updateChannelMoliiGrokBalance(&model.Channel{Key: "channel-key-placeholder"})
			require.Error(t, err)
			assert.Less(t, time.Since(started), time.Second)
		})
	}
}
