# CI diagnosis — archived, no implementation

User reported an error after the develop push. Read-only GitHub inspection confirms run 38047800232, head e81d4b9b8bcfa195994ab22bf44264c819ed75d7, completed with failure. Verify backend and frontend failed specifically at Test root and relaykit modules (`make test`). Frontend checks, builds and vets succeeded. Mandatory catalog PostgreSQL normal/race and lifecycle race steps were skipped, and Release was skipped. This run did not deploy dev.

The broad legacy entry point remains ahead of the new PostgreSQL gates. Makefile lines 76–82 run every root test package and relaykit. It does not set CATALOG_SYNC_POSTGRES_ONLY. Its preserved historical fixtures still use unsupported-for-this-task database paths; none were executed locally during this diagnosis.

Observed failures have multiple concrete fixture incompatibilities:

- controller/billing_option_test.go directly registers an in-memory plugin without matching persisted desired plugins. captureCatalogValidationInputTx correctly rejects that mismatch with `catalog plugin override publication is pending`.
- Legacy model-list and relay/helper fixtures migrate or directly assign model.DB without completing the production catalog runtime bootstrap/recovery. WithCatalogPricingReads requires catalogRuntime.ready and catalogRuntime.db == DB, so it rejects them with `catalog publication pending recovery`.
- catalogPostgresLegacyPrerequisite adapts older feature fixtures only under the explicit PostgreSQL-only flag. The legacy broad entry point does not enter that setup. In TestCatalogSyncSourceConcurrentMutation the ordinary vendor Insert can return before the Create callback; the test unconditionally waits on writerPaused at model/catalog_sync_test.go:2415. The actual timeout stack shows exactly that blocked receive, with no active writer goroutine. The model package timed out at 600 seconds.

Diagnosis: CI/fixture integration was incomplete despite the bounded PostgreSQL acceptance and frontend evidence. This is not proof that live dev has a pending publication, nor evidence that disabling runtime safety is appropriate. Existing success evidence must not be described as a passing broad suite or successful deployment.

Recommended separately authorized repair: make the deployment acceptance entry point explicitly PostgreSQL-only; adapt relevant legacy regression fixtures to real bootstrap and persisted plugin identities; add bounded failure-aware waits to the concurrent test; retain the publication safeguards and meaningful regression coverage. Do not merely skip failing assertions or remove safety guards. Run the exact corrected normal/race deployment checks before another push. No SQLite/MySQL reproduction, live data writes, external executors, workflow changes, rerun, push or deployment occurred during diagnosis.

Evidence: fresh gh run view status/jobs and failed-job logs; static Makefile, workflow, fixture and publication-guard tracing. The failed log output was initially truncated; exact timeout stack and assertion excerpts were subsequently isolated. No local tests were run because the request is diagnostic and known legacy entry points violate the user's PostgreSQL-only restriction. No spec update: .ccg/spec does not exist, and this report records the incident without introducing conventions.
