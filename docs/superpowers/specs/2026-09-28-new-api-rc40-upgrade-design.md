# Molii New API rc.40 Upgrade Design

**Date:** 2026-09-28  
**Status:** Draft for implementation approval  
**Scope:** Upgrade the Molii fork from its latest upstream ancestor, `v1.0.0-rc.36`, through `v1.0.0-rc.37`, `rc.38`, `rc.39`, and `rc.40`.

## 1. Context

The deployed Molii branches contain substantial backend, frontend, billing, task, storage, branding, and deployment customizations on top of New API. Their latest official release ancestor is `v1.0.0-rc.36`; upgrading by changing an image tag or merging `rc.40` directly would bypass four release boundaries and produce a large, hard-to-audit conflict set.

The integration must preserve persisted data semantics, especially channel type integers. In the Molii fork these values already mean:

| Persisted value | Molii meaning |
| --- | --- |
| `61` | StarAI |
| `62` | Molii Grok AIGC |
| `63` | Task Plugin |
| `64` | ByteDance Seedance |

Upstream rc.38 uses `61`, `62`, and `63` for Task Plugin, vLLM, and SGLang. Reusing those upstream numbers would silently reinterpret existing database rows, so the fork's persisted meanings are authoritative.

## 2. Selected approach

Merge official release tags one at a time in this order:

1. `v1.0.0-rc.37`
2. `v1.0.0-rc.38`
3. `v1.0.0-rc.39`
4. `v1.0.0-rc.40`

Each release is a separate, tested checkpoint. Conflicts are resolved by behavior, not by accepting an entire side. Compatibility tests are written before the relevant conflict resolution so they fail against the unsafe upstream interpretation and pass only after the Molii contract is restored.

### Alternatives considered

**Direct merge to rc.40.** This has the shortest Git history but combines all conflict causes into one step. It makes it difficult to identify which release changed billing, migrations, routing, or UI behavior and is rejected.

**Cherry-pick only selected upstream fixes.** This reduces immediate conflicts but does not produce an rc.40-compatible fork and leaves future upgrades harder. It is rejected.

**Rebase Molii customizations onto rc.40.** This could yield a cleaner history, but the fork has many interdependent custom commits and persisted contracts. The risk of silently losing behavior is higher than a staged merge and is rejected.

## 3. Compatibility architecture

### 3.1 Channel type registry

Molii's values `61`–`64` remain unchanged. New upstream vLLM and SGLang channel types receive the next unused values after auditing the complete registry. The expected allocation is `65` and `66`; any existing fork-only type occupying those values must be moved only if it has never been persisted, otherwise the next free values are used.

All channel-type consumers must share the same registry: backend constants and base URLs, adaptor dispatch, validation, model lists, frontend options and icons, upstream model refresh, and round-trip create/edit behavior.

The upgrade adds contract tests that load literal persisted values `61`–`64` rather than only referring to constants. This ensures a future constant edit cannot make the test pass accidentally.

### 3.2 Database migrations

rc.37 adds Passkey relying-party identity and repairs the `options.key` uniqueness constraint. The implementation keeps upstream dialect support while adding repeatability tests. The `options` repair is treated as an operationally significant migration because it can create an `options_legacy_*` table and upstream logs rather than aborting on failure.

rc.40 introduces a dialect-aware long-text type for Task Plugin source and icon fields. PostgreSQL must continue to use `text`; MySQL may widen to `longtext`; SQLite remains compatible. Migration tests verify a second startup is a no-op and that existing rows survive.

No automatic migration may reinterpret channel type integers or bind existing Molii channels to new Task Plugins.

### 3.3 Billing and settlement

rc.37 pricing-expression changes are adopted, including upstream image quantity fixes, but current administrator overrides and Molii effective prices remain authoritative. Regression fixtures cover GPT Image 2, Grok, StarAI, Seedance, and Task Plugin prices.

rc.39's configurable trust threshold is adopted with a fork default of `quota_setting.trust_quota_usd = 0`. That preserves the current always-pre-consume behavior unless an administrator explicitly opts into a positive bypass threshold. The input pre-consume multiplier remains configurable with the upstream default of `1`.

The trust threshold is separate from rc.39's reservation formula change: generic legacy text reservations no longer apply the old completion-token estimate in the same way. Molii GPT Image 2, Grok, StarAI, Seedance, mapped aliases, fixed-request expressions, subscriptions, and forced pre-consume retain their current reservation/final-price contracts. Generic text adopts the upstream formula only after focused tests document the before/after amount; the multiplier affects reservation, never final settlement or fixed request charges.

Molii asynchronous task settlement keeps its transaction compare-and-swap plus outbox flow. Terminal performance sampling must have exactly one owner: the current fork already records it after the reconciliation transaction commits, while rc.39 also records it after polling wins its CAS. The integration must select one post-commit sampling point and remove the duplicate. Upstream's direct terminal settlement must not bypass the outbox or create double charges, refunds, or performance samples.

### 3.4 Task Plugin and media routing

Molii StarAI, Grok, and ByteDance Seedance adaptors remain first-class and retain their existing model mapping, asset persistence, proxying, retry, billing, and finalization behavior.

