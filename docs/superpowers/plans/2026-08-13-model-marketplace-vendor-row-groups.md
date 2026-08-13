# Model Marketplace Vendor Row Groups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Group marketplace cards by vendor so every new vendor starts on a new responsive grid row without adding vendor headings.

**Architecture:** Add a pure stable grouping helper keyed by `vendor_id`, then `vendor_name`, with an individual fallback for unknown vendors. The card grid keeps existing filtering, sorting, pagination and card rendering, but renders each vendor group inside its own responsive grid.

**Tech Stack:** React 19, TypeScript, TanStack Query, Bun test, happy-dom, Tailwind CSS.

## Global Constraints

- Do not show vendor headings or separator copy.
- Keep existing model card content, pricing, filters, sorting, pagination and click behavior.
- Preserve input order within each vendor and order vendor groups by first appearance.
- Models without `vendor_id` and `vendor_name` must not be merged together.
- Do not modify backend APIs or model data.
- Do not run a production build.

---

### Task 1: Stable vendor groups and row-isolated grids

**Files:**
- Create: `web/src/features/pricing/lib/vendor-model-groups.ts`
- Create: `web/src/features/pricing/lib/__tests__/vendor-model-groups.test.ts`
- Create: `web/src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx`
- Modify: `web/src/features/pricing/components/model-card-grid.tsx`

**Interfaces:**
- Consumes: `PricingModel[]` after existing filtering, sorting and pagination.
- Produces: `groupModelsByVendor(models: PricingModel[]): PricingModel[][]`.
- Produces: one DOM element with `data-model-vendor-group` per vendor group, each using `grid-cols-1 md:grid-cols-2 2xl:grid-cols-3`.

- [ ] **Step 1: Write failing grouping tests**

Add literal fixtures proving that repeated vendors are collected in first-appearance order, that model order within a vendor is stable, and that two models without vendor metadata become separate groups.

- [ ] **Step 2: Verify grouping tests fail**

Run:

```bash
cd web && bun test src/features/pricing/lib/__tests__/vendor-model-groups.test.ts
```

Expected: failure because `groupModelsByVendor` does not exist.

- [ ] **Step 3: Implement the pure grouping helper**

Use a `Map<string, PricingModel[]>`. Build the key as `id:<vendor_id>`, otherwise `name:<vendor_name>`, otherwise `unknown:<model id or model name>:<input index>`. Append to the first-created group so group and model order remain stable.

- [ ] **Step 4: Verify grouping tests pass**

Run the command from Step 2 and expect all tests to pass.

- [ ] **Step 5: Write the failing grid integration test**

Render `ModelCardGrid` inside a `QueryClientProvider` whose queries are disabled. Assert that two models from one vendor share the first `[data-model-vendor-group]`, another vendor is in the second group, no vendor heading is rendered, and every group has the existing responsive grid classes.

- [ ] **Step 6: Verify the grid integration test fails**

Run:

```bash
cd web && bun test src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx
```

Expected: failure because the current component renders one shared grid and exposes no vendor groups.

- [ ] **Step 7: Render each stable vendor group as an independent grid**

In `model-card-grid.tsx`, group `pagedModels` with `groupModelsByVendor`, keep the existing outer spacing container and pagination, and render each group as:

```tsx
<div
  key={vendorGroupKey}
  data-model-vendor-group='true'
  className='grid grid-cols-1 gap-3 sm:gap-4 md:grid-cols-2 2xl:grid-cols-3'
>
  {group.map((model) => <ModelCard ... />)}
</div>
```

Use a stable key derived from the first model's vendor identity; do not insert empty cards or visible headings.

- [ ] **Step 8: Verify focused and full pricing tests**

Run:

```bash
cd web
bun test src/features/pricing/lib/__tests__/vendor-model-groups.test.ts
bun test src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx
bun test src/features/pricing
bun run typecheck
bunx oxfmt --check src/features/pricing/lib/vendor-model-groups.ts src/features/pricing/lib/__tests__/vendor-model-groups.test.ts src/features/pricing/components/model-card-grid.tsx src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx
bunx oxlint src/features/pricing/lib/vendor-model-groups.ts src/features/pricing/lib/__tests__/vendor-model-groups.test.ts src/features/pricing/components/model-card-grid.tsx src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx
```

Expected: all commands pass with no errors.

- [ ] **Step 9: Verify local layout and commit**

Open `http://127.0.0.1:3000/pricing`, confirm each vendor starts on a new row at the desktop breakpoint and that no vendor heading appears. Then stage only the four implementation files, update the task review, archive the task, and commit with:

```bash
git commit -m "feat: group marketplace cards by vendor"
```
