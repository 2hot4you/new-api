# Upgrade Molii new-api from rc.30 base to upstream rc.36

## Goal

Merge upstream `v1.0.0-rc.36` into Molii `develop` without regressing Molii-specific routing, billing, catalog, media, documentation, or administration features. The finished upgrade must compile and pass the relevant PostgreSQL, Redis, Go, relaykit, frontend, i18n, and production-build checks before it is considered mergeable.

## Global constraints

- Work only on branch `upgrade/upstream-rc36` in the isolated upgrade worktree.
- Merge the release tag once so future upstream upgrades retain ancestry.
- PostgreSQL and Redis are the acceptance data stores. SQLite is not an acceptance target.
- Preserve Molii durable task finalization plus billing-job outbox; every terminal polling path must use one idempotent settlement mechanism.
- Preserve private task/provider data boundaries, protected media delivery, COS-backed previews, and sanitized public errors.
- Preserve local model/vendor metadata authority, marketplace publication, order, custom capabilities and mixed-currency pricing.
- Preserve ordered API-key group/provider routing, model and CIDR restrictions, Molii AIGC logs, temporary assets, quotations, rankings and branded layouts.
- Adopt upstream session/action-bound security proofs, audit store, dual password hash readers, canonical billing identity, model modifiers, plugin metadata and safe relay fixes as cohesive contracts.
- Never resolve a whole customized directory using only `ours` or `theirs`.
- Do not connect to production databases or submit paid provider tasks during implementation.
- Do not push, merge into `develop`, or deploy until final verification and user authorization.

## Task 1 — Create the upstream merge and inventory the live conflicts

Files: Git index plus `.ccg/tasks/upgrade-upstream-rc36/**`.

1. Commit the task analysis and plan.
2. Merge `v1.0.0-rc.36` with `--no-commit`.
3. Record the exact conflict set and assign every conflict to one owner.
4. Preserve all unconflicted upstream additions for later semantic review.

Acceptance: merge is paused only on the inventoried conflicts; no unrelated worktree files changed.

## Task 2 — Resolve backend, data, security, polling, billing and relay contracts

Ownership: all non-`web/` conflicts and all fork-only Go adaptors/callers affected by upstream interface changes.

1. Merge dependencies and environment settings as a union, keeping Molii-only SDKs/settings.
2. Adopt upstream security proof, audit, password, OAuth and migration behavior while preserving Molii transactional registration/default-key behavior and stable public errors.
3. Adapt all task polling implementations and test doubles to the rc.36 interface.
4. Route new failure accounting and terminal conditions through Molii durable finalization/outbox.
5. Merge typed log metadata without exposing provider secrets and retain Molii media/billing fields.
6. Integrate canonical billing identity/model modifiers with Molii mappings and routing.
7. Preserve PostgreSQL migration wrappers plus all Molii tables/backfills.

Tests: focused Go package tests, task fault/idempotency tests, auth/security tests, PostgreSQL migration tests where disposable DSN is available, Redis tests where disposable DSN is available, root and relaykit compilation.

## Task 3 — Resolve frontend model, vendor, pricing, channel and plugin contracts

Ownership: `web/src/features/models/**`, `web/src/features/pricing/**`, channel drawer/API/type conflicts and their tests.

1. Keep Molii local catalog and marketplace order/publication behavior; do not restore retired upstream-sync ownership.
2. Port safe upstream vendor/model visibility, plugin metadata, default base URL and task price improvements only against actual merged backend endpoints.
3. Preserve Molii mixed-currency pricing, complete image/video matrices, Seedance/Grok/GPT Image 2 examples and quotation pricing inputs.
4. Reconcile channel key disclosure with the new scoped security proof contract.

Tests: models, pricing, channel and plugin feature tests plus typecheck for owned modules.

## Task 4 — Resolve frontend keys, usage logs, navigation, security UI and localization

Ownership: remaining `web/` conflicts, generated routes, locale JSON, layout and their tests.

1. Preserve Molii ordered multi-group routing, icons, success rates, model/IP details and CIDR editing while adopting upstream quota/table primitives.
2. Add upstream audit/security pages and proof flows while retaining branded auth/layout.
3. Preserve every Molii media preview/download/billing renderer and task-family route while integrating audit/group-filter/mobile improvements.
4. Keep `/product-quotation`, `/temporary-assets`, `/security` and `/usage-logs/audit` with correct guards.
5. Object-merge locale keys and regenerate the route tree.

Tests: keys, usage logs, auth/security, navigation, i18n and build checks.

## Task 5 — Integrate, test and repair cross-domain failures

1. Confirm no conflict markers or unmerged index entries remain.
2. Run formatting and dependency normalization.
3. Run focused suites, complete Go tests, relaykit tests, frontend typecheck/lint/i18n/tests and production build.
4. Exercise PostgreSQL startup/migration twice and Redis session/proof behavior using disposable development resources only.
5. Perform browser smoke checks for anonymous auth, API keys, models, pricing, logs, temporary assets, quotation, security and audit routes.
6. Repair failures with focused regression tests.

Acceptance: all available gates pass; any skipped external-resource test is explicitly reported and cannot be counted as passed.

## Task 6 — Review and handoff

1. Run independent backend and frontend reviews over the complete branch diff.
2. Fix Critical and Important findings and re-run affected tests.
3. Record review results, task decisions and remaining operational rollout steps.
4. Archive the CCG task only when implementation and verification are complete.
5. Present the verified branch to the user; push or merge only after authorization.
