# Local acceptance — four recovery gaps

Status: completed local-only acceptance. Full normal/race gates passed; local code commit `6df1ec3760d5f583034c6c6df103d0afd457429e`. Not pushed, merged or deployed.

## Source and authority

Base `0feb59bcc`, branch `fix/catalog-publication-liveness`, existing isolated worktree retained. Initial diff `c4863d...` was superseded after the full gate caught a compatibility regression; frozen final application/test/gate diff SHA-256 `28e637c59c1db9a4229e030c7a081d0d9931ed3409d35d9e63e001f2fc3451aa`. No source changes after amended freeze. No pushes, merges, deployments, production connections/DB/configuration writes or service restarts; no Aigc contact or paid generation. User deletions/drafts untouched.

## Four repaired paths

| Finding | Local change | Regression proof |
| --- | --- | --- |
| I1 status/OptionMap lock | Complete owned JSON envelope before network I/O; reserve OptionMap with full cache set, explicit nonrecursive locked helpers | Genuine handler/NaN/header/body RED→GREEN; real publication cancellation with old projections intact; blocked-status real LLM publication/settlement |
| I2 reader starvation | Counter-based writer admission at all live R entrypoints; cancellable actual W wait, single callback, no intent during own R/staging/no-op | Pending plus overlapping old reader/new read traffic, export/context/blocking reads, two-writer cancellation and successful ACK |
| I3 observation drift | Detached routing copy removes exactly3 observation keys; raw unknown JSON and precise canonical decimals retained | SQLite/MySQL8/PG; empty→observations; recheck/preinstall/ACK observations; genuine provider/ignored/model/unknown/invalid JSON guards |
| I4 stale expected target | Attempt-local exact obsolete result does not newly invalidate healthy newer ACK; joined/unknown failures remain closed | Three engines real R+1 ACK, stale R selection and unchanged state, existing invalid retained, joined capture failure closes reads |

I3 final compatibility correction: only when strict normalization fails and AdvancedCustom's existing typed parser also rejects, retain exact raw routing identity until unconditional lifecycle projection rejects before exchange/ACK; this preserves original ordinary postcommit pending semantics, error classification and zero lifecycle repairs. Typed-decoder-accepted ambiguity still fails strict normalization. Well-formed typed-invalid JSON needs no new exemption and fails the same mandatory projection. Both original test files unchanged; new ordinary matrix covers enabled/disabled malformed/typed-invalid/ambiguous, no ACK/read closure/full row equality/non-disclosure. Final covering normal/race22.861s/29.163s; final routing matrix11.893s/17.806s; original managed PG2.564s/3.924s. Both independent scoped reviews ADDRESSED/Approved. See review.md and appended task-2-report.md for failed prefix-only attempt and exact commands.

## Actual commands/results

All raw logs under `.superpowers/sdd/plan-fix-catalog-recovery-four-gaps/`. All Go commands use `env -u SQL_DSN -u LOG_SQL_DSN GOWORK=off`. Owned numeric-loopback fixture DSNs only; never existing5432/6379 or remote business DBs.

- `task-1-red.log`: old handler fails desired lock-release assertions. Fixed status normal/race pass, including existing OIDC/passkey/theme behavior and complete JSON/header compatibility. Task1 independent review PASS.
- `task-2-red.log`: all four correct-behavior regressions fail old source. Final target normal `go test ./model -run 'TestCatalog(Publication|Refresh|Reload|MixedPasskey|Startup|PublicRefresh)' -count=1 -v`:56top-level PASS115.032s. Target race56top-level PASS133.372s. No skips/races.
- `task-2-green-matrix.log`: real SQLite3.50.4 / MySQL8.0.46 / PostgreSQL17.11 affected I3/I4 and extra identity/admission tests PASS9.623s, no skips. Final joined-stale matrix `-race` PASS6.879s. Native supported PG-only lost-ACK/acquisition contracts PASS2.054s. Not the historical full three-engine repository suite.
- `billing-status-red-clean.log`: old GetStatus overlay fails because real price publication is blocked by status body I/O; fixture drains the old request before cleanup. An earlier draft had the same genuine assertion plus cleanup panic; preserved but not counted as clean RED.
- `billing-final-green.log` / `billing-final-race-green.log`: real loopback adaptor/pricing/preconsume/wallet/token/settlement/log pipeline PASS2.936s /12.828s. Seven requests, three held-request publications; settled quotas1500/2250/3000/3000/5250/5250/3000, used23250, remaining976750; frozen/new expressions, sourceCNY, FX7/14/28 and seven consume records checked. No external upstream or authentication-middleware coverage claimed.
- Mandatory selection inventory53groups/599entries preserved,54groups/611entries now;610 unique pairs. No original required child/matrix/package removed. Python gate tests12PASS; independent reviewer A23verifier testsPASS.
- Frontend: `bun run test --maxWorkers=2 --reporter=json --outputFile=...` exit0,663suites/3174tests PASS, zero pending/fail. `bun x tsgo -b`, `bun run build`, `bun run i18n:check` exit0. `bun run lint` exit1 with142preexisting errors; sorted error/warning diagnostics exactly equal previous immutable baseline. No new lint debt; not a green wholelint claim. Existing `bun check -b` compatibility debt untouched.
- Initial root `go vet ./...` and post-assets `go build ./...` exit0; relaykit/tokenkit builds exit0. Latest final-code build/vet stalled reading preexisting hidden duplicate generated `web/dist/... 2.js` files; serial retry and a materialization copy also stalled there. Read-only process samples/lsof evidence retained. No original duplicate was changed or deleted. Stopped only own stalled processes. Final `go vet -p 2 ./...` / `go build -p 2 ./...` on the same-source temporary packaging artifact both exit0 (`go-vet-artifact.log`, `go-build-artifact.log`). Initial earlier build/frontend cleanup race produced missing embed errors, preserved separately.
- Two independent full-diff nonauthor reviews: specPASS/qualityApproved, zero actionable findings; external antigravity/Claude wrapper unavailable, no false external certification. See review.md.

