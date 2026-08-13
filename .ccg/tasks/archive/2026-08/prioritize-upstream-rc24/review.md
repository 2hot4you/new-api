# rc.24 upgrade priority

1. P0 — user/access-token and affiliate accounting concurrency hardening.
2. P0 — per-user critical rate limiting for access-token rotation and affiliate transfer.
3. P0 — HTTP/2 request replay and redirect safety.
4. P0 — redemption-code editable quota precision.
5. P1 — native Claude/Gemini channel-test requests.
6. P1 — expanded fetched-model vendor categorization.
7. P2/skip — GitCode release synchronization workflow.

Recommended implementation set: 1–6. Item 7 is unrelated to Molii runtime.
