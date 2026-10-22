# Findings

## 1. Generic save offers protected Seedance models

Backend UpdateModelPricing rejects any of four dedicated Seedance model entries before beginning the pricing transaction (model/model_pricing_config.go:513-530). A mixed batch fails wholly. Frontend upstream selection/bulk selection does not exclude these models; applyPriceSyncSelections feeds buildPricingChanges and PATCH /api/option/model_pricing. General reset also includes every entry. The regular visual editor already hides protected Seedance entries and the single-model editor redirects to the dedicated tab.

Keep backend protection and the dedicated matrix save, which generates coherent expressions/CNY pricing. Correct the generic selection/reset/change-construction boundaries with explicit dedicated-tab messaging; do not silently claim excluded imports were saved. Existing frontend tests lack mixed protected-model coverage. The last label-only change did not introduce these selection/save rules.

## 2. Environment sync not provisioned

Read-only SSH using existing verified host and deployment identity confirmed hostname molii and healthy molii-production/molii-development containers. Both containers have zero CATALOG_SYNC_* environment variable names; no credential contents were retrieved. Production logs show authenticated GET /api/catalog_sync/status returning 503, including 17:34:51 and 17:38:16 server time, 2026-10-22. Public unauthenticated curl returns 401 as expected and is not evidence of administrator session failure.

Code defaults missing role to disabled. Even disabled status requires a configured external browser origin; ExternalOrigin() rejects the missing origin and status returns CATALOG_SYNC_UNAVAILABLE. Frontend renders any failed status query as the same generic unavailable title. This is independent from the protected-model save rejection.

Actually enabling dev-to-molii sync requires source/target protected runtime configuration, matching least-privilege reader credential/verifier, and validated deployment topology. Adding only the origin would make disabled/read-only status display, not enable synchronization. Configuration is captured once; runtime changes need controlled container replacement rather than editing DB options. No configuration changes or restarts were performed.

## Verification and review boundary

- Main application run 39742533810 completed successfully for all four production targets. Public Molii version is production-molii-936aade39dcb.
- Two independent read-only agents traced the separate chains. External antigravity/Claude wrapper unavailable; no external dual-model review claimed.
- Existing pricing-sync and catalog-sync frontend suites rerun: 2 files, 36 tests passed. This is baseline regression evidence, not a fixed mixed-Seedance regression or production-write test.
- Browser showed unavailable state. Browser diagnostics were interrupted; server logs supplied authoritative HTTP503 evidence. No production saves/sync operations were submitted.

Diagnosis complete. No implementation, push, deployment, account changes, credential creation or production data changes in this task.
