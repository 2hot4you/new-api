package router

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestFilesAPIRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetRelayRouter(engine)

	routes := make(map[string]string, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = route.Handler
	}
	expected := map[string]string{
		http.MethodPost + " /v1/files":            "CreateFile",
		http.MethodGet + " /v1/files":             "ListFiles",
		http.MethodGet + " /v1/files/:id":         "RetrieveFile",
		http.MethodDelete + " /v1/files/:id":      "DeleteFile",
		http.MethodGet + " /v1/files/:id/content": "DownloadFile",
	}
	for route, suffix := range expected {
		handler, ok := routes[route]
		assert.True(t, ok, route)
		assert.True(t, strings.HasSuffix(handler, suffix), "%s uses %s", route, handler)
	}
}