## Preserved failures / boundaries

Existing350ms changed-policy test expired before intended option-loader rendezvous under SQL latency; replaced only its schedule with actual loader/writer-intent wait + explicit cancellation, retaining unchanged fallback and later publication assertions. Focused normal3.922s/race6.237s passed; independent deadline tests unchanged. Earlier harness routing query counting and historical cross-engine eligibility prerequisite runs are documented in task-2-report.md, not counted as passing or repaired by weakening guards.

Preexisting ordinary SelfUse-save and plugin-generation hot updates remain separate unverified consistency boundaries. This task neither fixes nor certifies all ordinary option writes, distributed/mixed-binary publication or production rollout. A genuine price change with failed/unknown publication still rejects new selections; no force-ready, false price or mode bypass.

## Packaging-artifact acceptance environment

`/tmp/catalog-four-gaps-build.a42m8Z` was materialized from the base tracked tree plus exactly the15changed application/test/gate files. All tracked Go files and Go module files are SHA256-equal to the current worktree (`build-source-{worktree,artifact}.sha256`, `cmp` exit0); Makefile and both Python gate files also byte-identical. Fresh frontend bundle uses the unchanged Rsbuild configuration and supported `--dist-path` only; its build exits0. No source/gate predicate or required selection is altered to avoid the original filesystem problem.

The amended worktree normal run passed through runtime/request contracts but stopped before any main-package test body because compilation was stuck reading a hidden duplicate (zero-byte JSON evidence). Interrupted own group60366, not counted as full acceptance. Full unmodified normal/race gates are rerun from the beginning in the same-source artifact. This verifies the fixed application and clean packaging build, not the original worktree filesystem's health. Raw source evidence and user files remain in place.

## Final full acceptance and local handoff

Full fixed-source artifact normal `make test-postgres` exit0, 2026-11-10 19:00 Beijing:54mandatory groups /611selection entries /2039tests/subtests, no fail/skip; complete independent43packages/2733, relaykit9packages/583, tokenkit1package/7, no fail/skip. Logs `postgres-normal-artifact.log`; copied full evidence `postgres-normal-complete-evidence` and `postgres-normal-independent-evidence`. Not an implicit full historical cross-engine `make test` claim.

Full artifact `make test-postgres-race` exit0 at2026-11-10 19:27Beijing: same54groups/611selection entries/2039tests/subtests passed, zero test failures/skips and no race report. Complete independent suites also passed43packages/2733tests/subtests, relaykit9/583, tokenkit1/7. Logs `postgres-race-artifact.log`; copied complete raw directories `postgres-race-complete-evidence` and `postgres-race-independent-evidence`. Both modes use identical frozen source and unmodified mandatory inventory. Empty relaykit packages are explicitly allowed package-only no-test events, not skipped tests.

After execution, root independently replayed the actual unchanged mandatory/full-suite verifiers against every saved normal/race JSON file and compared every saved command with the current runner. `final-evidence-replay.log` exit0 confirms both54/611/2039 and complete43/2733,9/583,1/7; zero test skips/failures. Fresh Go/module manifests still match the artifact; gofmt and diff checks clean. Committed15file code delta from full base to6df1ec376 hashes exactly28e637c59c1db9a4229e030c7a081d0d9931ed3409d35d9e63e001f2fc3451aa, identical to reviewed/tested source. Real committed-range review package retained as `committed-review-package.diff`.

Only this task's three exact label-validated PG/MySQL/Redis container IDs were stopped/removed at19:29Beijing; ephemeral fixture data is discarded and reproducible from tests. Existing user development PG/Redis remain running, no daemon restart/prune or unrelated cleanup. Raw RED/GREEN, full JSON/command evidence and temporary same-source packaging artifact remain available. Original worktree/branch and unrelated user deletions/duplicate drafts remain preserved. Local archive/audit commit contains only this task's records and the learned backend spec, never a push.
