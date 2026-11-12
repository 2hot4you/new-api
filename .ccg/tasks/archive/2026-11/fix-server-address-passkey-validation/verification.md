# Verification complete

## Final production acceptance2026-11-03T11:28+08:00

Main d92203da2101ff90ebb45dcef83a4f9045232da5 app41452078936 push/attempt3 completed SUCCESS. Actual retried Claudeye125707468319 and Deploy immutable image success, completed03:27:48Z; full main Verify125707508126 and other two Releases125707467701/125707468006 retained success for same immutable source. Docs41452078994 push/attempt1 exact d922 Verify/all three Releases success. Fresh public status all success=true with exact production-molii-d92203da2101, production-claudeye-d92203da2101, production-model-claudeye-d92203da2101. Remote main/develop unchanged d922/b49. No additional push, server proxy/daemon/config change or Aigc contact. User must re-save production ServerAddress themselves. Historical pending/failure entries below are superseded by this final acceptance, not erased.

Actions: https://github.com/2hot4you/new-api/actions/runs/41452078936 and https://github.com/2hot4you/new-api/actions/runs/41452078994 . Development acceptance: https://github.com/2hot4you/new-api/actions/runs/41442170578 . Archive locally and delete only the associated monitor; no audit-only deployment push.

## Recovered proxy / attempt3 pending2026-11-03T11:26+08:00

User repaired proxy and authorized retry after checks. Same actual daemon proxy now verified GHCR/DockerHub CONNECT200 + expected anonymous401 + TLS0 and GitHub200/TLS0, all about1s; Claudeye app/PG/Redis healthy. Only failed Claudeye125689958408 rerun requested, exit0; exact d922 push app41452078936 now attempt3, new Claudeye125707468319 in_progress. No server configuration or daemon changes, no Aigc contact. Deployment not yet accepted; monitor actual Release and exact public version.

## Read-only network evidence2026-11-03T11:15+08:00

User authorized Claudeye-only read-only SSH. Existing host-key/key verification and target identity succeeded. Direct anonymous GHCR endpoint returns401 with valid TLS, but via Docker's configured nonempty proxy HTTP CONNECT succeeds200 then TLS fails SSL_ERROR_SYSCALL in~5s; same proxy failure for DockerHub/GitHub. Docker daemon proxy configured, GHCR not bypassed, live-restore=true; app proxy env names empty. Before/after app/PG/Redis healthy and app status exact old ce7. No deployment, proxy/service/config write or restart performed, no Aigc contact. Diagnosis supports failing daemon proxy path, not registry credential failure; repair needs separate authority.

## Retry result2026-11-03T10:42+08:00

Exact d922 app41452078936 attempt2 failed only Claudeye125689958408 at remote GHCR authentication: GET https://ghcr.io/v2/ EOF repeated four times, exhausted02:33:37Z. CI build/login and SSH success, assets/deploy skipped. Public three sites success=true; Molii/model-Claudeye exact d922, Claudeye remains ce7. Docs same-source verification/three Releases success; refs unchanged. Retry did not resolve transport failure; no full deployment acceptance. Stop retries and ask permission for read-only server network diagnostics, preserving Aigc exclusion and all settings.

## Authorized failed-target retry2026-11-03T10:32+08:00

User explicitly approved one Claudeye retry. Job-specific rerun of125323953575 accepted exit0; same exact main d922 app41452078936 now attempt2 with Claudeye125689958408 in_progress. Successful prerequisite/other-target results retained; no new main push or successful-target dispatch. Three public sites healthy, Claudeye remains ce7 awaiting deployment; Molii/model-Claudeye exact d922. Not full acceptance yet. Existing15-minute monitor updated; do not duplicate retry or contact Aigc.

## Partial production acceptance2026-11-02T22:30+08:00

Main d922 app41452078936 attempt1 complete Verify success. Molii Release125323953711 and model-Claudeye125323953738 success; live versions exact production-molii-d92203da2101 / production-model-claudeye-d92203da2101 with success=true. Claudeye125323953575 failed remote GHCR login with repeated registry EOF, before assets/deploy; live healthy old production-claudeye-ce7c4eb36272. Docs41452078994 all verification/three Releases success. Not full release acceptance: await user direction for failed-target retry or network investigation; no automatic new retry made.

## Main verification passed2026-11-02T22:15+08:00

Exact main d922 push run41452078936 attempt1 Verify backend and frontend125299577236 completed success, independently of develop. Three actual non-Aigc Release jobs started building; real rollout/version acceptance still pending. Triggered docs41452078994 Verify and all three individual Releases completed success. This supersedes older pending-main-verification entries, not the remaining production deployment gate.

## Development accepted / main promoted2026-11-02T21:20+08:00

