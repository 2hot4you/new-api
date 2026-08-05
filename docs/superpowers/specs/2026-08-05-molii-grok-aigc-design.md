# Molii Grok Imagine API Channel Design

## Goal

Add a server-side-only `Molii Grok Imagine API` channel that supports two synchronous image models and one asynchronous video model while keeping the existing Molii AIGC and official xAI implementations unchanged.

## Chosen approach

Use independent image and video adaptors and add narrowly scoped optional capabilities to the existing relay and polling pipelines. The core pipelines remain provider-neutral; only adaptors that implement the new capabilities receive special error sanitization, polling status handling, and safe task-data persistence.

Rejected alternatives:

1. Channel-specific branches throughout controllers and services would duplicate provider behavior and make upstream synchronization harder.
2. A separate router and task subsystem would duplicate authentication, billing, task persistence, SSRF protection, and OpenAI-compatible responses.

## Registration and routing

- Preserve `ChannelTypeStarAI = 61`.
- Add `ChannelTypeMoliiGrokAIGC = 62` and move only the placeholder `ChannelTypeDummy` to 63.
- Add a separate `APITypeMoliiGrokAIGC`.
- Register a synchronous image adaptor and an asynchronous task adaptor.
- Resolve image endpoints only for the two image models and video endpoints only for `grok-imagine-video-1.5`.
- Register all three models in the global model catalog without adding built-in prices.

## Image flow

1. `/v1/images/generations` parses the OpenAI request through the existing image relay.
2. The new adaptor reads a dedicated request DTO from the stored raw request body so an explicit `n: 0` remains distinguishable from an omitted `n`.
3. The adaptor trims and validates the prompt, model, count, resolution, and aspect ratio, then sends only the five supported fields upstream.
4. `/v1/images/edits` is rejected before any upstream request.
5. The adaptor decodes the upstream success response and emits only an OpenAI-compatible `created/data` response. Upstream usage and wholesale cost are discarded.
6. The initial fixed-price charge uses validated `n`; the successful response updates the existing image ratio with the validated actual result count. Safe quota conversion remains in the existing billing pipeline.
7. A provider-neutral optional image error sanitizer prevents non-2xx upstream bodies, request IDs, domains, or keys from reaching users.

## Video flow

1. The existing `/v1/videos` and legacy `/v1/video/generations` routes parse `duration`, `aspect_ratio`, and `resolution` directly into `TaskSubmitReq`.
2. The independent task adaptor validates the model and prompt, applies conservative defaults (`duration: 5`, `aspect_ratio: 16:9`, `resolution: 480p`), and forwards the explicit request fields.
3. The adaptor returns only the pre-generated public `task_*` identifier; the upstream `request_id` is stored in `Task.PrivateData.UpstreamTaskID`.
4. Automatic task submit retry is always disabled because a timeout or ambiguous response may already have created a paid task.
5. Polling accepts HTTP 200 and 202. `pending` and unknown non-empty intermediate states remain in progress; `done` succeeds; `failed` and `expired` fail. Progress is clamped to 0–100.
6. A provider-neutral optional polling privacy capability stores only safe status/progress data and prevents upstream IDs, raw bodies, cost fields, and result URLs from normal logs.
7. The upstream result URL is stored only in private task data. OpenAI task responses contain the public `/v1/videos/{task_id}/content` URL.
8. VideoProxy accepts only HTTPS result URLs for this channel, runs the existing SSRF checks, forwards Range/If-Range, and forces `video/mp4` outward.

## Errors and privacy

- Both adaptors map upstream failures to stable Molii-facing error messages and sanitized codes.
- Pricing configuration errors map to `task_pricing_not_configured` and `provider_configuration_error` without an upstream Request-ID.
- 400, 401, and 403 are non-retryable. 429 and 5xx may be categorized as transient, but task submit retry remains disabled at the adaptor level.
- Transport timeouts produce an outcome-unknown error and never trigger a second automatic video submission.
- Normal logs may contain only the local request ID, public task ID, channel ID, model, HTTP status, duration/progress, and sanitized error code.

## Billing

- No upstream `cost_in_usd_ticks` conversion is attempted.
- Both image models require independent administrator-configured fixed prices and multiply the fixed price by the validated actual image count.
- The video model requires one administrator-configured fixed per-task price. Duration, resolution, and aspect ratio are stored as audit metadata but do not multiply the first-version price.
- Missing prices fail before upstream submission through the existing model-price helper.
- Video precharge, success settlement, failure refund, and terminal-state CAS protection remain in the existing task billing lifecycle.

## Admin frontend

- Add a new channel option named exactly `Molii Grok Imagine API` with a Grok/xAI-style icon.
- Hide Base URL for this type and never put the server default URL in frontend source.
- Require only Key and fill all three models.
- Channel testing performs local configuration validation only and reports that no paid request was sent.
- Keep Molii AIGC constants, labels, models, and behavior unchanged.

## Testing

Tests cover registration and numeric stability, image DTO/defaults/validation/redaction/billing count, task submit privacy and retry policy, polling status mapping including HTTP 202, public content URLs, HTTPS/SSRF/Range behavior, fixed-price failure, refund/idempotent settlement, admin form behavior, and frontend source privacy.
