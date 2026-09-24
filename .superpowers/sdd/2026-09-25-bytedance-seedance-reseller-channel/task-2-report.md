# Task 2 — native ByteDance Seedance task adaptor

## Implementation

- Added native channel-64 task adaptor using `seedanceprotocol.Payload`, `ResponseEnvelope`, supported models and validation. Submit and poll call only the configured Molii public `/v1/video/generations` API. Automatic submit retry is disabled.
- Submit accepts the Molii public task ID from the response; stored task data and client response use only the reseller public ID. Polling implements `service.PrivateTaskPollingAdaptor` and persists an allowlisted status/progress/usage/duration/resolution projection. Raw nested diagnostics, `upstream_id`, and raw failure strings are not copied into public task facts. The OpenAI-video converter reads usage from the safe projection and never emits the Molii task ID or provider diagnostics.
- Registered native task routing, four models, unified Seedance 2.x route selection, and OpenAI-video endpoint type. Existing direct StarAI storage, billing, timing, and diagnostic predicates were not changed.
- The endpoint type owner is `common/endpoint_type.go`; `common/channel.go` does not exist, so this is the documented file substitution.

## Files changed

- `relay/channel/task/bytedanceseedance/adaptor.go`
- `relay/channel/task/bytedanceseedance/constants.go`
- `relay/channel/task/bytedanceseedance/adaptor_test.go`
- `relay/channel/task/bytedanceseedance/registration_test.go`
- `relay/relay_adaptor.go`
- `controller/model.go`
- `service/channel_select.go`
- `common/endpoint_type.go`
- `.superpowers/sdd/2026-09-25-bytedance-seedance-reseller-channel/task-2-report.md`

## TDD evidence

- RED: `go test ./relay/channel/task/bytedanceseedance ./relay` failed with `undefined: TaskAdaptor` in five test locations before the implementation.
- GREEN: the same command passed after implementing the adaptor and native registration.
- RED: `TestOpenAIVideoPollPreservesUsageWithoutPrivateFields` failed because the converted response omitted `total_tokens:150`; GREEN after reading usage from sanitized polling data.
- RED: `TestPollRejectsResponseWithoutStatus` failed because an incomplete response was accepted; GREEN after requiring status.
- RED: `TestPollRejectsPrivateResolutionDiagnostic` failed because `cgt-private` appeared in safe polling data; GREEN after restricting stored resolution to recognized values.

## Validation

- PASS: `go test ./relay/channel/task/bytedanceseedance ./relay ./service ./controller -run 'ByteDanceSeedance|UnifiedNativeTask|ChannelSelect' -count=1` (service/controller/relay had no matching tests; adaptor registration test ran).
- PASS: `go test ./relay/channel/task/bytedanceseedance ./relay ./controller ./common -count=1`.
- PASS: `git diff --check`.
- FAIL: `go test ./service -run 'TestPinnedTaskPluginChannelTypes' -count=1`: existing `TestPinnedTaskPluginChannelTypesIncludesStarAIForUnifiedSeedance2` expects `[54,45,61]` but routing correctly produces `[54,45,61,64]`. `service/channel_select_test.go` is outside this task's permitted file ownership; parent task must update the assertion/test name.
- FAIL: `go test ./...` for the same stale service assertion and root package setup error `main.go:43:12: pattern web/dist: no matching files found`. All other packages reported as passing in that run.

## Self-review and concerns

- Reviewed the diff for accidental edits to StarAI-only predicates; none. The native adaptor stores the Molii public ID only in the task's upstream ID field, and all ordinary persisted polling data is allowlisted.
- `result_url` is accepted only from the public envelope's explicit `result_url` fields, not nested provider content. Molii controls that value; further origin-policy validation would require a separate media-delivery contract.
- The required routing test command's regex misses the existing service test named `TestPinnedTaskPluginChannelTypesIncludesStarAIForUnifiedSeedance2`; the full service run exposed it. This is an expected assertion update, not an implementation regression.
- The plan's `ParseResponse` pseudocode used direct `RelayInfo` fields; this repository nests `PublicTaskID` in `TaskRelayInfo`, so tests and implementation use the project-native structure.
