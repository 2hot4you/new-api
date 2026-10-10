# PostgreSQL CI repair evidence (in progress)

## Scope

Repair base: 275423c64. Remote develop before repair: d739627b21728d6d855435c20010d1935958e61e. Original failed workflow: https://github.com/2hot4you/new-api/actions/runs/38047800232, job114200780655.

Test fixtures, test runner/workflow wiring and task records are changed. A final race failure additionally confirmed a production first-heartbeat timestamp ordering defect; its bounded correction initializes creation/heartbeat timestamps consistently without changing strict eligibility policy. No production prices, billing, auth policy, catalog publication protections or live database records are changed. Explicit human authorization: repair, verify, nonforce push develop; PostgreSQL only; no antigravity/Claude external execution. Existing user untracked web ` 2.*` files are untouched.

## Coverage and prerequisites

- Full suites:41 audited database-independent root packages and relaykit. Actual JSON acceptance observed41 root packages /2249 tests and subtests;9 relaykit packages /486 tests and subtests, with zero test skips. Relaykit packages without test files are allowed only when `go list` positively identifies them as such, never as test passes.
- Mandatory PostgreSQL selections retain all104 top-level failures from the original run. An independent reviewer fetched the raw failed GitHub log and found exactly104 declarations, with zero missing/extra and none missing from executable groups.
- Additional24 safe converted controller/relay/helper regressions and3 parser safety tests are included. The heartbeat repair adds2 more required top-level regressions: final inventory31 exact groups,372 distinct top-level tests. Final normal/race results will be recorded below.
- This is NOT historical `make test` acceptance and NOT a wholesale migration of roughly98 legacy SQL fixture files. Fifteen legacy package paths are excluded from full-suite execution; only cases explicitly in `GROUPS` run, and some excluded packages have no selected cases. No unrelated old SQL test result is claimed.
- Owned disposable PostgreSQL15.18 container:molii-catalog-sync-test-pg-20261009, loopback port32768. Task-only Redis7 container:molii-catalog-ci-redis-20261010, loopback port32773. No user development/production services or other engine containers are tested.
- Identical local and workflow entry points:make test-postgres and make test-postgres-race. Missing DSN/Redis prerequisite, test skips, empty/missing required selections or non-PostgreSQL children fail. GOWORK=off and GOFLAGS empty in every runner subprocess.

## RED/GREEN and independent review

- Controller, relay/helper and model concurrency RED/GREEN evidence is in their individual reports. Original catalog consistency/barrier/business assertions retained.
- Python runner scaffold initially had missing functions and syntax errors; those are not behavioral RED proof. Actual inventory RED then rejected54 missing incident regressions before selections were added. Final Python unit tests:16 passed.
- Deployment contracts RED:185 assertions with exactly1 failure, the obsolete broad lifecycle-race command. Updated contracts require exact newline-terminated PostgreSQL normal/race commands in both workflows and forbid old broad entries. GREEN:192 assertions; retry helper passed.
- Full independent suite race RED exposed Gin process-global mode writes in parallel test bodies in relay/channel and relay/channel/gemini (no skips). Moved Gin test mode to one TestMain per package; kept parallelism and all assertions. Focused full-package race GREEN:channel2.904s and gemini2.066s. Next full race run exposed the same pre-existing MiniMax mode write/read race; applied the same change in its sole adaptor_test.go. MiniMax full-package race count3 passed2.096s. Static scan of all41 full packages found no other mode writes inside concurrently runnable test bodies. Final complete root race phase passed41packages/2249test events and relaykit9packages/486events; PostgreSQL race groups continue.
- Independent review found same-name `dbname`/`database` DSN query overrides can survive disposable URL path rewriting. Strict sole sslmode query allowlists and pure parser regression tests were applied to controller/relay/helper fixtures before any connection (60502bec3,867b073c9,c6c9fc00e). Dangerous-override RED probes are parser-only, never database connections; all focused normal/race GREEN and actual guarded TestMain diagnostics are in owner reports.
- Independent review found controller standalone FX/SystemName restoration gaps; fixed with explicit snapshots/restoration and OptionMap mutex-protected cloning. Targeted image quota/vendor refresh passed3.756s. Earlier controller normal51 top-level /168 total passes, zero skips. A240s race test process timed out compiling next embedded factory source, with no assertion/race failure; retry480s evidence is tracked by controller owner. Mandatory gate per-process budget is900s.
- Independent infra review's GOWORK and exact normal/race contract findings fixed. Reports record each resolution. Independent controller/model/relay/infra/root-race and whole-change source reviews approved with no open material findings; MiniMax follow-up separately approved.

## Fresh static/build checks

Root `go vet ./... && go build ./...`:exit0. Relaykit `GOWORK=off go vet ./... && GOWORK=off go build ./...`:exit0. These compile-only commands do not execute SQL TestMain/fixtures. Python unit discover:16 tests passed. Exact tracked `git diff --check`:passed. No frontend source changed; frontend checks/build remain in automatic CI.

After the final MiniMax change, root vet/build was repeated (exit0), MiniMax normal passed0.864s, and BOTH complete independent normal phases were refreshed with the exact runner package inventory/flags. JSON verifier accepted root41packages/2249tests and relaykit9packages/486tests, zero test skips. Evidence:/tmp/catalog-ci-frozen-independent-normal.jsonl and /tmp/catalog-ci-frozen-relaykit-normal.jsonl. The in-progress370-parent PostgreSQL normal gate already uses the final controller/relay/helper/model test code; no PostgreSQL fixture was changed afterward.

