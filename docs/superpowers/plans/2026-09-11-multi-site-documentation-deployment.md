# Multi-Site Documentation Branding and Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish the same Docusaurus content revision at `/docs/` on development, Molii production, and iXiaozu production with independent branding, canonical URLs, sitemaps, API base URLs, fonts, deployment health, and notifications.

**Architecture:** Docusaurus receives a validated public brand profile at build time. The workflow builds one development artifact or two production artifacts, each as a fixed static snapshot, and deploys them with a `fail-fast: false` target matrix. Brand assets are copied into the artifact during preparation so the published documentation does not depend on another brand's host.

**Tech Stack:** Docusaurus 3.10, React 19, TypeScript, Bun, GitHub Actions, Bash, SSH

**Spec:** `docs/superpowers/specs/2026-09-11-three-environment-multi-site-branding-design.md`

## Global Constraints

- Start from `origin/develop` in an isolated worktree.
- All sites use the same Markdown/MDX and OpenAPI content revision.
- Development canonical base is `https://dev.molii.co/docs/` and is not indexed.
- Production canonical bases are `https://molii.co/docs/` and `https://aigc.ixiaozu.cn/docs/` and are indexable.
- iXiaozu output must contain no Molii title, logo, favicon, canonical host, sitemap host, API base URL, or footer brand fallback.
- New API and QuantumNous attribution remains visible on both sites.

---

### Task 1: Extend validated public documentation configuration

**Files:**
- Modify: `docs-site/src/config.ts`
- Modify: `docs-site/src/config.test.ts`

**Interfaces:**
- Consumes: `DOCS_ENV`, `DOCS_SITE_URL`, `DOCS_BASE_URL`, `DOCS_API_BASE_URL`, `DOCS_BRAND_ID`, `DOCS_SITE_TITLE`, `DOCS_TAGLINE`, `DOCS_NAVBAR_TITLE`, `DOCS_LOGO_PATH`, `DOCS_FAVICON_PATH`, `DOCS_SOCIAL_IMAGE_PATH`, and `DOCS_DEFAULT_FONT`.
- Produces: `PublicConfig` including a typed `brand` object.

- [ ] **Step 1: Write failing validation tests**

```ts
expect(resolvePublicConfig(COMPLETE_IXIAOZU_ENV).brand).toEqual({
  id: 'ixiaozu',
  siteTitle: 'iXiaozu fixture docs',
  tagline: 'Fixture tagline',
  navbarTitle: 'iXiaozu',
  logoPath: 'img/brand/logo.svg',
  faviconPath: 'img/brand/favicon.svg',
  socialImagePath: 'img/brand/social.svg',
  defaultFont: 'sans',
})
```

Also assert unknown brands, empty fields, absolute asset paths, `..` traversal, unsupported fonts, URL paths in `DOCS_SITE_URL`, and secret-like `DOCS_*` variables are rejected.

- [ ] **Step 2: Run tests and confirm current `PublicConfig` has no brand object**

Run: `cd docs-site && bun test src/config.test.ts`

- [ ] **Step 3: Implement strict public brand validation**

Allow only `molii|ixiaozu`, `sans|serif`, and relative files under `img/brand/`. Retain current URL/origin/base-path and public-secret checks.

- [ ] **Step 4: Run focused tests**

Run: `cd docs-site && bun test src/config.test.ts`

- [ ] **Step 5: Commit**

```bash
git add docs-site/src/config.ts docs-site/src/config.test.ts
git commit -m "feat: validate documentation brand profiles"
```

### Task 2: Drive Docusaurus metadata, navigation, footer, and fonts from the profile

**Files:**
- Modify: `docs-site/docusaurus.config.ts`
- Modify: `docs-site/src/css/fonts.css`
- Modify: `docs-site/src/pages/index.tsx`
- Modify: `docs-site/src/pages/index.module.css`
- Modify: `docs-site/src/config.test.ts`
- Modify: `docs-site/scripts/default-theme-contract.test.ts`

**Interfaces:**
- Consumes: `publicConfig.brand`, `publicConfig.siteUrl`, `baseUrl`, and `apiBaseUrl`.
- Produces: brand-correct Docusaurus title, tagline, favicon, canonical/sitemap host, social image, navbar, homepage, footer, font axis, and API examples.

- [ ] **Step 1: Add failing config tests for both profiles**

Assert `themeConfig.navbar`, `themeConfig.image`, favicon, copyright, `customFields.apiBaseUrl`, and the selected font are generated from configuration rather than Molii literals.

- [ ] **Step 2: Run tests and confirm the hard-coded Molii values fail iXiaozu**

Run: `cd docs-site && bun test src/config.test.ts scripts/default-theme-contract.test.ts`

- [ ] **Step 3: Replace hard-coded presentation values**

Use `publicConfig.brand` for title, tagline, favicon, social image, navbar title/logo/alt text, homepage heading, and copyright. Keep the attribution suffix exactly visible:

```ts
`Copyright © ${year} ${brand.navbarTitle}. 保留所有权利。基于 New API（QuantumNous）构建。`
```

- [ ] **Step 4: Add font-profile CSS**

Set `data-docs-font` from Docusaurus configuration and map `serif` to the current Lora/CJK stack and `sans` to the existing Public Sans/system CJK stack. Keep code blocks monospace and retain current responsive root sizes.

- [ ] **Step 5: Run tests and two fixture builds**

