package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// videoProxyError returns a standardized OpenAI-style error response.
func videoProxyError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func videoProxyCodedError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
			"type":    "server_error",
		},
	})
}

type storedVideoFetcher func(context.Context, string, string, string) (*http.Response, error)

func VideoProxy(c *gin.Context) {
	videoProxyWithStoredFetcher(c, service.FetchCOSObject)
}

func videoProxyWithStoredFetcher(c *gin.Context, fetchStored storedVideoFetcher) {
	taskID := c.Param("task_id")
	if taskID == "" {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	userID := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}

	if task.Status != model.TaskStatusSuccess {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("Task is not completed yet, current status: %s", task.Status))
		return
	}
	if task.PrivateData.StoredResult != nil {
		serveStoredGrokVideo(c, task, fetchStored)
		return
	}

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to get channel for task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	var videoURL string
	proxy := channel.GetSetting().Proxy
	client := service.GetSSRFProtectedHTTPClient()
	if proxy != "" {
		// 渠道代理路径的连接由代理侧建立，无法做拨号时逐 IP 校验，
		// 因此后面对 videoURL 保留请求前的一次性 SSRF 校验。
		client, err = service.GetHttpClientWithProxy(proxy)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create proxy client for task %s", taskID))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy client")
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "", nil)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create request: %s", err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}

	switch channel.Type {
	case constant.ChannelTypeGemini:
		apiKey := task.PrivateData.Key
		if apiKey == "" {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Missing stored API key for Gemini task %s", taskID))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "API key not stored for task")
			return
		}
		videoURL, err = getGeminiVideoURL(channel, task, apiKey)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Gemini video URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Gemini video URL")
			return
		}
		req.Header.Set("x-goog-api-key", apiKey)
	case constant.ChannelTypeVertexAi:
		videoURL, err = getVertexVideoURL(channel, task)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Vertex video URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Vertex video URL")
			return
		}
	case constant.ChannelTypeOpenAI, constant.ChannelTypeSora:
		videoURL = fmt.Sprintf("%s/v1/videos/%s/content", baseURL, task.GetUpstreamTaskID())
		req.Header.Set("Authorization", "Bearer "+channel.Key)
	default:
		// Video URL is stored in PrivateData.ResultURL (fallback to FailReason for old data)
		videoURL = task.GetResultURL()
	}

	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL is empty for task %s", taskID))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	if channel.Type == constant.ChannelTypeMoliiGrokAIGC {
		parsedResultURL, parseErr := url.Parse(videoURL)
		if parseErr != nil || !strings.EqualFold(parsedResultURL.Scheme, "https") {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("Molii Grok Imagine API result URL rejected: public_task_id=%s reason=https_required", taskID))
			videoProxyCodedError(c, http.StatusBadGateway, "upstream_invalid_result_url", "Molii Grok Imagine API video result must use HTTPS")
			return
		}
	}
	if channel.Type == constant.ChannelTypeStarAI && service.IsUnsignedStarAIPrivateTOSURL(videoURL) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf(
			"Molii Volcengine Imagine API result URL rejected: public_task_id=%s reason=unsigned_private_tos host=%s",
			taskID, "ark-acg-cn-beijing.tos-cn-beijing.volces.com",
		))
		videoProxyCodedError(c, http.StatusBadGateway, "upstream_invalid_result_url",
			"Molii Volcengine Imagine API returned an unsigned private TOS result URL")
		return
	}

	if strings.HasPrefix(videoURL, "data:") {
		if err := writeVideoDataURL(c, videoURL); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to decode video data URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		}
		return
	}

	trustedSignedStarAITOSURL := channel.Type == constant.ChannelTypeStarAI && service.IsSignedStarAIPrivateTOSURL(videoURL)
	trustedMoliiGrokVideoURL := channel.Type == constant.ChannelTypeMoliiGrokAIGC && service.IsTrustedMoliiGrokVideoURL(videoURL)
	if (trustedSignedStarAITOSURL || trustedMoliiGrokVideoURL) && proxy == "" {
		// These exact result hosts are operator-controlled and TLS-verified. Use
		// the normal relay client for these narrowly trusted URLs so local proxy
		// DNS modes (198.18.0.0/15) do not trip the protected dialer's rejection.
		client = service.GetHttpClient()
	}
	var validateErr error
	if !trustedSignedStarAITOSURL && !trustedMoliiGrokVideoURL {
		if proxy == "" {
			validateErr = service.ValidateSSRFProtectedFetchURL(videoURL)
		} else {
			fetchSetting := system_setting.GetFetchSetting()
			validateErr = common.ValidateURLWithFetchSetting(videoURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain)
		}
	}
	if validateErr != nil {
		if channel.Type == constant.ChannelTypeMoliiGrokAIGC {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Molii Grok Imagine API video URL blocked for public task %s", taskID))
			videoProxyError(c, http.StatusForbidden, "server_error", "Molii Grok Imagine API video result was blocked")
		} else {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL blocked for task %s: url=%s", taskID, service.SanitizeURLForLog(videoURL)))
			videoProxyError(c, http.StatusForbidden, "server_error", fmt.Sprintf("request blocked: %v", validateErr))
		}
		return
	}

	req.URL, err = url.Parse(videoURL)
	if err != nil {
		if channel.Type == constant.ChannelTypeMoliiGrokAIGC {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to parse Molii Grok Imagine API video URL for public task %s", taskID))
		} else {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to parse video URL for task %s: url=%s", taskID, service.SanitizeURLForLog(videoURL)))
		}
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}
	if rangeHeader := strings.TrimSpace(c.GetHeader("Range")); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	if ifRangeHeader := strings.TrimSpace(c.GetHeader("If-Range")); ifRangeHeader != "" {
		req.Header.Set("If-Range", ifRangeHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		if channel.Type == constant.ChannelTypeMoliiGrokAIGC {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch Molii Grok Imagine API video for public task %s", taskID))
		} else {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch video for task %s: url=%s", taskID, service.SanitizeURLForLog(videoURL)))
		}
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		if channel.Type == constant.ChannelTypeMoliiGrokAIGC {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Molii Grok Imagine API video fetch returned status %d for public task %s", resp.StatusCode, taskID))
		} else {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Upstream returned status %d for task %s: url=%s", resp.StatusCode, taskID, service.SanitizeURLForLog(videoURL)))
		}
		videoProxyError(c, http.StatusBadGateway, "server_error",
			fmt.Sprintf("Upstream service returned status %d", resp.StatusCode))
		return
	}

	for key, values := range resp.Header {
		if channel.Type == constant.ChannelTypeMoliiGrokAIGC && !isMoliiGrokVideoResponseHeaderAllowed(key) {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	if channel.Type == constant.ChannelTypeMoliiGrokAIGC {
		applyMoliiGrokVideoResponseHeaders(c.Writer.Header())
	}
	if strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "video/") {
		c.Writer.Header().Set("Content-Disposition", "inline")
	}

	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream video content: %s", err.Error()))
	}
}

