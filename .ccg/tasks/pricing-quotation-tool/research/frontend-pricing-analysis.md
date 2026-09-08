# Frontend pricing and quotation research

Scope: read-only investigation for the product quotation calculator. This
document proposes frontend integration points only; it does not change the
existing public model directory or system pricing configuration.

## Files Found

### API contract and existing consumer

- `web/src/features/pricing/api.ts:27-30` — the sole frontend request helper
  for `GET /api/pricing`; it returns `res.data` directly, so a quotation page
  should reuse `getPricing()` rather than introduce a second endpoint wrapper.
- `web/src/features/pricing/types.ts:23-168` — authoritative client-side
  `PricingData`, `PricingVendor`, `PricingModel`, video, and Grok catalog
  types. Current develop also adds task-usage dimensions:
  `billing_usage_schema` (numeric/enum fields with units) and optional labeled
  `billing_usage_examples`. The useful identity mapping is `PricingVendor.id` <->
  `PricingModel.vendor_id`; the user-visible provider name is
  `PricingVendor.name`. Both vendor/model have optional `display_order`;
  `PricingModel.model_name` is the stable model ID.
- `web/src/features/pricing/hooks/use-pricing-data.ts:26-77` — React Query
  consumer with `queryKey: ['pricing']`, a five-minute `staleTime`, and
  normalized models. It builds a vendor map, fills `vendor_name`,
  `vendor_icon`, `vendor_description`, assigns `key: model_name`, and attaches
  the global `data.group_ratio` to every model. It also exposes `refetch`,
  `groupRatio`, and `usableGroup`.
- `web/src/features/pricing/index.tsx:46-294` — current `/pricing` page.
  Useful reference for loading layout, error/empty handling, responsive
  desktop sidebar/mobile controls, model searching, and passing selected group
  / exchange-rate context into formatters.
- `web/src/features/pricing/lib/filters.ts:41-66` — case-insensitive search
  includes model ID, description, tags, and `vendor_name`; vendor filtering
  currently uses a name, not an ID. Use model name as selected-model ID, but
  group selections/overrides by numeric vendor ID to avoid collisions between
  identically named providers.
- `web/src/features/pricing/components/pricing-sidebar.tsx:182-199` — builds
  provider options from `vendors`, then counts the normalized `vendor_name` on
  models. It is a reference for provider grouping and icons, not a reusable
  multi-select component.

### Price calculation and special-price renderers

- `web/src/features/pricing/lib/price.ts:57-239` — fixed token and per-request
  price derivation. `formatPrice` derives input/output/cache/create-cache/image
  / audio prices from ratios; `formatGroupPrice` and `formatFixedPrice` apply a
  specified group ratio. These functions return display strings, so they are
  unsuitable as the quotation's canonical calculation values.
- `web/src/features/pricing/lib/model-helpers.ts:29-94` — reusable
  `getAvailableGroups`, `getConfiguredGroupRatio`, and
  `getDisplayGroupRatio`. The latter silently selects the minimum enabled
  group when no group is specified; a quotation must instead require an
  explicit selected group/baseline to prevent accidental underquoting.
- `web/src/features/pricing/lib/dynamic-price.ts:172-225,482-648` and
  `web/src/features/pricing/lib/billing-expr.ts:31-143` — dynamic model
  detection and the intentionally narrow parser for token *and task-usage*
  tier expressions and request rules. `getDynamicPricingSummary` retains
  parsed tiers, request-rule presence, raw expression, `isTaskUsage`, and a
  `isSpecialExpression` fallback. For a task schema, units can be `second`,
  `count`, `token`, or `credit` rather than a universal per-token amount.
- `web/src/features/pricing/lib/task-expr.ts:25-76,280-344` — task pricing
  parser/evaluator. It provides schema-derived numeric/enum field discovery
  and evaluates an explicit usage example into a base/request charge plus
  unit charges. It is the correct reference for a task-plugin-style model;
  preserve the input facts/conditions, rather than presenting its value as a
  simple unit price.
- `web/src/features/pricing/components/dynamic-pricing-breakdown.tsx:160-260`
  — reference UI for full tier/request-rule disclosure. Its fallback renders
  the raw expression when no structured tier parse is possible.
- `web/src/features/pricing/lib/video-pricing.ts:21-31` and
  `web/src/features/pricing/components/video-pricing-matrix.tsx:25-179` —
  video pricing rows have resolution, `with_video` vs `without_video`, fps,
  extra frames, and an explicit formula. Prices are per one million/k token
  units, not a single per-video price.
- `web/src/features/pricing/components/grok-pricing-matrix.tsx:25-130` and
  `web/src/features/pricing/lib/grok-pricing-table.ts:41-77` — direct CNY
  catalog pricing. It preserves output resolution/quality tiers and separately
  exposes image and video input charges with their own units.
