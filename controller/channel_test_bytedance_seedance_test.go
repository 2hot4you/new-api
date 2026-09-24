package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func seedanceResellerChannel(baseURL, key string) *model.Channel {
	return &model.Channel{
		Type: constant.ChannelTypeByteDanceSeedance, BaseURL: &baseURL,
		Key: key, Name: "reseller", Group: "default",
	}
}

func TestByteDanceSeedanceFetchModelsFiltersAuthorizationResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "Bearer instance-key", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"data":[{"id":"doubao-seedance-2-5-260628"},{"id":"gpt-5.6-sol"},{"id":"doubao-seedance-unknown"}]}`))
	}))
	defer server.Close()

	got, err := fetchChannelUpstreamModelIDs(seedanceResellerChannel(server.URL+"/", "instance-key"))
	require.NoError(t, err)
	require.Equal(t, []string{"doubao-seedance-2-5-260628"}, got)
}

func TestByteDanceSeedanceConnectionTestCannotOverrideConfiguredBearer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer instance-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"doubao-seedance-2-5-260628"}]}`))
	}))
	defer server.Close()
	channel := seedanceResellerChannel(server.URL, "instance-key")
	override := `{"Authorization":"Bearer replacement-key"}`
	channel.HeaderOverride = &override

	result := testChannel(context.Background(), channel, 0, "", "", false)
	require.NoError(t, result.localErr)
}

func TestByteDanceSeedanceFetchModelsRejectsInvalidAndSelfBaseURL(t *testing.T) {
	oldAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://reseller.example:443/"
	t.Cleanup(func() { system_setting.ServerAddress = oldAddress })
	for _, baseURL := range []string{"", "reseller.example", "ftp://reseller.example"} {
		t.Run(baseURL, func(t *testing.T) {
			_, err := fetchChannelUpstreamModelIDs(seedanceResellerChannel(baseURL, "instance-key"))
			require.ErrorContains(t, err, "absolute HTTP(S) Base URL")
		})
	}
	_, err := fetchChannelUpstreamModelIDs(seedanceResellerChannel("https://reseller.example", "instance-key"))
	require.ErrorContains(t, err, "cannot point to this instance")
}

func TestByteDanceSeedanceFetchModelsRejectsAlternateSelfAddresses(t *testing.T) {
	oldAddress := system_setting.ServerAddress
	t.Cleanup(func() { system_setting.ServerAddress = oldAddress })
	tests := []struct {
		name, serverAddress, upstreamAddress string
	}{
		{"alternate scheme", "https://reseller.example", "http://reseller.example"},
		{"IPv4 loopback alias", "http://localhost:3000", "http://127.0.0.1:3000"},
		{"IPv6 loopback alias", "http://localhost:3000", "http://[::1]:3000"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			system_setting.ServerAddress = test.serverAddress
			_, err := fetchChannelUpstreamModelIDs(seedanceResellerChannel(test.upstreamAddress, "instance-key"))
			require.ErrorContains(t, err, "cannot point to this instance")
		})
	}
}

func TestByteDanceSeedanceFetchModelsPermitsSameHostDifferentPort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":"doubao-seedance-2-5-260628"}]}`))
	}))
	defer server.Close()
	oldAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "http://127.0.0.1:443"
	t.Cleanup(func() { system_setting.ServerAddress = oldAddress })

	models, err := fetchChannelUpstreamModelIDs(seedanceResellerChannel(server.URL, "instance-key"))
	require.NoError(t, err)
	require.Equal(t, []string{"doubao-seedance-2-5-260628"}, models)
}

func TestByteDanceSeedanceFetchModelsRejectsZeroPaddedSelfPort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"doubao-seedance-2-5-260628"}]}`))
	}))
	defer server.Close()
	oldAddress := system_setting.ServerAddress
	system_setting.ServerAddress = server.URL
	t.Cleanup(func() { system_setting.ServerAddress = oldAddress })
	portSeparator := strings.LastIndex(server.URL, ":")
	paddedURL := server.URL[:portSeparator+1] + "0" + server.URL[portSeparator+1:]

	_, err := fetchChannelUpstreamModelIDs(seedanceResellerChannel(paddedURL, "instance-key"))
	require.ErrorContains(t, err, "cannot point to this instance")
}

func TestByteDanceSeedanceChannelTestUsesOnlyAuthorizedModelList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v1/models", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":"doubao-seedance-2-0-260128"}]}`))
	}))
	defer server.Close()

	result := testChannel(context.Background(), seedanceResellerChannel(server.URL, "instance-key"), 0, "", "", false)
	require.NoError(t, result.localErr)
	require.Contains(t, result.successMessage, "模型")
}

