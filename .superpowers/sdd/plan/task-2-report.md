# Task 2 — backend rc.36 merge

Status: DONE_WITH_CONCERNS. All owned non-web conflicts resolved and staged; no commit created. No web or .ccg files edited by this worker.

## Implementation

- Preserved Molii terminal task CAS and durable billing outbox for every polling completion/failure path, including bounded rc.36 single/batch failures. Adapted StarAI, Molii Grok and test doubles to full-task polling APIs. Counter reset, missing-batch-result handling, plugin-state bounds and private-data sanitization retained.
- Added regression tests for threshold-to-one-outbox behavior, repeated terminal polls and oversized plugin state. Accounting metadata excludes private upstream identifiers.
- Adopted typed LogOther/AuditOther, actor-role audit context, unified login/security/password contracts and route labels. Kept transactional registration/default token creation. Fixed auto-merged token deletion bypass of protected default-token policy.
- Kept local catalog authority, publication intent, immutable model identity, Molii fields and ordering. Remote sync controller and synthetic pricing-default catalog remain deleted. Added upstream channel listing/endpoint metadata compatibility without replacing local authority. Catalog operations share marketplace-first lock ordering; vendor validation remains transactional.
- Pricing removal coupled to metadata deletion is explicitly rejected: Molii custom pricing must be edited separately, avoiding partial deletion of custom pricing maps.
- Preserved canonical client model identity while adopting separate billing model identity. Kept Molii performance counters and legacy recent_success_rates alongside upstream series.
- Dependency union retains Molii SDKs. PostgreSQL migration wrapper/outbox/file migrations preserved. Fixed private-task serialization dropping stored-result-only values.

## Files resolved or semantically adapted

Configuration/dependencies:
- .env.example
- go.mod
- go.sum

Controllers/routes:
- controller/channel.go
- controller/channel_task_plugin_bind_test.go
- controller/channel_test_molii_grok_test.go
- controller/model_management_test.go
- controller/model_meta.go
- controller/model_meta_catalog_test.go
- controller/model_sync.go (retained deletion)
- controller/relay.go
- controller/token.go
- controller/user.go
- controller/vendor_meta.go
- controller/vendor_meta_pricing_test.go
- controller/wechat.go
- middleware/audit.go
- router/api-router.go

Models:
- model/log.go
- model/main.go
- model/marketplace_display_order_test.go
- model/model_catalog_metadata_test.go
- model/model_catalog_reconcile_test.go
- model/model_meta.go
- model/model_metadata_sync.go
- model/postgres_migration_integration_test.go
- model/pricing_default.go (retained deletion)
- model/task.go
- model/vendor_meta.go

Relay/performance:
- pkg/perf_metrics/metrics.go
- pkg/perf_metrics/metrics_test.go
- pkg/perf_metrics/types.go
- relay/channel/task/moliigrok/adaptor.go
- relay/channel/task/moliigrok/adaptor_test.go
- relay/channel/task/starai/adaptor.go
- relay/channel/task/starai/adaptor_test.go
- relay/common/relay_info.go
- relay/helper/model_mapped.go
- relay/helper/price.go

Services:
- service/gpt_image_2_log.go
- service/gpt_image_2_log_test.go
- service/task_billing.go
- service/task_billing_molii_grok_test.go
- service/task_billing_reconcile.go
- service/task_billing_test.go
- service/task_polling.go
- service/task_polling_molii_grok_test.go
- service/task_polling_rc36_test.go (new)
- service/task_polling_test.go
- service/text_quota.go
- service/text_quota_test.go

Relevant auto-merges inspected include OAuth transactional registration, security middleware, auth-session setup, task artifact handling, database initialization and upstream plugin polling adaptors.

## Validation

