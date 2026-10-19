package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetImageResultProxy serves a temporary capability URL without API-key auth.
// Only the server can mint a valid capability; callers cannot choose a target.
func GetImageResultProxy(c *gin.Context) {
	serveImageResultProxy(c, service.ResolveImageResultProxyToken, service.FetchImageResultProxy)
}

func serveImageResultProxy(
	c *gin.Context,
	resolve func(string) (string, error),
	fetch func(context.Context, string) ([]byte, string, error),
) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	if !common.GetEnvOrDefaultBool("IMAGE_RESULT_PROXY_ENABLED", false) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
			"code": "image_link_invalid", "message": "Image link is invalid or unavailable",
		}})
		return
	}

	sourceURL, err := resolve(c.Param("token"))
	if err != nil {
		if errors.Is(err, service.ErrImageResultProxyExpired) {
			c.JSON(http.StatusGone, gin.H{"error": gin.H{
				"code": "image_link_expired", "message": "Image link has expired",
			}})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
			"code": "image_link_invalid", "message": "Image link is invalid or unavailable",
		}})
		return
	}

	image, contentType, err := fetch(c.Request.Context(), sourceURL)
	if err != nil {
		// Transport errors can contain the entire upstream URL. Do not expose
		// them or upstream response headers/bodies on this public endpoint.
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{
			"code": "image_unavailable", "message": "Image is temporarily unavailable",
		}})
		return
	}
	c.Data(http.StatusOK, contentType, image)
}
