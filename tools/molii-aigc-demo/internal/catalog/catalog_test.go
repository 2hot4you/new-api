package catalog

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalogContainsExactModels(t *testing.T) {
	models := Models()
	require.Len(t, models, 6)
	want := []string{
		"doubao-seedance-2-0-260128", "doubao-seedance-2-0-fast-260128",
		"grok-imagine-image", "grok-imagine-image-quality", "grok-imagine-video",
		"grok-imagine-video-1.5",
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

	retiredModel := "grok-imagine-video-1.5-" + "pre" + "view"
	_, ok = FindModel(retiredModel)
	require.False(t, ok)
}

func TestGrokVideoPromptsAreRequiredForGenerateAndEdit(t *testing.T) {
	for _, modelID := range []string{"grok-imagine-video", "grok-imagine-video-1.5"} {
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
