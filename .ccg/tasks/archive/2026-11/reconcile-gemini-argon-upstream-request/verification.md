# Read-only upstream request lookup, 2026-11-06

User supplied upstream request ID 202611060608195705959968268d9d61Z43OTrS, Beijing completion time 14:14:50, model gemini-4-argon and input/output/image counts 54506/3196/1120. Queried only model.claudeye.com via its existing verified LA SSH host. Main/log DB is local PostgreSQL new_api; queries used default_transaction_read_only and five-second statement timeout. No credentials, prompts or unrelated user identities output.

- Exact equality against request_id and upstream_request_id in logs: zero rows.
- Same ID in app stdout logs 14:06–14:17 and persisted detail/content 14:06–14:25: zero matches.
- Other same-model completions near 14:14:50 have different upstream IDs and usage. Within inspected 14:06–14:25 records, no same-model match for input54506 or output3196. Same-model 14:06–14:17 sample had 382 consumption and 1 error row, all with a stored upstream ID. No time-based matching accepted.
- In this code, local X-Oneapi-Request-Id is preserved for the client and the matching upstream response header is recorded separately. If the upstream ID is different from the header observed, the request did not pass through this site, or no correlated terminal record was persisted, the current evidence cannot distinguish those cases. No confirmation of this request's success, timeout, token transform or charge. The prior catalog-busy request was a different ID.

No replay, billing action, DB writes, changes to production config, source changes, deployment or other host contact. No code test required; no fix claim. No new spec convention.
