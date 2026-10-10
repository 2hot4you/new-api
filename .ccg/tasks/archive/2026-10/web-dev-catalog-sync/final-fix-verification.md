# Consolidated catalog final fix wave

Base: `0dbc5053b4ecb357d46b69c04c78c756a0c9fb20`. Status: DONE_WITH_CONCERNS; all covering checks passed. Local commit: `c9e1416f7` (`fix(catalog): recover proof refusals and tighten final validation`), exactly the 12 owned source/test/locale paths. Scope: I1 and M1–M3 only. M4 and optional module splitting remain deferred. Post-commit tracked status contains only the three pre-existing root-owned CCG record edits.

## Changes and contract evidence

### I1 — definite pre-write proof refusal recovery

`index.tsx` recognizes only Axios responses with HTTP 403, `success: false`, and one of the exact shared middleware codes: `SECURITY_PROOF_REQUIRED`, `SECURITY_PROOF_EXPIRED`, `SECURITY_PROOF_SCOPE_MISMATCH`, `SECURITY_METHOD_UNAVAILABLE`, `SECURITY_PROOF_METHOD_MISMATCH`, `SECURITY_PROOF_CONSUMED`, `SECURITY_PROOF_CONTEXT_MISMATCH`, `SECURITY_ACTION_FORBIDDEN`, `SECURITY_PROOF_INVALID`.

Evidence: `middleware/secure_verification.go` lines 39–98 emits precisely these structured failures from `RequireSecurityProof`; `controller/catalog_sync.go` checks committed replay first and returns on a nil proof at lines 341–343 before calling `ApplyCatalogSyncPlan`. The generic `AUTH_INTERNAL_ERROR` is deliberately not classified because that generic code is not uniquely scoped to this proof refusal contract. No prefix, prose, generic 403/500, receipt absence or network failure establishes rollback.

For these definite refusals, clear the unused in-memory/sessionStorage binding. Retain the saved plan and its choices: proof rejection does not establish plan invalidity, and existing expiry checks still apply before obtaining proof and sending. A new explicit confirmation creates a new UUID and gets a new proof bound to that UUID, final digest, target and kind. No automatic verification or write retry is added. Existing catalog stale/blocked errors still invalidate the plan. Known receipts from the error, mounted state or the exact bound receipt query prevent clearing. Unknown commit, disconnect, an unrecognized proof code, wrong status, immediate actual `CATALOG_NOT_FOUND` HTTP 404, and committed-pending publication retain the operation and prevent resubmission.

Regressions exercise actual expiry and method-mismatch response shapes, enabled review after refusal, cleared sessionStorage, remount with no unknown-operation lock, and new user-driven preview/verification/apply with a different bound operation and successful receipt. Existing pending/receipt tests remain, and the unknown-result test now includes actual 404 responses plus unknown commit, unrecognized security code and wrong-status variants.

