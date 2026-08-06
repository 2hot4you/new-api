package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayMoliiGrokImagePricingErrorDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{
		"model":"unsupported-grok-image-model",
		"prompt":"test",
		"resolution":"1k",
		"n":1
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeMoliiGrokAIGC)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "unsupported-grok-image-model")

	require.NotPanics(t, func() {
		Relay(ctx, types.RelayFormatOpenAIImage)
	})
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "Molii Grok image pricing is not configured")
}
