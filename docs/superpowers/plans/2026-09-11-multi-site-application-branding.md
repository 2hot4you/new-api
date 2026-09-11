# Multi-Site Application Branding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Molii and iXiaozu application variants from one source revision without leaking Molii title, favicon, homepage brand text, or default typography into the iXiaozu first paint.

**Architecture:** A build-time `SiteBrand` object supplies safe public fallbacks to Rsbuild, static HTML, and the React application. Existing `/api/status` values continue to override system name and logo after startup. The existing `MoliiBrandSentence` component remains in place but accepts an arbitrary brand name and colors its Unicode grapheme clusters pink/blue in sequence.

**Tech Stack:** React 19, TypeScript, Rsbuild 2, Tailwind CSS, Node test runner, happy-dom

**Spec:** `docs/superpowers/specs/2026-09-11-three-environment-multi-site-branding-design.md`

## Global Constraints

- Start implementation from `origin/develop` in an isolated worktree; do not build on the divergent local `main` checkout.
- Keep `MoliiBrandSentence`; generalize it instead of deleting or replacing it.
- Molii defaults preserve the current colors, title, favicon, banner text, and typography.
- `ixiaozu` builds fail when required public brand inputs are absent; they never fall back to Molii values.
- Runtime database settings remain authoritative after `/api/status` loads.
- Do not add secrets to frontend build variables.

---

### Task 1: Typed build-time site brand configuration

**Files:**
- Create: `web/build/site-brand.ts`
- Modify: `web/rsbuild.config.ts`
- Modify: `web/src/env.d.ts`
- Test: `web/build/site-brand.test.ts`

**Interfaces:**
- Consumes: `VITE_SITE_PROFILE`, `VITE_SITE_TITLE`, `VITE_SITE_DESCRIPTION`, `VITE_SITE_LOGO`, `VITE_SITE_FAVICON`, `VITE_SITE_APPLE_TOUCH_ICON`, `VITE_SITE_BANNER_BRAND`, and `VITE_SITE_DEFAULT_FONT`.
- Produces: `SiteBrand`, `resolveSiteBrand(environment)`, the compile-time constant `__SITE_BRAND__`, and HTML template parameters.

- [ ] **Step 1: Write failing profile tests**

```ts
test('preserves the Molii profile defaults', () => {
  expect(resolveSiteBrand({ VITE_SITE_PROFILE: 'molii' })).toMatchObject({
    id: 'molii',
    title: 'Molii Gateway',
    bannerBrand: 'Molii',
    defaultFont: 'serif',
  })
})

test('requires every public field for ixiaozu', () => {
  expect(() => resolveSiteBrand({ VITE_SITE_PROFILE: 'ixiaozu' })).toThrow(
    'VITE_SITE_TITLE must be set for ixiaozu'
  )
})

test('accepts a complete non-Molii profile', () => {
  expect(resolveSiteBrand(IXIAOZU_FIXTURE).bannerBrand).toBe('iXiaozu')
})
```

- [ ] **Step 2: Run the test and confirm it fails because `resolveSiteBrand` does not exist**

Run: `cd web && bun test build/site-brand.test.ts`

- [ ] **Step 3: Implement the resolver**

```ts
export type SiteBrand = {
  id: 'molii' | 'ixiaozu'
  title: string
  description: string
  logo: string
  favicon: string
  appleTouchIcon: string
  bannerBrand: string
  defaultFont: 'sans' | 'serif'
}

export function resolveSiteBrand(
  environment: Record<string, string | undefined>
): SiteBrand
```

Trim every input, reject unknown profiles, reject empty iXiaozu fields, restrict the font to `sans` or `serif`, and retain the current Molii assets as Molii-only defaults.

- [ ] **Step 4: Inject the profile into HTML and application code**

In `rsbuild.config.ts`, call `resolveSiteBrand(process.env)`, provide `html.templateParameters`, and define the serialized value:

```ts
source: {
  define: {
    __SITE_BRAND__: JSON.stringify(siteBrand),
  },
},
html: {
  template: './index.html',
  templateParameters: { siteBrand },
},
```

Declare `__SITE_BRAND__: import('../build/site-brand').SiteBrand` in `env.d.ts`.

