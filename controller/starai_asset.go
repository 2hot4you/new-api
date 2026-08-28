package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type createStarAIAssetRequest struct {
	URL       string `json:"url"`
	AssetType string `json:"asset_type"`
	Name      string `json:"name"`
}

type updateStarAIAssetSourceURLRequest struct {
	URL string `json:"url"`
}

type createStarAICOSUploadRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	AssetType   string `json:"asset_type"`
	Name        string `json:"name"`
	FileSize    int64  `json:"file_size"`
}

type completeStarAICOSUploadRequest struct {
	UploadID string `json:"upload_id"`
}

type starAIAssetResponse struct {
	ID          string `json:"id"`
	UserID      int    `json:"user_id,omitempty"`
	Username    string `json:"username,omitempty"`
	AssetType   string `json:"asset_type"`
	Name        string `json:"name,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
	PreviewURL  string `json:"preview_url,omitempty"`
	SourceKind  string `json:"source_kind,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	FileSize    int64  `json:"file_size,omitempty"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
	VerifiedAt  int64  `json:"verified_at"`
}

type starAIAssetUpstreamResponse struct {
	ID        string                       `json:"id"`
	Status    string                       `json:"status"`
	AssetType string                       `json:"asset_type"`
	Data      *starAIAssetUpstreamResponse `json:"data,omitempty"`
}

type starAIAssetUpstreamFailure struct {
	Operation string
	Status    int
	Code      string
	Reason    string
	Cause     error
}

func (e *starAIAssetUpstreamFailure) Error() string {
	return e.Reason
}

var (
	starAIAssetURLPattern    = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
	starAIAssetSecretPattern = regexp.MustCompile(`(?i)(bearer\s+|sk-|(?:api[_-]?key|token|secret|authorization)[=:]\s*)[a-z0-9._-]+`)
	starAIAssetIDPattern     = regexp.MustCompile(`(?i)\b(?:asset|task)-[a-z0-9_-]{8,}\b`)
	starAIBrandPattern       = regexp.MustCompile(`(?i)\bstar[\s_-]*ai\b`)
)

func starAIAssetStringField(value any, keys ...string) string {
	wanted := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		wanted[strings.ToLower(key)] = struct{}{}
	}
	var visit func(any, int) string
	visit = func(current any, depth int) string {
		if depth > 4 {
			return ""
		}
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if _, ok := wanted[strings.ToLower(key)]; ok {
					if text, ok := child.(string); ok && strings.TrimSpace(text) != "" {
						return strings.TrimSpace(text)
					}
				}
			}
			for _, child := range typed {
				if text := visit(child, depth+1); text != "" {
					return text
				}
			}
		case []any:
			for _, child := range typed {
				if text := visit(child, depth+1); text != "" {
					return text
				}
			}
		}
		return ""
	}
	return visit(value, 0)
}

func sanitizeStarAIAssetErrorText(value string) string {
	value = starAIAssetURLPattern.ReplaceAllString(value, "[URL]")
	value = starAIAssetSecretPattern.ReplaceAllString(value, "[REDACTED]")
	value = starAIAssetIDPattern.ReplaceAllString(value, "[ID]")
	value = common.MaskSensitiveInfo(value)
	value = starAIBrandPattern.ReplaceAllString(value, "Molii Volcengine Imagine API")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 240 {
		value = string(runes[:240]) + "…"
	}
	return strings.TrimSpace(value)
}

func sanitizeStarAIAssetErrorCode(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
			return r
		}
		return -1
	}, value)
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func parseStarAIAssetUpstreamFailure(operation string, status int, body []byte, cause error) *starAIAssetUpstreamFailure {
	failure := &starAIAssetUpstreamFailure{Operation: operation, Status: status, Cause: cause}
	var value any
	if len(body) > 0 && common.Unmarshal(body, &value) == nil {
		failure.Code = sanitizeStarAIAssetErrorCode(starAIAssetStringField(value, "code", "error_code", "type"))
		failure.Reason = sanitizeStarAIAssetErrorText(starAIAssetStringField(value, "message", "msg", "detail", "error_description", "reason"))
	}
	if failure.Reason == "" {
		switch {
		case cause != nil:
			failure.Reason = "无法连接 Molii Volcengine Imagine API 服务"
		case status == http.StatusRequestEntityTooLarge:
			failure.Reason = "素材文件超过 Molii Volcengine Imagine API 大小限制"
		case status == http.StatusUnsupportedMediaType:
			failure.Reason = "Molii Volcengine Imagine API 不支持该素材格式"
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			failure.Reason = "Molii Volcengine Imagine API 渠道认证失败，请联系管理员"
		case status == http.StatusTooManyRequests:
			failure.Reason = "Molii Volcengine Imagine API 服务繁忙，请稍后重试"
		case status >= 400 && status < 500:
			failure.Reason = "素材参数或媒体规格不符合 Molii Volcengine Imagine API 要求"
		default:
			failure.Reason = "Molii Volcengine Imagine API 服务异常"
		}
	}
	return failure
}