Final race command evidence:/var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/postgres-ci-race-ax1p45pt; its complete root/relaykit phases passed after MiniMax fix. PostgreSQL group evidence:/var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/catalog-postgres-race-glkrba4h. Final normal command evidence:/var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/postgres-ci-normal-3f2bx7mk; PostgreSQL groups:/var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/catalog-postgres-normal-alcf4ec5.

## Workflow tooling scratch incident

SDD helper plan basename mapping resolved to pre-existing ignored `.superpowers/sdd/plan`; generating task briefs overwrote the historical ignored task-2-brief.md before collision was recognized. No product/source/user web files were affected. Original historical brief was not recovered. No further access to that legacy scratch was used; task reports live in this task directory.

## Final acceptance / push

Final PostgreSQL NORMAL acceptance completed exit0:31groups,370required top-level parents,1103passing tests/subtests,zero skips. Complete independent normal phases were refreshed after the last MiniMax test-only change:41rootpackages/2249events and9relaykitpackages/486events,zero test skips. Every originally failed104parent is included.

First final RACE acceptance failed in TestCatalogSyncTargetNamespaceManagedRollback/restore=false/aba=true during first-heartbeat eligibility setup (no data race). The unchanged exact case subsequently passed normal/race, while a deterministic real PostgreSQL callback-delay regression reliably proved CreatedAt > LastSeenAt across a Unix second. See controller-runtime-followup.md. Repair and full fresh normal/race acceptance are required; earlier normal counts are historical, not final corrected-source acceptance. Pending archive, nonforce push and inspection of the triggered workflow. Do not interpret local pass counts as a completed deployment.

Spec evolution: added .ccg/spec/guides/index.md documenting the non-obvious pgx query-overrides-path hazard, disposable-fixture safety, actual catalog startup/parity requirements, honest PG acceptance boundaries and process-global Gin initialization.

## Corrected-heartbeat acceptance (latest)

Production correction commit41e5f190e initializes CreatedAt from the resolved heartbeat observation, preserving all conflict columns and the strict eligibility guard. Focused controller normal/race8.780s/14.285s and model startup normal/race8.537s/12.354s passed. Two independent source reviews inspected the production invariant and regression semantics. One reviewer caught GORM Update auto-changing UpdatedAt in the two new negative cases; b22a0e385 uses UpdateColumn plus full-row readback proving only CreatedAt changed. All19 eligibility cases then passed normal2.934s/race3.402s.

Fresh full commands use the same task PostgreSQL/Redis prerequisites and make entry points. Normal evidence:postgres-ci-normal-1z933z22 and catalog-postgres-normal-tje23_u2. Race evidence:postgres-ci-race-q2vki9hh and catalog-postgres-race-_rdes8vy, all under the recorded task temp root. Both independent-suite phases passed41/2249 and9/486. The negative-test-only b22a0e385 correction occurred before either run reached the model startup group; that final group must compile/execute the corrected tests. Production/controller/full-independent-suite source did not change during these runs. Final PG results pending.

Fresh root and relaykit vet/build after the production correction:exit0. Root vet/build was refreshed again after b22a0e385 (exit0). Python16, deploy-contract192 and retry helper passed. Unrelated web files remain untracked and untouched. No push yet.

Latest complete NORMAL command finished exit0:41 independent root packages/2249 tests-subtests,9 relaykit packages/486 tests-subtests,31 mandatory PostgreSQL groups/372 required parents/1109 passing tests-subtests. Zero required skips. The final startup group passed32 events with the corrected UpdateColumn/full-row-readback cases. Every original104 incident regression remains selected. Latest race results still pending below.

Latest RACE has now passed controller-catalog_sync_test (156 events), including the exact previously failing managed namespace rollback and real first-heartbeat boundary regression. Remaining HTTP/lifecycle/catalog groups still running; this is not a final race-pass claim. Both heartbeat source reviews are approved; the primary report explicitly records the resolved GORM negative-test warning.

## Final corrected-source result

Both exact complete commands finished exit0 on the corrected source: make test-postgres and make test-postgres-race. Each accepted41 full independent root packages/2249 tests-subtests,9 relaykit packages/486 tests-subtests and31 PostgreSQL selections/372 required top-level tests/1109 tests-subtests, zero required skips. Race mode reported no DATA RACE. Both startup groups executed32 events including the final isolated creation-time negative cases; both previously failing controller groups executed156 events. All104 original failed top-level names remain mandatory.

Final static/build/contract checks: root and relaykit vet/build exit0; Python16 passed; deployment contracts192 passed; retry helper passed; git diff --check passed. No frontend changes and no forbidden engine execution. Independent reviews have no unresolved Critical/Warning findings. Remote develop fetch remained unchanged with zero commits behind and9 ahead before the root infrastructure commit. Authorized nonforce push and post-push workflow confirmation are next; this record does not claim remote deployment completion.

## Delivery

Nonforce push succeeded: remote develop d739627b2 → ff736866c13126a41f3674a8195ce807ff5fea96, with zero commits behind and10 reviewed/local commits ahead immediately before push. Automatic deployment workflow38054113686 was created at2026-10-10T13:01:09Z for that exact source SHA: https://github.com/2hot4you/new-api/actions/runs/38054113686. Observed status in_progress; deployment success is NOT yet claimed. No workflow_dispatch, live catalog apply, price/data mutation or manual deployment was performed.

Task records are archived and pushed separately after this confirmed code push. The archive-only commit touches .ccg/tasks/**, which the existing deployment workflow paths-ignore excludes; it does not replace the verified immutable deployment source SHA or trigger another business-code release. Worktree and unrelated untracked web files remain preserved.
