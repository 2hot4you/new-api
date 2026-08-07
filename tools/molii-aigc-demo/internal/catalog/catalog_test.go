package catalog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogContainsExactModelsAndPreview(t *testing.T) {
	models := Models()
	require.Len(t, models, 7)
	want := []string{
		"doubao-seedance-2-0-260128", "doubao-seedance-2-0-fast-260128",
		"grok-imagine-image", "grok-imagine-image-quality", "grok-imagine-video",
		"grok-imagine-video-1.5", "grok-imagine-video-1.5-preview",
	}
	got := make([]string, 0, len(models))
	for _, model := range models {
		got = append(got, model.ID)
	}
	require.Equal(t, want, got)

	standard, ok := FindModel(want[0])
	require.True(t, ok)
	require.Len(t, standard.Operations, 4)
	require.Equal(t, "seedance.asset.create", standard.Operations[1].ID)

	preview, ok := FindModel("grok-imagine-video-1.5-preview")
	require.True(t, ok)
	require.Len(t, preview.Operations, 1)
	require.True(t, preview.Operations[0].Fields[2].Required)
}

func TestGrokVideoPromptsAreRequiredForGenerateAndEdit(t *testing.T) {
	for _, modelID := range []string{"grok-imagine-video", "grok-imagine-video-1.5", "grok-imagine-video-1.5-preview"} {
		model, ok := FindModel(modelID)
		require.True(t, ok)
		for _, operation := range model.Operations {
			var prompt *Field
			for i := range operation.Fields {
				if operation.Fields[i].Name == "prompt" {
					prompt = &operation.Fields[i]
					break
				}
			}
			require.NotNil(t, prompt, "%s/%s", modelID, operation.ID)
			require.True(t, prompt.Required, "%s/%s", modelID, operation.ID)
		}
	}
}
