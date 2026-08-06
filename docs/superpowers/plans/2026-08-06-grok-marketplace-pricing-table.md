# Grok Marketplace Pricing Table Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show backend-provided Grok image and video catalog prices as compact three-column tables on marketplace cards.

**Architecture:** Add a pure row builder that sorts backend price maps into stable resolution order, then render those rows through a Grok-only compact matrix component. `ModelCard` selects this component whenever `molii_grok_pricing` is present, while existing Seedance and generic branches remain unchanged.

**Tech Stack:** TypeScript, React 19, i18next, happy-dom, Node test runner, Rsbuild.

---

### Task 1: Define and test Grok pricing rows

**Files:**
- Create: `web/src/features/pricing/lib/grok-pricing-table.ts`
- Create: `web/src/features/pricing/lib/__tests__/grok-pricing-table.test.ts`

- [ ] **Step 1: Write failing tests**

Cover image pricing, legacy video media-input pricing, Video 1.5 image-input pricing, stable `1k/2k` and `480p/720p/1080p` sorting, and empty output maps.

- [ ] **Step 2: Verify the tests fail**

Run:

```bash
bun test src/features/pricing/lib/__tests__/grok-pricing-table.test.ts
```

Expected: failure because `grok-pricing-table.ts` does not exist.

- [ ] **Step 3: Implement the pure builder**

Export `buildGrokPricingRows(pricing: MoliiGrokPricing): GrokPricingTableRow[]`. Each row contains `resolution`, `outputPrice`, and optional `imageInputPrice` / `videoInputPrice`, populated only from `MoliiGrokPricing`.

- [ ] **Step 4: Verify the focused tests pass**

Run the same Bun command and expect all tests to pass.

### Task 2: Render the compact three-column matrix

**Files:**
- Create: `web/src/features/pricing/components/grok-pricing-matrix.tsx`
- Create: `web/src/features/pricing/components/__tests__/grok-pricing-matrix.test.tsx`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/en.json`

- [ ] **Step 1: Write failing component tests**

Render image, legacy video, and Video 1.5 pricing with happy-dom. Assert three headers, ordered rows, visible currency/unit text, both legacy media-input charges, and the unavailable state.

- [ ] **Step 2: Verify the component test fails**

Run:

```bash
bun test src/features/pricing/components/__tests__/grok-pricing-matrix.test.tsx
```

Expected: failure because the matrix component does not exist.

- [ ] **Step 3: Implement the component and translations**

Use the same rounded table, muted header, compact spacing, monospaced prices, and border hierarchy as `VideoPricingMatrix`. Add localized labels for image output, video output, media input, and resolution/input billing summary.

- [ ] **Step 4: Verify component tests pass**

Run the focused component test and expect all assertions to pass.

### Task 3: Integrate the matrix into marketplace cards

**Files:**
- Modify: `web/src/features/pricing/components/model-card.tsx`

- [ ] **Step 1: Select the Grok presentation from backend data**

When `model.molii_grok_pricing` exists, replace the `¥1 / request` header with `Tiered by resolution and input type`, render `GrokPricingMatrix` below the description, and omit the footer Token-unit label.

- [ ] **Step 2: Preserve existing branches**

Keep `video_pricing`, dynamic pricing, token pricing, and fixed-price rendering unchanged when Grok catalog pricing is absent.

- [ ] **Step 3: Run all pricing tests**

Run:

```bash
bun test src/features/pricing/lib/__tests__ src/features/pricing/components/__tests__
```

Expected: all pricing tests pass.

### Task 4: Verify, restart, and archive

**Files:**
- Modify: `.ccg/tasks/grok-marketplace-pricing-table/task.json`
- Create: `.ccg/tasks/grok-marketplace-pricing-table/review.md`

- [ ] **Step 1: Run static verification**

Run targeted Oxlint, `pnpm typecheck`, `pnpm format:check`, and `pnpm build` from `web/`.

- [ ] **Step 2: Review the diff**

Confirm no hard-coded catalog prices, no changes to backend billing, no secret values, and no unrelated files.

- [ ] **Step 3: Rebuild and restart local service**

Build the embedded Go binary to `/Users/naf/Library/Application Support/Molii/new-api/new-api`, restart `com.molii.new-api`, and wait for `/api/status` to return HTTP 200.

- [ ] **Step 4: Browser QA**

At `http://127.0.0.1:3000/pricing`, compare Seedance and all four Grok cards, verify price units and row order, and confirm no relevant console errors.

- [ ] **Step 5: Commit and archive**

Commit the implementation, mark the CCG task completed, move it to `.ccg/tasks/archive/2026-08/`, and commit the archive. Do not push.
