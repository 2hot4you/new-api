# Task 7 report — Grok video edit billing resolution

## Delivered

- Removed provider-cost-based Grok video edit resolution inference entirely.
  `usage.cost_in_usd_ticks` remains only on the transient polling result for
  diagnostics and never participates in selecting a price tier or formula. It
  is not persisted in the public-safe billing snapshot or user usage logs.
- Extended the private polling DTO with optional `video.resolution`. Polling
  normalizes explicit 480p, 720p, and 1080p values; unknown values remain
  indeterminate. Edit completion additionally requires the explicit value to
  exist in the submission-time price snapshot and remain configured for the
  billed model.
- Added `actual_resolution` and versioned
  `resolution_source=provider_poll_v1` to the version-1 billing snapshot.
  Existing JSON without the new optional field continues to decode.
- Explicit 480p/720p edit results use their snapshotted unit price. Extreme or
  contradictory provider cost values do not change the tier or quota.
- A successful edit with missing, unknown, or unsupported resolution remains a
  successful delivered task. Its video URL is persisted, its terminal billing
  job has `target_quota=nil`, reconciliation enters `review_required`, and the
  user balance and precharge remain unchanged.
- Legacy edit snapshots without the versioned explicit-resolution source no
  longer silently settle. Legacy edit contexts without a snapshot are converted
  to an indeterminate billing snapshot and routed to terminal review without
  guessing 480p or 720p.
- Generation and image-to-video completion retain their submitted
  requested/estimated resolution contract.

## TDD evidence

RED initially failed to compile because `TaskInfo.ActualResolution`,
`GrokVideoBillingSnapshot.ResolutionSource`, and audit `ProviderCost` did not
exist. The old completion tests also demonstrated tier selection solely from
provider cost. GREEN adds the fields and exercises explicit 480p/720p,
missing/unknown/unsupported values, extreme provider costs, legacy JSON, and
terminal outbox review behavior.

## Validation

Passed:

```text
go test ./relay/channel/task/moliigrok ./service ./model ./relay/common -count=1
go test -race ./relay/channel/task/moliigrok ./service -run 'TestGrokVideoEditCompletion|TestLegacyGrokVideoEdit|TestParseVideoTaskResultNormalizesExplicitResolution|TestFinalizedGrokVideoEdit|TestFinalizeGrokVideoBilling|TestMoliiGrokFinalUsageMissingCompletionFinalization' -count=5
go vet ./relay/channel/task/moliigrok ./service ./model ./relay/common
gofmt -w <changed Go files>
git diff --check
```

No real provider request was made; polling integration tests use in-memory
adaptors and local data only.

## Concerns

None. If the provider omits `video.resolution`, as its current documented
completion example does, successful edits intentionally require manual billing
review while retaining the precharge.
