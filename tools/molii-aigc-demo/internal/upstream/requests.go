package upstream

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

const maxPromptRunes = 10000

type PreparedRequest struct {
	Operation  string          `json:"operation"`
	Method     string          `json:"method"`
	Path       string          `json:"path"`
	Body       json.RawMessage `json:"body,omitempty"`
	Generation bool            `json:"generation"`
	Async      bool            `json:"async"`
}

type mediaRef struct {
	URL    string `json:"url,omitempty"`
	FileID string `json:"file_id,omitempty"`
	Type   string `json:"type,omitempty"`
}

func (m *mediaRef) UnmarshalJSON(data []byte) error {
	var direct string
	if err := json.Unmarshal(data, &direct); err == nil {
		m.URL = strings.TrimSpace(direct)
		if m.URL == "" {
			return errors.New("media URL must not be empty")
		}
		return nil
	}
	type alias mediaRef
	var value alias
	if err := decodeStrict(data, &value); err != nil {
		return errors.New("media must be a URL string or an object containing url or file_id")
	}
	value.URL = strings.TrimSpace(value.URL)
	value.FileID = strings.TrimSpace(value.FileID)
	if (value.URL == "") == (value.FileID == "") {
		return errors.New("media must contain exactly one of url or file_id")
	}
	*m = mediaRef(value)
	return nil
}

type contentItem struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *mediaURL `json:"image_url,omitempty"`
	VideoURL *mediaURL `json:"video_url,omitempty"`
	AudioURL *mediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type mediaURL struct {
	URL string `json:"url"`
}

type tool struct {
	Type string `json:"type"`
}

type seedanceRequest struct {
	Model         string        `json:"model"`
	Content       []contentItem `json:"content,omitempty"`
	Prompt        string        `json:"prompt,omitempty"`
	GenerateAudio *bool         `json:"generate_audio,omitempty"`
	Resolution    string        `json:"resolution,omitempty"`
	Ratio         string        `json:"ratio,omitempty"`
	Duration      *int          `json:"duration,omitempty"`
	Watermark     *bool         `json:"watermark,omitempty"`
	Tools         []tool        `json:"tools,omitempty"`
}

type grokImageRequest struct {
	Model       string     `json:"model"`
	Prompt      string     `json:"prompt"`
	AspectRatio string     `json:"aspect_ratio"`
	Resolution  string     `json:"resolution"`
	N           *int       `json:"n,omitempty"`
	Image       *mediaRef  `json:"image,omitempty"`
	Images      []mediaRef `json:"images,omitempty"`
}

type grokVideoRequest struct {
	Model       string    `json:"model"`
	Prompt      string    `json:"prompt"`
	Image       *mediaRef `json:"image,omitempty"`
	Duration    int       `json:"duration"`
	AspectRatio string    `json:"aspect_ratio"`
	Resolution  string    `json:"resolution"`
}

type grokVideoEditRequest struct {
	Model  string   `json:"model"`
	Prompt string   `json:"prompt"`
	Video  mediaRef `json:"video"`
}

type assetCreateRequest struct {
	URL       string `json:"url"`
	AssetType string `json:"asset_type"`
	Name      string `json:"name"`
}

func decodeStrict(raw []byte, target any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func marshalPrepared(operation, method, path string, payload any, generation, async bool) (PreparedRequest, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return PreparedRequest{}, err
	}
	return PreparedRequest{Operation: operation, Method: method, Path: path, Body: body, Generation: generation, Async: async}, nil
}

