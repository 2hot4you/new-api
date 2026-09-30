# Unified Model Pricing Currency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Interpret every expression price in its model's source currency, settle it as USD-equivalent quota, and display all user-facing prices in the site's configured currency.

**Architecture:** A new pure Go money package owns USD/CNY validation and source-to-USD normalization. Every billing path freezes that money context into its request snapshot before quota calculation; the model-pricing API persists expression and source currency atomically. The frontend converts source amounts to USD before using the existing site-currency formatter, while the administrator editor stores expression coefficients exactly in the selected source currency.

**Tech Stack:** Go 1.24, GORM, Gin, React 19, TypeScript, Zustand, TanStack Query, Vitest, Testing Library

**Spec:** `docs/superpowers/specs/2026-09-30-unified-model-pricing-currency-design.md`

## Global Constraints

- `models.billing_currency` means source pricing currency, not display currency.
- Supported source currencies are exactly `USD` and `CNY`.
- Internal quota remains USD-equivalent; CNY source cost is divided by the frozen `usd_exchange_rate` before quota calculation.
- Group ratio is applied after source-to-USD normalization.
- Current display settings are CNY with `1 USD = 7 CNY`; recharge `Price=7` remains an independent setting.
- Missing metadata defaults to USD only for reading and legacy billing; a model-pricing write requires an exact metadata row.
- Invalid CNY exchange rates fail closed and never fall back to 1.
- Legacy ratio and fixed-price modes retain their existing USD semantics.
- Do not call antigravity or Claude external executors.
- Do not modify or stage the four untracked `* 2.tsx` files already present in the worktree.
- Implement test-first, keep commits scoped by task, and do not push until the full verification gate passes.

## Review Focus

- A mapped request must resolve currency from the final billing model, not the requested or upstream display model; Task 2 and Task 3 add mapped-model tests.
- A rate change between pre-consume and stream/task completion must not alter settlement; Task 3 adds frozen-snapshot tests.
- Switching an existing expression from USD to CNY must keep its numeric coefficients unchanged and visibly change only their interpretation; Task 6 pins this behavior.
- A legacy ratio model marked CNY must still bill as legacy USD; Task 3 and Task 6 add regression tests.
- Historical Grok v1 snapshots must remain readable while new v2 snapshots distinguish source cost from USD cost; Task 4 and Task 8 add compatibility tests.

---

### Task 1: Pure source-currency normalization

**Files:**
- Create: `pkg/billingmoney/money.go`
- Create: `pkg/billingmoney/money_test.go`

**Interfaces:**
- Consumes: no project state; callers supply source currency and the frozen CNY-per-USD rate.
- Produces: `billingmoney.Currency`, `billingmoney.Context`, `billingmoney.Amounts`, `billingmoney.NewContext(source string, cnyPerUSD float64) (Context, error)`, and `Context.Normalize(sourceCost float64) (Amounts, error)`.

- [ ] **Step 1: Write failing table tests for USD, CNY, zero cost, invalid currency, invalid rate, NaN, infinity, and negative cost**

  Assert `$5 -> $5`, `¥2 at 7 -> $2/7`, USD accepts an otherwise unused positive rate, and every invalid CNY context or amount returns an error.

- [ ] **Step 2: Run the focused test and verify it fails because `pkg/billingmoney` does not exist**

  Run: `go test ./pkg/billingmoney -run Test -count=1`

- [ ] **Step 3: Implement the minimal immutable value types and normalization methods**

  Keep validation pure; this package must not import model, settings, Gin, or GORM.

- [ ] **Step 4: Run the focused tests and race detector**

  Run: `go test -race ./pkg/billingmoney -count=1`

- [ ] **Step 5: Commit**

  ```bash
  git add pkg/billingmoney
  git commit -m "feat: add billing currency normalization"
  ```

### Task 2: Resolve model money context from exact metadata

