package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"

	"molii-aigc-demo/internal/upstream"
)

type SeedancePriceRow struct {
	Resolutions  []string        `json:"resolutions"`
	WithoutVideo decimal.Decimal `json:"without_video"`
	WithVideo    decimal.Decimal `json:"with_video"`
}

type SeedancePricing struct {
	Unit        string             `json:"unit"`
	FPS         int                `json:"fps"`
	ExtraFrames int                `json:"extra_frames"`
	Rows        []SeedancePriceRow `json:"rows"`
}

type GrokPricing struct {
	Kind            string                     `json:"kind"`
	OutputUnit      string                     `json:"output_unit"`
	OutputPrices    map[string]decimal.Decimal `json:"output_prices"`
	ImageInputUnit  string                     `json:"image_input_unit"`
	ImageInputPrice decimal.Decimal            `json:"image_input_price"`
	VideoInputUnit  string                     `json:"video_input_unit"`
	VideoInputPrice decimal.Decimal            `json:"video_input_price"`
}

type ModelPricing struct {
	ModelName        string           `json:"model_name"`
	VideoPricing     *SeedancePricing `json:"video_pricing,omitempty"`
	MoliiGrokPricing *GrokPricing     `json:"molii_grok_pricing,omitempty"`
}

type Catalog struct {
	Models      map[string]ModelPricing
	GroupRatios map[string]decimal.Decimal
	Version     string
}

type pricingEnvelope struct {
	Success    bool                       `json:"success"`
	Message    string                     `json:"message"`
	Data       []ModelPricing             `json:"data"`
	GroupRatio map[string]decimal.Decimal `json:"group_ratio"`
	Version    string                     `json:"pricing_version"`
}

func FetchCatalog(ctx context.Context, client *upstream.Client, baseURL, apiKey string) (Catalog, error) {
	if client == nil {
		return Catalog{}, errors.New("upstream client is required")
	}
	result, err := client.Do(ctx, baseURL, apiKey, upstream.PreparedRequest{Operation: "pricing.fetch", Method: http.MethodGet, Path: "/api/pricing"})
	if err != nil {
		return Catalog{}, fmt.Errorf("fetch pricing: %w", err)
	}
	var envelope pricingEnvelope
	if err := json.Unmarshal(result.Body, &envelope); err != nil {
		return Catalog{}, fmt.Errorf("decode pricing: %w", err)
	}
	if !envelope.Success {
		return Catalog{}, fmt.Errorf("pricing API failed: %s", strings.TrimSpace(envelope.Message))
	}
	catalog := Catalog{Models: make(map[string]ModelPricing), GroupRatios: envelope.GroupRatio, Version: envelope.Version}
	for _, item := range envelope.Data {
		if item.VideoPricing != nil || item.MoliiGrokPricing != nil {
			catalog.Models[item.ModelName] = item
		}
	}
	return catalog, nil
}

type EstimateInput struct {
	Model           string
	Operation       string
	Resolution      string
	Quality         string
	Ratio           string
	Duration        int
	OutputCount     int
	InputImageCount int
	HasVideoInput   bool
	Group           string
}

type Estimate struct {
	Amount     decimal.Decimal `json:"amount"`
	Currency   string          `json:"currency"`
	Available  bool            `json:"available"`
	Adaptive   bool            `json:"adaptive"`
	Reason     string          `json:"reason,omitempty"`
	Formula    string          `json:"formula,omitempty"`
	GroupRatio decimal.Decimal `json:"group_ratio"`
	TokenCount int64           `json:"token_count,omitempty"`
}

func (c Catalog) Estimate(input EstimateInput) Estimate {
	result := Estimate{Currency: "CNY", GroupRatio: decimal.NewFromInt(1)}
	if ratio, ok := c.GroupRatios[input.Group]; ok && ratio.GreaterThanOrEqual(decimal.Zero) {
		result.GroupRatio = ratio
	}
	pricing, ok := c.Models[input.Model]
	if !ok {
		result.Reason = "pricing is unavailable for the selected model"
		return result
	}
	if pricing.VideoPricing != nil {
		return estimateSeedance(*pricing.VideoPricing, input, result)
	}
	if pricing.MoliiGrokPricing != nil {
		return estimateGrok(*pricing.MoliiGrokPricing, input, result)
	}
	result.Reason = "pricing dimensions are unavailable"
	return result
}

var seedanceDimensions = map[string]map[string][2]int64{
	"480p":  {"16:9": {864, 496}, "4:3": {752, 560}, "1:1": {640, 640}, "3:4": {560, 752}, "9:16": {496, 864}, "21:9": {992, 432}},
	"720p":  {"16:9": {1280, 720}, "4:3": {1112, 834}, "1:1": {960, 960}, "3:4": {834, 1112}, "9:16": {720, 1280}, "21:9": {1470, 630}},
	"1080p": {"16:9": {1920, 1080}, "4:3": {1440, 1080}, "1:1": {1080, 1080}, "3:4": {1080, 1440}, "9:16": {1080, 1920}, "21:9": {2520, 1080}},
	"4k":    {"16:9": {3840, 2160}, "4:3": {2880, 2160}, "1:1": {2160, 2160}, "3:4": {2160, 2880}, "9:16": {2160, 3840}, "21:9": {5040, 2160}},
}

