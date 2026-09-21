# Claudeye Runtime Brand Colors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let super administrators configure independent light/dark mark and wordmark colors for `claudeye.com` and `model.claudeye.com`, then use those colors immediately in the application and documentation Header, Footer, and Favicon without rebuilding either site.

**Architecture:** Register four validated `brand_setting.*` options, render the approved nine-block mark and embedded wordmark alpha mask through public read-only SVG endpoints, and consume those endpoints only in `claudeye` builds. The application uses React components with static neutral fallbacks; Docusaurus keeps static assets in its generated HTML and upgrades them to same-origin dynamic assets after successful probing, so a backend outage never removes the brand.

**Tech Stack:** Go 1.26, Gin, existing option/config manager, React 19, TypeScript, Zod 4, TanStack Query, Tailwind CSS, Docusaurus 3.10, Bun/Vitest, Go `embed`

**Spec:** `docs/superpowers/specs/2026-09-21-claudeye-runtime-brand-colors-design.md`

## Global Constraints

- Work only in the isolated `feat/claudeye-runtime-brand-colors` worktree based on `origin/develop`.
- Preserve the approved nine-block mark geometry and the approved `claudeye` wordmark mask; do not invent or redraw the logo.
- Store exactly four uppercase `#RRGGBB` values under `brand_setting.claudeye_light_mark_color`, `brand_setting.claudeye_light_text_color`, `brand_setting.claudeye_dark_mark_color`, and `brand_setting.claudeye_dark_text_color`.
- Default values are `#242424`, `#6A6A6A`, `#FFFFFF`, and `#B8B8B8` respectively.
- `claudeye.com` and `model.claudeye.com` remain isolated by their separate PostgreSQL databases; do not add a site identifier or synchronize options between environments.
- Preserve an explicitly configured custom `Logo` URL as the highest-priority Header/Footer/Favicon override.
- Do not change Molii or iXiaozu application or documentation branding.
- Dynamic SVG output accepts no user-provided paths, CSS, HTML, script, event attributes, or external URLs.
- Keep static neutral fallbacks for application and documentation rendering failures.
- Do not add a database schema migration or a new frontend dependency.

## Review Focus

- A manually corrupted database color must fall back to the documented default and must never be reflected verbatim into SVG.
- Concurrent option reads and updates must remain race-safe under `go test -race` and produce a complete old or new palette, not malformed output.
- A custom runtime `Logo` must continue to replace the dynamic claudeye wordmark and may continue to provide the custom Favicon.
- A failed dynamic asset request in Docusaurus must leave the static neutral Navbar logo and Favicon intact instead of showing a broken image.
- Invalid or partially typed HEX input must keep the last valid preview visible, show field validation, and block saving until all four fields are valid.

---

### Task 1: Register and validate the four brand color options

**Files:**
- Create: `setting/brand_setting/config.go`
- Create: `setting/brand_setting/config_test.go`
- Modify: `model/option.go:1-255`
- Create: `model/option_brand_setting_test.go`

**Interfaces:**
- Produces: `brand_setting.ConfigName`, four option-key constants, `DefaultClaudeyePalette`, `ClaudeyePalette`, `NormalizeHexColor(string) (string, error)`, `NormalizeOption(key, value string) (string, bool, error)`, and `GetClaudeyePalette() ClaudeyePalette`.
- Consumes: `config.GlobalConfig`, `common.OptionMap`, and its existing read/write mutex.

- [ ] **Step 1: Write failing configuration and validation tests**

```go
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
```

Also assert that `config.GlobalConfig.ExportAllConfigs()` exports all four exact option keys with the four documented defaults.

- [ ] **Step 2: Run the package test and verify it fails because `brand_setting` does not exist**

Run: `go test ./setting/brand_setting`

Expected: FAIL with a missing package/file error.

- [ ] **Step 3: Implement the registered configuration and immutable palette snapshot**

```go
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
```

Register a `Settings` value with `config.GlobalConfig.Register(ConfigName, &settings)` in `init`. Implement `NormalizeHexColor` with a compiled `^#[0-9A-Fa-f]{6}$` regexp, trim whitespace, and return uppercase. `GetClaudeyePalette` must acquire `common.OptionMapRWMutex.RLock`, copy all four strings, release the lock, normalize each copy, and replace only invalid entries with the corresponding default.

- [ ] **Step 4: Write failing model normalization tests**

