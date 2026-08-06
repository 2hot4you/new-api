# Grok Video Final Usage Logs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every new Molii Grok asynchronous video task emit exactly one detailed consumption log after success, or one sanitized error log after failure, with no user-visible precharge/refund/delta logs.

**Architecture:** Persist a versioned Grok video billing snapshot and a rollout gate in the existing task `private_data` JSON. Keep money reservation and settlement unchanged, but route new Grok tasks through the same terminal-only log lifecycle already used by Molii AIGC. Expose the final snapshot as `other.grok_video_billing` and render it with a dedicated frontend parser/card modeled on Grok Image.

**Tech Stack:** Go 1.26, Gin, GORM JSON fields, PostgreSQL/SQLite tests, React 19, TypeScript, Vitest/Node tests, Tailwind/shadcn components.

---

### Task 1: Persist the rollout gate and Grok video billing contract

**Files:**
- Modify: `model/task.go`
- Modify: `controller/relay.go`
- Modify: `service/task_billing_molii_grok_test.go`
- Modify: `relay/channel/task/moliigrok/adaptor_test.go`

- [ ] **Step 1: Write failing JSON round-trip and submit snapshot tests**

Add tests asserting that a newly submitted Molii Grok task stores `FinalUsageLogOnly=true` and a V1 snapshot with the exact model, operation, input type, requested/estimated parameters, input counts, and snapshotted unit prices. Add a model round-trip assertion so zero-valued prices survive JSON encoding.

```go
type GrokVideoBillingSnapshot struct {
    Version                    int     `json:"version"`
    Model                      string  `json:"model"`
    Operation                  string  `json:"operation"`
    InputType                  string  `json:"input_type"`
    RequestedDurationSeconds   float64 `json:"requested_duration_seconds"`
    EstimatedDurationSeconds   float64 `json:"estimated_duration_seconds"`
    ActualDurationSeconds      float64 `json:"actual_duration_seconds"`
    RequestedResolution        string  `json:"requested_resolution"`
    EstimatedResolution        string  `json:"estimated_resolution"`
    ActualResolution           string  `json:"actual_resolution"`
    AspectRatio                string  `json:"aspect_ratio"`
    InputImageCount            int     `json:"input_image_count"`
    VideoInputBilledSeconds    float64 `json:"video_input_billed_seconds"`
    OutputUnitPrice            float64 `json:"output_unit_price"`
    ImageInputUnitPrice        float64 `json:"image_input_unit_price"`
    VideoInputUnitPrice        float64 `json:"video_input_unit_price"`
    OutputCost                 float64 `json:"output_cost"`
    ImageInputCost             float64 `json:"image_input_cost"`
    VideoInputCost             float64 `json:"video_input_cost"`
    Subtotal                   float64 `json:"subtotal"`
    GroupRatio                 float64 `json:"group_ratio"`
    FinalCost                  float64 `json:"final_cost"`
}
```

- [ ] **Step 2: Run focused tests and verify RED**

Run:

```bash
go test ./model ./service ./relay/channel/task/moliigrok -run 'GrokVideoBilling|MoliiGrokFixedPriceBillingContext' -count=1
```

Expected: compile/test failure because the rollout field and snapshot do not exist.

- [ ] **Step 3: Add the persisted fields and snapshot builder inputs**

Add to `TaskBillingContext`:

```go
FinalUsageLogOnly bool                      `json:"final_usage_log_only,omitempty"`
GrokVideoBilling *GrokVideoBillingSnapshot `json:"grok_video_billing,omitempty"`
```

When `result.Platform` is `ChannelTypeMoliiGrokAIGC`, set the rollout flag and build the snapshot from the already validated `RelayInfo` fields. Use `video_edit`, `image_to_video`, or `text_to_video`; record only media counts/types, never URLs or file IDs. Existing in-flight tasks lack the flag and remain on the legacy path.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit the persisted contract**

```bash
git add model/task.go controller/relay.go service/task_billing_molii_grok_test.go relay/channel/task/moliigrok/adaptor_test.go
git commit -m "feat: persist Grok video billing snapshots"
```

### Task 2: Defer Grok user logs until terminal state

**Files:**
- Modify: `controller/relay.go`
- Modify: `service/task_billing.go`
- Modify: `service/task_polling.go`
- Modify: `service/task_billing_test.go`
- Modify: `service/task_polling_molii_grok_test.go`

- [ ] **Step 1: Write failing submit/success/failure/idempotency tests**

Cover these exact expectations for tasks with `FinalUsageLogOnly=true`:

```text
submit accepted           -> 0 user logs, 0 usage-stat increments
success without delta     -> 1 consume log with full final quota
success with refund delta -> 1 consume log, 0 refund logs
success with charge delta -> 1 consume log, 0 intermediate consume logs
failure/expired           -> 1 error log, 0 refund logs
stale second poll         -> no second log or statistic increment
legacy Grok in-flight     -> existing behavior remains unchanged
```