// Build validates raw UI JSON, applies the same defaults as the New API
// adapters, and constructs an exact upstream request.
func Build(operation string, raw []byte) (PreparedRequest, error) {
	switch operation {
	case "seedance.video.generate":
		var req seedanceRequest
		if err := decodeStrict(raw, &req); err != nil {
			return PreparedRequest{}, fmt.Errorf("invalid Seedance request: %w", err)
		}
		if err := validateSeedance(&req); err != nil {
			return PreparedRequest{}, err
		}
		return marshalPrepared(operation, http.MethodPost, "/v1/video/generations", req, true, true)
	case "grok.image.generate", "grok.image.edit":
		var req grokImageRequest
		if err := decodeStrict(raw, &req); err != nil {
			return PreparedRequest{}, fmt.Errorf("invalid Grok image request: %w", err)
		}
		edit := operation == "grok.image.edit"
		if err := validateGrokImage(&req, edit); err != nil {
			return PreparedRequest{}, err
		}
		path := "/v1/images/generations"
		if edit {
			path = "/v1/images/edits"
		}
		return marshalPrepared(operation, http.MethodPost, path, req, true, false)
	case "grok.video.generate":
		var req grokVideoRequest
		if err := decodeStrict(raw, &req); err != nil {
			return PreparedRequest{}, fmt.Errorf("invalid Grok video request: %w", err)
		}
		if err := validateGrokVideo(&req); err != nil {
			return PreparedRequest{}, err
		}
		return marshalPrepared(operation, http.MethodPost, "/v1/videos", req, true, true)
	case "grok.video.edit":
		var req grokVideoEditRequest
		if err := decodeStrict(raw, &req); err != nil {
			return PreparedRequest{}, fmt.Errorf("invalid Grok video edit request: %w", err)
		}
		if req.Model != "grok-imagine-video" {
			return PreparedRequest{}, errors.New("video editing requires grok-imagine-video")
		}
		if err := validatePrompt(req.Prompt, true); err != nil {
			return PreparedRequest{}, err
		}
		if req.Video.URL == "" && req.Video.FileID == "" {
			return PreparedRequest{}, errors.New("video is required")
		}
		return marshalPrepared(operation, http.MethodPost, "/v1/videos/edits", req, true, true)
	case "seedance.asset.create":
		var req assetCreateRequest
		if err := decodeStrict(raw, &req); err != nil {
			return PreparedRequest{}, fmt.Errorf("invalid asset request: %w", err)
		}
		if err := validateAsset(&req); err != nil {
			return PreparedRequest{}, err
		}
		return marshalPrepared(operation, http.MethodPost, "/v1/assets", req, false, false)
	default:
		return PreparedRequest{}, fmt.Errorf("unsupported operation %q", operation)
	}
}

// BuildResource constructs path-parameter requests without putting identifiers
// into query strings.
func BuildResource(operation, id string) (PreparedRequest, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, "/?#") {
		return PreparedRequest{}, errors.New("invalid resource ID")
	}
	escaped := url.PathEscape(id)
	switch operation {
	case "seedance.asset.get":
		return PreparedRequest{Operation: operation, Method: http.MethodGet, Path: "/v1/assets/" + escaped}, nil
	case "seedance.asset.delete":
		return PreparedRequest{Operation: operation, Method: http.MethodDelete, Path: "/v1/assets/" + escaped}, nil
	case "video.get":
		return PreparedRequest{Operation: operation, Method: http.MethodGet, Path: "/v1/videos/" + escaped, Async: true}, nil
	case "video.content":
		return PreparedRequest{Operation: operation, Method: http.MethodGet, Path: "/v1/videos/" + escaped + "/content", Async: true}, nil
	default:
		return PreparedRequest{}, fmt.Errorf("unsupported resource operation %q", operation)
	}
}

func validatePrompt(prompt string, required bool) error {
	prompt = strings.TrimSpace(prompt)
	if required && prompt == "" {
		return errors.New("prompt is required")
	}
	if utf8.RuneCountInString(prompt) > maxPromptRunes {
		return fmt.Errorf("prompt must not exceed %d characters", maxPromptRunes)
	}
	return nil
}

