# Task 6 report — Imagine model mapping family hardening

## Delivered

- Added a centralized Imagine capability/price family matrix:
  - Seedance standard and fast remain distinct.
  - Grok image standard and quality remain distinct.
  - Grok image, legacy video, and Video 1.5 remain distinct.
  - The retired Grok Video 1.5 preview ID is not in any family; mappings
    between it and Grok Video 1.5 are rejected in both directions.
- Enforced the matrix after a mapped upstream model is finalized, before task billing.
- Validated channel and tag model_mapping JSON before persistence.
- Kept user-facing requested/origin model values while using the billed/upstream
  model for StarAI and Molii Grok price lookup.
- Added requested and billed model values to Grok video billing snapshots,
  preserving the legacy model field as the requested model for logs.
- Added requested and billed model values to Grok image billing snapshots.
  Provider pricing uses the final upstream model, while logs retain the
  requested model and legacy snapshots remain readable.
- Moved synchronous image preparation into the relay attempt loop, after
  `getChannel` has finalized that attempt's channel and before estimation,
  model pricing, pre-consumption, or the upstream request. The first attempt
  preserves the middleware-selected channel (including specific-channel and
  affinity routing). For ordinary channels, every retry resets to the requested
  model, captures the newly selected channel metadata, remaps and validates
  once, re-estimates, prices the final billed model, and reserves any higher
  quota before sending.
- Removed the image-handler mapping marker. The handler now uses exactly the
  upstream model prepared by the controller, preventing both stale-channel
  metadata and double mapping. Credentials, base URL, logging context, and
  channel-error ownership remain aligned with the selected attempt channel.
- Removed Grok Video 1.5 preview from all backend Go runtime paths. The preview
  string remains only in tests that assert it is retired and unsupported.
- Disabled all automatic create-request replay and channel switching for Molii
  Volcengine Imagine (type 61) and Molii Grok Imagine (type 62). This applies to
  synchronous relay failures and async task/video submission failures,
  including 429 and 5xx responses. Ordinary OpenAI/image/task channels retain
  their existing retry and channel-switch behavior. The existing exact-one
  management constraint remains specific to Seedance; no Grok exact-one
  restriction was added.
- Added a close-to-end-to-end type-62 regression with two eligible channels and
  local `httptest` upstream transport. A failed image creation calls the
  middleware-selected upstream exactly once; the eligible fallback channel's
  credential is never used and `use_channel` contains only the selected ID.
  Type 61 remains protected both by its exact-one configuration invariant and
  its explicit no-submit-retry policy.
- Fixed the full controller race caused by the StarAI database test fixture
  writing the unrelated global `common.RedisEnabled` flag while an earlier
  Relay failure metric was still being recorded asynchronously. The fixture
  never needs Redis and now leaves that process-wide runtime setting untouched;
  full controller race passes five consecutive runs without sleeps or race
  suppression.

## Validation

Passed:

```text
go test ./constant ./relay/helper ./controller -run 'ImagineModel|ModelMapping' -count=1
go test ./constant ./relay/helper ./controller ./relay ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./relay/channel/task/starai ./service ./setting/ratio_setting ./model -run 'Mapping|Mapped|PrepareImage|EstimateImageBilling|BillingSnapshot|Price|Preview|LegacySnapshot' -count=1
go test -race ./controller ./relay -run 'TestMoliiImagineSyncRequestsNeverRetryOrSwitchChannel|TestOrdinarySyncChannelRetainsRetryBehavior|TestStarAITaskSubmitPolicy|TestMoliiGrokTaskSubmitPolicy|TestOrdinaryTaskSubmitPolicy|TestOrdinaryImageAttemptPreparationTracksSelectedChannelAndRetry|TestPrepareImage|TestPrepareImageModelForAttempt' -count=1
go test ./controller -run 'TestMoliiGrokFailureCallsSelectedUpstreamOnceWithEligibleFallback|TestMoliiImagineSyncRequestsNeverRetryOrSwitchChannel|TestOrdinaryImageAttemptPreparationTracksSelectedChannelAndRetry|TestOrdinarySyncChannelRetainsRetryBehavior|TestStarAITaskSubmissionRejectsNonUniqueOrMismatchedSelectedChannelBeforeBilling' -count=1
go test -race ./controller -count=5
go test ./common ./constant ./controller ./model ./relay ./relay/common ./relay/helper ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./relay/channel/task/starai ./service ./setting/ratio_setting -count=1
go vet ./common ./constant ./controller ./model ./relay ./relay/common ./relay/helper ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./relay/channel/task/starai ./service ./setting/ratio_setting
gofmt -w <changed Go files>
git diff --check
```

The same full test and vet commands also passed from a temporary checkout
created from a temporary index containing only the Task 6 patch on top of
HEAD. That staged-equivalent snapshot contained no preview references in
non-test Go files and did not depend on unrelated dirty worktree files.

## Concerns

Frontend and documentation preview references were intentionally left untouched
per scope and are being handled separately. No backend Go runtime preview
registration remains.
