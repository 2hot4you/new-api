# Task 8 independent spec and quality gate

## Spec Compliance

- ✅ Spec compliant for this task's approved implementation and verification scope. Reviewed BASE `0d8994a024bdd45fc22a0d19b2f6337674eb9096` → HEAD `0dbc5053b4ecb357d46b69c04c78c756a0c9fb20`, using the supplied 1,705-line diff in four bounded passes. The diff contains exactly the 15 approved paths: both workflows; both new Go integration test files; five environment examples; the shell gate, Python gate/validator and deployment test; and the three model test files. No production Go implementation, dependencies, user ` 2` files, billing formula, actual runtime secret or frontend implementation appears in this diff.
- ✅ `model/task_cas_test.go:20` branches before the legacy initializer on any nonempty opt-in; `model/catalog_sync_postgres_gate_test.go:25` rejects values other than literal `1`, validates a literal-loopback PostgreSQL URL with the narrow query contract, connects/pings/closes without migration, and establishes PostgreSQL flags before `m.Run`. The three actual negative-preflight logs each contain the expected guard diagnostic at line 1. No legacy-mode test was run by this reviewer.
- ✅ `model/catalog_sync_test.go:812`, `model/catalog_sync_test.go:2443`, `model/catalog_sync_test.go:2511`, and `model/catalog_sync_test.go:2618` introduce only the approved helper calls. `model/catalog_sync_postgres_gate_test.go:176` restricts the legacy prerequisite by callsite and approved test names, uses actual migration/heartbeat/recovery, and preserves globals. Its engine adapter at line 222 rejects unaudited non-PostgreSQL fixtures. Existing assertions and actor/session creation remain unchanged in the supplied diff.
- ✅ `deploy/tests/catalog_postgres_gate.sh:5` rejects absent DSN; `deploy/tests/test_catalog_postgres_gate.py:1` contains the explicit inventory, required parent/PG-child pass verification, fail/skip/wrong-engine rejection and normal/race runner. Every model invocation inherits literal opt-in `1`; test-only bridge tags are attached only to controller test commands. The preserved raw gate summaries each report 824 tests/subtests across 27 exact selections, with zero skips; this reviewer did not rerun tests.
- ✅ `model/catalog_sync_postgres_gate_test.go:58` tests the complete reference transaction using a restricted LOGIN role over 10,000 rows in each of three history tables, including 15,000 active rows. It checks an actual write is absent after cancellation, original row counts remain, and later ordinary/fenced writes plus registry/runtime-lock release succeed. Normal raw evidence reports terminal scan 28.508708 ms, mixed scan 46.772375 ms and forced timeout 2.001242834 s; these are one local scale, not production sizing.
- ✅ `catalog_postgres_upgrade_test.go:21` explicitly labels its Model/Vendor schema as representative, and line 111 runs two isolated subprocess boots. Assertions preserve original row IDs, description, vendor association, creation timestamp, indexes and empty pending operation. Raw boot evidence shows both child runs and parent passed. Current full fresh/startup cases remain explicitly selected by the gate; this does not establish every released schema or an old-binary upgrade.
- ✅ `controller/catalog_submit_integration_test.go:28` accurately bounds the integration: actual submission, persistence, wallet and terminal consumers with substituted local upstream and supplied request identity. Both insertion branches preserve the selected expression through concurrent publication; the native Grok test checks selected pricing and detached snapshot. No claim of full routing/authentication middleware coverage follows from these new tests alone.
- ✅ `.github/workflows/ci.yml:19` and `.github/workflows/deploy.yml:78` contain the separately approved finite 60-minute deadlines. Existing unrelated jobs and DSNs are preserved; the new gates are PostgreSQL only. `.github/workflows/deploy.yml:147` adds frontend tests/i18n. All five environment examples add disabled defaults and source/target instructions at their appended catalog blocks (development/claudeye/model-claudeye/molii:14; ixiaozu:17), without real credentials. `deploy/tests/deploy_test.sh:146` supplies fixture-only catalog settings; line 219 checks runtime-file preservation and credential exclusion for all targets.
- ⚠️ Cross-task invariants—fixed dev origin, 15-second/10-MiB transport limits, ten-minute plans, managed deletion/ownership, permission/proof enforcement, whole-batch conflicts, singleton deployment eligibility, common JSON wrapping and coherent production billing selection—are not newly implemented by this diff. Their named regression cases are selected, but this task gate does not independently re-review every underlying implementation. Root must address them in the scheduled whole-feature review.
- ⚠️ The strict-wire map-null limitation, known Grok/Seedance billing arithmetic limitations and Task 7 restore-copy/accessibility/module-size concerns are disclosed in `catalog-task-8-report.md` under its deferred-limitations section. They remain cross-task review inputs, not independently confirmed fixes or automatic waivers from this gate. No broader code crawl was performed to pre-judge them.
- ⚠️ Cloud CI execution, unknown production history scale, actual target deployment, first production apply, final CCG completion/archive and whole-feature acceptance are outside this task gate. Representative upgrade coverage is deliberately narrower than a complete released database. Existing unrelated legacy CI commands were preserved, not executed or accepted here; no SQLite/MySQL or cross-engine acceptance is claimed.

