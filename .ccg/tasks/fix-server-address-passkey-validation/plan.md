# Server address/passkey validation implementation plan

> Required execution skill: superpowers:executing-plans. Inline single-module implementation per CCG; independent native analyses and final reviews because external wrapper is unavailable.

**Goal:** Allow saving the HTTPS server address from unused implicit localhost defaults without weakening configured Passkey trust.
**Architecture:** Keep the existing transaction, row locks and removal confirmation. Only auto-retain a previously usable implicit RP/origin; reject explicit invalid trust exactly as before. Existing frontend status invalidation remains unchanged.
**Tech Stack:** Go1.26/Gin/GORM; SQLite/MySQL/PostgreSQL tests; existing React status projection.
**Spec:** design.md and requirements.md in this task directory.

## Global constraints

No unrelated user edits, pricing changes, new dependencies, security bypass, SQL maintenance or paid generation. Aigc excluded from bulk release. Preserve server-side explicit RP/origin validation and known/unknown credential removal confirmation. Database behavior tested on all three real engines.

## Review focus

- Legacy empty origins and `[]` represent implicit defaults, not administrator consent to HTTP.
- Known localhost-bound and NULL/empty RP credentials still require confirmation before domain removal.
- Previously valid HTTPS trust (and explicitly allowed development HTTP) is retained.
- Explicit invalid RP/origin updates still roll back every option; disabled login is not a bypass.
- Main integration must contain aigc exclusion/maintenance guards and no unverified new source.

### Task 1: Correct implicit-default retention

**Files:** model/passkey_option.go; existing controller/passkey_test.go. Extend existing model/catalog_sync_reload_test.go only if required for engine fixture reuse; at most one new test file, prefer none.
**Interfaces:** unchanged UpdatePasskeyDomainOptions/UpdateOption and HTTP PUT /api/option/. No new public API.

- [x] Add regression TestPasskeyServerAddressReplacesUnusableImplicitDefaults for empty and `[]` origins, request through UpdateOption, new persisted ServerAddress and status; literal expected RP model.claudeye.com, no localhost legacy/origin. Test repeat save and preview no-write.
- [x] Add known/unknown credential removal-confirmation cases and explicit-invalid-origin preservation alongside existing passkey tests.
- [x] Run focused controller tests, observe the real expected invalid-domain failure before production edit.
- [x] In updatePasskeyDomainOptionsGuarded compute previous usability using existing RP/origin validation and apply it only to implicit origin auto-retention and legacy auto-retention. Do not relax explicit validation.
- [ ] Run focused tests, full controller and affected system-setting suites; real SQLite/MySQL/PostgreSQL matrix. Reuse safe owned disposable fixtures, never production DSNs.
- [x] Independent security and release-scope review, fix confirmed findings with RED→GREEN, inspect exact diff; commit only owned paths.

### Task 2: Development acceptance and main release

**Files:** task checkpoint/verification/review only unless CI exposes a task-caused failure.
**Interfaces:** existing deployment workflow, no gate modifications.

- [x] Fresh fetch; if develop unchanged push fix SHA to develop without force. Record Actions run and exact source.
- [ ] Wait for full Verify and Release development success; verify public development version exact SHA and non-mutating route health. Preserve existing settings/prices.
- [ ] Fresh fetch; review integration diff with main including existing rc47 and maintenance exclusions. Merge only accepted develop to main, no skip-ci, push and confirm automatic production targets exclude aigc.
- [ ] Observe three production releases and exact status versions. Report aigc unchanged and separate dated maintenance approval requirement. Archive task locally when authorized release endpoint is achieved; do not push audit-only SHA to retrigger production.
