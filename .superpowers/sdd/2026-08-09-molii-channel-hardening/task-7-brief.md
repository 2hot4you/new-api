# Task 7 — Grok video edit billing resolution

## Goal

Remove all provider-cost-based inference of Grok video edit resolution. Final
billing may use only an explicit, validated resolution returned by polling.

## Contract

- Extend the polling DTO with optional `video.resolution` and normalize it.
- Accepted explicit final resolutions are the configured Grok video tiers
  supported by the applicable model/operation. Unknown values are not guessed.
- Preserve provider-reported cost only as non-authoritative audit data; it must
  never select a price tier.
- Record `actual_resolution` and a versioned `resolution_source` in the billing
  snapshot when the result is explicit.
- When a successful video-edit result lacks a trustworthy resolution, deliver
  the successful video but leave terminal billing target indeterminate. The
  billing outbox must enter `review_required` and retain the precharge.
- Generation/image-to-video billing continues to use its requested/estimated
  resolution contract.
- Preserve legacy snapshot decoding; do not invent a final tier for old edits.

## Evidence

The current xAI REST completion example returns `video.url` and
`video.duration`, but does not document `video.resolution`. The same official
documentation says editing retains input resolution capped at 720p. Because the
gateway does not inspect the input video's dimensions, neither the provider
cost nor the cap can identify a 480p versus 720p final tier.

## Tests

- RED then GREEN for explicit 480p/720p polling values.
- Empty/unknown result resolution produces an indeterminate target and
  `review_required`, with balances/precharge unchanged.
- Changing provider cost to extreme values never changes the selected tier.
- Legacy snapshots remain readable and do not silently settle an uncertain
  edit.
- Focused/full affected Go tests, race tests where relevant, vet, gofmt, and
  `git diff --check`.
