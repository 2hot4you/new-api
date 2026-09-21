package controller

import (
	"crypto/sha256"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/brand_setting"
	"github.com/gin-gonic/gin"
)

func GetClaudeyeWordmark(c *gin.Context) {
	palette := brand_setting.GetClaudeyePalette()
	surface := service.ClaudeyeSurface(c.Query("surface"))
	if surface == "" {
		surface = service.ClaudeyeSurfaceLight
	}
	if surface != service.ClaudeyeSurfaceLight && surface != service.ClaudeyeSurfaceDark {
		c.String(http.StatusBadRequest, "invalid surface")
		return
	}

	if value, present := c.GetQuery("mark"); present {
		normalized, err := brand_setting.NormalizeHexColor(value)
		if err != nil {
			c.String(http.StatusBadRequest, "invalid mark color")
			return
		}
		if surface == service.ClaudeyeSurfaceLight {
			palette.LightMark = normalized
		} else {
			palette.DarkMark = normalized
		}
	}
	if value, present := c.GetQuery("text"); present {
		normalized, err := brand_setting.NormalizeHexColor(value)
		if err != nil {
			c.String(http.StatusBadRequest, "invalid text color")
			return
		}
		if surface == service.ClaudeyeSurfaceLight {
			palette.LightText = normalized
		} else {
			palette.DarkText = normalized
		}
	}

	body, err := service.RenderClaudeyeWordmark(palette, surface)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid brand colors")
		return
	}
	serveClaudeyeSVG(c, body)
}

func GetClaudeyeFavicon(c *gin.Context) {
	body := service.RenderClaudeyeFavicon(brand_setting.GetClaudeyePalette())
	serveClaudeyeSVG(c, body)
}

func serveClaudeyeSVG(c *gin.Context, body []byte) {
	digest := sha256.Sum256(body)
	etag := fmt.Sprintf(`"%x"`, digest)

	c.Header("Content-Type", "image/svg+xml; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-cache, must-revalidate")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", body)
}
