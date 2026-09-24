package seedanceprotocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterSupportedModelsRejectsUnrelatedAndUnknownModels(t *testing.T) {
	got := FilterSupportedModels([]string{
		"gpt-5.6-sol",
		" doubao-seedance-2-0-260128 ",
		"doubao-seedance-unknown",
		"doubao-seedance-2-0-260128",
	})
	assert.Equal(t, []string{"doubao-seedance-2-0-260128"}, got)
}

func TestSupportedModelsReturnsIndependentOrderedList(t *testing.T) {
	want := []string{
		"doubao-seedance-2-0-260128",
		"doubao-seedance-2-0-fast-260128",
		"doubao-seedance-2-0-mini-260615",
		"doubao-seedance-2-5-260628",
	}
	first := SupportedModels()
	assert.Equal(t, want, first)
	first[0] = "changed"
	assert.Equal(t, want, SupportedModels())
}

func TestPayloadSerializationPreservesSeedanceWireShape(t *testing.T) {
	audio, watermark, seconds := true, false, 5
	got, err := json.Marshal(Payload{
		Model: "doubao-seedance-2-0-260128", Content: []ContentItem{{Type: "text", Text: "prompt"}},
		GenerateAudio: &audio, Resolution: "720p", Ratio: "adaptive", Duration: &seconds, Watermark: &watermark,
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"prompt"}],"generate_audio":true,"resolution":"720p","ratio":"adaptive","duration":5,"watermark":false}`, string(got))
}

func TestDecodeResponseAcceptsWrappedProviderBodies(t *testing.T) {
	for _, body := range []string{
		"\xef\xbb\xbf{\"data\":{\"task_id\":\"task-1\"}}",
		"```json\n{\"data\":{\"task_id\":\"task-1\"}}\n```",
		"data: {\"data\":{\"task_id\":\"task-1\"}}\n\n",
	} {
		var got ResponseEnvelope
		require.NoError(t, DecodeResponse([]byte(body), &got))
		assert.Equal(t, "task-1", got.Data.TaskID)
	}
	var got ResponseEnvelope
	assert.Error(t, DecodeResponse([]byte("not JSON"), &got))
}

func TestValidatePayloadPreservesSeedanceRules(t *testing.T) {
	valid := &Payload{Model: "doubao-seedance-2-0-260128", Resolution: " 720P ", Ratio: "adaptive", Content: []ContentItem{{Type: "text", Text: "prompt"}}}
	require.NoError(t, ValidatePayload(valid))
	assert.Equal(t, "720p", valid.Resolution)

	invalid := &Payload{Model: "doubao-seedance-2-0-fast-260128", Resolution: "1080p", Ratio: "adaptive", Content: []ContentItem{{Type: "text", Text: "prompt"}}}
	assert.EqualError(t, ValidatePayload(invalid), "1080p is not supported by doubao-seedance-2-0-fast-260128")

	invalid = &Payload{Model: "doubao-seedance-2-0-260128", Resolution: "720p", Ratio: "adaptive", Content: []ContentItem{{Type: "image_url", ImageURL: &MediaURL{URL: "https://example.com/a.png"}, Role: "first_frame"}, {Type: "video_url", VideoURL: &MediaURL{URL: "https://example.com/b.mp4"}, Role: "reference_video"}}}
	assert.EqualError(t, ValidatePayload(invalid), "frame-based and multimodal reference content cannot be mixed")
}

func TestValidateDurationPreservesModelLimits(t *testing.T) {
	thirty, tooLong, smart := 30, 31, -1
	assert.NoError(t, ValidateDuration("doubao-seedance-2-5-260628", &thirty))
	assert.EqualError(t, ValidateDuration("doubao-seedance-2-5-260628", &tooLong), "duration must be -1 or between 4 and 30")
	assert.NoError(t, ValidateDuration("doubao-seedance-2-0-260128", &smart))
}
