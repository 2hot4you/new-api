# Molii High-Density Model Directory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the grouped card marketplace with a ZenMux-style high-density model directory and a continuous independent model detail page, using only Molii's real pricing, metadata, and performance data.

**Architecture:** Add a pure catalog-classification layer for release-date sorting, model categories, modalities, capabilities, and context buckets. Keep `usePricingData` as the single server-data source and extend `useFilters` for directory filters. Replace vendor-group rendering with one responsive bordered grid, navigate cards to the existing `/pricing/$modelId` route, and reshape the existing detail content into anchored continuous sections instead of tabs.

**Tech Stack:** React 19, TypeScript, TanStack Router/Query, Base UI components, Tailwind CSS 4, i18next, Node test runner, Happy DOM, Bun.

## Global Constraints

- Keep Molii navigation, branding, Public Sans/Lora typography, color semantics, and existing public API contracts.
- Do not display token volume, cache-hit rate, provider count, discounts, or free status without a real backend field.
- Do not change pricing formulas, channel behavior, backend routes, or persistence.
- Use `/api/pricing` and existing performance endpoints as the only runtime data sources.
- Default sort is newest release date; undated models sort last and then by model ID.
- Do not group cards by vendor; vendor remains a filter only.
- Do not run a production build; verify in the local development environment only.
- Do not add new dependencies.

---

### Task 1: Catalog classification, release sorting, and filter state

**Files:**
- Create: `web/src/features/pricing/lib/model-directory.ts`
- Create: `web/src/features/pricing/lib/__tests__/model-directory.test.ts`
- Modify: `web/src/features/pricing/constants.ts`
- Modify: `web/src/features/pricing/lib/filters.ts`
- Modify: `web/src/features/pricing/hooks/use-filters.ts`
- Modify: `web/src/routes/pricing/index.tsx`
- Modify: `web/src/routes/pricing/$modelId/index.tsx`

**Interfaces:**
- Produces: `ModelCategory`, `ModelCategoryId`, `ContextBucketId`, `DirectoryFilterState`.
- Produces: `getModelCategory(model)`, `getModelCategories(models)`, `getModelInputModalities(model)`, `getContextBuckets(models)`, `sortModelsByReleaseDate(models)`, and filter predicates consumed by the sidebar and directory.
- Preserves: existing URL search fields and adds optional `category`, `input`, `context`, and `capability` strings.

- [ ] **Step 1: Write failing catalog behavior tests**

Add Node tests covering:

```ts
assert.deepEqual(
  sortModelsByReleaseDate([
    model('undated'),
    model('newer', { release_date: '2026-08-13' }),
    model('older', { release_date: '2026-07-01' }),
  ]).map((item) => item.model_name),
  ['newer', 'older', 'undated']
)

assert.equal(getModelCategory(videoModel), 'video')
assert.equal(getModelCategory(imageModel), 'image')
assert.equal(getModelCategory(chatModel), 'text')
assert.deepEqual(getModelCategories(models).map((item) => item.id), [
  'all',
  'text',
  'image',
  'video',
])
```

Also assert same-date model-ID stability, zero-count category removal, metadata-first modality detection, endpoint fallback, context buckets, and combined vendor/category/capability filtering.

- [ ] **Step 2: Run the focused tests and confirm RED**

Run: `cd web && bun test src/features/pricing/lib/__tests__/model-directory.test.ts`

Expected: FAIL because `model-directory.ts` and the new filter functions do not exist.

- [ ] **Step 3: Implement the pure catalog functions**

Use explicit endpoint fallbacks:

```ts
export type ModelCategoryId =
  | 'all'
  | 'text'
  | 'image'
  | 'video'
  | 'audio'
  | 'embedding'
  | 'rerank'

export function compareModelsByReleaseDate(
  left: PricingModel,
  right: PricingModel
): number {
  const leftTime = parseReleaseDate(left.release_date)
  const rightTime = parseReleaseDate(right.release_date)
  if (leftTime !== rightTime) return rightTime - leftTime
  if (leftTime === Number.NEGATIVE_INFINITY && rightTime !== leftTime) return 1
  return left.model_name.localeCompare(right.model_name)
}
```

