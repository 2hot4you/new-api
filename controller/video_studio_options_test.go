package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVideoStudioMaskedKeyUsesClientFacingSKFormat(t *testing.T) {
	assert.Equal(t, "sk-abcdxxxxwxyz", videoStudioMaskedKey("abcdefghijklmnopwxyz"))
	assert.Equal(t, "sk-xxxx", videoStudioMaskedKey("short"))
}
