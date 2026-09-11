# Review

## Root cause

The concurrent account deletion contract used the SQLite fallback even when CI
provided PostgreSQL. Under the race detector, SQLite intermittently returned
`SQLITE_BUSY`, so both requests could fail instead of producing one successful
deletion and one rejected duplicate.

## Change

The concurrency contract now requires `TEST_POSTGRES_DSN` and explicitly selects
the PostgreSQL-backed isolated security test database. Ordinary local runs without
PostgreSQL skip this integration contract instead of reporting an unreliable
SQLite result.

## Verification

- SQLite reproduction: failed once across five race runs with `SQLITE_BUSY`.
- PostgreSQL targeted race run: five of five passed.
- CI-equivalent race command passed for `common`, `middleware`, `service`,
  `controller`, `relay`, and `router`.
- `git diff --check` passed.

## Findings

- Critical: none.
- Warning: none.
- Info: the PostgreSQL test creates isolated databases on an ephemeral CI service;
  production and development databases are not used.