- [ ] **Step 2: Run focused tests and verify RED**

```bash
go test ./service -run 'Grok.*FinalUsage|Grok.*Refund|Grok.*Stale|TaskConsumption' -count=1
```

Expected: current submit consume/refund logs make the new assertions fail.

- [ ] **Step 3: Generalize the terminal-only log policy**

Introduce one predicate that preserves StarAI behavior and gates only new Grok tasks:

```go
func taskUsesFinalUsageLog(task *model.Task) bool {
    if isStarAITask(task) {
        return true
    }
    return isMoliiGrokTask(task) && task.PrivateData.BillingContext != nil &&
        task.PrivateData.BillingContext.FinalUsageLogOnly
}
```

At submit, skip `LogTaskConsumption` for Molii Grok. In `RefundTaskQuota` and `RecalculateTaskQuota`, suppress user-visible delta/refund logs and statistics when the predicate is true, while still adjusting wallet/subscription and token balances. Rename/generalize the existing StarAI terminal logging functions so the terminal CAS winner records one success/error log for both supported policies.

- [ ] **Step 4: Make settlement success explicit**

Change terminal settlement helpers to return a success boolean. If funding adjustment fails, do not emit a successful consume log. Keep task quota markers intact for reconciliation. A successful terminal log must use settled `task.Quota`, and statistics must be updated exactly once from that value.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 6: Commit terminal-only logging**

```bash
git add controller/relay.go service/task_billing.go service/task_polling.go service/task_billing_test.go service/task_polling_molii_grok_test.go
git commit -m "feat: defer Grok video logs until completion"
```

### Task 3: Finalize actual Grok video prices and formula terms

**Files:**
- Modify: `relay/channel/task/moliigrok/adaptor.go`
- Modify: `relay/channel/task/moliigrok/adaptor_test.go`
- Create: `service/grok_video_billing.go`
- Create: `service/grok_video_billing_test.go`
- Modify: `service/task_billing.go`

- [ ] **Step 1: Write failing formula tests for all operations**

Assert these V1 calculations:

```text
text_to_video:  output price × actual seconds
image_to_video: output price × actual seconds + image input price × 1
video_edit:      output price × actual seconds + video input price × billed seconds
final cost:      settled quota ÷ QuotaPerUnit
```

Include `grok-imagine-video`, `grok-imagine-video-1.5`, 480p/720p/1080p where supported, group ratios, zero prices, and a video edit completion with 6 seconds and provider total cost ¥0.36 resolving to 480p.

- [ ] **Step 2: Run focused tests and verify RED**

```bash
go test ./relay/channel/task/moliigrok ./service -run 'GrokVideo.*Billing|VideoEditCompletion' -count=1
```

Expected: missing final snapshot fields/content cause failures.

- [ ] **Step 3: Update the snapshot during completion adjustment**

Use the submitted price snapshot, not current admin configuration. For generation, use returned duration when present and requested duration otherwise; actual resolution is the requested validated tier. For edits, retain the existing provider-total-rate tier inference, then store actual duration, actual resolution, output unit price, video input billed seconds, and both subtotals.

- [ ] **Step 4: Build the safe final log contract**

Create a focused backend formatter that copies only the public V1 snapshot into `other.grok_video_billing`, sets `GroupRatio`, derives `FinalCost` from settled quota, and returns readable fallback content such as:

```text
Grok 视频编辑, 模型 grok-imagine-video, 实际 480p · 6 秒, 计费 (¥0.050000 × 6 + ¥0.010000 × 6) × 1.0000 = ¥0.360000
```

The formatter must never include media URLs, file IDs, upstream IDs, result URLs, keys, or provider response bodies.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 6: Commit actual billing details**

```bash
git add relay/channel/task/moliigrok/adaptor.go relay/channel/task/moliigrok/adaptor_test.go service/grok_video_billing.go service/grok_video_billing_test.go service/task_billing.go
git commit -m "feat: record detailed Grok video billing"
```

### Task 4: Parse and format Grok video billing in the frontend

**Files:**
- Modify: `web/src/features/usage-logs/types.ts`
- Create: `web/src/features/usage-logs/lib/grok-video-billing.ts`
- Create: `web/src/features/usage-logs/lib/__tests__/grok-video-billing.test.ts`
- Modify: `web/src/features/usage-logs/lib/index.ts`

- [ ] **Step 1: Write failing parser/formula tests**

Test complete V1 parsing, malformed/negative/non-finite rejection, model mismatch, historical fallback, and exact formulas for text/image/video input. The parser must accept explicit zero prices.

- [ ] **Step 2: Run tests and verify RED**