func validateSeedance(req *seedanceRequest) error {
	if req.Model != "doubao-seedance-2-0-260128" && req.Model != "doubao-seedance-2-0-fast-260128" {
		return errors.New("unsupported Seedance model")
	}
	if req.GenerateAudio == nil {
		v := true
		req.GenerateAudio = &v
	}
	if req.Watermark == nil {
		v := false
		req.Watermark = &v
	}
	if req.Resolution == "" {
		req.Resolution = "720p"
	}
	allowedResolution := map[string]bool{"480p": true, "720p": true, "1080p": true, "4k": true}
	if !allowedResolution[req.Resolution] {
		return errors.New("resolution must be one of 480p, 720p, 1080p, or 4k")
	}
	if req.Model == "doubao-seedance-2-0-fast-260128" && (req.Resolution == "1080p" || req.Resolution == "4k") {
		return fmt.Errorf("%s is not supported by %s", req.Resolution, req.Model)
	}
	if req.Ratio == "" {
		req.Ratio = "adaptive"
	}
	allowedRatio := map[string]bool{"16:9": true, "4:3": true, "1:1": true, "3:4": true, "9:16": true, "21:9": true, "adaptive": true}
	if !allowedRatio[req.Ratio] {
		return fmt.Errorf("unsupported ratio %q", req.Ratio)
	}
	if req.Duration == nil {
		v := 5
		req.Duration = &v
	}
	if *req.Duration != -1 && (*req.Duration < 4 || *req.Duration > 15) {
		return errors.New("duration must be -1 or between 4 and 15")
	}
	for _, item := range req.Tools {
		if item.Type != "web_search" {
			return fmt.Errorf("unsupported tool type %q", item.Type)
		}
	}
	return validateSeedanceContent(req)
}

func validateSeedanceContent(req *seedanceRequest) error {
	textCount, imageCount, videoCount, audioCount := 0, 0, 0, 0
	firstCount, lastCount, refImageCount := 0, 0, 0
	if strings.TrimSpace(req.Prompt) != "" {
		textCount++
	}
	for _, item := range req.Content {
		switch strings.TrimSpace(item.Type) {
		case "text":
			if strings.TrimSpace(item.Text) == "" {
				return errors.New("text content must not be empty")
			}
			textCount++
		case "image_url":
			if item.ImageURL == nil || strings.TrimSpace(item.ImageURL.URL) == "" {
				return errors.New("image_url content requires a URL")
			}
			imageCount++
			switch item.Role {
			case "", "first_frame":
				firstCount++
			case "last_frame":
				lastCount++
			case "reference_image":
				refImageCount++
			default:
				return fmt.Errorf("unsupported image role %q", item.Role)
			}
		case "video_url":
			if item.VideoURL == nil || strings.TrimSpace(item.VideoURL.URL) == "" || item.Role != "reference_video" {
				return errors.New("video_url requires a URL and reference_video role")
			}
			videoCount++
		case "audio_url":
			if item.AudioURL == nil || strings.TrimSpace(item.AudioURL.URL) == "" || item.Role != "reference_audio" {
				return errors.New("audio_url requires a URL and reference_audio role")
			}
			audioCount++
		default:
			return fmt.Errorf("unsupported content type %q", item.Type)
		}
	}
	if textCount+imageCount+videoCount == 0 {
		return errors.New("prompt or reference image/video is required")
	}
	if imageCount > 9 || videoCount > 3 || audioCount > 3 {
		return errors.New("at most 9 images, 3 videos, and 3 audio files are supported")
	}
	frameScene := firstCount > 0 || lastCount > 0
	referenceScene := refImageCount > 0 || videoCount > 0 || audioCount > 0
	if frameScene && referenceScene {
		return errors.New("frame-based and multimodal reference content cannot be mixed")
	}
	if frameScene && (firstCount != 1 || lastCount > 1 || imageCount > 2) {
		return errors.New("frame-based generation requires one first frame and at most one last frame")
	}
	if referenceScene && imageCount != refImageCount {
		return errors.New("multimodal images must use role reference_image")
	}
	if audioCount > 0 && imageCount+videoCount == 0 {
		return errors.New("audio requires at least one reference image or video")
	}
	return nil
}