Normalize invalid/missing dates to an undated sentinel without letting them sort before dated models. Prefer `output_modalities`, then capability flags, then `supported_endpoint_types` for category classification.

- [ ] **Step 4: Extend filters and route search schemas**

Make `SORT_OPTIONS.RELEASE_DATE = 'release-date'` the default. Extend `filterAndSortModels` and `useFilters` with category, input modality, context bucket, and capability values. Clear all directory filters from one action while preserving price-unit and view preferences.

- [ ] **Step 5: Run focused and existing filter tests**

Run:

```bash
cd web
bun test src/features/pricing/lib/__tests__/model-directory.test.ts src/features/pricing/lib/__tests__
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

```bash
git add web/src/features/pricing/lib web/src/features/pricing/hooks/use-filters.ts web/src/features/pricing/constants.ts web/src/routes/pricing
git commit -m "feat: add model directory taxonomy and release sorting"
```

### Task 2: Directory sidebar, category bar, and toolbar

**Files:**
- Modify: `web/src/features/pricing/index.tsx`
- Modify: `web/src/features/pricing/components/pricing-sidebar.tsx`
- Modify: `web/src/features/pricing/components/pricing-toolbar.tsx`
- Create: `web/src/features/pricing/components/model-category-bar.tsx`
- Create: `web/src/features/pricing/components/__tests__/model-directory-controls.test.tsx`
- Modify: `web/src/features/pricing/components/index.ts`

**Interfaces:**
- Consumes: Task 1 catalog functions and filter state.
- Produces: `ModelCategoryBar` and expanded `PricingSidebar` controls.
- Preserves: mobile filter sheet and standard/recharge pricing switch.

- [ ] **Step 1: Write failing control rendering tests**

Render the controls with text, image, and video fixtures and assert:

```ts
assert.equal(container.querySelectorAll('[data-model-category]').length, 4)
assert.match(container.textContent ?? '', /All/)
assert.match(container.textContent ?? '', /Text/)
assert.match(container.textContent ?? '', /Image/)
assert.match(container.textContent ?? '', /Video/)
assert.equal(container.querySelector('[data-model-category="audio"]'), null)
```

Assert the sidebar exposes input type, context, vendor, capability, protocol, billing type, group, and tag sections; empty options are omitted; category and filters call their supplied handlers.

- [ ] **Step 2: Run focused controls test and confirm RED**

Run: `cd web && bun test src/features/pricing/components/__tests__/model-directory-controls.test.tsx`

Expected: FAIL because `ModelCategoryBar` and expanded controls do not exist.

- [ ] **Step 3: Implement compact controls**

Replace the large hero with a compact content header:

```tsx
<header className='border-b px-4 py-5 lg:px-6'>
  <h1 className='text-2xl font-semibold'>{t('Models')}</h1>
  <p className='text-muted-foreground text-sm'>
    {t('{{count}} models', { count: models.length })}
  </p>
  <ModelCategoryBar ... />
</header>
```

Keep search, price mode, and sorting in the toolbar. Use the existing Sheet for mobile filters. Render only sidebar options with nonzero matches.

- [ ] **Step 4: Wire controls into `Pricing`**

Use a full-width shell below `PublicLayout`, with a sticky desktop sidebar and a min-width-zero content column. Pass all Task 1 filter states through both desktop and mobile control instances.

- [ ] **Step 5: Run controls and type checks**

Run:

```bash
cd web
bun test src/features/pricing/components/__tests__/model-directory-controls.test.tsx
bun run typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

```bash
git add web/src/features/pricing/index.tsx web/src/features/pricing/components
git commit -m "feat: add high-density model directory controls"
```

### Task 3: Continuous model grid and dense model unit

**Files:**
- Modify: `web/src/features/pricing/components/model-card-grid.tsx`
- Modify: `web/src/features/pricing/components/model-card.tsx`
- Create: `web/src/features/pricing/lib/model-card-summary.ts`
- Create: `web/src/features/pricing/lib/__tests__/model-card-summary.test.ts`
- Replace: `web/src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx`
- Delete: `web/src/features/pricing/lib/vendor-model-groups.ts`
- Delete: `web/src/features/pricing/lib/__tests__/vendor-model-groups.test.ts`

