package relay

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
)

func TestPrepareImageModelForAttemptUsesControllerPreparedModelWithoutRemapping(t *testing.T) {
	request := &dto.ImageRequest{Model: "requested-image", Prompt: "cat"}
	info := &relaycommon.RelayInfo{
		OriginModelName: "requested-image",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "selected-channel-model",
		},
	}

	prepareImageModelForAttempt(info, request)

	assert.Equal(t, "selected-channel-model", request.Model)
}
