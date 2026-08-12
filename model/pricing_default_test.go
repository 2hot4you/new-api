package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSeedanceDescriptionKeys(t *testing.T) {
	assert.Equal(t, "Seedance 2.0 standard model description", getDefaultModelDescriptionI18nKey("doubao-seedance-2-0-260128"))
	assert.Equal(t, "Seedance 2.0 fast model description", getDefaultModelDescriptionI18nKey("doubao-seedance-2-0-fast-260128"))
	assert.Empty(t, getDefaultModelDescriptionI18nKey("unrelated-model"))
}

func TestDefaultGrokDescriptionKeys(t *testing.T) {
	models := []string{
		"grok-imagine-image",
		"grok-imagine-image-quality",
		"grok-imagine-video",
		"grok-imagine-video-1.5",
	}
	for _, modelName := range models {
		assert.NotEmpty(t, getDefaultModelDescriptionI18nKey(modelName), modelName)
	}
	assert.Empty(t, getDefaultModelDescriptionI18nKey("grok-imagine-video-1.5-preview"))
}

func TestDefaultThreeModelDescriptionKeys(t *testing.T) {
	want := map[string]string{
		"minimax-m3":    "MiniMax M3 model description",
		"qwen3.5-flash": "Qwen3.5 Flash model description",
		"qwen3.5-plus":  "Qwen3.5 Plus model description",
	}
	for modelName, descriptionKey := range want {
		assert.Equal(t, descriptionKey, getDefaultModelDescriptionI18nKey(modelName))
	}
	assert.Empty(t, getDefaultModelDescriptionI18nKey("qwen3.5-plus-preview"))
}

func TestDefaultTextModelDescriptionKeys(t *testing.T) {
	want := map[string]string{
		"deepseek-v4-flash-202605": "DeepSeek V4 Flash model description",
		"deepseek-v4-pro-202606":   "DeepSeek V4 Pro model description",
		"glm-5.2":                  "GLM 5.2 model description",
		"kimi-k3":                  "Kimi K3 model description",
	}
	for modelName, descriptionKey := range want {
		assert.Equal(t, descriptionKey, getDefaultModelDescriptionI18nKey(modelName))
	}
	assert.Empty(t, getDefaultModelDescriptionI18nKey("deepseek-v4-preview"))
}
