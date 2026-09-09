# Pricing Infinite Scroll Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `/pricing` pagination with progressive infinite scrolling while keeping the desktop filter sidebar independently sticky and scrollable.

**Architecture:** Keep the existing client-side `/api/pricing` data source and filtering pipeline. A parent-level hook owns the visible model count so card/table view changes share progress; presentational card and table components render every model passed to them without pagination. An intersection sentinel requests the next 20 models, while unsupported browsers render the complete result set.

**Tech Stack:** React 19, TypeScript, TanStack Table, Vitest, Testing Library, Tailwind CSS

**Spec:** `.ccg/tasks/pricing-infinite-scroll/requirements.md`

## Global Constraints

- Do not change the pricing API or server-side billing logic.
- Preserve existing URL-backed filters and mobile filter drawer behavior.
- Use the existing `DEFAULT_PRICING_PAGE_SIZE` value as the infinite-scroll batch size.
- Do not stage or modify unrelated existing untracked files.
- The configured antigravity and Claude wrapper is absent; record that both required analysis/review invocations failed instead of claiming external review.

---

### Task 1: Progressive model slice hook

**Files:**
- Create: `web/src/features/pricing/hooks/use-infinite-models.ts`
- Create: `web/src/features/pricing/hooks/__tests__/use-infinite-models.test.tsx`

**Interfaces:**
- Consumes: `PricingModel[]` and optional batch size.
- Produces: `visibleModels`, `hasMore`, and a sentinel ref.

- [x] **Step 1: Write failing hook tests**

Test that 45 models initially expose 20, observer intersection exposes 40 then 45, a changed model array resets to 20, and missing `IntersectionObserver` exposes all models.

- [x] **Step 2: Run hook tests and verify RED**

Run: `pnpm --dir web test -- src/features/pricing/hooks/__tests__/use-infinite-models.test.tsx`

Expected: FAIL because `use-infinite-models` does not exist.

- [x] **Step 3: Implement the minimal hook**

Use one `IntersectionObserver` with a positive bottom `rootMargin`, increase the visible count by the batch size, disconnect on cleanup, and reset when the filtered model array changes.

- [x] **Step 4: Run hook tests and verify GREEN**

Run: `pnpm --dir web test -- src/features/pricing/hooks/__tests__/use-infinite-models.test.tsx`

Expected: PASS.

### Task 2: Remove component pagination and integrate the sentinel

**Files:**
- Modify: `web/src/features/pricing/components/model-card-grid.tsx`
- Modify: `web/src/features/pricing/components/pricing-table.tsx`
- Modify: `web/src/features/pricing/index.tsx`
- Modify: `web/src/features/pricing/__tests__/model-cards.test.tsx`
- Modify: `web/src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx`
- Create: `web/src/features/pricing/components/__tests__/pricing-table-infinite.test.tsx`

**Interfaces:**
- Consumes: `visibleModels` and sentinel ref from Task 1.
- Produces: card/table views without pagination controls and a shared infinite-scroll endpoint.

- [x] **Step 1: Replace pagination assertions with failing continuous-render assertions**

Assert that card and table components render all models passed to them and contain no previous/next controls.

- [x] **Step 2: Run component tests and verify RED**

Run the three affected pricing component test files and confirm existing pagination makes them fail.

- [x] **Step 3: Remove internal pagination and connect parent progressive state**

Delete card page state/buttons, disable the table pagination row model and remove `DataTablePagination`, pass `visibleModels` from `Pricing`, and render a status sentinel after the active view.

- [x] **Step 4: Run component and hook tests and verify GREEN**

Run all affected pricing tests and confirm they pass.

### Task 3: Sticky independent sidebar and final verification

**Files:**
- Modify: `web/src/features/pricing/index.tsx`
- Modify: `web/src/features/pricing/components/__tests__/model-directory-controls.test.tsx`
- Create: `.ccg/tasks/pricing-infinite-scroll/review.md`

**Interfaces:**
- Consumes: existing `PricingSidebar` className extension.
- Produces: sticky `overflow-y-auto overscroll-contain` desktop sidebar.

- [x] **Step 1: Verify the sidebar layout in a local browser**

Verify the desktop sidebar receives sticky positioning, a viewport max height, independent vertical overflow, and overscroll containment after the main page scrolls.

- [x] **Step 2: Add `overscroll-contain` without changing the mobile drawer**

Keep the existing sticky/overflow classes and append overscroll containment on the desktop sidebar instance only.

- [x] **Step 3: Run complete verification**

Run affected tests, `pnpm --dir web typecheck`, `pnpm --dir web lint`, and `pnpm --dir web build`.

- [x] **Step 4: Review diff, archive CCG task, and create the required local archive commit**

Record review limitations and verification evidence, move the task under `.ccg/tasks/archive/2026-09/`, and commit only the scoped implementation, tests, plan, and archive.