Server proof, authorization, billing and database policy are untouched. Consulted the OWASP [Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html#require-re-authentication-for-sensitive-features) and [Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html#the-sessionstorage-api) guidance: recovery retains server authority, fresh operation-bound verification and existing single-use/no-replay behavior; only non-secret operation bindings are persisted. This narrow UI repair is not a new ASVS certification or full authentication audit.

### M1 — raw map null strictness

The existing `exactWireFields` now recursively checks map values against the authoritative reflected element type using `common.Unmarshal`. A valid empty-kind snapshot with numeric `vendor: 0` still passes the real controlled TLS transport; changing only that count to JSON null is rejected. The fixture calls the existing snapshot validation before its one wire mutation. Fixed source URL/TLS/public-IP resolution, common JSON wrapper, limits, digest/schema validation and existing inner `Entry.Value` semantics remain unchanged. No extra manifest parser.

### M2 — restore copy

The changelog receives the plan kind. Restore values say “After restore”; defensive conflict rendering says “Restore saved value for entire item.” Ordinary sync wording is unchanged. Both keys are translated in all seven locales. The actual restore-preview flow checks the reachable restore value heading; a separate explicitly hypothetical component case checks defensive restore conflict wording. It does not claim the current restore engine emits conflicts or enable a backend conflict override.

### M3 — composed keyboard coverage

An actual keyboard-only flow traverses the existing page, `ConfirmSyncDialog`/shared `ConfirmDialog`, real `useSecureVerification` and real `SecureVerificationDialog`. Only API responses are mocked. It uses Tab/Enter/Space and keyboard text input; no click, direct `.focus()`, artificial focus mock or dialog replacement. It verifies disabled save before consent, cancellation back to the initiating review button, deletion selection with Space, saved final digest/consent, final confirmation, autofocus on the real 2FA input, no verification/write before input, verification cancellation returning focus to review, no stored operation after cancellation, then re-entry and one successful operation whose proof has the final binding. Bounded Tab traversal fails if the intended control is unreachable.

This test passed against existing production dialog behavior on first execution; it is added characterization of previously missing coverage, not a claimed production focus bug or RED-driven focus fix. No shared dialog refactor was needed. The repository shadcn skill and component reuse rules informed keeping the existing shared confirmation/security workflow intact.

## Exact owned changed files

- `internal/catalogtransport/transport.go`
- `internal/catalogtransport/transport_test.go`
- `web/src/features/system-settings/catalog-sync/index.tsx`
- `web/src/features/system-settings/catalog-sync/changelog.tsx`
- `web/src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx`
- `web/src/i18n/locales/en.json`
- `web/src/i18n/locales/zh.json`
- `web/src/i18n/locales/zh-TW.json`
- `web/src/i18n/locales/fr.json`
- `web/src/i18n/locales/ru.json`
- `web/src/i18n/locales/ja.json`
- `web/src/i18n/locales/vi.json`

This ignored report is the sole reporting artifact written. Root-owned CCG records and all user untracked ` 2` files were preserved. No extra source paths or dependencies were introduced.

## RED/GREEN and checks

Commands below ran from the worktree root, except `bun` commands from `web/`. Output is retained under `/tmp/`.

| Check | Command | Result / log |
|---|---|---|
| M1 RED | `go test ./internal/catalogtransport -run '^TestCatalogTransportRejectsNullCoverageForEmptyKind$' -count=1` | Expected rejection got nil; numeric zero passed. `/tmp/catalog-final-fix-m1-red.log` |
| M1 GREEN | Same focused command after map recursion | PASS. `/tmp/catalog-final-fix-m1-green.log` |
| Initial UI check | `bun run test src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx -t 'clears an unused binding\|restore preview is a local inverse'` | Restore header RED is genuine. Initial two I1 failures stopped on incorrectly expected untranslated server message and **are not behavioral RED evidence**. `/tmp/catalog-final-fix-ui-red.log` |
| Initial UI rerun | Same focused command | Restore header GREEN; I1 still stopped on message expectation. `/tmp/catalog-final-fix-ui-green.log` is a historical filename, not a passing result for that run. |
| Correct I1/M2 RED | `bun run test src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx -t 'clears an unused binding\|restore changelog uses'` | With proof-clear branch temporarily restored to baseline and restore choice label restored to baseline: both I1 cases fail because Review remains disabled; restore choice fails because it says dev. `/tmp/catalog-final-fix-i1-m2-red.log` |
| M3 existing behavior | `bun run test src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx -t 'keyboard-only deletion'` | PASS using real components. `/tmp/catalog-final-fix-m3-initial.log` |
| Feature GREEN | `bun run test src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx` | 23/23 PASS after restoring fixes and adding conservative classification cases. `/tmp/catalog-final-fix-feature-green.log` |
| Final covering web | `bun run test src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx src/features/auth/secure-verification/__tests__/verification-flow.test.tsx src/features/auth/secure-verification/__tests__/verification-api.test.ts` | 3 files, 50 tests PASS, including final actual 404 fixture and lint fixes. `/tmp/catalog-final-fix-covering-web.log` |
| Full web, once | `bun run test` | Exit 0; 291 files / 2,755 tests PASS, 181.27s. `/tmp/catalog-final-fix-full-web.log` |
| Transport normal | `go test ./internal/catalogtransport -count=1` | PASS, 15.981s. `/tmp/catalog-final-fix-transport.log` |
| Transport race | `go test -race ./internal/catalogtransport -count=1` | PASS, 31.065s. `/tmp/catalog-final-fix-transport-race.log` |
| Typecheck | `bun run typecheck` | Exit 0. `/tmp/catalog-final-fix-typecheck.log` |
| Affected lint | `bunx --no-install oxlint -c .oxlintrc.json src/features/system-settings/catalog-sync/index.tsx src/features/system-settings/catalog-sync/changelog.tsx src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx` | Initial three missing-braces errors in new test fixed; final exit 0. `/tmp/catalog-final-fix-lint.log`, `/tmp/catalog-final-fix-lint-green.log` |
| i18n | `bun run i18n:check` | PASS. `/tmp/catalog-final-fix-i18n.log`; all seven parsed JSON catalogs contain 7,931 keys including both additions. |
| Frontend build | `bun run build` | Exit 0. `/tmp/catalog-final-fix-web-build.log` |
| Untagged production build | `go build ./...` | Initial overlap with frontend build encountered disappearing `web/dist` embed inputs, **not** a code/compiler regression. Rerun after frontend build completion exits 0. Both `/tmp/catalog-final-fix-go-build.log` and `/tmp/catalog-final-fix-go-build-green.log` retained. No catalogsynctest bridge or persistent GOFLAGS. |

`gofmt` ran on both owned Go files; `oxfmt` ran on the three owned TSX files. Final `oxfmt --check` on those three files and `git diff --check` passed (`/tmp/catalog-final-fix-format.log`). Read-only `bunx --no-install shadcn info --json` confirmed existing Base UI configuration (`/tmp/catalog-final-fix-shadcn-info.log`). Self-review covered the exact owned production/test/locale diffs and confirmed scope, no new secrets and no bypass of shared dialogs or proof authority.

## Evidence boundaries and retained concerns

No DB execution, forbidden legacy TestMain, SQLite/MySQL/container, live dev, paid upstream, push, merge, deploy, online apply, archive or cleanup occurred. The existing 27-group PostgreSQL normal/race evidence (824 tests/subtests in each gate) remains evidence **on base `0dbc5053b`**. It was not rerun or represented as a fresh all-feature gate on amended HEAD. This wave changes transport parsing and frontend lifecycle/copy/tests only; the fresh covering evidence above applies to these changes.

M4 remains intentionally disclosed: existing actor-role/Gin and historical setup diagnostics are not suppressed or “fixed” with an invented identity; failure audit remains best effort. Full frontend output retains jsdom navigation/scrollTo diagnostics. The large lifecycle module split is deferred.

The independent review's billing limitations remain: retained Grok/direct-image quantity and reservation math and Seedance positive-only multiplier/zero-rate behavior are not corrected by catalog capture. Representative rc40 catalog upgrade fixtures are not a complete released database or old-binary execution. The bounded 30k-history/15k-active load fixture is not production capacity evidence; O(all-history) scans and broad relation fences remain scaling constraints. No arbitrary-scale deployment readiness, other-engine compatibility, live Redis/cloud CI acceptance, real dev credentials/TLS acceptance, paid provider behavior, distributed publication or duplicate manual-node discovery claim is added.