func estimateSeedance(pricing SeedancePricing, input EstimateInput, result Estimate) Estimate {
	resolution := strings.ToLower(input.Resolution)
	adaptiveRatio := input.Ratio == "adaptive" || input.Ratio == ""
	smartDuration := input.Duration == -1
	if adaptiveRatio || smartDuration {
		result.Adaptive = true
		switch {
		case adaptiveRatio && smartDuration:
			result.Reason = "adaptive ratio and smart duration (-1) are selected; final dimensions, duration, and cost cannot be predicted exactly"
		case adaptiveRatio:
			result.Reason = "adaptive ratio is selected; final dimensions and cost cannot be predicted exactly"
		default:
			result.Reason = "smart duration (-1) is selected; final duration and cost cannot be predicted exactly"
		}
		return result
	}
	if input.Duration < 4 || input.Duration > 15 {
		result.Reason = "duration must be -1 or between 4 and 15"
		return result
	}
	dimensions, ok := seedanceDimensions[resolution][input.Ratio]
	if !ok {
		result.Reason = "resolution or ratio is not priced"
		return result
	}
	unitPrice, ok := seedanceUnitPrice(pricing.Rows, resolution, input.HasVideoInput)
	if !ok {
		result.Reason = "the selected Seedance pricing tier is unavailable"
		return result
	}
	fps := int64(pricing.FPS)
	if fps <= 0 {
		fps = 24
	}
	extra := int64(pricing.ExtraFrames)
	numerator := dimensions[0] * dimensions[1] * (fps*int64(input.Duration) + extra)
	tokens := numerator / 1024
	if numerator%1024 != 0 {
		tokens++
	}
	result.TokenCount = tokens
	result.Amount = decimal.NewFromInt(tokens).Mul(unitPrice).Div(decimal.NewFromInt(1_000_000)).Mul(result.GroupRatio)
	result.Available = true
	result.Formula = fmt.Sprintf("ceil(%d×%d×(%d×%d+%d)/1024) × %s / 1000000 × %s", dimensions[0], dimensions[1], fps, input.Duration, extra, unitPrice.String(), result.GroupRatio.String())
	return result
}

func seedanceUnitPrice(rows []SeedancePriceRow, resolution string, withVideo bool) (decimal.Decimal, bool) {
	for _, row := range rows {
		for _, candidate := range row.Resolutions {
			if strings.EqualFold(candidate, resolution) {
				if withVideo {
					return row.WithVideo, true
				}
				return row.WithoutVideo, true
			}
		}
	}
	return decimal.Zero, false
}

func estimateGrok(pricing GrokPricing, input EstimateInput, result Estimate) Estimate {
	resolution := strings.ToLower(input.Resolution)
	if quality := strings.ToLower(strings.TrimSpace(input.Quality)); quality != "" {
		resolution = quality + "/" + resolution
	}
	if input.Operation == "grok.video.edit" && resolution == "" {
		// New API precharges video edits at the 720p tier for 8.7 seconds,
		// then settles against the actual upstream duration and inferred tier.
		resolution = "720p"
	}
	outputPrice, ok := pricing.OutputPrices[resolution]
	if !ok {
		result.Reason = "the selected Grok resolution is not priced"
		return result
	}
	switch pricing.Kind {
	case "image":
		count := input.OutputCount
		if count == 0 {
			count = 1
		}
		amount := outputPrice.Mul(decimal.NewFromInt(int64(count)))
		if input.Operation == "grok.image.edit" {
			amount = amount.Add(pricing.ImageInputPrice.Mul(decimal.NewFromInt(int64(input.InputImageCount))))
		}
		result.Amount = amount.Mul(result.GroupRatio)
		result.Formula = fmt.Sprintf("(%s×%d + %s×%d inputs) × %s", outputPrice, count, pricing.ImageInputPrice, input.InputImageCount, result.GroupRatio)
	case "video":
		duration := decimal.NewFromInt(int64(input.Duration))
		if input.Operation == "grok.video.edit" {
			duration = decimal.NewFromFloat(8.7)
			result.Adaptive = true
			result.Reason = "video edit uses the upstream 8.7-second precharge estimate; actual media duration may differ"
		}
		if duration.LessThanOrEqual(decimal.Zero) {
			result.Reason = "duration is required for a video estimate"
			return result
		}
		amount := outputPrice.Mul(duration)
		if input.Operation == "grok.video.edit" {
			amount = amount.Add(pricing.VideoInputPrice.Mul(duration))
		} else if input.InputImageCount > 0 {
			amount = amount.Add(pricing.ImageInputPrice.Mul(decimal.NewFromInt(int64(input.InputImageCount))))
		}
		result.Amount = amount.Mul(result.GroupRatio)
		result.Formula = fmt.Sprintf("(%s×%s seconds + media input) × %s", outputPrice, duration, result.GroupRatio)
	default:
		result.Reason = "unknown Grok pricing kind"
		return result
	}
	result.Available = true
	return result
}
