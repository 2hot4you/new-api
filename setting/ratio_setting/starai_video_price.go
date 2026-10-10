package ratio_setting

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	seedance20Model     = "doubao-seedance-2-0-260128"
	seedance20FastModel = "doubao-seedance-2-0-fast-260128"
	seedance20MiniModel = "doubao-seedance-2-0-mini-260615"
	seedance25Model     = "doubao-seedance-2-5-260628"
)

var starAIVideoPricingModels = []string{
	seedance20Model,
	seedance20FastModel,
	seedance20MiniModel,
	seedance25Model,
}

var starAIVideoPriceFields = map[string]struct{}{
	"standard_720p": {}, "standard_720p_video": {},
	"standard_1080p": {}, "standard_1080p_video": {},
	"standard_4k": {}, "standard_4k_video": {},
	"fast_720p": {}, "fast_720p_video": {},
	"mini_720p": {}, "mini_720p_video": {},
	"seedance_25_720p": {}, "seedance_25_720p_video": {},
	"seedance_25_1080p": {}, "seedance_25_1080p_video": {},
}

// StarAIVideoPriceSetting stores CNY source prices per 1M tokens. Generated
// expressions keep these coefficients unchanged; the billing snapshot converts
// their result to USD-equivalent quota using the frozen exchange rate.
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
// The values are CNY source prices per 1M tokens.
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

// ParseStarAIVideoPriceSetting rejects partial or extended payloads. A missing
// field must never silently become a zero-price tier.
func ParseStarAIVideoPriceSetting(raw string) (StarAIVideoPriceSetting, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil {
		return StarAIVideoPriceSetting{}, err
	}
	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return StarAIVideoPriceSetting{}, fmt.Errorf("price matrix must be a JSON object")
	}
	fields := make(map[string]json.RawMessage, len(starAIVideoPriceFields))
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return StarAIVideoPriceSetting{}, err
		}
		name, ok := nameToken.(string)
		if !ok {
			return StarAIVideoPriceSetting{}, fmt.Errorf("price matrix field name must be a string")
		}
		if _, exists := fields[name]; exists {
			return StarAIVideoPriceSetting{}, fmt.Errorf("duplicate price field %s", name)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return StarAIVideoPriceSetting{}, err
		}
		fields[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return StarAIVideoPriceSetting{}, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return StarAIVideoPriceSetting{}, fmt.Errorf("price matrix must contain one JSON object")
		}
		return StarAIVideoPriceSetting{}, err
	}
	for name := range starAIVideoPriceFields {
		value, ok := fields[name]
		if !ok {
			return StarAIVideoPriceSetting{}, fmt.Errorf("missing required price %s", name)
		}
		var number *float64
		if err := json.Unmarshal(value, &number); err != nil || number == nil {
			return StarAIVideoPriceSetting{}, fmt.Errorf("price %s must be a number", name)
		}
	}
	for name := range fields {
		if _, ok := starAIVideoPriceFields[name]; !ok {
			return StarAIVideoPriceSetting{}, fmt.Errorf("unsupported price field %s", name)
		}
	}
	var prices StarAIVideoPriceSetting
	if err := json.Unmarshal([]byte(raw), &prices); err != nil {
		return StarAIVideoPriceSetting{}, err
	}
	return prices, nil
}

func IsStarAIVideoPricingModel(name string) bool {
	for _, model := range starAIVideoPricingModels {
		if name == model {
			return true
		}
	}
	return false
}

func StarAIVideoPricingModels() []string {
	return append([]string(nil), starAIVideoPricingModels...)
}

var starAIVideoPriceSetting = DefaultStarAIVideoPriceSetting()
var starAIVideoPriceSettingMu sync.RWMutex

