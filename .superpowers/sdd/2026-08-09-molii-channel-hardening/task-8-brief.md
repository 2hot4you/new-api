# Task 8 — Reject unsupported Grok file_id media

## Goal

Make the public Grok image/video contract match the implemented user media
lifecycle. Until Molii has a user-owned `/v1/files` API, Grok requests must not
accept opaque upstream `file_id` values.

## Contract

- Grok image generation/editing and video generation/editing accept supported
  HTTPS/Data URL shapes only where already implemented.
- Any Grok `file_id` input is rejected before estimation, precharge, channel
  request, or network I/O.
- Return HTTP 400 with stable public code `file_id_not_supported` and a
  provider-neutral Molii message.
- Keep `file_id` fields in decode-only DTOs if needed to distinguish an explicit
  unsupported value from an unknown field, but never forward them upstream.
- Do not change generic New API media handling or other channel types.
- Do not implement `/v1/files` in this task.
- Public OpenAPI, Grok user docs, and Test Lab must expose URL inputs only.

## Tests

- Image single/multiple `file_id` inputs fail before adaptor HTTP transport and
  billing.
- Video image/video `file_id` inputs fail before async precharge/submit.
- Supported URL string/object forms still pass.
- The stable status/code/message are covered.
- OpenAPI/Demo contract tests contain no Grok `file_id` examples or fields.
- Focused/full affected tests, vet, formatting, and `git diff --check` pass.
