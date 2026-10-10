# PostgreSQL CI fixtures

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
