package seedanceprotocol

import (
	"fmt"
	"strings"
)

type modelCapabilities struct {
	maxDuration         int
	maxImages           int
	maxVideos           int
	maxAudioFiles       int
	supportedResolution map[string]struct{}
}

// ModelCapabilities is the public, presentation-safe description consumed by
// the dashboard. Validation continues to use the same internal source of truth.
type ModelCapabilities struct {
	MaxDuration          int      `json:"max_duration"`
	MaxImages            int      `json:"max_images"`
	MaxVideos            int      `json:"max_videos"`
	MaxAudioFiles        int      `json:"max_audio_files"`
	Resolutions          []string `json:"resolutions"`
	Ratios               []string `json:"ratios"`
	SupportsAutoDuration bool     `json:"supports_auto_duration"`
	SupportsWebSearch    bool     `json:"supports_web_search"`
}

func CapabilitiesForModel(model string) (ModelCapabilities, bool) {
	if _, ok := modelSet[model]; !ok {
		return ModelCapabilities{}, false
	}
	internal := capabilitiesForModel(model)
	resolutionOrder := []string{"480p", "720p", "1080p", "4k"}
	resolutions := make([]string, 0, len(internal.supportedResolution))
	for _, resolution := range resolutionOrder {
		if _, ok := internal.supportedResolution[resolution]; ok {
			resolutions = append(resolutions, resolution)
		}
	}
	return ModelCapabilities{
		MaxDuration:          internal.maxDuration,
		MaxImages:            internal.maxImages,
		MaxVideos:            internal.maxVideos,
		MaxAudioFiles:        internal.maxAudioFiles,
		Resolutions:          resolutions,
		Ratios:               []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"},
		SupportsAutoDuration: true,
		SupportsWebSearch:    true,
	}, true
}

func capabilitiesForModel(model string) modelCapabilities {
	capabilities := modelCapabilities{
		maxDuration:   15,
		maxImages:     9,
		maxVideos:     3,
		maxAudioFiles: 3,
		supportedResolution: map[string]struct{}{
			"480p": {}, "720p": {}, "1080p": {}, "4k": {},
		},
	}
	switch model {
	case "doubao-seedance-2-0-fast-260128", "doubao-seedance-2-0-mini-260615":
		capabilities.supportedResolution = map[string]struct{}{"480p": {}, "720p": {}}
	case "doubao-seedance-2-5-260628":
		capabilities.maxDuration = 30
		capabilities.maxImages = 30
		capabilities.maxVideos = 10
		capabilities.maxAudioFiles = 10
		capabilities.supportedResolution = map[string]struct{}{"480p": {}, "720p": {}, "1080p": {}}
	}
	return capabilities
}

func ValidateDuration(model string, duration *int) error {
	if duration == nil {
		return nil
	}
	maxDuration := capabilitiesForModel(model).maxDuration
	if *duration != -1 && (*duration < 4 || *duration > maxDuration) {
		return fmt.Errorf("duration must be -1 or between 4 and %d", maxDuration)
	}
	return nil
}

func ValidatePayload(payload *Payload) error {
	payload.Resolution = strings.ToLower(strings.TrimSpace(payload.Resolution))
	if payload.Resolution != "480p" && payload.Resolution != "720p" && payload.Resolution != "1080p" && payload.Resolution != "4k" {
		return fmt.Errorf("resolution must be one of 480p, 720p, 1080p, or 4k")
	}
	if _, ok := capabilitiesForModel(payload.Model).supportedResolution[payload.Resolution]; !ok {
		return fmt.Errorf("%s is not supported by %s", payload.Resolution, payload.Model)
	}
	validRatios := map[string]struct{}{"16:9": {}, "4:3": {}, "1:1": {}, "3:4": {}, "9:16": {}, "21:9": {}, "adaptive": {}}
	if _, ok := validRatios[payload.Ratio]; !ok {
		return fmt.Errorf("unsupported ratio %q", payload.Ratio)
	}
	for _, item := range payload.Tools {
		if item.Type != "web_search" {
			return fmt.Errorf("unsupported tool type %q", item.Type)
		}
	}

	var textCount, imageCount, videoCount, audioCount int
	var firstFrameCount, lastFrameCount, referenceImageCount int
	for _, item := range payload.Content {
		switch strings.ToLower(strings.TrimSpace(item.Type)) {
		case "text":
			if strings.TrimSpace(item.Text) == "" {
				return fmt.Errorf("text content must not be empty")
			}
			textCount++
		case "image_url":
			if item.ImageURL == nil || strings.TrimSpace(item.ImageURL.URL) == "" {
				return fmt.Errorf("image_url content requires a URL")
			}
			imageCount++
			switch item.Role {
			case "", "first_frame":
				firstFrameCount++
			case "last_frame":
				lastFrameCount++
			case "reference_image":
				referenceImageCount++
			default:
				return fmt.Errorf("unsupported image role %q", item.Role)
			}
		case "video_url":
			if item.VideoURL == nil || strings.TrimSpace(item.VideoURL.URL) == "" {
				return fmt.Errorf("video_url content requires a URL")
			}
			if item.Role != "reference_video" {
				return fmt.Errorf("video role must be reference_video")
			}
			videoCount++
		case "audio_url":
			if item.AudioURL == nil || strings.TrimSpace(item.AudioURL.URL) == "" {
				return fmt.Errorf("audio_url content requires a URL")
			}
			if item.Role != "reference_audio" {
				return fmt.Errorf("audio role must be reference_audio")
			}
			audioCount++
		default:
			return fmt.Errorf("unsupported content type %q", item.Type)
		}
	}
	if textCount+imageCount+videoCount+audioCount == 0 || (imageCount+videoCount == 0 && textCount == 0) {
		return fmt.Errorf("prompt or reference image/video is required")
	}
	capabilities := capabilitiesForModel(payload.Model)
	if imageCount > capabilities.maxImages || videoCount > capabilities.maxVideos || audioCount > capabilities.maxAudioFiles {
		return fmt.Errorf(
			"at most %d images, %d videos, and %d audio files are supported",
			capabilities.maxImages,
			capabilities.maxVideos,
			capabilities.maxAudioFiles,
		)
	}
	frameScene := firstFrameCount > 0 || lastFrameCount > 0
	referenceScene := referenceImageCount > 0 || videoCount > 0 || audioCount > 0
	if frameScene && referenceScene {
		return fmt.Errorf("frame-based and multimodal reference content cannot be mixed")
	}
	if frameScene {
		if firstFrameCount != 1 || lastFrameCount > 1 || imageCount > 2 {
			return fmt.Errorf("frame-based generation requires one first frame and at most one last frame")
		}
	}
	if referenceScene {
		if imageCount != referenceImageCount {
			return fmt.Errorf("multimodal images must use role reference_image")
		}
		if audioCount > 0 && imageCount+videoCount == 0 {
			return fmt.Errorf("audio requires at least one reference image or video")
		}
	}
	return nil
}
