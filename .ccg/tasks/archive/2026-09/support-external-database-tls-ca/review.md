# Review

## Scope

- Added explicit Redis CA loading for `rediss://` connections.
- Mounted each deployment target's `certs/` directory read-only at `/app/certs`.
- Protected the certificate directory with mode `0750`.
- Added the verified iXiaozu PostgreSQL and Redis TLS configuration examples.

## Security review

- Certificate verification remains enabled; no `InsecureSkipVerify` path was added.
- A custom Redis CA cannot be configured for a plaintext `redis://` URL.
- Missing or malformed Redis CA files cause startup configuration to fail.
- Existing environments remain compatible when `REDIS_TLS_CA_FILE` is unset.
- Runtime passwords and private infrastructure addresses were not added to Git.

## Verification

- `go test ./common -count=1`: passed.
- `bash deploy/tests/deploy_test.sh`: all 102 assertions passed.
- `git diff --check`: passed.
- `go test ./... -count=1`: all packages except `controller` passed. The unrelated existing `TestSecurityAccountDeletionConcurrentRequestsHaveOneWinner` test intermittently fails with SQLite `SQLITE_BUSY`; running it ten times reproduced three failures. The production targets use PostgreSQL and Redis, and this task did not modify that test or its code path.

The user explicitly requested that external dual-model review not be used, so this task received an inline review only.
