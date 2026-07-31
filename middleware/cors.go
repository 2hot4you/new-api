package middleware

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}
	corsHandler := cors.New(config)
	return func(c *gin.Context) {
		if !isDashboardAPIPath(c.Request.URL.Path) {
			corsHandler(c)
		}
	}
}

func DashboardCORS() gin.HandlerFunc {
	allowedOrigins := append([]string(nil), common.DashboardCORSAllowedOrigins...)
	if len(allowedOrigins) == 0 {
		return func(*gin.Context) {}
	}

	config := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowCredentials: true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Authorization",
			"Cache-Control",
			"Content-Type",
			"X-Auth-Session",
			"X-Security-Proof",
			"Accept-Language",
		},
		ExposeHeaders: []string{
			"Retry-After",
			"Auth-Version",
			common.RequestIdKey,
		},
	}
	corsHandler := cors.New(config)

	return func(c *gin.Context) {
		if isDashboardAPIPath(c.Request.URL.Path) {
			corsHandler(c)
		}
	}
}

func isDashboardAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func Version() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
