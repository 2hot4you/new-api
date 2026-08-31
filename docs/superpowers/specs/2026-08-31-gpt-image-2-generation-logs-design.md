# GPT Image 2 Generation Logs Design

## Goal

Make `gpt-image-2` requests readable in common usage logs, expose them as a dedicated image-generation source, and provide Molii-owned previews for Base64 results for exactly 24 hours.

## Confirmed behavior

- The OpenAI-compatible API response remains unchanged. A request that receives `b64_json` continues to receive `b64_json`.
- Molii best-effort persists a decoded copy of completed `gpt-image-2` outputs to the existing COS configuration.
- Preview persistence failure must not turn an already successful, billable upstream response into an API failure.
- Stored result objects and preview metadata expire 24 hours after the request start time.
- Logs never store prompts, input images, masks, Base64 payloads, COS credentials, or permanent public URLs.

## Structured log contract

For the exact public model ID `gpt-image-2`, the consume log `other` payload gains a versioned `gpt_image_2` object:

```json
{
  "version": 1,
  "model": "gpt-image-2",
  "operation": "generation",
  "quality": "auto",
  "background": "auto",
  "output_format": "png",
  "moderation": "auto",
  "size": "auto",
  "user": "optional-client-identifier",
  "requested_output_count": 1,
  "output_count": 1
}
```

`operation` is `generation` or `edit`. Missing request values are normalized to the documented GPT Image 2 defaults. `user` is omitted when absent. Actual output count is taken from the successful response. The existing log `use_time` remains the single source of truth for total duration.

The ordinary `content` string becomes a concise fallback summary, while the frontend renders the versioned object as a bordered metric layout.

## Preview storage

### Object persistence

- Decode Base64 through a bounded streaming reader; do not create a second unbounded decoded byte slice.
- Detect and validate the actual image MIME type. Supported types are PNG, JPEG, and WebP.
- Store under a deterministic, user-owned `gpt-image-2-results/<user>/<date>/...` key.
- Write the immutable expiry timestamp into COS object metadata.
- Register the key in the existing Redis-backed COS cleanup index before upload, so crashes cannot leave untracked objects.
- The existing master-node cleanup worker deletes expired objects and then removes their cleanup entries.

### Preview lookup

- Redis stores only user-owned object keys and immutable expiry timestamps, keyed by an HMAC of user ID and request ID.
- The preview endpoint authorizes the owner or an administrator, validates object ownership, and creates fresh signed URLs whose lifetime never exceeds the remaining retention window.
- Missing, expired, malformed, unauthorized, Redis-unavailable, and signing-failed previews all return 404 without leaking storage details.
- The log contains only `gpt_image_2_preview_available: true`, never an object key or signed URL.

## Response paths

- Non-stream JSON responses persist each final `data[].b64_json` result before the handler returns, then relay the original response bytes unchanged.
- JSON responses converted to SSE and native image SSE collect only completed final images; partial images are not persisted.
- Preview work is best effort. Errors are sanitized in user-facing behavior and recorded only as request-correlated backend diagnostics.

## Frontend

- Add `GPT Image 2` after `Grok Image` under `/usage-logs/drawing`.
- The source sends `log_category=gpt_image_2`; backend filtering matches the exact model ID `gpt-image-2`.
- Common-log desktop and mobile summaries use the structured snapshot.
- The details dialog shows a bordered preview/parameter card with quality, background, format, moderation, dimensions, user, requested/actual output count, operation, and total duration.
- The preview card supports multiple thumbnails, full-size viewing, download, loading, unavailable, and expired states.

## Compatibility and security

- Historical `gpt-image-2` logs without the versioned snapshot still appear and fall back to existing text content.
- Other OpenAI image models are unchanged.
- Existing Grok Image behavior and routes are unchanged.
- No database migration is required because structured metadata uses the existing JSON `other` field.
- COS is private; preview access always goes through short-lived signed URLs returned by an authenticated lookup.

## Tests

- Backend category filtering includes only exact `gpt-image-2` logs.
- Snapshot normalization covers defaults, edits, optional user, and actual output count.
- Base64 persistence verifies MIME, bounds, deterministic ownership, cleanup registration, exact 24-hour expiry, and best-effort failure behavior.
- Preview lookup verifies owner/admin access, cross-user denial, expiry, invalid object ownership, and Redis/COS failure collapse.
- OpenAI handler tests prove response bytes remain unchanged.
- Frontend tests cover source routing, structured parsing, summaries, parameter layout, duration, preview states, and historical fallback.
