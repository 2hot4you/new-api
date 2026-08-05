package moliigrok

type videoRequestPayload struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	Duration    int    `json:"duration"`
	AspectRatio string `json:"aspect_ratio"`
	Resolution  string `json:"resolution"`
}

type videoSubmitResponse struct {
	RequestID string                 `json:"request_id"`
	Error     *videoProviderError    `json:"error,omitempty"`
	Extra     map[string]interface{} `json:"-"`
}

type videoProviderError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type videoPollResponse struct {
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Model    string `json:"model,omitempty"`
	Video    struct {
		URL      string `json:"url"`
		Duration int    `json:"duration"`
	} `json:"video,omitempty"`
	Error *videoProviderError `json:"error,omitempty"`
}

type safeTaskData struct {
	Status   string `json:"status"`
	Progress int    `json:"progress"`
}
