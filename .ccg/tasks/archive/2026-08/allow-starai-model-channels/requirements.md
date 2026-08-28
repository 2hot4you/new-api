# Requirements

- Permit multiple enabled Molii Volcengine Imagine API (StarAI) channels.
- Configure one upstream key and one supported model per channel.
- Select the submission channel through the normal group/model channel routing.
- Poll asynchronous tasks through the original task `channel_id` and that channel's key.
- Remove temporary-asset behavior that assumes a unique enabled StarAI channel.
- Do not assume asset IDs created with one upstream key are accessible from another key.
- Preserve existing behavior for other channel types and avoid exposing upstream pricing discounts or credentials.
- Add regression tests before implementation.
