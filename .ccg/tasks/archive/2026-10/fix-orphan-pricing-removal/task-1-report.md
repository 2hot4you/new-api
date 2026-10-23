# Task 1 backend handoff

Status: focused verification GREEN; owned implementation and test files stable. Full acceptance and independent reviews remain coordinator-owned.

## Files changed

- `model/model_pricing_config.go`: optional JSON `remove,omitempty`; reject conflicting Reset, nonempty Pricing, or nonblank currency and blank identities; preserve Seedance preguard. Compare the captured complete read pricing projection and exact currency version (USD read fallback only when metadata is absent). Remove leaves only through maps present in captured persisted options, including all flattened provider identities. Do not queue currency writes. Ordinary writes, Reset, preparation, authoritative input recheck, transaction and runtime publication remain unchanged.
- `model/model_pricing_removal_test.go`: new `TestModelPricingRemovalDatabaseMatrix` with real SQLite/MySQL/PostgreSQL children and six scenario groups each. Explicit real actor, migrations, isolated singleton identity/heartbeat and recovery on every engine. Exercises JSON remove intent rather than relying on a test-only struct field.
- `deploy/tests/test_catalog_postgres_gate.py`: add the exact new parent to existing `model-runtime-pg`, retaining `matrix=True` and every existing selection.
- This report.

## Regression-first evidence

Before any production change, added and formatted the new regression file, then ran exactly:

```sh
env GOWORK=off CATALOG_SYNC_POSTGRES_ONLY=1 TEST_POSTGRES_DSN='postgresql://postgres:orphan-pricing-local-only@127.0.0.1:32768/new_api_ci?sslmode=disable' TEST_MYSQL_DSN='root:orphan-pricing-local-only@tcp(127.0.0.1:32769)/new_api_ci?charset=utf8mb4&parseTime=True&loc=Local' go test ./model -run '^TestModelPricingRemovalDatabaseMatrix$/^postgres$' -count=1 -timeout=180s -v
```

RED, exit 1, `model` 1.408s. Fixture/recovery succeeded and ordinary orphan rejection passed. Removal failed with `model qwen3.7-max exact metadata is required before saving pricing`; conflicting-intent validation and stale-version tests encountered the same missing feature. CNY/builtin and absent-default removal also failed at the unchanged metadata prerequisite. This was a runtime behavioral failure, not a missing-field compiler error or missing fixture.

After the narrow implementation, the identical command passed, exit 0, `model` 1.764s. All six PostgreSQL scenarios passed, no skips.

## Three-engine focused GREEN

```sh
env GOWORK=off CATALOG_SYNC_POSTGRES_ONLY='' TEST_POSTGRES_DSN='postgresql://postgres:orphan-pricing-local-only@127.0.0.1:32768/new_api_ci?sslmode=disable' TEST_MYSQL_DSN='root:orphan-pricing-local-only@tcp(127.0.0.1:32769)/new_api_ci?charset=utf8mb4&parseTime=True&loc=Local' go test ./model -run '^TestModelPricingRemovalDatabaseMatrix$' -count=1 -timeout=180s -v
```

PASS, exit 0, `model` 4.067s; parent 3.27s. Actual versions: SQLite 3.50.4 (0.18s), MySQL 8.4.11 (2.33s), PostgreSQL 15.18 (0.77s). All three engine children and all 18 scenario children passed; zero skips.

Scenarios assert actual option rows, metadata rows, public read versions, typed runtime caches, and durable catalog state:

1. Ordinary orphan add/edit/empty-save/currency and Reset still reject without mutation.
2. Remove combined with Reset/pricing/USD/CNY and blank identity reject; Seedance remains managed even with conflicting intent.
3. Stale remove and mixed batches containing stale remove, negative pricing, undeclared provider expression, or ordinary orphan save reject atomically, preserving raw rows, caches, metadata, versions and catalog state.
4. Successful two-orphan batch deletes persisted legacy/model/provider leaves, preserves unrelated exact raw option text and unrelated leaves, leaves absent ModelRatio absent, changes versions and digest, increments revision once and acknowledges the committed runtime.
5. Exact CNY metadata row is unchanged; removing an explicit builtin override exposes its compiled fallback rather than deleting the model/default.
6. Inherited legacy pricing, default-only builtin pricing and never-configured identities are storage/runtime/revision no-ops, without materializing absent maps or suppressing defaults.

## Other validation

- `python3 -m unittest deploy.tests.test_catalog_postgres_gate`: PASS, 6 tests, exit 0.
- `git diff --check`: PASS, exit 0.
- `gofmt -w model/model_pricing_removal_test.go` and `gofmt -w model/model_pricing_config.go` completed successfully.

## Boundaries and remaining work

No blockers found. No commits, pushes, live configuration changes, external wrappers, model calls or spawned agents. External antigravity/Claude wrapper substitution is coordinator-managed and disclosed: native independent analyses/reviews replace the unavailable wrappers. Read the approved brief, requirements, global plan, backend/general specs and full billing/expr documentation; did not re-run orchestration or follow AGENTS workflow as a dispatched implementer.

Only the four owned files above were edited by this worker. Preserved pre-existing deleted task files, untracked duplicate files and the frontend worker's disjoint edits. Fixtures used only coordinator-owned loopback disposable databases; each real fixture creates/drops its own isolated database and restores process state. Full Go/front-end gates, race runs and independent final review are intentionally delegated to the coordinator, not claimed by this scoped report.
