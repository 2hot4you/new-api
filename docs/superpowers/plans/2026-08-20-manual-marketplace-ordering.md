# Manual Marketplace Ordering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist administrator-defined model and vendor order, expose atomic reorder APIs, and make `/pricing` use those orders by default instead of release date.

**Architecture:** Add independent `display_order` columns to `models` and `vendors`, initialize and append them deterministically, and update complete order sets transactionally through administrator-only APIs. The model metadata page and vendor manager use dedicated full-list reorder modes backed by `motion/react`; the public pricing cache emits model and vendor records in persisted order while keeping visitor-selected name and price sorts.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL, React 19, TypeScript, TanStack Query/Table, `motion/react`, Bun, Vitest/Node test utilities.

**Spec:** `docs/superpowers/specs/2026-08-20-manual-marketplace-ordering-design.md`

## Global Constraints

- The default `/pricing` order comes from persisted model order; `release_date` must not participate in that default.
- Model order and vendor order are independent. Vendor order controls vendor selectors, not model grouping.
- Retain visitor-selected name, price ascending, and price descending sorts; remove the “Newest” option.
- Reorder writes require a complete current ID set and must commit atomically or not at all.
- Support SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+ through GORM-compatible operations.
- New user-facing text must use `useTranslation()` and all seven frontend locale files must remain complete.
- Reuse `motion/react`; do not add a drag-and-drop dependency.
- Do not modify model pricing, billing, publication state, release dates, or capabilities.

---

### Task 1: Persist model and vendor display order

**Files:**
- Modify: `model/model_meta.go`
- Modify: `model/vendor_meta.go`
- Modify: `model/main.go`
- Create: `model/marketplace_display_order.go`
- Create: `model/marketplace_display_order_test.go`
- Create: `deploy/migrations/20260820_marketplace_display_order.sql`
- Create: `deploy/migrations/20260820_marketplace_display_order_test.sh`

**Interfaces:**
- Produces: `Model.DisplayOrder int`, `Vendor.DisplayOrder int`.
- Produces: `InitializeMarketplaceDisplayOrders(db *gorm.DB) error`.
- Produces: `GetModelOrderItems() ([]*Model, error)` and `GetVendorOrderItems() ([]*Vendor, error)`.
- Produces: `ReorderModels(orderedIDs []int) error` and `ReorderVendors(orderedIDs []int) error`.
- Produces sentinel errors `ErrMarketplaceOrderInvalid` and `ErrMarketplaceOrderConflict` for controller mapping.

- [ ] **Step 1: Write failing schema, backfill, append, reorder, and rollback tests**

Add table-driven tests using file-backed SQLite and `testify`:

```go
func TestInitializeMarketplaceDisplayOrdersBackfillsOnlyUnsetRows(t *testing.T) {
    db := newMarketplaceOrderTestDB(t)
    createModels(t, db,
        Model{ModelName: "older", ReleaseDate: "2026-01-01"},
        Model{ModelName: "newer", ReleaseDate: "2026-08-01"},
        Model{ModelName: "pinned", DisplayOrder: 7},
    )

    require.NoError(t, InitializeMarketplaceDisplayOrders(db))
    first := loadModelOrders(t, db)
    require.NoError(t, InitializeMarketplaceDisplayOrders(db))
    assert.Equal(t, first, loadModelOrders(t, db))
    assert.Less(t, first["newer"], first["older"])
    assert.Equal(t, 7, first["pinned"])
}

func TestReorderModelsRejectsIncompleteSetWithoutPartialWrite(t *testing.T) {
    db := newMarketplaceOrderTestDB(t)
    ids := createOrderedModels(t, db, "one", "two", "three")
    DB = db

    err := ReorderModels([]int{ids[2], ids[0]})
    require.ErrorIs(t, err, ErrMarketplaceOrderConflict)
    assert.Equal(t, ids, orderedModelIDs(t, db))
}
```