func TestByteDanceSeedanceChannelTestFailureIsVisibleToHealthCheck(t *testing.T) {
	channel := seedanceResellerChannel("", "instance-key")
	result := testChannel(context.Background(), channel, 0, "", "", false)
	require.Error(t, result.localErr)
	require.NotNil(t, result.newAPIError)
	require.NotNil(t, result.context)
}

func TestByteDanceSeedanceRefreshRevokesModelsAndKeepsLocalDisableAndOptions(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"doubao-seedance-2-5-260628"},{"id":"doubao-seedance-2-0-fast-260128"}]}`))
	}))
	defer server.Close()

	channel := seedanceResellerChannel(server.URL, "instance-key")
	channel.Models = "doubao-seedance-2-0-260128,doubao-seedance-2-5-260628"
	channel.Other = `{"pricing":"local-only","currency":"CNY"}`
	channel.SetSetting(dto.ChannelSettings{HTTPProtocol: dto.HTTPProtocolHTTP1})
	settingBefore := *channel.Setting
	settings := dto.ChannelOtherSettings{
		UpstreamModelUpdateCheckEnabled: true, UpstreamModelUpdateAutoSyncEnabled: true,
		UpstreamModelUpdateIgnoredModels: []string{"doubao-seedance-2-0-fast-260128"},
	}
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Create(channel).Error)

	changed, added, err := checkAndPersistChannelUpstreamModelUpdates(channel, &settings, true, true)
	require.NoError(t, err)
	require.True(t, changed)
	require.Zero(t, added)
	reloaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, "doubao-seedance-2-5-260628", reloaded.Models)
	require.Equal(t, `{"pricing":"local-only","currency":"CNY"}`, reloaded.Other)
	require.Equal(t, settingBefore, *reloaded.Setting)
	require.Empty(t, reloaded.GetOtherSettings().UpstreamModelUpdateLastRemovedModels)
}

func TestByteDanceSeedanceScheduledRevocationReportsRemovedModel(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"doubao-seedance-2-5-260628"}]}`))
	}))
	defer server.Close()
	channel := seedanceResellerChannel(server.URL, "instance-key")
	channel.Models = "doubao-seedance-2-0-260128,doubao-seedance-2-5-260628"
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		UpstreamModelUpdateCheckEnabled: true, UpstreamModelUpdateAutoSyncEnabled: true,
	})
	require.NoError(t, db.Create(channel).Error)

	summary := runChannelUpstreamModelUpdateTaskOnce(context.Background(), true, true, nil)
	require.Equal(t, 1, summary.CheckedChannels)
	require.Equal(t, 1, summary.ChangedChannels)
	require.Equal(t, 1, summary.DetectedRemoveModels)
	require.Zero(t, summary.FailedChannels)
	reloaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, "doubao-seedance-2-5-260628", reloaded.Models)
	require.Empty(t, reloaded.GetOtherSettings().UpstreamModelUpdateLastRemovedModels)
}

func TestByteDanceSeedanceFailedRefreshKeepsExistingModels(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()
	channel := seedanceResellerChannel(server.URL, "instance-key")
	channel.Models = "doubao-seedance-2-0-260128"
	settings := dto.ChannelOtherSettings{UpstreamModelUpdateCheckEnabled: true, UpstreamModelUpdateAutoSyncEnabled: true}
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Create(channel).Error)

	changed, added, err := checkAndPersistChannelUpstreamModelUpdates(channel, &settings, true, true)
	require.Error(t, err)
	require.False(t, changed)
	require.Zero(t, added)
	reloaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, "doubao-seedance-2-0-260128", reloaded.Models)
	require.Empty(t, reloaded.GetOtherSettings().UpstreamModelUpdateLastRemovedModels)
}

func TestByteDanceSeedanceUnsupportedOnlyRefreshKeepsExistingModels(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.6-sol"},{"id":"doubao-seedance-unknown"}]}`))
	}))
	defer server.Close()
	channel := seedanceResellerChannel(server.URL, "instance-key")
	channel.Models = "doubao-seedance-2-0-260128"
	settings := dto.ChannelOtherSettings{UpstreamModelUpdateCheckEnabled: true, UpstreamModelUpdateAutoSyncEnabled: true}
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Create(channel).Error)

	_, _, err := checkAndPersistChannelUpstreamModelUpdates(channel, &settings, true, true)
	require.ErrorContains(t, err, "supported")
	reloaded, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, "doubao-seedance-2-0-260128", reloaded.Models)
}
