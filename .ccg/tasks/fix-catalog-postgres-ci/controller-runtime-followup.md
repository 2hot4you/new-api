# Controller runtime follow-up: first heartbeat clock boundary

## Scope and outcome

Investigated the actual full-race failure in `TestCatalogSyncTargetNamespaceManagedRollback/restore=false/aba=true`, which failed during `catalogSyncTarget` setup with `ErrCatalogInstanceIneligible`. The supplied gate log was `/var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/catalog-postgres-race-glkrba4h/controller-catalog_sync_test.jsonl`; no data-race report accompanied the assertion failure.

This is a production timestamp invariant defect, not a justified fixture-only repair. The initial diagnosis stopped at deterministic RED and requested scope direction. The root task then confirmed that the human's incident-fix authorization covers the bounded one-timestamp repair, explicitly adding `model/system_instance.go` and `model/catalog_sync_startup_test.go` to this agent's ownership. The authorized repair initializes a new row's `CreatedAt` with the same resolved heartbeat timestamp as `LastSeenAt` and `UpdatedAt`; the existing conflict-update columns and strict eligibility guard are unchanged. No retry, fake database timestamp, readiness bypass or guard relaxation was introduced.

## Evidence

The real `service.ReportCurrentSystemInstance` captures a Unix-second heartbeat timestamp and passes it to `model.UpsertSystemInstance`. Upsert initializes `LastSeenAt` and `UpdatedAt` with that value, but leaves `CreatedAt` zero. `SystemInstance.BeforeCreate` independently samples the clock to initialize creation time. Crossing the second boundary between those samples persists `CreatedAt > LastSeenAt`. `catalogSensitiveInstanceTx` intentionally rejects that ordering. The stale threshold is 90 seconds, not the approximately one-second interval observed in the failed fixture.

`TestCatalogSyncHeartbeatCreationAcrossSecond` calls the real reporter once using a disposable PostgreSQL database. A test-only GORM callback delays the first `system_instances` create until the next second, before the actual `BeforeCreate` hook. It does not modify any timestamps. The persisted row was:

```text
created_at=1791635710 last_seen_at=1791635709 updated_at=1791635709
```

The unchanged ordering assertion failed deterministically in 1.23 seconds (package 2.599 seconds), exit 1. This is an assertion failure, not a process timeout or a reported Go data race.

Before the production edit, the new model regression also failed: explicit heartbeat time `1791635956` was persisted with `CreatedAt=1791635958`, and actual `CreateCatalogSyncPlan` returned `ErrCatalogInstanceIneligible`. That package exited 1 in 1.674 seconds. The zero-timestamp default child passed; the defect was specifically the delayed explicit first heartbeat.

## Focused commands

All commands used `CATALOG_SYNC_POSTGRES_ONLY=1` and the owned task-only numeric-loopback PostgreSQL DSN. Controller TestMain opens no database; model TestMain explicitly took its guarded PostgreSQL branch. No SQLite/MySQL helper ran and no selected case skipped. Post-fix focused commands also set `GOWORK=off`.

```sh
go test ./controller -run '^TestCatalogSyncTargetNamespaceManagedRollback$/^restore=false$/^aba=true$' -count=1 -timeout=90s -v
# PASS: case 2.05s, package 3.127s

go test -race ./controller -run '^TestCatalogSyncTargetNamespaceManagedRollback$/^restore=false$/^aba=true$' -count=1 -timeout=90s -v
# PASS: case 2.83s, package 5.663s (intermittent original failure)

go test ./controller -run '^TestCatalogSyncHeartbeatCreationAcrossSecond$' -count=1 -timeout=90s -v
# RED: CreatedAt > LastSeenAt; case 1.23s, package 2.599s

go test ./model -run '^TestCatalogStartupHeartbeatTimestamps$' -count=1 -timeout=90s -v
# RED: delayed first heartbeat timestamp mismatch + actual eligibility failure; package 1.674s
```

## Focused GREEN verification

```sh
go test ./controller -run '^(TestCatalogSyncHeartbeatCreationAcrossSecond|TestCatalogSyncTargetNamespaceManagedRollback)$' -count=1 -timeout=90s -v
# PASS: package 8.780s; new boundary regression + all four restore/ABA rollback children

go test -race ./controller -run '^(TestCatalogSyncHeartbeatCreationAcrossSecond|TestCatalogSyncTargetNamespaceManagedRollback)$' -count=1 -timeout=90s -v
# PASS: package 14.285s; same complete focused selection, no DATA RACE

go test ./model -run '^TestCatalogStartup(HeartbeatTimestamps|EligibilityFacts|EnrollmentRequired|ApplyRechecksHeartbeat|HeartbeatLock|PendingEligibility|FinalAckEligibility|InstanceStorage)$' -count=1 -timeout=90s -v
# PASS: package 8.537s; eight top-level tests, both new timestamp children and all19 strict eligibility facts

go test -race ./model -run '^TestCatalogStartup(HeartbeatTimestamps|EligibilityFacts|EnrollmentRequired|ApplyRechecksHeartbeat|HeartbeatLock|PendingEligibility|FinalAckEligibility|InstanceStorage)$' -count=1 -timeout=90s -v
# PASS: package 12.354s; same complete focused selection, no DATA RACE

git diff --check -- model/system_instance.go model/catalog_sync_startup_test.go controller/catalog_sync_test.go
# PASS
```

The controller boundary test now persisted `created_at=last_seen_at=updated_at=1791636037` despite the same real pre-hook delay. The model regression verifies original creation time survives a later upsert, the heartbeat and update times advance, info updates, the zero default resolves within the actual call interval, and real catalog plan creation remains eligible. Existing authorization/single-instance assertions remain intact, with added `created-missing` and `created-after-heartbeat` rejection children proving the strict ordering requirement still applies.

## Exact owned changes and coverage boundary

- `model/system_instance.go`: one-field production correction; no hook or conflict-assignment changes.
- `model/catalog_sync_startup_test.go`: new `TestCatalogStartupHeartbeatTimestamps/explicit-create-and-update` and `/zero-timestamp-default`; extend existing `TestCatalogStartupEligibilityFacts` with `/created-missing` and `/created-after-heartbeat`.
- `controller/catalog_sync_test.go`: new `TestCatalogSyncHeartbeatCreationAcrossSecond`; no existing fixture or rollback assertions changed.
- This report only; root owns gate inventory and the independent review report.

The root task was notified of exact mandatory test names before acceptance. Source remained stable throughout focused GREEN runs. This agent did not rerun any whole acceptance suite and makes no claim that historical cross-engine suites passed. Root performs final PostgreSQL acceptance and integration; no live/user database or external executor was used.
