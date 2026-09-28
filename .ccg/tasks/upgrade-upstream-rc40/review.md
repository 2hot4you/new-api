# rc.37 checkpoint review

## Result

No open Critical or Warning findings after the final internal backend and frontend reviews.

## Resolved findings

- Immediate terminal task billing now persists the task and a leased `TaskBillingJob` atomically; inline failure or process interruption is recovered by the existing reconciler without duplicate settlement.
- Multi-candidate Task Plugin routing delays candidate-specific origin intent until selection, preserves native Seedance eligibility, rebinds to the selected decoder snapshot, and safely switches to a validated origin channel when required.
- Option primary-key migration fails closed on conflicting duplicate values and preserves the original table before swap.
- Passkey effective RP/origin validation, plugin origin ownership, wallet additive reserve accounting, and fresh-database option invariants are covered by focused tests.
- Usage-log and catalog dynamic-pricing displays carry the model `billing_currency`; CNY coefficients are not converted a second time.
- Molii channel Base URL rules, runtime branding, local-only metadata, and persisted channel IDs 61-64 are preserved.

## Verification

- `go test ./... -count=1`: passed.
- `GOWORK=off go test ./... -count=1` in `relaykit`: passed.
- `go vet ./...` and relaykit `go vet ./...`: passed.
- Frontend typecheck: passed.
- Frontend Vitest: 249 files, 2068 tests passed on the latest run.
- Frontend production build: passed.
- i18n completeness: passed.
- Compatibility-fix files: targeted oxlint and oxfmt checks passed.
- Plugin formatting and `go test ./plugins`: passed.
- `git diff --cached --check`: passed.
- Final candidate-routing review also passed focused and race-enabled middleware/jsplugin tests.

## Known baseline/tooling debt

The whole-tree rc.37 frontend lint, format, and copyright checks report broad pre-existing/upstream rule and header debt across numerous files unrelated to the compatibility fixes. This checkpoint does not perform a repository-wide formatting or lint rewrite. MySQL/PostgreSQL DSN-backed migration tests were not available locally; SQLite migration tests and dialect-specific unit coverage passed.

## Review execution

Review used local tests and internal review agents only. The user explicitly prohibited antigravity/Claude external executors.

# rc.38 checkpoint review

## Result

No open Critical or Warning findings after internal backend and frontend re-review.

## Resolved findings

- Preserved persisted Molii channel IDs 61-64; assigned vLLM and SGLang to unused IDs 65 and 66, with `ChannelTypeDummy` moved to 67. Literal registry and database reload tests guard the mapping.
- Integrated rc.38 Responses HTTP/WebSocket, request-policy, performance metrics, GPT Image 2, vLLM/SGLang, system-task, and channel-management changes while retaining Molii task billing, origin routing, COS previews, CNY usage, and local metadata behavior.
- Updated test database fixtures for the fail-closed Option primary-key verifier and OAuth default-token creation.
- Fixed the custom Combobox portal so dropdowns inside transformed dialogs/drawers use container-relative absolute coordinates; added a non-zero-offset geometry regression.
- Added frontend labels, filters, tests, and translations for the local `async_task_billing_reconcile` and `starai_result_cleanup` system-task types.

## Verification

- `go test ./... -count=1`: passed.
- `GOWORK=off go test ./... -count=1` in `relaykit`: passed.
- Root and relaykit `go vet ./...`: passed.
- Frontend typecheck: passed.
- Frontend Vitest: 268 files, 2513 tests passed.
- Frontend production build: passed.
- i18n completeness: passed.
- Staged frontend TypeScript/JavaScript files: targeted oxlint and oxfmt checks passed.
- Plugin format check passed; plugin lint completed with existing non-blocking warnings.
- `git diff --cached --check`: passed.

## Review execution

Review used local tests and two internal review agents only. No antigravity/Claude external executor was called.

# rc.39 checkpoint review

## Result

No open Critical or Warning findings after independent internal backend and frontend review and backend re-review.

## Resolved findings

