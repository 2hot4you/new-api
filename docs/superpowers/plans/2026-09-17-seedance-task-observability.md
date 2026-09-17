# Seedance Task Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give administrators a safe, precise Seedance timing breakdown, expose the platform task ID in common usage logs, link task request IDs to usage-log search, and show input-media counts in video previews.

**Architecture:** Persist a provider-neutral timing snapshot and media-count summary in the existing task private JSON. Build an admin-only timing DTO with a legacy fallback parser for already-sanitized StarAI task data; keep upstream IDs root-only and raw provider responses private. Reuse the existing request-ID filter contract and add focused React views for timing and counts.

**Tech Stack:** Go, GORM JSON fields, Gin DTO projection, React 19, TypeScript, TanStack Table/Router, Vitest.

**Spec:** `.ccg/tasks/seedance-task-observability/requirements.md`

## Global Constraints

- Detailed timing is administrator-only.
- Upstream task IDs remain root-only.
- Never expose raw StarAI responses, prompts, media URLs, asset IDs, or channel keys.
- Legacy missing values render as unavailable, never as zero.
- No new database tables or schema migrations.
- Implement each behavior test-first and verify the failing test before production changes.

---

### Task 1: Safe backend timing and media snapshots

**Files:**
- Modify: `model/task.go`
- Modify: `relay/common/relay_info.go`
- Modify: `relay/channel/task/starai/adaptor.go`
- Modify: `controller/relay.go`
- Create: `service/starai_task_observability.go`
- Create: `service/starai_task_observability_test.go`
- Modify: `service/task_polling.go`
- Modify: `relay/channel/task/starai/adaptor_test.go`

**Interfaces:**
- Produces `model.TaskTimingSnapshot` and `model.TaskInputMediaSummary` stored under `TaskPrivateData`.
- Produces `service.CaptureStarAITaskTiming(task, payload, observedAt)` and a safe timing projection builder.
- StarAI validation writes exact image/video/audio counts into `RelayInfo`; task creation persists them.

- [x] Write failing tests for nested StarAI timestamps, observed transition timestamps, clock skew, legacy missing values, and media counting.
- [x] Run the focused Go tests and confirm failures are caused by missing observability types/functions.
- [x] Add minimal snapshot types and StarAI extraction/count capture.
- [x] Update polling and task creation to persist only normalized timestamps and integer counts.
- [x] Run the focused Go tests until green.

### Task 2: Role-safe task DTO projection

**Files:**
- Modify: `dto/task.go`
- Modify: `controller/task.go`
- Modify: `controller/task_log_view_test.go`
- Modify: `controller/task_video_preview_test.go`

**Interfaces:**
- `TaskAdminInfo.timing` contains safe timestamps and derived durations only for administrators.
- `TaskVideoParams.input_image_count`, `input_video_count`, and `input_audio_count` are nullable so historical unknown data is distinct from zero.

- [x] Write failing controller tests proving normal users cannot receive timing, admins receive timing, root diagnostics remain unchanged, and media counts project without private data.
- [x] Run the focused controller tests and confirm the expected failures.
- [x] Implement the minimal DTO projection and legacy fallback.
- [x] Run the focused controller tests until green.

### Task 3: Common usage-log task ID and request-ID navigation

**Files:**
- Modify: `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- Modify: `web/src/features/usage-logs/components/dialogs/task-details-dialog.tsx`
- Create: `web/src/features/usage-logs/lib/task-log-links.ts`
- Create: `web/src/features/usage-logs/lib/__tests__/task-log-links.test.ts`
- Create: `web/src/features/usage-logs/components/__tests__/common-log-task-id.test.tsx`

**Interfaces:**
- `buildUsageLogRequestLink(requestId)` returns the canonical common-log URL using the existing `requestId` query key.
- Common logs render the public `other.task_id` as a copyable column for every viewer who can access that log row.

- [x] Write failing tests for canonical request-ID URLs and role-safe task-ID rendering.
- [x] Run the focused Vitest files and confirm failures.
- [x] Implement the URL builder, clickable request ID, and task-ID column.
- [x] Run the focused Vitest files until green.

### Task 4: Administrator timing dialog and preview media counts

**Files:**
- Modify: `web/src/features/usage-logs/types.ts`
- Modify: `web/src/features/usage-logs/components/columns/task-logs-columns.tsx`
- Create: `web/src/features/usage-logs/components/dialogs/task-timing-dialog.tsx`
- Create: `web/src/features/usage-logs/components/dialogs/__tests__/task-timing-dialog.test.tsx`
- Modify: `web/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx`
- Create: `web/src/features/usage-logs/components/dialogs/__tests__/video-preview-observability.test.tsx`

**Interfaces:**
- Admin duration cells open `TaskTimingDialog`; non-admin duration cells remain static.
- Video preview displays nullable image/video/audio counts and labels unknown historical values as unavailable.

- [x] Write failing component tests for admin-only duration interaction, detailed phase rendering, and exact/unknown media counts.
- [x] Run the focused Vitest files and confirm failures.
- [x] Implement the timing dialog and media-count cards using existing UI primitives.
- [x] Run the focused Vitest files until green.

### Task 5: Localization and verification

**Files:**
- Modify: `web/src/i18n/locales/*.json` through the existing synchronization command.
- Modify: `.ccg/tasks/seedance-task-observability/review.md`

**Interfaces:**
- Every new visible label has complete locale coverage.

- [x] Run the locale synchronization command and inspect Chinese and English labels.
- [x] Run focused Go tests, frontend tests, frontend typecheck, lint, and production build.
- [x] Inspect `git diff` for secrets, raw upstream payload exposure, and unrelated changes.
- [x] Record review findings and verification evidence in `review.md`.
- [x] Update the task state to completed and archive it under `.ccg/tasks/archive/2026-09/` after all checks pass.
