# Independent controller review

Reviewed controller commits `fab5b18f0` and `867b073c9` against baseline `275423c64`, Task 1 and global constraints. Read the complete controller diff in focused pieces, supporting bootstrap/plugin/write paths, and the controller owner's report. No suites repeated, no external executors, and no code changes made by this reviewer.

## Findings

- Critical: none remaining.
- Warning: none remaining.
- Resolved during review: the expanded shared fixture initially failed to restore standalone USDExchangeRate/SystemName and assigned OptionMap without the mutex. The owner fixed snapshots/restoration, cloned OptionMap under its mutex, and supplied focused verification in `fab5b18f0`.
- Resolved during review: the reused fixture's previous host-only URL validation allowed query parameters to override a rewritten disposable database path. `867b073c9` now validates the DSN before any connection: numeric loopback PostgreSQL URL, user and single database path, no fragment, only one `sslmode=disable` query value, and matching pgx target/TLS/fallback facts. Its pure regression proves the rewritten path is the effective database and rejects same-name database aliases. Root includes `TestControllerPostgresDSNGuard` in mandatory selection.

## Preservation and consistency

Every converted legacy owning fixture branches before its old driver/InitDB path when PostgreSQL-only mode is enabled. Matrix enumeration independently limits mode=1 to PostgreSQL while the root selector also requires PostgreSQL children. Existing legacy paths remain available outside this acceptance; none were executed for review.

The shared fixture supplies all reference/catalog/billing tables required by the selected tests and calls actual migrations, option bootstrap and recovery. Factory plugins come from real embedded sources. Custom plugin helpers persist exact source/hash/API version/version/active/enabled facts matching compiled runtime programs and recover through the public production path. Provider removal/replacement changes desired and effective facts together. No fake-ready flag or production parity relaxation appears.

Business assertions remain intact. Listing's deliberately invalid/empty expressions are moved after real vendor publication so the intended exclusion assertions are reached rather than erased by publication. Image retry/vendor prices are persisted before production publication reconstructs settings. The image billing nil check is now an equivalent fatal diagnostic. The overflow test additionally excludes publication-pending false positives.

The rollback injection is stronger and aligned with the actual production write path: model pricing uses `Create` plus `OnConflict` for option rows. The fixture now injects at both portable callback classes, changes two real option values, requires failure on the second write, and retains full persisted snapshot/runtime rollback assertions. This does not replace a rollback assertion with mere error acceptance.

Skipping `record.Delete()` only during teardown of the shared-provider PostgreSQL fixture is safe: plugin cleanup would otherwise make that separate ordinary mutation invalid, and the entire exactly owned database is dropped after closing pools. Runtime config, pricing/group maps, FX/name, options, DB/log handles/types, registry and relevant flags are restored. Existing per-test cleanup remains ahead of fixture restoration under LIFO ordering; controller TestMain makes background work synchronous.

## Assessment

Approved for root's final full normal/race entry-point validation. Owner-reported normal/race selections cover 51 top-level tests and 168 children without skips; review did not repeat or independently assert those runtime results. Root owns fresh combined acceptance and final push authorization. Scope is selected PostgreSQL acceptance, not historical full cross-engine controller coverage.
