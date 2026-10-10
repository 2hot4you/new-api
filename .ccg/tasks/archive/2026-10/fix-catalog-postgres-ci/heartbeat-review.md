# Independent first-heartbeat correction review

Reviewed `41e5f190e` and warning-resolution `b22a0e385`, the amended Task 4 plan, repository AGENTS, `.ccg/spec/guides/index.md`, and `controller-runtime-followup.md`. Supporting reads traced the real service reporter, constructor/hook/upsert, catalog startup eligibility and lifecycle tests, and pinned GORM timestamp-update behavior. This reviewer made no source changes, ran no Go suites, used no external executor, and wrote only this report.

## Findings

- Critical: none.
- Warning: none remaining.
- Resolved Warning W1: the two new created-time rejection cases initially used GORM `Update`, which automatically changes the `UpdatedAt` field. After a second boundary, they could falsely pass because of the separate unchanged update-time guard. `b22a0e385` changes only those corruptions to `UpdateColumn` and reads back the entire persisted row before attempting real plan creation. The expected row differs from the healthy snapshot only in `CreatedAt`. This independently proves `LastSeenAt`, `UpdatedAt`, node identity, start and info remain healthy, isolating the creation invariant. All 19 facts passed in the owner's focused normal/race rerun.
- Info: verification is PostgreSQL-only under the explicit incident restriction. The one-field constructor change uses existing portable ORM behavior and adds no schema/dialect branch, but this review does not claim execution on SQLite/MySQL or a completed cross-database compatibility matrix.

## Production correctness and scope

The production change is exactly `CreatedAt: lastSeenAt` in the new `SystemInstance` value, after the existing zero timestamp resolution. New-row creation now uses one resolved observation timestamp for creation, last-seen and update. The GORM BeforeCreate hook is unchanged and cannot replace the provided nonzero creation value. Existing conflict-update columns still exclude `created_at`, so later heartbeats preserve creation identity while updating info/start/last-seen/update as before. The only production caller is the real service reporter; there is no retry, live-row rewrite, forged readiness, clock override or new API/policy change.

`catalogSensitiveInstanceTx` is unchanged: explicit single-instance permission, manual safe identity, master role, healthy self row, positive/start/fresh/future checks, `CreatedAt > 0`, `CreatedAt <= LastSeenAt`, `UpdatedAt == LastSeenAt`, payload schema/identity/start, other live/future node rejection, PostgreSQL relation/primary-key/storage/RLS/inheritance checks and transactional table lock all remain. Existing enrollment, apply recheck, heartbeat lock, pending recovery, final acknowledgment and storage tests remain selected alongside the additions.

## Regression quality and cleanup

The controller regression calls `service.ReportCurrentSystemInstance` once. Its synchronous callback delays only the actual first `SystemInstance` create, before `gorm:before_create`, until the captured heartbeat's next Unix second. This deliberate boundary scheduling is the defect stimulus, not a random sleep, fake timestamp or tolerance relaxation. It asserts the persisted ordering and equal update time. Callback removal is registered before the reporter call, runs before fixture pool/database teardown, and still executes on a fatal assertion. No worker/goroutine is introduced by this test, so the local observation flag has no new race.

The model test's explicit delayed timestamp deterministically reproduces the old constructor defect without clock mocking, checks exact creation/heartbeat/update equality, and invokes real `CreateCatalogSyncPlan` to prove actual eligibility. A later upsert must preserve creation time, advance heartbeat/update, preserve start and update info. The zero timestamp case bounds the resolved observation to the real call interval and checks all three fields agree. The additional invalid creation-time rows still fail actual plan creation, with entire-row readback preventing another eligibility fault from masking them.

## Evidence reviewed

Owner-recorded deterministic RED: controller persisted `created_at=1791635710` with `last_seen_at=updated_at=1791635709`; model explicit first-seen `1791635956` acquired creation `1791635958` and actual plan creation returned `ErrCatalogInstanceIneligible`. These failures preceded the one-field production repair.

Owner-recorded focused GREEN on stable repair: controller boundary test plus all four managed rollback children normal 8.780s / race 14.285s; eight model startup top-level tests (new two timestamp children plus existing lifecycle guards and 19 eligibility facts) normal 8.537s / race 12.354s. The final W1 isolation/readback follow-up ran all 19 facts again: normal 2.934s / race 3.402s. Reports record no failures, skips or DATA RACE. Review did not repeat those suites or relabel these focused results as complete acceptance.

Root's mandatory inventory includes `TestCatalogSyncHeartbeatCreationAcrossSecond` and `TestCatalogStartupHeartbeatTimestamps` (31 groups / 372 top-level tests). Root still owns fresh complete normal/race acceptance, final static/build checks, archive and push/workflow evidence. The old historical cross-engine full-suite boundary remains unchanged.

## Assessment

Approved for final root validation. The bounded production repair fixes the reported ordering defect while preserving existing creation identity, strict eligibility guards and all meaningful assertions.
