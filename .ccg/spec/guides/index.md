# PostgreSQL CI fixtures

## Checkbox labels with localized actions

- Base UI checkbox labels need both the visible localized action and unique model/source context in their accessible name. Reference both IDs explicitly with aria-labelledby, in selected and unselected states; do not assume one translation is a substring of another language's contextual label.
- Use native Label behavior rather than adding parallel click/key handlers around a checkbox; test direct checkbox clicks, text clicks, Space, disabled state and focus to catch double toggles.

## Disposable database connection safety

- Validate connection URLs before the first connection, not only in the outer CI runner. Require an explicit numeric loopback PostgreSQL host, user, database path and exactly one `sslmode=disable` query value for local disposable fixtures.
- Reject all other query keys. In pgx, a `dbname` or `database` query parameter can override the URL path even when it initially equals that path. Replacing only the path with a random temporary database name is therefore unsafe without a strict query allowlist.
- Prove dangerous override rejection with pure parser tests; do not connect with a dangerous URL to demonstrate the bug. Assert that parsing the rewritten valid URL resolves to the owned temporary database.
- Close fixture pools before dropping only the exact database the fixture created. Never operate on a user development or production database.

## Catalog test startup and acceptance

- Catalog write/pricing fixtures must perform real migrations, option bootstrap and runtime recovery. Runtime-only plugin registration does not establish persisted/effective catalog parity: persist matching source/hash/API version/version and enabled/active facts when a test performs guarded catalog mutations.
- Preserve intentional invalid request inputs at the request boundary after valid startup. Do not bypass production readiness protections to make old fixtures pass.
- PostgreSQL deployment acceptance uses `make test-postgres` and `make test-postgres-race`. Missing prerequisites, empty required selections, skipped required tests and missing PostgreSQL matrix children are failures. Keep full-suite allowlists audited for hidden database TestMain/setup.
- State the coverage boundary explicitly: selected PostgreSQL acceptance is not the historical broad cross-engine `make test` result. Normalize `GOWORK=off` in every acceptance subprocess.
- Gin mode is process-global. Packages with parallel Gin tests must initialize mode before `m.Run`, not write it from parallel test bodies. Retain test assertions and concurrency when repairing fixture races.
- First heartbeat creation must use the same resolved observation timestamp for `created_at`, `last_seen_at` and `updated_at`. Independently sampling the creation hook across a Unix second can violate the intentionally strict catalog eligibility ordering. Later upserts must preserve `created_at`; test the real create callback boundary rather than adding retries or relaxing eligibility guards.
- PostgreSQL DDL fixtures must match production `PreferSimpleProtocol: true`. Default pgx statement caching can retain a pre-DDL `SELECT *` result shape and intermittently fail with SQLSTATE `0A000` after a column is added; do not hide this mismatch with retries or weaker DDL assertions.
- To test concurrent single-use proof consumption, rendezvous after catalog plan prechecks and before the real proof UPDATE. Uncontrolled HTTP overlap can legitimately yield writer-busy plus proof-consumed with zero operations; do not assume one success for that unrelated schedule, relax single-use checks, or retry consumed proofs.
- Source export releases its read-transaction table locks before the final fresh metadata proof. A positive concurrent-DDL fixture must control the DDL commit until that proof completes; an autocommit ALTER can legitimately win this interval and cause fail-closed metadata-change rejection. Preserve real lock-wait, completion and cancellation assertions rather than retrying or accepting either outcome.

## Pricing editor prerequisites and catalog feedback

- A model appearing in a compiled/default pricing snapshot does not prove exact writable metadata or source currency exists. Missing prerequisites are not mutation pending. Preflight every dirty batch before any write, preserve rejected drafts, and retain an independent save action when removing the selected or final override closes its editor. Removing an override must not be represented as deleting a builtin model/default.
- Successful source/target export does not prove prospective catalog pricing is compatible. Preserve task-usage versus fixed-request pricing rejection; carry a typed allowlisted conflict from its validation origin to sanitized management feedback, leaving unknown failures redacted. Failed unsent previews must invalidate stale apply choices without discarding durable operation/receipt recovery.
- Orphan-price cleanup needs explicit removal-only intent, separate from ordinary save or Reset. Compare against the public read version, using absent-metadata fallback currency only for that comparison; never write guessed metadata/currency. Detached pricing maps already contain inherited defaults, so remove leaves only from actually persisted option maps and leave absent maps absent. Prove default-only removal is a storage/runtime no-op and mixed stale/invalid batches remain atomic across all three database engines.

## Upstream upgrade acceptance

- Inspect a compiled billing default's provenance through configured-map presence, not expression equality. An administrator's identical numeric expression still has the metadata's configured currency; a USD fallback must never silently become a CNY quote. Validate only the actually selected identity/provider, and keep public/editor projections consistent with that selection.
- Scoped personal access tokens must intersect account permissions at secondary field-level checks, not only the route guard. An administrator identity does not make a personal usage scope an administrator-wide artifact read scope; test real persisted narrow tokens against sensitive field edits and foreign-owned artifacts.
- Backup restore verification must be isolated, resource bounded and verified before start. Docker PostgreSQL readiness must wait for its final server, not only the entrypoint's temporary initialization server. Restoring schema/data without owners/ACLs is not a complete disaster-recovery exercise.
- Specify the fork repository explicitly for GitHub workflow/status operations. A checkout with an upstream remote can make CLI inference select upstream despite origin pointing to the fork.

## Narrow offline catalog maintenance

- A standalone maintenance process cannot share the running application's catalog RWMutex or refresh its immutable caches. For explicitly approved offline repairs, gracefully stop the sole app instance under deployment locks, durably preserve raw preimages, use bounded exact-value CAS and postcondition checks, then start the same container for normal recovery. Never invent revision/digest/ready values. Reconcile an uncertain write read-only before any restoration; an unchanged preimage needs no restoring write.
- Qualify temporary tables as pg_temp in maintenance SQL, including client COPY: an explicit public-first search_path can otherwise resolve a same-name permanent table. PostgreSQL forbids CREATE TEMP TABLE inside READ ONLY transactions; prepare only temporary data first, then open the physically read-only transaction for persistent-table comparison. Verify these boundaries against real disposable PostgreSQL, including permanent-name collisions.
