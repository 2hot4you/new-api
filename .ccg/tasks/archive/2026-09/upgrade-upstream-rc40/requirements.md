# Upgrade New API from rc.36 to rc.40

## Goal

Integrate official `v1.0.0-rc.37`, `v1.0.0-rc.38`, `v1.0.0-rc.39`, and `v1.0.0-rc.40` sequentially into the Molii fork while preserving all Molii behavior and persisted data contracts.

## Mandatory compatibility constraints

- Start from current `origin/develop`, whose latest upstream release ancestor is `v1.0.0-rc.36`.
- Preserve persisted channel IDs: StarAI `61`, Molii Grok `62`, Task Plugin `63`, ByteDance Seedance `64`.
- Assign vLLM and SGLang unused IDs; never reinterpret existing channel rows.
- Preserve Molii StarAI, Grok, ByteDance Seedance, task billing/outbox, COS asset persistence, marketplace, runtime branding, multi-site deployment, and documentation behavior.
- Keep PostgreSQL, MySQL, and SQLite compatibility for upstream database code; keep the configured PostgreSQL deployment path working.
- Preserve prior pre-consume behavior unless an explicit setting opts into rc.39's wallet bypass; default `quota_setting.trust_quota_usd` to `0` for the fork.
- Integrate release tags in order and create a verifiable checkpoint after each version.
- Add behavior tests before compatibility code and verify each new test fails for the intended reason.
- Do not deploy or push shared branches as part of implementation without separate authorization.

## Acceptance criteria

- All four official release deltas are present, with Molii custom behavior retained.
- Existing persisted channel types route to the same adaptors before and after the upgrade.
- Effective prices and pre-consume/settlement semantics remain stable for Molii GPT Image 2, Grok, StarAI, and Seedance paths unless a release fix is explicitly adopted.
- Database migrations are repeatable and tested on the supported dialects available in CI.
- Root and relaykit Go builds/tests, frontend typecheck/build/tests, deployment contract tests, and targeted billing/routing tests pass.
