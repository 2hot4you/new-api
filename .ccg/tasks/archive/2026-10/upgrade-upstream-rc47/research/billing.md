# rc.47 billing compatibility research

Read-only analysis at local base `e32fdcda17c5c48d26a461f6e4fd6c2ce862aa11`, merge base / rc.46 `7d8b91268dab69d7150f345c448b5e250163146a`, target `e835b1bfc43b565d625537830dbfb19328925134`.
Read in full: `AGENTS.md`, `.agents/rules/billing.md`, `pkg/billingexpr/expr.md`, task `requirements.md`; also read both existing `.ccg/spec/` indexes. No tests, DB initialization, network calls, source changes, index/ref/object writes, or external executors were used. The sole write is this report. PostgreSQL-only authorization takes precedence over generic three-engine instructions.

## Files Found

| Files | Responsibility |
| --- | --- |
| `pkg/billingexpr/{compile,run,types}.go` | v1 environment, AST identifier detection, execution input and frozen snapshot/result contracts. rc.47 adds `cc12h`. |
| `pkg/billingexpr/{settle,round}.go` | Local source-currency normalization, frozen CNY/USD rate, checked settlement clamp and strict reservation conversion; untouched upstream in rc.47. |
| `setting/billing_setting/{builtin_billing,tiered_billing}.go` | GPT-6 Astra compiled quote; explicit override/legacy precedence, builtin provenance, schema checks, smoke validation. |
| `types/price_data.go` | rc.47 shared TTL multipliers and `CacheCreation12hRatio`; guarded private multiplier map remains intact. |
| `relay/helper/price.go` | Legacy ratios and expression reservation; local immutable `selectedModelPricing` and frozen currency must survive conflict resolution. |
| `relay/helper/catalog_pricing.go`, `relay/common/pricing_selection.go` | Local atomic attempt capture of model/provider, currency, ratio, expression and tools. Critical integration boundary outside the originally enumerated billing files. |
| `service/{tiered_settle,text_quota}.go` | Cache normalization, actual expression/legacy settlement, checked quota calculation, tool surcharges and log usage. |
| `service/{responses_usage,log_info_generate}.go` | Deep-copy incoming Responses detail pointers and log actual normalized `cc12h`, preserving local money audit. |
| `relaykit/dto/{openai_response,usage_merge,billing_usage}.go` | Independent DTO prerequisites for cache-write TTL split, bounds, copying, presence and merge. Coordinate protocol-module ownership. |
| `model/model_pricing_conversion.go`, `model/model_pricing_config.go` | rc.47 legacy migration/preview `openai_ttl` and provider variant visibility. Coordinate model/catalog owner. |
| `deploy/tests/{test_postgres_ci,test_catalog_postgres_gate}.py` | Audited pure packages and required PostgreSQL test selection. Existing service selection misses the new billing behavior. Coordinate acceptance owner. |

## Dependencies

1. Upstream usage → independent relaykit `InputTokenDetails.CacheWriteTokensDetails` (`ttl_30m_tokens`, `ttl_12h_tokens`) → detached `Clone()` and nonzero merge → `CacheWrite12hTokens()` clamps the 12h subset to `[0, CacheCreationTokensTotal()]`.
2. Host Responses accumulation → `NormalizeResponsesUsage` / `ApplyResponsesUsage` → canonical `BillingUsage` selection → `calculateTextQuotaSummary` and `BuildTieredTokenParams` consume the same effective usage.
3. Catalog read boundary → `CaptureRequestPricing` or `CaptureTaskPricing` → immutable `RequestPricingSelection` → `ModelPriceHelper` → frozen `BillingSnapshot` and request input → `TryTieredSettle` → `ExpressionAmounts` → checked quota → existing durable billing session → public money/usage audit plus admin-only saturation marker.
4. Token expression `cc12h` must exist in compile prototype, runtime environment and `TokenParams` together; normalized token log injection must also carry it. Frontend expression parser/editor/display and model conversion must recognize the same new variable/mode.
5. Legacy ratio 12h settlement needs `PriceData.CacheCreation12hRatio`, derived from the already selected short-write ratio times 2, before receiving usage. Do not resolve it again during settlement.

## Exact merge conflicts and bounded resolutions

