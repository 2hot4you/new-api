package moliigrok

import "encoding/json"

type imageRequestPayload struct {
	Model       string            `json:"model"`
	Prompt      string            `json:"prompt"`
	AspectRatio string            `json:"aspect_ratio"`
	Resolution  string            `json:"resolution"`
	Quality     string            `json:"quality,omitempty"`
	N           int               `json:"n"`
	Image       *imageMediaInput  `json:"image,omitempty"`
	Images      []imageMediaInput `json:"images,omitempty"`
}

type rawImageRequest struct {
	Model       string          `json:"model"`
	Prompt      string          `json:"prompt"`
	AspectRatio string          `json:"aspect_ratio"`
	Resolution  string          `json:"resolution"`
	Quality     string          `json:"quality"`
	N           *int            `json:"n"`
	Image       json.RawMessage `json:"image,omitempty"`
	Images      json.RawMessage `json:"images,omitempty"`
}

type imageMediaInput struct {
	URL  string `json:"url,omitempty"`
	Type string `json:"type,omitempty"`
}

type imageResponsePayload struct {
	Data []imageResponseData `json:"data"`
}

type imageResponseData struct {
	URL           string `json:"url"`
	MimeType      string `json:"mime_type"`
	RevisedPrompt string `json:"revised_prompt"`
}