Upstream Task Plugin improvements are adopted without implicitly claiming Molii model names. rc.39's Alibaba plugin serves the OpenAI Images protocol. The Doubao plugin adds native Seedream routes and Responses support, but it must not be treated as an automatic owner of existing `/v1/images` traffic. Molii's native Seedance bridge remains limited to the explicitly supported Seedance 2.x video models; it must not claim Seedream, Seedance 1.x, native image, or Responses traffic. New plugin bindings are explicit administrator actions.

rc.40's acceptance of any successful 2xx task submission is adopted. Media conversion fixes for Claude and Responses are adopted and verified independently from Molii's COS and temporary-asset persistence.

rc.39 omits the task `data` field from list queries, while Molii currently derives displayed Seedance facts from that field. The integration must first persist the required display projection in private data, or selectively retain data for the affected Molii platforms. Task history and video-workbench pagination must not lose actual resolution, size, duration, or related generation facts.

### 3.5 Frontend and branding

Upstream channel controls, request policies, Responses WebSocket settings, pricing UI, and Task Plugin improvements are adopted. Molii branding, navigation, marketplace, custom provider forms, API key security UX, and deployment-specific links remain intact.

Molii deliberately locks the base theme to `system` and locks theme customization to the brand default. rc.40's storage isolation helpers may be adopted, but providers must not start reading or applying user customization. Fork-specific tests document that the setters remain no-ops and stale stored preferences cannot override brand defaults.

Frontend channel options use the backend-compatible type allocation. Create/edit round trips must not change a stored channel type or discard Molii-specific settings.

## 4. Release checkpoints

### rc.37 checkpoint

- Integrate pricing-expression conversion, Task Plugin pricing/streaming, unified channel editing, image quantity billing, and Passkey RP ID support.
- Verify the `options` primary-key/unique-key repair and Passkey migrations.
- Snapshot and compare effective pricing.

### rc.38 checkpoint

- Integrate request policies, Responses WebSocket, per-route pass-through, health/rate-limit outcomes, and vLLM/SGLang.
- Apply the non-colliding channel type allocation.
- Verify HTTP and WebSocket settlement, interruption, rejection, cancellation, and proxy behavior.

### rc.39 checkpoint

- Integrate multi-plugin channel bindings, OpenAI Images through Task Plugins, expanded Seedance/Seedream metadata, retention semantics, and performance metrics.
- Preserve the zero trust-threshold default and Molii outbox settlement.
- Verify that plugin activation and routing are explicit and do not take over Molii models.

### rc.40 checkpoint

- Integrate 8 MiB Task Plugin sources, dialect-aware long text, all-successful-2xx submission handling, pricing enum resilience, storage isolation helpers, and media conversion fixes while retaining Molii's locked theme behavior.
- Run the complete regression matrix and create a release-candidate build.

## 5. Validation strategy

The implementation follows red-green-refactor for compatibility behavior. Targeted tests run after each change; the full suite runs at every release checkpoint.

Required coverage:

- Literal persisted channel IDs and adaptor selection.
- Channel create/edit/model-refresh round trips for all fork and new upstream types.
- PostgreSQL-oriented migration behavior plus available SQLite/MySQL migration tests.
- Options uniqueness repair, Passkey RP ID, Task Plugin long text, and restart idempotency.
- Effective pricing and pre-consume/final settlement/refund behavior.
- Async task terminal races, outbox reconciliation, and performance metrics.
- Responses HTTP/WebSocket success, rejection, disconnect, cancellation, and media conversion.
- StarAI, Grok, ByteDance Seedance, Task Plugin, COS asset, marketplace, branding, and deployment contracts.
- Frontend typecheck, Vitest suite, production build, lint, i18n, and protected-header checks.

Baseline before the merge:

- Relaykit tests pass.
- Frontend typecheck and production build pass.
- The correct frontend test command is `bun run test` (`vitest run`); direct `bun test` uses Bun's incompatible Jest/Vitest shim and is not a valid project baseline.
- Root Go tests require `web/dist` to exist because it is embedded by `main.go`; build the frontend before the root suite.

## 6. Rollout and rollback

Before staging, export effective pricing and back up `options`, `channels`, `task_plugins`, Passkey tables, representative tasks/logs, and the full PostgreSQL database. Keep any generated `options_legacy_*` table until acceptance is complete.

Deploy the rc.40-derived Molii image first to development against a production database clone. Verify startup migrations twice, `/api/status`, channel identity, pricing, task submission/finalization, WebSocket proxying, asset access, and administrative UI.

Production rollout uses an immutable image and a database snapshot. Rollback restores the previous image and database snapshot together if a non-backward-compatible migration or billing/routing regression is detected. MySQL rollback is rehearsed explicitly because an old binary's AutoMigrate may try to narrow rc.40 `longtext` fields back to `text`; image-only rollback is not considered safe. Newly created billing records make database restoration a coordinated operational decision, not an automatic lossless action. No production deployment or shared-branch push is included in this implementation without separate authorization.

## 7. Review constraint

The repository's configured external `antigravity` and `Claude` wrapper is absent on this host. Two independent internal research agents are being used to cross-check release groups, but they are not represented as the required external-model review. This limitation remains explicit in the task record and final handoff.
