package starai

import "github.com/QuantumNous/new-api/relay/channel/task/seedanceprotocol"

const ChannelName = "molii-aigc"

const (
	ModelSeedance20     = "doubao-seedance-2-0-260128"
	ModelSeedance20Fast = "doubao-seedance-2-0-fast-260128"
	ModelSeedance20Mini = "doubao-seedance-2-0-mini-260615"
	ModelSeedance25     = "doubao-seedance-2-5-260628"
)

var ModelList = seedanceprotocol.SupportedModels()
