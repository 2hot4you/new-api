# Product Quotation Calculator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a root-only product quotation workspace that reuses live pricing data, calculates discounts and group ratios, persists a browser draft, previews a complete commercial quote, and exports the same snapshot as safe offline HTML.

**Architecture:** A frontend-only `product-quotation` feature consumes `usePricingData`, normalizes every supported pricing shape into typed quote dimensions, and keeps calculation separate from formatting. A versioned localStorage adapter owns draft persistence. React preview and standalone HTML export both consume one immutable quotation snapshot; the authenticated route and sidebar enforce root-only UI access without changing the public pricing API.

**Tech Stack:** React 19, TypeScript, TanStack Router/Query, React Hook Form/Zod conventions, Tailwind CSS, Bun test runner, existing new-api pricing parsers and UI primitives.

**Spec:** `docs/superpowers/specs/2026-09-08-product-quotation-calculator-design.md`

## Global Constraints

- Do not modify actual billing settings, model prices, group ratios, or database state.
- Do not add a backend endpoint unless the existing public pricing data proves insufficient; any new endpoint must use `RootAuth()`.
- Provider identity is `vendor_id -> vendors[].id`; model identity is `model_name`.
- Never parse displayed currency strings back into numbers and never apply group ratio or quotation discount more than once.
- Preserve every supported pricing dimension and unit; unparseable dynamic pricing is conditional/pending, never zero.
- All customer-facing copy uses i18n; all exported text is context-safe escaped.
- Keep the existing public `/api/pricing` and `/pricing` behavior unchanged.
- Do not deploy to production.

---

### Task 1: Canonical quotation pricing and calculator domain

**Files:**
- Create: `web/src/features/product-quotation/types.ts`
- Create: `web/src/features/product-quotation/lib/quotation-math.ts`
- Create: `web/src/features/product-quotation/lib/quotation-format.ts`
- Create: `web/src/features/product-quotation/lib/__tests__/quotation-math.test.ts`
- Create: `web/src/features/product-quotation/lib/__tests__/quotation-format.test.ts`
- Modify: `web/src/features/pricing/types.ts`
- Modify: `web/src/features/pricing/hooks/use-pricing-data.ts`

**Interfaces:**
- Produces `QuotationDraft`, `QuoteProviderSection`, `QuoteModelSection`, `QuotePriceDimension`, `QuotationSnapshot`, and `QuotationValidation` types.
- Produces `normalizeDiscount(discountInZhe): number | null`, `resolveEffectiveDiscount(globalDiscount, providerDiscount): number | null`, `calculateSuggestedGroupRatio(actualRate, discountInZhe, fixedRate): number | null`, `buildQuotationSnapshot(input): QuotationSnapshot`, `validateQuotation(snapshot): QuotationValidation`, `formatQuoteAmount(amount, currency): string`, and `sanitizeQuotationFilename(title, date): string`.
- Consumes existing `PricingModel`, `PricingVendor`, dynamic billing expression, task usage, video, Grok, and group-ratio helpers.

- [ ] Write failing domain tests with literal expectations for 8折/5折, provider override precedence, `6.8 × 0.5 ÷ 7 = 0.485714...`, invalid NaN/zero/negative rates, fixed token dimensions, request pricing, explicit zero price, tiny non-zero values, direct CNY, all video/Grok rows, every dynamic tier, task usage units, unparseable dynamic fallback, and unavailable selected-group models.
- [ ] Run the two new test files and confirm failures are caused by missing production modules.
- [ ] Implement typed normalization. Use an explicit basis ratio of `1` for raw pricing or `group_ratio[group]` for a selected group; do not call `getDisplayGroupRatio` without a group. Keep `sourceAmount`, `quoteAmount`, `currency`, `unit`, `condition`, and `status` separate.
- [ ] Add top-level `pricing_version?: string` to `PricingData` and expose no unrelated API changes.
- [ ] Expose React Query `dataUpdatedAt` and the response `pricing_version` from `usePricingData` so refresh timestamps advance only after successful data retrieval.
- [ ] Implement precision formatting with up to eight meaningful fractional digits for tiny values and a filename sanitizer that removes controls and `/\\:*?\"<>|`, trims trailing dots/spaces, limits length, and falls back to `quotation`.
- [ ] Run the new tests, existing pricing library tests, and `bun run typecheck`; refactor only after green.
- [ ] Commit the domain change.

