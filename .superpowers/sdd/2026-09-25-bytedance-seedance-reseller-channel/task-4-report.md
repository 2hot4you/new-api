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

## Integration concern

The existing direct StarAI generation adaptor calls `service.ResolveStarAIAssetURI` and now rejects a reseller binding before contacting upstream. The ByteDance Seedance generation adaptor currently does not call that verifier at all. Wiring that adaptor to call the service with `ChannelTypeByteDanceSeedance` is outside Task 4's assigned file scope and must be handled by its owner before claiming end-to-end reseller generation ownership enforcement.