**Files:**
- Create: `model/model_billing_money.go`
- Create: `model/model_billing_money_test.go`
- Modify: `model/model_meta.go`

**Interfaces:**
- Consumes: `billingmoney.NewContext`, `operation_setting.USDExchangeRate`, exact `models.model_name` records.
- Produces: `ResolveBillingMoneyContext(db *gorm.DB, modelName string) (ctx billingmoney.Context, hasMetadata bool, err error)` and `LoadModelBillingCurrencies(db *gorm.DB, modelNames []string, forUpdate bool) (map[string]ModelBillingCurrency, error)`.

- [ ] **Step 1: Write failing SQLite and PostgreSQL-compatible resolver tests**

  Cover exact USD, exact CNY at rate 7, missing metadata defaulting to USD with `hasMetadata=false`, illegal stored currency failing closed, and prefix metadata not matching a concrete billing model.

- [ ] **Step 2: Run the resolver tests and verify the missing functions fail compilation**

  Run: `go test ./model -run 'TestResolveBillingMoneyContext|TestLoadModelBillingCurrencies' -count=1`

- [ ] **Step 3: Implement exact-row loading and validation without changing marketplace wildcard resolution**

  `forUpdate=true` must use the existing row-lock helper and preserve deterministic model-name ordering.

- [ ] **Step 4: Run model tests**

  Run: `go test ./model -run 'BillingMoney|BillingCurrency|CatalogMetadata' -count=1`

- [ ] **Step 5: Commit**

  ```bash
  git add model/model_billing_money.go model/model_billing_money_test.go model/model_meta.go
  git commit -m "feat: resolve model billing currency context"
  ```

### Task 3: Apply frozen currency context to expression billing

**Files:**
- Modify: `pkg/billingexpr/types.go`
- Modify: `pkg/billingexpr/settle.go`
- Modify: `pkg/billingexpr/settle_test.go`
- Modify: `relay/helper/price.go`
- Modify: `relay/helper/price_test.go`
- Modify: `relay/relay_task.go`
- Modify: `relay/relay_task_test.go`
- Modify: `service/tiered_settle.go`
- Modify: `service/tiered_settle_test.go`
- Modify: `service/log_info_generate.go`
- Modify: `service/task_billing.go`
- Modify: `service/task_billing_test.go`

**Interfaces:**
- Consumes: `ResolveBillingMoneyContext` and `billingmoney.Context.Normalize` from Tasks 1-2.
- Produces: flat snapshot fields `source_currency`, `cny_per_usd`, `estimated_source_cost`, `estimated_cost_usd`; result fields `actual_source_cost`, `actual_cost_usd`; log fields `source_currency`, `source_cost`, `cny_per_usd`, `cost_usd`, `display_currency`, `display_cost`; `billingexpr.ExpressionAmounts(exprOutput float64, snap *BillingSnapshot) (billingmoney.Amounts, error)`.

- [ ] **Step 1: Write failing expression settlement tests for all billing units**

  Assert token `p*2` with one million tokens and CNY/7 yields source cost 2 and USD cost `2/7`; task expression `u("tokens")*46/1000000` yields the same unit semantics; fixed request pricing normalizes once; USD snapshots retain current quotas.

- [ ] **Step 2: Add failing integration tests for frozen rate, mapped billing model, group ordering, failed settlement fallback, and legacy ratio isolation**

  Freeze rate 7, mutate the global rate to 8 before settlement, and assert the request still settles at 7. Assert a requested alias billed as a CNY target uses the target currency. Assert a legacy ratio model with CNY metadata keeps its old USD quota.

- [ ] **Step 3: Implement source-cost extraction and snapshot-based normalization in pre-consume and settlement**

  Replace direct `$ / 1M` quota formulas in both token and task expression paths. Auto-group refresh may change only `GroupRatio` and derived quota, never currency or rate.

