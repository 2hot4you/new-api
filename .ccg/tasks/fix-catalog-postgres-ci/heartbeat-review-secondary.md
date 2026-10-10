# Secondary independent heartbeat review

Assessment: **Approved**. Bounded read-only review of the current heartbeat correction and regressions; only this report was written. No tests, database operations, external executors or subagents were run.

## Critical

None.

## Warning

None.

## Info / evidence

- Read the incident amendment in plan.md and `.ccg/spec/guides/index.md`, complete `model/system_instance.go`, `model/catalog_sync_startup.go`, `model/catalog_sync_startup_test.go` and `service/system_instance.go`, plus the complete changed controller test and its disposable fixture. The only production change is `CreatedAt: lastSeenAt` in `model/system_instance.go:59`.
- The sole production Upsert caller is `ReportCurrentSystemInstance` (`service/system_instance.go:123`), which captures the observation timestamp before the create operation. Upsert already resolves zero timestamps once and initializes LastSeenAt/UpdatedAt from that value. Initializing CreatedAt from the same value prevents the later BeforeCreate clock sample from violating the first observation ordering. The hook still supplies defaults to other direct creates; marshal/error handling, report identity and heartbeat cadence are unchanged.
- OnConflict remains keyed only by node_name and still updates only info, started_at, last_seen_at and updated_at (`model/system_instance.go:62–70`). It deliberately does not update created_at, preserving the original row's creation timestamp on subsequent heartbeats, including process restart reports. No identity, authorization, lock or catalog eligibility condition was relaxed. The unchanged guard still rejects absent/nonpositive creation times, creation after last observation, future/stale heartbeats, inconsistent update time, mismatched instance identity/start metadata and unsafe storage (`model/catalog_sync_startup.go:20–95`). This is not a live repair of previously stored rows.
- `TestCatalogSyncHeartbeatCreationAcrossSecond` (`controller/catalog_sync_test.go:564–588`) delays the real service report's GORM create path before gorm:before_create, targeting the next Unix second from the captured LastSeenAt. It does not fabricate timestamps or retry the report. The delay is normally at most approximately one second; if processing has already crossed the boundary, no extra delay is needed. The callback is removed by cleanup and the test verifies the callback was reached plus the persisted ordering/update invariant. This is a controlled clock-boundary reproduction, not a probabilistic arbitrary sleep.
- `TestCatalogStartupHeartbeatTimestamps` (`model/catalog_sync_startup_test.go:57–104`) covers an explicit older first observation, subsequent conflict update preserving creation identity while changing timestamp/info, and the zero-input timestamp default bounded by before/after wall-clock samples. It invokes real CreateCatalogSyncPlan after each relevant state, so successful catalog eligibility is tested in addition to timestamp assertions. The fixture starts the process ten seconds before the current time, keeping the intentionally older observation valid. New created-missing and created-after-heartbeat scenarios in TestCatalogStartupEligibilityFacts prove the strict guard still rejects malformed rows with ErrCatalogInstanceIneligible.
- Both new top-level tests are explicitly selected in their existing controller/model gate groups (`deploy/tests/test_catalog_postgres_gate.py:310`, `:440`); the existing strict eligibility suite remains selected. No new engine branch or fixture-ready bypass is introduced.

Verification boundary: this is source approval, not an independent GREEN claim. Test execution and final combined normal/race acceptance remain controller/implementer responsibilities. No historical cross-engine full-suite result is implied.
