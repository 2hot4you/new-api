package catalog

// Field describes one UI-editable request field. Conditions use field names
// from the same operation and are intentionally simple so a native client can
// render them without evaluating code.
type Field struct {
	Name        string         `json:"name"`
	Label       string         `json:"label"`
	Type        string         `json:"type"`
	Required    bool           `json:"required,omitempty"`
	Default     any            `json:"default,omitempty"`
	Options     []string       `json:"options,omitempty"`
	Minimum     *int           `json:"minimum,omitempty"`
	Maximum     *int           `json:"maximum,omitempty"`
	ItemType    string         `json:"item_type,omitempty"`
	Description string         `json:"description,omitempty"`
	Condition   map[string]any `json:"condition,omitempty"`
}

type Operation struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Async       bool    `json:"async,omitempty"`
	Generation  bool    `json:"generation,omitempty"`
	Description string  `json:"description,omitempty"`
	Fields      []Field `json:"fields"`
}

type Model struct {
	ID          string      `json:"id"`
	Label       string      `json:"label"`
	Provider    string      `json:"provider"`
	Kind        string      `json:"kind"`
	Description string      `json:"description,omitempty"`
	Operations  []Operation `json:"operations"`
}

func intp(v int) *int { return &v }

var imageRatios = []string{"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20", "auto"}
var seedanceRatios = []string{"16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"}

func seedanceFields(resolutions []string) []Field {
	return []Field{
		{Name: "model", Label: "Model", Type: "select", Required: true},
		{Name: "prompt", Label: "Prompt", Type: "textarea", Description: "At least prompt or valid content is required."},
		{Name: "content", Label: "Ordered content", Type: "array", ItemType: "seedance_content", Description: "text, first/last frame, reference image/video/audio"},
		{Name: "generate_audio", Label: "Generate audio", Type: "boolean", Default: true},
		{Name: "resolution", Label: "Resolution", Type: "select", Default: "720p", Options: resolutions},
		{Name: "ratio", Label: "Aspect ratio", Type: "select", Default: "adaptive", Options: seedanceRatios},
		{Name: "duration", Label: "Duration", Type: "integer", Default: 5, Minimum: intp(-1), Maximum: intp(15), Description: "-1 or an integer from 4 through 15."},
		{Name: "watermark", Label: "Watermark", Type: "boolean", Default: false},
		{Name: "tools", Label: "Tools", Type: "array", ItemType: "web_search", Description: "Only {\"type\":\"web_search\"} is supported."},
	}
}

func grokImageOperations() []Operation {
	common := []Field{
		{Name: "model", Label: "Model", Type: "select", Required: true},
		{Name: "prompt", Label: "Prompt", Type: "textarea", Required: true, Maximum: intp(10000)},
		{Name: "aspect_ratio", Label: "Aspect ratio", Type: "select", Default: "16:9", Options: imageRatios},
		{Name: "resolution", Label: "Resolution", Type: "select", Default: "1k", Options: []string{"1k", "2k"}},
		{Name: "n", Label: "Output count", Type: "integer", Default: 1, Minimum: intp(1), Maximum: intp(4)},
	}
	edit := append([]Field(nil), common...)
	edit = append(edit,
		Field{Name: "image", Label: "Input image", Type: "media"},
		Field{Name: "images", Label: "Input images", Type: "array", ItemType: "media", Description: "Use image and/or images for a total of 1–3 inputs."},
	)
	return []Operation{
		{ID: "grok.image.generate", Label: "Image generation", Method: "POST", Path: "/v1/images/generations", Generation: true, Fields: common},
		{ID: "grok.image.edit", Label: "Image edit", Method: "POST", Path: "/v1/images/edits", Generation: true, Fields: edit},
	}
}

