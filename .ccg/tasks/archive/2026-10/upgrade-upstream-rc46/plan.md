# new-api rc.46 Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement task-by-task. User AGENTS.md explicitly requires parallel, disjoint-file implementation for L tasks; this overrides the skill's sequential dispatch default. No worker may spawn or commit shared-index changes.

**Goal:** Integrate official rc.46 into current customized develop, preserve channel/billing/catalog contracts, and verify PostgreSQL-only acceptance before dev deployment.

**Architecture:** A real local merge of pinned upstream tag into isolated upgrade/rc46, with domain-owned resolution and behavioral regression tests. Existing persisted channel IDs and catalog generation/money snapshots remain authoritative. Root coordinates the single shared index and commit history; no blanket ours/theirs resolution or deleting user files.

**Tech Stack:** Go1.26.10 root + independent relaykit/tokenkit, moejs, React/TypeScript/Bun, PostgreSQL15/Redis7.

**Spec:** ../archive/2026-10/audit-upstream-rc46/assessment.md (user-approved upgrade flow and preservation constraints).

**Execution outcome:** Tasks1–4 and the requested development portion of Task5 completed; run39762321755 accepted the immutable source and dev deployment, then develop fast-forwarded to that same commit. Additional ordinary push run39767173903 is still in progress and must pass before later separately authorized production work. Checkboxes below preserve the original plan; final evidence is in verification.md/deployment-result.md. The reused checkout contains user files and is deliberately preserved.

## Global Constraints

- PostgreSQL ONLY. Keep make test-postgres and make test-postgres-race; do not invoke make test or root go test ./... that initializes other engines. Root/relaykit/tokenkit compile/vet and audited pure suites are allowed.
- No antigravity/Claude external executors. Native Codex workers only, fork_turns=none, no child spawning. Preserve user untracked duplicate files and divergent local main.
- Preserve persisted channel IDs, CNY/USD and frozen conversion context, calendar, Seedance native routing, Grok/image proxy, single-tier semantics, vendor/date default ordering, managed catalog pin/digest and once-only billing.
- Import upstream auth/scoped tokens with server-side authorization and single-use step-up; follow OWASP ASVS5.0.0 + Authentication/Session/CSRF guidance. Never weaken proof checks to make a fixture pass.
- New upstream builtins never overwrite administrator expression/currency/metadata. No catalog apply, live-price edits or credential provisioning as a deployment step.
- Local plan/task files are under .ccg/tasks, not docs/. All commits are exact-path staged; root alone stages/resolves/commits. No worker edits outside its assigned domain without coordination.

## Review Focus

1. Same numeric CNY/USD quote is not same price; monetary sorting normalizes units/currency while source display stays original and settlement snapshot remains frozen.
2. Moejs engine changes cannot lose exact compiled-source SHA or generation leases; runtime pin and mutation protections stay enforced.
3. Task retry moves to a more expensive provider/group: reprice selected frozen facts, reserve difference before sending, no insufficient-balance upstream call.
4. Migrated dashboard credentials, user edits and catalog proofs retain expiry, scope, actor/session/version binding and single use; no API-key-vs-PAT confusion.
5. Startup schema additions coexist with catalog recovery and fail-closed options uniqueness; repeat PostgreSQL migration twice without data loss and cover independent log DB.

## Task 1: Baseline, integration and shared-index coordination (root)

**Files:** task records; all remaining conflict paths outside Tasks2–4; especially model/{pricing,model_pricing_config,option,option_primary_key_migration,log}.go, relay/{relay_task,request_billing}.go, service/{text_quota,task_billing,funding_source,group}.go and focused existing tests.
**Interfaces:** pinned tag/base; unresolved path list distributed below. Preserve CaptureTaskPricing/PricingSelection money + plugin identity contracts; import upstream logger/common.Message signatures across callers. Root owns all git index/ref writes.

- [ ] Run audited pure baseline packages and record outcome before merge; inspect current dev CI as supporting not substitute evidence.
- [ ] Merge pinned rc.46 locally with --no-commit --no-ff. Record unresolved paths; do not touch user untracked data.
- [ ] Preserve existing behavioral tests first; extend focused tables for missed currency/retry cases before changing their production branch. Do not duplicate tests by layer or create source-text tests.
- [ ] Combine root pricing/task/log code with new upstream semantics. Compile/vet all modules with GOWORK=off; share exact dependency errors with owners.
- [ ] Confirmed integration fix: an exact CNY metadata model with no saved quote must not activate a newly added USD builtin as CNY. Fail closed for that actually selected fallback; keep it unpriced in pricing/editor effective defaults until explicit save. Detect configured provenance by map presence, never numerical equality. Preserve USD defaults, explicit CNY quotes and provider overrides. Plugin implementer owns the bounded setting accessor/catalog capture/pricing snapshot/public pricing fix plus one existing integration test file; root owns review/index and mandatory inventory addition.

## Task 2: Frontend, build and CI integration (frontend-ops worker)