func starAIAssetClientStatus(upstreamStatus int) int {
	switch upstreamStatus {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict,
		http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType,
		http.StatusUnprocessableEntity, http.StatusTooManyRequests:
		return upstreamStatus
	default:
		return http.StatusBadGateway
	}
}

func writeStarAIAssetUpstreamFailure(c *gin.Context, channel *model.Channel, failure *starAIAssetUpstreamFailure) {
	logger.LogError(c.Request.Context(), fmt.Sprintf(
		"Molii Volcengine Imagine API asset %s failed channel_id=%d upstream_status=%d upstream_code=%q reason=%q cause_type=%T",
		failure.Operation, channel.Id, failure.Status, failure.Code, failure.Reason, failure.Cause,
	))
	response := gin.H{
		"success": false,
		"code":    "starai_asset_upstream_error",
		"message": failure.Reason,
	}
	if failure.Status > 0 {
		response["upstream_status"] = failure.Status
	}
	if failure.Code != "" {
		response["upstream_code"] = failure.Code
	}
	c.JSON(starAIAssetClientStatus(failure.Status), response)
}

func (r *starAIAssetUpstreamResponse) payload() *starAIAssetUpstreamResponse {
	if r.Data != nil {
		return r.Data
	}
	return r
}

func normalizeStarAIAssetStatus(status string) string {
	return service.NormalizeStarAIAssetStatus(status)
}

func safeStarAIAsset(binding *service.StarAIAssetBinding) starAIAssetResponse {
	sourceKind := binding.SourceKind
	previewURL := binding.SourceURL
	if binding.COSKey != "" {
		sourceKind = "cos"
		previewURL, _ = service.GetStarAICOSPreviewURL(context.Background(), binding.COSKey)
	} else if sourceKind == "" && binding.SourceURL != "" {
		sourceKind = "url"
	}
	return starAIAssetResponse{ID: binding.ID, AssetType: binding.AssetType, Name: binding.Name, SourceURL: binding.SourceURL, PreviewURL: previewURL, SourceKind: sourceKind, FileName: binding.FileName, ContentType: binding.ContentType, FileSize: binding.FileSize, Status: binding.Status, CreatedAt: binding.CreatedAt, ExpiresAt: binding.ExpiresAt, VerifiedAt: binding.VerifiedAt}
}

func safeStarAIAssetForAdmin(binding *service.StarAIAssetBinding) starAIAssetResponse {
	username, _ := model.GetUsernameById(binding.UserID, false)
	response := safeStarAIAsset(binding)
	response.UserID = binding.UserID
	response.Username = username
	return response
}

func isDashboardAssetRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/")
}

func getStarAIAssetChannel() (*model.Channel, error) {
	return model.GetUniqueEnabledChannelByType(constant.ChannelTypeStarAI)
}

func validatePublicAssetURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" {
		return errors.New("url must be a public HTTP or HTTPS URL")
	}
	if parsed.User != nil {
		return errors.New("url credentials are not allowed")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("local URLs are not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
		return errors.New("private network URLs are not allowed")
	}
	return nil
}

func doStarAIAssetRequest(channel *model.Channel, method, path string, body io.Reader) ([]byte, int, error) {
	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeStarAI]
	}
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	req.Header.Set("Content-Type", "application/json")
	client, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return data, resp.StatusCode, err
}

