package controller

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Bound buffered downloads process-wide, including time spent writing the response.
var imageResultProxySlots = make(chan struct{}, 2)

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

	select {
	case imageResultProxySlots <- struct{}{}:
		defer func() { <-imageResultProxySlots }()
	default:
		c.Header("Retry-After", "3")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
			"code": "image_proxy_busy", "message": "Image is temporarily unavailable",
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
	// Slow readers must not hold a download slot indefinitely.
	responseController := http.NewResponseController(c.Writer)
	_ = responseController.SetWriteDeadline(time.Now().Add(60 * time.Second))
	defer func() { _ = responseController.SetWriteDeadline(time.Time{}) }()
	c.Data(http.StatusOK, contentType, image)
}