Also cover duplicate/non-positive IDs, valid complete reorder, vendor reorder, a forced update failure rollback, soft-deleted rows excluded from the required set, and sequential creates appending after the maximum order.

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
go test ./model -run 'MarketplaceDisplayOrder|ReorderModels|ReorderVendors' -count=1
```

Expected: compile or assertion failure because the fields and functions do not exist.

- [ ] **Step 3: Implement the model fields and transactional order service**

Add the fields:

```go
DisplayOrder int `json:"display_order" gorm:"not null;default:0;index"`
```

`InitializeMarketplaceDisplayOrders` must load unset rows and assign unused positive values deterministically without changing rows that already have positive values. Models use valid release date descending then model name; vendors use ID ascending. `ReorderModels` and `ReorderVendors` must:

```go
return DB.Transaction(func(tx *gorm.DB) error {
    currentIDs, err := loadAndLockCurrentIDs(tx, table)
    if err != nil { return err }
    if err := validateCompleteOrder(orderedIDs, currentIDs); err != nil { return err }
    for index, id := range orderedIDs {
        if err := tx.Model(entity).Where("id = ?", id).
            UpdateColumn("display_order", index+1).Error; err != nil {
            return err
        }
    }
    return nil
})
```

Use `lockForUpdate(tx)` for the locking query. Keep validation and database helpers in `model/marketplace_display_order.go`. Update model/vendor inserts to assign `MAX(display_order)+1` before creation and include `display_order` in complete metadata updates without allowing ordinary edit forms to reset it.

- [ ] **Step 4: Wire runtime migration and explicit PostgreSQL migration**

Call `InitializeMarketplaceDisplayOrders(DB)` after `AutoMigrate` in both normal and fast migration paths. Add an idempotent PostgreSQL migration that:

```sql
ALTER TABLE public.models ADD COLUMN IF NOT EXISTS display_order bigint NOT NULL DEFAULT 0;
ALTER TABLE public.vendors ADD COLUMN IF NOT EXISTS display_order bigint NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_models_display_order ON public.models (display_order);
CREATE INDEX IF NOT EXISTS idx_vendors_display_order ON public.vendors (display_order);
```

Backfill only `display_order <= 0`, preserve positive values, and make a second execution a no-op. The shell contract test must run the migration twice in PostgreSQL 15 and assert columns, defaults, indexes, positive order, and idempotence.

- [ ] **Step 5: Run GREEN and migration checks**

Run:

```bash
gofmt -w model/model_meta.go model/vendor_meta.go model/main.go model/marketplace_display_order.go model/marketplace_display_order_test.go
go test ./model -run 'MarketplaceDisplayOrder|ReorderModels|ReorderVendors' -count=1
sh deploy/migrations/20260820_marketplace_display_order_test.sh
```

Expected: all pass.

- [ ] **Step 6: Commit Task 1**

```bash
git add model/model_meta.go model/vendor_meta.go model/main.go model/marketplace_display_order.go model/marketplace_display_order_test.go deploy/migrations/20260820_marketplace_display_order.sql deploy/migrations/20260820_marketplace_display_order_test.sh
git commit -m "feat: persist marketplace display order"
```

---

### Task 2: Add administrator reorder API contracts

**Files:**
- Create: `controller/marketplace_order.go`
- Create: `controller/marketplace_order_test.go`
- Modify: `router/api-router.go`
- Modify: `web/src/features/models/types.ts`
- Modify: `web/src/features/models/api.ts`
- Create: `web/src/features/models/lib/__tests__/marketplace-order-api.test.ts`

**Interfaces:**
- Consumes Task 1 model functions and sentinel errors.
- Produces `GET|PUT /api/models/order` and `GET|PUT /api/vendors/order`.
- Produces frontend `getModelOrder`, `saveModelOrder`, `getVendorOrder`, and `saveVendorOrder`.

- [ ] **Step 1: Write failing route/controller contract tests**

Test real Gin routes with administrator middleware fixtures. Assert list ordering and the request contract:

```go
request := httptest.NewRequest(http.MethodPut, "/api/models/order",
    strings.NewReader(`{"ordered_ids":[3,1,2]}`))