- Preserved Molii channel IDs 61–64, assigned vLLM/SGLang to 65/66, and guarded the Dummy sentinel at 67. Corrected the Task Plugin binding documentation to type 63.
- Kept the trust quota opt-in default at zero and retained `ForcePreConsume`, subscription, wallet-only bypass, and multiplier behavior.
- Preserved terminal task CAS plus billing-outbox atomicity. A CAS loser now reloads the authoritative database task before a synchronous caller can render it, while only the winner records the terminal performance sample.
- Added the missing immediate-terminal performance sample after the task and billing intent commit; reconciliation does not duplicate it.
- Defined `task_sync` consistently as the persisted synchronous-protocol contract, including immediate jobs, timeout/disconnect, and reconciliation logs.
- OpenAI Images keeps the terminal snapshot until rendering and Base64 conversion succeed. Base64 download failure returns a 502 OpenAI error and retains recoverable data; successful preparation atomically clears the snapshot and marks it discarded.
- Preserved safe Seedance/StarAI/Grok list facts while keeping raw task data private, and retained Molii task projections, CNY pricing, local metadata, branding, plugin marketplace behavior, and legacy model display semantics.
- Removed the accidentally reintroduced official metadata-sync column, restored unavailable legacy-model icon behavior, corrected pricing filter counts and task quotation fallback semantics, and fixed Dialog/OAuth/i18n merge regressions.

## Verification

- `go test ./... -count=1`: passed.
- `go vet ./...`: passed.
- `go test -race ./service -count=1`: passed.
- RelayKit tests and vet: passed.
- Frontend typecheck: passed.
- Frontend Vitest: 282 files, 2655 tests passed.
- Frontend production build: passed.
- i18n completeness: passed.
- Staged diff and conflict-marker checks: passed.

## Known environment limits

MySQL/PostgreSQL DSN-backed integration tests were not available locally. SQLite coverage and dialect-aware unit tests passed; this is recorded as unavailable rather than as a database integration pass.

## Review execution

Review used local verification and independent internal review agents only. The user explicitly prohibited antigravity/Claude external executors, and none were used.

# rc.40 checkpoint review

## Result

No open Critical or Warning findings after independent internal backend and frontend review and a focused RelayKit re-review.

## Resolved findings

- Upgraded Task Plugin `Source` and `Icon` storage to dialect-aware long text: MySQL `LONGTEXT`, PostgreSQL/SQLite `TEXT`.
- Raised plugin uploads to an inclusive 8 MiB limit on both backend and frontend, with exact-limit acceptance and limit-plus-one rejection tests.
- Kept sync snapshots metadata-only. Source is loaded by row ID only for changed plugins; source read or compilation failure retains the incumbent runtime.
- Accepted every HTTP 2xx task submission status while preserving the adaptor's original status and empty-body behavior for 204.
- Integrated Claude/Responses tool-output media hoisting and stream lifecycle fixes. Internal review found and fixed repeated tool-call chat-index reuse after `finish_reason` by making tool state segment-aware.
- Added static-only plugin metadata preview parsing with bounded AST traversal; dynamic or unsafe JavaScript is never evaluated.
- Integrated pricing enum/condition/copy fixes and locale updates without changing Molii CNY/catalog behavior.
- Integrated theme storage helpers and cache keys while keeping the active Molii providers locked to system mode and branded defaults; setters/reset remain no-ops and stored arbitrary presets/fonts/modes are ignored.
- Preserved Molii branding, local metadata policy, channel IDs 61–67, Task Plugin marketplace, COS/task billing behavior, Seedance, StarAI and Grok integrations.

## Verification

- `go test ./... -count=1`: passed after the frontend production artifact was generated.
- `go vet ./...`: passed.
- RelayKit full tests and vet: passed after the final segment-aware tool-call fix.
- Frontend typecheck: passed.
- Frontend Vitest: 283 files, 2694 tests passed.
- Frontend production build: passed.
- i18n completeness: passed.
- Staged diff, conflict and conflict-marker checks: passed.

## Known environment limits

Real MySQL/PostgreSQL DSN-backed migration tests were not configured. SQLite and dialect-aware unit coverage passed, but live MySQL/PostgreSQL migration behavior remains unverified in this local environment.

## Review execution

Review used local verification and two independent internal review agents only. The user explicitly prohibited antigravity/Claude external executors, and none were used.
