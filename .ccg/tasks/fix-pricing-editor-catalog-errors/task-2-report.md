# Task 2 report: typed catalog pricing conflicts

## Scope and behavior

Owned application files only:
- `model/model_pricing_config.go`
- `model/catalog_sync_pricing_conflict_test.go` (the one new backend test file)
- `controller/catalog_sync.go`
- `controller/catalog_sync_http_test.go`
- `web/src/features/system-settings/catalog-sync/index.tsx`
- `web/src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx`

`ModelPricingValidationConflict` wraps the existing error at the three task smoke-test rejection origins (model provider, explicit provider, resolved alias). Classification runs only after rejection and uses the existing compiling `billingexpr.UsesFixedPricing` helper. `.Error()` delegates to the prior formatted error and `.Unwrap()` returns it. Existing unchanged-single-provider, stale-override, shadowing and settlement/fixed-pricing decisions are unchanged.

Catalog HTTP uses `errors.As` after existing higher-priority error responses. Only `task_fixed_pricing_unsupported`, a valid plugin key, and a bounded conservative ASCII model identity are projected as 422 `CATALOG_PRICING_CONFLICT`. The response includes safe identity/reason plus `common.NewMessage` fields; it never renders the error/cause or expression. Unrecognized reasons/identities/errors remain generic503. Audit outcome remains a fixed code.

The existing destructive Alert/error helper renders the actionable message. Failed sync or restore previews clear only unsent preview/selection/dialog state. They do not clear durable operation binding, receipt, session storage or original-result recovery. This applies consistently to unknown preview failures too. No retry was added.

No edits were needed in catalog validation/apply staging. No SQL/schema/auth/plugin lifecycle, metadata eligibility, transaction, attestation, accepted-pricing or persistence behavior changed. Therefore this patch does not itself introduce a new three-engine DB-behavior matrix requirement. The genuine HTTP regression ran against a fresh disposable PostgreSQL15.18 fixture database created and cleaned by the existing helper; it never migrates/writes the base admin database. PostgreSQL coverage is not a claim of SQLite/MySQL verification.

## Translation handoff

One exact new key (already sent to coordinator/locale owner), parameters `model` and `provider`:

`Catalog pricing for model {{model}} conflicts with provider {{provider}}. Fixed request pricing cannot be used for task usage billing. Review the source pricing configuration.`

No locale files modified by Task2.

## RED evidence

1. `GOWORK=off go test ./model -run '^TestCatalogSyncGrokFixedPricingConflict$' -count=1`
   - Initial compile RED: new typed error did not exist.
   - After adding only the error-type scaffold, behavioral RED: create/adopt stage cases and direct validation failed `require.ErrorAs`: actual chain remained the old `*fmt.wrapError`/`*errors.errorString`, not `*model.ModelPricingValidationConflict`. The old rejection text was already asserted unchanged.
2. From `web/`: `npm test -- src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx -t 'pricing conflict invalidates|unknown preview failure'`
   - RED: 3 failed / 23 intentionally unselected. HTTP422, business-failure and unknown-failure cases all found the stale enabled `Review and confirm` button after the failed replacement preview.
3. `GOWORK=off go test -tags catalogsynctest ./controller -run '^TestCatalogSyncHTTP(GrokPricingConflict|PricingConflictProjection)$' -count=1` with the supplied disposable-fixture `TEST_POSTGRES_DSN`.
   - Projection RED: typed wrapped conflict expected422, actual503.
   - The new real HTTP fixture initially failed snapshot validation (missing price Unit, then incorrect JSON-pointer path). Corrected the fixture to the existing manifest contract, `mode`/`expression` and `/grok-imagine-video`; did not weaken validation. These setup failures were not claimed as behavioral RED.

## GREEN verification

All Go commands use `GOWORK=off`. DB commands use coordinator-supplied loopback `TEST_POSTGRES_DSN` and the existing exact-name disposable database fixtures; DSN credentials are omitted from this report.

