# Independent diagnosis: source DDL-stability fixture scheduling

Analysis only. Inspected the actual failure log, `controller/catalog_sync_test.go:804-848`, and `model/catalog_sync_references.go:467-567`. No tests rerun, database/server calls, application edits or child agents. The failure is real evidence, not a reason to retry blindly or relax the production proof.

## Cause

The current test pauses after the source's actual vendors query, launches an **autocommit** `ALTER TABLE models ADD COLUMN ddl_probe integer`, and proves that PostgreSQL reports a real lock wait. It then resumes source and asserts both export and DDL succeed.

However, the export deliberately commits its READ ONLY/REPEATABLE READ transaction at `model/catalog_sync_references.go:544`, releasing ACCESS SHARE, **then** captures fresh relation/namespace evidence at553 and compares full MVCC tokens at557-559. The queued ALTER can obtain ACCESS EXCLUSIVE and commit before this fresh proof. Its changed `pg_class` tuple must invalidate the export. The observed `catalog metadata changed during read` is the intended fail-closed outcome of that schedule; healthy source rows do not make the metadata proof optional.

The test has synchronized “DDL was blocked by the read transaction,” but has not synchronized “DDL commits only after the final proof.” Its unconditional line826 NoError assertion assumes the latter without establishing it. This is unrelated to the new pricing error projection.

## Preferred smallest deterministic positive schedule

Keep the existing vendors Query callback and real `pg_stat_activity` Lock assertion. Change **only the fixture DDL's transaction/commit timing**:

1. Start the DDL goroutine with `db.Transaction(func(ddlTx *gorm.DB) error { ... })`, not autocommit `db.Exec`.
2. Inside that transaction execute the same real ALTER. It still actually blocks behind source ACCESS SHARE and the existing test must observe that wait before resuming source.
3. After ALTER's `Exec` returns successfully, close a `ddlApplied` channel and wait on an `allowDDLCommit` channel **before returning nil from the transaction callback**. The ALTER now really ran and owns its transaction lock; its catalog changes are still uncommitted.
4. Main resumes source, observes `ddlApplied` with a timeout, then requires source export NoError (and ideally complete snapshot). The final metadata SELECT reads committed catalog tuples; the DDL's private uncommitted update cannot replace the original committed proof.
5. Main closes `allowDDLCommit`, waits for the DDL result and requires NoError, then removes the pause callback and continues the existing cancellation assertions unchanged.

Pseudocode for the fixture only:

```go
ddlApplied, allowDDLCommit := make(chan struct{}), make(chan struct{})
allowCommit := sync.OnceFunc(func() { close(allowDDLCommit) })
defer allowCommit() // must run even if the main test fails
go func() {
    ddl <- db.Transaction(func(ddlTx *gorm.DB) error {
        if err := ddlTx.Exec("ALTER TABLE models ADD COLUMN ddl_probe integer").Error; err != nil {
            return err
        }
        close(ddlApplied)
        <-allowDDLCommit
        return nil
    })
}()
catalogSyncWaitForLock(t, db, "ALTER TABLE models ADD COLUMN ddl_probe")
resume()
// Bounded wait for ddlApplied OR an early DDL error, then source result.
// require.NoError(t, sourceResult); require.True(t, snapshot.Complete)
allowCommit()
// Bounded wait for ddl result; require.NoError(t, ddlResult)
```

An existing GORM callback can hold the same phase: a DDL-transaction-only `Raw().After("gorm:raw")` handler, filtered to the exact ALTER, pauses after real SQL completes but before the enclosing explicit transaction commits. The direct wait after `Exec` is simpler and avoids intercepting unrelated raw statements. An `After("gorm:raw")` pause on an **autocommit** Exec is too late—the database has already committed—so it does not fix the race.

This preserves the positive NoError assertions by arranging the schedule that warrants them. It neither skips ALTER nor delays the real ALTER lock acquisition, and requires no production hook, weaker fence, fake SQL result, altered isolation or retry. The implementer must verify this schedule on the disposable PostgreSQL fixture; this analysis does not claim a passing run.

## Cleanup requirements

- Register one-shot release of both source and DDL-commit barriers before any failing assertion; arrange cleanup so these releases precede fixture pool/drop cleanup. Drain/join both goroutines with bounded waits before the test returns.
- Do not call `require`/`t.Fatal` inside the DDL goroutine/its callback: return the error to the buffered result channel. Otherwise Goexit can leave the source/DDL barrier held.
- When waiting for `ddlApplied`, also observe the DDL error channel so an ALTER failure is reported immediately rather than becoming a confusing timeout. Set bounded fixture deadlines; do not leave a transaction hanging on a failed source assertion.
- Do not start the later ACCESS EXCLUSIVE cancellation blocker until DDL commit has completed. Leave the existing cancellation error, process-barrier release and final fresh-export NoError assertions intact.

## Justified deterministic fail-closed alternative

If deliberately testing the opposite schedule, retain the original autocommit ALTER but force its commit **before** the helper's final metadata read. A `Row().Before("gorm:row")` callback can identify the postcommit `SELECT ... FROM pg_catalog.pg_class ...` pass: arm a flag only in the actual vendors projection callback (preflight and in-transaction verification already completed), filter the metadata SQL, and optionally require `Statement.ConnPool` is the reserved `*sql.Conn` rather than the `*sql.Tx`. The metadata code uses `.Raw(...).Row().Scan`, so Query or Raw callbacks are not the right callback family for this SELECT.

At that boundary source has committed and released its lock. Wait, with a deadline, for the already queued ALTER to finish/commit; then let the real metadata SELECT execute. Require the **specific** metadata-change error, require returned snapshot not complete, and require the ALTER succeeded. A subsequent fresh export must succeed, proving rejection was due to intervening DDL rather than permanent fixture corruption. Remove the callback before cancellation/fresh-export checks.

That is a valid separate fail-closed test, not a reason to permit arbitrary errors. Do not use `if err != nil { return }`, accept either result unconditionally, drop line826, relax `sameCatalogReadFence`, move the production check back inside stale repeatable-read state, or suppress this error. The preferred positive schedule above is the smaller adjustment that retains this test's original success intent and real-lock proof.
