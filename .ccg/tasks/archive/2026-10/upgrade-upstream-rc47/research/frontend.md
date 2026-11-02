# Frontend rc47 integration research

Read-only analysis at local base `e32fdcda17c5c48d26a461f6e4fd6c2ce862aa11`, upstream rc46 `7d8b91268dab69d7150f345c448b5e250163146a`, rc47 `e835b1bfc43b565d625537830dbfb19328925134`. No dependencies installed, builds/tests run, source/index/ref writes, database connections, provider calls, or environment operations. Only this report is written.

Read root/web AGENTS, task requirements, shared guide/backend specs, project shadcn-ui skill and relevant composition/Base UI rules, billing rules, and full billing-expression design. Native-only review and PostgreSQL-only authorization override generic external-model and database-matrix instructions.

## Files Found

| Paths | Responsibility / integration treatment |
|---|---|
| `controller/ratio_sync.go:211`, `:450`, `:600` | Upstream source fetch/normalization and response. rc47 adds optional include_unchanged, includes identical quotes, determines expression precedence from normalized modes across all sources, omits source-less rows. No local delta since rc46: adopt rc47 changes without dropping request validation/fetch constraints. |
| `web/src/features/system-settings/models/upstream-ratio-sync{,-helpers,-table}.tsx/.ts`, `upstream-price-cells.tsx`, `conflict-confirm-dialog.tsx` | Pricing sync state, row/bulk selection, accessible labels, source comparisons and frozen confirmation. Needs semantic merge with all local Seedance and accessibility guards. |
| New rc47 `upstream-price-adjustment.ts`, `price-adjustment-select.tsx`; shared `components/data-table/core/row-action-menu.tsx` | Pure keep/divide/multiply adjustment, finite checks, supported AST leaf updates, source immutability; shared action menu gets compatible optional custom trigger. |
| `features/model-pricing/api.ts:76,194,212`, `pricing.ts:253,327` | Local write boundary and atomic request construction; target metadata/currency/version requirements, provider overrides and orphan removal. Preserve local implementation; pricing.ts receives task/ordinary classification and openai_ttl additions. |
| `features/model-pricing/model-pricing-panel.tsx`, `currency.ts`, `source-currency-selector.tsx`; `system-settings/models/model-pricing-sheet.tsx` | Local source USD/CNY semantics, metadata preflight, embedded editor, source versus site preview, safe draft retention. Do not resurrect deleted `pricing-currency-selector.tsx`. |
| `system-settings/models/model-ratio-visual-editor.tsx:417,678,859`, `ratio-settings-card.tsx` | Later develop prerequisite, last-override save action, explicit orphan removal and default preservation; keep even where rc47 does not touch these files. |
| `pricing/lib/billing-expr.ts`, `billing-expression/{types,visual}.ts`, `tier-expr.ts`, `dynamic-price.ts`; `system-settings/models/{model-pricing-core,tier-price-fields,tiered-pricing-editor,visual-billing-document-editor}.tsx/.ts` | cc12h token lane, estimator fallback and image_count-preserving visual edit; merge around local holiday/calendar and original-currency behavior. |
| `usage-logs/{types.ts,lib/format.ts,components/columns/common-logs-columns.tsx,components/dialogs/details-dialog.tsx,components/model-badge.tsx}` | cc12h token/price display and CopyButton adoption; preserve frozen currency/rate/provider audit, native image/video/task details and existing routing. |
| `keys/components/api-keys-mutate-drawer.tsx`, `keys/lib/api-key-form.ts`; `redemption-codes/components/redemptions-mutate-drawer.tsx`, `lib/redemption-form.ts`; `models/components/drawers/model-mutate-drawer.tsx`, `lib/model-form.ts` | rc47 close/reopen defaults, explicit null expiry, no stale vendor selection. Preserve local API-key routes/quota controls and metadata publication/order fields. |
| `channels/api.ts`, `lib/channel-form.ts`, `components/drawers/channel-mutate-drawer.tsx`, passthrough warning | Adopt compatible getChannelOps config composition and advanced-custom channel passthrough warning; preserve local provider IDs, categories, mappings, native routing and permission-aware request config. |
| `web/package.json`, `bun.lock`, `vitest.config.ts`, `src/test-setup.ts` | Bun 1.4.4/built-in typechecking; security lock overrides; local test infrastructure. |
| `build/site-brand.ts`, `src/config/site-brand.ts`, layout brand components, `rsbuild.config.ts`, assets, theme | Local Molii / Claudeye / iXiaozu profile behavior must survive; no upstream branding replacement needed. |
| Local `features/billing-calendar/{calendar.ts,index.tsx}`, `product-quotation/lib/quotation-math.ts:55` | Extra cc12h consumers absent upstream; labels and supported quote-variable parser need compatibility follow-up. |

