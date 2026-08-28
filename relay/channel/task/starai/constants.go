package starai

const ChannelName = "molii-aigc"

const (
	ModelSeedance20     = "doubao-seedance-2-0-260128"
	ModelSeedance20Fast = "doubao-seedance-2-0-fast-260128"
	ModelSeedance20Mini = "doubao-seedance-2-0-mini-260615"
	ModelSeedance25     = "doubao-seedance-2-5-260628"
)

var ModelList = []string{
	ModelSeedance20,
	ModelSeedance20Fast,
	ModelSeedance20Mini,
	ModelSeedance25,
}

type modelCapabilities struct {
	maxDuration         int
	supportedResolution map[string]struct{}
}

func capabilitiesForModel(model string) modelCapabilities {
	capabilities := modelCapabilities{
		maxDuration: 15,
		supportedResolution: map[string]struct{}{
			"480p": {}, "720p": {}, "1080p": {}, "4k": {},
		},
	}
	switch model {
	case ModelSeedance20Fast, ModelSeedance20Mini:
		capabilities.supportedResolution = map[string]struct{}{"480p": {}, "720p": {}}
	case ModelSeedance25:
		capabilities.maxDuration = 30
		capabilities.supportedResolution = map[string]struct{}{"480p": {}, "720p": {}, "1080p": {}}
	}
	return capabilities
}