```bash
cd web && pnpm exec tsx --test src/features/usage-logs/lib/__tests__/grok-video-billing.test.ts
```

Expected: module/type does not exist.

- [ ] **Step 3: Implement the strict V1 parser and helpers**

Export:

```ts
parseGrokVideoBilling(other)
getGrokVideoBillingState(log)
isGrokVideoModel(model)
formatGrokVideoFormula(billing)
formatGrokVideoCny(value)
getGrokVideoListSummary(log)
```

Validate every required field and operation/input enum. Never synthesize missing V1 values; use the historical state instead.

- [ ] **Step 4: Run tests and verify GREEN**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 5: Commit frontend contract parsing**

```bash
git add web/src/features/usage-logs/types.ts web/src/features/usage-logs/lib/grok-video-billing.ts web/src/features/usage-logs/lib/__tests__/grok-video-billing.test.ts web/src/features/usage-logs/lib/index.ts
git commit -m "feat: parse Grok video billing logs"
```

### Task 5: Render the Grok video billing card and list summary

**Files:**
- Create: `web/src/features/usage-logs/components/dialogs/grok-video-billing-card.tsx`
- Modify: `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`
- Modify: `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- Create: `web/src/features/usage-logs/components/__tests__/grok-video-log-display.test.tsx`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`

- [ ] **Step 1: Write failing render tests**

Render one current video-edit log and one historical Grok video log. Assert visible model, operation, input type, requested/estimated/actual parameters, unit prices, subtotals, group ratio, formula, and final charge. Assert that generic anchor model price and Token breakdown are absent for current Grok video logs.

- [ ] **Step 2: Run tests and verify RED**

```bash
cd web && pnpm exec tsx --test src/features/usage-logs/components/__tests__/grok-video-log-display.test.tsx
```

Expected: component does not exist.

- [ ] **Step 3: Implement the card and detail routing**

Mirror the compact visual hierarchy of `GrokImageBillingCard`, using a video icon and responsive metric grids. In `details-dialog.tsx`, render the card for both current and historical Grok video models and suppress `TokenBreakdown`/`BillingBreakdown` for those logs. In common columns, use the structured summary when available.

- [ ] **Step 4: Add complete English and Chinese labels**

Add labels for Grok Video Billing, Text/Image/Video to Video, Requested/Estimated/Actual Duration, Requested/Estimated/Actual Resolution, Input Images, Video Input Billed Seconds, all three unit-price/subtotal labels, Billing Formula, Final Charge, and historical fallback.

- [ ] **Step 5: Run render, type, and lint checks**

```bash
cd web
pnpm exec tsx --test src/features/usage-logs/components/__tests__/grok-video-log-display.test.tsx
pnpm typecheck
pnpm exec oxlint src/features/usage-logs src/i18n/locales
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit the UI**

```bash
git add web/src/features/usage-logs/components web/src/features/usage-logs/components/columns/common-logs-columns.tsx web/src/i18n/locales/en.json web/src/i18n/locales/zh.json
git commit -m "feat: display Grok video billing details"
```

### Task 6: Integration, regression, and live verification

**Files:**
- Modify: `.ccg/tasks/finalize-grok-async-usage-logs/review.md`
- Modify: `.ccg/tasks/finalize-grok-async-usage-logs/task.json`

- [ ] **Step 1: Run complete backend verification**

```bash
gofmt -w controller/relay.go model/task.go relay/channel/task/moliigrok/adaptor.go service/task_billing.go service/task_polling.go service/grok_video_billing.go
go test ./...
VERIFY_BIN="$(mktemp -d)/new-api"
go build -o "$VERIFY_BIN" .
git diff --check
```

Expected: all exit 0.

- [ ] **Step 2: Run complete frontend verification**

```bash
cd web
pnpm typecheck
pnpm test
pnpm build
```

Expected: all exit 0.

- [ ] **Step 3: Security and scope review**

Search changed files and fixtures for real `sk-` credentials, upstream task IDs, result URLs, input media URLs, and provider response bodies. Verify only fixture domains and placeholder secrets exist. Confirm ordinary channels and legacy Grok in-flight tasks retain their previous paths.

- [ ] **Step 4: Restart and perform a new live Grok task**

Build and replace the launchd binary, wait for `/api/status` 200, submit one authorized low-cost Grok async request, poll to success, then query logs by public task ID. Verify exactly one type-2 consumption log, zero type-6 refund logs, a complete `grok_video_billing` V1 snapshot, and quota/formula equality. Do not persist a new test credential.

- [ ] **Step 5: Archive the CCG task and commit**

Write the review evidence, mark the task completed, move it to `.ccg/tasks/archive/2026-08/`, and commit without pushing.

```bash
git add .ccg/tasks docs/superpowers/plans/2026-08-06-grok-video-final-usage-logs.md
git commit -m "chore: archive Grok video usage log task"
```
