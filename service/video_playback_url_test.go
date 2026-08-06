package service

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignedVideoProxyURLRoundTrip(t *testing.T) {
	previousSecret := common.SessionSecret
	previousAddress := system_setting.ServerAddress
	common.SessionSecret = "video-playback-test-secret"
	system_setting.ServerAddress = "https://molii.example/"
	t.Cleanup(func() {
		common.SessionSecret = previousSecret
		system_setting.ServerAddress = previousAddress
	})

	raw := BuildSignedVideoProxyURL("task_public_123", 42)
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Equal(t, "https://molii.example/v1/videos/task_public_123/content", parsed.Scheme+"://"+parsed.Host+parsed.Path)

	query := parsed.Query()
	expiresAt, err := strconv.ParseInt(query.Get("expires"), 10, 64)
	require.NoError(t, err)
	userID, err := VerifyVideoPlaybackSignature("task_public_123", query.Get("user_id"), query.Get("expires"), query.Get("signature"), time.Unix(expiresAt-1, 0))
	require.NoError(t, err)
	assert.Equal(t, 42, userID)
}

func TestSignedVideoProxyPathRoundTrip(t *testing.T) {
	previousSecret := common.SessionSecret
	common.SessionSecret = "video-playback-test-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	raw := BuildSignedVideoProxyPath("task_dashboard_123", 42)
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	assert.Empty(t, parsed.Scheme)
	assert.Empty(t, parsed.Host)
	assert.Equal(t, "/v1/videos/task_dashboard_123/content", parsed.Path)

	query := parsed.Query()
	expiresAt, err := strconv.ParseInt(query.Get("expires"), 10, 64)
	require.NoError(t, err)
	userID, err := VerifyVideoPlaybackSignature("task_dashboard_123", query.Get("user_id"), query.Get("expires"), query.Get("signature"), time.Unix(expiresAt-1, 0))
	require.NoError(t, err)
	assert.Equal(t, 42, userID)
}

func TestVideoPlaybackSignatureRejectsTamperingAndExpiry(t *testing.T) {
	previousSecret := common.SessionSecret
	common.SessionSecret = "video-playback-test-secret"
	t.Cleanup(func() { common.SessionSecret = previousSecret })

	expiresAt := time.Now().Add(time.Hour).Unix()
	signature := videoPlaybackSignature("task_public_123", 42, expiresAt)
	_, err := VerifyVideoPlaybackSignature("task_other", "42", strconv.FormatInt(expiresAt, 10), signature, time.Now())
	assert.ErrorIs(t, err, ErrVideoPlaybackSignatureInvalid)

	_, err = VerifyVideoPlaybackSignature("task_public_123", "42", strconv.FormatInt(expiresAt, 10), strings.Repeat("0", len(signature)), time.Now())
	assert.ErrorIs(t, err, ErrVideoPlaybackSignatureInvalid)

	_, err = VerifyVideoPlaybackSignature("task_public_123", "42", strconv.FormatInt(expiresAt, 10), signature, time.Unix(expiresAt, 0))
	assert.ErrorIs(t, err, ErrVideoPlaybackSignatureInvalid)
}
