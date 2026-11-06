# Read-only log interpretation

Reviewed the earlier verified production catalog-busy diagnosis and the platform relay/usage logging source. The known gemini-4-argon failure was rejected before upstream forwarding and before precharge; it can appear in application/request logs with no upstream record. No evidence that it is the user's newly observed discrepancy.

controller/relay.go has a bounded channel-attempt loop; one inbound request can yield more than one upstream attempt depending on retry policy. model/log.go records platform consumption data and platform/upstream request IDs; service/text_quota.go persists settlement summaries rather than verbatim upstream usage. relay/channel/openai/relay-openai.go estimates usage in applicable paths when stream usage is missing. These are possible accounting differences, not established causes for an unmatched pair. Specific comparison needs category, endpoint, timestamp and matching upstream request ID when present.

No production access or mutations in this follow-up, no new code, tests or deployment. No new spec convention.
