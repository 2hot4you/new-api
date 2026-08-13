package common

import (
	"io"
	"testing"

	basecommon "github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOutboundJSONBodyReplayReadersAreIndependent(t *testing.T) {
	payload := []byte(`{"model":"test-model","input":"abcdefghijklmnopqrstuvwxyz"}`)
	body, closer, err := NewOutboundJSONBody(payload)
	require.NoError(t, err)
	defer closer.Close()

	assert.EqualValues(t, len(payload), body.Size())
	half := len(payload) / 2
	primaryHead := make([]byte, half)
	_, err = io.ReadFull(body, primaryHead)
	require.NoError(t, err)

	a, err := body.NewReader()
	require.NoError(t, err)
	b, err := body.NewReader()
	require.NoError(t, err)

	aHead := make([]byte, half)
	_, err = io.ReadFull(a, aHead)
	require.NoError(t, err)
	bAll, err := io.ReadAll(b)
	require.NoError(t, err)
	require.NoError(t, b.Close())
	aRest, err := io.ReadAll(a)
	require.NoError(t, err)
	require.NoError(t, a.Close())
	primaryRest, err := io.ReadAll(body)
	require.NoError(t, err)

	assert.Equal(t, payload, bAll)
	assert.Equal(t, payload[half:], aRest)
	assert.Equal(t, payload[half:], primaryRest)

	require.NoError(t, closer.Close())
	_, err = body.NewReader()
	require.ErrorIs(t, err, basecommon.ErrStorageClosed)
}
