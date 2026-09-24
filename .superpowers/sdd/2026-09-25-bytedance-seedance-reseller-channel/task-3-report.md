# Task 3 — ByteDance Seedance connection test and model synchronization

## Files changed

- `controller/channel-test.go`: read-only `/v1/models` channel test; failure is visible to health checks.
- `controller/channel_upstream_update.go`: validate absolute non-self Base URL, use the shared Bearer model-list fetch, strictly parse and filter authorized Seedance models, and apply upstream revocations during automatic sync.
- `controller/channel_test_bytedance_seedance_test.go` (new): fetch, validation, no-generation test, refresh, local disable, pricing/options preservation, and failure retention coverage.

## TDD evidence

- RED: `go test ./controller -run 'ByteDanceSeedance.*(FetchModels|Refresh|ChannelTest)' -count=1` failed on double-slash model path and generic generation-test fallback.
- RED: `go test ./controller -run 'ByteDanceSeedance.*Refresh' -count=1` failed because revoked models were not removed and malformed empty lists were accepted.
- RED: `go test ./controller -run 'ByteDanceSeedanceUnsupportedOnlyRefresh' -count=1` failed because unsupported-only lists could clear old models.
- RED: `go test ./controller -run 'ByteDanceSeedanceChannelTestFailure' -count=1` failed because test failures were not visible to health checks.
- GREEN: `go test ./controller -run 'Channel.*(Model|Fetch|Update|Test)|ByteDanceSeedance' -count=1` passed.
- GREEN: `go test ./controller -count=1` passed.
- GREEN: `git diff --check` passed.
- `go test ./... -count=1` ran all non-root packages successfully, but the root package failed setup because `main.go` embeds missing `web/dist` assets. This is a pre-existing checkout/build-artifact prerequisite, not a controller test failure.

## Behavior and limitations

The native `UpstreamModelUpdateIgnoredModels` list is the explicit local-disable mechanism. The refresh test confirms an ignored authorized model is not re-enabled. Successful automatic sync removes models revoked upstream. Malformed, empty, and unsupported-only lists are errors and retain persisted models. The persistence test confirms `Other` (local pricing/currency fixture) and `Setting` (transport options fixture) remain unchanged. No price, ratio, currency, or option sync was added.

The requested project-native SQLite channel/model harness was used for persistence tests. Save-time form validation lives outside Task 3 file ownership; this task validates Base URL whenever connection testing or model discovery runs.

## Fix Round 1 — review findings

1. `TestByteDanceSeedanceConnectionTestCannotOverrideConfiguredBearer` exercises the real connection test against an HTTP server that returns 401 unless it receives `Bearer instance-key`. A custom `Authorization` override now cannot replace the configured key. Other channel types still use the unchanged shared header behavior.
2. `TestByteDanceSeedanceFetchModelsRejectsAlternateSelfAddresses` covers the same DNS host over another scheme and `localhost` versus IPv4/IPv6 loopback aliases on the same port. These are rejected before an HTTP request.
3. `TestByteDanceSeedanceScheduledRevocationReportsRemovedModel` checks that a revocation-only scheduled scan reports one changed channel and one detected removal, persists the narrower model list, and leaves no stale pending removal.

RED command:

```text
go test ./controller -run 'ByteDanceSeedance(ConnectionTestCannotOverrideConfiguredBearer|FetchModelsRejectsAlternateSelfAddresses|ScheduledRevocationReportsRemovedModel)' -count=1
--- FAIL: TestByteDanceSeedanceConnectionTestCannotOverrideConfiguredBearer: status code: 401
--- FAIL: TestByteDanceSeedanceFetchModelsRejectsAlternateSelfAddresses: alternate scheme and IPv4/IPv6 loopback aliases were not rejected
--- FAIL: TestByteDanceSeedanceScheduledRevocationReportsRemovedModel: expected changed_channels=1, actual=0
FAIL
```

GREEN commands/output:

```text
go test ./controller -run 'ByteDanceSeedance(ConnectionTestCannotOverrideConfiguredBearer|FetchModelsRejectsAlternateSelfAddresses|ScheduledRevocationReportsRemovedModel)' -count=1
ok  github.com/QuantumNous/new-api/controller  1.361s
go test ./controller -run 'Channel.*(Model|Fetch|Update|Test)|ByteDanceSeedance' -count=1
ok  github.com/QuantumNous/new-api/controller  1.275s
go test ./controller -count=1
ok  github.com/QuantumNous/new-api/controller  39.190s
git diff --check
(no output; exit 0)
```

## Fix Round 2 — same hostname, different service port

`TestByteDanceSeedanceFetchModelsPermitsSameHostDifferentPort` uses a real model-list server on `127.0.0.1` while the instance address is `http://127.0.0.1:443`; it verifies that the distinct upstream port is allowed and the authorized Seedance model is fetched. The direct-self check now compares effective ports for the same hostname, while retaining the existing no-explicit-port alternate-scheme guard and the loopback-alias guard.

RED command/output:

```text
go test ./controller -run 'ByteDanceSeedanceFetchModelsPermitsSameHostDifferentPort' -count=1
--- FAIL: TestByteDanceSeedanceFetchModelsPermitsSameHostDifferentPort
    Received unexpected error: ByteDance Seedance Base URL cannot point to this instance
FAIL
```

GREEN commands/output:

```text
go test ./controller -run 'ByteDanceSeedance' -count=1
ok  github.com/QuantumNous/new-api/controller  1.313s
go test ./controller -run 'Channel.*(Model|Fetch|Update|Test)|ByteDanceSeedance' -count=1
ok  github.com/QuantumNous/new-api/controller  1.080s
go test ./controller -count=1
ok  github.com/QuantumNous/new-api/controller  32.787s
git diff --check
(no output; exit 0)
```
