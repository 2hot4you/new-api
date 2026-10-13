# Evidence and bounded repair plan

Failed run: 38213115322 at e05a25706cac598a17f91dec830ebb54535357f6. Deployments skipped.
Local isolated PostgreSQL 15.18 container new-api-ddl-ci-pg-20261012 reproduced failure on iteration 2 of a bounded count=10 diagnosis. Assertion controller/catalog_sync_test.go:846: load managed models: cached plan must not change result type (SQLSTATE 0A000).
Production model/main.go chooseDB uses PreferSimpleProtocol=true, while controller catalogSyncPostgres uses postgres.Open defaults. Existing model catalog fixtures already set PreferSimpleProtocol=true.

1. Align only controller catalog fixture connection protocol with production; preserve all DDL and cancellation assertions. Prove before/after with real disposable PostgreSQL and focused normal/race tests.
2. Improve gate failure evidence with bounded, credential-redacted test diagnostics and behavioral tests; do not print raw environments or DSNs.
3. Run affected controller gate selection and Python gate tests, then complete PostgreSQL normal/race acceptance if prerequisites allow. No business changes planned; report coverage accurately.
4. Independent review, archive task, and propose PR into develop. No direct main push or production operations.

External analysis/review prerequisite: specified codeagent-wrapper absent at /Users/naf/.claude/bin/codeagent-wrapper and no installed alternative found. User explicitly authorized independent review on 2026-10-12; independent reviewer completed analysis and final code review with no Critical/Warning findings.
