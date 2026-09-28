# New API rc.40 Upgrade Implementation Plan

> **For implementers:** Follow this plan with `superpowers:subagent-driven-development` and `superpowers:test-driven-development`. Core release integrations are sequential because later tags depend on earlier conflict resolutions. Use separate agents only for disjoint validation or documentation work.

**Goal:** Bring the Molii New API fork from upstream rc.36 through rc.40 while preserving persisted data, custom providers, billing, assets, branding, and deployment behavior.

**Architecture:** Merge each official tag in order and treat each as a tested checkpoint. Before resolving a risky compatibility area, add a test that demonstrates the unsafe upstream behavior. Keep Molii channel IDs and async settlement authoritative while adopting upstream features around them.

**Stack:** Go, Gin, GORM, PostgreSQL/MySQL/SQLite, React, TypeScript, Rsbuild, Vitest, Bun, Docker.

---

## Task 1: Record the baseline and add compatibility guardrails

**Files:**

- Create: `constant/channel_persistence_test.go`
- Modify: `constant/channel_test.go`
- Modify: `.ccg/tasks/upgrade-upstream-rc40/task.json`
- Create: `.ccg/tasks/upgrade-upstream-rc40/context.jsonl`

**Step 1: Write failing literal-ID tests**

Add tests that assert literal persisted values map to StarAI `61`, Molii Grok `62`, Task Plugin `63`, and ByteDance Seedance `64`, and that each resolves to the correct name/base URL/adaptor family. Do not construct expected values from the constants under test.

**Step 2: Run the focused tests**

Run: `go test ./constant ./relay/... ./controller/... -run 'ChannelType|PersistedChannel|Adaptor' -count=1`

Expected: existing Molii IDs pass; add a deliberately unsafe upstream vLLM/SGLang expectation so the test is red until their safe IDs are introduced during rc.38.

**Step 3: Capture baseline commands and results**

After `bun run build` creates `web/dist`, run:

- `go test ./... -count=1`
- `(cd relaykit && GOWORK=off go test ./... -count=1)`
- `(cd web && bun run test)`
- `(cd web && bun run typecheck && bun run build)`

Record only pre-existing failures in `context.jsonl` with command, exit status, and concise reason.

**Step 4: Commit**

```bash
git add constant/channel_persistence_test.go constant/channel_test.go .ccg/tasks/upgrade-upstream-rc40
git commit -m "test: guard persisted Molii channel identities"
```

## Task 2: Integrate v1.0.0-rc.37

**Files:**

- Merge: `v1.0.0-rc.37`
- Modify: `model/main.go`, `model/option.go`, Passkey model/service/controller files
- Modify: `setting/task_pricing_setting/*`, `setting/ratio_setting/*`, pricing controllers/services
- Modify: image/task relay and billing files touched by the merge
- Modify: channel form/components and related frontend tests touched by the merge
- Create/modify: focused migration, pricing, image-quantity, and channel-form tests

**Step 1: Merge without committing**

Run: `git merge --no-ff --no-commit v1.0.0-rc.37`

Capture the conflict list in the task context. Resolve generated/localization changes only after their source files are correct.

**Step 2: Write migration tests before resolving migration behavior**

Cover an old `options` table without the required unique key, preservation of every row, creation/retention of the legacy table when needed, repeated startup, and Passkey credentials with and without `rp_id`.

Run the focused migration tests and confirm they fail for the expected missing rc.37 behavior.

**Step 3: Resolve database and Passkey changes**

Adopt upstream cross-dialect behavior while preserving Molii authentication/session security. Make migration failure observable and keep startup behavior compatible with the application's existing policy.

**Step 4: Write and resolve pricing/billing tests**

Add fixtures for current administrator overrides, GPT Image 2 variants, image quantity, Task Plugin streaming, StarAI, Grok, and Seedance. Confirm the effective-price snapshot is unchanged except for explicitly adopted upstream fixes.

**Step 5: Resolve frontend/channel conflicts**

Adopt unified create/edit and pricing UI while preserving Molii provider fields, branding, key disclosure, marketplace, and navigation behavior.

**Step 6: Verify rc.37 checkpoint**

Run focused migration/pricing/task/image tests, then all baseline commands. Inspect `git diff --check` and the merge diff for accidental deletion of fork-only files.

**Step 7: Commit**

```bash
git commit -m "merge: integrate new-api v1.0.0-rc.37"
```

## Task 3: Integrate v1.0.0-rc.38 channel registry and routing

**Files:**

- Merge: `v1.0.0-rc.38`
- Modify: `constant/channel.go`
- Modify: channel adaptor registry, routing middleware/controllers, channel model helpers
- Modify: frontend channel constants/options/icons/forms
- Modify: `constant/channel_persistence_test.go` and channel CRUD/routing tests

**Step 1: Merge without committing and isolate the channel-ID conflicts**

Run: `git merge --no-ff --no-commit v1.0.0-rc.38`

Do not finish unrelated conflict resolution until the channel registry contract is testable.

**Step 2: Keep the new-channel assertions red**

