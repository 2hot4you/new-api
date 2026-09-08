# Review — upstream rc.36 upgrade

## Scope and review method

- Reviewed the complete branch from `e4120574b` through the final rc.36 integration commits.
- The configured external CCG wrapper was attempted for both antigravity and Claude, but `/Users/naf/.claude/bin/codeagent-wrapper` was unavailable and both calls exited 127. No external-model result is claimed.
- Independent repository agents reviewed backend/data/security and frontend/catalog/pricing behavior. Findings were fixed and re-reviewed.

## Resolved findings

### Backend

- Batch polling now validates plugin state and data before mutating a durable task, so invalid nonterminal responses accumulate failures and cannot partially pollute task state.
- JavaScript task plugins return invalid/oversized state as a failed polling result instead of silently dropping it and resetting the failure counter.
- Per-call and task billing consistently use the canonical billing identity, including mapped tiered expressions and self-use mode.
- Automatically inferred billing identities and tier snapshots are recalculated per channel attempt; explicit request-scoped billing overrides remain stable.
- Terminal OpenAI SSE parser failures no longer log the rejected response frame.

One re-review warning considered an impossible production path: no retry-loop callback or non-test caller can inject a new `BillingModelName` between task attempts. If that capability is added later, the assignment API must carry an explicit source marker.

### Frontend and catalog

- `/api/pricing` now publishes CNY for Molii's direct-CNY tiered models and StarAI/Grok task matrices while leaving ordinary USD entries unmarked.
- CNY is preserved through task ranges, examples, compact model cards, administrator price cells, and quotation snapshots.
- Administrator price cells no longer combine a CNY amount with a USD/TOKENS/custom-currency caption.
- Existing model names are read-only in the edit drawer, matching the immutable backend contract; creation remains editable.
- A catalog interaction test received a local 15-second timeout budget because its complete router/dialog/tab flow exceeds Vitest's default five seconds in the full merged suite. Assertions were not relaxed.

## Verification evidence

### Go and relaykit

- `go test ./... -count=1`: passed.
- `go vet ./...`: passed.
- `go build ./...`: passed.
- `go test -race ./relay ./relay/channel/task/jsplugin ./service ./relay/helper ./relay/channel/openai -count=1`: passed.
- `relaykit`: `go test ./... -count=1`, `go vet ./...`, and `go build ./...`: passed.

### PostgreSQL and Redis

- PostgreSQL 17 full-schema migration and repeated migration passed with `TestPostgresFullSchemaMigrationIsIdempotent`.
- Redis 7 configured round-trip and TTL behavior passed with `TestConfiguredRedisRoundTripPreservesTTL`.
- A local server started twice against disposable PostgreSQL/Redis resources; `/api/status`, `/`, `/pricing`, `/sign-in`, and `/setup` returned HTTP 200.
- Browser smoke showed the Molii setup screen, PostgreSQL detection, expected layout, and no console errors.
- The disposable containers and their data were removed after verification.

### Frontend and plugins

- `bun run test`: 206 files and 1,476 tests passed.
- `bun run typecheck`: passed.
- `bun run i18n:check`: passed.
- `bun run build:check`: passed.
- `bun run lint:plugins`: passed with seven existing `preserve-caught-error` warnings.
- `bun run format:plugins:check`: passed.
- Focused lint passed for every changed frontend file.

Repository-wide frontend lint/format/copyright checks still contain pre-existing upstream/local technical debt. The same gates fail on the pre-upgrade `develop` worktree; the upgrade reduced lint diagnostics from 406 (331 errors, 75 warnings) to 347 (273 errors, 74 warnings). No unrelated mass-formatting was performed.

### Integration integrity

- `git diff --check` and staged diff checks passed.
- No unmerged index entries or conflict markers remain.
- `v1.0.0-rc.36` is an ancestor of the upgrade branch, preserving upstream merge ancestry.
- All changed Go files are `gofmt` clean.

## Remaining rollout work

- The branch is verified but has not been pushed, merged into `develop`, or deployed.
- Before production deployment, take a PostgreSQL backup and use the normal single-instance smoke/rolling deployment procedure.