Build once with Molii variables and once with complete iXiaozu fixture variables. Search the iXiaozu `build/` tree for forbidden Molii presentation strings while allowing the repository's required license/attribution text only where legally necessary.

- [ ] **Step 6: Commit**

```bash
git add docs-site/docusaurus.config.ts docs-site/src docs-site/scripts/default-theme-contract.test.ts
git commit -m "feat: apply documentation brand profiles"
```

### Task 3: Prepare self-contained brand assets

**Files:**
- Create: `docs-site/scripts/prepare-brand-assets.ts`
- Create: `docs-site/scripts/prepare-brand-assets.test.ts`
- Modify: `docs-site/package.json`
- Modify: `docs-site/.gitignore`

**Interfaces:**
- Consumes: controlled HTTPS values `DOCS_LOGO_SOURCE_URL`, `DOCS_FAVICON_SOURCE_URL`, and `DOCS_SOCIAL_IMAGE_SOURCE_URL` supplied by the selected GitHub Environment.
- Produces: `static/img/brand/logo`, `static/img/brand/favicon`, and `static/img/brand/social` files referenced by the validated configuration.

- [ ] **Step 1: Write failing tests using a local HTTP fixture server**

Assert the script rejects non-HTTPS production URLs, redirects to non-HTTPS, responses over 2 MiB, unsupported MIME types, empty bodies, and SVG files containing scripts or external references. Assert valid PNG and sanitized SVG fixtures are written atomically.

- [ ] **Step 2: Run the test and confirm the script is absent**

Run: `cd docs-site && bun test scripts/prepare-brand-assets.test.ts`

- [ ] **Step 3: Implement bounded downloads and atomic output**

Use `fetch` with an abort timeout, `redirect: 'manual'`, an explicit byte limit, allowed MIME types `image/png`, `image/jpeg`, `image/webp`, `image/svg+xml`, and temporary files renamed only after validation. Never log URL query strings.

- [ ] **Step 4: Add `brand:prepare` and build integration**

Make CI call `bun run brand:prepare` before `bun run build`; do not make ordinary local development download remote assets automatically.

- [ ] **Step 5: Run asset tests and secret checks**

Run: `cd docs-site && bun test scripts/prepare-brand-assets.test.ts && bun run check:secrets`

- [ ] **Step 6: Commit**

```bash
git add docs-site/scripts/prepare-brand-assets.ts docs-site/scripts/prepare-brand-assets.test.ts docs-site/package.json docs-site/.gitignore
git commit -m "feat: package documentation brand assets"
```

### Task 4: Restore and generalize documentation deployment workflow

**Files:**
- Modify or restore from `origin/develop`: `.github/workflows/docs-deploy.yml`
- Modify: `docs-site/scripts/docs-workflow-contract.test.ts`
- Modify: `docs-site/deploy/deploy.sh`
- Modify: `docs-site/deploy/deploy.test.sh`

**Interfaces:**
- Consumes: a one-entry development matrix or two-entry production matrix and each target's GitHub Environment variables/secrets.
- Produces: immutable static archives deployed independently to all required `/docs/` paths.

- [ ] **Step 1: Write matrix workflow tests**

Assert `develop` emits only `development`, `main` emits `production-molii` and `production-ixiaozu`, `fail-fast: false` is present, each artifact name includes target ID and SHA, and each job selects `environment: ${{ matrix.target.environment }}`.

- [ ] **Step 2: Run tests and confirm the current single-production workflow fails them**

Run: `cd docs-site && bun test scripts/docs-workflow-contract.test.ts`

- [ ] **Step 3: Build and package one artifact per target**

Run non-browser content/tests once, then run brand preparation, Docusaurus build, link crawl, tar packaging, and artifact upload per target using that GitHub Environment's public documentation variables.

- [ ] **Step 4: Deploy each archive independently**

Retain checksum verification, release directories, atomic `current` symlink switching, public page check, and previous-release rollback. Generalize target validation to the three exact site origins and deployment roots.

- [ ] **Step 5: Add per-target Telegram messages and a partial-success summary**

Identify documentation, site ID, domain, SHA, result, rollback result, and Actions URL. Do not cancel the successful documentation target when its peer fails.

- [ ] **Step 6: Run workflow, deploy-script, and build checks**

Run `cd docs-site && bun test scripts/docs-workflow-contract.test.ts`, the documentation deploy contract, shell syntax, workflow parsing, two production builds, and internal link crawls.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/docs-deploy.yml docs-site/scripts/docs-workflow-contract.test.ts docs-site/deploy
git commit -m "ci: deploy branded docs to both production sites"
```

### Task 5: Documentation regression verification

**Files:**
- Modify: `.ccg/tasks/design-new-api-cicd/review.md`

**Interfaces:**
- Consumes: Tasks 1-4.
- Produces: verified, self-contained, independently deployable documentation artifacts.

- [ ] Run all docs tests, forbidden-term checks, secretlint, API lint, catalog checks, both brand builds, and internal link crawls.
- [ ] Inspect generated canonical URLs, sitemap URLs, API examples, navbar, footer, font, logo, favicon, and social metadata for every target.
- [ ] Verify the iXiaozu artifact can be opened with network disabled and does not request Molii brand assets.
- [ ] Record exact commands, results, remaining operator-provided public asset URLs, and the unavailable external CCG model-wrapper limitation in the review file.

