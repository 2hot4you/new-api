package brand_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func withOptionMap(t *testing.T, values map[string]string) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = values
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestNormalizeHexColor(t *testing.T) {
	value, err := NormalizeHexColor("  #a1b2c3 ")
	require.NoError(t, err)
	require.Equal(t, "#A1B2C3", value)

	for _, value := range []string{"", "A1B2C3", "#ABC", "#GG0000", "#11223344", "#112233;stroke:red"} {
		_, err := NormalizeHexColor(value)
		require.Error(t, err, value)
	}
}

func TestGetClaudeyePaletteFallsBackFromInvalidRuntimeValue(t *testing.T) {
	withOptionMap(t, map[string]string{
		ClaudeyeLightMarkColorKey: "<script>",
	})

	require.Equal(t, DefaultClaudeyePalette.LightMark, GetClaudeyePalette().LightMark)
}

func TestBrandColorDefaultsAreExported(t *testing.T) {
	exported := config.GlobalConfig.ExportAllConfigs()

	require.Equal(t, "#242424", exported["brand_setting.claudeye_light_mark_color"])
	require.Equal(t, "#6A6A6A", exported["brand_setting.claudeye_light_text_color"])
	require.Equal(t, "#FFFFFF", exported["brand_setting.claudeye_dark_mark_color"])
	require.Equal(t, "#B8B8B8", exported["brand_setting.claudeye_dark_text_color"])
}
