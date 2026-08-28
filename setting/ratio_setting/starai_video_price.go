package ratio_setting

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// StarAIVideoPriceSetting stores direct platform-currency prices per 1M tokens.
// molii uses a 1:1 CNY/USD platform value, so no exchange-rate conversion applies.
type StarAIVideoPriceSetting struct {
	Standard720p         float64 `json:"standard_720p"`
	Standard720pVideo    float64 `json:"standard_720p_video"`
	Standard1080p        float64 `json:"standard_1080p"`
	Standard1080pVideo   float64 `json:"standard_1080p_video"`
	Standard4K           float64 `json:"standard_4k"`
	Standard4KVideo      float64 `json:"standard_4k_video"`
	Fast720p             float64 `json:"fast_720p"`
	Fast720pVideo        float64 `json:"fast_720p_video"`
	Mini720p             float64 `json:"mini_720p"`
	Mini720pVideo        float64 `json:"mini_720p_video"`
	Seedance25720p       float64 `json:"seedance_25_720p"`
	Seedance25720pVideo  float64 `json:"seedance_25_720p_video"`
	Seedance251080p      float64 `json:"seedance_25_1080p"`
	Seedance251080pVideo float64 `json:"seedance_25_1080p_video"`
}

// StarAIVideoPriceRow is a public, read-only view of one resolution tier.
// The values are direct platform-currency prices per 1M tokens.
type StarAIVideoPriceRow struct {
	Resolutions  []string `json:"resolutions"`
	WithoutVideo float64  `json:"without_video"`
	WithVideo    float64  `json:"with_video"`
}

// StarAIVideoPricing describes the pricing dimensions required by the model
// catalog. Keeping this view next to the billing settings prevents the public
// price display from drifting away from the prices used for deductions.
type StarAIVideoPricing struct {
	Unit                   string                `json:"unit"`
	FPS                    int                   `json:"fps"`
	ExtraFrames            int                   `json:"extra_frames"`
	Rows                   []StarAIVideoPriceRow `json:"rows"`
	UnsupportedResolutions []string              `json:"unsupported_resolutions,omitempty"`
}

var starAIVideoPriceSetting = StarAIVideoPriceSetting{
	Standard720p:         46,
	Standard720pVideo:    28,
	Standard1080p:        51,
	Standard1080pVideo:   31,
	Standard4K:           26,
	Standard4KVideo:      16,
	Fast720p:             37,
	Fast720pVideo:        22,
	Mini720p:             23,
	Mini720pVideo:        14,
	Seedance25720p:       70,
	Seedance25720pVideo:  42,
	Seedance251080p:      77,
	Seedance251080pVideo: 46,
}

func init() {
	config.GlobalConfig.Register("starai_video_price", &starAIVideoPriceSetting)
}

// GetStarAIVideoPrice returns the configured direct price per 1M tokens.
func GetStarAIVideoPrice(model, resolution string, hasVideo bool) (float64, bool) {
	var price float64
	switch model {
	case "doubao-seedance-2-0-260128":
		switch strings.ToLower(strings.TrimSpace(resolution)) {
		case "4k":
			if hasVideo {
				price = starAIVideoPriceSetting.Standard4KVideo
			} else {
				price = starAIVideoPriceSetting.Standard4K
			}
		case "1080p":
			if hasVideo {
				price = starAIVideoPriceSetting.Standard1080pVideo
			} else {
				price = starAIVideoPriceSetting.Standard1080p
			}
		default:
			if hasVideo {
				price = starAIVideoPriceSetting.Standard720pVideo
			} else {
				price = starAIVideoPriceSetting.Standard720p
			}
		}
	case "doubao-seedance-2-0-fast-260128":
		if hasVideo {
			price = starAIVideoPriceSetting.Fast720pVideo
		} else {
			price = starAIVideoPriceSetting.Fast720p
		}
	case "doubao-seedance-2-0-mini-260615":
		if hasVideo {
			price = starAIVideoPriceSetting.Mini720pVideo
		} else {
			price = starAIVideoPriceSetting.Mini720p
		}
	case "doubao-seedance-2-5-260628":
		if strings.EqualFold(strings.TrimSpace(resolution), "1080p") {
			if hasVideo {
				price = starAIVideoPriceSetting.Seedance251080pVideo
			} else {
				price = starAIVideoPriceSetting.Seedance251080p
			}
		} else if hasVideo {
			price = starAIVideoPriceSetting.Seedance25720pVideo
		} else {
			price = starAIVideoPriceSetting.Seedance25720p
		}
	default:
		return 0, false
	}
	return price, price >= 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

// GetStarAIVideoPricing returns the configured pricing matrix used by both the
// public model catalog and the task billing path.
func GetStarAIVideoPricing(model string) (*StarAIVideoPricing, bool) {
	pricing := &StarAIVideoPricing{
		Unit:        "cny_per_million_tokens",
		FPS:         24,
		ExtraFrames: 1,
	}

	switch model {
	case "doubao-seedance-2-0-260128":
		pricing.Rows = []StarAIVideoPriceRow{
			{
				Resolutions:  []string{"480p", "720p"},
				WithoutVideo: starAIVideoPriceSetting.Standard720p,
				WithVideo:    starAIVideoPriceSetting.Standard720pVideo,
			},
			{
				Resolutions:  []string{"1080p"},
				WithoutVideo: starAIVideoPriceSetting.Standard1080p,
				WithVideo:    starAIVideoPriceSetting.Standard1080pVideo,
			},
			{
				Resolutions:  []string{"4K"},
				WithoutVideo: starAIVideoPriceSetting.Standard4K,
				WithVideo:    starAIVideoPriceSetting.Standard4KVideo,
			},
		}
	case "doubao-seedance-2-0-fast-260128":
		pricing.Rows = []StarAIVideoPriceRow{
			{
				Resolutions:  []string{"480p", "720p"},
				WithoutVideo: starAIVideoPriceSetting.Fast720p,
				WithVideo:    starAIVideoPriceSetting.Fast720pVideo,
			},
		}
		pricing.UnsupportedResolutions = []string{"1080p", "4K"}
	case "doubao-seedance-2-0-mini-260615":
		pricing.Rows = []StarAIVideoPriceRow{{
			Resolutions: []string{"480p", "720p"}, WithoutVideo: starAIVideoPriceSetting.Mini720p, WithVideo: starAIVideoPriceSetting.Mini720pVideo,
		}}
		pricing.UnsupportedResolutions = []string{"1080p", "4K"}
	case "doubao-seedance-2-5-260628":
		pricing.Rows = []StarAIVideoPriceRow{
			{Resolutions: []string{"480p", "720p"}, WithoutVideo: starAIVideoPriceSetting.Seedance25720p, WithVideo: starAIVideoPriceSetting.Seedance25720pVideo},
			{Resolutions: []string{"1080p"}, WithoutVideo: starAIVideoPriceSetting.Seedance251080p, WithVideo: starAIVideoPriceSetting.Seedance251080pVideo},
		}
		pricing.UnsupportedResolutions = []string{"4K"}
	default:
		return nil, false
	}

	return pricing, true
}