## Dependencies

1. Source dialog -> getModelPricing baseline + fetchUpstreamRatios -> controller normalized full source quotes -> selected source+adjustment -> adjustSyncPrice -> applyPriceSyncSelections -> **local buildPricingChanges** -> frozen SyncPreview -> PATCH /api/option/model_pricing -> invalidation and reload.
2. buildPricingChanges compares complete configured drafts, includes expected_version and exact metadata source currency, preserves provider map, rejects the entire invalid batch before any write. Only explicitly emptied configured orphan entries get remove:true.
3. Pricing editor -> source-currency helpers + provider variant classifier -> shared visual parser/serializer -> preview/public dynamic price -> billing calendar / quotation -> usage-log summary. Adding cc12h must propagate along this entire path.
4. Cache lane chain: TOKEN_VARIABLES -> BILLING_VARS / cache-var map -> VisualTier -> estimator ExtraTokenValues -> model-pricing display/openai_ttl -> read-only details/log token fields. The rc47 buildEstimatorTokens signature gains exprStr, requiring every call site update.
5. Frontend runtime/toolchain is coupled to Docker/CI owners: local package scripts switch tsgo to bun check, remove native-preview and declare bun@1.4.4; root-owned Docker/CI must agree before acceptance.

## Patterns / non-negotiable local behavior

- `model-pricing/api.ts:212`: exact metadata + known USD/CNY source currency for every dirty model; untouched metadata-less builtin rows cannot block unrelated valid changes. Currency-only changes are dirty without rewriting coefficients; model versions remain the public read versions.
- `api.ts:252` onward: whole-batch prerequisite rejection and explicit orphan-only removal. Never turn Reset into removal, delete compiled defaults, manufacture fallback metadata/currency, flatten provider maps, or save builtin display expressions as configured overrides.
- `pricing.ts:327`: sync one complete quote per model, clears conflicting legacy/expression fields, preserves provider expressions and literal source expression bytes. Adjustment must always derive from immutable source, never compound.
- `model-pricing-sheet.tsx:293,824,1248`: exact-name metadata eligibility gates editor commit/save; unknown trimmed Add name retains draft. Last selected/final override removal retains an independent save action (`model-ratio-visual-editor.tsx:859`).
- All four Seedance managed names in `api.ts:113` remain excluded at API payload builder, sync state, row actions, bulk eligible counts and generic editor; route users to dedicated Seedance menu. rc47 upstream files lack these local guards.
- `upstream-price-cells.tsx:155,233`: native Label + Checkbox, visible localized action and unique model/source referenced with aria-labelledby. Retain direct-click/text-click/Space/focus/disabled behavior; no parallel toggle handlers.
- `model-pricing-snapshots.ts:103`: one unconditional tier is expression pricing; multiple parsed branches are tiered. Preserve labels and all local vendor/release/group ordering controls.
- `usage-logs/.../common-logs-columns.tsx:290`, `details-dialog.tsx:297`: frozen source_currency and cny_per_usd outrank current catalog/site rates. Preserve tool/provider identity and admin-only currency audit.
- Holiday functions/parser/runtime/visual branch order and timezone fallback are local contracts. rc47 image_count AST root support must compose with holiday/request factors and preserve untouched source strings.
- Shared UI discovery: read `components/data-table/README.md`, public exports, row-action-menu implementation, `dialog.tsx`, `confirm-dialog.tsx`, `copy-button.tsx`, `error-state.tsx`, local Select/Dropdown implementations, use-dialog and use-copy-to-clipboard, plus actual conflict dialog/sync/source-currency call sites. Existing ConfirmDialog+StaticDataTable covers confirmation; ErrorState covers reload/retry; CopyButton covers model copy; DataTableView/MobileCardList cover layouts. Reuse these.
- The current shared action-menu lacks only a custom trigger. rc47 adds that prop compatibly for compact adjustment control; no replacement menu is needed. SourceCurrencySelector changes USD/CNY denomination and is semantically unsuitable for keep/divide/multiply quote adjustment, so a feature PriceAdjustmentSelect composing project Select and shared menu is appropriate. Keep Base UI render/items APIs and SelectGroup composition.