PASS:
- go mod tidy.
- Nested relaykit: go test ./... (all packages).
- Full service, common, relay/helper, middleware, pkg/perf_metrics, StarAI and Molii Grok suites passed during this worker run.
- Full model suite passed before final vendor/catalog transaction refinements. Latest broad model run exposed one missing Option migration fixture; fixture fixed and focused model rerun passed.
- go test ./service -run 'TestPoll(FailureThreshold|Oversized)' -count=1.
- go test ./controller -run 'Test(AddChannelTaskPlugin|UpdateMoliiGrokChannel|ModelMarketplaceRule|UpdateVendorMeta|DefaultTokenResponses|SecurityAccountDeletion|VideoProxyKeeps)' -count=1.
- Router tests passed.
- Final compile: go test ./controller ./model ./relay/helper -run '^$'.
- git diff --check for non-web/non-.ccg changes.
- Final git diff --name-only --diff-filter=U returned empty (all agents' conflicts resolved).

NOT GREEN / pending parent integration:
- Full controller suite failed; not rerun broadly after focused fixes, per parent request to finish and centralize integration validation.
- Remaining catalog matrix cases assume upstream synthetic default vendors, automatic public visibility for routable models, mutable model names, remote ApplyMetadataSync import behavior or coupled pricing removal. Those assumptions conflict with retained Molii catalog contracts. Tests must be reviewed/reconciled rather than relaxing those contracts.
- TestGetModelMetaDynamicBlockersDoNotChangePublicationIntent has global default ratio/test-isolation sensitivity.
- SecurityAccountDeletionConcurrentRequestsHaveOneWinner failed in one broad SQLite fixture run but passed focused. PostgreSQL acceptance remains required.
- Root go test ./... compilation was blocked by missing web/dist embed before frontend build; nested/module/backend compilation succeeded.
- PostgreSQL + Redis disposable acceptance was not run: no test service DSNs supplied to this worker. SQLite unit fixtures are not acceptance evidence.
- A temporary experimental controller test rewrite had compile errors and was reverted to the staged implementation; final compile above passed. No broad-suite success is claimed.

## Review attention

1. Run full PostgreSQL/Redis acceptance and root build after frontend output exists.
2. Review catalog matrix expectations against intentional Molii policy; do not restore remote catalog authority or partial pricing deletion to satisfy upstream-only tests.
3. Poll batch oversized-result handling rejects persistence of oversized plugin state, but some transient in-memory task fields may already have been decoded before failure classification; review alongside plugin-state hardening.
4. Run final dual-model review and full test suite centrally; this report is not a claim of release readiness.

## Fix round 1 — controller/catalog compatibility

Status: DONE. Parent authorized a follow-up commit after the merge commit.

Files changed:
- controller/model_management_test.go
- controller/model_meta_catalog_test.go
- model/model_metadata_sync.go
- .superpowers/sdd/plan/task-2-report.md

Investigation and changes:
- Reproduced the six reported failure groups. SquareState operational/routing visibility was correctly computed; upstream tests incorrectly equated it with Molii publication. Assertions now explicitly require unpublished drafts/channel-only records to stay out of the public catalog.
- Reproduced the internal metadata compatibility helper's JSON-default RETURNING scan failure with a targeted failing regression. Replaced map creation with typed Model creation through the existing display-order helper, preserving JSON serializer handling, draft publication defaults and explicit disabled status. Kept optimistic-version concurrency checks; the regression asserts one import winner, one conflict, stored array defaults and no implicit publication. No remote synchronization route was restored.
- Reworked rename expectations to assert model_name_immutable and no partial update, followed by a successful same-name metadata edit that preserves channel bindings, pricing identity and local sync preference.
- Replaced synthetic-provider-brand expectations with a positive/negative local-publication test: channel-only data creates no public brands; explicit complete published local metadata exposes its saved vendor; deletion does not synthesize or recreate the brand.
- Metadata deletion tests now assert rejection of combined remove_pricing requests without side effects, then independently exercise metadata/channel deletion and versioned pricing reset. Exact-rule channel-removal validation is tested without an unrelated prohibited pricing flag.
- Dynamic-blocker fixture used gpt-4, whose built-in pricing depends on suite-global settings. Changed to a unique unpriced fixture, preserving the intended publication/blocker assertions.

Fresh validation:
- Before fix: targeted legacy_metadata_import regression failed with the reported JSON string-to-*[]string RETURNING scan error.
- PASS: go test ./controller ./model -run 'Test(ModelManagementDatabaseMatrix|VendorManagementDatabaseMatrix|ModelDeletionDatabaseMatrix|GetModelMetaDynamicBlockersDoNotChangePublicationIntent|ModelCatalog|Marketplace|Vendor)' -count=1 (controller 1.706s; model 2.540s).
- PASS: go test ./controller -count=1 (30.675s).
- PASS: git diff --check on the three changed Go files.

The previously reported full-controller failures are resolved. PostgreSQL/Redis deployment acceptance remains a root-agent integration gate; SQLite fixtures passing here are unit compatibility evidence only. No web files were edited by this worker.
