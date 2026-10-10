# Task 2: Relay/helper PostgreSQL fixture repair

## Scope and result

Test-only repair in `relay/catalog_pricing_capture_test.go`, `relay/relay_task_test.go`, `relay/helper/price_test.go`, `relay/helper/price_identity_test.go`, and `relay/helper/molii_grok_price_test.go`. `relay/relay_task_retry_billing_test.go` needed no edit: its existing assertions use the amended owning fixture. No production changes, assertion removals, new skips, SQLite/MySQL execution, external model executors, or child spawning.

The helper TestMain branches on any nonempty `CATALOG_SYNC_POSTGRES_ONLY` before legacy initialization. Only exactly `1` is accepted. It validates a numeric loopback PostgreSQL URL, a database, disabled TLS, and the pgx-resolved host/database/TLS/fallback facts before running even an empty selection. Each selected persistent helper regression creates its own disposable PostgreSQL database, complete reference schema, catalog migrations, option bootstrap, and actual runtime recovery. Previously missing HTTP request fixtures now contain real requests for request-scoped pricing capture.

Relay request fixtures reuse `catalogPricingPostgres`. That fixture now restores typed config, OptionMap, FX/SystemName, eight pricing maps, DB/registry/dialects and cache/Redis/batch/log flags. Helper fixtures restore the equivalent globals they modify. Existing per-test cleanup executes before fixture restoration. Sequential normal and race runs exercise the entire selected group in one process.

Relay plugin fixtures save real active/enabled `TaskPlugin` rows with exact compiled API version, plugin version, source and SHA-256 before installing the matching effective registry/pin. No fabricated ready marker or consistency bypass is used. Actual catalog startup happens before request fixture changes. Existing malformed expressions and historical/narrowed usage-schema cases remain request-level tests: their existing typed setters are restored after baseline startup, rather than attempting to adopt invalid request inputs as startup catalog configuration. Re-running full recovery after those intentional request inputs would reject them before their original request assertions; this is not the production periodic plugin-sync behavior either.

## RED evidence

All commands used `CATALOG_SYNC_POSTGRES_ONLY=1`, the supplied disposable loopback PostgreSQL DSN, and `GOWORK=off`.

After routing helper fixtures safely to PostgreSQL and supplying HTTP requests, but before real catalog bootstrap/recovery:

```sh
go test ./relay/helper -run '^(TestPerCallPricingUsesBillingIdentity|TestMoliiGrokFixedPriceAnchorsAreAvailable|TestModelPriceHelperTieredUsesFrozenBillingModelCurrency)$' -count=1 -v
```

All three failed with `catalog publication pending recovery`, proving the fixture prerequisite rather than hiding the failures.

Relay bootstrap initially replaced request settings configured before fixture setup:

```sh
go test ./relay -run '^(TestRelayTaskSubmitTieredUsesFrozenBillingModelCurrency|TestTaskRetryRecomputesInferredBillingAndPreservesOverride)$' -count=1 -v
```

Retry children failed their existing `Should not be: "model_price_error"` expectations; frozen-currency submission failed its existing snapshot assertion. Preserving the original typed request inputs after real startup repaired this fixture-order defect. An exploratory attempt to recover again after plugin/request changes rejected deliberately stale/incompatible schemas at recovery, so it was removed in favor of the existing request-boundary behavior described above.

## GREEN evidence

PostgreSQL server observed: PostgreSQL 15.18, aarch64 Alpine. No selected owned tests contain `t.Skip`/`t.Parallel` paths. The normal and race invocations below passed with all selected top-level tests and their existing children.

Helper selector: 19 top-level tests: all 16 tests in `price_test.go`, price identity, fixed Grok anchors, and the pure default Grok anchor test.

```sh
go test ./relay/helper -run '^(TestModelPriceHelper.*|TestFixedPricePreConsumeAndRealtimeRejection|TestInputPreConsumeMultiplierLegacyAndRequestPrices|TestPerCallPricingUsesBillingIdentity|TestMoliiGrokModelsUseDirectCostAnchors|TestMoliiGrokFixedPriceAnchorsAreAvailable)$' -count=1 -timeout=3m
go test -race ./relay/helper -run '^(TestModelPriceHelper.*|TestFixedPricePreConsumeAndRealtimeRejection|TestInputPreConsumeMultiplierLegacyAndRequestPrices|TestPerCallPricingUsesBillingIdentity|TestMoliiGrokModelsUseDirectCostAnchors|TestMoliiGrokFixedPriceAnchorsAreAvailable)$' -count=1 -timeout=5m
```

Normal PASS 15.221s; race PASS 14.884s.

Relay selector: 21 top-level tests, including all eight original failed relay business regressions, the other owning submit/mapping fixtures, retry children, and eight existing catalog capture tests.