request.Header.Set("Content-Type", "application/json")
```

Cover unauthenticated/ordinary-user rejection, valid administrator save, duplicate IDs, incomplete set conflict, empty/oversized arrays, and safe response messages without SQL details.

- [ ] **Step 2: Run controller tests and verify RED**

Run:

```bash
go test ./controller ./router -run 'MarketplaceOrder' -count=1
```

Expected: FAIL because routes and handlers are absent.

- [ ] **Step 3: Implement handlers and routes**

Use a bounded DTO:

```go
type marketplaceOrderRequest struct {
    OrderedIDs []int `json:"ordered_ids" binding:"required,max=10000,dive,gt=0"`
}
```

Return list data under the standard success envelope. Map invalid payloads to the existing safe validation response, and map `ErrMarketplaceOrderConflict` to a message instructing the administrator to refresh. Register static `/order` routes before `/:id` within both AdminAuth groups. Call `model.RefreshPricing()` only after a successful reorder transaction.

- [ ] **Step 4: Write failing frontend API contract tests**

Assert exact methods and payloads:

```ts
test('saves complete model order', async () => {
  await saveModelOrder([3, 1, 2])
  assert.equal(request.method, 'PUT')
  assert.equal(request.url, '/api/models/order')
  assert.deepEqual(request.body, { ordered_ids: [3, 1, 2] })
})
```

Run `bun test src/features/models/lib/__tests__/marketplace-order-api.test.ts` and verify RED.

- [ ] **Step 5: Add typed frontend clients and verify GREEN**

Add `display_order` to `Model` and `Vendor`, define lightweight order response types, and implement the four API functions without storing order locally.

Run:

```bash
go test ./controller ./router -run 'MarketplaceOrder' -count=1
cd web
bun test src/features/models/lib/__tests__/marketplace-order-api.test.ts
bun run typecheck
```

Expected: all pass.

- [ ] **Step 6: Commit Task 2**

```bash
git add controller/marketplace_order.go controller/marketplace_order_test.go router/api-router.go web/src/features/models/types.ts web/src/features/models/api.ts web/src/features/models/lib/__tests__/marketplace-order-api.test.ts
git commit -m "feat: add marketplace reorder APIs"
```

---

### Task 3: Publish persisted order through `/pricing`

**Files:**
- Modify: `model/pricing.go`
- Modify: `model/pricing_metadata_intersection_test.go`
- Modify: `web/src/features/pricing/types.ts`
- Modify: `web/src/features/pricing/constants.ts`
- Modify: `web/src/features/pricing/lib/filters.ts`
- Modify: `web/src/features/pricing/hooks/use-filters.ts`
- Modify: `web/src/features/pricing/index.tsx`
- Modify: `web/src/features/pricing/lib/__tests__/model-directory.test.ts`
- Modify: `web/src/features/pricing/components/__tests__/model-directory-controls.test.tsx`

**Interfaces:**
- Consumes `Model.DisplayOrder` and `Vendor.DisplayOrder`.
- Produces `Pricing.DisplayOrder` and `PricingVendor.DisplayOrder` in public JSON.
- Produces `SORT_OPTIONS.RECOMMENDED = 'recommended'` as the default directory sort.

- [ ] **Step 1: Write failing backend public-order tests**

Create published models whose release dates intentionally conflict with display order and assert persisted order wins:

```go
assert.Equal(t,
    []string{"manual-first", "manual-second"},
    pricingModelNames(GetPricing()),
)
assert.Equal(t,
    []string{"Vendor B", "Vendor A"},
    pricingVendorNames(GetVendors()),
)
```

Run `go test ./model -run 'Pricing.*DisplayOrder' -count=1` and verify RED.

- [ ] **Step 2: Sort the pricing cache and expose order fields**

Copy metadata order into each `Pricing` record, sort `pricingMap` after construction using `DisplayOrder ASC` then `ModelName ASC`, and build/sort `vendorsList` using vendor display order then name. Keep release date in the DTO for display and homepage consumers.

- [ ] **Step 3: Write failing frontend default-sort tests**

Replace the release-default assertions with:

```ts
test('recommended keeps backend display order', () => {
  const models = [model('manual-first'), model('manual-second')]
  assert.deepEqual(
    sortModels(models, SORT_OPTIONS.RECOMMENDED).map((item) => item.model_name),
    ['manual-first', 'manual-second']
  )
})
```

Assert the toolbar contains Recommended, Name, Price low/high and omits Newest. Run the focused tests and verify RED.

- [ ] **Step 4: Replace the `/pricing` release default**

Add `RECOMMENDED`, remove `RELEASE_DATE` from public sort labels, make `useFilters` default to recommended, and make `sortModels` return the input copy unchanged for recommended. Keep `sortModelsByReleaseDate` for homepage/related-model use. Update URL serialization so recommended is omitted like the old default.

- [ ] **Step 5: Run backend and frontend GREEN checks**

```bash
go test ./model -run 'Pricing.*DisplayOrder|PricingMetadata' -count=1
cd web
bun test src/features/pricing/lib/__tests__/model-directory.test.ts src/features/pricing/components/__tests__/model-directory-controls.test.tsx
bun run typecheck
```

- [ ] **Step 6: Commit Task 3**

```bash
git add model/pricing.go model/pricing_metadata_intersection_test.go web/src/features/pricing/types.ts web/src/features/pricing/constants.ts web/src/features/pricing/lib/filters.ts web/src/features/pricing/hooks/use-filters.ts web/src/features/pricing/index.tsx web/src/features/pricing/lib/__tests__/model-directory.test.ts web/src/features/pricing/components/__tests__/model-directory-controls.test.tsx
git commit -m "feat: use managed marketplace order"
```

---

### Task 4: Add model metadata reorder mode

**Files:**
- Create: `web/src/features/models/components/model-order-editor.tsx`
- Create: `web/src/features/models/components/__tests__/model-order-editor.test.tsx`
- Modify: `web/src/features/models/components/models-primary-buttons.tsx`
- Modify: `web/src/features/models/components/models-provider.tsx`
- Modify: `web/src/features/models/components/models-table.tsx`
- Modify: `web/src/features/models/index.tsx`

**Interfaces:**
- Consumes Task 2 model order APIs.
- Produces `isOrderingModels`, `startModelOrdering`, and `stopModelOrdering` in model context.
- Produces `ModelOrderEditor` with `onSaved` and `onCancel` callbacks.

- [ ] **Step 1: Write failing user-behavior tests**

Render the models page/provider and assert:

```ts
test('edit order enables row handles and hides normal pagination', async () => {
  await user.click(screen.getByRole('button', { name: 'Edit order' }))
  expect(screen.getByRole('button', { name: 'Drag model-a to reorder' })).toBeEnabled()
  expect(screen.queryByText('Page 1 of')).not.toBeInTheDocument()
})
```

Cover loading/error/empty states, pointer reorder callback, ArrowUp/ArrowDown, full ID save, cancel without request, save failure retaining draft, and normal mode without handles.

- [ ] **Step 2: Run focused tests and verify RED**

```bash
cd web
bun test src/features/models/components/__tests__/model-order-editor.test.tsx
```

Expected: FAIL because the editor and state are absent.

- [ ] **Step 3: Implement the accessible reorder editor**

Use the established pattern:

```tsx
<Reorder.Group axis='y' values={items} onReorder={setItems} as='div'>
  {items.map((item, index) => (
    <ModelOrderRow
      key={item.id}
      item={item}
      index={index}
      count={items.length}
      onMove={moveItem}
    />
  ))}
