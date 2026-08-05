package moliigrok

type imageRequestPayload struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	AspectRatio string `json:"aspect_ratio"`
	Resolution  string `json:"resolution"`
	N           int    `json:"n"`
}

type rawImageRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	AspectRatio string `json:"aspect_ratio"`
	Resolution  string `json:"resolution"`
	N           *int   `json:"n"`
}

type imageResponsePayload struct {
	Data []imageResponseData `json:"data"`
}

type imageResponseData struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
}