- [ ] **Step 4: Publish expression money audit fields through shared log injection**

  On successful settlement use actual amounts; on evaluation fallback use estimated amounts. Preserve the existing meaning of `quota` and `fixed_price`.

- [ ] **Step 5: Run focused backend tests**

  Run: `go test ./pkg/billingexpr ./relay/helper ./relay ./service -run 'Tiered|BillingMoney|TaskExpression|Mapped' -count=1`

- [ ] **Step 6: Commit**

  ```bash
  git add pkg/billingexpr relay/helper/price.go relay/helper/price_test.go relay/relay_task.go relay/relay_task_test.go service/tiered_settle.go service/tiered_settle_test.go service/log_info_generate.go service/task_billing.go service/task_billing_test.go
  git commit -m "feat: settle expressions in source currency"
  ```

### Task 4: Normalize Seedance and Grok specialized billing

**Files:**
- Modify: `setting/ratio_setting/starai_video_price.go`
- Modify: `setting/ratio_setting/starai_video_price_test.go`
- Modify: `setting/ratio_setting/molii_grok_price.go`
- Modify: `setting/ratio_setting/model_ratio.go`
- Modify: `relay/common/relay_info.go`
- Modify: `relay/channel/moliigrok/adaptor.go`
- Modify: `relay/channel/moliigrok/adaptor_test.go`
- Modify: `relay/channel/task/moliigrok/adaptor.go`
- Modify: `relay/channel/task/moliigrok/adaptor_test.go`
- Modify: `service/text_quota.go`
- Modify: `service/grok_image_billing_test.go`
- Modify: `service/task_billing.go`
- Modify: `service/task_billing_test.go`

**Interfaces:**
- Consumes: the shared money resolver and normalizer; expression snapshots from Task 3.
- Produces: Grok image/video snapshot version 2 with `source_currency`, `cny_per_usd`, `subtotal`, `cost_usd`, `final_source_cost`, and `final_cost_usd`; v1 remains readable.

- [ ] **Step 1: Write failing Seedance tests proving the generated CNY task expression charges `¥46/7` USD-equivalent per million tokens**

  Keep the editable matrix and generated expression coefficients unchanged; only quota normalization changes.

- [ ] **Step 2: Write failing Grok image and video tests for CNY normalization and frozen async rate**

  A `¥0.02` image at rate 7 must reserve `$0.02/7` equivalent before group ratio. A video submitted at rate 7 and completed after a global rate change must still use 7.

- [ ] **Step 3: Add historical v1 compatibility tests**

  Existing serialized v1 snapshots must parse and display their legacy final cost without being normalized a second time; new requests emit only v2.

- [ ] **Step 4: Replace the 1:1 anchor convention with shared normalization**

  Preserve ModelPrice anchors only as mechanical ratio anchors where required; calculate every new direct-cost ratio from `cost_usd`, and update misleading 1:1 comments.

- [ ] **Step 5: Run specialized billing tests**

  Run: `go test ./setting/ratio_setting ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./service -run 'StarAI|Seedance|Grok' -count=1`

- [ ] **Step 6: Commit**

  ```bash
  git add setting/ratio_setting relay/common/relay_info.go relay/channel/moliigrok relay/channel/task/moliigrok service/text_quota.go service/grok_image_billing_test.go service/task_billing.go service/task_billing_test.go
  git commit -m "fix: normalize specialized CNY billing"
  ```

### Task 5: Persist source currency with model pricing atomically

**Files:**
- Modify: `model/model_pricing_config.go`
- Modify: `model/model_pricing_config_test.go`
- Modify: `controller/model_pricing_config.go`
- Modify: `controller/model_pricing_config_test.go`
- Modify: `model/model_pricing_seedance_test.go`

**Interfaces:**
- Consumes: `LoadModelBillingCurrencies(..., forUpdate=true)` from Task 2.
- Produces: `ModelPricingEntry.BillingCurrency`, `ModelPricingEntry.HasMetadata`, `ModelPricingChange.BillingCurrency`, and a version hash over `{pricing,billing_currency}`.