func serveStoredGrokVideo(c *gin.Context, task *model.Task, fetchStored storedVideoFetcher) {
	c.Writer.Header().Set("Cache-Control", "private, no-store")
	stored := task.PrivateData.StoredResult
	if stored == nil {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Video result is unavailable")
		return
	}
	if stored.ExpiresAt <= time.Now().Unix() {
		videoProxyCodedError(c, http.StatusGone, "result_expired", "Video result has expired")
		return
	}
	if stored.MIMEType != "video/mp4" || !service.IsOwnedGrokResultObject(task.UserId, "video", stored.ObjectKey) {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stored video metadata is invalid for public task %s", task.TaskID))
		videoProxyCodedError(c, http.StatusBadGateway, "stored_result_invalid", "Stored video result metadata is invalid")
		return
	}
	if fetchStored == nil {
		videoProxyCodedError(c, http.StatusBadGateway, "stored_result_unavailable", "Stored video result is unavailable")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	response, err := fetchStored(ctx, stored.ObjectKey, c.GetHeader("Range"), c.GetHeader("If-Range"))
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch stored video for public task %s", task.TaskID))
		videoProxyCodedError(c, http.StatusBadGateway, "stored_result_unavailable", "Stored video result is unavailable")
		return
	}
	if response == nil || response.Body == nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stored video fetch returned no response for public task %s", task.TaskID))
		videoProxyCodedError(c, http.StatusBadGateway, "stored_result_unavailable", "Stored video result is unavailable")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent && response.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stored video fetch returned status %d for public task %s", response.StatusCode, task.TaskID))
		videoProxyCodedError(c, http.StatusBadGateway, "stored_result_unavailable", "Stored video result is unavailable")
		return
	}

	for key, values := range response.Header {
		if !isMoliiGrokVideoResponseHeaderAllowed(key) {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	applyMoliiGrokVideoResponseHeaders(c.Writer.Header())
	c.Writer.WriteHeader(response.StatusCode)
	if response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		c.Writer.WriteHeaderNow()
		return
	}
	if _, err := io.Copy(c.Writer, response.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream stored video for public task %s", task.TaskID))
	}
}

func isMoliiGrokVideoResponseHeaderAllowed(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "content-length", "content-range", "accept-ranges", "etag", "last-modified":
		return true
	default:
		return false
	}
}

func applyMoliiGrokVideoResponseHeaders(header http.Header) {
	header.Set("Content-Type", "video/mp4")
	header.Set("Content-Disposition", "inline")
}

func writeVideoDataURL(c *gin.Context, dataURL string) error {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data url")
	}

	header := parts[0]
	payload := parts[1]
	if !strings.HasPrefix(header, "data:") || !strings.Contains(header, ";base64") {
		return fmt.Errorf("unsupported data url")
	}

	mimeType := strings.TrimPrefix(header, "data:")
	mimeType = strings.TrimSuffix(mimeType, ";base64")
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	videoBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		videoBytes, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return err
		}
	}

	c.Writer.Header().Set("Content-Type", mimeType)
	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write(videoBytes)
	return err
}