Extend the literal-ID test so vLLM and SGLang must use audited unused values, expected to be `65` and `66`. Add CRUD round-trip tests that persist literal values `61`–`66` and verify provider selection after reload.

**Step 3: Implement the safe registry**

Preserve `61`–`64`; assign vLLM/SGLang to unused values consistently across backend and frontend. If `65` or `66` is discovered to be persisted by another fork type, use the next free values and document the allocation.

**Step 4: Verify model discovery and routing**

Test base URLs, adaptor dispatch, model lists, model refresh, create/edit payloads, and channel tests for StarAI, Grok, Task Plugin, ByteDance Seedance, vLLM, and SGLang.

**Step 5: Continue with rc.38 protocol work**

Proceed to Task 4 while the merge remains open.

## Task 4: Complete v1.0.0-rc.38 policies, Responses WebSocket, and settlement

**Files:**

- Modify: request-policy settings, middleware, routing records, rate-limit and stream-outcome files
- Modify: Responses WebSocket router/controller/relay files
- Modify: `relaykit/relayconvert/**` as required by the tag
- Modify: channel advanced settings UI and tests
- Create/modify: WebSocket, request-policy, rate-limit, stream settlement, and pass-through tests

**Step 1: Write protocol outcome tests**

Cover successful completion, upstream rejection, disconnect before terminal usage, cancellation, refund, rate-limit slot release, `stream_id` correlation, and per-route pass-through.

**Step 2: Resolve backend protocol conflicts**

Adopt shared HTTP/WebSocket routing and rc.38 outcome classification. Preserve Molii billing hooks and ensure every failure path settles or refunds once.

**Step 3: Resolve relaykit conversion changes**

Run relaykit tests independently with `GOWORK=off`. Preserve existing Molii media/model conversions where upstream changes touch the same paths.

**Step 4: Resolve frontend controls**

Expose WebSocket, pass-through, health, and request-policy controls only for supported channel types. Preserve Molii custom forms and translated labels.

**Step 5: Verify rc.38 checkpoint**

Run focused routing/WebSocket/billing tests, relaykit full tests, root full tests, and frontend checks. Verify proxy configuration documentation covers WebSocket upgrades and timeouts.

**Step 6: Commit**

```bash
git commit -m "merge: integrate new-api v1.0.0-rc.38"
```

## Task 5: Integrate v1.0.0-rc.39 billing and Task Plugin routing

**Files:**

- Merge: `v1.0.0-rc.39`
- Modify: quota/pre-consume settings and tests
- Modify: async task finalization, transaction, outbox, and performance metric files/tests
- Modify: Task Plugin bindings, plugin protocol/model metadata, image relay, and UI files/tests
- Modify: Molii StarAI/Grok/ByteDance Seedance task and billing files only where required by conflicts

**Step 1: Merge without committing**

Run: `git merge --no-ff --no-commit v1.0.0-rc.39`

**Step 2: Write the pre-consume default test**

Assert a missing `quota_setting.trust_quota_usd` setting behaves as `0`, a configured positive value enables the bypass only below its threshold, and the input multiplier defaults to `1`. Verify subscriptions and forced pre-consume never bypass reservation.

Confirm the default assertion fails against the unmodified upstream rc.39 value of USD 10.

**Step 3: Implement fork-safe quota defaults**

Adopt the settings and UI, but keep the fork default at `0`. Preserve explicit administrator values during migration and serialization.

**Step 4: Document the reservation-formula boundary**

Add focused before/after expectations for generic text, GPT Image 2, mapped aliases, Grok image/video, StarAI, Seedance, fixed-request expressions, and zero group-ratio cases. Preserve Molii custom/fixed paths. Adopt the upstream generic-text reservation formula only with an explicit test showing that the multiplier changes reservation but not final settlement.

**Step 5: Write terminal race and outbox tests**

Cover two workers observing the same terminal task, one CAS winner, one outbox event, one charge/refund, retry behavior, and exactly one performance sample after the winning funds transaction commits.

**Step 6: Reconcile finalization**

Keep Molii's transactional CAS + outbox settlement. Reconcile rc.39's polling sample with the existing `service/task_billing_reconcile.go` post-commit sample so there is one owner. Do not call an upstream direct-settlement path that bypasses the outbox.

**Step 7: Protect provider ownership**

Test that existing StarAI/Grok/ByteDance channels and model names keep their adaptors. Keep the native Seedance bridge limited to the supported Seedance 2.x video models, excluding Seedream, Seedance 1.x, native image, and Responses traffic. Adopt Alibaba OpenAI Images support and Doubao native Seedream/Responses capabilities without automatically routing existing `/v1/images` traffic or binding Molii models.

**Step 8: Preserve task-list display facts**

Write a failing task-list and video-workbench pagination test for a stored ByteDance Seedance task whose actual generation facts exist in `task.Data`. Implement a stable private-data projection or a narrow platform-aware query exception before adopting upstream's general `Omit("data")` optimization. Verify the response keeps actual resolution, size, duration, and related display facts without returning raw sensitive task data.

