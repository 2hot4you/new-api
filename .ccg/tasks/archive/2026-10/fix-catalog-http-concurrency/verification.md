# Verification

Base38793748924f176fc680a7a2f5ddb8b683ddb586; failed main run39227619885, same SHA development run39227556806 passed.
Owned isolated PostgreSQL15.18 arm64 container new-api-http-ci-pg-20261020 and Redis7 container new-api-http-ci-redis-20261020, random numeric-loopback ports, disposable test databases only. No production operations.

Unmodified count10 test passed locally; this does not disprove timing failure. Deterministic temporary GORM callbacks paused A before proof UPDATE and B during its stored-plan query holding catalog writer. Existing test assertions failed exactly as Actions: success0, operationcount0, digest-query404 rather than409. Responses were503 CATALOG_SYNC_UNAVAILABLE and403 SECURITY_PROOF_CONSUMED. Probe removed after diagnosis.

Final change synchronizes prechecks before actual competing proof UPDATEs. No production changes/retries/sleeps/skips. Exact assertion strengthened from positive successes to exactly1 success and explicit consumed-proof rejection.

Focused normal count10: PASS27.575s. Focused race count10: PASS49.856s. Command with owned TEST_POSTGRES_DSN and CATALOG_SYNC_POSTGRES_ONLY=1 GOWORK=off: go test [-race] -tags catalogsynctest ./controller -run '^TestCatalogSyncHTTPConcurrentProofAndExactBinding$' -count=10 -timeout=180s -v.
Full selected controller-catalog_sync_http_test race group via existing gate.run_gate:33 tests/subtests passed, zero skips.
Python deployment contracts: python3 -m unittest discover -s deploy/tests -p 'test_*.py':17 passed.
git diff --check passed; independent review no Critical/Warning.
Complete make test-postgres exited0:41 root packages/2319 tests and subtests,9 relaykit packages/486,31 mandatory catalog groups/1109; zero skips. Same source throughout acceptance. Full cross-engine suites and frontend not run locally (test-only change). Full normal/race/frontend remote PR CI remains pending.