- [ ] **Step 5: Run focused verification**

Run: `cd web && bun test build/site-brand.test.ts && bun run typecheck`

- [ ] **Step 6: Commit**

```bash
git add web/build/site-brand.ts web/build/site-brand.test.ts web/rsbuild.config.ts web/src/env.d.ts
git commit -m "feat: add application brand profiles"
```

### Task 2: Brand-correct initial HTML and runtime metadata

**Files:**
- Modify: `web/index.html`
- Create: `web/src/config/site-brand.ts`
- Modify: `web/src/lib/constants.ts`
- Modify: `web/src/lib/dom-utils.ts`
- Modify: `web/src/main.tsx`
- Modify: `web/src/hooks/use-system-config.ts`
- Test: `web/src/lib/__tests__/site-brand.test.ts`
- Test: `web/src/lib/__tests__/dom-utils.test.ts`
- Test: `web/src/lib/__tests__/favicon-assets.test.ts`

**Interfaces:**
- Consumes: `__SITE_BRAND__` and `/api/status` fields `system_name` and `logo`.
- Produces: `SITE_BRAND`, site-specific first-paint HTML, and runtime title/favicon overrides.

- [ ] **Step 1: Write failing fallback and override tests**

Assert that the iXiaozu fixture contains no `Molii`, that a missing runtime system name resolves to `SITE_BRAND.title`, and that a missing runtime logo resolves to `SITE_BRAND.logo`/`SITE_BRAND.favicon` rather than Molii assets.

- [ ] **Step 2: Run the focused tests and confirm the existing Molii constants fail them**

Run: `cd web && bun test src/lib/__tests__/site-brand.test.ts src/lib/__tests__/dom-utils.test.ts src/lib/__tests__/favicon-assets.test.ts`

- [ ] **Step 3: Add the browser-safe brand export**

```ts
export const SITE_BRAND = Object.freeze(__SITE_BRAND__)
export const DEFAULT_SYSTEM_NAME = SITE_BRAND.title
export const DEFAULT_LOGO = SITE_BRAND.logo
export const DEFAULT_FAVICON = SITE_BRAND.favicon
```

- [ ] **Step 4: Template the initial document**

Use escaped Rsbuild template values for `<title>`, `meta[name=title]`, `meta[name=description]`, favicon, and Apple touch icon. Keep `/api/status` refresh logic, but make its empty/error fallback use `SITE_BRAND`.

- [ ] **Step 5: Separate logo and favicon fallback behavior**

Replace `MOLII_FAVICON_URL` with `DEFAULT_FAVICON`. A custom runtime logo may continue to become the favicon for backward compatibility; an empty runtime logo must use the active build profile, never a different profile.

- [ ] **Step 6: Build both profiles and inspect generated HTML**

Run Molii with `VITE_SITE_PROFILE=molii bun run build`, then run an iXiaozu fixture build with all eight public variables. Assert the first `dist/index.html` contains Molii metadata and the second contains only the fixture brand metadata.

- [ ] **Step 7: Commit**

```bash
git add web/index.html web/src/config/site-brand.ts web/src/lib/constants.ts web/src/lib/dom-utils.ts web/src/main.tsx web/src/hooks/use-system-config.ts web/src/lib/__tests__
git commit -m "feat: apply site branding before first paint"
```

### Task 3: Generalize the existing colored brand sentence

**Files:**
- Modify: `web/src/features/home/components/molii-brand-sentence.tsx`
- Modify: `web/src/features/home/components/sections/hero.tsx`
- Modify: `web/src/features/home/components/__tests__/home-sections.test.tsx`

**Interfaces:**
- Consumes: `sentence: string`, `brandName?: string`, and `SITE_BRAND.bannerBrand`.
- Produces: the unchanged `MoliiBrandSentence` export with grapheme-safe alternating color rendering.

- [ ] **Step 1: Replace literal-Molii tests with generic-brand tests**

