package router

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// SetDevWebRouter proxies dashboard and hot-reload traffic to the local
// frontend development server while keeping backend namespaces on New API.
func SetDevWebRouter(router *gin.Engine, rawTargetURL string) error {
	target, err := url.Parse(strings.TrimSpace(rawTargetURL))
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return fmt.Errorf("FRONTEND_DEV_SERVER_URL must be an absolute HTTP(S) URL")
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		director(request)
		request.Host = target.Host
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(writer, "frontend development server is unavailable", http.StatusBadGateway)
	}

	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		path := c.Request.URL.Path
		for _, prefix := range []string{"/api", "/v1", "/assets", "/mj", "/pg"} {
			if strings.HasPrefix(path, prefix) {
				controller.RelayNotFound(c)
				return
			}
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	})
	return nil
}