```go
func TestBrandColorOptionIsNormalizedBeforePersistence(t *testing.T) {
	value, err := normalizeOptionValue(
		brand_setting.ClaudeyeLightMarkColorKey,
		" #abcdef ",
	)
	require.NoError(t, err)
	require.Equal(t, "#ABCDEF", value)
}

func TestBrandColorOptionRejectsInjection(t *testing.T) {
	_, err := normalizeOptionValue(
		brand_setting.ClaudeyeLightTextColorKey,
		`#000000\" onload=\"alert(1)`,
	)
	require.Error(t, err)
}
```

- [ ] **Step 5: Route brand options through model normalization**

At the start of `normalizeOptionValue`, call:

```go
if normalized, handled, err := brand_setting.NormalizeOption(key, value); handled {
	return normalized, err
}
```

This single gate must protect `UpdateOption`, `UpdateOptionsBulk`, and startup database loading. Do not duplicate color validation only in the HTTP controller.

- [ ] **Step 6: Run focused tests and the race detector**

Run: `go test -race ./setting/brand_setting ./model -run 'BrandColor|ClaudeyePalette|NormalizeHexColor'`

Expected: PASS with no race report.

- [ ] **Step 7: Commit the option layer**

```bash
git add setting/brand_setting model/option.go model/option_brand_setting_test.go
git commit -m "feat: add validated claudeye brand colors"
```

### Task 2: Render and serve safe dynamic SVG brand assets

**Files:**
- Create: `service/branding_assets/claudeye-wordmark-mask.png`
- Create: `service/claudeye_branding.go`
- Create: `service/claudeye_branding_test.go`
- Create: `controller/branding.go`
- Create: `controller/branding_test.go`
- Modify: `router/api-router.go:23-36`

**Interfaces:**
- Consumes: `brand_setting.ClaudeyePalette` and optional validated preview overrides.
- Produces: `service.RenderClaudeyeWordmark(palette, surface) ([]byte, error)`, `service.RenderClaudeyeFavicon(palette) []byte`, `controller.GetClaudeyeWordmark`, and `controller.GetClaudeyeFavicon`.
- Public routes: `GET /api/branding/claudeye/wordmark.svg` and `GET /api/branding/claudeye/favicon.svg`.

- [ ] **Step 1: Copy the approved alpha-mask source and verify identity**

```bash
cp /Users/naf/Documents/Codex/Projects/molii.co/new-api/output/brand/claudeye-review-v1/claudeye-word-text-source.png \
  service/branding_assets/claudeye-wordmark-mask.png
sha256sum service/branding_assets/claudeye-wordmark-mask.png
```

Expected SHA-256: `38ff1c8e3d6ba7f82ece7e9193b10018618fdfe5f7009d591738e948e2e75fee`; expected dimensions: `1445 x 440`, RGBA. Stop if either differs.

- [ ] **Step 2: Write failing renderer tests**

```go
func TestRenderClaudeyeWordmarkUsesSelectedSurface(t *testing.T) {
	palette := brand_setting.ClaudeyePalette{
		LightMark: "#111111", LightText: "#222222",
		DarkMark: "#EEEEEE", DarkText: "#DDDDDD",
	}
	light, err := RenderClaudeyeWordmark(palette, ClaudeyeSurfaceLight)
	require.NoError(t, err)
	require.Contains(t, string(light), `fill="#111111"`)
	require.Contains(t, string(light), `fill="#222222"`)
	require.NotContains(t, string(light), "#EEEEEE")
}

func TestRenderedSVGContainsNoActiveOrExternalContent(t *testing.T) {
	svg, err := RenderClaudeyeWordmark(brand_setting.DefaultClaudeyePalette, ClaudeyeSurfaceLight)
	require.NoError(t, err)
	for _, forbidden := range []string{"<script", "foreignObject", "onload=", "http://", "https://"} {
		require.NotContains(t, string(svg), forbidden)
	}
}
```

Add tests for dark colors, unsupported surfaces, a `1995 440` wordmark viewBox, the exact nine fixed mark elements, embedded `data:image/png;base64,` mask data, and Favicon `prefers-color-scheme: dark` output.

- [ ] **Step 3: Implement the fixed SVG renderer**

Embed the verified mask:

```go
//go:embed branding_assets/claudeye-wordmark-mask.png
var claudeyeWordmarkMask []byte

type ClaudeyeSurface string

