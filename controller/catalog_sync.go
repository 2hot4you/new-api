package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// CatalogSyncManagementMiddleware must protect every catalog management method,
// including status/preview/history reads. It does not consume a security proof:
// apply/restore must precheck the stored intent before consuming that proof.
func CatalogSyncManagementMiddleware() []gin.HandlerFunc {
	runtime, err := service.CurrentCatalogSyncRuntime()
	return []gin.HandlerFunc{middleware.RootAuth(), catalogSyncManagementGuard(runtime, err)}
}

func catalogSyncManagementGuard(runtime *service.CatalogSyncRuntime, configurationError error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if configurationError != nil || runtime == nil || !runtime.Status().ManagementReady {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "CATALOG_SYNC_UNAVAILABLE", "message": "catalog sync configuration unavailable"})
			return
		}
		origin, ok := middleware.StrictRequestBrowserOrigin(c.Request)
		expected, err := runtime.ExternalOrigin()
		if !ok || err != nil || origin != expected {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_ORIGIN_FORBIDDEN", "message": "request origin is not allowed"})
			return
		}
		identity, ok := middleware.GetSessionAuthIdentity(c)
		if !ok || model.ValidateCatalogRootAuthSession(c.Request.Context(), identity) != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_SESSION_REQUIRED", "message": "a current root login session is required"})
			return
		}
		c.Next()
	}
}

// CatalogSyncSourceGuard grants only a GET export privilege, without dashboard
// identity, role or proof. Capture the process singleton, preserving counters
// across requests and guard instances; protected rotation requires restart.
func CatalogSyncSourceGuard() gin.HandlerFunc {
	runtime, err := service.CurrentCatalogSyncRuntime()
	return catalogSyncSourceGuard(runtime, err)
}

func catalogSyncSourceGuard(runtime *service.CatalogSyncRuntime, configurationError error) gin.HandlerFunc {
	return func(c *gin.Context) {
		values := c.Request.Header.Values("Authorization")
		token := ""
		if len(values) == 1 {
			parts := strings.Split(values[0], " ")
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token = parts[1]
			}
		}
		if configurationError != nil || runtime == nil || !runtime.Status().SourceReady || c.Request.Method != http.MethodGet || runtime.AuthenticateReader(token) != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": "CATALOG_SYNC_READER_DENIED", "message": "catalog sync reader denied"})
			return
		}
		c.Next()
	}
}