func DefaultStarAIVideoPriceSetting() StarAIVideoPriceSetting {
	return StarAIVideoPriceSetting{
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
}

func GetStarAIVideoPriceSettingCopy() StarAIVideoPriceSetting {
	starAIVideoPriceSettingMu.RLock()
	defer starAIVideoPriceSettingMu.RUnlock()
	return starAIVideoPriceSetting
}

// PublishStarAIVideoPriceSetting accepts a complete prevalidated value; the caller
// owns the catalog writer across the batch and preserves registration identity.
func PublishStarAIVideoPriceSetting(value StarAIVideoPriceSetting) {
	starAIVideoPriceSettingMu.Lock()
	starAIVideoPriceSetting = value
	starAIVideoPriceSettingMu.Unlock()
}

// BuildStarAIVideoBillingExpressions turns the editable Seedance price matrix
// into the task expressions used for reservation and settlement. The
// expression result is the request cost, so token prices are divided by 1M.
func BuildStarAIVideoBillingExpressions(prices StarAIVideoPriceSetting) (map[string]string, error) {
	values := []struct {
		name  string
		value float64
	}{
		{"standard_720p", prices.Standard720p}, {"standard_720p_video", prices.Standard720pVideo},
		{"standard_1080p", prices.Standard1080p}, {"standard_1080p_video", prices.Standard1080pVideo},
		{"standard_4k", prices.Standard4K}, {"standard_4k_video", prices.Standard4KVideo},
		{"fast_720p", prices.Fast720p}, {"fast_720p_video", prices.Fast720pVideo},
		{"mini_720p", prices.Mini720p}, {"mini_720p_video", prices.Mini720pVideo},
		{"seedance_25_720p", prices.Seedance25720p}, {"seedance_25_720p_video", prices.Seedance25720pVideo},
		{"seedance_25_1080p", prices.Seedance251080p}, {"seedance_25_1080p_video", prices.Seedance251080pVideo},
	}
	for _, entry := range values {
		if entry.value < 0 || math.IsNaN(entry.value) || math.IsInf(entry.value, 0) {
			return nil, fmt.Errorf("%s must be a finite, non-negative number", entry.name)
		}
	}
	term := func(price float64) string {
		return `u("tokens") * ` + strconv.FormatFloat(price, 'f', -1, 64) + ` / 1000000`
	}
	videoTier := func(tier string, withoutVideo, withVideo float64) string {
		return `u("video_input") == "video" ? tier("` + tier + `_video", ` + term(withVideo) + `) : tier("` + tier + `", ` + term(withoutVideo) + `)`
	}

	return map[string]string{
		seedance20Model: `u("resolution") == "4k" ? (` + videoTier("4k", prices.Standard4K, prices.Standard4KVideo) +
			`) : u("resolution") == "1080p" ? (` + videoTier("1080p", prices.Standard1080p, prices.Standard1080pVideo) +
			`) : (` + videoTier("720p", prices.Standard720p, prices.Standard720pVideo) + `)`,
		seedance20FastModel: videoTier("720p", prices.Fast720p, prices.Fast720pVideo),
		seedance20MiniModel: videoTier("720p", prices.Mini720p, prices.Mini720pVideo),
		seedance25Model: `u("resolution") == "1080p" ? (` + videoTier("1080p", prices.Seedance251080p, prices.Seedance251080pVideo) +
			`) : (` + videoTier("720p", prices.Seedance25720p, prices.Seedance25720pVideo) + `)`,
	}, nil
}

// OptionValues serializes the matrix using the existing dotted option keys so
// old nodes and the settings loader continue to read the same configuration.
func (prices StarAIVideoPriceSetting) OptionValues() map[string]string {
	format := func(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }
	return map[string]string{
		"starai_video_price.standard_720p":           format(prices.Standard720p),
		"starai_video_price.standard_720p_video":     format(prices.Standard720pVideo),
		"starai_video_price.standard_1080p":          format(prices.Standard1080p),
		"starai_video_price.standard_1080p_video":    format(prices.Standard1080pVideo),
		"starai_video_price.standard_4k":             format(prices.Standard4K),
		"starai_video_price.standard_4k_video":       format(prices.Standard4KVideo),
		"starai_video_price.fast_720p":               format(prices.Fast720p),
		"starai_video_price.fast_720p_video":         format(prices.Fast720pVideo),
		"starai_video_price.mini_720p":               format(prices.Mini720p),
		"starai_video_price.mini_720p_video":         format(prices.Mini720pVideo),
		"starai_video_price.seedance_25_720p":        format(prices.Seedance25720p),
		"starai_video_price.seedance_25_720p_video":  format(prices.Seedance25720pVideo),
		"starai_video_price.seedance_25_1080p":       format(prices.Seedance251080p),
		"starai_video_price.seedance_25_1080p_video": format(prices.Seedance251080pVideo),
	}
}

func init() {
	config.GlobalConfig.Register("starai_video_price", &starAIVideoPriceSetting)
}

// GetStarAIVideoPrice returns the configured direct price per 1M tokens.
func GetStarAIVideoPrice(model, resolution string, hasVideo bool) (float64, bool) {
	return GetStarAIVideoPriceSettingCopy().VideoPrice(model, resolution, hasVideo)
}

func (prices StarAIVideoPriceSetting) VideoPrice(model, resolution string, hasVideo bool) (float64, bool) {
	var price float64
	switch model {
	case "doubao-seedance-2-0-260128":
		switch strings.ToLower(strings.TrimSpace(resolution)) {
		case "4k":
			if hasVideo {
				price = prices.Standard4KVideo
			} else {
				price = prices.Standard4K
			}
		case "1080p":
			if hasVideo {
				price = prices.Standard1080pVideo
			} else {
				price = prices.Standard1080p
			}
		default:
			if hasVideo {
				price = prices.Standard720pVideo
			} else {
				price = prices.Standard720p
			}
		}
	case "doubao-seedance-2-0-fast-260128":
		if hasVideo {
			price = prices.Fast720pVideo
		} else {
			price = prices.Fast720p
		}
	case "doubao-seedance-2-0-mini-260615":
		if hasVideo {
			price = prices.Mini720pVideo
		} else {
			price = prices.Mini720p
		}
	case "doubao-seedance-2-5-260628":
		if strings.EqualFold(strings.TrimSpace(resolution), "1080p") {
			if hasVideo {
				price = prices.Seedance251080pVideo
			} else {
				price = prices.Seedance251080p
			}
		} else if hasVideo {
			price = prices.Seedance25720pVideo
		} else {
			price = prices.Seedance25720p
		}
	default:
		return 0, false
	}
	return price, price >= 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

// GetStarAIVideoPricing returns the configured pricing matrix used by both the
// public model catalog and the task billing path.
func GetStarAIVideoPricing(model string) (*StarAIVideoPricing, bool) {
	prices := GetStarAIVideoPriceSettingCopy()
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
				WithoutVideo: prices.Standard720p,
				WithVideo:    prices.Standard720pVideo,
			},
			{
				Resolutions:  []string{"1080p"},
				WithoutVideo: prices.Standard1080p,
				WithVideo:    prices.Standard1080pVideo,
			},
			{
				Resolutions:  []string{"4K"},
				WithoutVideo: prices.Standard4K,
				WithVideo:    prices.Standard4KVideo,
			},
		}
	case "doubao-seedance-2-0-fast-260128":
		pricing.Rows = []StarAIVideoPriceRow{
			{
				Resolutions:  []string{"480p", "720p"},
				WithoutVideo: prices.Fast720p,
				WithVideo:    prices.Fast720pVideo,
			},
		}
		pricing.UnsupportedResolutions = []string{"1080p", "4K"}
	case "doubao-seedance-2-0-mini-260615":
		pricing.Rows = []StarAIVideoPriceRow{{
			Resolutions: []string{"480p", "720p"}, WithoutVideo: prices.Mini720p, WithVideo: prices.Mini720pVideo,
		}}
		pricing.UnsupportedResolutions = []string{"1080p", "4K"}
	case "doubao-seedance-2-5-260628":
		pricing.Rows = []StarAIVideoPriceRow{
			{Resolutions: []string{"480p", "720p"}, WithoutVideo: prices.Seedance25720p, WithVideo: prices.Seedance25720pVideo},
			{Resolutions: []string{"1080p"}, WithoutVideo: prices.Seedance251080p, WithVideo: prices.Seedance251080pVideo},
		}
		pricing.UnsupportedResolutions = []string{"4K"}
	default:
		return nil, false
	}

	return pricing, true
}
