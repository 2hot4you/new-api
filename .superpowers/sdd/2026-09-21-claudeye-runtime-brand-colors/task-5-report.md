# Task 5 Report: Claudeye runtime branding for Docusaurus

## Outcome

Implemented Claudeye-only runtime Navbar, Footer, and Favicon branding for the Docusaurus site. Dynamic asset URLs are absolute and derived from `DOCS_API_BASE_URL`; the approved neutral wordmark fallback is aware of Docusaurus `baseUrl`. Molii and iXiaozu retain their configured static assets and existing navbar-title behavior.

## TDD evidence

The initial focused test run failed for the expected missing behavior:

```text
Cannot find module './claudeye-branding'
ENOENT: src/theme/BrandImage/index.tsx
Expected navbar.title to be undefined; received "claudeye"
```

After the minimal implementation, the required focused command passed:

```bash
cd docs-site
bun test src/claudeye-branding.test.ts src/config.test.ts scripts/default-theme-contract.test.ts
```

```text
33 pass
0 fail
120 expect() calls
```

Coverage includes both Claudeye origins, absolute light/dark/Favicon endpoints, base-path-aware fallback resolution, preservation of static non-Claudeye assets, omission of the duplicate Claudeye navbar title, exact-URL navbar error handling, post-probe Favicon upgrade, and the one-shot Footer image fallback.

## Asset verification

```text
SHA-256: f77944e3ea47969bf3774ca849648d478922d5e9414ac154f0818e92670b5d7d
Format: PNG image data, 1995 x 440, 8-bit/color RGBA, non-interlaced
```

## Production build and generated HTML

Ran one Claudeye production build with `DOCS_SITE_URL` and `DOCS_API_BASE_URL` set to `https://claudeye.com` and `DOCS_BASE_URL=/docs/`:

```text
Server: Compiled successfully
Client: Compiled successfully
Generated static files in "build".
```

The worktree had no installed dependency tree, so the successful build temporarily exposed the repository checkout's existing locked `docs-site/node_modules` through a symlink. The symlink was removed after the build; no dependency or lockfile changed.

Generated `build/quick-start/index.html` checks:

```text
staticFavicon: true
lightWordmark: true
darkWordmark: true
dynamicFavicon: true
neutralFallback: true
navbarTitleNode: false
moliiAsset: false
ixiaozuAsset: false
```

## Self-review

- The generated HTML keeps `img/brand/favicon.png` before JavaScript executes.
- The favicon link changes only after a successful `fetch` response; failure leaves the static link untouched.
- The capturing image error handler compares `image.src` to the exact dynamic light wordmark URL before changing it, so unrelated images are not modified.
- The Footer uses the dynamic dark wordmark through `BrandImage`; its `onError` is removed after switching to the fallback.
- The social image and all Molii/iXiaozu behavior remain unchanged.
- No dependency, API contract, or file outside Task 5 ownership changed.
- `git diff --check` passes.

## Concerns

- No browser-level visual QA was requested for this delegated task. The production HTML and runtime/fallback contracts were verified from the generated build output.
- Base commit: `314f536ef8ebf274d17728bd71d304ee4d636f7a`.
