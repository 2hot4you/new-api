package service

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/setting/brand_setting"
)

//go:embed branding_assets/claudeye-wordmark-mask.png
var claudeyeWordmarkMask []byte

var claudeyeWordmarkMaskDataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(claudeyeWordmarkMask)

type ClaudeyeSurface string

const (
	ClaudeyeSurfaceLight ClaudeyeSurface = "light"
	ClaudeyeSurfaceDark  ClaudeyeSurface = "dark"
)

const claudeyeMarkSVG = `<rect x="310" y="70" width="60" height="60"/>
<rect x="201.5" y="97" width="60" height="60" transform="rotate(45 231.5 127)"/>
<rect x="255.5" y="195.5" width="60" height="60" transform="rotate(45 285.5 225.5)"/>
<rect x="70" y="160" width="60" height="60"/>
<rect x="70" y="235" width="60" height="60"/>
<rect x="145" y="235" width="60" height="60"/>
<rect x="70" y="310" width="60" height="60"/>
<rect x="145" y="310" width="60" height="60"/>
<rect x="220" y="310" width="60" height="60"/>`

func RenderClaudeyeWordmark(palette brand_setting.ClaudeyePalette, surface ClaudeyeSurface) ([]byte, error) {
	var markColor, textColor string
	switch surface {
	case ClaudeyeSurfaceLight:
		markColor, textColor = palette.LightMark, palette.LightText
	case ClaudeyeSurfaceDark:
		markColor, textColor = palette.DarkMark, palette.DarkText
	default:
		return nil, fmt.Errorf("unsupported claudeye surface %q", surface)
	}

	markColor, err := brand_setting.NormalizeHexColor(markColor)
	if err != nil {
		return nil, fmt.Errorf("invalid claudeye mark color: %w", err)
	}
	textColor, err = brand_setting.NormalizeHexColor(textColor)
	if err != nil {
		return nil, fmt.Errorf("invalid claudeye text color: %w", err)
	}

	var svg strings.Builder
	svg.Grow(len(claudeyeWordmarkMaskDataURI) + 1200)
	fmt.Fprintf(&svg, `<svg xmlns="http&#58;//www.w3.org/2000/svg" viewBox="0 0 1995 440" role="img" aria-labelledby="claudeye-wordmark-title">
<title id="claudeye-wordmark-title">claudeye</title>
<defs><mask id="claudeye-wordmark-mask" maskUnits="userSpaceOnUse" x="550" y="0" width="1445" height="440" mask-type="alpha"><image x="550" y="0" width="1445" height="440" href="%s"/></mask></defs>
<g fill="%s">%s</g>
<rect x="550" y="0" width="1445" height="440" fill="%s" mask="url(#claudeye-wordmark-mask)"/>
</svg>`, claudeyeWordmarkMaskDataURI, markColor, claudeyeMarkSVG, textColor)
	return []byte(svg.String()), nil
}

func RenderClaudeyeFavicon(palette brand_setting.ClaudeyePalette) []byte {
	lightMark := normalizeClaudeyeColorOrDefault(palette.LightMark, brand_setting.DefaultClaudeyePalette.LightMark)
	darkMark := normalizeClaudeyeColorOrDefault(palette.DarkMark, brand_setting.DefaultClaudeyePalette.DarkMark)

	return []byte(fmt.Sprintf(`<svg xmlns="http&#58;//www.w3.org/2000/svg" viewBox="0 0 440 440" role="img" aria-labelledby="claudeye-mark-title">
<title id="claudeye-mark-title">claudeye</title>
<style>.mark{fill:%s}@media (prefers-color-scheme: dark){.mark{fill:%s}}</style>
<g class="mark" fill="%s">%s</g>
</svg>`, lightMark, darkMark, lightMark, claudeyeMarkSVG))
}

func normalizeClaudeyeColorOrDefault(value, fallback string) string {
	normalized, err := brand_setting.NormalizeHexColor(value)
	if err != nil {
		return fallback
	}
	return normalized
}
