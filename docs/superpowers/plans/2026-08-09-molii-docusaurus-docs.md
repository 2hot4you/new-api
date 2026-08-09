# Molii Docusaurus User Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete, Chinese-first Molii user documentation site covering ordinary platform operations and the Seedance/Grok/Assets public APIs, with local hot reload and self-hostable static output.

**Architecture:** `docs-site/` is an independent Docusaurus application inside the New API repository. Handwritten MDX provides product and platform guidance; a public allowlisted OpenAPI bundle generates API reference pages. The site runs locally on `127.0.0.1:3100` and produces only `docs-site/build/` for later deployment to the user's own server.

**Tech Stack:** Docusaurus 3.10.2, TypeScript, React 19, Bun 1.3.14, docusaurus-plugin-openapi-docs 5.1.3, Redocly CLI, local Chinese search, linkinator, secretlint.

## Global Constraints

- Do not modify `web/`, the Go backend build, root Makefile, root Dockerfile, Compose files, or existing New API runtime.
- Do not add Cloudflare Pages, Vercel, GitHub Pages, deployment credentials, or cloud workflows.
- Publish only ordinary-user platform operations and public Seedance/Grok/Assets APIs.
- Physically exclude administrator, channel, system settings, operations, database, Redis, upstream credentials, and internal management APIs.
- Examples use `$MOLII_API_KEY`, `$MOLII_BASE_URL`, `task_xxx`, and `example.com`; never use a token-shaped realistic secret.
- No paid requests in tests or build steps.
- Preserve New API and QuantumNous attribution required by repository policy.
- Local development address is `http://127.0.0.1:3100`.

---

### Task 1: Scaffold the independent Docusaurus application

**Files:**
- Create: `docs-site/package.json`
- Create: `docs-site/bun.lock`
- Create: `docs-site/.env.example`
- Create: `docs-site/.gitignore`
- Create: `docs-site/.nvmrc`
- Create: `docs-site/tsconfig.json`
- Create: `docs-site/docusaurus.config.ts`
- Create: `docs-site/sidebars.ts`
- Create: `docs-site/src/css/custom.css`
- Create: `docs-site/src/pages/index.tsx`
- Create: `docs-site/src/pages/index.module.css`
- Create: `docs-site/static/img/molii-mark.svg`
- Create: `docs-site/README.md`
- Modify: `.dockerignore`

**Interfaces:**
- Produces: `bun run dev`, `bun run build`, and a static `build/` directory.
- Consumes: public environment variables only.

- [ ] **Step 1: Create failing configuration tests**

Add `docs-site/src/config.test.ts` for URL origin validation, slash-normalized base URL, no secrets in exposed environment, and development `noIndex` behavior.

```ts
test('rejects a site URL with a path', () => {
  expect(() => resolvePublicConfig({DOCS_SITE_URL: 'https://docs.example.com/path'})).toThrow();
});
```

- [ ] **Step 2: Scaffold with exact pinned dependencies**

Pin Docusaurus `3.10.2`, OpenAPI plugin/theme `5.1.3`, local search `2.0.1`, React/React DOM `19.2.8`, Redocly `2.46.0`, linkinator `8.0.3`, and secretlint preset `13.0.4`. Set `packageManager` to `bun@1.3.14` and engines to Node `>=22.12.0`.

- [ ] **Step 3: Implement public configuration**

`.env.example` contains only:

```dotenv
DOCS_ENV=development
DOCS_SITE_URL=http://127.0.0.1:3100
DOCS_BASE_URL=/
DOCS_API_BASE_URL=http://127.0.0.1:3000
```

The config validates values at startup, uses `zh-Hans` as the only initial locale, disables indexing outside production, and sets broken links, anchors, Markdown links/images, and duplicate routes to throw.

- [ ] **Step 4: Implement the branded shell and navigation**

Create a professional Molii landing page with paths for Quick Start, Platform, API Basics, Models, API Reference, Examples, Help, and Changelog. Keep attribution in the footer without exposing internal configuration.

- [ ] **Step 5: Install and verify**

Run: `cd docs-site && bun install --frozen-lockfile && bun test && bun run build`

Expected: PASS and `docs-site/build/index.html` exists.

- [ ] **Step 6: Commit Task 1**

```bash
git add docs-site .dockerignore
git commit -m "feat: scaffold Molii Docusaurus docs"
```

### Task 2: Build the public OpenAPI allowlist pipeline

**Files:**
- Create: `docs-site/openapi/public-api-surface.json`
- Create: `docs-site/scripts/prepare-openapi.mjs`
- Create: `docs-site/scripts/prepare-openapi.test.ts`
- Create: `docs-site/redocly.yaml`
- Create: `docs-site/openapi/relay.public.template.yaml`
- Modify: `docs-site/package.json`
- Modify: `docs-site/docusaurus.config.ts`
- Modify: `docs-site/sidebars.ts`