### Task 2: Versioned draft persistence and safe standalone HTML

**Files:**
- Create: `web/src/features/product-quotation/lib/draft-storage.ts`
- Create: `web/src/features/product-quotation/lib/quotation-html.ts`
- Create: `web/src/features/product-quotation/lib/download-html.ts`
- Create: `web/src/features/product-quotation/lib/__tests__/draft-storage.test.ts`
- Create: `web/src/features/product-quotation/lib/__tests__/quotation-html.test.ts`

**Interfaces:**
- Consumes Task 1 `QuotationDraft` and `QuotationSnapshot`.
- Produces `createDefaultQuotationDraft(today): QuotationDraft`, `loadQuotationDraft(storage): QuotationDraft | null`, `saveQuotationDraft(storage, draft): boolean`, `clearQuotationDraft(storage): void`, `escapeHtml(value): string`, `buildQuotationHtml(snapshot): string`, and `downloadQuotationHtml(snapshot): void`.

- [ ] Write failing tests for valid restore, corrupt JSON, wrong schema version, invalid/oversized fields, storage exceptions, safe defaults, export snapshot immutability, long provider notes, all price dimensions, pending conditional pricing, HTML injection in every untrusted field, offline CSP/no external resources, A4 print CSS, and sanitized filenames.
- [ ] Run the new tests and confirm expected missing-module failures.
- [ ] Implement a versioned Zod localStorage envelope under `new-api:product-quotation:draft:v1`; persist only user inputs and identifiers, never live price objects or credentials.
- [ ] Implement a script-free standalone HTML generator with escaped text, inline CSS, print headers, page-break protection, UTF-8, no-referrer, and `default-src 'none'` CSP. Render notes with `white-space: pre-wrap`; never interpolate user data into CSS.
- [ ] Implement Blob download with `text/html;charset=utf-8`, anchor `download`, and object-URL revocation after click.
- [ ] Run both new test files and Task 1 tests; refactor only after green.
- [ ] Commit persistence and export.

### Task 3: Quotation editor, model selection, calculator, and live preview

**Files:**
- Create: `web/src/features/product-quotation/index.tsx`
- Create: `web/src/features/product-quotation/hooks/use-quotation-draft.ts`
- Create: `web/src/features/product-quotation/components/quotation-editor.tsx`
- Create: `web/src/features/product-quotation/components/exchange-discount-calculator.tsx`
- Create: `web/src/features/product-quotation/components/provider-model-selector.tsx`
- Create: `web/src/features/product-quotation/components/provider-quote-settings.tsx`
- Create: `web/src/features/product-quotation/components/quotation-preview.tsx`
- Create: `web/src/features/product-quotation/components/price-dimension-table.tsx`
- Create: `web/src/features/product-quotation/components/__tests__/quotation-workspace.test.tsx`
- Create: `web/src/features/product-quotation/components/__tests__/provider-model-selector.test.tsx`

**Interfaces:**
- Consumes Task 1 domain builders/validation and Task 2 persistence/export.
- Page uses `usePricingData(true)` and records `dataUpdatedAt` or an equivalent successful fetch timestamp without resetting draft state.

