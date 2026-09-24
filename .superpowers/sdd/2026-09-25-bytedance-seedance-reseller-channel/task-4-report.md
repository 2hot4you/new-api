# Task 4 report — temporary assets

## Implementation

- Added provider-neutral channel resolution and asset HTTP request helper. New assets prefer an enabled ByteDance Seedance channel; legacy bindings without `channel_type` remain direct StarAI. Disabled/replaced channels can fall back only within the same provider type.
- Persisted `channel_type` in the existing Redis JSON binding; no schema migration. Preserved the raw upstream asset ID as the local ID.
- Creation now refreshes status and upstream `expires_at` immediately if the create response omits expiry. An earlier upstream expiry shortens the local Redis TTL and updates the COS cleanup index.
- Local user binding lookup remains before user query, delete, and service generation verification. Reseller 404/410 verification does not falsely expire a binding, and reseller upstream diagnostics are not exposed or logged.

## TDD evidence

- RED: `go test ./controller ./service -run 'TemporaryAsset|StarAIAsset' -count=1` failed on missing `ChannelType` fields and `resolveTemporaryAssetChannel`.
- RED: `go test ./controller -run TestTemporaryAssetResellerFailureDoesNotExposeUpstreamDiagnostics -count=1` failed because the upstream private code/reason appeared in the response and log.
- GREEN: both focused test commands passed after implementation.

## Validation

- `go test ./controller ./service -run 'TemporaryAsset|StarAIAsset' -count=1` — PASS.
- `go test ./controller ./service -run 'Asset|COSUpload' -count=1` — PASS.
- `go test ./controller ./service -count=1` — PASS.
- `git diff --check` — PASS.

## Round 1 security-review fixes

- The ByteDance Seedance request builder now verifies every image/video/audio `asset://` reference using `RelayInfo.UserId` and the selected reseller channel before submit. It preserves the exact URI on the generation wire. Missing/deleted, cross-user, expired, and non-success bindings stop before generation.
- Reseller HTTP-200 FAILED envelopes now persist and return only stable `temporary_asset_failed` / `temporary asset processing failed` values. Unknown status text is not persisted or exposed. Direct StarAI sanitization is unchanged.
- Past or boundary upstream expiry is rejected and an existing local binding is removed. An upstream expiry learned during generation verification is enforced too.
- Reseller binding TTL is capped at 168 hours even if configured higher. When required create refresh returns no upstream expiry, the 168-hour local cap remains; failed refresh removes the new local binding.

### RED evidence

`go test ./controller ./service ./relay/channel/task/bytedanceseedance -run 'TemporaryAssetReseller' -count=1` exited 1 with:

```text
--- FAIL: TestTemporaryAssetResellerHTTP200FailureDoesNotPersistPrivateDiagnostics
    "private_code" should not contain "private"
--- FAIL: TestTemporaryAssetResellerRefreshRemovesPastExpiry
    Expected error with "temporary asset has expired upstream" in chain but got nil.
--- FAIL: TestTemporaryAssetResellerRejectsExpiredTimestampsAndCapsUnknownExpiry
    Expected error with "temporary asset has expired upstream" in chain but got nil.
--- FAIL: TestTemporaryAssetResellerFailedEnvelopeHidesPrivateReason
    "temporary asset upstream verification failed: private account diagnostic" should not contain "private"
--- FAIL: TestTemporaryAssetResellerGenerationChecksOwnerAndPreservesRawURI
    An error is expected but got nil.
--- FAIL: TestTemporaryAssetResellerGenerationRejectsDeletedExpiredAndNonSuccessBindings
    An error is expected but got nil.
```

`go test ./service -run TestTemporaryAssetResellerVerificationRejectsUpstreamExpiredTimestamp -count=1` exited 1 with `Expected error with "temporary asset has expired upstream" in chain but got nil.`

`go test ./service -run TestTemporaryAssetResellerUnknownStatusDoesNotExposeDiagnostic -count=1` exited 1 with `expected: "ACTIVE"`, `actual: "PRIVATE ACCOUNT DIAGNOSTIC"`.

### GREEN evidence / final validation

```text
$ go test ./controller ./service ./relay/channel/task/bytedanceseedance -run 'TemporaryAssetReseller' -count=1
ok  github.com/QuantumNous/new-api/controller  3.021s
ok  github.com/QuantumNous/new-api/service  1.964s
ok  github.com/QuantumNous/new-api/relay/channel/task/bytedanceseedance  1.173s

$ go test ./controller ./service -run 'Asset|COSUpload' -count=1
ok  github.com/QuantumNous/new-api/controller  2.275s
ok  github.com/QuantumNous/new-api/service  1.182s

$ go test ./relay/channel/task/bytedanceseedance -count=1
ok  github.com/QuantumNous/new-api/relay/channel/task/bytedanceseedance  0.864s

$ go test ./controller ./service -count=1
ok  github.com/QuantumNous/new-api/controller  33.718s
ok  github.com/QuantumNous/new-api/service  3.001s

$ go vet ./controller ./service ./relay/channel/task/bytedanceseedance
(exit 0; no output)

$ git diff --check
(exit 0; no output)
```

No known outstanding Task 4 security-review findings remain in the assigned scope.

## Round 2 direct StarAI compatibility fix

The Round 1 expiry hardening was too broad. It now applies only when `channel_type` is ByteDance Seedance. Legacy bindings with no channel type and explicit direct StarAI bindings retain their former behavior: save resets the configured local expiry, lookup/update do not reject based on the JSON expiry field, create does not require an immediate expiry refresh, and refresh/generation ignore upstream `expires_at`. Reseller ownership, diagnostic, expiry, and 168-hour cap controls remain intact.

### RED evidence

`go test ./controller ./service -run 'TemporaryAssetDirect' -count=1` exited 1 with:

```text
--- FAIL: TestTemporaryAssetDirectCreateDoesNotRequireExpiryRefresh
    Should be true
--- FAIL: TestTemporaryAssetDirectRefreshIgnoresUpstreamExpiresAt
    Received unexpected error: temporary asset has expired upstream
--- FAIL: TestTemporaryAssetDirectSaveLookupAndUpdateKeepLegacyExpiryBehavior
    --- FAIL: .../channel-type-0
        Received unexpected error: temporary asset has expired upstream
    --- FAIL: .../channel-type-61
        Received unexpected error: temporary asset has expired upstream
--- FAIL: TestTemporaryAssetDirectGenerationIgnoresUpstreamExpiresAt
    --- FAIL: .../channel-type-0
        Received unexpected error: temporary asset has expired upstream
    --- FAIL: .../channel-type-61
        Received unexpected error: temporary asset has expired upstream
```

### GREEN evidence / final validation

```text
$ go test ./controller ./service -run 'TemporaryAssetDirect|TemporaryAssetReseller' -count=1
ok  github.com/QuantumNous/new-api/controller  1.394s
ok  github.com/QuantumNous/new-api/service  2.179s

$ go test ./controller ./service -run 'Asset|COSUpload' -count=1
ok  github.com/QuantumNous/new-api/controller  1.415s
ok  github.com/QuantumNous/new-api/service  2.141s

$ go test ./relay/channel/task/bytedanceseedance -count=1
ok  github.com/QuantumNous/new-api/relay/channel/task/bytedanceseedance  0.839s

$ go test ./controller ./service -count=1
ok  github.com/QuantumNous/new-api/controller  29.679s
ok  github.com/QuantumNous/new-api/service  2.993s

$ go vet ./controller ./service ./relay/channel/task/bytedanceseedance
(exit 0; no output)

$ git diff --check
(exit 0; no output)
```