func validateGrokImage(req *grokImageRequest, edit bool) error {
	if req.Model != "grok-imagine-image" && req.Model != "grok-imagine-image-quality" {
		return errors.New("unsupported Grok image model")
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if err := validatePrompt(req.Prompt, true); err != nil {
		return err
	}
	if req.AspectRatio == "" {
		req.AspectRatio = "16:9"
	}
	allowed := map[string]bool{"1:1": true, "16:9": true, "9:16": true, "4:3": true, "3:4": true, "3:2": true, "2:3": true, "2:1": true, "1:2": true, "19.5:9": true, "9:19.5": true, "20:9": true, "9:20": true, "auto": true}
	if !allowed[req.AspectRatio] {
		return errors.New("aspect_ratio is not supported")
	}
	req.Resolution = strings.ToLower(strings.TrimSpace(req.Resolution))
	if req.Resolution == "" {
		req.Resolution = "1k"
	}
	if req.Resolution != "1k" && req.Resolution != "2k" {
		return errors.New("resolution must be one of 1k or 2k")
	}
	if req.N == nil {
		one := 1
		req.N = &one
	}
	if *req.N < 1 || *req.N > 4 {
		return errors.New("n must be an integer between 1 and 4")
	}
	inputs := len(req.Images)
	if req.Image != nil {
		inputs++
	}
	if edit && (inputs < 1 || inputs > 3) {
		return errors.New("image edits require between 1 and 3 input images")
	}
	if !edit && inputs != 0 {
		return errors.New("image inputs are only supported for edits")
	}
	return nil
}

func validateGrokVideo(req *grokVideoRequest) error {
	if req.Model != "grok-imagine-video" && req.Model != "grok-imagine-video-1.5" {
		return errors.New("unsupported Grok video model")
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if err := validatePrompt(req.Prompt, true); err != nil {
		return err
	}
	if req.Model != "grok-imagine-video" && req.Image == nil {
		return errors.New("Grok Imagine Video 1.5 models require an image")
	}
	if req.Duration == 0 {
		req.Duration = 5
	}
	if req.Duration < 1 || req.Duration > 15 {
		return errors.New("duration must be between 1 and 15 seconds")
	}
	if req.AspectRatio == "" {
		req.AspectRatio = "16:9"
	}
	if len(req.AspectRatio) > 32 {
		return errors.New("invalid aspect_ratio")
	}
	if req.Resolution == "" {
		req.Resolution = "480p"
	}
	allowed := map[string]bool{"480p": true, "720p": true}
	if req.Model != "grok-imagine-video" {
		allowed["1080p"] = true
	}
	if !allowed[strings.ToLower(req.Resolution)] {
		return errors.New("resolution is not supported by the selected model")
	}
	req.Resolution = strings.ToLower(req.Resolution)
	return nil
}

func validateAsset(req *assetCreateRequest) error {
	parsed, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return errors.New("url must be a public HTTP or HTTPS URL without credentials")
	}
	if req.AssetType != "image" && req.AssetType != "video" && req.AssetType != "audio" {
		return errors.New("asset_type must be image, video, or audio")
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 80 {
		return errors.New("name is required and must not exceed 80 characters")
	}
	return nil
}

// CurlPreview never embeds an API key; shell expansion happens only when the
// user executes the preview.
func CurlPreview(baseURL string, req PreparedRequest) (string, error) {
	base, err := normalizeBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("curl")
	if req.Method != http.MethodGet {
		b.WriteString(" -X ")
		b.WriteString(req.Method)
	}
	b.WriteString(" '")
	b.WriteString(base.ResolveReference(&url.URL{Path: req.Path}).String())
	b.WriteString("' \\\n  -H 'Authorization: Bearer $MOLII_API_KEY' \\\n  -H 'Accept: application/json'")
	if len(req.Body) > 0 {
		b.WriteString(" \\\n  -H 'Content-Type: application/json' \\\n  --data '")
		b.WriteString(strings.ReplaceAll(string(req.Body), "'", "'\\''"))
		b.WriteString("'")
	}
	return b.String(), nil
}
