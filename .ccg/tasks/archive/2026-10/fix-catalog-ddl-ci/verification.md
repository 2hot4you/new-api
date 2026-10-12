# Verification

Owned disposable containers: new-api-ddl-ci-pg-20261012 (PostgreSQL 15.18, arm64, numeric loopback random port) and new-api-ddl-ci-redis-20261012 (Redis 7.4.9, numeric loopback random port). No production services or business data used.

RED: original TestCatalogSyncSourceDDLStabilityAndCancellation failed on iteration 2 of count=10, controller/catalog_sync_test.go:846, SQLSTATE 0A000 cached plan must not change result type. Original source held in base commit e05a25706.
GREEN: same unchanged test count=10 normal (4.163s) and race (5.558s), no failures/skips. Command: GOWORK=off CATALOG_SYNC_POSTGRES_ONLY=1 TEST_POSTGRES_DSN=<owned-loopback> TEST_CATALOG_SYNC_POSTGRES_DSN=<owned-loopback> go test [-race] -tags catalogsynctest ./controller -run '^TestCatalogSyncSourceDDLStabilityAndCancellation$' -count=10 -failfast -timeout=120s -v.

Diagnostic regression RED: missing catalog_sync_test.go:846. GREEN: five catalog gate tests passed; four outer gate tests passed. Full Python discovery: 17 passed.
Deployment shell contract: bash deploy/tests/deploy_test.sh, all192 assertions passed.
Independent read-only review: no Critical/Warning findings.

Normal acceptance: complete41 database-independent root packages /2249 events and9 relaykit packages /486 events passed. Initial make test-postgres passed18 catalog groups before main_channel_cache_test hit missing ignored web/dist embed input (setup failure, no failed behavioral assertion). Matched CI's mkdir -p web/dist + touch web/dist/index.html, then resumed the unchanged run_gate normal selector inventory from main_channel_cache_test through the end. All13 remaining groups passed. Combined31 distinct groups /1109 events, zero skips. This was segmented normal acceptance, not one uninterrupted successful make invocation.
Full make test-postgres-race exited0:41 root packages /2249 events,9 relaykit packages /486 events,31 catalog groups /1109 events; zero skips or failures. No source changed during normal/race acceptance. git diff --check passed. This is CI fixture/diagnostic-only work, not a change to business queries/migrations; no cross-engine compatibility claim is made. Frontend unchanged.
