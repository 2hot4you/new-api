# rc.47 Compatibility Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. User AGENTS L+ requires parallel disjoint ownership; root alone owns shared index/refs. Never spawn children from workers.

**Goal:** Integrate official rc.47 with Molii custom contracts and PostgreSQL-only acceptance, deploy only to develop, and make future main releases exclude iXiaozu by default.

**Architecture:** Pinned full upstream merge into isolated upgrade/rc47 from e32fdcda17c5c48d26a461f6e4fd6c2ce862aa11. Resolve domain conflicts with existing behavioral regressions, retaining immutable billing/catalog/polling contracts; use a centralized environment allowlist and fail-closed maintenance guards.

**Tech Stack:** Go1.26.10 root/tokenkit, independent relaykit, React19/Bun1.4.4, PostgreSQL15, Redis7, GitHub Actions.

**Spec:** design.md and requirements.md in this task directory.

## Global Constraints

- PostgreSQL ONLY; no root go test ./... or historical make test, no SQLite/MySQL execution. Use GOWORK=off and audited make test-postgres/test-postgres-race.
- No main merge or push. No production dispatch, data/catalog/pricing/credential writes, or paid provider calls. Never contact or mutate iXiaozu during this task, even if current wall time is after20:00.
- Actual development branch is develop; dev does not exist. Preserve divergent local main/develop, untracked user files, original workspaces and current branch snapshots. No force push/reset or broad staging.
- No antigravity/Claude executors. Native Codex research/review only. Workers are not alone, cannot revert others or spawn, and cannot stage/commit/change refs.
- Preserve Molii persisted channel IDs, immutable expression/currency/rate/provider/source-hash facts, durable once-only billing, native/task/image routing, calendar and original-currency display, Seedance authority, expected_version/metadata prerequisites and orphan-removal intent.
- Root owns Docker/go-module/workflow/CI inventory integration, task ledger and all Git. No new documentation under docs/; task artifacts stay in .ccg/tasks. Only one consolidated archive commit after completion.

## Review Focus

- Mixed-cache12h/unsplit-Claude/explicit0 under CNY snapshot and altered live settings must retain correct charge with no overlap or late lookup.
- A shared ordinary/plugin model must choose the actual channel by priority/weight without reclassifying Molii native channel IDs, routing retries across execution kinds, leaking headers or reinterpreting a task quote as token price.
-24–72h video jobs stay live until their effective deadline; bounded merged Kling duration and repeated success/failure preserve once-only charge/refund.
- A same-price upstream row may be adjusted, but stale confirmation, source-currency changes, protected Seedance rows, orphan drafts or missing metadata cannot be silently persisted as a different price.
- Main/bulk app AND documentation deployments must exclude iXiaozu; no approval/daytime/expired queue/server-call bypass may pull/login/upload/restart/write there.

## Task 1: Baseline, merged-source coordination and environment safety (root)

