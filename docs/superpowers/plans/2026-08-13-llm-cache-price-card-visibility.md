# LLM Cache Price Card Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render cached input/read pricing with the same prominence as input and output pricing in the LLM model overview.

**Architecture:** Keep the shared dynamic pricing classifier unchanged and promote cache-read entries locally inside `PriceSection`. Fixed Token pricing uses the same local three-card model, while all other secondary price types retain the existing compact list.

**Tech Stack:** React 19, TypeScript, Bun test, happy-dom, Tailwind CSS.

## Global Constraints

- Input, output and cached input/read use equal price cards.
- Use three columns at the model-detail drawer width and wrap on narrow screens.
- Models without cache pricing render only input and output cards.
- Cache write, media and audio prices remain in the secondary area.
- Do not change prices, multipliers, currency conversion, Token units, backend APIs or billing logic.
- Do not run a production build.

---

### Task 1: Promote cache pricing in fixed and dynamic overview layouts

**Files:**
- Create: `web/src/features/pricing/components/__tests__/model-details-cache-price.test.tsx`
- Modify: `web/src/features/pricing/components/model-details.tsx`

**Interfaces:**
- Consumes: existing `PricingModel`, `TokenUnit` and dynamic pricing summary.
- Produces: `PriceSection` with `data-base-price-primary-grid`, equal cards marked by `data-base-price-card-type` or `data-base-price-card-field`, and the existing secondary area marked by `data-base-price-secondary`.

- [ ] **Step 1: Write failing fixed-price layout tests**

Render a fixed Token model with `cache_ratio` and assert input, output and cache are direct children of the same primary grid. Render a model without `cache_ratio` and assert the grid has only input and output.

- [ ] **Step 2: Verify the fixed-price tests fail**

Run:

```bash
cd web && bun test src/features/pricing/components/__tests__/model-details-cache-price.test.tsx
```

Expected: failure because cache is still rendered in the secondary price area.

- [ ] **Step 3: Promote fixed cached input to the primary grid**

Add cached input to the primary type list only when `cache_ratio` exists, remove it from secondary types, and choose `grid-cols-1 sm:grid-cols-3` for three cards or `grid-cols-2` for two cards. Preserve the existing price formatter and Token unit.

- [ ] **Step 4: Verify fixed-price tests pass**

Run the command from Step 2 and expect the fixed cases to pass.

- [ ] **Step 5: Write the failing dynamic-price layout test**

Render a tiered CNY expression containing input, output, cache read and cache write. Assert cache read is in the primary grid and cache write remains in the secondary area.

- [ ] **Step 6: Verify the dynamic-price test fails**

Run the command from Step 2 and expect the cache-read primary assertion to fail.

- [ ] **Step 7: Promote dynamic cache read locally**

Inside `PriceSection`, derive `detailPrimaryEntries` from existing primary entries plus `cacheReadPrice`; derive `detailSecondaryEntries` by excluding `cacheReadPrice`. Do not modify `getDynamicPricingSummary`, because marketplace cards and pricing columns consume its global classification.

- [ ] **Step 8: Run focused and full verification**

```bash
cd web
bun test src/features/pricing/components/__tests__/model-details-cache-price.test.tsx
bun test src/features/pricing
bun run typecheck
bunx oxfmt --check src/features/pricing/components/model-details.tsx src/features/pricing/components/__tests__/model-details-cache-price.test.tsx
bunx oxlint src/features/pricing/components/model-details.tsx src/features/pricing/components/__tests__/model-details-cache-price.test.tsx
```

Expected: all commands pass.

- [ ] **Step 9: Verify local UI and commit**

Open an LLM model at `http://127.0.0.1:3000/pricing`, confirm the three equal cards and unchanged secondary/group tables, archive the task, then commit:

```bash
git commit -m "fix: emphasize cached input pricing"
```
