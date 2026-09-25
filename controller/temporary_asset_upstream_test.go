package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTemporaryAssetResellerPreservesSafeBusinessFields(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	body := []byte(`{"error":{"code":"asset_not_ready","message":"Temporary asset is not ready","type":"invalid_request_error","diagnostics":{"api_key":"canary","upstream_id":"cgt-private"}}}`)
	failure := parseStarAIAssetUpstreamFailure("create", 400, body, nil)
	writeStarAIAssetUpstreamFailure(c, &model.Channel{Type: constant.ChannelTypeByteDanceSeedance}, failure)
	require.Contains(t, recorder.Body.String(), `"code":"asset_not_ready"`)
	require.Contains(t, recorder.Body.String(), `"type":"invalid_request_error"`)
	require.Contains(t, recorder.Body.String(), `"message":"Temporary asset is not ready"`)
	require.NotContains(t, recorder.Body.String(), "canary")
	require.NotContains(t, recorder.Body.String(), "cgt-private")
}

func TestTemporaryAssetResellerStoredBusinessErrorRetainsSafeType(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"id":"asset-failed","status":"FAILED","expires_at":%d,"error":{"code":"unsupported_media_type","message":"Unsupported media format","type":"invalid_request_error","diagnostics":{"token":"private-canary"}}}`, time.Now().Add(time.Hour).Unix())
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	require.NoError(t, db.Create(&model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusEnabled, Name: "reseller", Key: "account-key", BaseURL: &baseURL}).Error)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	c.Set("id", 42)
	binding, ok := createStarAIAssetUpstream(c, createStarAIAssetRequest{AssetType: "image", Name: "reference"}, &service.StarAIAssetBinding{})
	require.True(t, ok)
	for round := 0; round < 2; round++ {
		stored, err := service.GetStarAIAssetBinding(binding.ID, 42)
		require.NoError(t, err)
		public, err := common.Marshal(safeStarAIAsset(stored))
		require.NoError(t, err)
		require.Contains(t, string(public), `"code":"unsupported_media_type"`)
		require.Contains(t, string(public), `"message":"Unsupported media format"`)
		require.Contains(t, string(public), `"type":"invalid_request_error"`)
		require.NotContains(t, string(public), "private-canary")
		_, err = refreshStarAIAsset(c, binding)
		require.NoError(t, err)
	}
}

func TestTemporaryAssetResellerSelectsOneEnabledCredential(t *testing.T) {
	for _, keys := range []string{"disabled-key\nenabled-key", `["disabled-key","enabled-key"]`} {
		t.Run(keys, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer enabled-key" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = w.Write([]byte(`{"id":"asset-one"}`))
			}))
			defer server.Close()
			ch := seedanceResellerChannel(server.URL, keys)
			ch.ChannelInfo.IsMultiKey = true
			ch.ChannelInfo.MultiKeySize = 2
			ch.ChannelInfo.MultiKeyStatusList = map[int]int{0: common.ChannelStatusManuallyDisabled}
			for _, method := range []string{http.MethodPost, http.MethodGet} {
				_, status, err := doTemporaryAssetRequest(ch, method, "/v1/assets", strings.NewReader(`{}`))
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, status)
			}
		})
	}
}

func TestTemporaryAssetUsesByteDanceSeedanceChannelOnReseller(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	direct := &model.Channel{Type: constant.ChannelTypeStarAI, Status: common.ChannelStatusEnabled, Name: "direct", Key: "direct-key"}
	reseller := &model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusEnabled, Name: "reseller", Key: "instance-key"}
	require.NoError(t, db.Create(direct).Error)
	require.NoError(t, db.Create(reseller).Error)

	selected, err := resolveTemporaryAssetChannel(nil)
	require.NoError(t, err)
	require.Equal(t, reseller.Id, selected.Id)

	legacy, err := resolveTemporaryAssetChannel(&service.StarAIAssetBinding{ChannelID: direct.Id})
	require.NoError(t, err)
	require.Equal(t, direct.Id, legacy.Id)
}

func TestTemporaryAssetResellerFailureDoesNotExposeUpstreamDiagnostics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	writeStarAIAssetUpstreamFailure(ctx, &model.Channel{Id: 7, Type: constant.ChannelTypeByteDanceSeedance}, &starAIAssetUpstreamFailure{
		Operation: "create", Status: http.StatusBadRequest, Code: "private_code", Reason: "private account diagnostic",
	})
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "private")
}

func TestTemporaryAssetDoesNotCrossProviderWhenOriginalChannelDisabled(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	disabled := &model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusManuallyDisabled, Name: "old", Key: "old-key"}
	direct := &model.Channel{Type: constant.ChannelTypeStarAI, Status: common.ChannelStatusEnabled, Name: "direct", Key: "direct-key"}
	require.NoError(t, db.Create(disabled).Error)
	require.NoError(t, db.Create(direct).Error)
	_, err := resolveTemporaryAssetChannel(&service.StarAIAssetBinding{ChannelID: disabled.Id, ChannelType: constant.ChannelTypeByteDanceSeedance})
	require.Error(t, err)
}

func TestTemporaryAssetQueryRejectsOtherLocalUserBeforeUpstream(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusEnabled, Name: "reseller", Key: "instance-key", BaseURL: &baseURL}
	require.NoError(t, db.Create(channel).Error)
	binding := &service.StarAIAssetBinding{UpstreamID: "asset-reseller", UserID: 42, ChannelID: channel.Id, ChannelType: constant.ChannelTypeByteDanceSeedance, Status: "ACTIVE"}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/assets/asset-reseller", nil)
	ctx.Params = gin.Params{{Key: "id", Value: binding.ID}}
	ctx.Set("id", 84)
	GetStarAIAsset(ctx)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Zero(t, calls.Load())

	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/v1/assets/asset-reseller", nil)
	ctx.Params = gin.Params{{Key: "id", Value: binding.ID}}
	ctx.Set("id", 84)
	DeleteStarAIAsset(ctx)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Zero(t, calls.Load())
	_, err := service.GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
}

func TestTemporaryAssetResellerQueryNotFoundDoesNotExpireLocalAsset(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusEnabled, Name: "reseller", Key: "different-account", BaseURL: &baseURL}
	require.NoError(t, db.Create(channel).Error)
	binding := &service.StarAIAssetBinding{UpstreamID: "asset-reseller", UserID: 42, ChannelID: channel.Id, ChannelType: constant.ChannelTypeByteDanceSeedance, Status: "ACTIVE"}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))
	_, err := refreshStarAIAsset(nil, binding)
	require.Error(t, err)
	stored, err := service.GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.Equal(t, "ACTIVE", stored.Status)
}

func TestTemporaryAssetResellerHTTP200FailureDoesNotPersistPrivateDiagnostics(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	expiresAt := time.Now().Add(time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"id":"asset-failed","status":"FAILED","expires_at":%d,"error":{"code":"private_code","message":"private account diagnostic"}}`, expiresAt)
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	require.NoError(t, db.Create(&model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusEnabled, Name: "reseller", Key: "account-key", BaseURL: &baseURL}).Error)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	ctx.Set("id", 42)
	binding, ok := createStarAIAssetUpstream(ctx, createStarAIAssetRequest{AssetType: "image", Name: "reference"}, &service.StarAIAssetBinding{})
	require.True(t, ok)
	require.NotContains(t, binding.ErrorCode, "private")
	require.NotContains(t, binding.ErrorMessage, "private")
	require.NotContains(t, recorder.Body.String(), "private")
	response := safeStarAIAsset(binding)
	require.NotNil(t, response.Error)
	require.NotContains(t, response.Error.Code, "private")
	require.NotContains(t, response.Error.Message, "private")
	_, err := refreshStarAIAsset(ctx, binding)
	require.NoError(t, err)
	stored, err := service.GetStarAIAssetBinding(binding.ID, 42)
	require.NoError(t, err)
	require.NotContains(t, stored.ErrorCode, "private")
	require.NotContains(t, stored.ErrorMessage, "private")
}

