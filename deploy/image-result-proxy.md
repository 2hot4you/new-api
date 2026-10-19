# Image result proxy activation

This feature must be deployed and explicitly enabled before publishing `docs/image-api.md` as the live interface contract. Local tests are not evidence of production activation.

## Runtime configuration

Add these entries to the application's environment, using its runtime env file or the Compose service `environment` block:

```yaml
environment:
  IMAGE_RESULT_PROXY_ENABLED: "true"
  IMAGE_RESULT_PROXY_BASE_URL: "https://YOUR_PUBLIC_API_HOST"
```

Use the site's actual public HTTPS origin. If `IMAGE_RESULT_PROXY_BASE_URL` is omitted, the configured server address (`ServerAddress`, database setting) is used. Do not derive this value from request headers. Existing `CRYPTO_SECRET`, or the fallback `SESSION_SECRET`, must be persistent and identical across all replicas; preserve the existing secret rather than rotating it for this feature. Secret changes invalidate previously issued links.

The feature is opt-in and applies to `gpt-image-2` / `gpt-image-2.5` and their hyphen-suffixed variants, matching either the original or mapped upstream model. Other models and Base64 results remain unchanged. It covers synchronous image generation/editing and the shared image SSE response handler.

## Routing and behavior

Allow `GET /v1/images/proxy/*` through the public reverse proxy to this application. Do not redirect it to an upstream URL or static frontend. Links contain encrypted, authenticated capabilities valid for one hour and require no API key. Application access logs redact the token; apply equivalent redaction or disable access logging for this location in an external reverse proxy. Downloads use the existing download rate limit and a process-wide limit of two concurrent downloads, held until the response write completes. Image response writes have a fixed 60-second deadline so slow readers cannot occupy slots indefinitely. Excess concurrent downloads return generic HTTP 503 with `Retry-After: 3`.

The server fetches HTTPS images directly with mandatory private-address rejection and DNS pinning. It validates redirects and serves only sniffed PNG, JPEG, WebP or GIF data, capped at 30 MiB with a 60-second fetch deadline. Bytes are buffered transiently for validation; there is no object storage or database migration. Upstream failure bodies and headers are not relayed to clients.

The existing **Worker Proxy** administration setting remains unchanged. It is a server-side download transport, not an output-URL rewriting feature. This public image proxy uses its own strict direct fetcher so that relaxed global fetch settings or a remote Worker's DNS behavior cannot bypass its security boundary.

## Generation gateway timeout

Install `deploy/image-generation-timeout.conf` inside the API site's Nginx/OpenResty server block, adjusting the local upstream address if needed. It sets a 600-second upstream read inactivity timeout for `/v1/images/`; the longer `/v1/images/proxy/` location retains its independent download timeout and logging rules. Preserve any site-specific proxy headers or authentication directives when adapting this template. Validate with `nginx -t` before a graceful reload. This setting does not extend a CDN or client's timeout and does not retry generation. To roll back, remove this included file, validate, and reload.

## Verify after rollout

1. Confirm the deployed version contains this feature and both environment entries are available to the running application.
2. Make one real authenticated generation request for an enabled model with `response_format: url`.
3. Confirm every HTTP image URL in `data` uses the configured public origin and `/v1/images/proxy/` path; other response fields and usage must remain intact.
4. Open a returned link without Authorization or cookies. Expect `200`, a valid raster image, `Cache-Control: no-store`, and no upstream `Location`, `Set-Cookie` or diagnostic body.
5. Alter one character in the token. Expect a generic `404`; an authentic expired token returns `410`.
6. Check a Base64 response and an unrelated model to confirm they retain their existing behavior. Verify streaming if offered to customers.

Do not retry a generation request automatically after a timeout or server error. Check whether generation already completed first.

## Rollback

Set `IMAGE_RESULT_PROXY_ENABLED=false` and restart the application, or restore the previous deployed image. Disabled mode restores the previous image response behavior and makes issued proxy links unavailable; the previous behavior may expose provider URLs, so communicate the change before customer use.
