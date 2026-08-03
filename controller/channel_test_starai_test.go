package controller

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStarAIChannelTestOnlyOpensTCPConnection(t *testing.T) {
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

	baseURL := "http://" + listener.Addr().String() + "/v1/video/generations"
	channel := &model.Channel{
		Id:      61,
		Type:    constant.ChannelTypeStarAI,
		Name:    "Molii AIGC reachability",
		Key:     "must-not-be-sent",
		BaseURL: common.GetPointer(baseURL),
	}

	result := testChannel(context.Background(), channel, 1, "doubao-seedance-2-0-260128", string(constant.EndpointTypeOpenAIVideo), false)

	require.NoError(t, result.localErr)
	assert.Nil(t, result.newAPIError)
	require.NotNil(t, result.context)
	select {
	case count := <-receivedBytes:
		assert.Zero(t, count, "reachability test must not send HTTP or authentication bytes")
	case <-time.After(2 * time.Second):
		t.Fatal("TCP reachability connection was not accepted")
	}
}

func TestStarAIChannelTestReportsClosedPortAsReachabilityFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	baseURL := "https://" + address
	channel := &model.Channel{
		Type:    constant.ChannelTypeStarAI,
		Name:    "Molii AIGC unreachable",
		BaseURL: common.GetPointer(baseURL),
	}

	result := testChannel(context.Background(), channel, 1, "", "", false)

	require.Error(t, result.localErr)
	require.NotNil(t, result.newAPIError)
	assert.Equal(t, types.ErrorCodeDoRequestFailed, result.newAPIError.GetErrorCode())
	assert.Equal(t, http.StatusServiceUnavailable, result.newAPIError.StatusCode)
	assert.Contains(t, result.localErr.Error(), "reachability test failed")
}

func TestStarAIReachabilityRejectsNonHTTPURL(t *testing.T) {
	baseURL := "ftp://starai.example:21"
	channel := &model.Channel{
		Type:    constant.ChannelTypeStarAI,
		BaseURL: common.GetPointer(baseURL),
	}

	err := testStarAIReachability(context.Background(), channel)

	require.ErrorContains(t, err, "unsupported URL scheme")
}