func createStarAIAssetUpstream(c *gin.Context, input createStarAIAssetRequest, binding *service.StarAIAssetBinding) (*service.StarAIAssetBinding, bool) {
	channel, err := getStarAIAssetChannel()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("asset channel configuration unavailable: type=%d error=%v", constant.ChannelTypeStarAI, err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "asset service is temporarily unavailable"})
		return nil, false
	}
	body, _ := common.Marshal(input)
	respBody, status, err := doStarAIAssetRequest(channel, http.MethodPost, "/v1/assets", strings.NewReader(string(body)))
	if err != nil || status < 200 || status >= 300 {
		writeStarAIAssetUpstreamFailure(c, channel, parseStarAIAssetUpstreamFailure("create", status, respBody, err))
		return nil, false
	}
	var envelope starAIAssetUpstreamResponse
	if err := common.Unmarshal(respBody, &envelope); err != nil || envelope.payload().ID == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Molii Volcengine Imagine API asset create returned invalid response channel_id=%d upstream_status=%d", channel.Id, status))
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "invalid Molii Volcengine Imagine API asset response"})
		return nil, false
	}
	upstream := envelope.payload()
	initialStatus := normalizeStarAIAssetStatus(upstream.Status)
	if initialStatus == "" {
		initialStatus = "PROCESSING"
	}
	binding.UpstreamID = upstream.ID
	binding.UserID = c.GetInt("id")
	binding.TokenID = c.GetInt("token_id")
	binding.AssetType = input.AssetType
	binding.Name = input.Name
	binding.Status = initialStatus
	if err := service.SaveStarAIAssetBinding(binding); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "temporary asset mapping unavailable"})
		return nil, false
	}
	return binding, true
}

func writeStarAIAssetCreateSuccess(c *gin.Context, binding *service.StarAIAssetBinding) {
	if isDashboardAssetRequest(c) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": safeStarAIAsset(binding)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": binding.ID})
}

func CreateStarAIAsset(c *gin.Context) {
	if !common.RedisEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "temporary assets require Redis"})
		return
	}
	var input createStarAIAssetRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	input.AssetType = strings.ToLower(strings.TrimSpace(input.AssetType))
	if input.AssetType != "image" && input.AssetType != "video" && input.AssetType != "audio" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "asset_type must be image, video, or audio"})
		return
	}
	if err := validatePublicAssetURL(input.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "name is required and must not exceed 80 characters"})
		return
	}
	binding, ok := createStarAIAssetUpstream(c, input, &service.StarAIAssetBinding{SourceURL: strings.TrimSpace(input.URL), SourceKind: "url"})
	if ok {
		writeStarAIAssetCreateSuccess(c, binding)
	}
}

func GetStarAICOSUploadConfig(c *gin.Context) {
	config := operation_setting.GetCOSConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled": config.Enabled && config.Validate() == nil,
			"limits":  gin.H{"image": 30 * 1024 * 1024, "video": 50 * 1024 * 1024, "audio": 15 * 1024 * 1024},
		},
	})
}

func TestStarAICOSStorage(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := service.TestStarAICOSConnection(ctx); err != nil {
		logger.LogError(c.Request.Context(), "COS connection test failed: "+err.Error())
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "腾讯云 COS 连接测试失败，请检查存储桶、地域、域名和密钥权限"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func CreateStarAICOSUploadIntent(c *gin.Context) {
	var input createStarAICOSUploadRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	authorization, err := service.BeginStarAICOSUpload(c.Request.Context(), c.GetInt("id"), input.FileName, input.ContentType, input.AssetType, input.Name, input.FileSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": authorization})
}

func CompleteStarAICOSUpload(c *gin.Context) {
	var input completeStarAICOSUploadRequest
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.UploadID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	intent, readURL, err := service.VerifyStarAICOSUpload(c.Request.Context(), input.UploadID, c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	assetInput := createStarAIAssetRequest{URL: readURL, AssetType: intent.AssetType, Name: intent.Name}
	binding, ok := createStarAIAssetUpstream(c, assetInput, &service.StarAIAssetBinding{SourceKind: "cos", COSKey: intent.ObjectKey, FileName: intent.FileName, ContentType: intent.ContentType, FileSize: intent.FileSize})
	if !ok {
		return
	}
	if err := service.FinishStarAICOSUpload(c.Request.Context(), intent, binding.ExpiresAt); err != nil {
		logger.LogError(c.Request.Context(), "failed to finalize COS upload intent: "+err.Error())
	}
	writeStarAIAssetCreateSuccess(c, binding)
}

func refreshStarAIAsset(c *gin.Context, binding *service.StarAIAssetBinding) (*service.StarAIAssetBinding, error) {
	channel, err := getStarAIAssetChannel()
	if err != nil {
		return nil, err
	}
	body, status, err := doStarAIAssetRequest(channel, http.MethodGet, "/v1/assets/"+url.PathEscape(binding.UpstreamID), nil)
	if err == nil && (status == http.StatusNotFound || status == http.StatusGone) {
		if updateErr := service.UpdateStarAIAssetStatus(binding, "EXPIRED"); updateErr != nil {
			return nil, updateErr
		}
		return binding, nil
	}
	if err != nil || status < 200 || status >= 300 {
		return nil, parseStarAIAssetUpstreamFailure("query", status, body, err)
	}
	var envelope starAIAssetUpstreamResponse
	if err := common.Unmarshal(body, &envelope); err != nil {
		return nil, errors.New("invalid Molii Volcengine Imagine API asset response")
	}
	upstream := envelope.payload()
	if upstream.Status != "" {
		binding.Status = normalizeStarAIAssetStatus(upstream.Status)
	}
	if upstream.AssetType != "" {
		binding.AssetType = upstream.AssetType
	}
	if err := service.UpdateStarAIAssetStatus(binding, binding.Status); err != nil {
		return nil, err
	}
	return binding, nil
}

func GetStarAIAsset(c *gin.Context) {
	binding, err := service.GetStarAIAssetBinding(c.Param("id"), c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "temporary asset not found or expired"})
		return
	}
	binding, err = refreshStarAIAsset(c, binding)
	if err != nil {
		var upstreamFailure *starAIAssetUpstreamFailure
		if errors.As(err, &upstreamFailure) {
			channel, channelErr := getStarAIAssetChannel()
			if channelErr == nil {
				writeStarAIAssetUpstreamFailure(c, channel, upstreamFailure)
				return
			}
		}
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "临时素材状态查询失败"})
		return
	}
	if isDashboardAssetRequest(c) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": safeStarAIAsset(binding)})
	} else {
		c.JSON(http.StatusOK, safeStarAIAsset(binding))
	}
}

