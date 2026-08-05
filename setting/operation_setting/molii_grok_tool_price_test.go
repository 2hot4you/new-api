package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultMoliiGrokToolPrices(t *testing.T) {
	model := "grok-4.5"
	assert.Equal(t, 5.0, GetToolPriceForModel("web_search", model))
	assert.Equal(t, 5.0, GetToolPriceForModel("x_search", model))
	assert.Equal(t, 5.0, GetToolPriceForModel("code_interpreter", model))
	assert.Equal(t, 5.0, GetToolPriceForModel("code_execution", model))
	assert.Equal(t, 10.0, GetToolPriceForModel("attachment_search", model))
	assert.Equal(t, 2.5, GetToolPriceForModel("collections_search", model))
	assert.Equal(t, 50.0, GetToolPriceForModel("image_generation", model))
	assert.Equal(t, defaultWebSearchToolPrice, GetToolPriceForModel("web_search", "gpt-4o"))
}
