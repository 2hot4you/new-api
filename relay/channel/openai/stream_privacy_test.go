package openai

import (
	"bytes"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOaiStreamTerminalParseFailureDoesNotLogPayload(t *testing.T) {
	previousTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = previousTimeout })
	var logs bytes.Buffer
	common.LogWriterMu.Lock()
	previous := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logs
	common.LogWriterMu.Unlock()
	t.Cleanup(func() { common.LogWriterMu.Lock(); gin.DefaultErrorWriter = previous; common.LogWriterMu.Unlock() })
	c, _, resp, info := newResponsesChatTestContext(t, "data: {\"private\":\"secret-stream-content\"\n\ndata: [DONE]\n\n", true)
	_, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.Contains(t, logs.String(), "error handling last response")
	require.NotContains(t, logs.String(), "secret-stream-content")
	require.NotContains(t, logs.String(), "lastStreamData")
}