const (
	ClaudeyeSurfaceLight ClaudeyeSurface = "light"
	ClaudeyeSurfaceDark  ClaudeyeSurface = "dark"
)
```

Build the wordmark with `viewBox="0 0 1995 440"`: render the exact fixed mark geometry at `0..440`, define an alpha mask from the embedded 1445×440 PNG, and fill a `1445×440` rectangle starting at `x=550` with the selected text color. The Favicon uses only the nine-block geometry and an internal media query that switches from `LightMark` to `DarkMark`; it has a transparent background.

- [ ] **Step 4: Write failing controller contract tests**

```go
func TestGetClaudeyeWordmarkPreviewOverridesDoNotPersist(t *testing.T) {
	recorder := performBrandRequest(
		"/api/branding/claudeye/wordmark.svg?surface=light&mark=%23112233&text=%23445566",
		GetClaudeyeWordmark,
	)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "#112233")
	require.Contains(t, recorder.Body.String(), "#445566")
	require.Equal(t, brand_setting.DefaultClaudeyePalette, brand_setting.GetClaudeyePalette())
}
```

Also test missing `surface` defaults to `light`, invalid surface and invalid overrides return HTTP 400, `Content-Type`, `nosniff`, `Cache-Control`, and ETag are present, and matching `If-None-Match` returns 304 with no body.

- [ ] **Step 5: Implement response handling and routes**

```go
apiRouter.GET("/branding/claudeye/wordmark.svg", controller.GetClaudeyeWordmark)
apiRouter.GET("/branding/claudeye/favicon.svg", controller.GetClaudeyeFavicon)
```

Controller behavior:

1. Snapshot the stored palette.
2. Parse `surface`, defaulting an empty value to `light`.
3. Validate optional `mark` and `text` with `NormalizeHexColor` and apply them only to the selected surface in memory.
4. Render bytes, calculate `sha256` over the response body, quote it as ETag, and honor exact `If-None-Match`.
5. Set `Content-Type: image/svg+xml; charset=utf-8`, `X-Content-Type-Options: nosniff`, and `Cache-Control: no-cache, must-revalidate`.

- [ ] **Step 6: Run backend tests**

Run: `go test -race ./service ./controller ./router -run 'Claudeye|Branding'`

Expected: PASS with no race report.

- [ ] **Step 7: Commit the renderer and endpoints**

```bash
git add service/branding_assets service/claudeye_branding.go service/claudeye_branding_test.go controller/branding.go controller/branding_test.go router/api-router.go
git commit -m "feat: serve dynamic claudeye brand assets"
```

### Task 3: Apply dynamic branding in the React application

**Files:**
- Create: `web/public/claudeye-wordmark-neutral.png`
- Create: `web/src/components/layout/components/claudeye-wordmark.tsx`
- Create: `web/src/components/layout/components/__tests__/claudeye-wordmark.test.tsx`
- Modify: `web/build/site-brand.ts:38-71`
- Modify: `web/build/site-brand.test.ts`
- Modify: `web/src/components/layout/components/header-brand.tsx:19-95`
- Modify: `web/src/components/layout/components/__tests__/header-brand.test.tsx`
- Modify: `web/src/features/home/components/sections/home-footer.tsx:19-376`
- Modify: `web/src/features/home/components/__tests__/home-footer.test.tsx`
- Modify: `web/src/hooks/use-system-config.ts:173-197`
- Modify: `web/src/lib/dom-utils.ts:24-56`
- Modify: `web/src/lib/__tests__/favicon-assets.test.ts`

**Interfaces:**
- Consumes: `SITE_BRAND.id`, `DEFAULT_LOGO`, the dynamic branding routes, and the existing runtime `Logo` value.
- Produces: `ClaudeyeWordmark({surface, alt, className})`, `HomeFooterContentProps.brandId: SiteBrandId`, dynamic default Header/Footer, and a dynamic default Favicon without changing explicit custom logos.

- [ ] **Step 1: Copy and verify the approved neutral wordmark fallback**

```bash
cp /Users/naf/Documents/Codex/Projects/molii.co/new-api/output/brand/claudeye-review-v1/final/claudeye-wordmark-transparent.png \
  web/public/claudeye-wordmark-neutral.png