The legacy read-only `git merge-tree <base> <local> <rc47>` result has **two textual conflicts in the requested billing code domain**:

* `relay/helper/price.go`, local lines 124–130: retain `selected.CacheCreationRatio`, `selected.ImageRatio`, `selected.AudioRatio`, `selected.AudioCompletionRatio`; use new shared `hosttypes.ClaudeCacheCreation1hMultiplier` and `hosttypes.OpenAICacheCreation12hMultiplier`. Upstream's live `ratio_setting.Get*` alternatives would undo frozen catalog pricing. Retain local strict pre-consume conversions below this hunk.
* `service/text_quota.go`, local summary literal at lines 234–248: keep `UseTimeSeconds: useTimeSeconds` from the local `GPTImage2ResponseCompletedAt` boundary and add `CacheCreationRatio12h`. Selecting the upstream literal wholesale reintroduces preview-persistence time into billed/logged latency.

Other billing engine/normalization/builtin/log/Responses edits auto-merge. Auto-merge is insufficient for the following semantic boundaries.

## Patterns and semantic gaps

### 1. Adopt TTL and unsplit-Claude behavior explicitly

* `service/tiered_settle.go:26`: rc.47 replaces the direct Claude split with `relayconvert.NormalizeCacheCreationSplit(total, split5m, split1h)`. Total-only and partial-split write usage must retain the remainder as short writes, including in `len`. Existing code drops that remainder from both expression price and Claude context length.
* Long-lived writes omitted from the expression now fold into short writes at a fixed factor (OpenAI 12h 2x; Claude 1h 6/3.75x); if `cc` is omitted too, the weighted quantity folds into `p`. This is an intentional rc.47 behavior change authorized by the task, not a stored-expression rewrite. Preserve `len` as the unweighted full context length.
* For OpenAI sample prompt 12000, read 4000, total write 7000 (30m 3000 / 12h 4000), expression `p*10 + cr*1 + cc*12.5 + cc12h*25` must receive `p=1000, cc=3000, cc12h=4000, len=12000` and cost 151500 pre-/1M expression units (75750 quota at 500000 quota/USD). Omitting `cc12h` produces `cc=11000` and the same charge; omitting both write variables produces `p=12000` after the read exclusion and the extra 12h weighting.
* Image-cache logic at `service/tiered_settle.go:38–69` is independent: preserve valid `img_cr` removal from both `cr` and `img`, nil versus explicit zero, invalid-detail fallback, and final nonnegative P/C remainder. TTL weighting must not change image-cache opt-in or double-subtract overlapping categories.
* Upstream reuses an existing relaykit split helper containing unbounded integer subtraction/addition (`relaykit/relayconvert/internal/shared/claude/cache.go:3`). Extreme invalid signed splits can overflow (e.g. total 0 and both splits `math.MaxInt`). This is a latent helper issue newly reached directly from expression normalization. Add a bounded hostile-input case at the new boundary; if repairing it, coordinate the relaykit owner and normalize/saturate split arithmetic without changing valid unsplit/partial-split behavior. Final checked quota conversion alone does not repair a corrupted context length.

### 2. Preserve the local monetary and immutable-catalog contract

* `relay/helper/price.go:393–425` must continue freezing `selected.Money`, `ExpressionVersion`, source cost, USD cost and strict quota. `pkg/billingexpr/settle.go:12–58` must continue using snapshot `SourceCurrency/CNYPerUSD`; do not replace with upstream's USD-only conversion or resolve a new exchange rate at settlement.
* `relay/helper/catalog_pricing.go:111–126` rejects only a selected CNY identity that would use a compiled USD fallback. `IsBuiltinBillingExprFallback` in `setting/billing_setting/tiered_billing.go:127` uses configured presence, not numeric expression equality. An administrator's explicit expression identical to the new builtin is still its configured currency. Preserve explicit zero/empty override semantics, legacy ratio precedence, and per-provider identity.
* `service/text_quota.go:107–118` obtains tool prices from `info.ToolPrice`; `composeTieredTextQuota:201` adds tool surcharge exactly once after base expression normalization/grouping. Retain this and local Grok zero-price / original-currency audit additions while importing new fields.
* The upstream helper's new `usedVars["u"]` rejection protects ordinary requests from task-USD expressions. Retain that protection. Its adjacent shared-task candidate loop, however, calls **live** billing settings for **every** endpoint candidate. A blind import bypasses the local frozen pricing selection and can make an unselected provider invalidate the selected provider. Adapt with routing owner: enforce separate ordinary/token-vs-task pricing using the pinned selected attempt, validate the applicable provider, and capture any prerequisite pricing read inside the catalog boundary. Do not undo the existing provider snapshot semantics to reproduce upstream code literally.