func grokVideoOperations(model string) []Operation {
	resolutions := []string{"480p", "720p"}
	imageRequired := false
	if model == "grok-imagine-video-1.5" {
		resolutions = append(resolutions, "1080p")
		imageRequired = true
	}
	fields := []Field{
		{Name: "model", Label: "Model", Type: "select", Required: true},
		{Name: "prompt", Label: "Prompt", Type: "textarea", Required: true, Maximum: intp(10000)},
		{Name: "image", Label: "Input image", Type: "media", Required: imageRequired},
		{Name: "duration", Label: "Duration", Type: "integer", Default: 5, Minimum: intp(1), Maximum: intp(15)},
		{Name: "aspect_ratio", Label: "Aspect ratio", Type: "text", Default: "16:9", Maximum: intp(32)},
		{Name: "resolution", Label: "Resolution", Type: "select", Default: "480p", Options: resolutions},
	}
	ops := []Operation{{ID: "grok.video.generate", Label: "Video generation", Method: "POST", Path: "/v1/videos", Async: true, Generation: true, Fields: fields}}
	if model == "grok-imagine-video" {
		ops = append(ops, Operation{ID: "grok.video.edit", Label: "Video edit", Method: "POST", Path: "/v1/videos/edits", Async: true, Generation: true, Fields: []Field{
			{Name: "model", Label: "Model", Type: "select", Required: true},
			{Name: "prompt", Label: "Prompt", Type: "textarea", Required: true, Maximum: intp(10000)},
			{Name: "video", Label: "Input video", Type: "media", Required: true},
		}})
	}
	return ops
}

// Models returns a detached catalog safe for JSON encoding.
func Models() []Model {
	assetFields := []Field{
		{Name: "url", Label: "Public media URL", Type: "url", Required: true},
		{Name: "asset_type", Label: "Asset type", Type: "select", Required: true, Options: []string{"image", "video", "audio"}},
		{Name: "name", Label: "Name", Type: "text", Required: true, Maximum: intp(80)},
	}
	seedanceAssets := []Operation{
		{ID: "seedance.asset.create", Label: "Create temporary asset", Method: "POST", Path: "/v1/assets", Fields: assetFields},
		{ID: "seedance.asset.get", Label: "Get temporary asset", Method: "GET", Path: "/v1/assets/{id}", Fields: []Field{{Name: "id", Label: "Asset ID", Type: "text", Required: true}}},
		{ID: "seedance.asset.delete", Label: "Delete temporary asset", Method: "DELETE", Path: "/v1/assets/{id}", Fields: []Field{{Name: "id", Label: "Asset ID", Type: "text", Required: true}}},
	}
	seedanceOps := func(resolutions []string) []Operation {
		return append([]Operation{{ID: "seedance.video.generate", Label: "Video generation", Method: "POST", Path: "/v1/video/generations", Async: true, Generation: true, Fields: seedanceFields(resolutions)}}, seedanceAssets...)
	}
	return []Model{
		{ID: "doubao-seedance-2-0-260128", Label: "Seedance 2.0", Provider: "seedance", Kind: "video", Operations: seedanceOps([]string{"480p", "720p", "1080p", "4k"})},
		{ID: "doubao-seedance-2-0-fast-260128", Label: "Seedance 2.0 Fast", Provider: "seedance", Kind: "video", Operations: seedanceOps([]string{"480p", "720p"})},
		{ID: "grok-imagine-image", Label: "Grok Imagine Image", Provider: "grok", Kind: "image", Operations: grokImageOperations()},
		{ID: "grok-imagine-image-quality", Label: "Grok Imagine Image Quality", Provider: "grok", Kind: "image", Operations: grokImageOperations()},
		{ID: "grok-imagine-video", Label: "Grok Imagine Video", Provider: "grok", Kind: "video", Operations: grokVideoOperations("grok-imagine-video")},
		{ID: "grok-imagine-video-1.5", Label: "Grok Imagine Video 1.5", Provider: "grok", Kind: "video", Operations: grokVideoOperations("grok-imagine-video-1.5")},
	}
}

func FindModel(id string) (Model, bool) {
	for _, model := range Models() {
		if model.ID == id {
			return model, true
		}
	}
	return Model{}, false
}
