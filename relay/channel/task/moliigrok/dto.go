package moliigrok

type videoRequestPayload struct {
	Model       string      `json:"model"`
	Prompt      string      `json:"prompt"`
	Duration    int         `json:"duration"`
	AspectRatio string      `json:"aspect_ratio"`
	Resolution  string      `json:"resolution"`
	Image       *mediaInput `json:"image,omitempty"`
}

type videoEditRequestPayload struct {
	Model  string     `json:"model"`
	Prompt string     `json:"prompt"`
	Video  mediaInput `json:"video"`
}

type mediaInput struct {
	URL    string `json:"url,omitempty"`
	FileID string `json:"file_id,omitempty"`
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
		URL        string  `json:"url"`
		Duration   float64 `json:"duration"`
		Resolution string  `json:"resolution,omitempty"`
	} `json:"video,omitempty"`
	Usage struct {
		CostInUSDTicks int64 `json:"cost_in_usd_ticks"`
	} `json:"usage,omitempty"`
	Error *videoProviderError `json:"error,omitempty"`
}

type safeTaskData struct {
	Status   string `json:"status"`
	Progress int    `json:"progress"`
}