## Risks and conflict plan

1. **Currency correctness (critical semantic integration risk):** rc47 sync helper comments and displays treat every expression as USD. Local baseline metadata can be CNY; controller pricingItems currently ignores upstream billing_currency, and getLocalPricingSyncData exports expressions without currency. Therefore a current CNY expression and a selected raw source expression must never silently gain a USD label or be written with a different implicit meaning. Preserve exact local metadata/source currency and coefficient bytes; make source quote / adjusted target / site preview units explicit and freeze the actual payload with those units. Legacy ratio/per-request values remain USD compatible. Do not infer upstream expression denomination from the site display setting or an arbitrary multiply/divide choice. Use known source metadata where available, and avoid claiming guessed conversion correctness where the source API supplies none.
2. **Sync merge:** start with rc47 adjustment architecture and confirmation snapshot, reapply all local Seedance state/row/bulk guards and localized label associations. Keep include_unchanged opt-in request and Show models with matching prices; selected rows remain visible after filter toggle; adjustments apply only to current selections, new source resets to keep. No old selections after source fetch error/reload. Preserve stale expected_version rejection and exact backend error with reload.
3. **Fixtures:** upstream new renderAdjustmentSync fixture has entries:[] and expects successful writes. With local safeguards it must fail. Repair fixtures with explicit eligible entries, real expected_version and billing_currency; never loosen buildPricingChanges or fabricate metadata in production code to satisfy those tests. The same applies to newly imported ordinary/provider fixtures.
4. **Provider editors:** merge usesTaskModelPricing and configureOrdinaryPricing with local sourceCurrency/metadataEligible/variant-only schema fallback and draft state. Ordinary defaults and provider overrides are separate; switching to ordinary freezes inherited active provider expressions without overwriting explicit or stale overrides. Keep per-call defaults, sole/stale provider removal, nested :: model names, unordered map equality and dirty-state reset.
5. **Cache/visual:** add cc12h without removing img_cr, is_holiday, local runtime calendar, exact-source spans or mixed fixed/token/request support. Preserve 1h 6/3.75 fallback; 12h falls back into cc at 2x only if cc is priced and cc12h is not. len includes raw counts once. Carry image_count around the whole pricing tree when adding conditions.
6. **Extra consumers:** quotation PRICE_VARIABLE_PATTERN lacks cc12h (and currently img_cr); a cc12h quote would be treated unsupported. Minimally add cc12h and a direct exact-price regression without expanding unrelated parser grammar. Billing calendar price label maps currently fallback to raw key for cc12h; add translated UI and ICS labels with focused tests.
7. **Log merge:** add cache_creation_tokens_12h/ratio_12h and hasAnyCacheTokens support. For OpenAI total cache writes, 30m=max(total-12h,0); display distinct 12h and do not add the aggregate as a second charged lane. Keep local source-currency formatters and task-specific details when porting upstream blocks (line numbers differ substantially).
8. **Drawer merge:** adopt null non-expiring form values and reset defaults on close, with asynchronous detail readiness gating saves. Model vendor clear is 0. Preserve local quota precision, key fields, model metadata source/publication fields and order editing; tests should control delayed detail promises, not sleep.
9. **Locales:** merge dictionaries by key/value while retaining local keys/translations; rc47 rewrites ~904 lines per language largely due ordering. Retain static key registry and local i18n:check script. Do not copy all upstream locale files wholesale.
10. **Branding/test harness:** preserve local profile assets, Claudeye dynamic favicon fallback and theme, Molii default wordmarks and footer attribution. Keep protected copyright headers. Preserve src/test-setup.ts reduced-motion/geometry/Storage shims, mock aliases and Vitest 20s heavy-suite timeout; do not delete user untracked duplicate-named files to reduce test failures.
11. Key drawer expiry work touches an authentication-related feature; if implementation extends beyond upstream expiry form reconciliation into credential/session behavior, root AGENTS OWASP read/verification gate applies. Do not broaden that scope.