- `go test ./model -run '^TestCatalogSyncGrokFixedPricingConflict$' -count=1 -v`: PASS; actual xAI source, create and strict/adopt cases, unchanged preservation, alias/provider identity, old message/unwrap, generic compiler/schema/runtime errors, valid shadowing override.
- `CATALOG_SYNC_POSTGRES_ONLY=1 go test ./model -run '^(TestCatalogSyncGrokFixedPricingConflict|TestCatalogSyncValidationPreservation|TestCatalogSyncValidationRuntimeStates|TestCatalogRuntimeModelPricingSave|TestCatalogRuntimeManagedResetAndStrictAdoption)$/postgres$' -count=1`: PASS, `ok .../model 15.015s`. Pure create/adopt subtests are verified separately by the previous command, since `/postgres$` intentionally selects only matrix PG children.
  - An initial invocation without `/postgres$` failed the existing gate on SQLite/MySQL children of `TestCatalogRuntimeModelPricingSave` and `TestCatalogRuntimeManagedResetAndStrictAdoption`. Corrected command selection; no test/guard code was changed. This was a runner mistake, not an application failure.
- `go test -tags catalogsynctest ./controller -run '^TestCatalogSyncHTTP(GrokPricingConflict|PricingConflictProjection)$' -count=1 -v`: PASS, `ok .../controller 5.196s`; real authenticated PostgreSQL preview422 and all11 projection cases pass. Hashes of models/vendors/options/plans/operations/baselines/catalog state unchanged after failure; coordination anchor intentionally excluded.
- `go test ./pkg/billingexpr ./setting/billing_setting -count=1`: PASS, both full package suites.
- From `web/`, `npm test -- src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx`: PASS, 27/27, including existing unknown-commit/proof-refusal/receipt recovery tests and a new durable succeeded-binding preservation case.
- From `web/`, `npm run typecheck`: PASS, exit0.
- From `web/`, `./node_modules/.bin/oxlint -c .oxlintrc.json src/features/system-settings/catalog-sync`: PASS, exit0.
- From `web/`, `./node_modules/.bin/oxfmt --check src/features/system-settings/catalog-sync/index.tsx src/features/system-settings/catalog-sync/__tests__/catalog-sync.test.tsx`: PASS, all matched files correct.
- `git diff --check -- controller/catalog_sync.go controller/catalog_sync_http_test.go model/model_pricing_config.go web/src/features/system-settings/catalog-sync`: PASS.

- Full existing catalog HTTP suite: `go test -tags catalogsynctest ./controller -run '^TestCatalogSyncHTTP' -count=1`: PASS, `ok .../controller 121.235s`, exit0. Existing auth/session/proof/SSRF/unknown-commit/publication tests retained.

Coordinator owns full frontend suite/build, normal/race PostgreSQL acceptance and independent review. No external models were invoked or claimed.

## Commit

`81f770e40f7c4d53a404c1b3ee2b7c7cfdbaf671` — `fix(catalog): report safe pricing conflicts and invalidate stale previews`

Exactly the six owned application paths were staged and committed with explicit paths. This report stays in the plan's ignored SDD workspace. No pushes/merges/server or dependency changes performed.

## Remaining concerns

- Conservative unusual/non-ASCII model identities deliberately retain generic503 instead of exposing unchecked configuration text.
- The incompatible source quote is still rejected. No quote conversion/removal, catalog apply, source mutation or production write was performed.
- Working tree contains concurrent Task1/coordinator changes; all preserved. Only exact owned application paths will be staged/committed.

## Follow-up: mandatory CI inventory

Independent Task2 review passed. Integration found that the exact-list mandatory PostgreSQL gate did not include the three new regressions. Coordinator extended ownership only to `deploy/tests/test_catalog_postgres_gate.py` and its existing unit tests for this follow-up.

- Added `TestCatalogSyncGrokFixedPricingConflict` to `model-store-direct` (`./model`, nonmatrix), so both create/adopt subtests execute instead of being filtered by a `/postgres` suffix.
- Added `TestCatalogSyncHTTPGrokPricingConflict` and `TestCatalogSyncHTTPPricingConflictProjection` to `controller-catalog_sync_http_test` (`./controller`, nonmatrix).
- Added one inventory guard unit test asserting required group, package, nonmatrix contract and presence. Existing required names and all runner/verifier behavior remain unchanged. No production files changed in this follow-up.

RED: `python3 deploy/tests/test_catalog_postgres_gate.py CatalogGateTest.test_pricing_conflict_regressions_are_required_in_nonmatrix_groups -v` failed both group subtests before list additions: pure model conflict and the two HTTP regressions were absent. An initial direct `validate_regression_inventory` version reported the same omissions as two expected ValueErrors; changed the check to a normal inclusion assertion for clearer failure output, then observed both assertion failures before adding list entries.

GREEN:

