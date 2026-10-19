# Image result proxy implementation plan

> For agentic workers: use subagent-driven-development; each worker owns listed files only, does not spawn, and does not revert others.

Goal: Hide image provider URLs behind directly viewable site URLs without permanent image storage.
Architecture: Rewrite HTTP image URLs into expiring AES-GCM capabilities, then serve bounded raster bytes via a public endpoint with strict SSRF protection. Existing Worker downloads do not rewrite API results; the new endpoint uses a strictly protected direct fetch initially, without changing existing Worker behavior.
Tech stack: existing Go, Gin, crypto standard library, protected fetch client, gjson/sjson.
Spec: requirements.md and user's approved proxy design in chat.

## Constraints
- Opt in via IMAGE_RESULT_PROXY_ENABLED=true; base URL from IMAGE_RESULT_PROXY_BASE_URL or configured ServerAddress. HTTPS absolute URL; never derive from request Host.
- Stable shared CRYPTO_SECRET or SESSION_SECRET required operationally. Purpose-separated AES-GCM encryption with random nonce; 1 hour expiry; source address never plaintext in client URL.
- Only image2/2.5 original or mapped upstream models; other models untouched.
- HTTP source URLs unsupported; accept HTTPS only, reject credentials, fragments, private targets. DNS pinning and private-IP rejection always enabled, no environment proxy. HTTPS redirects revalidated.
- Maximum image size 30 MiB, 60-second context deadline, PNG/JPEG/WebP/GIF content sniffing. Buffer within limit before response, no disk storage. Do not expose upstream bodies, headers, URLs or errors.
- JSON response fields, usage, billing remain unchanged apart from image URL; cover JSON, actual SSE and JSON-to-SSE fallback. b64_json and data URLs need no rewrite.
- Public route GET /v1/images/proxy/:token uses possession of encrypted token, no API key needed; GET route is outside token/distribute middleware. No cookies, upstream headers or redirects.

## Review focus
Tampered/expired capabilities; DNS-rebinding and private redirects; header/error leakage; huge/HTML responses; SSE or mapping paths that bypass rewriting.

## Task 1 — service (worker)
Files service/image_result_proxy.go, service/image_result_proxy_test.go only.
Interfaces: ImageResultProxyEnabledForModel(model string) bool; BuildImageResultProxyURL(sourceURL string) (string,error); ResolveImageResultProxyToken(token string) (string,error); FetchImageResultProxy(ctx context.Context, sourceURL string) ([]byte,string,error). Sentinel ErrImageResultProxyTokenInvalid and ErrImageResultProxyExpired.
- [ ] Failing tests for encrypted roundtrip, tamper/expiry/key rotation, bad target/config, image types, cap, SSRF protections. Run to capture missing behavior.
- [ ] Implement with common JSON helpers, fixed security minimum overriding permissive fetch settings. Reuse protected transport, force TLS verification (fail closed if global insecure TLS setting is active if no safe per-client override).
- [ ] Run focused go test ./service -run ImageResultProxy and report results. No commits.

## Task 2 — relay (worker)
Files relay/channel/openai/image_result_proxy.go, relay/channel/openai/image_result_proxy_test.go, relay/channel/openai/relay_image.go only.
Consume service APIs from Task 1. Select original/mapped model using RelayInfo. Rewrite data[].url and top-level SSE url; preserve all other fields. Fail closed with a generic non-retryable relay error if enabled URL rewriting fails, never original URL fallback. Tests verify correct body, usage, unchanged disabled/other-model behavior, SSE and fallback, and malformed targets. Capture red test, implement, run focused tests when Task 1 lands. No commits.

## Task 3 — HTTP route/controller (lead)
Files controller/image_result_proxy.go, controller/image_result_proxy_test.go, router/relay-router.go, router/image_result_proxy_test.go.
Consume Resolve/Fetch APIs. Generic 404 invalid, 410 expired, 502 fetch failure; 200 raster body and allowlisted headers only. Set no-store and nosniff. Test browser access without API key via actual registered route, tampered tokens and public error sanitization. Add meaningful controller success test using an injected fetch function at a private handler boundary if necessary.

## Task 4 — delivery (lead)
Create sanitized docs/image-api.md and deploy/image-result-proxy.md with activation instructions and deployment prerequisites. Verify examples and absence of provider domains/brand names from public doc (model identifiers are placeholders). Run affected packages, go test ./..., go vet affected packages, diff check. Required dual external analysis/review unavailable due missing codeagent-wrapper/antigravity; try available Claude CLI and independent Codex review, record limitations honestly. Archive task and commit scoped changes only. No production deploy until exact target and working deployment access established; do not claim live activation.

## Review adjustments
- Parent additionally owns relay/image_handler.go and relay/image_result_proxy_test.go to sanitize upstream transport and non-200 failures in enabled mode.
- Returning an error before any complete image is delivered follows the existing failed-response refund policy. Once a completed SSE image is delivered, a later proxy failure emits a generic stream error and preserves normal interrupted-stream settlement, preventing a full refund of delivered images.
- Enabled generation JSON/SSE strips provider response headers and sanitizes embedded errors. Opt-out behavior stays unchanged.
- Existing frontend build assets copied from the main checkout into the ignored web/dist directory solely to satisfy Go embed for full tests/build; no frontend source changes.
