# PostgreSQL CI Integration Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Task owners are disjoint; project AGENTS requires parallel implementation for L tasks.

**Goal:** Repair the observed failed deployment gate without disabling catalog consistency protections, then verify and push develop.
**Architecture:** Preserve engine-specific legacy tests outside the PostgreSQL deployment gate. Run exact audited mandatory PostgreSQL regressions including every failed business test in run 38047800232, plus complete database-independent package suites and relaykit. Do not pretend the historical broad cross-engine suite passes. Fixtures use real bootstrap, not fabricated ready flags. The confirmed first-heartbeat clock-boundary defect additionally requires the bounded production timestamp repair below.
**Tech Stack:** Go, PostgreSQL 15, Python/shell CI runner, GitHub Actions.
**Spec:** The human-approved repair direction in the preceding diagnosis; evidence is .ccg/tasks/archive/2026-10/diagnose-catalog-deploy-failure/review.md (repository-relative). No new feature or API contract.

## Global Constraints

- PostgreSQL ONLY. No SQLite/MySQL execution, including hidden TestMain or in-test fixtures.
- No external antigravity/Claude executor. Native Codex agents only; no child spawning.
- No production consistency/auth/billing bypass; preserve all meaningful assertions of failed business regressions.
- No live data writes, manual deployment, forced push, broad git staging or unrelated user file changes.
- Missing DSN, empty selections, required skips, missing required tests and test failures must fail the gate.
- Use owned disposable loopback PostgreSQL databases and actual bootstrap/recovery; no fabricated runtime-ready state.
- Local and CI use the same acceptance entry point. Normal and race gates must execute required PostgreSQL children.
- Existing broad make test stays available for legacy development, not invoked by PostgreSQL deployment acceptance.

## Review Focus

1. Writer rejection before callback must not hang a concurrent test (Task 3).
2. Runtime-only plugins must not be accepted by weakening persisted/effective parity (Task 1).
3. PostgreSQL-only helper TestMain must reject malformed mode/DSN before opening any database (Task 2).
4. Existing failures must map to executable mandatory cases, not disappear under selection changes (Task 3).
5. Complete database-independent suites must retain tests and report any legacy suites outside acceptance (Task 3).

### Task 1: Controller failed regression fixtures

**Ownership:** controller/model_management_test.go, billing_option_test.go, model_list_test.go, model_meta_catalog_test.go, vendor_meta_pricing_test.go; if necessary extend existing controller/catalog_image_pricing_test.go as shared PG fixture (not duplicate a new fixture file). Report any extra path before editing.
**Interfaces:** Existing CATALOG_SYNC_POSTGRES_ONLY=1 flag selects fail-closed PostgreSQL fixture branch. Reuse catalogImagePostgres or its real bootstrap pattern. Expose exact test names and PG matrix selectors for root-owned gate inventory.

- [x] Reproduce selected existing failures on real PostgreSQL, never execute legacy driver branches.
- [x] Add explicit guarded PG branches to the owning fixtures; for matrices select only postgres children via gate regex.
- [x] Supply complete reference tables and actual option bootstrap/recovery, persisting exact matching plugin records where tests need catalog mutations. Preserve deliberately invalid tests and all expectations.
- [x] Run focused normal/race selections covering all amended tests and report commands/output and exact paths.
- [x] Commit only owned paths, self-review and deliver brief report to .ccg/tasks/fix-catalog-postgres-ci/controller-report.md.

### Task 2: Relay and helper failed regression fixtures

**Ownership:** relay/relay_task_test.go, relay/relay_task_retry_billing_test.go, relay/catalog_pricing_capture_test.go, relay/helper/price_test.go, relay/helper/price_identity_test.go, relay/helper/molii_grok_price_test.go. Extend existing files, no production source changes. Report extra paths before editing.
**Interfaces:** Explicit CATALOG_SYNC_POSTGRES_ONLY=1; task DSN. Existing catalogPricingPostgres is an owned real PostgreSQL fixture. Helper TestMain must avoid any legacy initialization whenever mode is nonempty and validate mode/DSN fail-closed.

- [x] Reproduce original failing assertions with safe PG fixtures before fixes; do not run forbidden legacy TestMain.
- [x] Initialize complete schema and actual runtime before helper/relay pricing capture; preserve original business assertions and persisted registry parity.
- [x] Make each amended fixture restore settings/DB/registry or isolate each exact selection process. No safety bypass, no production fake state.
- [x] Run exact selected normal/race tests including original MoliiGrok anchors and price identity failures.
- [x] Commit only owned paths and report to .ccg/tasks/fix-catalog-postgres-ci/relay-report.md.

### Task 3: Model concurrency and CI acceptance wiring

**Ownership:** model/catalog_sync_test.go, deploy/tests/catalog_postgres_gate.sh, deploy/tests/test_catalog_postgres_gate.py, .github/workflows/deploy.yml, .github/workflows/ci.yml, makefile; one new deploy/tests/test_postgres_ci.py runner/test file only if required. Root owns task records.