sha256sum web/public/claudeye-wordmark-neutral.png
```

Expected SHA-256: `f77944e3ea47969bf3774ca849648d478922d5e9414ac154f0818e92670b5d7d`; expected dimensions: `1995 x 440`, RGBA.

- [ ] **Step 2: Write failing brand component and profile tests**

```tsx
test('uses light dynamic wordmark and falls back once', () => {
	const { getByRole } = render(
		<ClaudeyeWordmark surface='light' alt='claudeye' />
	)
	const image = getByRole('img') as HTMLImageElement
	expect(image.src).toContain('/api/branding/claudeye/wordmark.svg?surface=light')
	fireEvent.error(image)
	expect(image.src).toContain('/claudeye-wordmark-neutral.png')
})
```

Add a build-profile test asserting that a complete claudeye fixture resolves its default `favicon` to `/api/branding/claudeye/favicon.svg`, while Molii and iXiaozu retain their configured Favicon behavior.

- [ ] **Step 3: Implement `ClaudeyeWordmark` and claudeye default Favicon**

```ts
export const CLAUDEYE_WORDMARK_FALLBACK = '/claudeye-wordmark-neutral.png'

export function claudeyeWordmarkUrl(surface: 'light' | 'dark'): string {
	return `/api/branding/claudeye/wordmark.svg?surface=${surface}`
}
```

The component starts with the dynamic URL, switches to the neutral PNG once in `onError`, then clears the handler to prevent a retry loop. In `resolveSiteBrand`, retain configured Logo and Apple Touch Icon values, but use `/api/branding/claudeye/favicon.svg` as the claudeye profile's Favicon.

- [ ] **Step 4: Write failing Header and Footer behavior tests**

Cover these exact cases:

```tsx
const header = render(
	<HeaderBrand
		brandId='claudeye'
		systemLogo={DEFAULT_LOGO}
		siteName='claudeye'
		loading={false}
		logoLoaded
	/>
)
expect(header.getByRole('img').getAttribute('src')).toBe(
	'/api/branding/claudeye/wordmark.svg?surface=light'
)
expect(header.container.querySelector('[data-header-site-name]')).toBeNull()

const footer = render(
	<HomeFooterContent
		brandId='claudeye'
		displayLogo={DEFAULT_LOGO}
		displayName='claudeye'
		userAgreementEnabled={false}
		privacyPolicyEnabled={false}
		vendors={[]}
	/>
)
expect(footer.getByRole('img').getAttribute('src')).toBe(
	'/api/branding/claudeye/wordmark.svg?surface=dark'
)
```

Add a separate custom-logo assertion that renders `HeaderBrand` with `systemLogo='https://cdn.example/custom.png'`, finds that exact image, and verifies `[data-header-site-name]` contains `claudeye`.

Keep the existing Molii assertions unchanged.

- [ ] **Step 5: Use the dynamic wordmark only for the claudeye default**

In `HeaderBrand`, define `useClaudeyeWordmark` as `brandId === 'claudeye' && !customLogo && systemLogo === DEFAULT_LOGO`; render `surface='light'` and omit the separate site-name span. Add `brandId: SiteBrandId` to `HomeFooterContentProps`, pass `SITE_BRAND.id` from `HomeFooter`, and render `surface='dark'` only when `brandId === 'claudeye' && displayLogo === DEFAULT_LOGO`. Any different runtime Logo continues down the existing image-plus-name path.

- [ ] **Step 6: Protect dedicated Favicon behavior**

Update `useSystemConfig` so the default Logo preload calls `applyFaviconToDom(DEFAULT_FAVICON)`, while an explicit custom Logo still calls `applyFaviconToDom(customLogo)`. Preserve `resolveFaviconUrl` compatibility for existing profiles and add assertions for claudeye default and custom Logo values.

- [ ] **Step 7: Run application tests and type checking**

Run:

```bash
cd web
bun test build/site-brand.test.ts \
  src/components/layout/components/__tests__/claudeye-wordmark.test.tsx \
  src/components/layout/components/__tests__/header-brand.test.tsx \
  src/features/home/components/__tests__/home-footer.test.tsx \
  src/lib/__tests__/favicon-assets.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 8: Commit application branding**

```bash
git add web/build/site-brand.ts web/build/site-brand.test.ts web/public/claudeye-wordmark-neutral.png web/src/components/layout/components web/src/features/home/components web/src/hooks/use-system-config.ts web/src/lib/dom-utils.ts web/src/lib/__tests__/favicon-assets.test.ts
git commit -m "feat: apply dynamic claudeye branding"
```

### Task 4: Add the super-administrator brand appearance editor