func ListStarAIAssets(c *gin.Context) {
	items, err := service.ListStarAIAssetBindings(c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "temporary assets unavailable"})
		return
	}
	result := make([]starAIAssetResponse, 0, len(items))
	for i := range items {
		result = append(result, safeStarAIAsset(&items[i]))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetStarAIAssetStats(c *gin.Context) {
	stats, err := service.GetStarAIAssetStats()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "temporary asset statistics unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

func ListStarAIAssetsForAdmin(c *gin.Context) {
	items, err := service.ListAllStarAIAssetBindings()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "temporary assets unavailable"})
		return
	}
	result := make([]starAIAssetResponse, 0, len(items))
	for i := range items {
		result = append(result, safeStarAIAssetForAdmin(&items[i]))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetStarAIAssetForAdmin(c *gin.Context) {
	binding, err := service.GetStarAIAssetBindingForAdmin(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "temporary asset not found or expired"})
		return
	}
	binding, err = refreshStarAIAsset(c, binding)
	if err != nil {
		var upstreamFailure *starAIAssetUpstreamFailure
		if errors.As(err, &upstreamFailure) {
			channel, channelErr := getStarAIAssetChannel()
			if channelErr == nil {
				writeStarAIAssetUpstreamFailure(c, channel, upstreamFailure)
				return
			}
		}
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "临时素材状态查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": safeStarAIAssetForAdmin(binding)})
}

func updateStarAIAssetSourceURL(c *gin.Context, admin bool) {
	var input updateStarAIAssetSourceURLRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	input.URL = strings.TrimSpace(input.URL)
	if err := validatePublicAssetURL(input.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	var binding *service.StarAIAssetBinding
	var err error
	if admin {
		binding, err = service.GetStarAIAssetBindingForAdmin(c.Param("id"))
	} else {
		binding, err = service.GetStarAIAssetBinding(c.Param("id"), c.GetInt("id"))
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "temporary asset not found or expired"})
		return
	}
	if err := service.UpdateStarAIAssetSourceURL(binding, input.URL); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "temporary asset mapping unavailable"})
		return
	}
	if admin {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": safeStarAIAssetForAdmin(binding)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": safeStarAIAsset(binding)})
}

func UpdateStarAIAssetSourceURL(c *gin.Context) {
	updateStarAIAssetSourceURL(c, false)
}

func UpdateStarAIAssetSourceURLForAdmin(c *gin.Context) {
	updateStarAIAssetSourceURL(c, true)
}

func DeleteStarAIAssetForAdmin(c *gin.Context) {
	if err := service.DeleteStarAIAssetBindingForAdmin(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "temporary asset not found or expired"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteStarAIAsset(c *gin.Context) {
	if err := service.DeleteStarAIAssetBinding(c.Param("id"), c.GetInt("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "temporary asset not found or expired"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