- `web/src/lib/currency.ts:1-71, 264-330` — currency rules. For a pricing
  display use `formatBillingCurrencyFromUSD`, which never turns price into
  tokens. `formatCurrencyFromUSD` can render a token display and is currently
  used by legacy fixed-price helpers, so it is not a safe generic exporter
  formatter. Both formatters round according to configured small/large digits.

### Route, permission, navigation, persistence, export, and tests

- `web/src/routes/_authenticated/route.tsx:24-36` — authenticated route
  parent requires both an authenticated user and access token.
- `web/src/routes/_authenticated/system-settings/route.tsx:25-35` — all
  descendant system-settings routes are guarded at the route level by
  `auth.user.role === ROLE.SUPER_ADMIN`; a calculator added underneath inherits
  this frontend restriction.
- `web/src/lib/roles.ts:20-44` — `ROLE.SUPER_ADMIN` is exactly `100`.
- `web/src/hooks/use-sidebar-data.ts:127-174` — confirmed current develop
  root Admin navigation. It has a `Task Plugins` item at lines 161-166 and a
  `System Settings` item at lines 167-172, so this is the exact requested
  insertion point. Both the existing Task Plugins item and the proposed quote
  tool must carry `requiredRole: ROLE.SUPER_ADMIN`.
- `web/src/routes/_authenticated/task-plugins/index.tsx:19-30` — exact route
  guard/reference for the requested peer route: signed-in parent plus explicit
  `ROLE.SUPER_ADMIN`, then redirect `/403`. The calculator should be a sibling
  of `/task-plugins`, not a system-settings subsection.
- `web/src/features/task-plugins/index.tsx:48-79` — current peer page uses
  `SectionPageLayout` and i18n, a useful layout convention for a top-level
  admin tool.
- `web/src/components/layout/config/system-settings.config.ts:47-107` —
  separate contextual settings sidebar; it remains relevant only as the item
  *after* the new peer route, not as the calculator's home.
- `web/src/features/system-settings/utils/section-registry.ts:23-99` —
  reusable section registry for existing settings pages. Its `urlStyle: 'path'`
  form can create an item like `/system-settings/billing/<id>`, but calculator
  state is independent of persisted server settings and should not be treated
  as an editable billing section unless the intended URL is explicitly under
  billing.
- `web/src/components/layout/lib/sidebar-view-registry.ts:24-58` — only
  `SYSTEM_SETTINGS_VIEW` is registered; an independent calculator route would
  retain root navigation unless it registers a new sidebar workspace.
- `web/src/features/system-settings/hooks/use-accordion-state.ts:41-105` and
  `web/src/features/playground/lib/storage/storage.ts:32-87,279-306` — local
  storage patterns. The latter is the closest robust reference: versioned
  envelope, Zod validation after parsing, and `try/catch` around both storage
  access and JSON parsing. A calculator draft should use its own namespaced
  key, version, schema, and no sensitive data.
- `web/src/components/ai-elements/code-block.tsx:470-482` — existing browser
  download primitive: `Blob`, `URL.createObjectURL`, an anchor `download`,
  click, then `URL.revokeObjectURL`.
- `web/src/components/ui/markdown.tsx:165-172` — local five-character HTML
  escaping routine. Export must use an equivalent helper for *every* dynamic
  text field (title, provider/model names, notes, author/date, and values)
  before composing standalone HTML; React escaping does not protect a string
  later placed in a Blob as HTML.
- `web/src/routeTree.gen.ts:1-7` — generated by TanStack Router and explicitly
  must not be hand-edited. Adding a route source lets the router plugin rebuild
  it during development/build.
- `web/src/features/pricing/lib/__tests__/dynamic-cny-pricing.test.ts:1-42`
  — pure logic uses `node:test` and strict assertions. Component tests under
  `web/src/features/pricing/components/__tests__/` create a `happy-dom`
  window, initialize i18n, and render with `react-dom` + `act` (for example
  `grok-pricing-matrix.test.tsx:19-76`).

## Dependencies

```text
GET /api/pricing
  -> getPricing() [pricing/api.ts]
  -> usePricingData() [React Query key: ['pricing']]
     -> vendor_id joins PricingData.vendors[].id
     -> normalized model: model_name + vendor_name + group_ratio
     -> calculator model search/group selection
        -> canonical quotation price rows (new pure lib)
           -> editable discount / actual exchange / fixed exchange multiplier
           -> React preview and standalone HTML generator use the same snapshot
        -> Zod-validated localStorage draft (selected model_name values;
           provider overrides keyed by vendor id)
```

Route and navigation dependency on current develop:

```text
/_authenticated parent (signed in)
  -> /_authenticated/system-settings parent (role === SUPER_ADMIN)
  -> proposed peer calculator route
  -> root Admin navigation item (between Task Plugins and System Settings)
```