Incident-driven additions (root-owned, test-only): deploy/tests/deploy_test.sh updates the old broad-race workflow contract to exact PostgreSQL normal/race commands. The first complete race run exposed concurrent gin.SetMode writes in relay/channel/api_request_test.go and relay/channel/gemini/relay_gemini_usage_test.go. Set mode once before m.Run and remove per-test writes in those files plus relay/channel/api_request_redirect_test.go, relay/channel/api_request_getbody_test.go, and relay/channel/gemini/relay_responses_test.go. Preserve parallelism and every assertion.

The next complete race run exposed the same pre-existing global-mode race in relay/channel/minimax/adaptor_test.go; include this exact sixth file in the same bounded test-only repair. Static audit of all41 full packages checks remaining parallel tests against mode-writing helpers; no additional parallel mode writers are authorized without evidence.
**Interfaces:** Consume exact regression inventories from Tasks 1–2. Preserve existing catalog groups and explicit safe model TestMain. CI calls make test-postgres and make test-postgres-race (targets must use the existing mandatory gate and additional database-independent suites without legacy broad suites).

- [x] Extend behavioral runner tests with missing required regressions, skipped/empty cases and unsafe suite selection RED evidence.
- [x] Bound TestCatalogSyncSourceConcurrentMutation waits with context cancellation and writer-error propagation, always release blocked callback on failure.
- [x] Audit original failed top-level tests from run 38047800232 against mandatory PG inventory. All failures must be covered unchanged or by explicit equivalent migration, no silent omission.
- [x] Run whole database-independent packages with no arbitrary test regex; audit direct database setup and TestMain before allowing each package. List excluded legacy packages and rationale in evidence.
- [x] Route both deploy and PR workflows through identical PostgreSQL targets, eliminating broad legacy lifecycle race invocation. Retain frontend/vet/build/deploy contracts and protections.
- [x] Verify script unit tests and relevant model PG normal/race selections; self-review and commit only owned paths.

### Task 4: Independent review, full validation and push

Incident-driven production correction: final race acceptance exposed the real first-heartbeat creation ordering defect, deterministically reproduced by delaying the actual GORM create callback across a Unix second. Under the human's explicit authorization to fix this incident, extend ownership to model/system_instance.go, model/catalog_sync_startup_test.go and controller/catalog_sync_test.go. Initialize a new row's CreatedAt from the same resolved heartbeat timestamp as LastSeenAt/UpdatedAt; keep conflict-update creation identity and all strict single-instance eligibility conditions unchanged. Add creation/update/default-timestamp regressions and require them in normal/race inventory. This is not permission to change pricing, billing, auth policy or catalog guards; no live data repair is authorized.

- [x] Review each owned task diff for spec compliance and quality; fix material findings without weakening requirements.
- [x] Run exact complete normal/race PostgreSQL CI acceptance, deployment contracts, Go vet/build root and relaykit, relevant frontend checks if changed. Required cases have zero skips.
- [x] Record limits: PostgreSQL-only acceptance is not the historical cross-engine full-suite result; no claim all old fixtures were migrated.
- [ ] Independent whole-change review with no external executors; archive task and exact evidence.
- [ ] Fetch origin/develop, verify nonforce fast-forward and only expected commits; push HEAD:refs/heads/develop. Inspect triggered workflow and report actual CI/deployment outcome.

## Decisions / execution ledger

- Ruling: Repair failing regression fixtures and supported PG acceptance, not wholesale migration of ~98 legacy database test files — keeps incident scope bounded and obeys the human's PostgreSQL-only restriction — cost if wrong: older unrelated database regressions outside explicit PG inventory remain unverified and must not be claimed passing.
- Ruling: Existing worktree upgrade/rc40 is reused; user files remain untouched — it holds the reviewed feature and follow-up history — cost if wrong: any newly concurrent edits must be detected before staging/push.
- Ruling: Native parallel task ownership overrides SDD's generic sequential implementation prescription per explicit project AGENTS L-task policy; external dual-model instructions remain overridden by the human's explicit ban — cost if wrong: shared-fixture conflicts require rework, avoided through disjoint ownership.
- Review extension: reject every non-sslmode DSN query parameter in the three reused disposable controller/relay/helper fixtures before any connection. Same-name dbname/database overrides can otherwise survive randomized URL-path rewriting and target the original database. Pure parser RED/GREEN coverage is required; no test probe may connect using a dangerous override.
- Coverage extension: retain all other newly converted controller/relay/helper regressions in mandatory groups as well as all104 originally failed names, rather than only executing them once locally.

Preflight: Tasks 1/2 share only mode/DSN contract, no files. Task 3 consumes their inventories after delivery; gate edits stay root-owned. Task 4 consumes all prior commits/evidence. Each task's declared tests match its fixture scope; no production bypass is authorized.