**Files:** deploy/** including existing tests; .github/workflows/**; docs-site/deploy/** and scripts/docs-workflow-contract.test.ts for shared maintenance protection; Dockerfile; root/relaykit/tokenkit module manifests if upstream requires; common/init.go; task records and all index/ref operations. Controller currency bridge delegated to Task4. Do not edit worker-owned source.
**Interfaces:** Existing resolve(env) target/source contract; add explicit standalone iXiaozu approval/date plus real Shanghai maintenance checks reused by application/documentation release steps and server entrypoint. Main default/bulk IDs exactly three eligible sites. Preserve development source_ref, backup and repeated-startup inputs.

- [x] Confirm origin refs, current deployments/CI, prior tag and full adjacent-tag diff; run baseline deployment contracts and pure billing/plugin tests. Reuse isolated worktree, protect existing files.
- [x] Merge pinned v1.0.0-rc.47 with --no-commit --no-ff after baseline; give exact ownership lists. No blanket ours/theirs.
- [x] Extend existing deploy target/CLI/mocked side-effect tests before policy implementation; observe expected failure for three-site default and blocked unapproved/daytime/stale iXiaozu. Include19:59:59,20:00:00,23:59:59 and next-day00:00; deny before any environment contact or write.
- [x] Implement/test allowlist and local/remote maintenance gates, including documentation workflow's separate matrix; update existing deploy README policy, no new docs/ files. Do not invoke protected environment.
- [x] Coordinate Bun1.4.4 Docker/CI pins using verified official digest; preserve branding, calendar, catalog-sync binary and all three independent modules. Retain explicit PostgreSQL-only inventories; incorporate exact new regression selections after owners report.

## Task 2: Cache and immutable billing compatibility (billing worker)

**Files:** pkg/billingexpr/**; setting/billing_setting/**; types/price_data.go; relay/helper/{price,catalog_pricing}.go; relay/common/pricing_selection.go; relay/relay_task.go and existing tests; service/{text_quota,tiered_settle,log_info_generate,responses_usage}.go and existing relevant tests; model/model_pricing_{conversion,config}.go and corresponding existing tests; two dependency initializer lines in model/catalog_sync_{candidate,validate}.go. No frontend or deployment files.
**Interfaces:** relaykit new DTO cache facts supplied by Task3. Preserve selectedModelPricing/PricingSelection and frozen MoneyContext. Ordinary-task expression guard must use captured candidate facts; ask root before extending capture files/selection API. cc12h is opt-in and must survive normalization/legacy preview/logging.

- [x] Read billing rules/expr docs and research report. Add bounded behavior cases to existing files before implementing gaps; use upstream tests as regression input without weakening local ones.
- [x] Resolve price/text conflicts retaining frozen selection and CNY money; review auto-merged cache12h/Claude normalization and builtin/preconsume/log semantics. No administrator quote rewrites.
- [x] Run safe pure cache tests; run exact service/helper PG selectors with root-provided DSN, never broad service/helper suites. Report exact names, RED/GREEN and source identity for required inventory.

## Task 3: Channel routing, video settlement and stream compatibility (routing worker)

**Files:** middleware/{task_plugin,task_plugin_test,distributor}.go; model/channel_constraint.go; dto/channel_constraints.go; pkg/jsplugin/{routing,registry}.go; relay/channel/task/jsplugin/**; service/task_polling.go and existing tests; plugins/tasks/kling/plugin.js and existing plugin tests; relay/helper/{common,stream_scanner,stream_scanner_test,stream_result,model_modifier}.go; relay/channel/openai/{relay_responses,relay_responses_billing_test,relay_responses_compact}.go; relay/responses_websocket.go; router/task-plugin-protocol-router.go; independent relaykit/**; related existing request/route/relay tests only after coordination. Not relay/helper/price.go or service/text_quota.go.
**Interfaces:** Produce upstream DTO/cache fact clone/merge fields for Task2, without host imports in relaykit. Existing Molii channel enums and native driver bridge stay authoritative. Contract docs updated from upstream; public source SHA/generation lease retained.

- [x] Read plugin API/rules and research. Resolve conflicts for ordinary/plugin priority/weight, initial winner and same-kind retry restrictions; preserve native channel/type semantics.
- [x] Preserve local multiplier-bound checks even where upstream deletes broad validation; adopt bounded Kling merged-duration, native multipart and sanitized client headers. Add/extend observable cases before new fixes.
- [x] Adopt72h default while explicit timeout wins, durable once-only terminal/refund, slow large-event delivery and completed Responses usage; keep local proxy/image/funding/poll safeguards.
- [x] Run audited pure modules/plugins/converters and exact PG service/middleware/relay selectors; report unsafe DB TestMain and required new selectors to root. No broad DB suites.

## Task 4: Pricing sync, cache display and frontend preservation (frontend worker)

**Files:** web/** except user untracked * 2.* backups; delegated controller/ratio_sync.go, ratio_sync_test.go and ratio_config.go. No Dockerfile/workflows or other root Go changes.
**Interfaces:** Upstream include_unchanged and keep/multiply/divide price adjustment; existing source_currency/expected_version/Seedance authority and removal-only intent. Coordinate cc12h names/normalization with Task2 and Bun1.4.4 pin with root.

- [x] Read root/web AGENTS, project shadcn skill, required auth guidance for credential forms, matching shared UI and research before implementation.
- [x] Reconcile pricing/sync/editor/locales conflicts without losing whole-batch preflight, rejected drafts, special Seedance menu, stale confirmation invalidation, source CNY/USD, vendor/date and single-tier semantics. Add focused regressions for actual uncovered cases before code fixes.
- [x] Include local quotation/calendar consumers in cc12h support; preserve original number/Intl helpers and translations. Preserve stale-drawer fixes and local override-dialog behavior.
- [x] Use pinned Bun1.4.4 frozen installation, built-in check/typecheck, full frontend tests/i18n, changed-source lint/format and three eligible brand builds. Report any baseline failure explicitly. Do not alter user backups.

## Task 5: Reviewed acceptance and development-only rollout (root + independent native reviewers)

**Files:** bounded fixes per ownership, exact CI test inventories and consolidated task reports. No production mutations.
**Interfaces:** One immutable candidate source and actual develop workflow; app and documentation changes may trigger separate development workflows, all must be green before final handoff.

- [x] Fresh root/relaykit/tokenkit independent build/vet, complete audited normal/race suites plus PostgreSQL inventory with no missing/empty/skipped requirements. Test representative fresh/upgrade/repeated PostgreSQL startup/main/log preservation if affected. Actual task-owned DB only.
- [x] Independent native billing/catalog and routing/security/deploy reviews against full base-to-worktree diff; fix confirmed Important/Critical with observed regression and scoped re-review. No duplicated expensive full tests by reviewers.
- [x] Capture public dev status, prices/vendors/currencies/rate baseline. Verify no main/ixiaozu action or reference update. Commit only tested source; fast-forward actual develop directly with explicit refspec after local acceptance and ref freshness check, no force or automatic tags. Consolidated task archive follows deployment acceptance once.
- [x] Observe all relevant development CI/CD to completion, accepted immutable version, health and repeated startup; compare public configuration and no charged upstream requests. No early success report, no production promotion.
- [x] Archive task once, update relevant learned spec only if genuinely useful; task-only closeout must not trigger another application release. Preserve original/reused workspaces and user files. Final report identifies develop SHA, green runs, preserved custom contracts, tested environment isolation and still-protected main.
