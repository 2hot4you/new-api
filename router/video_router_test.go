package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestVideoGenerationRouteRegistersOfficialAlias(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetVideoRouter(engine)

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	_, legacy := routes[http.MethodPost+" /v1/videos"]
	_, official := routes[http.MethodPost+" /v1/videos/generations"]
	_, extension := routes[http.MethodPost+" /v1/videos/extensions"]
	assert.True(t, legacy)
	assert.True(t, official)
	assert.True(t, extension)
}
