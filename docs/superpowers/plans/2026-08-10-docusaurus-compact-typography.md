# Docusaurus Compact Typography Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Molii Docusaurus site use a `12px` root font size on desktop and a `14px` root font size at Docusaurus' `996px` mobile breakpoint and below.

**Architecture:** Extend the existing `fonts.css` root declaration with one desktop font size and one breakpoint override, allowing Docusaurus and Infima's existing `rem` units to scale typography and spacing consistently. Protect the responsive behavior with a real Playwright browser test that checks computed styles and document overflow, then rebuild and redeploy the static site through the existing LaunchAgent.

**Tech Stack:** Docusaurus 3.10.2, Infima, CSS, Bun test runner, Playwright Core, macOS LaunchAgent.

## Global Constraints

- The computed root font size must be exactly `12px` above `996px`.
- The computed root font size must be exactly `14px` at `996px` and below.
- Keep the existing Serif font stack, Docusaurus default theme, and monospace code font.
- Do not use CSS `zoom`, `transform`, or JavaScript scaling.
- Do not change colors, content width, breakpoints, border radius, or page structure.
- Preserve browser zoom and existing responsive navigation behavior.
- The deployed documentation must remain available at `http://127.0.0.1:3100` through `io.molii.docs`.

---

### Task 1: Add responsive compact typography and deploy it

**Files:**
- Modify: `docs-site/scripts/api-reference.browser.test.ts`
- Modify: `docs-site/src/css/fonts.css`

**Interfaces:**
- Consumes: Docusaurus' existing `customCss: './src/css/fonts.css'` registration, Infima's `rem`-based sizing, and the browser-test server at `http://127.0.0.1:3197`.
- Produces: A document root computed at `12px` on desktop and `14px` at widths up to `996px`, without page-level horizontal overflow.

- [ ] **Step 1: Write the failing browser behavior test**

Add this test inside the existing `describe('default MDX API reference', ...)` block in `docs-site/scripts/api-reference.browser.test.ts`:

```ts
  test('uses compact responsive root typography without page overflow', async () => {
    const chromePath = await resolveBrowserExecutable();
    const browser = await chromium.launch({ executablePath: chromePath, headless: true });
    const desktop = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } });

    try {
      await Promise.all([
        desktop.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'networkidle' }),
        mobile.goto(`${baseUrl}/api-reference/images`, { waitUntil: 'networkidle' }),
      ]);

      const desktopLayout = await desktop.evaluate(() => ({
        rootFontSize: getComputedStyle(document.documentElement).fontSize,
        overflows: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      }));
      const mobileLayout = await mobile.evaluate(() => ({
        rootFontSize: getComputedStyle(document.documentElement).fontSize,
        overflows: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      }));

      expect(desktopLayout).toEqual({ rootFontSize: '12px', overflows: false });
      expect(mobileLayout).toEqual({ rootFontSize: '14px', overflows: false });
    } finally {
      await browser.close();
    }
  }, 30_000);
```

- [ ] **Step 2: Run the browser test and verify RED**

Run from `docs-site/`:

```bash
bun test scripts/api-reference.browser.test.ts
```

Expected: the new test fails because both viewports still compute the browser-default `16px` root font size; the pre-existing API reference tests continue to pass.

- [ ] **Step 3: Add the minimal responsive root sizes**

Update `docs-site/src/css/fonts.css` to retain the existing font variables and add the desktop declaration plus the Docusaurus mobile breakpoint:

```css
@import '@fontsource-variable/lora';

:root {
  font-size: 12px;
  --ifm-font-family-base:
    'Lora Variable', 'Lora', 'Source Serif Pro', 'Source Serif 4',
    'Noto Serif SC', 'Noto Serif TC', 'Noto Serif JP', 'Noto Serif KR',
    'Source Han Serif SC', 'Source Han Serif TC', 'Source Han Serif',
    'Songti SC', 'STSong', 'STSongti-SC-Regular', 'PingFang SC', 'SimSun',
    'NSimSun', '宋体', 'FangSong', '仿宋', 'KaiTi', '楷体', Georgia,
    'Times New Roman', Cambria, 'Liberation Serif', serif;
  --ifm-heading-font-family: var(--ifm-font-family-base);
}

@media (max-width: 996px) {
  :root {
    font-size: 14px;
  }
}
```

- [ ] **Step 4: Run the browser test and verify GREEN**

Run from `docs-site/`:

```bash
bun test scripts/api-reference.browser.test.ts
```

Expected: all four API reference browser tests pass, including exact `12px` and `14px` computed values and no page-level horizontal overflow.

- [ ] **Step 5: Run the complete documentation quality gate**

Run from `docs-site/`:

```bash
bun run check
```

Expected: Bun tests, forbidden-term scan, secret scan, Docusaurus production build, local search index generation, and internal link check all pass.

- [ ] **Step 6: Perform visual desktop and mobile verification**

Open `/` and `/api-reference/seedance` in the in-app browser at desktop `1440×900` and mobile `390×844`. Confirm the computed root sizes are `12px` and `14px` respectively, navigation and mobile menus remain usable, and headings, tables, code blocks, and controls do not overlap or clip.

- [ ] **Step 7: Deploy the generated static site to the background service**

Run from the repository root:

```bash
rsync -a docs-site/build/ '/Users/naf/Library/Application Support/molii-docs/site/'
launchctl kickstart -k gui/$(id -u)/io.molii.docs
```

Expected: `io.molii.docs` restarts and serves the new build from `Application Support`.

- [ ] **Step 8: Verify the live service and deployed responsive CSS**

Run:

```bash
curl --fail --silent --show-error --output /tmp/molii-docs-compact.html --write-out '%{http_code}\n' http://127.0.0.1:3100/
launchctl print gui/$(id -u)/io.molii.docs | rg '^\s*(state|pid|last exit code) ='
rg -n 'font-size:\s*(12|14)px|max-width:\s*996px' '/Users/naf/Library/Application Support/molii-docs/site/assets/css/'
```

Expected: curl prints `200`; LaunchAgent state is `running` with a PID and no nonzero exit code; the deployed CSS contains `12px`, `14px`, and the `996px` breakpoint.

- [ ] **Step 9: Commit the implementation**

```bash
git add docs-site/scripts/api-reference.browser.test.ts docs-site/src/css/fonts.css
git commit -m "style: compact Docusaurus typography"
```