**Files:**
- Create: `web/src/features/system-settings/site/claudeye-brand-colors.ts`
- Create: `web/src/features/system-settings/site/claudeye-brand-appearance-section.tsx`
- Create: `web/src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts`
- Create: `web/src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx`
- Modify: `web/src/features/system-settings/types.ts:111-124`
- Modify: `web/src/features/system-settings/site/index.tsx:27-40`
- Modify: `web/src/features/system-settings/site/section-registry.tsx:19-105`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`

**Interfaces:**
- Consumes: four `brand_setting.*` values returned by the root-only `/api/option/` endpoint and the existing `useUpdateOption` mutation.
- Produces: pure HEX/contrast/preview helpers and a claudeye-only `brand-appearance` settings section with four pickers.

- [ ] **Step 1: Write failing pure-helper tests**

```ts
expect(normalizeBrandHex(' #a1b2c3 ')).toBe('#A1B2C3')
expect(normalizeBrandHex('#abc')).toBeNull()
expect(
  buildClaudeyePreviewUrl('dark', '#ffffff', '#b8b8b8')
).toBe(
  '/api/branding/claudeye/wordmark.svg?surface=dark&mark=%23FFFFFF&text=%23B8B8B8'
)
expect(contrastRatio('#FFFFFF', '#171717')).toBeGreaterThan(4.5)
```

Also test invalid partial input returns the last valid preview colors rather than constructing an invalid URL.

- [ ] **Step 2: Implement the pure helpers and defaults**

```ts
export const CLAUDEYE_BRAND_DEFAULTS = {
  'brand_setting.claudeye_light_mark_color': '#242424',
  'brand_setting.claudeye_light_text_color': '#6A6A6A',
  'brand_setting.claudeye_dark_mark_color': '#FFFFFF',
  'brand_setting.claudeye_dark_text_color': '#B8B8B8',
} as const

export const BRAND_HEX_PATTERN = /^#[0-9A-Fa-f]{6}$/
```

Implement WCAG relative luminance and contrast calculations without a new dependency. Preview URLs must use `URLSearchParams` and uppercase valid colors.

- [ ] **Step 3: Write failing section interaction tests**

Cover:

- all four labels render with native `input[type=color]` and HEX text input;
- editing either control updates the other;
- light and dark preview image URLs update only when both colors for that surface are valid;
- `#123` shows a validation message and blocks submit;
- low contrast displays a warning but still allows submit;
- “Restore default colors” marks all four fields dirty and does not save until the main Save action;
- submitting changed values calls `updateSystemOption` with uppercase values;
- a non-claudeye build has no `brand-appearance` navigation item.

- [ ] **Step 4: Implement the form with existing settings primitives**

Use `useSettingsForm`, `SettingsSection`, `SettingsForm`, `SettingsPageFormActions`, `FormNavigationGuard`, Zod, and `useUpdateOption`. Define the schema as four `z.string().regex(/^#[0-9A-Fa-f]{6}$/)` fields. Each visual field must use this structure:

```tsx
<div className='flex items-center gap-3'>
  <Input type='color' aria-label={pickerLabel} value={validPickerValue} onChange={onPickerChange} />
  <Input value={field.value} onChange={onHexChange} className='font-mono uppercase' />
  <span aria-hidden className='size-8 rounded-md border' style={{ backgroundColor: validPickerValue }} />
</div>
```

Render the light preview on `#FFFFFF` and the dark preview on `#171717`. Preview `<img>` elements use the optional `mark`/`text` query overrides and the static neutral wordmark on error.

- [ ] **Step 5: Register the section only for claudeye**

Extend `SiteSettings` and `defaultSiteSettings` with the four exact option keys. Add the section descriptor only when `SITE_BRAND.id === 'claudeye'`:

```tsx
{
  id: 'brand-appearance',
  titleKey: 'Brand appearance',
  build: (settings: SiteSettings) => (
    <ClaudeyeBrandAppearanceSection defaultValues={pickBrandValues(settings)} />
  ),
}
```

The entire route remains behind the existing authenticated root-admin system-settings route, and the save operation continues to use the root-only option endpoint.

- [ ] **Step 6: Add complete locale keys and run the i18n check**

Add translations for: `Brand appearance`, `Light surface mark color`, `Light surface wordmark color`, `Dark surface mark color`, `Dark surface wordmark color`, `Restore default colors`, `Color must use #RRGGBB format`, `Low contrast`, `Light header preview`, and `Dark footer preview` in all seven locale files. Use the exact Chinese labels from the approved design in `zh.json`.