**Interfaces:**
- Produces: `generated/openapi/relay.public.json` and generated API MDX under `generated/api/`.
- Consumes: exact route/DTO evidence from Go source and an explicit method+path allowlist.

- [ ] **Step 1: Write failing pipeline tests**

Test that only allowlisted operations survive, all `operationId` values are stable and unique, only Bearer authentication remains, admin schemas are pruned, server URL comes from `DOCS_API_BASE_URL`, and input objects are not mutated.

- [ ] **Step 2: Define the initial public surface**

Allow only implemented endpoints in scope:

```text
GET  /v1/models
POST /v1/video/generations
GET  /v1/video/generations/{task_id}
POST /v1/images/generations
POST /v1/images/edits
POST /v1/videos
POST /v1/videos/edits
GET  /v1/videos/{task_id}
GET  /v1/videos/{task_id}/content
POST /v1/assets
GET  /v1/assets/{id}
DELETE /v1/assets/{id}
```

Every operation includes models, required/conditional fields, enums, defaults, ranges, success/failure examples, retry guidance, and billing notes. Do not import `docs/openapi/api.json`.

- [ ] **Step 3: Implement preparation and validation**

The script reads the public template and allowlist, rejects missing summaries/responses/security or duplicate routes, injects the public server, prunes unreachable schemas, and writes generated output. It must fail if `/api/channel`, `/api/models`, `/api/assets/admin`, or an administrator schema appears.

- [ ] **Step 4: Configure generated API docs**

Use a separate docs plugin instance at `/api-reference/`, generate MDX, and keep product/sidebar content isolated from API reference content.

- [ ] **Step 5: Verify pipeline**

Run: `cd docs-site && bun test scripts/prepare-openapi.test.ts && bun run api:lint && bun run api:generate && bun run build`

- [ ] **Step 6: Commit Task 2**

```bash
git add docs-site/openapi docs-site/scripts docs-site/redocly.yaml docs-site/package.json docs-site/docusaurus.config.ts docs-site/sidebars.ts
git commit -m "feat: generate public Molii API reference"
```

### Task 3: Write API foundations and quick starts

**Files:**
- Create: `docs-site/docs/getting-started/quickstart.mdx`
- Create: `docs-site/docs/getting-started/video-workflow.mdx`
- Create: `docs-site/docs/api-basics/authentication.mdx`
- Create: `docs-site/docs/api-basics/base-url.mdx`
- Create: `docs-site/docs/api-basics/async-tasks.mdx`
- Create: `docs-site/docs/api-basics/media-inputs.mdx`
- Create: `docs-site/docs/api-basics/errors-retries.mdx`
- Create: `docs-site/docs/api-basics/billing-and-usage.mdx`
- Create: `docs-site/src/components/ApiLifecycle.tsx`
- Modify: `docs-site/sidebars.ts`

**Interfaces:**
- Consumes: verified backend routes/DTOs and public OpenAPI from Task 2.
- Produces: shared concepts referenced by all model/API guides.

- [ ] **Step 1: Add documentation contract tests**

Create `docs-site/scripts/content-contract.test.ts` to assert every P0 page has `audience: user`, `apiVersion: v1`, `lastReviewed`, source commit, no realistic secret, no forbidden management path, and no automatic retry of paid POST examples.

- [ ] **Step 2: Write the eight foundation pages**

Document Bearer API key security, environment-based Base URL, create→poll→download, terminal states, exponential backoff with total timeout, URL/Data URL/asset URI rules, stable error envelope, request ID, idempotency warning, estimated/final billing, and refund-pending semantics.

- [ ] **Step 3: Add complete curl/Python/TypeScript examples**

Examples must be copyable and use environment variables. Polling examples treat POST as non-idempotent, retry only safe GET requests, stop at terminal state, and download to a file only after validating HTTP status and content type.

- [ ] **Step 4: Verify content**

Run: `cd docs-site && bun test scripts/content-contract.test.ts && bun run build`

- [ ] **Step 5: Commit Task 3**

```bash
git add docs-site/docs/getting-started docs-site/docs/api-basics docs-site/src/components docs-site/sidebars.ts docs-site/scripts/content-contract.test.ts
git commit -m "docs: add Molii API foundations"
```

### Task 4: Write Seedance and temporary asset documentation

