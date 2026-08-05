package controller

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoliiGrokChannelTestPerformsConfigurationCheckOnly(t *testing.T) {
	channel := &model.Channel{Id: 62, Type: constant.ChannelTypeMoliiGrokAIGC, Name: "grok", Key: "sk-placeholder"}
	result := testChannel(context.Background(), channel, 1, "", "", false)
	require.NoError(t, result.localErr)
	assert.Equal(t, "配置校验通过，未发起付费请求", result.successMessage)
	assert.Equal(t, "https://api.wxiai.com/xai", channel.GetBaseURL())
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