**Interfaces:**
- Consumes: sorted `PricingModel[]`, existing price helpers, performance summary map, and route search state.
- Produces: `getCompactPricingSummary(model, options)` for token, request, Grok, Seedance, and dynamic tiered models.
- Changes: `ModelCardGrid.onModelClick` navigates to the independent detail route while copy remains isolated.

- [ ] **Step 1: Replace vendor-group tests with directory-grid RED tests**

Assert one continuous grid, no vendor headings/sections, and responsive columns:

```ts
const grid = container.querySelector('[data-model-directory-grid]')
assert.ok(grid)
assert.match(grid.className, /grid-cols-1/)
assert.match(grid.className, /md:grid-cols-2/)
assert.match(grid.className, /xl:grid-cols-3/)
assert.equal(container.querySelectorAll('[data-model-vendor-section]').length, 0)
```

Assert cards expose vendor identity, description, modalities, compact price, capabilities, release date, endpoint, and real/empty performance status.

- [ ] **Step 2: Write and run compact price tests**

Cover fixed LLM input/output/cache, dynamic tier minimum, Grok per-image/per-second minimum, Seedance minimum tier, and absent pricing. Run focused tests and confirm they fail before implementation.

- [ ] **Step 3: Implement one continuous bordered grid**

Remove `groupModelsByVendor`. Render a container using:

```tsx
<div
  data-model-directory-grid
  className='grid grid-cols-1 overflow-hidden border-y md:grid-cols-2 xl:grid-cols-3'
>
  {models.map((model) => <ModelCard key={model.id ?? model.model_name} ... />)}
</div>
```

Use shared borders with breakpoint-aware right/bottom separators, avoiding independent rounded cards and shadows.

- [ ] **Step 4: Redesign the model unit**

Make the card a keyboard-accessible link-like unit. Preserve copy isolation, real pricing helpers, and performance data. Limit descriptions to three lines, compact capability chips, and avoid full complex price matrices. Use explicit `data-*` hooks for pricing, modalities, capabilities, release date, and performance empty state.

- [ ] **Step 5: Navigate to independent details with filter context**

From `Pricing`, call TanStack Router navigation:

```ts
navigate({
  to: '/pricing/$modelId',
  params: { modelId: model.model_name },
  search: currentDirectorySearch,
})
```

Remove `selectedModelName`, `selectedModel`, and `ModelDetailsDrawer` rendering from the list page.

- [ ] **Step 6: Run card regressions and typecheck**

Run:

```bash
cd web
bun test src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx src/features/pricing/components/__tests__/model-card-text-pricing.test.tsx src/features/pricing/components/__tests__/model-card-grok-pricing.test.tsx src/features/pricing/lib/__tests__/model-card-summary.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 7: Commit Task 3**

```bash
git add web/src/features/pricing
git commit -m "feat: render continuous high-density model directory"
```

### Task 4: Continuous independent model detail page

**Files:**
- Modify: `web/src/features/pricing/components/model-details.tsx`
- Create: `web/src/features/pricing/components/model-details-anchor-nav.tsx`
- Create: `web/src/features/pricing/components/related-models.tsx`
- Create: `web/src/features/pricing/components/__tests__/model-details-directory-page.test.tsx`
- Modify: `web/src/features/pricing/components/index.ts`

**Interfaces:**
- Consumes: existing `ModelHeader`, pricing sections, capability details, performance components, `ModelDetailsApi`, all loaded models, and list search state.
- Produces: anchored sections with IDs `pricing`, `capabilities`, `performance`, and `api`.
- Produces: `RelatedModels` sorted with Task 1 release comparator.

- [ ] **Step 1: Write failing detail-page structure tests**

Render the detail content and assert tabs are absent and all sections coexist:

```ts
assert.equal(container.querySelector('[role="tablist"]'), null)
for (const id of ['pricing', 'capabilities', 'performance', 'api']) {
  assert.ok(container.querySelector(`#${id}`))
}
assert.ok(container.querySelector('[data-model-detail-anchor-nav]'))
```

Assert the related list includes only same-vendor alternatives, excludes the current model, and uses release ordering.

- [ ] **Step 2: Run focused detail test and confirm RED**

Run: `cd web && bun test src/features/pricing/components/__tests__/model-details-directory-page.test.tsx`

Expected: FAIL because the content still uses tabs and has no related-model section.

- [ ] **Step 3: Replace tabs with continuous sections**

Keep existing pricing and API subcomponents intact, but compose them as semantic sections:

```tsx
<ModelDetailsAnchorNav items={availableAnchors} />
<section id='pricing' className='scroll-mt-24'>...</section>
<section id='capabilities' className='scroll-mt-24'>...</section>
<section id='performance' className='scroll-mt-24'>...</section>
<section id='api' className='scroll-mt-24'>...</section>
```

Remove the drawer wrapper export and unused Tabs imports only after the list page no longer consumes them.

- [ ] **Step 4: Add detail hero actions and related models**

Preserve back-navigation search state. Add an API anchor action. Show Playground only when the model supports an actual text/chat endpoint. Pass `models` into the page content and render up to six same-vendor alternatives ordered by release date.

- [ ] **Step 5: Run existing detail regressions and typecheck**

Run:

```bash
cd web
bun test src/features/pricing/components/__tests__/model-details-directory-page.test.tsx src/features/pricing/components/__tests__/model-details-cache-price.test.tsx src/features/pricing/components/__tests__/model-details-capabilities.test.tsx src/features/pricing/components/__tests__/video-api-details.test.ts
bun run typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit Task 4**