**Files:**
- Create: `docs-site/docs/models/overview.mdx`
- Create: `docs-site/docs/models/seedance-2.mdx`
- Create: `docs-site/docs/guides/seedance-multimodal.mdx`
- Create: `docs-site/docs/guides/temporary-assets.mdx`
- Create: `docs-site/docs/examples/seedance-curl.mdx`
- Create: `docs-site/docs/examples/seedance-python.mdx`
- Create: `docs-site/docs/examples/seedance-typescript.mdx`
- Create: `docs-site/src/components/ParameterTable.tsx`
- Modify: `docs-site/sidebars.ts`

**Interfaces:**
- Consumes: `relay/channel/task/starai/constants.go`, adaptor validators/tests, `/v1/assets` controller/service, and the existing Seedance Markdown as a non-authoritative source.

- [ ] **Step 1: Add machine-readable parameter fixtures**

Create content tests covering model IDs, standard/Fast resolution support, duration `-1` or `4–15`, ratio enums, `generate_audio`, watermark, tools, content roles, media count limits, and mutual exclusions.

- [ ] **Step 2: Write model and multimodal guides**

Include valid/invalid JSON comparisons for text-to-video, first frame, first+last frame, reference image/video/audio, editing/extension behavior, and web search. Clearly distinguish gateway validation from service capability recommendations.

- [ ] **Step 3: Write the asset lifecycle guide**

Cover create/get/delete, PROCESSING/ACTIVE/error states, user ownership, `asset://` URI, TTL as response-driven, URL reachability, expiration, and deletion. Do not mention admin assets or storage implementation.

- [ ] **Step 4: Verify and commit**

Run: `cd docs-site && bun test && bun run build`

```bash
git add docs-site/docs/models docs-site/docs/guides docs-site/docs/examples/seedance-* docs-site/src/components/ParameterTable.tsx docs-site/sidebars.ts
git commit -m "docs: add Seedance and asset guides"
```

### Task 5: Write Grok Imagine image and video documentation

**Files:**
- Create: `docs-site/docs/models/grok-imagine-image.mdx`
- Create: `docs-site/docs/models/grok-imagine-video.mdx`
- Create: `docs-site/docs/examples/grok-image-curl.mdx`
- Create: `docs-site/docs/examples/grok-video-curl.mdx`
- Create: `docs-site/docs/examples/grok-poll-download.mdx`
- Modify: `docs-site/docs/models/overview.mdx`
- Modify: `docs-site/sidebars.ts`

**Interfaces:**
- Consumes: finalized channel hardening Tasks 3–5, current Grok adaptors/tests, and generated OpenAPI.
- Produces: user-facing contract with no `file_id` support and explicit billing-resolution behavior.

- [ ] **Step 1: Freeze and test the Grok parameter contract**

Assert image models, prompt bounds, `n`, resolution, aspect-ratio enums, edit image count, video models, duration, generation/edit support, 1.5 image requirement, and supported resolution behavior against machine-readable content fixtures.

- [ ] **Step 2: Write image documentation**

Cover generation and edits, URL/Data URL media, response count, result URL lifetime, safety errors, billing units, and complete curl examples.

- [ ] **Step 3: Write video documentation**

Cover legacy/1.5/1.5 Preview differences, text/image/video inputs, generation/edit restrictions, async create/status/content flow, resolution billing source, refund/review states, preview and download.

- [ ] **Step 4: Verify and commit**

Run: `cd docs-site && bun test && bun run build`

```bash
git add docs-site/docs/models docs-site/docs/examples/grok-* docs-site/sidebars.ts
git commit -m "docs: add Grok Imagine API guides"
```

### Task 6: Write ordinary-user platform operation documentation

**Files:**
- Create: `docs-site/docs/platform/register-and-sign-in.mdx`
- Create: `docs-site/docs/platform/dashboard.mdx`
- Create: `docs-site/docs/platform/api-keys.mdx`
- Create: `docs-site/docs/platform/model-square-and-playground.mdx`
- Create: `docs-site/docs/platform/temporary-assets.mdx`
- Create: `docs-site/docs/platform/usage-and-generation-records.mdx`
- Create: `docs-site/docs/platform/wallet-and-billing.mdx`
- Create: `docs-site/docs/platform/profile-and-security.mdx`
- Create: `docs-site/static/img/platform/.gitkeep`
- Modify: `docs-site/sidebars.ts`

**Interfaces:**
- Consumes: ordinary-user routes and components under `web/src/features/{auth,keys,pricing,playground,dashboard,temporary-assets,usage-logs,wallet,profile}`.

- [ ] **Step 1: Create a user-only route/source manifest**

Add a tested manifest listing the exact ordinary-user route and source files for each page. The test rejects `/channels`, `/users`, `/system-settings`, administrator assets, and management APIs.

- [ ] **Step 2: Write the eight operation guides**