### 3. Builtin and legacy pricing remain opt-in

* Import only GPT-6 Astra's new `cc12h` coefficients (25 standard / 50 above 272000 context); existing admin override maps are not migrated or overwritten. `getBillingMode/getBillingExpr` at `setting/billing_setting/tiered_billing.go:101–152` already preserve explicit modes, administrator expressions and existing legacy prices. No default ratio-table entry is needed.
* Model owner must import `CacheWriteOpenAITTL`, the configured-GPT-name condition and `cc12h` conversion lane from upstream `model/model_pricing_conversion.go`. Preserve explicit zero `CreateCacheRatio`, no-config generic fallback suppression, source-currency conversion and existing dry-run/CAS/atomic save behavior. This is preview/migration capability, not an instruction to bulk-migrate stored prices.
* Model owner should retain newer local prerequisite/override-draft/orphan-removal fixes in `model/model_pricing_config.go` and adopt the one-provider variant visibility change (`len(plugins)>0`, local line 355). Provider override discovery/removal must continue comparing the locked persisted configuration and preserve stale overrides.

### 4. AST/runtime/log completeness and saturation

* Add `cc12h` to compile/runtime/TokenParams together (`compile.go:127`, `run.go:74`, `types.go:26`); use the existing cached AST `UsedVarsByHash` mechanism at `compile.go:256–320`. Do not infer variable use from model names or expression text.
* `SmokeTestExpr` vectors in `setting/billing_setting/tiered_billing.go` currently have no nonzero cache-write dimension. A newly accepted `cc12h` expression with a negative coefficient can pass all existing save smoke vectors. A focused nonzero/boundary `CC12h` vector and deterministic save rejection case are appropriate for the introduced dimension; do not invent a second expression engine.
* `service/responses_usage.go:160` must use `src.InputTokensDetails.Clone()` (upstream fix) so TTL/cache-modality pointers are detached. DTO clone, `BillingUsage` clone and merge must preserve both cache-write and image-cache detail state; subsequent caller mutation must not change billing input.
* `service/log_info_generate.go:335–355` should add `cc12h` to the normalized billing token map and retain `appendTieredBillingMoneyAudit`. New raw log fields `cache_creation_tokens_12h` and `cache_creation_ratio_12h` belong with the existing raw total fields in text quota. Logs must distinguish raw write counts from weighted normalized expression quantities.
* Preserve `QuotaRoundStrict` / `QuotaFromFloatStrict` reservations, `QuotaRoundChecked` / `QuotaFromDecimalChecked` settlement and `noteQuotaClamp → attachQuotaSaturation` admin audit. `PriceData.AddOtherRatio` guards and private map remain unchanged. No bare float/int casts are needed for this upgrade.

## Deterministic verification needed (not executed)

Extend existing files, avoiding duplicate new fixtures:

1. **Imported rc.47 cases:** `TestBuildTieredTokenParams_ClaudeUnsplitCacheWrites`, `TestBuildTieredTokenParams_UnpricedLongTTLCacheWrites` in `service/tiered_settle_test.go`; `TestPostTextConsumeQuotaBillsOpenAI12hCacheWrites` in `service/text_quota_test.go`; new GPT-6 Astra builtin table rows in `setting/billing_setting/builtin_billing_test.go`.
2. **Normalization preservation:** existing `TestImageCacheBilling`, `TestSamePriceCacheReadsRemainInInput`, `TestBuildTieredTokenParams_Len_Claude`, and context boundary cases. Add only useful combined TTL/image-cache, omitted short/long category, split-only and bounded-invalid-TTL rows to the existing tables. Include negative/oversized 12h subset, missing split and explicit 0.
3. **Frozen money/identity:** retain `pkg/billingexpr/settle_currency_test.go` and helper `TestModelPriceHelperTieredUsesFrozenBillingModelCurrency`; extend one with a CNY `cc12h` expression and a post-capture rate/settings change. Existing `relay/catalog_pricing_capture_test.go` contains `TestCatalogRequestExpressionMoneyOverlap`, frozen tools, selected provider and `TestCatalogBuiltinFallbackCurrencyBoundary`; acceptance must preserve them. Add selected-vs-unselected ordinary/plugin pricing coverage with routing owner.
4. **Snapshots/logs:** extend `TestApplyResponsesUsageCopiesTokenDetails` in `service/responses_usage_test.go` with detached nested TTL/cache-modality mutation; retain partial-update BillingUsage tests. Existing 12h consume-log test must assert 7000 raw total, 4000 12h count and actual debit, and should run under the PostgreSQL service fixture.
5. **Safety:** retain `TestTryTieredSettleRecordsClampOnOverflow`, `TestModelPriceHelperTieredRejectsPreConsumeOverflow`, `TestAttachQuotaSaturationNestsUnderAdminInfo`, `pkg/billingexpr/settle_clamp_test.go`, and add TTL-specific huge usage/price only if needed to cover the new arithmetic. Strict reservations reject oversized charges; settled clamp stays nonnegative/admin-audited; errors keep the frozen reservation.
6. **Admin/catalog/migration:** existing builtin override tests, default published coefficient/context tests, pricing conversion matrix and selected builtin-currency tests remain mandatory. Extend conversion rows for configured/unconfigured GPT writes and explicit zero; no automatic legacy migration. Root/model owner should include existing atomic catalog and stale override regressions.

`deploy/tests/test_postgres_ci.py:20–64` already audits full `pkg/billingexpr` and `setting/billing_setting` packages as DB-independent. The service inventory at `deploy/tests/test_catalog_postgres_gate.py:100–148` currently selects task/refund tests only, so importing rc.47 tests alone does **not** exercise the new text/normalization behavior in acceptance. Add exact new tests and the bounded preservation tests to a required service rc.47 selection; both ordinary and race gates must verify no required test is skipped/absent. Helper selection at lines 312–336 already includes frozen currency, strict overflow and alias regressions; add any selected-provider guard regression there through the guarded fixture.

## Unsafe fixture inventory / execution boundary

* `service/task_billing_test.go:37–82`: package `TestMain` opens/migrates SQLite by default even for `-run` selecting a nominally pure test. Only `CATALOG_SYNC_POSTGRES_ONLY=1` uses `runServicePostgresTests` with strict loopback URL allowlist, owned disposable database, simple protocol and exact cleanup.
* `relay/helper/price_test.go:40–62`: package `TestMain` also opens SQLite by default. The PostgreSQL mode validates DSN and skips implicit SQLite, but each persistent test must explicitly call `setupHelperPricingPostgres`. Frozen-currency test lines 187–193 has its own SQLite fallback when the mode is absent.
* `service/text_quota_test.go:34` `TestFixedPriceBillingDatabaseMatrix` explicitly enumerates SQLite/MySQL/PostgreSQL. Do not invoke the parent/full service suite; if any matrix child is required, select its audited PostgreSQL child only.
* Other explicit SQLite service fixtures occur in `channel_select_auto_groups_test.go`, `auth_session_test.go`, `file_test.go`, `relay_error_test.go`, `task_billing_reconcile_test.go`, and `user_notify_test.go`. Do not broaden selection to these while validating pure billing cases.
* Tests under `pkg/billingexpr/` and `setting/billing_setting/` contain no `TestMain`, `gorm.Open`, `AutoMigrate`, SQLite or MySQL setup, consistent with the audited pure allowlist. Their imports of `service` do not compile service's test files/TestMain into this package.
* New upstream 12h consume test uses `truncate`, `seedUser`, `seedChannel`, `getLastLog` and calls `gin.SetMode`. It needs the PostgreSQL package fixture. Current service tests are not parallel; preserve initialization conventions and do not introduce parallel global Gin/settings changes.

No completion, test-pass or DB compatibility claim is made by this research report. Root owns all later implementation authorization, index/commits/refs and development acceptance.