</Reorder.Group>
```

Each handle uses `useDragControls`, `touch-none`, an accessible label, and ArrowUp/ArrowDown keyboard handling. Show model name, vendor, and status. Save calls `saveModelOrder(items.map(item => item.id))`, then invalidates model and pricing queries and exits; failure shows a localized error without discarding items.

- [ ] **Step 4: Wire mode into the metadata page**

Add a visible “Edit order” button beside “Add Model”. When active, replace `ModelsTable` with `ModelOrderEditor`, replace actions with Save/Cancel owned by the editor, and ensure closing/navigating clears draft state. The normal table remains unchanged.

- [ ] **Step 5: Run GREEN, type, and scoped lint checks**

```bash
cd web
bun test src/features/models/components/__tests__/model-order-editor.test.tsx
bun run typecheck
bunx oxlint -c .oxlintrc.json \
  src/features/models/components/model-order-editor.tsx \
  src/features/models/components/models-primary-buttons.tsx \
  src/features/models/components/models-provider.tsx \
  src/features/models/components/models-table.tsx \
  src/features/models/index.tsx \
  src/features/models/components/__tests__/model-order-editor.test.tsx
```

- [ ] **Step 6: Commit Task 4**

```bash
git add web/src/features/models/components/model-order-editor.tsx web/src/features/models/components/__tests__/model-order-editor.test.tsx web/src/features/models/components/models-primary-buttons.tsx web/src/features/models/components/models-provider.tsx web/src/features/models/components/models-table.tsx web/src/features/models/index.tsx
git commit -m "feat: reorder model marketplace metadata"
```

---

### Task 5: Add vendor reorder mode

**Files:**
- Modify: `web/src/features/models/components/dialogs/vendor-management-dialog.tsx`
- Modify: `web/src/features/models/components/dialogs/vendor-management.tsx`
- Create: `web/src/features/models/components/dialogs/__tests__/vendor-ordering.test.tsx`

**Interfaces:**
- Consumes Task 2 vendor order APIs.
- Keeps existing create/edit/delete workflow unchanged outside ordering mode.

- [ ] **Step 1: Write failing vendor ordering tests**

Cover activation, accessible handles, pointer and keyboard movement, save/cancel, error retention, query invalidation, disabled create/edit/delete while ordering, and close/reopen draft reset:

```ts
test('closing and reopening discards unsaved vendor order', async () => {
  await enterOrderModeAndMoveSecondVendorFirst()
  await closeAndReopenVendorManager()
  expect(visibleVendorNames()).toEqual(['Vendor A', 'Vendor B'])
})
```

- [ ] **Step 2: Run focused tests and verify RED**

```bash
cd web
bun test src/features/models/components/dialogs/__tests__/vendor-ordering.test.tsx
```

- [ ] **Step 3: Implement vendor order mode**

Add local `mode: 'list' | 'order'`, initialize the order draft only when entering order mode, and render `Reorder.Group` rows with the existing Logo preview, name, and icon value. Save the complete ID sequence, invalidate vendor/model/pricing queries, and return to the list. Cancel or Dialog close discards the draft. Do not send create/update/delete calls during ordering.

- [ ] **Step 4: Run GREEN and existing vendor regressions**

```bash
cd web
bun test src/features/models/components/dialogs/__tests__
bun run typecheck
bunx oxlint -c .oxlintrc.json \
  src/features/models/components/dialogs/vendor-management-dialog.tsx \
  src/features/models/components/dialogs/vendor-management.tsx \
  src/features/models/components/dialogs/__tests__/vendor-ordering.test.tsx
