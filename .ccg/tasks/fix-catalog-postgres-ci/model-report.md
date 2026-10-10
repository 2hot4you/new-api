# Task 3 model concurrency subpart

Status: complete, pending controller review. Scope: only `model/catalog_sync_test.go` and this report. No production changes, fixture-ready fabrication, CI edits, external executors, subagents, push, SQLite execution, or MySQL execution.

## Behavior

`TestCatalogSyncSourceConcurrentMutation/postgres` keeps its actual repeatable-read assertions: an independent committed DB writer changes the vendor description/model currency between reads, while the source transaction still exports `before` / `USD`. It still exercises real `Vendor.Insert`, observes the common writer at Create, starts source capture during that writer, and verifies two exported vendors and revision 1.

All channel waits in that test and its new test-only `catalogConcurrentVendorSnapshot` coordinator are bounded. The source phase uses a 45-second context and the independent SQL writer receives that context. Each ordinary-writer/read overlap uses a 5-second child context; waiting for Create selects the callback signal, the writer's early result, or cancellation. Writer and reader completions are bounded. Reader results travel in a buffered result channel rather than a shared snapshot variable.

Create callbacks select release/cancellation and reject cancellation even when both are ready. Idempotent release and cancellation run in deferred cleanup on every exit. Cleanup joins launched workers before callback removal and fixture/global cleanup, with a separate 35-second cap matching the ordinary production writer's existing 30-second budget. The independent SQL writer also has a cancellation-aware bounded cleanup join. No timeout or expected failure is converted to a success/skip.

In the same PostgreSQL child, a query callback injects the sentinel `injected catalog writer failure before Create` into an actual vendor read. Assertions prove the original error propagates, the Create observer was never reached, no third vendor committed, and the catalog revision remains unchanged. The successful and rejected writes use the same coordinator; the failure case is not a mock coordinator result. Existing bootstrap/recovery is reused unchanged.

## RED → GREEN evidence

Every command used these exact environment values:

```sh
CATALOG_SYNC_POSTGRES_ONLY=1
TEST_POSTGRES_DSN='postgresql://postgres:catalog-sync-test-only@127.0.0.1:32768/postgres?sslmode=disable'
```

Commands (environment set on each invocation):

```sh
go test ./model -run '^TestCatalogSyncSourceConcurrentMutation$/^postgres$' -count=1 -timeout=90s -v
go test -race ./model -run '^TestCatalogSyncSourceConcurrentMutation$/^postgres$' -count=1 -timeout=90s -v
```

RED: before adding the early writer-result branch, the bounded coordinator selected only Create/cancellation. Both commands failed the real injected-error assertion: expected sentinel in error chain, got `context deadline exceeded`. Normal child 6.54s / package 7.451s; race child 6.35s / package 7.678s. The five-second cancellation cap made this regression terminate rather than hang. Logs: `/tmp/pg-source-injection-red.log`, `/tmp/pg-source-injection-race-red.log`.

GREEN: adding the writer-result select case propagated the sentinel immediately. Final runs after cancellation cleanup self-review: normal child 0.88s / package 1.734s; race child 1.87s / package 3.992s; both exit 0, no skips or race diagnostics. Logs: `/tmp/pg-source-final-green.log`, `/tmp/pg-source-final-race-green.log`. Each output contains exactly the requested parent and `/postgres` child. Actual server logged PostgreSQL 15.18. `git diff --check` passed.

The original successful overlap was also checked before the injected case and passed; the actionable RED specifically demonstrates swallowed early writer failure, not a claim that every existing execution hung. Broad suites and other database engines were intentionally not executed under the task's PostgreSQL-only exact-selector restriction; complete CI gate validation remains controller-owned.

## Self-review

- Original snapshot, row-count, barrier, and revision assertions retained; injected failure adds rollback/non-observation checks.
- Callback release is idempotent, cancellation precedes cleanup release, and canceled callbacks attach the context error before SQL Create.
- Workers are joined before removing callbacks or restoring global state; buffered result sends cannot strand worker completion after caller cancellation.
- No shared snapshot variable is read during a canceled reader; all unrelated working-tree changes remain untouched.
