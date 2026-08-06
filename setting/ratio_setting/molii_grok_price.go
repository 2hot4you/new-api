package ratio_setting

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// MoliiGrokPriceSetting stores direct platform-currency prices. USD catalog
// numbers are treated as CNY at 1:1 without an exchange-rate conversion.
type MoliiGrokPriceSetting struct {
	ImageStandardInput float64 `json:"image_standard_input"`
	ImageStandard1K    float64 `json:"image_standard_1k"`
	ImageStandard2K    float64 `json:"image_standard_2k"`
	ImageQualityInput  float64 `json:"image_quality_input"`
	ImageQuality1K     float64 `json:"image_quality_1k"`
	ImageQuality2K     float64 `json:"image_quality_2k"`
	Video15ImageInput  float64 `json:"video_15_image_input"`
	Video15480p        float64 `json:"video_15_480p"`
	Video15720p        float64 `json:"video_15_720p"`
	Video151080p       float64 `json:"video_15_1080p"`
	VideoImageInput    float64 `json:"video_image_input"`
	VideoVideoInput    float64 `json:"video_video_input"`
	Video480p          float64 `json:"video_480p"`
	Video720p          float64 `json:"video_720p"`
}

// MoliiGrokCatalogPricing is the public, direct-currency price table used by
// the model marketplace. It intentionally excludes New API's internal ¥1
// fixed-price billing anchor.
type MoliiGrokCatalogPricing struct {
	Kind            string             `json:"kind"`
	OutputUnit      string             `json:"output_unit"`
	OutputPrices    map[string]float64 `json:"output_prices"`
	ImageInputUnit  string             `json:"image_input_unit,omitempty"`
	ImageInputPrice float64            `json:"image_input_price,omitempty"`
	VideoInputUnit  string             `json:"video_input_unit,omitempty"`
	VideoInputPrice float64            `json:"video_input_price,omitempty"`
}

var moliiGrokPriceSetting = MoliiGrokPriceSetting{
	ImageStandardInput: 0.002,
	ImageStandard1K:    0.02,
	ImageStandard2K:    0.02,
	ImageQualityInput:  0.01,
	ImageQuality1K:     0.05,
	ImageQuality2K:     0.07,
	Video15ImageInput:  0.01,
	Video15480p:        0.08,
	Video15720p:        0.14,
	Video151080p:       0.25,
	VideoImageInput:    0.002,
	VideoVideoInput:    0.01,
	Video480p:          0.05,
	Video720p:          0.07,
}

func init() {
	config.GlobalConfig.Register("molii_grok_price", &moliiGrokPriceSetting)
}

func validMoliiGrokPrice(values ...float64) bool {
	for _, value := range values {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

// GetMoliiGrokImagePrices returns output and per-input-image prices.
func GetMoliiGrokImagePrices(model, resolution string) (outputPrice, inputPrice float64, ok bool) {
	switch model {
	case "grok-imagine-image":
		inputPrice = moliiGrokPriceSetting.ImageStandardInput
		if strings.EqualFold(strings.TrimSpace(resolution), "2k") {
			outputPrice = moliiGrokPriceSetting.ImageStandard2K
		} else {
			outputPrice = moliiGrokPriceSetting.ImageStandard1K
		}
	case "grok-imagine-image-quality":
		inputPrice = moliiGrokPriceSetting.ImageQualityInput
		if strings.EqualFold(strings.TrimSpace(resolution), "2k") {
			outputPrice = moliiGrokPriceSetting.ImageQuality2K
		} else {
			outputPrice = moliiGrokPriceSetting.ImageQuality1K
		}
	default:
		return 0, 0, false
	}
	return outputPrice, inputPrice, validMoliiGrokPrice(outputPrice, inputPrice)
}

// GetMoliiGrokVideoPrices returns output/sec, input-image and input-video/sec prices.
func GetMoliiGrokVideoPrices(model, resolution string) (outputPrice, imageInputPrice, videoInputPrice float64, ok bool) {
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	switch model {
	case "grok-imagine-video-1.5":
		imageInputPrice = moliiGrokPriceSetting.Video15ImageInput
		switch resolution {
		case "480p":
			outputPrice = moliiGrokPriceSetting.Video15480p
		case "720p":
			outputPrice = moliiGrokPriceSetting.Video15720p
		case "1080p":
			outputPrice = moliiGrokPriceSetting.Video151080p
		default:
			return 0, 0, 0, false
		}
	case "grok-imagine-video":
		imageInputPrice = moliiGrokPriceSetting.VideoImageInput
		videoInputPrice = moliiGrokPriceSetting.VideoVideoInput
		switch resolution {
		case "480p":
			outputPrice = moliiGrokPriceSetting.Video480p
		case "720p":
			outputPrice = moliiGrokPriceSetting.Video720p
		default:
			return 0, 0, 0, false
		}
	default:
		return 0, 0, 0, false
	}
	return outputPrice, imageInputPrice, videoInputPrice, validMoliiGrokPrice(outputPrice, imageInputPrice, videoInputPrice)
}

// GetMoliiGrokCatalogPricing returns the configured direct price table for a
// Grok Imagine model. All values are CNY-denominated using the configured 1:1
// catalog convention.
func GetMoliiGrokCatalogPricing(model string) (*MoliiGrokCatalogPricing, bool) {
	switch model {
	case "grok-imagine-image", "grok-imagine-image-quality":
		oneK, input, ok := GetMoliiGrokImagePrices(model, "1k")
		if !ok {
			return nil, false
		}
		twoK, _, ok := GetMoliiGrokImagePrices(model, "2k")
		if !ok {
			return nil, false
		}
		return &MoliiGrokCatalogPricing{
			Kind: "image", OutputUnit: "image", OutputPrices: map[string]float64{"1k": oneK, "2k": twoK},
			ImageInputUnit: "image", ImageInputPrice: input,
		}, true
	case "grok-imagine-video", "grok-imagine-video-1.5":
		resolutions := []string{"480p", "720p"}
		if model == "grok-imagine-video-1.5" {
			resolutions = append(resolutions, "1080p")
		}
		prices := make(map[string]float64, len(resolutions))
		var imageInput, videoInput float64
		for _, resolution := range resolutions {
			output, imagePrice, videoPrice, ok := GetMoliiGrokVideoPrices(model, resolution)
			if !ok {
				return nil, false
			}
			prices[resolution] = output
			imageInput, videoInput = imagePrice, videoPrice
		}
		return &MoliiGrokCatalogPricing{
			Kind: "video", OutputUnit: "second", OutputPrices: prices,
			ImageInputUnit: "image", ImageInputPrice: imageInput,
			VideoInputUnit: "second", VideoInputPrice: videoInput,
		}, true
	default:
		return nil, false
	}
}
