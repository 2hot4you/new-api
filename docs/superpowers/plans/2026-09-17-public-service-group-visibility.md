# Public Service Group Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep identity-only groups out of the public pricing directory and group-success rankings while preserving user-specific service-group discounts.

**Architecture:** Add a selectable-group resolver that applies explicit per-identity additions/removals without the legacy own-group fallback. Public pricing consumes this resolver and further intersects it with groups that actually occur on visible pricing rows. Public rankings intersect active ratio groups with the global user-selectable registry. Existing routable-group resolution remains unchanged for backward compatibility.

**Tech Stack:** Go, Gin, React/TypeScript, Vitest, testify

**Spec:** `.ccg/tasks/public-service-group-visibility/requirements.md`

## Global Constraints

- Preserve `GroupGroupRatio(identityGroup, serviceGroup)` billing semantics.
- Preserve legacy authentication and routing behavior.
- Use `UserUsableGroups` as the source of truth for the “User selectable” setting.
- Add regression tests before production code.

---

### Task 1: Separate selectable groups from legacy routable groups

**Files:**
- Modify: `service/group.go`
- Test: `service/group_auto_groups_test.go`

**Interfaces:**
- Produces: `GetUserSelectableGroups(userGroup string) map[string]string`
- Preserves: `GetUserUsableGroups(userGroup string) map[string]string`

- [x] Write a failing test proving an identity-only group is excluded while explicit special additions/removals are applied.
- [x] Run the focused service test and verify the expected failure.
- [x] Extract the base-plus-special resolver and keep the own-group fallback only in `GetUserUsableGroups`.
- [x] Run the focused service tests and verify they pass.

### Task 2: Filter public pricing groups without losing special ratios

**Files:**
- Modify: `controller/pricing.go`
- Test: `controller/pricing_group_metadata_test.go`
- Modify: `web/src/features/pricing/index.tsx`
- Modify: `web/src/features/pricing/lib/model-helpers.ts`
- Test: `web/src/features/pricing/lib/__tests__/model-helpers.test.ts`

**Interfaces:**
- Consumes: `service.GetUserSelectableGroups`
- Produces: pricing group metadata and filters containing only selectable groups with visible models

- [x] Write failing Go tests proving pricing display groups exclude the identity fallback and retain the ByteDance override.
- [x] Write a failing Vitest case proving empty selectable groups are omitted from the sidebar.
- [x] Run both focused suites and verify the expected failures.
- [x] Switch pricing display data to the selectable resolver and intersect sidebar groups with visible model groups.
- [x] Run both focused suites and verify they pass.

### Task 3: Restrict group-success rankings to globally selectable groups

**Files:**
- Modify: `service/rankings.go`
- Test: `service/rankings_test.go`

**Interfaces:**
- Consumes: `setting.GetUserUsableGroupsCopy()` and active group ratios
- Produces: `group_success` containing only active globally selectable service groups

- [x] Change the existing ranking configuration test so an identity-only active ratio must be absent.
- [x] Run the focused ranking test and verify it fails under the current implementation.
- [x] Filter configured ranking groups by the selectable registry while preserving metadata order, icons, descriptions, and zero-request rows.
- [x] Run the focused ranking tests and verify they pass.

### Task 4: Verification and review

**Files:**
- Modify: `.ccg/tasks/public-service-group-visibility/task.json`
- Create: `.ccg/tasks/public-service-group-visibility/review.md`

**Interfaces:**
- Consumes: completed production and test changes
- Produces: verification evidence and archived task record

- [x] Run focused Go and frontend tests.
- [x] Run Go formatting, TypeScript type-check, affected lint, production build, and `git diff --check`.
- [x] Review the diff for billing/routing regressions and record the unavailable external-review tooling.
- [x] Archive the CCG task under `.ccg/tasks/archive/2026-09/`.