```

- [ ] **Step 5: Commit Task 5**

```bash
git add web/src/features/models/components/dialogs/vendor-management-dialog.tsx web/src/features/models/components/dialogs/vendor-management.tsx web/src/features/models/components/dialogs/__tests__/vendor-ordering.test.tsx
git commit -m "feat: reorder marketplace vendors"
```

---

### Task 6: Localize, integrate, review, and archive

**Files:**
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `.ccg/tasks/manual-marketplace-ordering/task.json`
- Create: `.ccg/tasks/manual-marketplace-ordering/review.md`
- Move on completion: `.ccg/tasks/manual-marketplace-ordering/` to `.ccg/tasks/archive/2026-08/`

**Interfaces:**
- Consumes all prior tasks.
- Produces a fully localized, verified, archived feature branch.

- [ ] **Step 1: Add and verify all translations**

Add keys for Recommended, Edit order, Save order, Cancel ordering, drag/move labels, load/save errors, refresh conflict, and explanatory copy in all seven locale files. Keep English source keys identical across files.

Run:

```bash
cd web
bun run i18n:check
```

- [ ] **Step 2: Run complete relevant backend verification**

```bash
gofmt -w \
  model/model_meta.go \
  model/vendor_meta.go \
  model/main.go \
  model/marketplace_display_order.go \
  model/marketplace_display_order_test.go \
  model/pricing.go \
  model/pricing_metadata_intersection_test.go \
  controller/marketplace_order.go \
  controller/marketplace_order_test.go \
  router/api-router.go
go test ./model -count=1
go test ./controller -count=1
go test ./router -count=1
go test ./... -count=1
go vet ./...
go build ./...
sh deploy/migrations/20260820_marketplace_display_order_test.sh
```

- [ ] **Step 3: Run complete relevant frontend verification**

```bash
cd web
bun test src/features/models/components/__tests__ src/features/models/components/dialogs/__tests__ src/features/models/lib/__tests__ src/features/pricing/lib/__tests__/model-directory.test.ts src/features/pricing/components/__tests__/model-directory-controls.test.tsx
bun run typecheck
bun run i18n:check
bun run build
```

Run scoped `oxlint` and `oxfmt --check` over every changed TS/TSX file, then return to the repository root and run `git diff --check`.

- [ ] **Step 4: Perform dual-model review or record tool unavailability**

Attempt both required review backends in parallel using the repository `AGENTS.md` template. If `~/.claude/bin/codeagent-wrapper` is absent, record both failed invocations in `review.md`, then perform a local review covering correctness, cross-database behavior, API authorization, concurrency, accessibility, i18n, caching, and scope. Any Critical finding must be fixed and all affected checks rerun.

- [ ] **Step 5: Update CCG task, archive, and commit**

Set task status to completed, summarize tests and review, then move the directory:

```bash
mkdir -p .ccg/tasks/archive/2026-08
mv .ccg/tasks/manual-marketplace-ordering .ccg/tasks/archive/2026-08/
git add .ccg/tasks web/src/i18n/locales
git commit -m "chore: archive ccg task manual-marketplace-ordering"
```

- [ ] **Step 6: Final branch audit**

```bash
git status --short --branch
git log --oneline --decorate -8
git diff origin/develop...HEAD --stat
git diff origin/develop...HEAD --check
```

Expected: no uncommitted changes, only approved feature commits, no secrets, no unrelated deployment/price/billing changes.
