package service

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func useStarAICOSTestConfig(t *testing.T) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{
		"COSEnabled":             "true",
		"COSBucket":              "molii-assets-1250000000",
		"COSRegion":              "ap-guangzhou",
		"COSSecretID":            "AKIDTEST",
		"COSSecretKey":           "SECRETTEST",
		"COSPathPrefix":          "users",
		"COSUploadExpiryMinutes": "30",
		"COSReadExpiryMinutes":   "60",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestBeginStarAICOSUploadScopesAuthorizationToOneUserObject(t *testing.T) {
	useStarAIAssetRedis(t)
	useStarAICOSTestConfig(t)

	authorization, err := BeginStarAICOSUpload(context.Background(), 42, "opening.mp4", "video/mp4", "video", "opening", 1024)
	require.NoError(t, err)
	require.NotEmpty(t, authorization.UploadID)
	require.Equal(t, "video/mp4", authorization.Headers["Content-Type"])
	parsed, err := url.Parse(authorization.UploadURL)
	require.NoError(t, err)
	require.Equal(t, "molii-assets-1250000000.cos.ap-guangzhou.myqcloud.com", parsed.Host)
	require.Contains(t, parsed.RawQuery, "q-signature")

	intent, err := GetStarAICOSUploadIntent(context.Background(), authorization.UploadID, 42)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(intent.ObjectKey, "users/42/starai-assets/video/"))
	require.True(t, strings.HasSuffix(intent.ObjectKey, ".mp4"))
	require.Equal(t, int64(1024), intent.FileSize)
	require.Greater(t, intent.ExpiresAt, time.Now().Unix())

	_, err = GetStarAICOSUploadIntent(context.Background(), authorization.UploadID, 7)
	require.ErrorIs(t, err, ErrStarAIAssetForbidden)
}

func TestValidateStarAICOSUploadEnforcesTypeAndProviderSizeLimits(t *testing.T) {
	require.Error(t, ValidateStarAICOSUpload("reference.HEIC", "image/heic", "image", 1024))
	require.Error(t, ValidateStarAICOSUpload("payload.zip", "application/zip", "image", 1024))
	require.NoError(t, ValidateStarAICOSUpload("clip.mp4", "video/mp4", "video", 50*1024*1024))
	require.Error(t, ValidateStarAICOSUpload("clip.mp4", "video/mp4", "video", 50*1024*1024+1))
	require.Error(t, ValidateStarAICOSUpload("voice.mp3", "video/mp4", "audio", 1024))
}