```tsx
const container = render(
  <MoliiBrandSentence sentence='Create with iXiaozu.' brandName='iXiaozu' />
)
assert.deepEqual(
  [...container.querySelectorAll('[data-home-brand-letter]')].map((node) => [
    node.textContent,
    node.getAttribute('data-color'),
  ]),
  [
    ['i', 'pink'], ['X', 'blue'], ['i', 'pink'], ['a', 'blue'],
    ['o', 'pink'], ['z', 'blue'], ['u', 'pink'],
  ]
)
```

Add cases for `Molii`, Chinese, emoji/combined characters, whitespace sequence advancement, a missing brand substring, and the complete accessible label.

- [ ] **Step 2: Run the tests and confirm the current literal search fails generic cases**

Run: `cd web && bun test src/features/home/components/__tests__/home-sections.test.tsx`

- [ ] **Step 3: Implement grapheme segmentation without adding a dependency**

```ts
function splitGraphemes(value: string): string[] {
  if (typeof Intl.Segmenter === 'function') {
    return [...new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(value)]
      .map(({ segment }) => segment)
  }
  return Array.from(value)
}
```

Keep the current two gradients, alternate by grapheme index, preserve whitespace spans, and retain `aria-label={sentence}` with colored spans hidden from assistive technology.

- [ ] **Step 4: Use the active profile in the homepage hero**

Render `Create with ${SITE_BRAND.bannerBrand}.` through the existing translation path and pass `brandName={SITE_BRAND.bannerBrand}`. Do not change hero layout, size, animation, search, or CTA spacing.

- [ ] **Step 5: Run component tests and type checking**

Run: `cd web && bun test src/features/home/components/__tests__/home-sections.test.tsx && bun run typecheck`

- [ ] **Step 6: Commit**

```bash
git add web/src/features/home/components/molii-brand-sentence.tsx web/src/features/home/components/sections/hero.tsx web/src/features/home/components/__tests__/home-sections.test.tsx
git commit -m "feat: color arbitrary homepage brand names"
```

### Task 4: Apply the site default font

**Files:**
- Modify: `web/src/lib/theme-customization.ts`
- Modify: `web/src/context/theme-customization-provider.tsx`
- Test: `web/src/styles/__tests__/theme-contract.test.ts`
- Test: `web/src/lib/__tests__/site-brand.test.ts`

**Interfaces:**
- Consumes: `SITE_BRAND.defaultFont`.
- Produces: the site's default font for body and homepage headings while retaining domain-local appearance behavior.

- [ ] **Step 1: Write a failing test that resolves each build profile to its configured font**

Assert `molii → serif` and the complete iXiaozu fixture's selected `sans|serif` value. Assert monospace code blocks remain unaffected.

- [ ] **Step 2: Run the focused tests and confirm defaults are currently global constants**

Run: `cd web && bun test src/styles/__tests__/theme-contract.test.ts src/lib/__tests__/site-brand.test.ts`

- [ ] **Step 3: Source the default font from `SITE_BRAND`**

Set `DEFAULT_THEME_CUSTOMIZATION.font` to `SITE_BRAND.defaultFont`. Do not change the CSS font stacks or code-font rules.

- [ ] **Step 4: Verify both profile builds and tests**

Run: `cd web && bun test src/styles/__tests__/theme-contract.test.ts src/lib/__tests__/site-brand.test.ts && bun run build:check`

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/theme-customization.ts web/src/context/theme-customization-provider.tsx web/src/styles/__tests__/theme-contract.test.ts web/src/lib/__tests__/site-brand.test.ts
git commit -m "feat: use site-specific default typography"
```

### Task 5: Application branding regression verification

**Files:**
- Modify: `.ccg/tasks/design-new-api-cicd/review.md`

**Interfaces:**
- Consumes: Tasks 1-4.
- Produces: a verified branding unit ready for deployment integration.

- [ ] Run all focused brand, DOM, homepage, header, footer, and theme tests.
- [ ] Run `cd web && bun run typecheck && bun run lint && bun run build` once per brand profile.
- [ ] Inspect each generated `dist/index.html` for title, description, favicon, Apple icon, and absence of cross-brand strings.
- [ ] Open both local builds at desktop and mobile widths; verify logo, title, banner font, alternating characters, accessible text, and runtime `/api/status` override.
- [ ] Record exact commands and results in the CCG review file.

