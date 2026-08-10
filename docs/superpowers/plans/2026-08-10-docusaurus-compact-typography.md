# Docusaurus Compact Typography Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the complete Molii Docusaurus interface to 85% of its current typographic and `rem`-based visual scale without changing fonts, layout structure, or theme behavior.

**Architecture:** Add one root `font-size: 85%` declaration to the existing font stylesheet so Docusaurus and Infima continue to own component sizing while their `rem` values scale consistently. Protect the contract with the existing default-theme test, then rebuild, inspect desktop and mobile layouts, and update the static background service.

**Tech Stack:** Docusaurus 3.10.2, Infima, CSS, Bun test runner, Playwright-compatible in-app browser, macOS LaunchAgent.

## Global Constraints

- The root font size must be exactly `85%` on desktop and mobile.
- Keep the existing Serif font stack and Docusaurus default theme.
- Do not use CSS `zoom`, `transform`, or JavaScript scaling.
- Do not change colors, content width, breakpoints, border radius, or page structure.
- Preserve browser zoom and the existing monospace code font.
- The deployed documentation must remain available at `http://127.0.0.1:3100` through `io.molii.docs`.

---

### Task 1: Apply and deploy the compact root scale

**Files:**
- Modify: `docs-site/scripts/default-theme-contract.test.ts:12-24`
- Modify: `docs-site/src/css/fonts.css:3-12`

**Interfaces:**
- Consumes: Docusaurus' existing `customCss: './src/css/fonts.css'` registration and Infima's `rem`-based sizing.
- Produces: A document root with computed font size `85%` of the browser default; no new JavaScript or component API.

- [ ] **Step 1: Write the failing style contract test**

Add this test inside the existing `describe('Docusaurus default-theme contract', ...)` block:

```ts
test('scales the complete default theme to 85 percent without layout transforms', async () => {
  const fonts = await source('src/css/fonts.css');

  expect(fonts).toMatch(/:root\s*\{[^}]*font-size:\s*85%;/s);
  expect(fonts).not.toMatch(/\b(?:zoom|transform)\s*:/i);
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run from `docs-site/`:

```bash
bun test scripts/default-theme-contract.test.ts
```

Expected: FAIL in `scales the complete default theme to 85 percent without layout transforms` because `fonts.css` does not contain `font-size: 85%`.

- [ ] **Step 3: Add the minimal root scale**

Update the existing `:root` block in `docs-site/src/css/fonts.css`:

```css
:root {
  font-size: 85%;
  --ifm-font-family-base:
    'Lora Variable', 'Lora', 'Source Serif Pro', 'Source Serif 4',
    'Noto Serif SC', 'Noto Serif TC', 'Noto Serif JP', 'Noto Serif KR',
    'Source Han Serif SC', 'Source Han Serif TC', 'Source Han Serif',
    'Songti SC', 'STSong', 'STSongti-SC-Regular', 'PingFang SC', 'SimSun',
    'NSimSun', '宋体', 'FangSong', '仿宋', 'KaiTi', '楷体', Georgia,
    'Times New Roman', Cambria, 'Liberation Serif', serif;
  --ifm-heading-font-family: var(--ifm-font-family-base);
}
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run from `docs-site/`:

```bash
bun test scripts/default-theme-contract.test.ts
```

Expected: all tests in the file PASS with no warnings.

- [ ] **Step 5: Run the complete documentation quality gate**

Run from `docs-site/`:

```bash
bun run check
```

Expected: Bun tests, forbidden-term scan, secret scan, Docusaurus production build, local search index generation, and internal link check all PASS.

- [ ] **Step 6: Deploy the generated static site to the background service**

Run from the repository root:

```bash
rsync -a docs-site/build/ '/Users/naf/Library/Application Support/molii-docs/site/'
launchctl kickstart -k gui/$(id -u)/io.molii.docs
```

Expected: `io.molii.docs` restarts and serves the new build from `Application Support`.

- [ ] **Step 7: Verify desktop and mobile rendering**

Use the in-app browser against both `/` and `/api-reference/seedance` at desktop `1440×900` and mobile `390×844` viewports. For every page and viewport, verify:

```js
const rootSize = getComputedStyle(document.documentElement).fontSize;
const overflows = document.documentElement.scrollWidth > document.documentElement.clientWidth;
({ rootSize, overflows });
```

Expected: `rootSize` is `13.6px` when the browser default is `16px`, `overflows` is `false`, navigation remains usable, and tables/code blocks do not overlap or clip controls.

- [ ] **Step 8: Verify the live service**

Run:

```bash
curl --fail --silent --show-error --output /tmp/molii-docs-compact.html --write-out '%{http_code}\n' http://127.0.0.1:3100/
launchctl print gui/$(id -u)/io.molii.docs | rg '^\s*(state|pid|last exit code) ='
```

Expected: curl prints `200`; LaunchAgent state is `running` with a PID and no nonzero exit code.

- [ ] **Step 9: Commit the implementation**

```bash
git add docs-site/scripts/default-theme-contract.test.ts docs-site/src/css/fonts.css
git commit -m "style: compact Docusaurus typography"
```