## Safe Bun / lock integration

- Observed read-only: PATH `/opt/homebrew/bin/bun` = **1.3.14**. Current root Docker and frontend CI/release/electron/deploy use 1.4.0. rc47 package declares **bun@1.4.4** and switches build:check/typecheck to `bun check -b`; do not run these with current PATH Bun.
- After root authorizes implementation/runtime provisioning, use a verified task-local Bun 1.4.4 executable or approved root-owned builder, without replacing the user's global runtime.
- Merge package.json first: take rc47 axios ^1.20.0, i18next ^26.5.3, react-i18next ^17.1.0, input-otp ^1.5.0, resizable-panels ^4.14.6, builtin typecheck and packageManager. Retain local happy-dom ^20.11.1 and i18n:check.
- Retain local security overrides exactly: @hono/node-server 2.0.12, brace-expansion 2.1.3, fast-uri 4.1.2, hono 4.12.34, postcss 8.5.25, plus existing dompurify/katex/mermaid/minimist/qs/etc. Do not replace lock wholesale with upstream (would lose these changes).
- Resolve targeted lock entries with Bun 1.4.4 after package merge, preserving pinned unaffected resolutions; inspect any transitive churn, native-preview removal, html-parse-stringify 4 and obsolete void-elements removal. Later prove `bun install --frozen-lockfile` and a clean package/lock diff using the same runtime. Avoid unrestricted bun update or deleting lock.
- Coordinate root-owned Docker verified 1.4.4 image digest and CI pins; do not edit those from frontend work.

## Deterministic verification plan (not executed)

Run affected suites first with Bun 1.4.4; restore all localStorage/store/config/query/mocks per case; control network and time only at boundaries. Use real production functions, exact fixtures and observable controls, not copied pricing logic or timing retries.