- [ ] **Step 1: Write failing snapshot tests for returned currency, metadata presence, and version changes caused only by currency**

  Missing metadata reads as USD plus `has_metadata=false`; existing CNY metadata reads as CNY; changing only the stored currency must invalidate the previous version.

- [ ] **Step 2: Write failing transaction tests for joint save, rollback, missing metadata, invalid currency, conflict, and reset**

  Assert Option rows and `models.billing_currency` commit together. Inject either-side failure and assert neither changes. Reset removes custom pricing but preserves the explicitly submitted currency.

- [ ] **Step 3: Extend the API DTO and transaction callback**

  Lock exact metadata rows in sorted model-name order after pricing Option locks, validate all drafts before any update, then write both resources in one transaction.

- [ ] **Step 4: Make Seedance matrix publication set all four exact model rows to CNY in the same transaction**

  Missing Seedance metadata must abort publication rather than create incomplete metadata automatically.

- [ ] **Step 5: Run model and controller tests**

  Run: `go test ./model ./controller -run 'ModelPricing|StarAIVideo|Seedance' -count=1`

- [ ] **Step 6: Commit**

  ```bash
  git add model/model_pricing_config.go model/model_pricing_config_test.go model/model_pricing_seedance_test.go controller/model_pricing_config.go controller/model_pricing_config_test.go
  git commit -m "feat: save model pricing currency atomically"
  ```

### Task 6: Make the administrator editor source-currency aware

**Files:**
- Modify: `web/src/features/model-pricing/api.ts`
- Modify: `web/src/features/model-pricing/currency.ts`
- Create: `web/src/features/model-pricing/source-currency-selector.tsx`
- Delete: `web/src/features/model-pricing/pricing-currency-selector.tsx`
- Modify: `web/src/features/model-pricing/model-pricing-panel.tsx`
- Modify: `web/src/features/system-settings/models/model-pricing-sheet.tsx`
- Delete: `web/src/stores/pricing-preferences-store.ts`
- Modify: `web/src/features/model-pricing/__tests__/editor-currency.test.tsx`
- Modify: `web/src/features/model-pricing/__tests__/save-errors.test.tsx`
- Modify: `web/src/features/model-pricing/__tests__/editor-layout.test.tsx`
- Modify: `web/src/features/system-settings/models/__tests__/plugin-pricing.test.tsx`
- Modify: `web/src/i18n/locales/*.json`

**Interfaces:**
- Consumes: Task 5 API fields and current site exchange-rate config.
- Produces: a controlled source-currency selector and save payload containing `billing_currency`; no browser-local pricing-currency state.

- [ ] **Step 1: Rewrite failing editor tests around source-currency semantics**

  Assert API-loaded CNY is selected, changing USD to CNY marks the draft dirty, raw expression coefficients remain numerically unchanged, source inputs use the matching symbol, and the CNY platform preview converts USD but leaves CNY unchanged.

- [ ] **Step 2: Add failing tests for legacy mode and missing metadata**

  Legacy mode labels billing as USD compatibility behavior and does not reinterpret ratios. A missing exact metadata row disables save with a link/instruction to create metadata.

- [ ] **Step 3: Extend TypeScript API types and pricing-change construction**

  Include currency-only changes in `buildPricingChanges`; keep the server version as the optimistic concurrency token.

- [ ] **Step 4: Replace the persisted display-preference selector with the controlled source-currency selector**

  Do not numerically rewrite an existing expression when the selection changes. Remove the now-unused Zustand store and its localStorage key.

- [ ] **Step 5: Add source and preview labels, help copy, and translations**

  Use “Source pricing currency” / “原始计价币种” and explicitly state that user display follows the site currency.

