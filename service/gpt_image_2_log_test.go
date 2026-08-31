package service

import (
	"encoding/json"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGPTImage2LogSnapshotNormalizesDocumentedDefaults(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		RequestURLPath:  "/v1/images/generations",
	}

	snapshot := BuildGPTImage2LogSnapshot(info, &dto.ImageRequest{})

	require.NotNil(t, snapshot)
	assert.Equal(t, 1, snapshot.Version)
	assert.Equal(t, "gpt-image-2", snapshot.Model)
	assert.Equal(t, "generation", snapshot.Operation)
	assert.Equal(t, "auto", snapshot.Quality)
	assert.Equal(t, "auto", snapshot.Background)
	assert.Equal(t, "png", snapshot.OutputFormat)
	assert.Equal(t, "auto", snapshot.Moderation)
	assert.Equal(t, "auto", snapshot.Size)
	assert.Equal(t, 1, snapshot.RequestedOutputCount)
	assert.Empty(t, snapshot.User)
}

func TestBuildGPTImage2LogSnapshotPreservesExplicitValuesAndEditOperation(t *testing.T) {
	n := uint(3)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		RequestURLPath:  "/v1/images/edits",
	}
	request := &dto.ImageRequest{
		N:            &n,
		Size:         "1536x1024",
		Quality:      "high",
		Background:   json.RawMessage(`"transparent"`),
		OutputFormat: json.RawMessage(`"webp"`),
		Moderation:   json.RawMessage(`"low"`),
		User:         json.RawMessage(`"customer-42"`),
	}

	snapshot := BuildGPTImage2LogSnapshot(info, request)

	require.NotNil(t, snapshot)
	assert.Equal(t, "edit", snapshot.Operation)
	assert.Equal(t, "high", snapshot.Quality)
	assert.Equal(t, "transparent", snapshot.Background)
	assert.Equal(t, "webp", snapshot.OutputFormat)
	assert.Equal(t, "low", snapshot.Moderation)
	assert.Equal(t, "1536x1024", snapshot.Size)
	assert.Equal(t, "customer-42", snapshot.User)
	assert.Equal(t, 3, snapshot.RequestedOutputCount)
}

func TestBuildGPTImage2LogSnapshotIgnoresOtherModels(t *testing.T) {
	info := &relaycommon.RelayInfo{OriginModelName: "gpt-image-1"}
	assert.Nil(t, BuildGPTImage2LogSnapshot(info, &dto.ImageRequest{}))
}

func TestAppendGPTImage2LogUsesActualOutputCount(t *testing.T) {
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-image-2",
		GPTImage2Log: &relaycommon.GPTImage2LogSnapshot{
			Version:              1,
			Model:                "gpt-image-2",
			Operation:            "generation",
			Quality:              "medium",
			Background:           "opaque",
			OutputFormat:         "jpeg",
			Moderation:           "auto",
			Size:                 "1024x1024",
			RequestedOutputCount: 3,
			OutputCount:          2,
		},
		GPTImage2PreviewAvailable: true,
	}
	other := map[string]interface{}{}

	content := appendGPTImage2Log(other, info)

	assert.Contains(t, content, "输出 2 张")
	assert.Same(t, info.GPTImage2Log, other["gpt_image_2"])
	assert.Equal(t, true, other["gpt_image_2_preview_available"])
}