- [ ] Write failing interaction tests for initial loading, fetch failure and retry, empty pricing, model search, single/provider/all selection, long model IDs, selected-group unavailability, global/provider discount precedence, invalid discount/rates, ratio result and copy feedback, model-ID copy feedback, refresh retaining the draft, clear draft, no-selection export guard, and responsive two-column/stacked layout contract.
- [ ] Run the component tests and confirm they fail for missing UI behavior rather than harness errors.
- [ ] Implement a `SectionPageLayout` workspace. On large screens use a bounded editor column and sticky paper preview; below the large breakpoint stack editor and preview without horizontal overflow.
- [ ] Implement calculator fields for actual exchange rate, fixed system exchange rate, and Chinese discount; show the exact effective coefficient and copyable group ratio without writing settings.
- [ ] Implement quote metadata, explicit base/group selection, provider-grouped searchable selection, select-all/provider selection, provider overrides, and notes. Preserve selected IDs across refresh; mark missing/unavailable selections visibly.
- [ ] Render all normalized price rows with original amount, effective discount, quote amount, currency, unit, condition, and pending state. Do not calculate cross-unit totals.
- [ ] Implement manual price refresh, successful fetch time, local draft status, clear draft, guarded HTML download, toast feedback, loading/error/empty states, accessible labels, keyboard operation, and copy buttons.
- [ ] Run component tests, all product-quotation tests, pricing tests, and typecheck; refactor only after green.
- [ ] Commit the page and components.

### Task 4: Root-only route, navigation placement, and i18n

**Files:**
- Create: `web/src/routes/_authenticated/product-quotation/index.tsx`
- Modify: `web/src/hooks/use-sidebar-data.ts`
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: other locale JSON files required by `i18n:check`
- Create: `web/src/features/product-quotation/components/__tests__/route-and-navigation.test.tsx`
- Generated by build: `web/src/routeTree.gen.ts`

**Interfaces:**
- Route imports `ProductQuotation` and guards with exact `ROLE.SUPER_ADMIN`, matching Task Plugins.
- Sidebar item uses a fitting existing Lucide icon, `url: '/product-quotation'`, and `requiredRole: ROLE.SUPER_ADMIN` between Task Plugins and System Settings.

- [ ] Write failing tests that prove admin/user roles redirect to `/403`, root renders, non-root sidebar omits the item, and root order is exactly Task Plugins → Product Quotation & Calculator → System Settings.
- [ ] Run the route/navigation tests and confirm the intended failures.
- [ ] Add the route and navigation item without changing Task Plugins or System Settings behavior.
- [ ] Add all page/control/status/export translation keys and complete every locale required by the repository checker; use English fallback text where a maintained translation is unavailable and provide polished Simplified Chinese.
- [ ] Regenerate the TanStack route tree through the standard build/plugin path; never hand-edit the generated file.
- [ ] Run route tests, `bun run i18n:check`, `bun run typecheck`, and affected-file lint; refactor only after green.
- [ ] Commit route/navigation/i18n integration.

### Task 5: Integration, visual QA, and production verification

**Files:**
- Modify only files above when a verified defect is found.
- Record: `.ccg/tasks/pricing-quotation-tool/review.md`

**Interfaces:**
- Validates Tasks 1–4 as one root-only workflow; no new production API or database state.

- [ ] Run all product-quotation tests and relevant pricing tests.
- [ ] Run `bun run typecheck`, affected-file `bun run lint`, `bun run format:check`, `bun run i18n:check`, and `bun run build:check`.
- [ ] Start the local frontend against an available API fixture/server, sign in as a root user or use a controlled test harness, and inspect desktop and narrow layouts, long IDs/notes, empty/error states, selection, copy feedback, refresh persistence, and export disabled states.
- [ ] Open the exported HTML offline and inspect A4 print preview, multi-provider pagination, long text wrapping, currencies, units, conditional pricing, and snapshot timestamps.
- [ ] Run `git diff --check` and review the complete branch diff for scope, secrets, generated files, and unrelated changes.
- [ ] Perform required cross-model and code review; record unavailable tooling honestly and resolve all Critical/Important findings.
- [ ] Update the CCG task to completed, archive it under `.ccg/tasks/archive/2026-09/`, and commit the archive. Do not deploy or push unless the user separately requests it.