```sh
go test ./relay -run '^(TestRelayTaskSubmit.*|TestTaskRetryRecomputesInferredBillingAndPreservesOverride|TestTaskSelfUseDefaultDoesNotMaskMappedExpression|TestSharedTaskBillingExpressionSelectionAndFrozenSettlement|TestEstimateTaskSubmitReusesBillingWithoutPreconsumingOrCallingUpstream|TestApplyChannelPinPreservesOriginTasksAndRetryMode|TestCatalogRequest.*)$' -count=1 -timeout=5m
go test -race ./relay -run '^(TestRelayTaskSubmit.*|TestTaskRetryRecomputesInferredBillingAndPreservesOverride|TestTaskSelfUseDefaultDoesNotMaskMappedExpression|TestSharedTaskBillingExpressionSelectionAndFrozenSettlement|TestEstimateTaskSubmitReusesBillingWithoutPreconsumingOrCallingUpstream|TestApplyChannelPinPreservesOriginTasksAndRetryMode|TestCatalogRequest.*)$' -count=1 -timeout=5m
```

Normal PASS 35.702s; race PASS 44.662s. `gofmt` and exact-owned-path `git diff --check` also passed.

The original failed relay names retained are:

- `TestRelayTaskSubmitTieredUsesFrozenBillingModelCurrency`
- `TestRelayTaskSubmitAliasBillingIdentityAndExprFallback`
- `TestRelayTaskSubmitPerCallBillingIdentity`
- `TestEstimateTaskSubmitReusesBillingWithoutPreconsumingOrCallingUpstream`
- `TestSharedTaskBillingExpressionSelectionAndFrozenSettlement`
- `TestRelayTaskSubmitAcceptsAnySuccessfulUpstreamStatus`
- `TestTaskRetryRecomputesInferredBillingAndPreservesOverride`
- `TestTaskSelfUseDefaultDoesNotMaskMappedExpression`

The helper selector includes every original failed helper name unchanged; its only additional pure anchor is `TestMoliiGrokModelsUseDirectCostAnchors`. Root owns the exact mandatory gate inventory and whole acceptance entry point.

## Fail-closed evidence

All probes use `GOWORK=off go test ./relay/helper -run '^$' -count=1`; each exited 1 before tests or database setup.

| Input | Diagnostic |
| --- | --- |
| nonempty mode `invalid`, empty DSN | `CATALOG_SYNC_POSTGRES_ONLY must be exactly 1 when set` |
| mode `1`, empty DSN | `helper PostgreSQL gate requires a loopback TEST_POSTGRES_DSN URL with sslmode=disable` |
| mode `1`, DSN `not-a-postgresql-url` | same loopback-URL diagnostic |
| mode `1`, loopback URL with `&host=192.0.2.1` | `helper PostgreSQL tests reject ambiguous TEST_POSTGRES_DSN overrides` |

## Self-review and limits

Reviewed the owned diff: original business expectations remain intact; schemas and readiness are initialized through public production paths; plugins are not runtime-only; owned disposable databases close before dropping; snapshots restore fixture globals; nonempty invalid mode cannot enter SQLite TestMain. No material finding remained. Independent root review and complete mandatory acceptance are still required. This is PostgreSQL-only selected acceptance, not a claim that every historical legacy cross-engine suite was migrated or passed.

## Independent-review follow-up: standalone DSN isolation

Independent review found that `dbname=postgres` or `database=postgres` could agree with the original URL path during validation but override the subsequently rewritten disposable database path. The root runner already rejected unknown query keys; the standalone helper now independently requires exactly one query key, `sslmode`, with exactly one value, `disable`, via `url.ParseQuery`. Query database/host/port/service aliases and duplicate SSL mode are rejected before pgx parsing or opening a database. A username, single database path, and no fragment are also required.

Parser-only `TestHelperPostgresDSNGuard` was added before the repair. RED: `dbname=postgres`, `database=postgres`, same port, same host and duplicate SSL mode unexpectedly returned nil errors. No database was opened. After repair, this test and all six override children pass: normal 1.339s, race 2.791s. Its accepted URL path is rewritten to `catalog_helper_guard` and pgx must resolve that exact database, proving isolation survives rewriting.

Five actual TestMain probes with mode `1` and the task loopback URL plus `dbname=postgres`, `database=postgres`, `port=32768`, `service=fixture`, or `host=192.0.2.1` each exited 1 before tests/database setup, with the unchanged diagnostic `helper PostgreSQL tests reject ambiguous TEST_POSTGRES_DSN overrides`. These used `GOWORK=off go test ./relay/helper -run '^$' -count=1`. Root was given the new exact test name for mandatory inventory inclusion. This follow-up changes only the helper test fixture and this report; whole acceptance evidence is owned by root.
