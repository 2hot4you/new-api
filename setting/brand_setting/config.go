package brand_setting

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const ConfigName = "brand_setting"

const (
	ClaudeyeLightMarkColorKey = ConfigName + ".claudeye_light_mark_color"
	ClaudeyeLightTextColorKey = ConfigName + ".claudeye_light_text_color"
	ClaudeyeDarkMarkColorKey  = ConfigName + ".claudeye_dark_mark_color"
	ClaudeyeDarkTextColorKey  = ConfigName + ".claudeye_dark_text_color"
)

type Settings struct {
	ClaudeyeLightMarkColor string `json:"claudeye_light_mark_color"`
	ClaudeyeLightTextColor string `json:"claudeye_light_text_color"`
	ClaudeyeDarkMarkColor  string `json:"claudeye_dark_mark_color"`
	ClaudeyeDarkTextColor  string `json:"claudeye_dark_text_color"`
}

type ClaudeyePalette struct {
	LightMark string
	LightText string
	DarkMark  string
	DarkText  string
}

var DefaultClaudeyePalette = ClaudeyePalette{
	LightMark: "#242424",
	LightText: "#6A6A6A",
	DarkMark:  "#FFFFFF",
	DarkText:  "#B8B8B8",
}

var (
	hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	settings        = Settings{
		ClaudeyeLightMarkColor: DefaultClaudeyePalette.LightMark,
		ClaudeyeLightTextColor: DefaultClaudeyePalette.LightText,
		ClaudeyeDarkMarkColor:  DefaultClaudeyePalette.DarkMark,
		ClaudeyeDarkTextColor:  DefaultClaudeyePalette.DarkText,
	}
)

func init() {
	config.GlobalConfig.Register(ConfigName, &settings)
}

func NormalizeHexColor(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !hexColorPattern.MatchString(value) {
		return "", fmt.Errorf("invalid hex color %q: expected #RRGGBB", value)
	}
	return strings.ToUpper(value), nil
}

func NormalizeOption(key, value string) (string, bool, error) {
	switch key {
	case ClaudeyeLightMarkColorKey,
		ClaudeyeLightTextColorKey,
		ClaudeyeDarkMarkColorKey,
		ClaudeyeDarkTextColorKey:
		normalized, err := NormalizeHexColor(value)
		return normalized, true, err
	default:
		return value, false, nil
	}
}

func GetClaudeyePalette() ClaudeyePalette {
	common.OptionMapRWMutex.RLock()
	palette := ClaudeyePalette{
		LightMark: common.OptionMap[ClaudeyeLightMarkColorKey],
		LightText: common.OptionMap[ClaudeyeLightTextColorKey],
		DarkMark:  common.OptionMap[ClaudeyeDarkMarkColorKey],
		DarkText:  common.OptionMap[ClaudeyeDarkTextColorKey],
	}
	common.OptionMapRWMutex.RUnlock()

	palette.LightMark = normalizeOrDefault(palette.LightMark, DefaultClaudeyePalette.LightMark)
	palette.LightText = normalizeOrDefault(palette.LightText, DefaultClaudeyePalette.LightText)
	palette.DarkMark = normalizeOrDefault(palette.DarkMark, DefaultClaudeyePalette.DarkMark)
	palette.DarkText = normalizeOrDefault(palette.DarkText, DefaultClaudeyePalette.DarkText)
	return palette
}

func normalizeOrDefault(value, fallback string) string {
	normalized, err := NormalizeHexColor(value)
	if err != nil {
		return fallback
	}
	return normalized
}