The backend data is already group-adjustable: `PricingData.group_ratio` is
attached to all normalized models (`use-pricing-data.ts:45-62`). Do not use the
directory's default “best available group” behavior for an official quote.

## Patterns

1. **Reuse the API and normalized hook, then keep quote math pure.**
   `usePricingData` owns cache/fetch/refetch and vendor joining
   (`use-pricing-data.ts:29-63`). New code can either consume that hook or
   factor the normalization into an exported pure helper if tests need it;
   do not duplicate `api.get('/api/pricing')`.

2. **Keep numbers and display strings separate.** Existing `formatPrice` and
   `formatFixedPrice` produce user-formatted strings and apply system group /
   recharge behavior (`price.ts:144-239`). A quotation should calculate a
   typed row such as `{ sourceAmount, sourceCurrency, unit, groupRatio,
   discountCoefficient, quoteAmount, displayAmount, condition? }`, then format
   it only at preview/export boundaries. This preserves zero values and small
   values without parsing formatted strings.

3. **Discount rule:** normalize Chinese “折” to a coefficient (`8 -> 0.8`,
   `5 -> 0.5`) once; provider override takes precedence over global. Keep the
   configured system group multiplier and quotation multiplier separately.
   The requirement's group multiplier is
   `actualExchangeRate * discountCoefficient / fixedSystemExchangeRate`; do
   not also multiply it by `data.group_ratio[group]` unless the selected price
   baseline explicitly requires that separate system group adjustment.

4. **Explicit price baseline:** show the selected user group, currency,
   `/1M` or `/1K` token basis, and price-fetch time in preview/export. In
   particular, `getDisplayGroupRatio` picks the lowest enabled group by design
   when no group is passed (`model-helpers.ts:53-94`), which is not suitable
   for a commercial quote.

5. **Use existing complete special-price data, not model-name heuristics.**
   Dynamic pricing is identified by `billing_mode === 'tiered_expr'` plus an
   expression (`dynamic-price.ts:172-174`); video by `video_pricing`; Grok
   direct catalog by `molii_grok_pricing`; current develop additionally marks
   task usage by a populated `billing_usage_schema`
   (`dynamic-price.ts:176-215`). Preserve model ID as the selection key even
   when `display_name` is shown. Preserve task fields, units, enum conditions,
   and a base charge as independent dimensions.

6. **Draft storage:** implement `loadQuotationDraft` / `saveQuotationDraft`
   in a feature-local `lib/draft-storage.ts`, validate with Zod, tolerate bad
   JSON/storage quotas, and include a version envelope. Save draft-only user
   inputs/selection; refreshed API records replace price data and never clear
   the draft.

7. **Export:** build a complete HTML string through a pure
   `buildQuotationHtml(snapshot)` function, escape all interpolated text,
   include inline CSS plus `@page` / print rules, and download exactly that
   immutable snapshot with the existing Blob/anchor pattern. Do not use
   `dangerouslySetInnerHTML` as the exporter.

8. **i18n and tests:** customer-facing controls must use `useTranslation()`
   per `web/AGENTS.md`; new keys need registration/translation coverage. Put
   pure tests in `features/pricing-quotation/lib/__tests__/` and component
   tests in `features/pricing-quotation/components/__tests__/`, rather than
   beside implementation files.

## Risks

### Price semantics

- **Double application of multipliers/discounts:** public directory helpers
  already choose/apply group ratios and optionally a recharge rate. The quote
  formula is distinct and must expose whether source values are base system
  values or group-adjusted. Treat every multiplier as named metadata.
- **Dynamic expressions are not universally normalizable:** the parser is
  intentionally narrow, and the current token/task entry builders drop `<= 0`
  values (`dynamic-price.ts:503-567`). For a quote, zero may be a valid free price;
  preserve it. If tiers or request rules cannot be represented reliably, print
  the condition/raw expression and “待确认”, never invent a zero unit price.
- **Task-usage expressions are a fourth special-price category:** current
  develop supports a per-request base charge plus units such as seconds,
  counts, credits, and token-scaled values (`task-expr.ts:280-326`). An export
  must print each field/unit and its tier conditions; do not collapse task
  examples, `video_pricing`, or direct Grok pricing into an unqualified
  “per request” price.
- **CNY dynamic values must bypass USD conversion:** `billing_currency: 'CNY'`
  is returned directly by `formatDynamicUnitPrice` (`dynamic-price.ts:124-148`)
  and is protected by `dynamic-cny-pricing.test.ts`. Applying currency exchange
  or a USD formatter to it changes the quoted price.
- **Image/video values are multi-dimensional:** `video_pricing` has output
  resolution, reference-video path, fps and actual token formula;
  `molii_grok_pricing` has resolution/quality output tiers and separate
  image/video input prices and units. Do not sum them into a model-level total
  or label them all “per request”. Include every applicable row and its unit.
