# Requirements

## Goal

Extend the existing safe Grok image persistence diagnostic with the integer HTTP
status returned by the remote image GET when that response is non-2xx.

## Required behavior

- Add `remote_status=<integer>` to `grok_image_result_persistence_failed` logs.
- Preserve `GrokImagePersistenceErrorDetails` compatibility; expose status with
  a separate extractor.
- Populate the status only for a received non-2xx remote image response.
- Report `remote_status=0` for every other failure.
- Keep the client-facing generic 502 unchanged.

## Security and scope

- Never log URL path/query, response body, authorization, API/channel keys,
  COS/Redis credentials, API tokens, or environment secrets.
- Do not add headers, retries, authentication, or alter MIME, SSRF, COS, Redis,
  billing, response, or video behavior.
- Do not issue a real Grok request during this task.
- Modify only the existing Grok image persistence diagnostic and its tests.

## Verification

- Cover 401, 403, 404, 410, 429, and 502.
- Verify status, stage/category, hostname-only source, safe `Error()` and logs.
- Verify ordinary errors report status zero.
- Run service, adaptor, controller, and full Go tests plus vet/format/diff checks.
- Complete dual-model analysis and review under CCG policy.
