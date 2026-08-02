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
