# rc.47 compatibility and environment isolation design

Authority: requirements.md captures the user's explicit plan-and-implement request. We keep the existing Go/React/Bun architecture. The user has delegated routine implementation choices and explicitly forbidden main/production actions. No extra approval menu is needed to repeat this scope.

## Approach

Integrate the pinned official rc.47 tag into latest develop in the reused isolated upgrade/rc47 branch, preserving later Molii fixes. This retains upstream ancestry and dependencies. Direct official-image replacement would reinterpret custom persisted channel types and lose custom flows; cherry-picking selected fixes would omit interacting routing/billing/stream changes. Neither is appropriate. Reconcile actual conflicts and semantic auto-merges with behavioral tests, then push a verified commit to actual remote develop only. No main push, merge, production dispatch, or iXiaozu operation.

## Compatibility boundaries

1. Cache: carry12h write facts through independent DTOs, clone/merge, AST normalization, immutable money snapshots, preconsume/settle/log/UI. Preserve explicit0, unsplit Claude creation, image-cache intersections and existing administrator expressions; no live quote rewrites.
2. Routing: ordinary and task-plugin candidates compete by existing priority/weight/group/pin rules, with selected identities/prices kept separate. Persisted Molii type61/62/63/64+ meanings are never replaced by upstream enum numbers. Native Seedance/Grok routes and compiled-source/generation ownership remain. Reject incompatible ordinary/task expressions before reservation and sending, using captured selection rather than live unrelated providers.
3. Tasks/streams:72h default respects explicit timeout; durable completion/refund is once-only. Kling bills bounded merged metadata duration. Preserve host-side multiplier safety, native uploads and secret-header stripping, and terminal stream usage even under slow delivery.
4. Frontend/sync: adopt keep/multiply/divide adjustments without silently relabeling CNY as USD; immutable confirmation must carry exact source and expected_version. Keep whole-batch prerequisite/metadata conflict handling, Seedance-only menu authority, orphan-removal intent, source-currency display switch, single tier and vendor/date order. Add cc12h to local quotation/calendar/display consumers.

## Environment policy

Main push and all-production mean exactly production-molii, production-claudeye and production-model-claudeye. Also inspect documentation deployment, which currently has an independent four-site matrix; it must not mutate iXiaozu when the other sites upgrade.

Keep production-ixiaozu only as a standalone manually requested target. A separate explicit approval input and approval date must match the real current Asia/Shanghai day; only20:00:00 through23:59:59 is permitted. Omission, false approval, malformed/stale date, wrong event/branch, a daytime dispatch, queued retry after midnight, or bulk selection must not contact or mutate that environment. Recheck at preparation, before each remote operation attempt, and before server-side state-changing operations. No caller-supplied clock bypass is accepted in production. Deterministic tests inject clocks only at test boundaries.

Prefer a reusable small policy gate with real CLI exit-code/mutation tests over source-string assertions. The server uses its real timezone data and requires the approval transiently; do not persist permission in .env.runtime. Fail closed if required clock/zone support is unavailable. Existing rollback stays unchanged for the three ordinary production sites; iXiaozu operations cannot silently continue outside the approved window. This turn never sets its approval input or dispatches it.

The controls are staged on develop, not magically active on untouched main. Once the user later explicitly approves the verified main promotion, the same push reads the new workflow/allowlist and excludes iXiaozu. Do not modify current main merely to activate this early.

## Verification and delivery

Use task-owned loopback PostgreSQL15/Redis7, never existing local services or live DBs. Baseline deployment29Python/192mocked shell assertions and pure billingexpr/billingmoney/jsplugin pass. Extend audited normal/race inventories with new critical regressions; no empty selections or required skips. Independently build/vet root, relaykit and tokenkit; Bun1.4.4 frozen install/check/full frontend tests/i18n and three eligible brand builds. Native independent reviews cover billing/catalog and routing/security/deploy, including default matrices, missing approval and19:59/20:00/23:59/midnight transitions with zero contact on denial.

After local acceptance commit/push upgrade source and actual develop without main. Wait every development workflow triggered by changed application/documentation policies and final deployed version/health/repeat startup checks. Capture public price/currency/vendor/rate before and after; do not mutate live prices, execute catalog apply, provision credentials or run paid upstream calls. Report only after all required development CI/CD and acceptance are green. Consolidate task evidence and archive once; no status/heartbeat-only commit stream.
