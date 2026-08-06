# Hide Grok Video Unsupported Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hide the unsupported Dimensions and Frame Rate detail cards for Molii Grok video previews while preserving them for Seedance and other video platforms.

**Architecture:** Add one exported pure platform-capability predicate beside the reusable video preview dialog. The dialog uses that predicate for conditional rendering, and a focused unit test protects both the Grok exclusion and Seedance compatibility behavior.

**Tech Stack:** React 19, TypeScript, Bun test runner, Rsbuild, Go embedded frontend

---

### Task 1: Add the platform capability regression test

**Files:**
- Create: `web/src/features/usage-logs/components/dialogs/__tests__/video-preview-metadata.test.ts`
- Modify: `web/src/features/usage-logs/components/dialogs/video-preview-dialog.tsx`

- [ ] **Step 1: Write the failing test**

```ts
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { TASK_PLATFORMS } from '../../../constants'
import { shouldShowVideoTechnicalMetadata } from '../video-preview-dialog'

describe('video preview metadata visibility', () => {
  test('hides dimensions and frame rate for Molii Grok tasks', () => {
    assert.equal(
      shouldShowVideoTechnicalMetadata(TASK_PLATFORMS.MOLII_GROK),
      false
    )
  })

  test('preserves dimensions and frame rate for Seedance and other platforms', () => {
    assert.equal(
      shouldShowVideoTechnicalMetadata(TASK_PLATFORMS.STARAI),
      true
    )
    assert.equal(shouldShowVideoTechnicalMetadata('runway'), true)
  })
})
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
cd web
bun test src/features/usage-logs/components/dialogs/__tests__/video-preview-metadata.test.ts
```

Expected: FAIL because `shouldShowVideoTechnicalMetadata` is not exported.

- [ ] **Step 3: Add the minimal predicate and conditional rendering**

Import `TASK_PLATFORMS` in `video-preview-dialog.tsx`, then add:

```ts
export function shouldShowVideoTechnicalMetadata(platform: string): boolean {
  return platform !== TASK_PLATFORMS.MOLII_GROK
}
```

Inside `VideoPreviewDialog`, derive:

```ts
const showTechnicalMetadata = shouldShowVideoTechnicalMetadata(log.platform)
```

Wrap only the Dimensions and Frame Rate `DetailItem` elements:

```tsx
{showTechnicalMetadata ? (
  <DetailItem label={t('Dimensions')} value={dimensions} mono />
) : null}

{showTechnicalMetadata ? (
  <DetailItem
    label={t('Frame Rate')}
    value={params?.fps ? `${params.fps} FPS` : '-'}
  />
) : null}
```

- [ ] **Step 4: Run the target test and usage-logs suite**

Run:

```bash
cd web
bun test src/features/usage-logs/components/dialogs/__tests__/video-preview-metadata.test.ts
bun test src/features/usage-logs
```

Expected: target tests PASS; all usage-logs tests PASS.

### Task 2: Verify, rebuild, and validate the rendered dialog

**Files:**
- Modify: `.ccg/tasks/hide-grok-video-unsupported-metadata/task.json`
- Create: `.ccg/tasks/hide-grok-video-unsupported-metadata/review.md`

- [ ] **Step 1: Run static and build verification**

```bash
cd web
pnpm typecheck
pnpm format:check
pnpm build
cd ..
go test ./...
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 2: Rebuild and restart the LaunchAgent service**

```bash
MOLII_APP_DIR='/Users/naf/Library/Application Support/Molii/new-api'
go build -o "$MOLII_APP_DIR/new-api.next" .
mv "$MOLII_APP_DIR/new-api.next" "$MOLII_APP_DIR/new-api"
launchctl kickstart -k gui/$(id -u)/com.molii.new-api
```

Wait for `http://127.0.0.1:3000/api/status` to return 200.

- [ ] **Step 3: Validate in the browser**

Open `http://127.0.0.1:3000/usage-logs/task?source=grok-video&page=1`, preview an existing successful Grok video task, and verify:

- “尺寸” is absent.
- “帧率” is absent.
- Resolution, duration, reference-video status, model and generation timing remain visible.
- Video playback still loads through the signed Molii content path.
- Browser error/warn log is empty.

- [ ] **Step 4: Review, commit, and archive**

Review the complete diff for scope and secrets, commit the functional change, update the task to completed, move it to `.ccg/tasks/archive/2026-08/`, and commit the archive. Do not push or merge.