- **Legacy request price masks absence as zero:** current `formatFixedPrice`
  uses `(model.model_price || 0)` (`price.ts:225-226`). A quotation's canonical
  model must distinguish missing/invalid from `0`, showing “待确认” for missing
  prices while retaining an explicit zero price.
- **Rounding can misquote small amounts:** current generic formatters normally
  use four or six decimals (`currency.ts:264-330`). Preserve numeric precision
  for calculation/export (or document a significant-digit policy); never
  calculate from rounded preview text.

### UI, state, security, and routing

- `/api/pricing` is currently a public directory API; frontend-only access
  controls do not secure data. The new backend endpoint/route guard must use
  the established super-admin authorization mechanism as required.
- The requested navigation location is now confirmed on develop:
  `use-sidebar-data.ts:161-172` has consecutive root Admin entries for Task
  Plugins and System Settings. Add the calculator as a peer immediately
  between them, with both sidebar `requiredRole` and the Task-Plugins-style
  route guard; do not put it under Billing & Payment.
- Route source changes regenerate `routeTree.gen.ts`; manual generated-file
  edits will be overwritten.
- Local storage is user-controllable and can be corrupt/stale. Validate all
  fields, ignore unknown model IDs after a price refresh, and avoid persisting
  live price objects as the source of truth.
- Standalone HTML is an XSS boundary. Escape text before template insertion;
  quoted notes and names can otherwise execute when the downloaded file is
  opened.
- `URL.revokeObjectURL` should occur after initiating the download; if browser
  compatibility proves problematic, schedule revocation on a short timeout,
  but do not leave object URLs unbounded.

## Recommended file split

Isolate a new peer feature rather than adding business logic to the public
`/pricing` directory:

```text
web/src/features/pricing-quotation/
  api.ts                         # optional re-export of getPricing only
  types.ts                       # draft, selected provider/model, quote row/snapshot
  index.tsx                      # page composition: desktop two-column / stacked mobile
  hooks/use-quotation-draft.ts   # load/save/debounce/reset around localStorage
  hooks/use-quotation-data.ts    # consumes usePricingData or normalized helper
  lib/quotation-math.ts          # discount parsing, validation, canonical price rows
  lib/quotation-format.ts        # precision-safe formatters, filename normalization
  lib/quotation-html.ts          # escapeHtml + standalone A4 snapshot HTML
  lib/draft-storage.ts           # versioned Zod guarded localStorage adapter
  components/quotation-editor.tsx
  components/provider-model-selector.tsx
  components/quote-preview.tsx
  components/price-dimension-table.tsx
  components/exchange-discount-calculator.tsx
  components/__tests__/selection.test.tsx
  components/__tests__/draft-preview.test.tsx
  lib/__tests__/quotation-math.test.ts
  lib/__tests__/quotation-html.test.ts
  lib/__tests__/draft-storage.test.ts
web/src/routes/_authenticated/product-quotation/index.tsx
```

Add a `Product Quotation` root Admin item in
`web/src/hooks/use-sidebar-data.ts` immediately after Task Plugins and before
System Settings, with `requiredRole: ROLE.SUPER_ADMIN`. The route is a sibling
of `task-plugins` and should copy that route's `beforeLoad` guard.

## Suggested tests and commands

Minimum behavioral coverage:

- pure math: global/provider override precedence; valid zero vs missing;
  tiny precision; invalid Chinese discount/exchange input; ratio formula;
  token/request/cache/image/audio dimensions; dynamic fallback; CNY behavior;
  video/Grok tier units; task usage base/unit charges, token scaling, enum
  tiers and noncanonical task-expression fallback;
- selection UI: search, single model, provider select-all and all select,
  disabled export for empty/invalid/failed states, and model ID copy;
- persistence: valid restore, corrupt/old schema ignored, price refresh retains
  user draft, unknown selected model safely omitted;
- exporter: immutable input snapshot, sanitized title/note/model/provider
  output, clean filename, inline CSS / A4 print output, all dimensions and
  fetch time present;
- route: non-super-admin redirect/frontend guard, plus backend permission test
  in its respective layer.

Run from `web/` after implementation:

```bash
bun test src/features/pricing-quotation/lib/__tests__/quotation-math.test.ts
bun test src/features/pricing-quotation/lib/__tests__/quotation-html.test.ts
bun test src/features/pricing-quotation/components/__tests__/selection.test.tsx
bun run typecheck
bun run lint
bun run format:check
bun run i18n:check
bun run build:check
```

The repository CI uses `bun test` (see `.github/workflows/ci.yml:88`); run the
full command once focused tests pass, subject to any pre-existing test baseline
failures.
