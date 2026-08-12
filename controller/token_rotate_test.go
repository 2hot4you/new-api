package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRotateTokenKeyRequiresConfirmationAndReturnsOnlyNewKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "default", "old-rotate-key")
	token.IsDefault = true
	require.NoError(t, db.Save(token).Error)

	noConfirmCtx, noConfirmRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/rotate", map[string]bool{"confirm": false}, 1)
	noConfirmCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	RotateTokenKey(noConfirmCtx)
	require.False(t, decodeAPIResponse(t, noConfirmRecorder).Success)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/rotate", map[string]bool{"confirm": true}, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	RotateTokenKey(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	var data struct {
		Key string `json:"key"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.NotEmpty(t, data.Key)
	require.NotEqual(t, "old-rotate-key", data.Key)
	require.NotContains(t, string(response.Data), "name")
	require.NotContains(t, string(response.Data), "is_default")
}
