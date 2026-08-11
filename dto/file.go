package dto

type FileObject struct {
	ID              string  `json:"id"`
	Object          string  `json:"object"`
	Bytes           int64   `json:"bytes"`
	CreatedAt       int64   `json:"created_at"`
	ExpiresAt       int64   `json:"expires_at"`
	Filename        string  `json:"filename"`
	Purpose         string  `json:"purpose"`
	MIMEType        string  `json:"mime_type,omitempty"`
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
}

type FileList struct {
	Object string       `json:"object"`
	Data   []FileObject `json:"data"`
}

type FileDeleted struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}