func TestTemporaryAssetResellerRefreshRemovesPastExpiry(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	past := time.Now().Add(-time.Second).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"status":"ACTIVE","expires_at":%d}`, past)
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusEnabled, Name: "reseller", Key: "account-key", BaseURL: &baseURL}
	require.NoError(t, db.Create(channel).Error)
	binding := &service.StarAIAssetBinding{UpstreamID: "asset-expired", UserID: 42, ChannelID: channel.Id, ChannelType: constant.ChannelTypeByteDanceSeedance, Status: "ACTIVE"}
	require.NoError(t, service.SaveStarAIAssetBinding(binding))
	_, err := refreshStarAIAsset(nil, binding)
	require.ErrorIs(t, err, service.ErrStarAIAssetExpired)
	_, err = service.GetStarAIAssetBinding(binding.ID, 42)
	require.ErrorIs(t, err, service.ErrStarAIAssetNotFound)
}

func TestTemporaryAssetResellerMissingUpstreamExpiryUses168HourCap(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	previousTTL := constant.StarAIAssetTTLHours
	constant.StarAIAssetTTLHours = 1000
	t.Cleanup(func() { constant.StarAIAssetTTLHours = previousTTL })
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"asset-unknown-expiry","status":"PROCESSING"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ACTIVE"}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	require.NoError(t, db.Create(&model.Channel{Type: constant.ChannelTypeByteDanceSeedance, Status: common.ChannelStatusEnabled, Name: "reseller", Key: "account-key", BaseURL: &baseURL}).Error)
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	ctx.Set("id", 42)
	binding, ok := createStarAIAssetUpstream(ctx, createStarAIAssetRequest{AssetType: "image", Name: "reference"}, &service.StarAIAssetBinding{})
	require.True(t, ok)
	require.Equal(t, int32(2), calls.Load())
	require.LessOrEqual(t, binding.ExpiresAt, time.Now().Add(168*time.Hour).Unix())
}

func TestTemporaryAssetDirectCreateDoesNotRequireExpiryRefresh(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	useControllerStarAIAssetRedis(t)
	var getCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			getCalls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"id":"asset-direct","status":"ACTIVE"}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	require.NoError(t, db.Create(&model.Channel{Type: constant.ChannelTypeStarAI, Status: common.ChannelStatusEnabled, Name: "direct", Key: "direct-key", BaseURL: &baseURL}).Error)
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/assets", nil)
	ctx.Set("id", 42)
	binding, ok := createStarAIAssetUpstream(ctx, createStarAIAssetRequest{AssetType: "image", Name: "reference"}, &service.StarAIAssetBinding{})
	require.True(t, ok)
	require.Equal(t, "asset-direct", binding.ID)
	require.Zero(t, getCalls.Load())
}

func TestTemporaryAssetDirectRefreshIgnoresUpstreamExpiresAt(t *testing.T) {
	for _, channelType := range []int{0, constant.ChannelTypeStarAI} {
		t.Run(fmt.Sprintf("channel-type-%d", channelType), func(t *testing.T) {
			db := setupSingleStarAIChannelTestDB(t)
			useControllerStarAIAssetRedis(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprintf(w, `{"status":"ACTIVE","expires_at":%d}`, time.Now().Add(-time.Hour).Unix())
			}))
			t.Cleanup(server.Close)
			baseURL := server.URL
			channel := &model.Channel{Type: constant.ChannelTypeStarAI, Status: common.ChannelStatusEnabled, Name: "direct", Key: "direct-key", BaseURL: &baseURL}
			require.NoError(t, db.Create(channel).Error)
			binding := &service.StarAIAssetBinding{UpstreamID: "asset-direct", UserID: 42, ChannelID: channel.Id, ChannelType: channelType, Status: "PROCESSING"}
			require.NoError(t, service.SaveStarAIAssetBinding(binding))
			originalExpiry := binding.ExpiresAt
			_, err := refreshStarAIAsset(nil, binding)
			require.NoError(t, err)
			require.Equal(t, "ACTIVE", binding.Status)
			require.Equal(t, originalExpiry, binding.ExpiresAt)
			stored, err := service.GetStarAIAssetBinding(binding.ID, 42)
			require.NoError(t, err)
			require.Equal(t, originalExpiry, stored.ExpiresAt)
		})
	}
}