Run: `cd web && bun run i18n:check`

- [ ] **Step 7: Run admin UI tests, type checking, and lint**

```bash
cd web
bun test src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts \
  src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx
bun run typecheck
bun run lint
```

Expected: PASS.

- [ ] **Step 8: Commit the administrator editor**

```bash
git add web/src/features/system-settings/site web/src/features/system-settings/types.ts web/src/i18n/locales
git commit -m "feat: add claudeye brand color editor"
```

### Task 5: Apply runtime branding to Docusaurus with safe fallbacks

**Files:**
- Create: `docs-site/static/img/claudeye-wordmark-neutral.png`
- Create: `docs-site/src/claudeye-branding.ts`
- Create: `docs-site/src/claudeye-branding.test.ts`
- Create: `docs-site/src/theme/BrandImage/index.tsx`
- Modify: `docs-site/src/site-config.ts:5-101`
- Modify: `docs-site/src/config.test.ts`
- Modify: `docs-site/src/theme/Footer/index.tsx:131-204`
- Modify: `docs-site/src/theme/Footer/styles.module.css`
- Modify: `docs-site/scripts/default-theme-contract.test.ts`

**Interfaces:**
- Consumes: `PublicBrandConfig.id`, `brand.logoPath` as the static fallback, and same-origin `/api/branding/claudeye/*` endpoints.
- Produces: `resolveDocsBrandAssets(brand)`, `BrandImage`, a full dynamic Navbar wordmark without duplicate title, dynamic Footer wordmark, and a dynamically upgraded Favicon.

- [ ] **Step 1: Copy and verify the documentation fallback**

```bash
cp /Users/naf/Documents/Codex/Projects/molii.co/new-api/output/brand/claudeye-review-v1/final/claudeye-wordmark-transparent.png \
  docs-site/static/img/claudeye-wordmark-neutral.png
sha256sum docs-site/static/img/claudeye-wordmark-neutral.png
```

Expected SHA-256: `f77944e3ea47969bf3774ca849648d478922d5e9414ac154f0818e92670b5d7d`.

- [ ] **Step 2: Write failing documentation asset resolver tests**

```ts
expect(resolveDocsBrandAssets(CLAUDEYE_BRAND)).toEqual({
  navbarLogo: '/api/branding/claudeye/wordmark.svg?surface=light',
  footerLogo: '/api/branding/claudeye/wordmark.svg?surface=dark',
  favicon: '/api/branding/claudeye/favicon.svg',
  fallbackLogo: '/docs/img/claudeye-wordmark-neutral.png',
  dynamic: true,
})
expect(resolveDocsBrandAssets(MOLII_BRAND).dynamic).toBe(false)
```

Compute fallback paths with Docusaurus `baseUrl`; do not hard-code `/docs/` in the implementation.

- [ ] **Step 3: Implement the resolver and reusable image fallback**

```tsx
export function BrandImage(props: {
  alt: string
  dynamicSrc: string
  fallbackSrc: string
  className?: string
}) {
  const [src, setSrc] = useState(props.dynamicSrc)
  return (
    <img
      src={src}
      alt={props.alt}
      className={props.className}
      onError={() => setSrc(props.fallbackSrc)}
    />
  )
}
```

Guard against repeated fallback errors by clearing or ignoring `onError` after `src === fallbackSrc`.

- [ ] **Step 4: Write failing Docusaurus configuration and Footer tests**

Assert that claudeye config:

- has no `navbar.title` because the full wordmark includes the name;
- points Navbar logo to the light dynamic endpoint;
- keeps the configured static Favicon in generated HTML before JavaScript runs;
- includes a claudeye-only client script that probes the dynamic Favicon and restores the configured static paths on failure;
- leaves Molii and iXiaozu config unchanged;
- renders the Footer through `BrandImage` with `surface=dark`.

- [ ] **Step 5: Upgrade Navbar and Favicon only after successful probing**

Extend the existing Docusaurus plugin's `injectHtmlTags()` for claudeye with a small inline script. It must:

```js
const dynamicFavicon = '/api/branding/claudeye/favicon.svg'
fetch(dynamicFavicon, { cache: 'no-cache' })
  .then((response) => {
    if (!response.ok) throw new Error('brand asset unavailable')
    document.querySelectorAll('link[rel~="icon"]').forEach((link) => {
      link.href = dynamicFavicon
      link.type = 'image/svg+xml'
    })
  })
  .catch(() => {})
```