```bash
git add web/src/features/pricing/components/model-details.tsx web/src/features/pricing/components/model-details-anchor-nav.tsx web/src/features/pricing/components/related-models.tsx web/src/features/pricing/components/__tests__ web/src/features/pricing/components/index.ts
git commit -m "feat: redesign model details as a continuous page"
```

### Task 5: Translation, layout regression, and local browser verification

**Files:**
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify only if required by sync: other `web/src/i18n/locales/*.json`
- Create: `web/src/features/pricing/components/__tests__/model-directory-layout.test.tsx`
- Modify: `.ccg/tasks/pricing-directory-refactor/task.json`
- Create: `.ccg/tasks/pricing-directory-refactor/review.md`

**Interfaces:**
- Consumes: all directory and detail components.
- Produces: complete English, Simplified Chinese, and Traditional Chinese UI labels with no raw fallback keys.

- [ ] **Step 1: Add layout contract tests**

Assert desktop sidebar stickiness, responsive one/two/three-column classes, mobile filter trigger, absence of the old gradient hero, absence of vendor group sections, and independent model links.

- [ ] **Step 2: Run layout test and confirm RED where labels/contracts are missing**

Run: `cd web && bun test src/features/pricing/components/__tests__/model-directory-layout.test.tsx`

- [ ] **Step 3: Add and synchronize translations**

Add concise labels for categories, filters, newest sort, pricing summaries, missing performance, anchors, publish date, related models, and back navigation. Run `bun run i18n:sync` and inspect the diff so unrelated locale content is not rewritten.

- [ ] **Step 4: Run complete local static verification without production build**

Run:

```bash
cd web
bun test
bun run typecheck
bun run lint
bun run format:check
cd ..
git diff --check
```

Expected: all commands PASS. Do not run `bun run build`.

- [ ] **Step 5: Verify the running development page**

Restart only the local frontend development process if hot reload is not healthy. Inspect `http://127.0.0.1:3000/pricing` at desktop and mobile widths, plus one LLM, one Grok model, and one Seedance model detail route. Confirm:

- default newest ordering;
- vendor filtering;
- category counts;
- continuous three-column grid;
- independent detail navigation and browser back behavior;
- price summaries and full detail matrices;
- performance empty states;
- no horizontal overflow;
- no browser console errors.

- [ ] **Step 6: Record review and archive the task**

Write verification evidence and remaining nonblocking limitations to `review.md`, change the task to completed, move it to `.ccg/tasks/archive/2026-08/pricing-directory-refactor`, and commit:

```bash
git add web/src .ccg/tasks docs/superpowers
git commit -m "chore: archive model directory refactor task"
```
