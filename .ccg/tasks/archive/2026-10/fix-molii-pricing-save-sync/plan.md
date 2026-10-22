# Molii pricing-save and environment-sync Implementation Plan

> For agentic workers: use subagent-driven-development with user-mandated disjoint-file parallel ownership.

**Goal:** Generic pricing saves exclude dedicated Seedance models; approved dev-to-molii synchronization becomes ready without writing pricing.
**Architecture:** Reuse existing dedicated-model predicate and UI components. No backend contract or catalog security changes. Provision existing runtime configuration only on two Molii containers.
**Tech stack:** React/TypeScript/Vitest, existing Docker Compose and environment-only catalog runtime.
**Spec:** requirements.md in this task directory.

## Global constraints
- No price changes, sync application, other environments or credentials in logs/repo.
- Tests first, preserve unrelated work, no pushes or code deployments authorized this turn.
- Existing root session, strict origin, verifier validation and dedicated Seedance backend guard remain intact.

2026-10-22 scope amendment: user explicitly approved backing up/removing ONLY the two DEV claude-opus-4-6 expression/mode leaves, followed by commit/push of current feature branch. All other price changes, sync apply/restore and additional merges remain excluded. Offline maintenance scripts and real isolated PostgreSQL safety tests are added under ops/.

## Review focus
- Mixed generic/Seedance selections must not reject the generic save or falsely report protected entries saved.
- Bulk selection respects filtered pages and disabled state and omits dedicated models from counts.
- Generic reset and JSON edits must not alter dedicated pricing.
- Runtime configuration must survive subsequent CI/CD while preserving image identity and deployment metadata.
- Unexpected active tasks, topology, configuration, or failed health must stop/roll back rather than change other services.

### Task 1: Upstream selection boundaries (worker 1)
Own: models/upstream-price-cells.tsx, models/upstream-ratio-sync-table.tsx, models/upstream-ratio-sync.tsx and __tests__/pricing-sync.test.tsx under web/src/features/system-settings. Reuse isSeedanceManagedPricingModel from existing API.
- [ ] Add failing mixed generic/dedicated model selection regression, including bulk and apply payload.
- [ ] Make protected models unselectable with existing translated dedicated-tab guidance; keep ordinary selection and confirmation unchanged; stale incoming selections may not construct saved resolutions.
- [ ] Verify focused tests, lint and report RED/GREEN evidence. Do not commit.

### Task 2: Generic save/reset defensive boundary (worker 2)
Own: web/src/features/model-pricing/api.ts, its __tests__/pricing.test.ts; web/src/features/system-settings/models/ratio-settings-card.tsx and existing appropriate ratio-settings test file (at most one new test if no suitable file). Do not edit Task 1 files.
- [ ] Add failing tests for mixed dirty dedicated/generic fields and generic reset preservation.
- [ ] Exclude protected entries in generic change builder and reset batches; retain versions, currencies and no-op behavior; do not claim dedicated edits saved. Use existing dedicated-tab message for form JSON dirty protected edits if needed.
- [ ] Verify targeted tests, lint; report exact files and RED/GREEN evidence. Do not commit.

### Task 3: Runtime setup (lead)
- [ ] Inventory compose metadata, exact images, env ownership, networks, tasks and current sync readiness read-only. Stop on unexpected preexisting configuration or active tasks.
- [ ] Prepare private staged env files and unique reader/verifier pairing, validate with existing runtime grammar and Compose; review provisioning before applying.
- [ ] Back up, install only catalog env additions, source-first then target app-only recreate with no image pull; health and version checks, auth/export connectivity checks. No sync write operations.

### Task 4: Integration review and verification
- [ ] Two independent read-only reviews (external wrapper unavailable, disclose fallback) for source diff and operational plan; fix scoped findings.
- [ ] Relevant and complete frontend tests, typecheck, changed-file lint, translation check, production build.
- [ ] Archive task and local commits. Final clearly separates local code fix from live runtime setup; no automatic push/deployment.

## Completion receipt

Tasks 1–3 completed and independently reviewed. Task 4 targeted verification is green; broad suite/lint limits are explicitly reported in review.md rather than claimed passing. Exact user-approved DEV cleanup completed with durable 0600 raw backup and healthy same-container restart. Source-first provisioning passed immutable image/config checks, real fixed-transport export validation, both public health checks, and authenticated browser target/management-ready checks. No catalog apply/restore submitted. Archive and explicitly authorized feature-branch push close the task; no develop/main merge.
