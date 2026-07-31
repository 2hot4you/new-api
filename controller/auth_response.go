package controller

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
)

// writeAuthErrorI18n preserves the legacy HTTP 200 application-error contract
// while adding a stable machine-readable error code.
func writeAuthErrorI18n(c *gin.Context, code string, messageKey string, args ...map[string]any) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"code":    code,
		"message": common.TranslateMessage(c, messageKey, args...),
	})
}

func writeAuthInternalErrorI18n(c *gin.Context, operation string, err error, messageKey string) {
	logger.LogError(c.Request.Context(), fmt.Sprintf("%s: %v", operation, err))
	writeAuthErrorI18n(c, "AUTH_INTERNAL_ERROR", messageKey)
}
