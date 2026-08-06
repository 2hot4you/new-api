# Generation Log Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing history-log cleanup task delete old terminal generation tasks as well as ordinary log rows, while preserving every active generation task and reporting both counts.

**Architecture:** Add focused terminal-task cleanup queries in the model layer, then extend the existing resumable system-task loop to process `logs` and `tasks` under one combined progress state. Keep the current API route and legacy aggregate fields, and add category counters consumed by a small frontend result helper.

**Tech Stack:** Go, GORM, SQLite test database, React 19, TypeScript, node:test, i18next.

---

### Task 1: Terminal generation-task cleanup queries

**Files:**
- Create: `model/task_cleanup.go`
- Create: `model/task_cleanup_test.go`

- [ ] **Step 1: Write the failing model test**

Create tasks on both sides of the cutoff with every relevant status and assert that only old `SUCCESS` and `FAILURE` rows are counted and deleted:

```go
func TestDeleteOldTerminalTaskBatchPreservesActiveAndRecentTasks(t *testing.T) {
    truncateTables(t)
    cutoff := time.Now().Unix() - 100
    rows := []*Task{
        {TaskID: "old-success", CreatedAt: cutoff - 20, Status: TaskStatusSuccess},
        {TaskID: "old-failure", CreatedAt: cutoff - 10, Status: TaskStatusFailure},
        {TaskID: "old-running", CreatedAt: cutoff - 30, Status: TaskStatusInProgress},
        {TaskID: "old-queued", CreatedAt: cutoff - 40, Status: TaskStatusQueued},
        {TaskID: "recent-success", CreatedAt: cutoff + 10, Status: TaskStatusSuccess},
    }
    for _, row := range rows {
        row.UpdatedAt = row.CreatedAt
        require.NoError(t, DB.Create(row).Error)
    }

    count, err := CountOldTerminalTasks(context.Background(), cutoff)
    require.NoError(t, err)
    assert.Equal(t, int64(2), count)

    deleted, err := DeleteOldTerminalTaskBatch(context.Background(), cutoff, 100)
    require.NoError(t, err)
    assert.Equal(t, int64(2), deleted)

    var remaining []Task
    require.NoError(t, DB.Order("task_id").Find(&remaining).Error)
    remainingIDs := make([]string, 0, len(remaining))
    for _, task := range remaining {
        remainingIDs = append(remainingIDs, task.TaskID)
    }
    assert.Equal(t, []string{"old-queued", "old-running", "recent-success"}, remainingIDs)
}
```

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./model -run TestDeleteOldTerminalTaskBatchPreservesActiveAndRecentTasks -count=1`

Expected: build failure because `CountOldTerminalTasks` and `DeleteOldTerminalTaskBatch` do not exist.

- [ ] **Step 3: Implement the focused queries**

Create `model/task_cleanup.go` with one shared scope:

```go
func oldTerminalTaskQuery(ctx context.Context, targetTimestamp int64) *gorm.DB {
    return DB.WithContext(ctx).Model(&Task{}).
        Where("created_at < ?", targetTimestamp).
        Where("status IN ?", []TaskStatus{TaskStatusSuccess, TaskStatusFailure})
}

func CountOldTerminalTasks(ctx context.Context, targetTimestamp int64) (int64, error) {
    var total int64
    err := oldTerminalTaskQuery(ctx, targetTimestamp).Count(&total).Error
    return total, err
}

func DeleteOldTerminalTaskBatch(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
    if limit <= 0 { limit = 100 }
    if err := ctx.Err(); err != nil { return 0, err }
    result := oldTerminalTaskQuery(ctx, targetTimestamp).Limit(limit).Delete(&Task{})
    return result.RowsAffected, result.Error
}
```

- [ ] **Step 4: Run the model test and full model package**

Run: `go test ./model -run 'Test(Delete|Count)OldTerminalTask' -count=1 && go test ./model`

Expected: PASS.

- [ ] **Step 5: Commit the model-layer change**

```bash
git add model/task_cleanup.go model/task_cleanup_test.go
git commit -m "fix: add terminal generation task cleanup"
```

### Task 2: Combined resumable system-task cleanup

**Files:**
- Modify: `service/system_task.go`
- Modify: `service/system_task_test.go`

- [ ] **Step 1: Write a failing end-to-end service test**

Seed two old `model.Log` rows, one old successful task, one old failed task, one old in-progress task and one recent successful task. Create and claim a log-cleanup system task, call `runLogCleanupTask`, then assert:

```go
assert.Equal(t, int64(4), result.DeletedCount)
assert.Equal(t, int64(2), result.DeletedLogCount)
assert.Equal(t, int64(2), result.DeletedGenerationCount)
assert.Equal(t, int64(0), remainingOldLogs)
assert.Equal(t, []string{"old-running", "recent-success"}, remainingTaskIDs)
```

Also decode the final state and assert `Processed == Total == 4`, `Remaining == 0`, and `Progress == 100`.

- [ ] **Step 2: Run the test and verify RED**

Run: `go test ./service -run TestRunLogCleanupTaskDeletesLogsAndTerminalGenerationTasks -count=1`

Expected: failure because the cleanup result has no category counters and old terminal tasks remain.

- [ ] **Step 3: Extend state and result without breaking legacy fields**

Update the structs:

```go
type LogCleanupState struct {
    Total                  int64 `json:"total"`
    Processed              int64 `json:"processed"`
    Progress               int   `json:"progress"`
    Remaining              int64 `json:"remaining"`
    DeletedLogCount        int64 `json:"deleted_log_count"`
    DeletedGenerationCount int64 `json:"deleted_generation_count"`
}

