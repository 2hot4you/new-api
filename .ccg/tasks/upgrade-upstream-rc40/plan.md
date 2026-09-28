# Implementation plan

The detailed implementation plan is maintained at:

`docs/superpowers/plans/2026-09-28-new-api-rc40-upgrade.md`

Execution order is rc.37 → rc.38 → rc.39 → rc.40, with a tested Git checkpoint after each tag. The core merges are sequential; disjoint validation and documentation work may be delegated in parallel.

Hard gates:

1. Literal persisted channel IDs `61`–`64` retain their Molii meanings.
2. vLLM/SGLang receive audited unused IDs across backend and frontend.
3. `quota_setting.trust_quota_usd` defaults to `0` for the fork.
4. Async terminal settlement retains transaction CAS + outbox semantics.
5. Migration, pricing, routing, billing, provider, branding, asset, and deployment tests pass at every relevant checkpoint.
6. No shared-branch push or deployment occurs without separate authorization.
