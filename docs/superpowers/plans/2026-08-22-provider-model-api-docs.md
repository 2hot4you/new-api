# Provider Model API Docs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Generate complete, default-style Docusaurus API documentation for every Provider and model published by the Development pricing catalog.

**Architecture:** An explicit network sync writes a sanitized, deterministic catalog snapshot. Offline generation converts that snapshot into Provider/model MDX pages, while shared OpenAPI reference pages document each supported protocol. Navigation consumes generated Docusaurus category metadata.

**Tech Stack:** Bun 1.3.14, TypeScript/JavaScript, Docusaurus 3.10.2, MDX, OpenAPI 3.1, Redocly CLI.

**Spec:** `docs/superpowers/specs/2026-08-22-provider-model-api-docs-design.md`

## Global Constraints

- The authoritative catalog URL is exactly `https://dev.molii.co/api/pricing`.
- Normal `dev`, `build`, and tests must not require network access.
- Generated pages use default Docusaurus MDX styling.
- Only public allowlisted catalog fields may enter the snapshot or documentation.
- Unknown endpoint types, unknown vendor references, duplicate slugs, and malformed responses fail closed.
- Existing hand-written Grok and Seedance guides remain intact.
- Examples never contain real keys, signed URLs, credentials, private routes, or administrator configuration.

---

### Task 1: Catalog sync and deterministic Provider/model MDX generation

**Files:**
- Create: `docs-site/data/development-model-catalog.json`
- Create: `docs-site/scripts/model-catalog.mjs`
- Create: `docs-site/scripts/sync-model-catalog.mjs`
- Create: `docs-site/scripts/generate-model-docs.mjs`
- Create: `docs-site/scripts/model-catalog.test.ts`
- Create/generated: `docs-site/docs/providers/**`
- Modify: `docs-site/package.json`

**Interfaces:**
- Produces `sanitizeCatalogResponse(raw)` returning `{source, pricing_version, vendors, models}` with public fields only.
- Produces `generateCatalogDocs({catalog, outputRoot})` writing the complete owned `docs/providers/` tree.
- Adds `catalog:sync`, `catalog:generate`, `catalog:check`, `predev`, and `prebuild` scripts.

- [ ] **Step 1: Write failing behavior tests**

Use a literal fixture with two ordered Providers and one model for each endpoint type. Assert sanitized output strips group/pricing/internal fields; malformed/unknown/duplicate inputs reject; generated Provider/model file counts equal the snapshot; routes and category positions are deterministic; Claude/Gemini/Grok/image/video pages contain their literal public endpoints; generation never fetches the network.

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd docs-site && bun test scripts/model-catalog.test.ts`

Expected: failure because the catalog modules do not exist.

- [ ] **Step 3: Implement public-field sanitization and offline generation**

Use explicit field pickers and a closed endpoint-type map. Write files through a temporary owned directory and rename only within `docs/providers`. Generate `_category_.json`, Provider `index.mdx`, model MDX pages, and the `/providers` index with default Markdown/MDX elements.

- [ ] **Step 4: Verify GREEN and sync the Development snapshot**

Run:

```bash
cd docs-site
bun test scripts/model-catalog.test.ts
bun run catalog:sync
bun run catalog:generate
bun run catalog:check
```

Expected: all tests pass; snapshot contains every Development Provider/model; a second generate produces no diff.

### Task 2: Add core LLM protocol OpenAPI and API reference pages

**Files:**
- Modify: `docs-site/openapi/relay.public.template.yaml`
- Modify: `docs-site/openapi/public-api-surface.json`
- Create: `docs-site/docs/api-reference/chat-completions.mdx`
- Create: `docs-site/docs/api-reference/responses.mdx`
- Create: `docs-site/docs/api-reference/anthropic-messages.mdx`
- Create: `docs-site/docs/api-reference/gemini-generate-content.mdx`
- Modify: `docs-site/scripts/mdx-api-reference-contract.test.ts`

**Interfaces:**
- Produces public operations for `POST /v1/chat/completions`, `POST /v1/responses`, `POST /v1/messages`, and `POST /v1beta/models/{model}:generateContent`.
- Produces stable reference routes consumed by generated model pages.

- [ ] **Step 1: Extend contract tests first**

Assert the public allowlist and generated OpenAPI contain the four routes with unique operation IDs, request schemas, success/error responses, and no administrator schema. Assert the four MDX pages expose request/response fields, safe curl examples, and links to authentication/errors guidance.

- [ ] **Step 2: Run tests and verify RED**

Run: `cd docs-site && bun test scripts/mdx-api-reference-contract.test.ts scripts/prepare-openapi.test.ts`

Expected: failure because the four operations/pages are absent.

- [ ] **Step 3: Add minimal OpenAPI schemas and default MDX references**

Model only the user-visible request/response core shared by the gateway; keep optional bodies extensible where protocol compatibility requires it. Use Bearer for OpenAI routes, document `x-api-key` plus `anthropic-version` for Anthropic compatibility, and `x-goog-api-key` for Gemini compatibility without exposing query-key examples.

- [ ] **Step 4: Verify GREEN and lint the public spec**

Run:

```bash
cd docs-site
bun test scripts/mdx-api-reference-contract.test.ts scripts/prepare-openapi.test.ts
bun run api:lint
```

Expected: tests and Redocly lint pass without warnings.

### Task 3: Integrate navigation, legacy guides, and whole-catalog contracts

**Files:**
- Modify: `docs-site/sidebars.ts`
- Modify: `docs-site/docs/models.md`
- Modify: `docs-site/docs/models/overview.mdx`
- Modify: `docs-site/docs/api-reference/index.mdx`
- Create: `docs-site/scripts/provider-model-content-contract.test.ts`
- Modify: `docs-site/scripts/content-contract.test.ts`

**Interfaces:**
- Consumes generated `/providers` tree and the four protocol reference routes.
- Produces a sidebar-reachable Provider category and catalog-wide coverage contract.

- [ ] **Step 1: Write the integration contract first**

Load the real sanitized snapshot, generated tree, and sidebar. Assert every Provider/model appears exactly once, in display order, with no orphan model; all declared endpoint types link to an existing API reference; Grok/Seedance pages link to existing deep guides; and the sidebar uses Docusaurus autogenerated Provider navigation.

- [ ] **Step 2: Run the integration test and verify RED**

Run: `cd docs-site && bun test scripts/provider-model-content-contract.test.ts scripts/content-contract.test.ts`

Expected: failure because navigation and catalog overview are not integrated.

- [ ] **Step 3: Add navigation and update overview copy**

Add “Provider 与模型” using the generated directory. Replace the legacy seven-model-only overview with a link to the generated catalog while preserving specialized Grok/Seedance guidance. Add the new protocol pages to API reference navigation.

- [ ] **Step 4: Run full verification**

Run:

```bash
cd docs-site
bun test
bun run catalog:check
bun run api:lint
bun run check:forbidden
bun run check:secrets
bun run build
bun run check:links
git diff --check
```

Expected: every command passes; build contains 10 Provider categories and 35 model routes from the Development snapshot.