- `python3 deploy/tests/test_catalog_postgres_gate.py -v`: 6/6 PASS.
- `python3 deploy/tests/test_postgres_ci.py -v`: 4/4 PASS.
- Focused runner imported the actual updated `GROUPS`, verified all three new names using `validate_regression_inventory`, derived the following nonmatrix selectors from those lists, ran with the same gate environment (`GOWORK=off`, `CATALOG_SYNC_POSTGRES_ONLY=1`, `TEST_SECURITY_DIALECT=postgres`, supplied loopback disposable-fixture DSN), and passed every Go JSON result through existing `verify_events`:
  - `go test ./model -run '^(TestCatalogSyncGrokFixedPricingConflict)$' -count=1 -failfast -timeout=900s -json`: PASS, 3 tests/subtests, zero skips.
  - `go test -tags catalogsynctest ./controller -run '^(TestCatalogSyncHTTPGrokPricingConflict|TestCatalogSyncHTTPPricingConflictProjection)$' -count=1 -failfast -timeout=900s -json`: PASS, 13 tests/subtests, zero skips.
- `git diff --check -- deploy/tests/test_catalog_postgres_gate.py`: PASS.

Follow-up commit: `872ce69f4004160efcd52efe11628555eb3c767b` — `test(catalog): require pricing conflict regressions in postgres gate`. Exactly one scoped file committed (16 insertions). Coordinator is running normal/race acceptance; no claim here that an old-inventory run executed these additions. No pushes, spawned agents or production changes.

## Follow-up: deterministic source DDL fixture schedule

Coordinator explicitly extended ownership only to `TestCatalogSyncSourceDDLStabilityAndCancellation` in `controller/catalog_sync_test.go` after integration exposed a preexisting scheduling flaw. Read the independent `.ccg/tasks/fix-pricing-editor-catalog-errors/analysis-ci-ddl.md` diagnosis and the unchanged production final-proof implementation before editing.

Existing actual RED was read directly, not regenerated by arbitrary repeats:

- `/var/folders/sz/8v337tbs5njdfdf9w3rh3yxc0000gn/T/catalog-postgres-normal-fzo8y4qe/controller-catalog_sync_test.jsonl`
- At `2026-10-23T00:07:37+08:00`, `TestCatalogSyncSourceDDLStabilityAndCancellation` failed at `catalog_sync_test.go:826` with `catalog metadata changed during read`.
- Main summary `/tmp/molii-pricing-repair-verify.Wdi5yR/postgres-normal.log` confirms that mandatory controller selection failure.

The autocommit ALTER could commit after source READ ONLY COMMIT released ACCESS SHARE but before the fresh final metadata proof. Correctly rejecting that intervening DDL was production fail-closed behavior, not a pricing regression. The fixture now orders its intended positive schedule: real ALTER inside an explicit cancellable transaction; observe the same actual PostgreSQL lock wait; resume source; observe successful ALTER execution via `ddlApplied`; require complete successful source export/final proof while DDL is uncommitted; then release commit and require DDL success. `sync.Once` release and deferred cancellation/bounded worker join protect all failure paths. Existing cancellation, writer-barrier release and final fresh-export assertions remain. No sleeps, retries, test skips, timeout increases, accepted-arbitrary-errors or production changes.

GREEN commands, each exactly once (`-count=1`) against PostgreSQL15.18 using coordinator's fresh loopback32770 fixture-admin DSN; the existing helper creates/closes/drops only its owned timestamped databases:

- `CATALOG_SYNC_POSTGRES_ONLY=1 TEST_SECURITY_DIALECT=postgres GOWORK=off go test -tags catalogsynctest ./controller -run '^TestCatalogSyncSource(DDLStabilityAndCancellation|ReadOnlyCoherentSnapshot|RejectsPrelockReplacement)$' -count=1 -v`: PASS, all3, `ok .../controller 1.872s`, exit0.
- Same command with `-race`: PASS, all3, `ok .../controller 5.589s`, exit0; no race report or skips.
- `gofmt -w controller/catalog_sync_test.go` and `git diff --check -- controller/catalog_sync_test.go`: PASS.

Commit: `9a58dffaeb3d8b58a504e2f4991404ddaaa663ad` — `test(catalog): order ddl commit after source metadata proof`. Exactly the owned fixture in one test file changed (68 insertions,6 deletions, including bounded failure cleanup); no production proof/contract/SQL behavior change. No pushes or spawned agents. Coordinator owns full normal/race gate rerun and independent review.