**Step 9: Verify rc.39 checkpoint**

Run targeted quota, async race, outbox, plugin binding, image, Seedance, Grok, and StarAI tests, then the full baseline matrix.

**Step 10: Commit**

```bash
git commit -m "merge: integrate new-api v1.0.0-rc.39"
```

## Task 6: Integrate v1.0.0-rc.40

**Files:**

- Merge: `v1.0.0-rc.40`
- Modify: `model/task_plugin.go` and migration tests
- Modify: task submission status handling and tests
- Modify: `relaykit/relayconvert/**` media conversion tests
- Modify: Task Plugin pricing/editor and theme storage UI/tests

**Step 1: Merge without committing**

Run: `git merge --no-ff --no-commit v1.0.0-rc.40`

**Step 2: Write migration and upload-limit tests**

Verify existing Task Plugin source/icon data survives migration, PostgreSQL remains `text`, MySQL receives long-text semantics, restart is idempotent, payloads up to 8 MiB are accepted, and larger payloads are rejected. Use real MySQL/PostgreSQL DSNs; a skipped integration test is not a pass.

**Step 3: Resolve task submission and pricing changes**

Adopt any 2xx status as success. Preserve Molii provider response parsing and outbox behavior. Verify Task Plugin pricing remains editable after enum narrowing.

**Step 4: Resolve media conversion and theme storage**

Adopt Claude/Responses tool-result media hoisting and Alibaba metadata retention. Keep COS and temporary-asset behavior. Add tests proving stale local theme/customization values cannot override Molii defaults and the locked setters remain no-ops; reuse upstream storage isolation helpers only where they do not re-enable customization.

**Step 5: Verify rc.40 checkpoint**

Run focused migration, task, media, plugin editor, and theme tests, followed by all backend, relaykit, frontend, deployment, and contract checks.

**Step 6: Commit**

```bash
git commit -m "merge: integrate new-api v1.0.0-rc.40"
```

## Task 7: Cross-release regression and operational rehearsal

**Files:**

- Modify/create: deployment contract tests as indicated by failures
- Modify: `docs/deployment/*` only for verified upgrade/rollback instructions
- Create: `.ccg/tasks/upgrade-upstream-rc40/review.md`

**Step 1: Run static and full test gates**

Run:

- `git diff --check`
- `go test ./... -count=1`
- `go vet ./...`
- `(cd relaykit && GOWORK=off go test ./... -count=1)`
- `(cd relaykit && GOWORK=off go vet ./...)`
- `(cd web && bun run lint)`
- `(cd web && bun run test)`
- `(cd web && bun run typecheck)`
- `(cd web && bun run build)`
- `(cd web && bun run i18n:check)`
- `(cd web && bun run copyright:check)`
- Repository deployment/branding contract suites discovered in the project instructions.

**Step 2: Exercise migrations on a database clone**

Back up and export the required tables/settings, start the upgraded service twice, inspect the `options` constraint and any `options_legacy_*` table, verify Task Plugin data, and compare effective prices.

**Step 3: Exercise runtime smoke tests**

Verify `/api/status`, login/Passkey, literal channel identities, channel editing, request policies, Responses WebSocket through the proxy, representative StarAI/Grok/Seedance/Task Plugin tasks, asset access, marketplace, branding, and multi-site deployment inputs.

**Step 4: Rehearse rollback**

Using the clone, restore the prior image and database snapshot together. For MySQL, verify the prior binary does not narrow Task Plugin fields or start against the upgraded schema without the coordinated restore. Record the exact recovery point, post-upgrade billing cutoff, and validation result.

**Step 5: Perform dual review**

Run the configured antigravity and Claude review in parallel if the wrapper becomes available. Otherwise record the unavailable wrapper, run independent internal reviews, and do not describe them as external-model approval. Fix every Critical issue and re-review; resolve or explicitly accept Warnings.

**Step 6: Final task record**

Update `review.md`, task status, verification evidence, release checkpoint SHAs, and remaining deployment actions. Do not push or deploy without separate authorization.

## Task 8: Archive after accepted completion

**Files:**

- Move: `.ccg/tasks/upgrade-upstream-rc40` to `.ccg/tasks/archive/2026-09/upgrade-upstream-rc40`
- Modify: `.ccg/spec/{domain}/index.md` only if a reusable, non-obvious convention was learned

**Step 1: Check for spec evolution**

Record only durable conventions such as the persisted channel-ID allocation policy or async terminal-settlement invariant; do not add task-specific notes as general rules.

**Step 2: Archive and commit**

```bash
mkdir -p .ccg/tasks/archive/2026-09
mv .ccg/tasks/upgrade-upstream-rc40 .ccg/tasks/archive/2026-09/
git add .ccg/tasks .ccg/spec
git commit -m "chore: archive ccg task upgrade-upstream-rc40"
```

**Step 3: Handoff**

Report the final checkpoint SHAs, tests, known baseline exceptions, database-clone evidence, rollback evidence, and the fact that production deployment remains separately authorized.