type LogCleanupResult struct {
    DeletedCount           int64 `json:"deleted_count"`
    DeletedLogCount        int64 `json:"deleted_log_count"`
    DeletedGenerationCount int64 `json:"deleted_generation_count"`
}
```

In each loop pass, recount both data sources, set `remaining := logRemaining + generationRemaining`, grow `Total` to at least `Processed + Remaining`, delete a log batch when `logRemaining > 0`, otherwise delete one terminal-task batch, and increment the matching category field. Preserve the existing no-progress failure guard and lock-renewal state writes.

- [ ] **Step 4: Return the combined and category totals**

Build the result from final state:

```go
result := LogCleanupResult{
    DeletedCount: state.Processed,
    DeletedLogCount: state.DeletedLogCount,
    DeletedGenerationCount: state.DeletedGenerationCount,
}
```

- [ ] **Step 5: Run focused and package tests**

Run: `go test ./service -run 'TestRunLogCleanupTask|TestSystemTask' -count=1 && go test ./service`

Expected: PASS and the in-progress task remains.

- [ ] **Step 6: Commit the service change**

```bash
git add service/system_task.go service/system_task_test.go
git commit -m "fix: clean terminal generation records with logs"
```

### Task 3: Maintenance UI category reporting

**Files:**
- Create: `web/src/features/system-settings/maintenance/log-cleanup-result.ts`
- Create: `web/src/features/system-settings/maintenance/__tests__/log-cleanup-result.test.ts`
- Modify: `web/src/features/system-settings/types.ts`
- Modify: `web/src/features/system-settings/maintenance/log-settings-section.tsx`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`

- [ ] **Step 1: Write the failing result-normalization test**

```ts
test('prefers category counts and preserves legacy aggregate fallback', () => {
  assert.deepEqual(
    getLogCleanupResultCounts({
      deleted_count: 7,
      deleted_log_count: 5,
      deleted_generation_count: 2,
    }),
    { total: 7, logs: 5, generations: 2, categorized: true }
  )
  assert.deepEqual(getLogCleanupResultCounts({ deleted_count: 3 }), {
    total: 3,
    logs: 3,
    generations: 0,
    categorized: false,
  })
})
```

- [ ] **Step 2: Run the test and verify RED**

Run from `web`: `bun test src/features/system-settings/maintenance/__tests__/log-cleanup-result.test.ts`

Expected: module-not-found failure.

- [ ] **Step 3: Add typed category fields and the pure helper**

Make both new fields optional for old system-task rows:

```ts
export type LogCleanupTaskResult = {
  deleted_count: number
  deleted_log_count?: number
  deleted_generation_count?: number
}
```

The helper must clamp invalid values to zero and set `categorized` only when at least one category field is present.

- [ ] **Step 4: Render the detailed completion toast**

When the task succeeds and `categorized` is true, show:

```ts
t('Removed {{logs}} log entries and {{generations}} generation records.', {
  logs: counts.logs,
  generations: counts.generations,
})
```

Keep the legacy aggregate message for historical task rows and the existing empty-result message for total zero. Update the maintenance description and confirmation text to say that old terminal generation records are included while active jobs are preserved.

- [ ] **Step 5: Add exact English and Chinese translations**

Add translations for the detailed result, terminal-record cleanup explanation and active-task preservation note to `en.json` and `zh.json`.

- [ ] **Step 6: Run frontend checks for this track**

Run from `web`:

```bash
bun test src/features/system-settings
pnpm typecheck
pnpm exec oxlint -c .oxlintrc.json src/features/system-settings/maintenance
pnpm format:check
```

Expected: PASS.

- [ ] **Step 7: Commit the maintenance UI change**

```bash
git add web/src/features/system-settings/maintenance web/src/features/system-settings/types.ts web/src/i18n/locales/en.json web/src/i18n/locales/zh.json
git commit -m "fix: report generation records in log cleanup"
```

### Task 4: Track verification

**Files:**
- No production file changes expected.

- [ ] **Step 1: Run complete backend verification**

Run: `gofmt -w model/task_cleanup.go model/task_cleanup_test.go service/system_task.go service/system_task_test.go && go test ./...`

Expected: PASS.

- [ ] **Step 2: Run complete frontend verification**

Run from `web`: `bun test src/features/system-settings && pnpm typecheck && pnpm format:check && pnpm build`

Expected: PASS.

- [ ] **Step 3: Verify the destructive boundary in the browser**

At `http://127.0.0.1:3000/system-settings/operations/logs`, open the cleanup confirmation and verify it explicitly mentions terminal generation records and preservation of active jobs. Use seeded disposable records for an interaction test; do not delete user-owned current records during QA.

- [ ] **Step 4: Check the final diff**

Run: `git diff --check && git status --short`

Expected: only files named in this plan are changed.