- [ ] **Step 6: Run focused UI tests, typecheck, and i18n validation**

  Run: `cd web && pnpm test -- src/features/model-pricing src/features/system-settings/models/__tests__/plugin-pricing.test.tsx && pnpm typecheck && pnpm i18n:check`

- [ ] **Step 7: Commit**

  ```bash
  git add web/src/features/model-pricing web/src/features/system-settings/models web/src/stores/pricing-preferences-store.ts web/src/i18n/locales
  git commit -m "feat: edit source currency with model pricing"
  ```

### Task 7: Convert all public pricing to site currency

**Files:**
- Modify: `web/src/lib/currency.ts`
- Modify: `web/src/features/pricing/lib/price.ts`
- Modify: `web/src/features/pricing/lib/dynamic-price.ts`
- Modify: `web/src/features/pricing/components/dynamic-pricing-breakdown.tsx`
- Modify: `web/src/features/pricing/components/model-price-cell.tsx`
- Modify: `web/src/features/pricing/components/model-details.tsx`
- Modify: `web/src/features/pricing/lib/__tests__/dynamic-cny-pricing.test.ts`
- Modify: `web/src/features/pricing/components/__tests__/model-price-cell-currency.test.tsx`
- Modify: `web/src/features/pricing/components/__tests__/model-card-text-pricing.test.tsx`
- Modify: `web/src/features/pricing/components/__tests__/model-card-seedance-pricing.test.tsx`
- Modify: `web/src/features/pricing/__tests__/task-price-display.test.tsx`
- Modify: `web/src/features/users/components/__tests__/quota-display.test.tsx`

**Interfaces:**
- Consumes: model `billing_currency`, current `usdExchangeRate`, current site display config.
- Produces: `sourceAmountToUSD(amount, sourceCurrency, cnyPerUSD)`, `formatSourceBillingAmount(amount, sourceCurrency, options)`, and pricing components that always label the site display currency.

- [ ] **Step 1: Write failing currency-helper tests**

  With CNY display and rate 7, assert source `$5 -> ¥35` and source `¥2 -> ¥2`; with USD display assert source `¥2 -> $2/7`; invalid rates return `-` instead of silently misformatting.

- [ ] **Step 2: Update public pricing tests to require CNY output for both USD and CNY models**

  Cover fixed token, dynamic expression, task usage, Seedance matrix, Grok direct prices, model cards and detail drawers. No user-facing caption may choose its currency label from `billing_currency`.

  Add a balance regression proving raw quota is still converted from internal USD through `formatQuotaWithCurrency`; model source currency must never be applied to an account balance.

- [ ] **Step 3: Implement the shared frontend source-to-display pipeline**

  Normalize source amount to USD exactly once, then call `formatBillingCurrencyFromUSD`. Preserve the explicit recharge-price view by applying `Price` only after normalization.

- [ ] **Step 4: Replace every direct `formatCatalogCurrencyAmount` pricing call**

  Keep `formatCatalogCurrencyAmount` only for administrator source-value/audit contexts; public pricing must use the new helper.

- [ ] **Step 5: Run pricing tests and typecheck**

  Run: `cd web && pnpm test -- src/features/pricing src/lib && pnpm typecheck`

- [ ] **Step 6: Commit**

  ```bash
  git add web/src/lib/currency.ts web/src/features/pricing web/src/features/users/components/__tests__/quota-display.test.tsx
  git commit -m "fix: display model prices in site currency"
  ```

### Task 8: Expose auditable amounts in logs and quotation tools

