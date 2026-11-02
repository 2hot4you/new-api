# Verification in progress

## RED → GREEN

New controller handler/storage/status tests initially failed at the same Invalid Passkey domain error (two implicit-origin cases plus three credential-removal cases). Minimal production retention guard then passed new and existing Passkey cases. Explicit invalid historical settings still fail with unchanged DB/runtime; no insecure-origin switch enabled.

## Completed

- SQLite3.50.4: selected TestPasskey* plus GetStatus privacy suite PASS25top-level tests; new regression cases including enabled=false/true and empty/[] origins PASS.
- MySQL8.4.11, disposable task-owned container: same selected suite PASS25top-level tests,35.694s.
- PostgreSQL15.18, disposable task-owned container: same selected suite PASS25top-level tests,55.180s. Fresh isolated main/log DB names and proper pool close/drop; unrelated local development DB not used.
- `GOWORK=off go test ./setting/system_setting ./service/passkey -count=1` PASS (service/passkey has no test files).
- Existing frontend `bun run test src/features/keys/components/__tests__/api-addresses.test.tsx`:5/5PASS.
- Python CI verifier/gate suites:22tests PASS; independent reviewers additionally validated target/time-window tests6+2PASS.
- Scoped gofmt and git diff --check PASS.
- Focused real PostgreSQL Passkey race suite PASS25top-level tests,95.314s.

## Pending

Full audited `make test-postgres` normal acceptance (complete independent root/relaykit/tokenkit suites already green; mandatory PostgreSQL groups running), remote full normal/race/frontend verification, dev exact deployed version, main promotion and three production releases. No broad historical make-test claim: attempted unfiltered controller execution was stopped because the project requires audited DB selections rather than legacy implicit DB suites.

Containers created solely for this task: server-address-test-postgres-20261102 (127.0.0.1:32768), server-address-test-mysql-20261102 (32769), server-address-test-redis-20261102 (32770). Remove only these exact labelled fixtures after local verification; do not touch molii-new-api-dev-* services.