- Pricing sync: extend existing `features/system-settings/__tests__/pricing-sync.test.tsx` with both upstream adjustment tests and local managed-model/accessible selection tests. Add CNY current versus target preview denomination and frozen currency/version payload, missing/unknown metadata blocks entire batch, provider override retention, matching-price selection, invalid-rate/overflow/underflow/unsupported-expression, source-reset/no-compounding, delayed confirmation rate-change test. Adapt upstream fixture entries as above.
- Local editor/write contracts: retain all five model-pricing test files, model-pricing-snapshots, plugin-pricing, ratio-settings-card, metadata editing. Keep orphan-removal exact payload and failure-preserved draft tests, last-override save action, metadata-less builtin no-op, untouched defaults, currency-only dirty and unknown currency rejection. Add ordinary/plugin mixed editor tests with metadata-valid USD/CNY fixtures.
- Cache/AST: extend existing billing-expression and visual-billing-editor suites for explicit/free cc12h, 2x fallback, unpriced free lane, 1h fallback unchanged, raw len threshold, image_count wrapping a newly inserted branch, preserved holiday/request rules and source text. Update all buildEstimatorTokens callers.
- Logs: preserve usage-facts, tiered-pricing-labels, token-context-display, source-billing-currency, task billing/Grok/image tests. Add 30m+12h count decomposition, frozen CNY price and zero fee cases; keep model copy interactions in usage-facts and CSV export cache detail.
- Drawers/channels: import upstream expiry-close/save/reopen-delayed-load and Never/Clear tests into existing key/redemption suites; add model vendor stale-reset case while retaining metadata/publication/order tests. Retain channel IDs/native routing tests and advanced-custom passthrough warnings. Retain channel priority-order/quick-options additions.
- Extra consumers: add cc12h quotation exact dimensions and calendar/ICS label regression in existing quotation-math/calendar/page tests; preserve holiday and original currency cases.
- Full frontend acceptance later: `bun run test`, `bun run typecheck`, changed-file `bun run lint -- <paths>` (script includes ".", so use direct configured oxlint for truly changed-only lint), `bun run i18n:check`, `bun run i18n:sync` followed by dictionary diff review, protected-header-safe format check, production build and bundle/brand-profile inspection.
- Important coverage gap: vitest include covers src and scripts/oxlint only, **not web/build/site-brand.test.ts**. This file imports bun:test; run `bun test build/site-brand.test.ts` separately, in addition to the full Vitest suite. Node-script i18n test also runs separately.
- No PostgreSQL or other database work in this research phase. Any later backend/controller tests only through root's audited PostgreSQL acceptance inventory. Never full root go test ./..., make test, SQLite/MySQL, live source/catalog writes, paid calls or production environment contact.

## Exact local frontend test preservation inventory

The following 164 added/modified tracked test files differ between rc46 and local base. Preserve every real contract in them; merge upstream tests into these where overlapping. This inventory intentionally omits two tests already deleted locally (`api-key-group-combobox.test.tsx`, `models/__tests__/metadata-sync.test.tsx`); do not resurrect them by wholesale upstream checkout. No inference is made about untracked files.