Use the dynamic light wordmark for Navbar and attach a capturing `error` handler that replaces only that exact URL with the base-path-aware neutral fallback. Do not inspect or modify unrelated images. Footer uses the React `BrandImage` wrapper with the dark endpoint.

- [ ] **Step 6: Run documentation tests and production build**

```bash
cd docs-site
bun test src/claudeye-branding.test.ts src/config.test.ts scripts/default-theme-contract.test.ts
bun run build
```

Inspect the generated claudeye HTML fixture and assert it contains the dynamic endpoints, the neutral fallback path, no duplicate Navbar title, and no Molii or iXiaozu brand asset path.

- [ ] **Step 7: Commit documentation branding**

```bash
git add docs-site/static/img/claudeye-wordmark-neutral.png docs-site/src/claudeye-branding.ts docs-site/src/claudeye-branding.test.ts docs-site/src/theme/BrandImage docs-site/src/site-config.ts docs-site/src/config.test.ts docs-site/src/theme/Footer docs-site/scripts/default-theme-contract.test.ts
git commit -m "feat: apply runtime branding to claudeye docs"
```

### Task 6: Integrate, visually verify, review, and close the CCG task

**Files:**
- Modify: `.ccg/tasks/claudeye-brand-assets-review/task.json`
- Create: `.ccg/tasks/claudeye-brand-assets-review/review.md`
- Move on completion: `.ccg/tasks/claudeye-brand-assets-review/` to `.ccg/tasks/archive/2026-09/claudeye-brand-assets-review/`

**Interfaces:**
- Consumes: Tasks 1-5 and the approved design specification.
- Produces: a tested feature branch with review evidence and an archived CCG task.

- [ ] **Step 1: Run the complete affected backend verification**

```bash
go test -race ./setting/brand_setting ./service ./controller ./router ./model
```

Expected: PASS with no race report.

- [ ] **Step 2: Run the complete application verification**

```bash
cd web
bun test
bun run i18n:check
bun run typecheck
bun run lint
bun run build
cd ..
```

Expected: PASS. Any known unrelated failure must be reproduced on `origin/develop` before it is recorded as unrelated.

- [ ] **Step 3: Run the complete documentation verification**

```bash
cd docs-site
bun test
bun run check:forbidden
bun run check:secrets
bun run build
cd ..
```

Expected: PASS.

- [ ] **Step 4: Perform browser-level visual verification**

Run the application and documentation against a local PostgreSQL-backed instance, then inspect:

- light Header and dark Footer at desktop and mobile widths;
- all four color pickers, HEX input synchronization, contrast warning, reset, save, and refresh persistence;
- application and documentation Favicon in light and dark browser themes;
- document Navbar and Footer logo dimensions, baseline, and fallback behavior with `/api/branding/claudeye/*` deliberately blocked;
- a custom `Logo` URL override;
- a Molii build and an iXiaozu build to confirm no regression.

Save screenshots or exact browser observations in `review.md`; do not claim visual verification from source inspection alone.

- [ ] **Step 5: Run the required code review**

Attempt the CCG antigravity and Claude reviewers in parallel over `git diff origin/develop...HEAD`. If `/Users/naf/.claude/bin/codeagent-wrapper` remains unavailable, record the exact command failure in `review.md` and perform a structured local review covering correctness, XSS/content-type safety, option isolation, cache behavior, accessibility, mobile layout, and non-claudeye regressions. Fix every Critical issue and rerun affected tests.

- [ ] **Step 6: Record evidence and archive the task**

Set `task.json` to `status: completed`, `currentPhase: completed`, and `nextAction: archive`. Write all commands, results, visual observations, review findings, and remaining limitations to `review.md`, then archive:

```bash
mkdir -p .ccg/tasks/archive/2026-09
mv .ccg/tasks/claudeye-brand-assets-review \
  .ccg/tasks/archive/2026-09/claudeye-brand-assets-review
git add .ccg/tasks
git commit -m "chore: archive ccg task claudeye-brand-assets-review"
```

- [ ] **Step 7: Final branch check**

```bash
git status --short --branch
git log --oneline --decorate origin/develop..HEAD
git diff --check origin/develop...HEAD
```

Expected: clean worktree, only planned commits ahead of `origin/develop`, and no whitespace errors.
