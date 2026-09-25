package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/seedanceprotocol"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const VideoStudioTokenIDHeader = "X-Video-Studio-Token-ID"
const VideoStudioRequestIDHeader = "X-Video-Studio-Request-ID"

const videoStudioIdempotencyTTL = 24 * time.Hour

var (
	videoStudioRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,80}$`)
	videoStudioLocalRequests    = struct {
		sync.Mutex
		expiresAt map[string]time.Time
	}{expiresAt: make(map[string]time.Time)}
)

// VideoStudioTokenAuth turns an owned dashboard token selection into the same
// authorization context used by the public relay API. The token secret is read
// server-side and is never returned to the browser.
func VideoStudioTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("id")
		tokenID, err := strconv.Atoi(strings.TrimSpace(c.GetHeader(VideoStudioTokenIDHeader)))
		if err != nil || tokenID <= 0 {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "请选择可用的 API 密钥")
			return
		}
		token, err := model.GetTokenByIds(tokenID, userID)
		if err != nil || token == nil {
			abortWithOpenAiMessage(c, http.StatusForbidden, "API 密钥不可用")
			return
		}

		originalAuthorization := c.GetHeader("Authorization")
		c.Request.Header.Set("Authorization", "Bearer sk-"+token.Key)
		TokenAuth()(c)
		if originalAuthorization == "" {
			c.Request.Header.Del("Authorization")
		} else {
			c.Request.Header.Set("Authorization", originalAuthorization)
		}
	}
}

// VideoStudioIdempotency prevents a retry or a double click from submitting
// the same long-running generation twice. Redis coordinates multiple app
// instances; the in-process fallback keeps local development safe as well.
func VideoStudioIdempotency() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(VideoStudioRequestIDHeader))
		if !videoStudioRequestIDPattern.MatchString(requestID) {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "请求标识无效，请刷新页面后重试")
			return
		}
		key := videoStudioIdempotencyKey(c.GetInt("id"), requestID)
		acquired, err := acquireVideoStudioRequest(c.Request.Context(), key, time.Now())
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "无法确认请求状态，请稍后重试")
			return
		}
		if !acquired {
			abortWithOpenAiMessage(c, http.StatusConflict, "该视频生成请求已提交，请勿重复操作")
			return
		}

		c.Next()
		if c.Writer.Status() >= http.StatusBadRequest {
			releaseVideoStudioRequest(c.Request.Context(), key)
		}
	}
}

func videoStudioIdempotencyKey(userID int, requestID string) string {
	digest := sha256.Sum256([]byte(strconv.Itoa(userID) + ":" + requestID))
	return "video-studio:idempotency:" + hex.EncodeToString(digest[:])
}

func acquireVideoStudioRequest(ctx context.Context, key string, now time.Time) (bool, error) {
	if common.RedisEnabled && common.RDB != nil {
		return common.RDB.SetNX(ctx, key, "1", videoStudioIdempotencyTTL).Result()
	}
	videoStudioLocalRequests.Lock()
	defer videoStudioLocalRequests.Unlock()
	if expiresAt, ok := videoStudioLocalRequests.expiresAt[key]; ok && now.Before(expiresAt) {
		return false, nil
	}
	videoStudioLocalRequests.expiresAt[key] = now.Add(videoStudioIdempotencyTTL)
	return true, nil
}

func releaseVideoStudioRequest(ctx context.Context, key string) {
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.Del(ctx, key).Err()
		return
	}
	videoStudioLocalRequests.Lock()
	delete(videoStudioLocalRequests.expiresAt, key)
	videoStudioLocalRequests.Unlock()
}

// PrepareVideoStudioRequest validates the same wire payload consumed by the
// Seedance adaptors, captures a reusable user-owned snapshot, and temporarily
// maps the BFF route to the native /v1/videos route for existing middleware.
func PrepareVideoStudioRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		var payload seedanceprotocol.Payload
		if err := common.UnmarshalBodyReusable(c, &payload); err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "视频生成参数无效")
			return
		}
		if _, supported := seedanceprotocol.CapabilitiesForModel(payload.Model); !supported {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "该页面仅支持 Seedance 视频模型")
			return
		}
		if err := seedanceprotocol.ValidateDuration(payload.Model, payload.Duration); err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, err.Error())
			return
		}
		if err := seedanceprotocol.ValidatePayload(&payload); err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, err.Error())
			return
		}

		snapshot, err := buildVideoStudioSnapshot(c, &payload)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, err.Error())
			return
		}
		service.SetVideoStudioRequestSnapshot(c, snapshot)
		originalPath := c.Request.URL.Path
		c.Request.URL.Path = "/v1/videos"
		c.Next()
		c.Request.URL.Path = originalPath
	}
}

func buildVideoStudioSnapshot(c *gin.Context, payload *seedanceprotocol.Payload) (*model.VideoStudioRequestSnapshot, error) {
	snapshot := &model.VideoStudioRequestSnapshot{
		Version:    1,
		RequestID:  strings.TrimSpace(c.GetHeader(VideoStudioRequestIDHeader)),
		Model:      payload.Model,
		Resolution: payload.Resolution,
		Ratio:      payload.Ratio,
	}
	if payload.Duration != nil {
		snapshot.Duration = *payload.Duration
	}
	if payload.GenerateAudio != nil {
		snapshot.GenerateAudio = *payload.GenerateAudio
	}
	if payload.Watermark != nil {
		snapshot.Watermark = *payload.Watermark
	}
	for _, tool := range payload.Tools {
		if tool.Type == "web_search" {
			snapshot.WebSearch = true
		}
	}
	for _, item := range payload.Content {
		if item.Type == "text" {
			if snapshot.Prompt == "" {
				snapshot.Prompt = strings.TrimSpace(item.Text)
			} else {
				snapshot.Prompt += "\n" + strings.TrimSpace(item.Text)
			}
			continue
		}
		mediaType, rawURL := videoStudioMediaValue(item)
		if rawURL == "" {
			continue
		}
		reference := model.VideoStudioMediaReference{Type: mediaType, Role: item.Role, Source: "url", URL: rawURL}
		if assetID, ok := strings.CutPrefix(rawURL, "asset://"); ok {
			binding, err := service.GetStarAIAssetBinding(assetID, c.GetInt("id"))
			if err != nil || binding == nil {
				return nil, service.ErrStarAIAssetNotReady
			}
			reference.Source = "asset"
			reference.AssetID = binding.ID
			reference.URL = ""
			reference.Name = binding.Name
			reference.ExpiresAt = binding.ExpiresAt
		}
		snapshot.Media = append(snapshot.Media, reference)
	}
	return snapshot, nil
}

func videoStudioMediaValue(item seedanceprotocol.ContentItem) (string, string) {
	switch item.Type {
	case "image_url":
		if item.ImageURL != nil {
			return "image", strings.TrimSpace(item.ImageURL.URL)
		}
	case "video_url":
		if item.VideoURL != nil {
			return "video", strings.TrimSpace(item.VideoURL.URL)
		}
	case "audio_url":
		if item.AudioURL != nil {
			return "audio", strings.TrimSpace(item.AudioURL.URL)
		}
	}
	return "", ""
}
