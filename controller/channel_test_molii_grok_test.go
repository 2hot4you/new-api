package controller

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoliiGrokChannelTestOnlyOpensTCPConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	receivedBytes := make(chan int, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			receivedBytes <- -1
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		buffer := make([]byte, 1)
		count, _ := conn.Read(buffer)
		receivedBytes <- count
	}()

	originalBaseURL := constant.ChannelBaseURLs[constant.ChannelTypeMoliiGrokAIGC]
	constant.ChannelBaseURLs[constant.ChannelTypeMoliiGrokAIGC] = "http://" + listener.Addr().String() + "/xai"
	t.Cleanup(func() { constant.ChannelBaseURLs[constant.ChannelTypeMoliiGrokAIGC] = originalBaseURL })

	channel := &model.Channel{Id: 62, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "sk-placeholder"}
	result := testChannel(context.Background(), channel, 1, "", "", false)
	require.NoError(t, result.localErr)
	assert.Equal(t, "可达性测试通过，未发送付费请求", result.successMessage)
	select {
	case count := <-receivedBytes:
		assert.Zero(t, count, "reachability test must not send HTTP or authentication bytes")
	case <-time.After(2 * time.Second):
		t.Fatal("TCP reachability connection was not accepted")
	}
}

func TestMoliiGrokChannelAlwaysUsesServerDefaultBaseURL(t *testing.T) {
	customURL := "https://custom.example.invalid"
	channel := &model.Channel{Type: constant.ChannelTypeMoliiGrokAIGC, BaseURL: &customURL}
	assert.Equal(t, constant.ChannelBaseURLs[constant.ChannelTypeMoliiGrokAIGC], channel.GetBaseURL())
}

func TestMoliiGrokChannelTestRequiresOnlyKey(t *testing.T) {
	channel := &model.Channel{Id: 62, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok"}
	result := testChannel(context.Background(), channel, 1, "", "", false)
	require.Error(t, result.localErr)
	assert.Contains(t, result.localErr.Error(), "Key")
	assert.NotContains(t, result.localErr.Error(), "wxiai")
}

func TestMoliiGrokChannelTestUsesEnabledKeyBeforeDialing(t *testing.T) {
	channel := &model.Channel{
		Type: constant.ChannelTypeMoliiGrokAIGC,
		Key:  "disabled-key",
		ChannelInfo: model.ChannelInfo{
			IsMultiKey: true,
			MultiKeyStatusList: map[int]int{
				0: common.ChannelStatusManuallyDisabled,
			},
		},
	}

	result := testChannel(context.Background(), channel, 1, "", "", false)
	require.Error(t, result.localErr)
	assert.Contains(t, result.localErr.Error(), "可用")
	assert.NotContains(t, result.localErr.Error(), "disabled-key")
}
