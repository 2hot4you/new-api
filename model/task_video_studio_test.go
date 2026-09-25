package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPrivateDataRoundTripsVideoStudioRequest(t *testing.T) {
	original := TaskPrivateData{VideoStudioRequest: &VideoStudioRequestSnapshot{
		Version:       1,
		RequestID:     "studio-request-1",
		Model:         "doubao-seedance-2-5-260628",
		Prompt:        "a calm lake",
		Resolution:    "720p",
		Ratio:         "16:9",
		Duration:      15,
		GenerateAudio: true,
		Media: []VideoStudioMediaReference{{
			Type: "image", Role: "reference_image", Source: "asset", AssetID: "asset-1", Name: "reference",
		}},
	}}

	encoded, err := original.Value()
	require.NoError(t, err)
	require.NotNil(t, encoded)

	var decoded TaskPrivateData
	require.NoError(t, decoded.Scan(encoded))
	require.NotNil(t, decoded.VideoStudioRequest)
	assert.Equal(t, original.VideoStudioRequest, decoded.VideoStudioRequest)
}