- `web/build/site-brand.test.ts`
- `web/scripts/i18n-completeness.test.mjs`
- `web/src/components/__tests__/user-avatar.test.tsx`
- `web/src/components/layout/components/__tests__/claudeye-wordmark.test.tsx`
- `web/src/components/layout/components/__tests__/header-brand.test.tsx`
- `web/src/components/layout/components/__tests__/ixiaozu-brand.test.tsx`
- `web/src/components/layout/components/__tests__/system-brand.test.tsx`
- `web/src/context/__tests__/theme-preferences.test.tsx`
- `web/src/features/auth/__tests__/auth-layout.test.tsx`
- `web/src/features/billing-calendar/__tests__/access.test.ts`
- `web/src/features/billing-calendar/__tests__/calendar.test.ts`
- `web/src/features/billing-calendar/__tests__/page.test.tsx`
- `web/src/features/channels/components/__tests__/channel-configuration.test.tsx`
- `web/src/features/channels/components/__tests__/priority-guide.test.tsx`
- `web/src/features/channels/components/__tests__/table-refresh.test.tsx`
- `web/src/features/channels/lib/__tests__/bytedance-seedance-channel.test.ts`
- `web/src/features/channels/lib/__tests__/channel-model-fetch.test.ts`
- `web/src/features/channels/lib/__tests__/model-categories.test.ts`
- `web/src/features/channels/lib/__tests__/molii-grok-channel.test.ts`
- `web/src/features/channels/lib/__tests__/starai-channel.test.ts`
- `web/src/features/dashboard/components/models/__tests__/performance-overview-summary.test.ts`
- `web/src/features/dashboard/components/overview/__tests__/setup-guide.test.tsx`
- `web/src/features/home/components/__tests__/home-footer.test.tsx`
- `web/src/features/home/components/__tests__/home-sections.test.tsx`
- `web/src/features/home/components/__tests__/model-search.test.tsx`
- `web/src/features/home/hooks/__tests__/use-model-placeholder-typewriter.test.ts`
- `web/src/features/home/lib/__tests__/home-model-catalog.test.ts`
- `web/src/features/keys/components/__tests__/api-key-api-routes.test.tsx`
- `web/src/features/keys/components/__tests__/api-key-listing.test.tsx`
- `web/src/features/keys/components/__tests__/api-key-model-display-info.test.ts`
- `web/src/features/keys/components/__tests__/api-key-restriction-cells.test.tsx`
- `web/src/features/keys/components/__tests__/api-key-timestamp-cell.test.tsx`
- `web/src/features/keys/components/__tests__/api-keys-mutate-drawer.test.tsx`
- `web/src/features/keys/components/__tests__/auto-group-order-editor.test.tsx`
- `web/src/features/keys/components/__tests__/ip-cidr-chip-input.test.tsx`
- `web/src/features/keys/lib/__tests__/auto-group-form.test.ts`
- `web/src/features/keys/lib/__tests__/default-api-key.test.ts`
- `web/src/features/keys/lib/__tests__/rotate-api.test.ts`
- `web/src/features/model-pricing/__tests__/editor-currency.test.tsx`
- `web/src/features/model-pricing/__tests__/editor-layout.test.tsx`
- `web/src/features/model-pricing/__tests__/metadata-prerequisite.test.tsx`
- `web/src/features/model-pricing/__tests__/pricing.test.ts`
- `web/src/features/model-pricing/__tests__/save-errors.test.tsx`
- `web/src/features/models/__tests__/metadata-editing.test.tsx`
- `web/src/features/models/__tests__/model-listing.test.tsx`
- `web/src/features/models/components/__tests__/model-order-editor.test.tsx`
- `web/src/features/models/components/dialogs/__tests__/vendor-management.test.tsx`
- `web/src/features/models/components/dialogs/__tests__/vendor-ordering.test.tsx`
- `web/src/features/models/components/drawers/__tests__/model-publication-status.test.tsx`
- `web/src/features/models/lib/__tests__/local-metadata-source.test.ts`
- `web/src/features/models/lib/__tests__/marketplace-order-api.test.ts`
- `web/src/features/models/lib/__tests__/model-form.test.ts`
- `web/src/features/pricing/__tests__/dynamic-price.test.ts`
- `web/src/features/pricing/__tests__/model-cards.test.tsx`
- `web/src/features/pricing/__tests__/pricing-controls.test.tsx`
- `web/src/features/pricing/__tests__/sorting.test.ts`
- `web/src/features/pricing/__tests__/task-price-display.test.tsx`
- `web/src/features/pricing/components/__tests__/dynamic-pricing-breakdown.test.tsx`
- `web/src/features/pricing/components/__tests__/grok-pricing-matrix.test.tsx`
- `web/src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx`
- `web/src/features/pricing/components/__tests__/model-card-grok-pricing.test.tsx`
- `web/src/features/pricing/components/__tests__/model-card-seedance-pricing.test.tsx`
- `web/src/features/pricing/components/__tests__/model-card-text-pricing.test.tsx`
- `web/src/features/pricing/components/__tests__/model-details-cache-price.test.tsx`
- `web/src/features/pricing/components/__tests__/model-details-capabilities.test.tsx`
- `web/src/features/pricing/components/__tests__/model-details-directory-page.test.tsx`
- `web/src/features/pricing/components/__tests__/model-directory-controls.test.tsx`
- `web/src/features/pricing/components/__tests__/model-directory-layout.test.ts`
- `web/src/features/pricing/components/__tests__/model-price-cell-currency.test.tsx`
- `web/src/features/pricing/components/__tests__/pricing-table-infinite.test.tsx`
- `web/src/features/pricing/components/__tests__/video-api-details.test.ts`
- `web/src/features/pricing/components/__tests__/video-pricing-matrix.test.tsx`
- `web/src/features/pricing/hooks/__tests__/use-infinite-models.test.tsx`
- `web/src/features/pricing/lib/__tests__/dynamic-cny-pricing.test.ts`
- `web/src/features/pricing/lib/__tests__/dynamic-pricing-strategy.test.ts`
- `web/src/features/pricing/lib/__tests__/gpt-image-2-api-details.test.ts`
- `web/src/features/pricing/lib/__tests__/grok-api-parameters.test.ts`
- `web/src/features/pricing/lib/__tests__/grok-api-sample.test.ts`
- `web/src/features/pricing/lib/__tests__/grok-model.test.ts`
- `web/src/features/pricing/lib/__tests__/grok-pricing-table.test.ts`
- `web/src/features/pricing/lib/__tests__/holiday-billing-expression.test.ts`
- `web/src/features/pricing/lib/__tests__/model-card-summary.test.ts`
- `web/src/features/pricing/lib/__tests__/model-description.test.ts`
- `web/src/features/pricing/lib/__tests__/model-directory.test.ts`
- `web/src/features/pricing/lib/__tests__/model-helpers.test.ts`
- `web/src/features/pricing/lib/__tests__/pricing-search.test.ts`
- `web/src/features/pricing/lib/__tests__/text-model-card-pricing.test.ts`
- `web/src/features/pricing/lib/__tests__/three-model-chat-api.test.ts`
- `web/src/features/pricing/lib/__tests__/video-model.test.ts`
- `web/src/features/product-quotation/components/__tests__/provider-model-selector.test.tsx`
- `web/src/features/product-quotation/components/__tests__/quotation-workspace.test.tsx`
- `web/src/features/product-quotation/components/__tests__/route-and-navigation.test.tsx`
- `web/src/features/product-quotation/lib/__tests__/draft-storage.test.ts`
- `web/src/features/product-quotation/lib/__tests__/quotation-format.test.ts`
- `web/src/features/product-quotation/lib/__tests__/quotation-html.test.ts`
- `web/src/features/product-quotation/lib/__tests__/quotation-math.test.ts`
- `web/src/features/rankings/components/__tests__/group-success-section.test.tsx`
- `web/src/features/rankings/components/__tests__/ranking-icons.test.tsx`
- `web/src/features/rankings/lib/__tests__/chart-tooltip-icons.test.tsx`
- `web/src/features/system-info/__tests__/system-tasks-panel.test.tsx`
- `web/src/features/system-settings/__tests__/pricing-sync.test.tsx`
- `web/src/features/system-settings/billing/__tests__/molii-aigc-pricing-tabs.test.tsx`
- `web/src/features/system-settings/billing/__tests__/pricing-unit-input.test.tsx`
- `web/src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx`
- `web/src/features/system-settings/maintenance/__tests__/log-cleanup-result.test.ts`
- `web/src/features/system-settings/maintenance/__tests__/log-settings.test.tsx`
- `web/src/features/system-settings/models/__tests__/group-metadata-defaults.test.ts`
- `web/src/features/system-settings/models/__tests__/group-metadata-order.test.tsx`
- `web/src/features/system-settings/models/__tests__/group-pricing-decimals.test.tsx`
- `web/src/features/system-settings/models/__tests__/group-settings.test.tsx`
- `web/src/features/system-settings/models/__tests__/model-pricing-snapshots.test.ts`
- `web/src/features/system-settings/models/__tests__/model-ratio-table-columns.test.tsx`
- `web/src/features/system-settings/models/__tests__/plugin-pricing.test.tsx`
- `web/src/features/system-settings/models/__tests__/ratio-settings-card.test.tsx`
- `web/src/features/system-settings/models/__tests__/visual-billing-editor.test.tsx`
- `web/src/features/system-settings/site/__tests__/claudeye-brand-appearance-section.test.tsx`
- `web/src/features/system-settings/site/__tests__/claudeye-brand-colors.test.ts`
- `web/src/features/temporary-assets/components/__tests__/asset-refresh-button.test.tsx`
- `web/src/features/temporary-assets/components/__tests__/asset-type-filter.test.tsx`
- `web/src/features/temporary-assets/components/__tests__/source-options.test.tsx`
- `web/src/features/temporary-assets/lib/__tests__/asset-actions.test.ts`
- `web/src/features/temporary-assets/lib/__tests__/asset-utils.test.ts`
- `web/src/features/temporary-assets/lib/__tests__/upload-utils.test.ts`
- `web/src/features/usage-logs/audit/__tests__/viewer.test.tsx`
- `web/src/features/usage-logs/components/__tests__/common-log-task-id.test.tsx`
- `web/src/features/usage-logs/components/__tests__/detail-preview.test.tsx`
- `web/src/features/usage-logs/components/__tests__/grok-image-log-display.test.tsx`
- `web/src/features/usage-logs/components/__tests__/grok-video-log-display.test.tsx`
- `web/src/features/usage-logs/components/__tests__/group-filter.test.tsx`
- `web/src/features/usage-logs/components/__tests__/task-log-token-model-columns.test.tsx`
- `web/src/features/usage-logs/components/__tests__/tiered-pricing-labels.test.ts`
- `web/src/features/usage-logs/components/__tests__/token-context-display.test.tsx`
- `web/src/features/usage-logs/components/__tests__/usage-facts.test.tsx`
- `web/src/features/usage-logs/components/dialogs/__tests__/gpt-image-2-preview-card.test.tsx`
- `web/src/features/usage-logs/components/dialogs/__tests__/grok-image-preview-card.test.tsx`
- `web/src/features/usage-logs/components/dialogs/__tests__/task-timing-dialog.test.tsx`
- `web/src/features/usage-logs/components/dialogs/__tests__/video-preview-metadata.test.ts`
- `web/src/features/usage-logs/components/dialogs/__tests__/video-preview-observability.test.tsx`
- `web/src/features/usage-logs/lib/__tests__/gpt-image-2-billing.test.ts`
- `web/src/features/usage-logs/lib/__tests__/gpt-image-2.test.ts`
- `web/src/features/usage-logs/lib/__tests__/grok-image-billing.test.ts`
- `web/src/features/usage-logs/lib/__tests__/grok-video-billing.test.ts`
- `web/src/features/usage-logs/lib/__tests__/log-routing.test.ts`
- `web/src/features/usage-logs/lib/__tests__/source-registry.test.ts`
- `web/src/features/usage-logs/lib/__tests__/task-billing.test.ts`
- `web/src/features/usage-logs/lib/__tests__/task-log-links.test.ts`
- `web/src/features/usage-logs/lib/__tests__/task-video-preview.test.ts`
- `web/src/features/video-generation/__tests__/api.test.ts`
- `web/src/features/video-generation/__tests__/form-controls.test.tsx`
- `web/src/features/video-generation/components/__tests__/media-picker.test.tsx`
- `web/src/features/video-generation/components/__tests__/paid-request-dialog.test.tsx`
- `web/src/features/video-generation/components/__tests__/prompt-composer.test.tsx`
- `web/src/features/video-generation/components/__tests__/video-studio-results.test.tsx`
- `web/src/features/video-generation/lib/__tests__/authoring.test.ts`
- `web/src/features/video-generation/lib/__tests__/form-schema.test.ts`
- `web/src/features/video-generation/lib/__tests__/payload.test.ts`
- `web/src/hooks/__tests__/sidebar-config.test.tsx`
- `web/src/lib/__tests__/dom-utils.test.ts`
- `web/src/lib/__tests__/favicon-assets.test.ts`
- `web/src/lib/__tests__/quota-editing.test.ts`
- `web/src/lib/__tests__/site-brand.test.ts`
- `web/src/lib/__tests__/source-billing-currency.test.ts`
- `web/src/lib/__tests__/system-name.test.ts`
- `web/src/styles/__tests__/theme-contract.test.ts`