**Files:**
- Modify: `web/src/features/usage-logs/types.ts`
- Modify: `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- Modify: `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`
- Modify: `web/src/features/usage-logs/components/__tests__/usage-facts.test.tsx`
- Modify: `web/src/features/usage-logs/components/__tests__/detail-preview.test.tsx`
- Modify: `web/src/features/product-quotation/lib/quotation-math.ts`
- Modify: `web/src/features/product-quotation/lib/__tests__/quotation-math.test.ts`

**Interfaces:**
- Consumes: Task 3-4 log fields and Task 7 formatting helpers.
- Produces: user-visible site-currency totals plus administrator-only rows for source currency, source cost, frozen rate and USD-equivalent cost.

- [ ] **Step 1: Write failing usage-log tests for user/admin separation**

  A user sees only `¥` totals. An administrator sees the CNY or USD source amount, `1 USD = 7 CNY`, USD cost and CNY display cost. Dynamic tier rows use the frozen log currency when available instead of current model metadata.

- [ ] **Step 2: Add failing Grok v1/v2 rendering tests**

  V2 uses explicit source and USD fields; v1 renders its legacy record without a second conversion and is labeled historical where an exact source interpretation is unavailable.

- [ ] **Step 3: Update log types and rendering**

  Prefer the immutable log fields; use current pricing metadata only for old logs that lack a snapshot, and mark that fallback in admin detail.

- [ ] **Step 4: Normalize quotation dimensions to the selected/site quote currency before aggregation**

  Preserve each dimension's source currency for audit, but never add raw USD and CNY numbers together. Add rate as an explicit quotation input/snapshot field.

- [ ] **Step 5: Run usage-log and quotation tests**

  Run: `cd web && pnpm test -- src/features/usage-logs src/features/product-quotation && pnpm typecheck`

- [ ] **Step 6: Commit**

  ```bash
  git add web/src/features/usage-logs web/src/features/product-quotation
  git commit -m "feat: audit converted billing amounts"
  ```

### Task 9: Migrate confirmed CNY models and perform the release gate

**Files:**
- Modify: `model/model_billing_currency_migration.go`
- Modify: `model/model_billing_currency_migration_test.go`
- Create: `docs/billing/source-currency-audit.md`
- Create: `.ccg/tasks/unify-model-pricing-currency/review.md`

**Interfaces:**
- Consumes: all previous task interfaces.
- Produces: one-time v2 migration for explicitly confirmed CNY models and a checked audit table for every existing CNY metadata model.

- [ ] **Step 1: Write the failing one-time migration test**

  Add only confirmed source-CNY identifiers, including `deepseek-flash` and `deepseek-v4-pro-202606`; assert the marker prevents a later administrator edit from being overwritten.

- [ ] **Step 2: Implement the v2 migration and write the source-currency audit**

  The audit must list model, pricing mode, source, expression/matrix unit, expected CNY result and regression test. Do not infer CNY merely from vendor country.

- [ ] **Step 3: Run the complete backend verification**

  Run: `go test ./... -count=1`

- [ ] **Step 4: Run the complete frontend verification**

  Run: `cd web && pnpm test && pnpm typecheck && pnpm lint && pnpm format:check && pnpm i18n:check && pnpm build`

- [ ] **Step 5: Check scope and secrets**

  Run: `git diff --check && git status --short && git diff --stat origin/develop...HEAD`

  Expected: only planned files plus CCG/design/plan records; none of the four untracked `* 2.tsx` files staged; no credentials or environment secrets.

- [ ] **Step 6: Perform dev acceptance at rate 7**

  Verify `/pricing?search=deepseek` shows Flash and Pro in CNY; execute one controlled USD expression and one controlled CNY expression; reconcile displayed price, log audit fields, quota delta and database balance against the spec examples.

- [ ] **Step 7: Record review evidence and commit**

  ```bash
  git add model/model_billing_currency_migration.go model/model_billing_currency_migration_test.go docs/billing/source-currency-audit.md .ccg/tasks/unify-model-pricing-currency/review.md
  git commit -m "chore: audit model source currencies"
  ```

- [ ] **Step 8: Archive the CCG task only after dev acceptance passes**

  Move `.ccg/tasks/unify-model-pricing-currency` to the current monthly archive, commit the archive, then push the verified branch to remote `develop` to trigger CI/CD. Do not deploy if any CNY model remains unaudited or any acceptance amount differs.
