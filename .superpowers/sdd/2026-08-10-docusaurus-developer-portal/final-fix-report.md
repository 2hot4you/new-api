# Docusaurus developer portal — final review fix report

Date: 2026-08-11

Worktree: `/Users/naf/Documents/molii.io/new-api/.worktrees/docusaurus-developer-portal`

Final-fix base: `73ad04778a8b76aa8a00f27d16c6bd10d5c8fb15`

Commit: the single commit containing this report, with subject `docs: address developer portal final review` (the resulting hash is recorded in the final handoff because a commit cannot contain its own hash).

## Result

All four approved final-review findings are resolved in one fix wave. The exact six navigation labels, stock Docusaurus Docs renderer, public routes, protected New API（QuantumNous）attribution, redirect-security rule, and existing public scope remain unchanged. The focused regressions are GREEN, the full suite reports 90/90 passing, the scans and production build pass, and a controlled crawl reports 49/49 internal links returning 200.

## Finding resolutions

### 1. Homepage overflow and focus visibility

Root cause: the original portal CSS copied the plan's blanket `.portal { overflow: hidden; }`, which hid page-level overflow symptoms and could clip focus paint. Grid tracks used `minmax`, but their direct children retained the automatic minimum size. The code block happened to inherit `overflow: auto` from the theme, leaving the portal contract implicit.

Resolution:

- Removed blanket overflow from `docs-site/src/pages/index.module.css`.
- Added `min-width: 0` to direct children of the hero, card, model, step, and resource grids.
- Added explicit `overflow-x: auto` to `.codeCard pre`.
- Added a real Chromium regression in `docs-site/scripts/api-reference.browser.test.ts` at a 320 CSS-pixel viewport, equivalent to a 640-pixel window at 200% zoom. It checks rendered portal overflow, computed child minimum widths, every grid child's bounds, independent code scrolling, visible keyboard focus, and focus bounds without relying on root `scrollWidth`.

### 2. API-reference orientation

Root cause: commit `d3afaca5` replaced the former “基础约定” opening with a shorter orientation section and dropped the concrete Base URL, Authorization syntax, and API-key warning.

Resolution:

- `docs-site/docs/api-reference/index.mdx` now links directly to `/api-basics/base-url` and `/api-basics/authentication`, includes `Authorization: Bearer $MOLII_API_KEY`, and warns that the key belongs only in a server-side or local trusted environment.
- `docs-site/scripts/mdx-api-reference-contract.test.ts` protects both orientation links.

### 3. Trailing-slash-safe Grok POST URLs

Root cause: commit `ac710991` correctly removed redirect following from paid Grok POST examples, but direct `"$MOLII_API_BASE_URL/v1/..."` concatenation allowed a configured trailing slash to produce `//v1/...` and a now-intentionally-unfollowed redirect.

Resolution:

- Normalized all four POST URLs in `docs-site/docs/examples/grok-image-curl.mdx` and all three in `docs-site/docs/examples/grok-video-curl.mdx` as `"${MOLII_API_BASE_URL%/}/v1/..."`.
- Kept every paid POST free of `--location`, `--location-trusted`, and `-L`.
- `docs-site/scripts/grok-content-contract.test.ts` checks every paid POST fence for the normalized form and rejects the unsafe concatenation form.

### 4. Mixed-capability Grok navigation

Root cause: the homepage Grok card advertised both image and video capabilities while its only CTA went directly to the video model page.

Resolution:

- `docs-site/src/pages/index.tsx` routes “查看 Grok 模型” to the model-selection page `/models`.
- `docs-site/scripts/default-theme-contract.test.ts` protects the mixed-capability navigation contract.

## TDD evidence

### RED

The initial combined focused command was:

```text
$ bun test scripts/api-reference.browser.test.ts scripts/default-theme-contract.test.ts scripts/grok-content-contract.test.ts scripts/mdx-api-reference-contract.test.ts
17 pass
4 fail
279 expect() calls
Ran 21 tests across 4 files. [30.20s]
```

Three failures were the intended source contracts:

```text
Docusaurus default-theme contract > routes the mixed-capability Grok homepage card to model selection
Expected to contain: "<Link to=\"/models\">查看 Grok 模型 →</Link>"

ordinary MDX API reference contract > API overview links developers to the concrete Base URL and authentication contracts
Expected to contain: "[Base URL](/api-basics/base-url)"

Grok Imagine public documentation contract > paid Grok POST examples normalize a trailing slash before the versioned path
Expected to contain: "\"${MOLII_API_BASE_URL%/}/v1/"
Received: "curl --include --request POST ... \"$MOLII_API_BASE_URL/v1/images/generations\" ..."
```

The fourth failure in that combined run was an unrelated dev-server readiness timeout, so it was not accepted as browser RED evidence. Rerunning the browser file alone reached the intended assertion:

