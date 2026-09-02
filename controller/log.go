package controller

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

var (
	getGPTImage2PreviewObject    = service.GetGPTImage2PreviewObject
	fetchGPTImage2PreviewContent = service.FetchCOSObject
)

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logCategory := c.Query("log_category")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, upstreamRequestId, logCategory)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if c.GetInt("role") < common.RoleRootUser {
		model.FormatAdminLogs(logs)
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logCategory := c.Query("log_category")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId, logCategory)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// GetGrokImagePreview returns the temporary result URL set indexed by the log's
// owner and request ID. Missing, expired, unavailable, and unauthorized values
// intentionally share a 404 response to avoid exposing preview existence.
func GetGrokImagePreview(c *gin.Context) {
	targetUserID, _ := strconv.Atoi(c.Param("user_id"))
	requestID := c.Param("request_id")
	viewerID := c.GetInt("id")
	if targetUserID <= 0 || requestID == "" || (viewerID != targetUserID && c.GetInt("role") < common.RoleAdminUser) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	urls, err := service.GetGrokImagePreview(targetUserID, requestID)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	common.ApiSuccess(c, gin.H{"urls": urls})
}

// GetGPTImage2Preview returns short-lived signed URLs for Molii-owned GPT
// Image 2 result objects. All lookup and authorization failures intentionally
// share a 404 response so object existence is never disclosed.
func GetGPTImage2Preview(c *gin.Context) {
	targetUserID, _ := strconv.Atoi(c.Param("user_id"))
	requestID := c.Param("request_id")
	viewerID := c.GetInt("id")
	if targetUserID <= 0 || requestID == "" || (viewerID != targetUserID && c.GetInt("role") < common.RoleAdminUser) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	urls, err := service.GetGPTImage2Preview(targetUserID, requestID)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	common.ApiSuccess(c, gin.H{"urls": urls})
}

// DownloadGPTImage2Preview relays one owned temporary COS result with an
// attachment disposition so browsers download it instead of navigating to a
// cross-origin signed object URL.
func DownloadGPTImage2Preview(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	targetUserID, _ := strconv.Atoi(c.Param("user_id"))
	requestID := c.Param("request_id")
	index, indexErr := strconv.Atoi(c.Param("index"))
	viewerID := c.GetInt("id")
	if targetUserID <= 0 || requestID == "" || indexErr != nil || index < 0 ||
		(viewerID != targetUserID && c.GetInt("role") < common.RoleAdminUser) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	object, err := getGPTImage2PreviewObject(targetUserID, requestID, index)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	response, err := fetchGPTImage2PreviewContent(ctx, object.ObjectKey, c.GetHeader("Range"), c.GetHeader("If-Range"))
	if err != nil || response == nil || response.Body == nil {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent && response.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}
	for _, key := range []string{"Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if value := response.Header.Get(key); value != "" {
			c.Header(key, value)
		}
	}
	extension := ".img"
	switch object.MIMEType {
	case "image/png":
		extension = ".png"
	case "image/jpeg":
		extension = ".jpg"
	case "image/webp":
		extension = ".webp"
	}
	c.Header("Content-Type", object.MIMEType)
	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": "molii-gpt-image-2-" + strconv.Itoa(index+1) + extension,
	})
	if disposition == "" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", disposition)
	c.Status(response.StatusCode)
	if response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return
	}
	_, _ = io.Copy(c.Writer, response.Body)
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}
