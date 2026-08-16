package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestModelMetadataRoutesExcludeExternalSynchronization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	assert.NotContains(t, routes, http.MethodGet+" /api/models/sync_upstream/preview")
	assert.NotContains(t, routes, http.MethodPost+" /api/models/sync_upstream")
	assert.Contains(t, routes, http.MethodGet+" /api/models/missing")
	assert.Contains(t, routes, http.MethodGet+" /api/models/")
	assert.Contains(t, routes, http.MethodPost+" /api/models/")
}