```text
$ bun test scripts/api-reference.browser.test.ts
Expected: "visible"
Received: "hidden"
(fail) default MDX API reference > keeps zoomed narrow homepage content and keyboard focus inside visible bounds
9 pass
1 fail
64 expect() calls
Ran 10 tests across 1 file. [19.66s]
```

### GREEN

The same combined focused suite after the minimal implementation produced:

```text
$ bun test scripts/api-reference.browser.test.ts scripts/default-theme-contract.test.ts scripts/grok-content-contract.test.ts scripts/mdx-api-reference-contract.test.ts
30 pass
0 fail
365 expect() calls
Ran 30 tests across 4 files. [19.91s]
```

## Full verification

All commands ran from `docs-site/` unless noted.

```text
$ bun test
90 pass
0 fail
1112 expect() calls
Ran 90 tests across 11 files. [20.72s]

$ bun run check:forbidden
Forbidden content check passed for docs.

$ bun run check:secrets
$ secretlint "docs/**/*.{md,mdx}"

$ bun run build
[webpackbar] ✔ Server: Compiled successfully in 630.05ms
[webpackbar] ✔ Client: Compiled successfully in 2.62s
[SUCCESS] Generated static files in "build".

$ git diff --check
(no output; exit 0)
```

The first link-check invocation correctly refused to reuse the already-running LaunchAgent listener:

```text
$ bun run check:links
Port 3100 responded, but the documentation preview server exited.
[ERROR] Something is already running on port 3100.
error: script "check:links" exited with code 1
```

Following the repository's established LaunchAgent-safe verification sequence, `io.molii.docs` was temporarily booted out under a restore trap, the checker served the fresh `build/`, and the service was restored:

```text
$ bun run check:links
✓ Successfully scanned 49 links in 0.091 seconds.

$ curl --retry 20 --retry-delay 1 --retry-connrefused --fail --silent --show-error --output /dev/null --write-out '%{http_code}\n' http://127.0.0.1:3100/
200

$ launchctl print gui/501/io.molii.docs | rg '^\s*(state|pid|last exit code) ='
state = running
pid = 27928
```

## Visual and metric evidence

Fresh screenshots were captured from the verified production `build/` on a controlled preview and opened for visual review:

- Desktop 1440×900: `/Users/naf/Documents/molii.io/new-api/.worktrees/docusaurus-developer-portal/.superpowers/sdd/2026-08-10-docusaurus-developer-portal/final-fix-screenshots/desktop-home.png`
- Mobile 390×844: `/Users/naf/Documents/molii.io/new-api/.worktrees/docusaurus-developer-portal/.superpowers/sdd/2026-08-10-docusaurus-developer-portal/final-fix-screenshots/mobile-home.png`
- 200%-zoom equivalent 320×720 CSS pixels: `/Users/naf/Documents/molii.io/new-api/.worktrees/docusaurus-developer-portal/.superpowers/sdd/2026-08-10-docusaurus-developer-portal/final-fix-screenshots/zoomed-200pct-equivalent-home.png`
- Complete metrics: `/Users/naf/Documents/molii.io/new-api/.worktrees/docusaurus-developer-portal/.superpowers/sdd/2026-08-10-docusaurus-developer-portal/final-fix-screenshots/metrics.json`

| Layout | Root font | Portal height | Page overflow | Portal overflow-x | Grid bounds/min-width | Code overflow | Focus | Grok href |
| --- | --- | ---: | --- | --- | --- | --- | --- | --- |
| 1440×900 | 12px | 2479px | false | visible | inside / 0px | auto | visible, inside | `/models` |
| 390×844 | 14px | 4478px | false | visible | inside / 0px | auto, independently scrollable | visible, inside | `/models` |
| 320×720 zoom equivalent | 14px | 4983px | false | visible | inside / 0px | auto, independently scrollable | visible, inside | `/models` |

All three rendered the exact navigation sequence `开始使用, 平台与账户, 开发指南, 模型与能力, API 参考, 帮助与更新`. Visual inspection found no clipped cards, headings, controls, focusable content, or page-level horizontal overflow. The desktop code sample fits without needing horizontal scrolling; mobile and zoom-equivalent code samples scroll locally.

## Self-review

- Diff scope is limited to four production fixes, four focused test files, and the requested evidence artifacts.
- The homepage still uses isolated CSS; ordinary guide and API pages still use the stock Docs renderer.
- No public route was removed. The changed Grok CTA points at the existing `/models` route, which the 49-link crawl verified.
- All seven Grok paid POST URLs use `${MOLII_API_BASE_URL%/}` and none follows redirects.
- The exact six desktop navigation labels and protected New API（QuantumNous）attribution are unchanged.
- `git diff --check` is clean.

## Concerns

- Four unrelated pre-existing untracked duplicate files (`docs-site/docs/examples 2.md`, `help 2.md`, `platform 2.md`, and `quick-start 2.md`) were preserved and excluded from this commit.
- The live LaunchAgent was restored and remains available, but its deployed site directory was not overwritten because deployment was not requested for this final-fix wave. Screenshots, metrics, and the link crawl use the fresh verified production build.