**Files:** web/** (not user untracked * 2.* files); Dockerfile, Dockerfile.dev; go.mod/go.sum, go.work if present; .github/workflows/**; makefile; deploy/tests/{test_postgres_ci.py,test_catalog_postgres_gate.py,catalog_postgres_gate.sh,test_*.py} only as needed for module inventory. No backend business code.
**Interfaces:** backend keeps source_currency/billing fields and new upstream endpoints; tokenkit local module path must be copied before module download. Original per-site build arguments/immutable deploy matrix remain.

- [ ] Read web/AGENTS and existing required shadcn skill; read frontend report and root billing rules.
- [ ] Resolve web conflicts preserving local directory/vendor/release ordering, single tier, raw currency and history while adopting upstream features. Add/extend existing mixed-CNY/USD sorting test before adaptation and observe RED/GREEN.
- [ ] Upgrade root toolchain1.26.10, module packaging/tokenkit, lockfile and CI independent checks; keep existing PG-only gates, branding/calendar/catalog-sync binary and version flags.
- [ ] Run frozen Bun install, typecheck, focused then complete frontend suite/i18n, brand builds; report commands/results and any blockers. Do not commit/stage shared index.

## Task 3: Plugin runtime, channel and polling integration (plugin worker)

**Files:** pkg/jsplugin/**; plugins/**; relay/channel/task/jsplugin/**; relay/common/tool_usage.go and test; model/channel.go, model/channel_cache.go, model/task_model_alias.go and tests; controller/channel-billing.go,channel.go,channel_billing_test.go,plugin_protocol_image.go,video_proxy.go; middleware/task_plugin.go; service/task_polling.go and test. No relay/relay_task.go, no web/auth files.
**Interfaces:** keep Engine.sourceHash + registry TryPinGeneration lease behavior; custom channel constants unchanged; task snapshots live in root pricing code. New mojo utilities/adaptor contract must match docs/plugin-api v1 already merged from upstream.

- [ ] Read plugin API/rules and backend assessment before edits. Preserve registry pin tests as regression gate; add narrow tests only for uncovered behavior and verify failure before fix.
- [ ] Resolve engine/adaptor/task pipeline conflicts: moejs values/clone ownership, source hash, native/-1 durations, credential redirect/header policy, output usage and durable local flows.
- [ ] Run pkg/jsplugin and plugin/adaptor pure tests; report root compilation dependencies without editing other owners' files. Do not stage/commit.

## Task 4: Authentication, admin endpoints and startup integration (auth worker)

**Files:** controller/{token,token_test,user,user_manage_test,wechat,security_enrollment_test,relay,relay_error_log_test,model_meta,model_sync,perf_metrics,group,log}.go; middleware/{auth,rate-limit,turnstile-check}.go; router/api-router.go; service/security_verification.go; model/{user,main}.go and related auth/migration tests; main.go; pkg/perf_metrics/**; common/redis.go; relay/channel/openai/relay-openai.go,relay_responses.go; relay/channel/gemini/relay_gemini_usage_test.go; relaykit/relayconvert/internal/oai_chat/to_oai_responses_stream_resp.go. Coordinate additional caller fixes first.
**Interfaces:** upstream scoped PAT/service scopes and common.Message; preserve catalog-sensitive endpoint guards, custom routes/proxying, startup recovery and migration ordering. New UserAccessToken table + legacy retirement option, quota cache columns and audit index coexist with local models.

- [ ] Read ASVS5.0.0 and applicable official OWASP cheat sheets before auth edits; record applicable security requirements and tests.
- [ ] Resolve assigned auth/admin/startup/caller conflicts without dropping custom proof/version/role guards or routes. Preserve runtime PostgreSQL simple protocol and local migration ordering.
- [ ] Run audited auth pure tests and focused PG fixtures where configured, and relaykit independent compile/tests as safe. Report unsafe fixtures rather than switching to SQLite/MySQL. Do not stage/commit.

## Task 5: Integration verification, review and dev rollout (root + fresh reviewers)

**Files:** only bounded integration fixes in assigned ownership; focused tests; deployment evidence and task records.

- [ ] Require all conflict markers removed and all three reports; run root/relaykit/tokenkit independent vet/build, frontend gates, Python/shell deploy contracts, and isolated PostgreSQL normal/race no-skip inventories including new auth/migrations/tokenkit tests.
- [ ] Verify clean DB initialization, representative old-schema data upgrade and repeated startup with main/log PostgreSQL; preserve balances/expressions/currency/catalog state and in-flight task fields. Never use live DB for tests.
- [ ] Native independent code/security review, fix substantive findings with regressions, scoped re-review. Capture entire base-to-head diff evidence and exact test counts/skips.
- [ ] Commit integration with both parents; only after acceptance push upgrade branch or integrate develop per approved workflow, monitor actual dev CI/deploy and validate. Do not call production updated while run is queued.
- [ ] Production promotion and data sync remain separate gates: main/four-site release only after dev acceptance and clear release authority; no automatic catalog apply. Archive task only when requested scope is actually complete; otherwise record honest remaining work.