## Strengths

- `model/catalog_sync_postgres_gate_test.go:25` fixes the significant pre-selection safety boundary directly: a selected model test cannot reach the historical initializer through malformed nonempty opt-in. The generic guard diagnostics avoid leaking credentials.
- `deploy/tests/test_catalog_postgres_gate.py:1` treats a zero-selected or skipped run as failure and requires actual PostgreSQL matrix-child passes. The large file is predominantly a deliberate auditable test inventory, not a new production abstraction.
- `model/catalog_sync_postgres_gate_test.go:58` couples load measurements with restricted privileges, real cancellation, rollback and subsequent progress. It proves more than timing alone and does not weaken the reference fence.
- `controller/catalog_submit_integration_test.go:32` exercises real persistence during publication changes and checks wallet/idempotency effects; `catalog_postgres_upgrade_test.go:65` uses actual startup twice with explicit fixture limitations. These close concrete gaps in the prior constructed-context evidence.
- `deploy/tests/deploy_test.sh:219` protects the whole runtime file byte-for-byte and separately checks metadata/log exposure, while the examples preserve disabled defaults and distinguish deployment from synchronization authority.

## Issues

### Critical (Must Fix)

- None found within the reviewed task scope.

### Important (Should Fix)

- None found within the reviewed task scope.

### Minor (Nice to Have)

- Retained diagnostic noise is not pristine test output. `/var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/catalog-postgres-normal-r2jve9ny/controller-catalog_sync_http_test.jsonl:94` contains the expected unknown-actor audit error, and `/tmp/catalog-task8-web-test.log:5` contains jsdom navigation diagnostics. The report also explicitly carries Gin debug startup output. These do not invalidate the behavioral assertions or justify impersonating an administrator/global production-log suppression. A later focused test-only cleanup can capture/assert expected diagnostics and locally supply unsupported browser behavior; retain the existing raw evidence. The disclosed deliberately killed MySQL-driver diagnostic is outside this PostgreSQL run and must not be treated as reproduced here.

## Checks and evidence boundary

- Read the task-reviewer method, complete task brief/report, and supplied exact diff. No git commands, tests, database/container operations, external executors, live sites, other agents or implementation edits were used. Only this review artifact was written.
- Named concrete risk: the selected CLI package might bypass model TestMain and silently use a different engine or omit its database test. One focused unchanged-code check of `cmd/catalog-sync/main_test.go:40` and line 82 showed the helper opens PostgreSQL and its selected plan test consumes `TEST_CATALOG_SYNC_POSTGRES_DSN`; the gate explicitly supplies that variable and rejects skip events. Its secret-redaction test intentionally targets an unavailable loopback PostgreSQL port. No additional unchanged-code checks were taken.
- Read existing normal/race wrapper final summaries, the three guard-failure logs, and raw normal load/upgrade events. Boot JSONL lines 4/16 identify the two real boot outputs; lines 14/26/28 show both children and parent passed. Load JSONL lines 5–7 provide measured scan and timeout evidence; the passing wrapper alone was not substituted for these details.
- Read existing final deployment/Python/frontend and bridge/build logs. Deployment reports 185 passing assertions, Python 12 tests OK and frontend 2,748 tests passed. Untagged files are `[transport.go]`; explicitly tagged files add `catalog_http_testbridge.go`. Quiet build/type/lint logs do not independently encode exit status: their reported exit-zero status is controller/implementer execution evidence, not a claim this reviewer regenerated. Raw diagnostics are disclosed above. No duplicate test run was performed.

## Assessment

**Task quality: Approved — TaskQualityApproved.**

**Counts: 0 Critical, 0 Important, 1 Minor.**

The approved Task 8 changes establish a fail-closed PostgreSQL gate and concrete load, startup, submission and deployment-contract evidence without production-code or scope expansion. Approval is limited to this task; the explicitly identified cross-task concerns and root-owned final review/archive remain outstanding.
