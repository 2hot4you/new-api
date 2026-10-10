# Independent model concurrency review

Reviewed commit `0e026fb57266e333de53ad66f5c3be2a1d3be32a` (`model/catalog_sync_test.go`) against Task 3 and the global constraints. Read-only review of the diff plus the production vendor writer, catalog barriers/read snapshot, ordinary mutation budget, and owning PostgreSQL fixture. No tests repeated, no external executors, and no code changes.

## Findings

- Critical: none.
- Warning: none.
- Info: `readerStarted` signals that the reader goroutine started, not that it already blocked on the catalog read barrier. This limitation existed in the original test. The writer's explicit `TryRLock` exclusion assertion and resulting full snapshot assertions are retained, so the bounded-wait repair does not weaken that coverage.

## Reasoning

The independent database writer now inherits a 45-second context, reports its transaction result through a buffered channel, and is joined during cleanup after cancellation. Snapshot/export use that same context. An error or timeout returns through the original assertion instead of hanging on an unconditional receive.

The ordinary-writer coordinator watches both callback arrival and the buffered writer result, addressing the observed rejection-before-Create deadlock. It bounds waits with a five-second child context. `sync.Once` makes callback release idempotent; deferred cancellation/release runs on every return, including fatal assertions. The callback adds cancellation to the transaction rather than continuing a timed-out write. The worker result channels are buffered, and reader snapshot data is transferred by a result value rather than shared unsynchronized writes. Atomic callback observation is retained.

Worker joins happen before callback removal and before the owning fixture's global/database cleanup. The 35-second cleanup allowance exceeds the ordinary production writer's 30-second work budget. If a worker cannot exit, the test records a failure rather than silently accepting it. The reader's lock acquisition itself is not context-aware in production, but releasing the paused writer in every return path frees that lock dependency; no new production bypass was introduced.

Original snapshot consistency, vendor coverage, catalog barrier exclusion, and revision expectations remain. The new PostgreSQL-only pre-Create rejection case requires the injected sentinel, proves the Create callback was not reached, and proves neither a vendor row nor catalog revision committed. Matrix engines are unchanged; the deployment runner selects the required PostgreSQL child rather than running SQLite/MySQL.

## Assessment

Ready for root's complete normal/race acceptance and final review. This report is source-review evidence only, not an additional test-pass claim.
