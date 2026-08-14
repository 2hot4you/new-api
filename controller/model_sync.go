package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const modelMetadataSyncErrorCode = "model_metadata_sync_failed"

func modelMetadataSyncContext(c *gin.Context) (context.Context, context.CancelFunc) {
	timeoutSeconds := common.GetEnvOrDefault("MODEL_METADATA_SYNC_TIMEOUT_SECONDS", 10)
	if timeoutSeconds <= 0 || timeoutSeconds > 300 {
		timeoutSeconds = 10
	}
	return context.WithTimeout(c.Request.Context(), time.Duration(timeoutSeconds)*time.Second)
}

func respondModelMetadataSyncError(c *gin.Context, action string, err error) {
	common.SysError(action + " models.dev model metadata failed: " + err.Error())
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"code":    modelMetadataSyncErrorCode,
		"message": "models.dev 模型元数据同步失败，请稍后重试",
	})
}

// SyncUpstreamModels synchronizes enabled model metadata from models.dev.
// Pricing, channels, enabled state, and administrator-authored values are not
// overwritten by this operation.
func SyncUpstreamModels(c *gin.Context) {
	ctx, cancel := modelMetadataSyncContext(c)
	defer cancel()

	summary, err := model.SyncModelMetadataFromModelsDev(ctx)
	if err != nil {
		respondModelMetadataSyncError(c, "sync", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
		"source":  "models.dev",
	})
}

// SyncUpstreamPreview previews the same models.dev reconciliation used by the
// automatic and manual synchronization paths without writing to the database.
func SyncUpstreamPreview(c *gin.Context) {
	ctx, cancel := modelMetadataSyncContext(c)
	defer cancel()

	preview, err := model.PreviewModelMetadataFromModelsDev(ctx)
	if err != nil {
		respondModelMetadataSyncError(c, "preview", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    preview,
		"source":  "models.dev",
	})
}
