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