Run41442170578 attempt2 exact b49 source completed success, all jobs including Verify backend and frontend and Release development. Public dev success=true and exact development-b49b60f57373. Fresh deployment target6/time-window2 tests PASS. Main merged as d92203da2101ff90ebb45dcef83a4f9045232da5; merge tree identical to accepted b49, both historical main and accepted develop parents preserved. App41452078936 and docs41452078994 automatically triggered at this exact main SHA; production verification/deployments remain pending. No aigc deployment or production settings changes. Older pending entries below are historical and superseded by this acceptance evidence.

## Remote verification2026-11-02T21:00+08:00

Exact sourceb49 run41442170578 attempt1 Verify backend and frontend job125266545304 completed success, including frontend and both required normal/race suites. Release failed later at image dependency fetch (HTTP/2 peer error from Go module proxy), before server operations. One diagnosed failed-job-only retry now attempt2; original successful verification remains bound to immutable same source. No successful development deployment or main promotion claimed yet. See release-checkpoint for exact retry job IDs and unchanged public version.

## Corrected-source local broad acceptance2026-11-02T20:30+08:00

Source b49b60f573734c16f8e7d3c21ff2618a9972ef0b: `GOWORK=off TEST_POSTGRES_DSN=<owned loopback fixture> TEST_REDIS_DSN=<owned loopback fixture> make test-postgres` follow-up normal23754 completed exit0.49mandatory exact selections,1920tests/subtests,zero skips. Independent complete root43packages/2733tests/subtests, relaykit9/583 and tokenkit1/7 also passed. Model reload group18passed, including unchanged mixed Passkey contract. No claim of local full race or broad legacy unfiltered controller suite; required full remote race/frontend/deployment still pending.

All three exact labelled disposable task containers removed after completion; original local development services remain running. Temporary fixture data intentionally discarded and can be recreated. Earlier observations and rejected d40 records below are historical, not the current acceptance source.

## RED → GREEN

New controller handler/storage/status tests initially failed at the same Invalid Passkey domain error (two implicit-origin cases plus three credential-removal cases). Minimal production retention guard then passed new and existing Passkey cases. Explicit invalid historical settings still fail with unchanged DB/runtime; no insecure-origin switch enabled.

## Completed

- SQLite3.50.4: selected TestPasskey* plus GetStatus privacy suite PASS25top-level tests; new regression cases including enabled=false/true and empty/[] origins PASS.
- MySQL8.4.11, disposable task-owned container: same selected suite PASS25top-level tests,35.694s.
- PostgreSQL15.18, disposable task-owned container: same selected suite PASS25top-level tests,55.180s. Fresh isolated main/log DB names and proper pool close/drop; unrelated local development DB not used.
- `GOWORK=off go test ./setting/system_setting ./service/passkey -count=1` PASS (service/passkey has no test files).
- Existing frontend `bun run test src/features/keys/components/__tests__/api-addresses.test.tsx`:5/5PASS.
- Python CI verifier/gate suites:22tests PASS; independent reviewers additionally validated target/time-window tests6+2PASS.
- Scoped gofmt and git diff --check PASS.
- Focused real PostgreSQL Passkey race suite PASS25top-level tests,95.314s.

## Pending

Initial source d40cf5c5c is REJECTED by local full normal acceptance: TestCatalogMixedPasskeySave:79 failed (expected existing explicit-origin protection, got nil). Previous usability was incorrectly gating storage of the NEW origin as well as retention of the old one. Corrected working diff preserves storing the new origin, drops only unusable old origin. Existing mixed-save test remains unmodified; new controller test now covers preview/storage and later foreign-origin rejection and was observed RED against d40. Initial focused passes do not override this full-gate failure. Corrected source requires fresh verification and development acceptance; do not merge d40.

Corrected follow-up verification: SQLite selected25Passkey top-level tests PASS5.539s; PostgreSQL15.18 same suite PASS69.218s; MySQL8.4.11 same suite PASS52.040s; four existing mixed-Passkey model contracts normalPASS6.139s/racePASS9.061s; new controller address regressions on PostgreSQL racePASS18.221s. Fresh go vet/scoped diff-check PASS. Broad local normal rerun is session23754, log .superpowers/sdd/plan-fix-server-address-passkey-validation/postgres-ci-followup-normal.log; complete independent root43/2733, relaykit9/583, tokenkit1/7 PASS, PG groups in progress. No broad green claim yet.

Full audited `make test-postgres` normal acceptance (complete independent root/relaykit/tokenkit suites already green; mandatory PostgreSQL groups running), remote full normal/race/frontend verification, dev exact deployed version, main promotion and three production releases. No broad historical make-test claim: attempted unfiltered controller execution was stopped because the project requires audited DB selections rather than legacy implicit DB suites.

Containers created solely for this task: server-address-test-postgres-20261102 (127.0.0.1:32768), server-address-test-mysql-20261102 (32769), server-address-test-redis-20261102 (32770). Remove only these exact labelled fixtures after local verification; do not touch molii-new-api-dev-* services.