Use current Chinese UI labels and conditional wording for feature flags. Document success, empty, disabled, expired, and error states. Playground must not claim Seedance/Grok support if the current UI only supports chat.

- [ ] **Step 3: Add screenshot placeholders without sensitive screenshots**

Each page defines the required normal-user screenshot and alt text, but no real account data is committed. When screenshots are captured later, they must use masked/synthetic data and no administrator navigation.

- [ ] **Step 4: Verify and commit**

Run: `cd docs-site && bun test && bun run build`

```bash
git add docs-site/docs/platform docs-site/static/img/platform docs-site/sidebars.ts
git commit -m "docs: add Molii user platform guides"
```

### Task 7: Add help, changelog, local search, and quality gates

**Files:**
- Create: `docs-site/docs/help/troubleshooting.mdx`
- Create: `docs-site/docs/help/contact-support.mdx`
- Create: `docs-site/docs/changelog.mdx`
- Create: `docs-site/quality/forbidden-terms.txt`
- Create: `docs-site/scripts/check-forbidden-terms.mjs`
- Create: `docs-site/scripts/check-forbidden-terms.test.ts`
- Create: `docs-site/.secretlintrc.json`
- Create: `docs-site/.secretlintignore`
- Create: `docs-site/examples/nginx.conf.example`
- Modify: `docs-site/package.json`
- Modify: `docs-site/docusaurus.config.ts`
- Modify: `docs-site/README.md`

**Interfaces:**
- Produces: `bun run check`, deterministic internal link checks, optional external link checks, Chinese local search, and self-hosting notes.

- [ ] **Step 1: Write scanner tests**

Test exact file/line reporting for forbidden administrator paths, provider-internal terms, realistic secrets, placeholders, and internal domains. New API and QuantumNous attribution must remain allowed.

- [ ] **Step 2: Implement quality scripts**

Provide `check:forbidden`, `check:secrets`, `check:links`, `check:links:external`, and aggregate `check`. External link checking remains separate from deterministic CI/local checks.

- [ ] **Step 3: Configure Chinese local search**

Use `@cmfcmf/docusaurus-search-local` with `language: ['zh']` and `nodejieba`. Explain that the search index is created by production build and tested with `bun run preview`.

- [ ] **Step 4: Add self-hosting example only**

The Nginx example serves `build/` with SPA/static fallbacks and immutable asset caching. It contains no certificate, upload, Docker, Compose, server credential, or automatic deployment logic.

- [ ] **Step 5: Run the complete documentation checks**

Run: `cd docs-site && bun run check`

Expected: PASS.

- [ ] **Step 6: Commit Task 7**

```bash
git add docs-site
git commit -m "chore: validate and self-host Molii docs"
```

### Task 8: Run the documentation site persistently for local development

**Files:**
- Create: `docs-site/scripts/watch-and-run.sh`
- Modify: `docs-site/README.md`
- External runtime file: `~/Library/LaunchAgents/com.molii.docs-site.plist`

**Interfaces:**
- Produces: local service `http://127.0.0.1:3100` with Docusaurus hot reload and login-time restart.

- [ ] **Step 1: Write shell/plist validation checks**

Validate the script with `zsh -n`, the plist with `plutil -lint`, and ensure the working directory points only at `docs-site/`.

- [ ] **Step 2: Implement the local runner**

Use Bun to run Docusaurus dev at 127.0.0.1:3100. KeepAlive and RunAtLoad restart the process. Do not store API keys or secrets in the plist.

- [ ] **Step 3: Load and verify the service**

Run health checks against `/`, modify a documentation file without changing content, confirm hot reload, and verify 3000/5173/8787/8788 remain unaffected.

- [ ] **Step 4: Commit tracked runner documentation**

```bash
git add docs-site/scripts/watch-and-run.sh docs-site/README.md
git commit -m "chore: keep Molii docs running locally"
```

### Task 9: Full documentation review

**Files:**
- Create: `.ccg/tasks/build-molii-docusaurus-docs/review-docs.md`

- [ ] **Step 1: Run all checks**

Run: `cd docs-site && bun install --frozen-lockfile && bun run check`.

- [ ] **Step 2: Verify static output**

Run: `cd docs-site && bun run preview`, then inspect desktop/mobile navigation, search, API parameter tables, code copy, 404, and generated API pages.

- [ ] **Step 3: Audit public boundaries**

Confirm no management path/schema, realistic secret, upstream credential/domain, administrator screenshot, deployment credential, or internal billing anchor exists in `docs-site/build/`.

- [ ] **Step 4: Record findings**

Write commands, results, remaining ordinary-user screenshot work, and Critical/Warning/Info findings to `.ccg/tasks/build-molii-docusaurus-docs/review-docs.md`.
