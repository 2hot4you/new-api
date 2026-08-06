package service

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const VideoPlaybackURLTTL = 24 * time.Hour

var ErrVideoPlaybackSignatureInvalid = errors.New("video playback signature is invalid")

func videoPlaybackSignature(taskID string, userID int, expiresAt int64) string {
	payload := fmt.Sprintf("video-playback/v1\n%s\n%d\n%d", taskID, userID, expiresAt)
	return common.GenerateHMACWithKey([]byte(common.SessionSecret), payload)
}

// BuildSignedVideoProxyPath creates a same-origin playback path for dashboard
// pages. Keeping the path relative prevents a dashboard opened on a local or
// alternate domain from sending its task preview to ServerAddress instead.
func BuildSignedVideoProxyPath(taskID string, userID int) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || userID <= 0 {
		return ""
	}
	expiresAt := time.Now().Add(VideoPlaybackURLTTL).Unix()
	query := url.Values{}
	query.Set("expires", strconv.FormatInt(expiresAt, 10))
	query.Set("user_id", strconv.Itoa(userID))
	query.Set("signature", videoPlaybackSignature(taskID, userID, expiresAt))
	return fmt.Sprintf("/v1/videos/%s/content?%s",
		url.PathEscape(taskID),
		query.Encode(),
	)
}

// BuildSignedVideoProxyURL creates a short-lived absolute Molii URL that can
// be returned to API clients without forwarding the user's API key.
func BuildSignedVideoProxyURL(taskID string, userID int) string {
	path := BuildSignedVideoProxyPath(taskID, userID)
	if path == "" {
		return ""
	}
	return strings.TrimRight(system_setting.ServerAddress, "/") + path
}

func VerifyVideoPlaybackSignature(taskID, rawUserID, rawExpiresAt, signature string, now time.Time) (int, error) {
	taskID = strings.TrimSpace(taskID)
	userID, userErr := strconv.Atoi(strings.TrimSpace(rawUserID))
	expiresAt, expiresErr := strconv.ParseInt(strings.TrimSpace(rawExpiresAt), 10, 64)
	if taskID == "" || userErr != nil || userID <= 0 || expiresErr != nil || expiresAt <= now.Unix() || strings.TrimSpace(signature) == "" {
		return 0, ErrVideoPlaybackSignatureInvalid
	}
	expected := videoPlaybackSignature(taskID, userID, expiresAt)
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return 0, ErrVideoPlaybackSignatureInvalid
	}
	return userID, nil
}
