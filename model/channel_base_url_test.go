package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

// A legacy or copied channel can still have a SQL NULL base_url. The default
// provider URL must remain usable in that case; otherwise task plugins build a
// relative upstream path and reject the request before payload validation.
func TestChannelGetBaseURLUsesProviderDefaultWhenStoredValueIsNull(t *testing.T) {
	channel := &Channel{Type: constant.ChannelTypeDoubaoVideo, BaseURL: nil}

	assert.Equal(t, "https://ark.cn-beijing.volces.com", channel.GetBaseURL())
}

func TestChannelGetBaseURLKeepsEmptyDefaultForCustomProviderWhenStoredValueIsNull(t *testing.T) {
	channel := &Channel{Type: constant.ChannelTypeTaskPlugin, BaseURL: nil}

	assert.Empty(t, channel.GetBaseURL())
}
