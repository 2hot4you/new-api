package jsplugin

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryPinSourceIdentityAndPublication(t *testing.T) {
	registry := NewRegistry()
	source := routingTestPluginSource("pin", 0, `["model"]`, "", "")
	_, err := registry.Register(source, Options{})
	require.NoError(t, err)
	pin, err := registry.TryPinGeneration()
	require.NoError(t, err)
	t.Cleanup(pin.Release)
	require.Len(t, pin.Overrides, 1)
	assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(source))), pin.Overrides["pin"].SourceHash)
	assert.Equal(t, "success", pin.Status)
	assert.True(t, pin.Enabled)
	assert.False(t, registry.mu.TryLock(), "publication must remain excluded for the caller's complete transaction")
	pin.Release()
	pin.Release()
	require.True(t, registry.mu.TryLock(), "release must unblock publication exactly once")
	_, err = registry.TryPinGeneration()
	require.ErrorIs(t, err, ErrGenerationBusy, "pin must fail immediately while a writer owns the registry")
	registry.mu.Unlock()
	_, err = registry.Register(source+"\n// distinct exact compiled bytes", Options{})
	require.NoError(t, err)
	next, err := registry.TryPinGeneration()
	require.NoError(t, err)
	defer next.Release()
	assert.NotEqual(t, pin.Overrides["pin"].SourceHash, next.Overrides["pin"].SourceHash)
	assert.Equal(t, pin.Overrides["pin"].Meta.Version, next.Overrides["pin"].Meta.Version)
	assert.Greater(t, next.Generation.Number, pin.Generation.Number)
}

func TestRegistryPinReportsDesiredAndRetainedEffectiveIdentity(t *testing.T) {
	registry := NewRegistry()
	incumbent := mustCompileRoutingPlugin(t, "incumbent", 70, `["model-v1"]`, "", "")
	other := mustCompileRoutingPlugin(t, "other", 71, `["other-model"]`, "", "")
	require.NoError(t, registry.ReplaceOverrides([]*LoadedPlugin{incumbent, other}))
	conflicting := mustCompileRoutingPlugin(t, "incumbent", 71, `["model-v2"]`, "", "")
	require.NoError(t, registry.ReplaceOverrides([]*LoadedPlugin{conflicting, other}))
	pin, err := registry.TryPinGeneration()
	require.NoError(t, err)
	t.Cleanup(pin.Release)
	assert.Equal(t, "partial", pin.Status)
	assert.Equal(t, []string{"model-v2"}, pin.Overrides["incumbent"].Meta.Models)
	assert.Equal(t, []string{"model-v1"}, pin.Effective["incumbent"].Meta.Models)
	assert.NotEqual(t, pin.Overrides["incumbent"].SourceHash, pin.Effective["incumbent"].SourceHash)
	pin.Effective["incumbent"].Meta.Models[0] = "caller-owned-copy"
	assert.Equal(t, []string{"model-v1"}, incumbent.Meta.Models)
	pin.Release()
	require.Error(t, registry.SetGenerationPreparer(func(_, _ *RoutingGeneration) (PreparedRoutingGeneration, error) {
		return PreparedRoutingGeneration{}, fmt.Errorf("fixture rebuild failure")
	}))
	failed, err := registry.TryPinGeneration()
	require.NoError(t, err)
	defer failed.Release()
	assert.Equal(t, "failed", failed.Status)
	assert.Equal(t, incumbent.Engine.sourceHash, failed.Effective["incumbent"].SourceHash)
}

func TestRegistryPinIncludesFactoryAndMasterSwitches(t *testing.T) {
	registry := NewRegistry()
	require.NoError(t, registerTestPlugin(registry, "1.0.0-factory", true))
	registry.SetDisabledFactoryKeys([]string{"test"})
	registry.SetEnabled(false)
	pin, err := registry.TryPinGeneration()
	require.NoError(t, err)
	defer pin.Release()
	assert.False(t, pin.Enabled)
	assert.Equal(t, []string{"test"}, pin.DisabledFactory)
	require.Contains(t, pin.Factory, "test")
	assert.Len(t, pin.Factory["test"].SourceHash, 64)
	assert.Empty(t, pin.Effective)
}

func TestRegistryPinDoesNotInventUnprovableProgramIdentity(t *testing.T) {
	registry := NewRegistry()
	compiled := mustCompileRoutingPlugin(t, "legacy", 0, `["model"]`, "", "")
	// Historical/in-process callers can assemble LoadedPlugin without Compile.
	// Metadata alone must never be mistaken for proof of the actual program.
	legacy := &LoadedPlugin{Meta: compiled.Meta}
	require.NoError(t, registry.ReplaceOverrides([]*LoadedPlugin{legacy}))
	pin, err := registry.TryPinGeneration()
	require.NoError(t, err)
	defer pin.Release()
	require.Contains(t, pin.Effective, "legacy")
	assert.Empty(t, pin.Effective["legacy"].SourceHash)
	assert.Empty(t, pin.Overrides["legacy"].SourceHash)
}

func TestRegistryPinOwnsRouteMetadata(t *testing.T) {
	registry := NewRegistry()
	plugin, err := registry.Register(routingTestPluginSource("pin-copy", 0, `["model"]`,
		`routes: [{method: "POST", path: "/pin-copy/create", type: "submit", decode: "decode", render: "render", retainResult: false}],`,
		`export const native = {decode() { return {}; }, render() { return {}; }};`), Options{})
	require.NoError(t, err)
	pin, err := registry.TryPinGeneration()
	require.NoError(t, err)
	defer pin.Release()
	*pin.Effective["pin-copy"].Meta.Routes[0].RetainResult = true
	assert.False(t, *plugin.Meta.Routes[0].RetainResult, "mutating lease facts must not alter the published plugin")
}
