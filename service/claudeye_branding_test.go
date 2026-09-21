package service

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/brand_setting"
	"github.com/stretchr/testify/require"
)

var claudeyeMarkElements = []string{
	`<rect x="310" y="70" width="60" height="60"/>`,
	`<rect x="201.5" y="97" width="60" height="60" transform="rotate(45 231.5 127)"/>`,
	`<rect x="255.5" y="195.5" width="60" height="60" transform="rotate(45 285.5 225.5)"/>`,
	`<rect x="70" y="160" width="60" height="60"/>`,
	`<rect x="70" y="235" width="60" height="60"/>`,
	`<rect x="145" y="235" width="60" height="60"/>`,
	`<rect x="70" y="310" width="60" height="60"/>`,
	`<rect x="145" y="310" width="60" height="60"/>`,
	`<rect x="220" y="310" width="60" height="60"/>`,
}

func testClaudeyePalette() brand_setting.ClaudeyePalette {
	return brand_setting.ClaudeyePalette{
		LightMark: "#111111",
		LightText: "#222222",
		DarkMark:  "#EEEEEE",
		DarkText:  "#DDDDDD",
	}
}

func TestRenderClaudeyeWordmarkUsesSelectedSurface(t *testing.T) {
	palette := testClaudeyePalette()

	light, err := RenderClaudeyeWordmark(palette, ClaudeyeSurfaceLight)
	require.NoError(t, err)
	require.Contains(t, string(light), `fill="#111111"`)
	require.Contains(t, string(light), `fill="#222222"`)
	require.NotContains(t, string(light), "#EEEEEE")
	require.NotContains(t, string(light), "#DDDDDD")

	dark, err := RenderClaudeyeWordmark(palette, ClaudeyeSurfaceDark)
	require.NoError(t, err)
	require.Contains(t, string(dark), `fill="#EEEEEE"`)
	require.Contains(t, string(dark), `fill="#DDDDDD"`)
	require.NotContains(t, string(dark), "#111111")
	require.NotContains(t, string(dark), "#222222")
}

func TestRenderClaudeyeWordmarkRejectsUnsupportedSurfaceAndUnsafeColors(t *testing.T) {
	palette := testClaudeyePalette()

	_, err := RenderClaudeyeWordmark(palette, ClaudeyeSurface("sepia"))
	require.Error(t, err)

	palette.LightMark = `#112233" onload="alert(1)`
	_, err = RenderClaudeyeWordmark(palette, ClaudeyeSurfaceLight)
	require.Error(t, err)
}

func TestRenderClaudeyeWordmarkUsesApprovedGeometryAndEmbeddedMask(t *testing.T) {
	svg, err := RenderClaudeyeWordmark(brand_setting.DefaultClaudeyePalette, ClaudeyeSurfaceLight)
	require.NoError(t, err)
	body := string(svg)

	require.Contains(t, body, `viewBox="0 0 1995 440"`)
	for _, element := range claudeyeMarkElements {
		require.Equal(t, 1, strings.Count(body, element), element)
	}
	require.Equal(t, 10, strings.Count(body, "<rect "), "nine mark blocks plus one masked wordmark fill")
	require.Contains(t, body, `<mask id="claudeye-wordmark-mask"`)
	require.Contains(t, body, `href="data:image/png;base64,`)
	require.Contains(t, body, `<rect x="550" y="0" width="1445" height="440" fill="#6A6A6A" mask="url(#claudeye-wordmark-mask)"/>`)
}

func TestRenderedSVGContainsNoActiveOrExternalContent(t *testing.T) {
	wordmark, err := RenderClaudeyeWordmark(brand_setting.DefaultClaudeyePalette, ClaudeyeSurfaceLight)
	require.NoError(t, err)
	favicon := RenderClaudeyeFavicon(brand_setting.DefaultClaudeyePalette)

	for name, svg := range map[string][]byte{"wordmark": wordmark, "favicon": favicon} {
		t.Run(name, func(t *testing.T) {
			var root struct {
				XMLName xml.Name
			}
			require.NoError(t, xml.Unmarshal(svg, &root))
			require.Equal(t, "svg", root.XMLName.Local)
			require.Equal(t, "http://www.w3.org/2000/svg", root.XMLName.Space)

			lower := strings.ToLower(string(svg))
			for _, forbidden := range []string{"<script", "foreignobject", "onload=", "onclick=", "http://", "https://"} {
				require.NotContains(t, lower, forbidden)
			}
		})
	}
}

func TestRenderClaudeyeFaviconUsesThemeAwareApprovedGeometry(t *testing.T) {
	svg := string(RenderClaudeyeFavicon(testClaudeyePalette()))

	require.Contains(t, svg, `viewBox="0 0 440 440"`)
	require.Contains(t, svg, `fill="#111111"`)
	require.Contains(t, svg, `@media (prefers-color-scheme: dark)`)
	require.Contains(t, svg, `fill:#EEEEEE`)
	for _, element := range claudeyeMarkElements {
		require.Equal(t, 1, strings.Count(svg, element), element)
	}
	require.Equal(t, 9, strings.Count(svg, "<rect "))
	require.NotContains(t, svg, `<rect width="440" height="440"`)
}

func TestRenderClaudeyeFaviconFallsBackFromUnsafeColors(t *testing.T) {
	palette := testClaudeyePalette()
	palette.LightMark = `red;}</style><script>alert(1)</script>`
	palette.DarkMark = "not-a-color"

	svg := string(RenderClaudeyeFavicon(palette))
	require.Contains(t, svg, brand_setting.DefaultClaudeyePalette.LightMark)
	require.Contains(t, svg, brand_setting.DefaultClaudeyePalette.DarkMark)
	require.NotContains(t, strings.ToLower(svg), "<script")
}
