# Molii AIGC Channel Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Molii asynchronous billing recoverable and idempotent, enforce the single Seedance channel invariant, align mapped-model capability and pricing, remove Grok resolution guessing, and reject unsupported Grok `file_id` inputs.

**Architecture:** Terminal task state and terminal billing state are persisted separately. A main-database outbox job owns exactly-once wallet/subscription/token adjustment and a system-task worker retries recoverable failures. Channel/model/input validation happens before pre-consume; public errors are stable and provider-neutral.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL, testify, existing system-task scheduler.

## Global Constraints

- Preserve all existing uncommitted generation-record billing changes.
- Do not call a real upstream or issue a paid request in tests.
- Use `common` JSON wrappers in root-module business code.
- Use `common.Quota*Checked` helpers for quota conversions; never add a bare float-to-int quota cast.
- Keep SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6 compatible.
- New public error codes must not contain upstream provider names.
- Implement tasks in numeric order; do not begin a later task before the earlier task passes its focused tests.

---

### Task 1: Persist terminal billing jobs atomically

**Files:**
- Create: `model/task_billing_job.go`
- Create: `model/task_billing_job_test.go`
- Modify: `model/main.go`
- Modify: `model/task.go`

**Interfaces:**
- Produces: `TaskBillingJob`, `FinalizeTaskAndEnqueueBilling`, `ClaimTaskBillingJobs`, `RescheduleTaskBillingJob`, `RequireTaskBillingReview`.
- Consumes: existing `Task.UpdateWithStatus` semantics and main `model.DB`.

- [ ] **Step 1: Write failing model tests**

Cover atomic terminal CAS + job creation, CAS loser creating no job, unique task/idempotency keys, lease expiry recovery, pending ordering, and old task rows remaining untouched.

```go
func TestFinalizeTaskAndEnqueueBillingIsAtomic(t *testing.T) {
    task := createInProgressTask(t)
    target := 0
    job := &TaskBillingJob{TaskID: task.ID, IdempotencyKey: "async-task:" + task.TaskID + ":terminal-v1", Operation: TaskBillingOperationRefund, FromQuota: task.Quota, TargetQuota: &target}
    won, err := FinalizeTaskAndEnqueueBilling(task, TaskStatusInProgress, job)
    require.NoError(t, err)
    require.True(t, won)
    require.Equal(t, TaskBillingJobStatusPending, reloadBillingJob(t, task.ID).Status)
}
```

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `go test ./model -run 'Test(FinalizeTaskAndEnqueueBilling|ClaimTaskBillingJobs)' -count=1`

Expected: FAIL because the billing job model and functions do not exist.

- [ ] **Step 3: Implement the billing job model and migration registration**

Use a unique `TaskID`, unique `IdempotencyKey`, indexed `Status/NextAttemptAt`, lease fields, bounded sanitized `LastError`, and nullable `TargetQuota`. Add `&TaskBillingJob{}` to both normal and fast migration lists. Use `DB.Transaction` so task CAS and job insert commit together.

```go
const (
    TaskBillingJobStatusPending        = "pending"
    TaskBillingJobStatusProcessing     = "processing"
    TaskBillingJobStatusSucceeded      = "succeeded"
    TaskBillingJobStatusReviewRequired = "review_required"
)
```

- [ ] **Step 4: Run model tests**

Run: `go test ./model -run 'TaskBillingJob|FinalizeTaskAndEnqueueBilling|ClaimTaskBillingJobs' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

```bash
git add model/task_billing_job.go model/task_billing_job_test.go model/main.go model/task.go
git commit -m "feat: persist async task billing jobs"
```

### Task 2: Apply billing deltas transactionally and idempotently

**Files:**
- Create: `model/task_billing_tx.go`
- Create: `model/task_billing_tx_test.go`
- Modify: `model/subscription.go`
- Modify: `model/token.go`
- Modify: `model/user.go`

**Interfaces:**
- Produces: `ApplyTaskBillingDeltaTx(tx *gorm.DB, task *Task, targetQuota int) error` and cache synchronization after commit.
- Consumes: Task private billing source, subscription ID, token ID, current quota.

- [ ] **Step 1: Write failing transaction tests**

Test wallet settle/refund, subscription settle/refund, token quota, injected transaction rollback, saturation-safe statistics, and a second application becoming a no-op after the job succeeds.

- [ ] **Step 2: Verify tests fail**

Run: `go test ./model -run 'TaskBillingDeltaTx' -count=1`

Expected: FAIL because the transactional API is missing.

- [ ] **Step 3: Implement main-database adjustments**

Lock the job and task row, verify the job is processing and `task.quota == job.from_quota`, calculate `delta := target - from`, update wallet or subscription with GORM expressions, update token quota when applicable, write final `task.quota`, and mark the job succeeded in the same transaction. Cache synchronization runs only after commit and never changes database balances.

- [ ] **Step 4: Run focused and existing billing tests**

Run: `go test ./model ./service -run 'TaskBilling|RefundTaskQuota|SettleTaskBilling' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```bash
git add model/task_billing_tx.go model/task_billing_tx_test.go model/subscription.go model/token.go model/user.go
git commit -m "feat: apply async billing deltas transactionally"
```

### Task 3: Add the recoverable billing reconciliation worker

**Files:**
- Create: `service/task_billing_reconcile.go`
- Create: `service/task_billing_reconcile_test.go`
- Modify: `model/system_task.go`
- Modify: `controller/system_task_handlers.go`
- Modify: `controller/system_task_handlers_test.go`

**Interfaces:**
- Produces: `ResolveTerminalBillingIntent`, `RunTaskBillingReconciliationOnce`, `ApplyTaskBillingJob`.
- Consumes: Task billing jobs from Tasks 1–2 and existing system-task scheduling.

- [ ] **Step 1: Write failing worker tests**

Test refund, settle, target zero, recoverable backoff sequence `5s/30s/2m/10m/1h`, ten-attempt review transition, expired lease, worker concurrency, and scheduling even when upstream task polling is disabled.

- [ ] **Step 2: Verify tests fail**

Run: `go test ./service ./controller -run 'TaskBillingReconcil|AsyncTaskBilling' -count=1`

Expected: FAIL because the worker and system-task type are missing.

- [ ] **Step 3: Implement reconciliation**

The worker claims jobs using a stable runner ID, executes each in a main-database transaction, reschedules recoverable errors, and marks indeterminate pricing `review_required`. Do not retry or reapply a succeeded job. Emit a deterministic billing event key `taskbill_<job-id>`; a log failure must not roll back money or reapply the job.

- [ ] **Step 4: Register the system task**

Add `async_task_billing_reconcile`, a 10–15 second cadence, an enabled predicate based on runnable jobs, and independence from `constant.UpdateTask`.

- [ ] **Step 5: Run worker tests**

Run: `go test ./service ./controller -run 'TaskBillingReconcil|AsyncTaskBilling' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit Task 3**

```bash
git add service/task_billing_reconcile.go service/task_billing_reconcile_test.go model/system_task.go controller/system_task_handlers.go controller/system_task_handlers_test.go
git commit -m "feat: reconcile async task billing jobs"
```

### Task 4: Route every asynchronous terminal path through the outbox

**Files:**
- Modify: `service/task_polling.go`
- Modify: `service/task_polling_test.go`
- Modify: `service/task_polling_molii_grok_test.go`
- Modify: `service/task_billing.go`
- Modify: `service/task_billing_test.go`
- Modify: `controller/task.go`

**Interfaces:**
- Consumes: Tasks 1–3 billing APIs.
- Produces: terminal task persistence that always creates a settle/refund/review job.

- [ ] **Step 1: Add failing regression tests**

Cover success, upstream failure, timeout, missing upstream task ID, channel cache failure, duplicate poll result, process interruption after terminal persistence, `refund_pending` user state, and exactly one final consume log.

- [ ] **Step 2: Verify current behavior fails the regressions**

Run: `go test ./service ./controller -run 'Polling.*Billing|NullUpstream|ChannelCache|RefundPending' -count=1`

- [ ] **Step 3: Replace direct terminal billing calls**

Compute immutable intent, then call `FinalizeTaskAndEnqueueBilling`. Replace bulk FAILURE updates with per-task CAS + refund job. Timeout handling follows the same path. Existing legacy-cutoff behavior remains explicit and cannot create a new refund for old tasks.

- [ ] **Step 4: Expose safe user state**

Return `refund_pending` when the business task failed and its refund job is pending/processing; return the normal terminal state once succeeded. Do not expose attempts, internal error, runner, lease, or upstream identifiers.

- [ ] **Step 5: Run task suites**

Run: `go test ./service ./controller -run 'Task|Polling|Billing' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit Task 4**

```bash
git add service/task_polling.go service/task_polling_test.go service/task_polling_molii_grok_test.go service/task_billing.go service/task_billing_test.go controller/task.go
git commit -m "fix: make async terminal billing recoverable"
```

### Task 5: Enforce the single enabled Seedance channel invariant

**Files:**
- Modify: `model/channel.go`
- Modify: `controller/channel.go`
- Modify: `controller/channel_test_starai_test.go`
- Modify: `controller/starai_asset.go`
- Modify: `controller/starai_asset_test.go`
- Modify: `relay/relay_task.go`
- Modify: `controller/relay_starai_test.go`

**Interfaces:**
- Produces: `GetSingleEnabledChannelByType(channelType int) (*Channel, error)` and pre-billing runtime guard.

- [ ] **Step 1: Write 0/1/2-channel tests**

Test add, update type, single/batch enable, disable old then enable new, asset creation with 0/2 channels, and task submission failing before pre-consume when selected channel is not the unique enabled channel.

- [ ] **Step 2: Verify failures**

Run: `go test ./model ./controller ./relay -run 'Single.*StarAI|StarAI.*Channel' -count=1`

- [ ] **Step 3: Implement management and runtime guards**

Query at most two enabled rows, reject zero/multiple with provider-neutral configuration errors, validate channel writes before persistence, use the unique getter for assets, and guard task submission before pricing/pre-consume. Startup only logs an error so an operator can repair configuration.

- [ ] **Step 4: Run focused tests and commit**

Run: `go test ./model ./controller ./relay -run 'StarAI|SingleEnabledChannel' -count=1`

```bash
git add model/channel.go controller/channel.go controller/channel_test_starai_test.go controller/starai_asset.go controller/starai_asset_test.go relay/relay_task.go controller/relay_starai_test.go
git commit -m "fix: enforce a single Seedance channel"
```

### Task 6: Restrict model mappings to compatible capability and price families

**Files:**
- Create: `constant/imagine_model_family.go`
- Create: `constant/imagine_model_family_test.go`
- Modify: `relay/helper/model_mapped.go`
- Modify: `relay/helper/model_mapped_test.go`
- Modify: `controller/channel.go`
- Modify: `controller/channel_test_starai_test.go`
- Modify: `controller/channel_test_molii_grok_test.go`
- Modify: `relay/channel/task/starai/adaptor.go`
- Modify: `relay/channel/moliigrok/adaptor.go`
- Modify: `service/grok_video_billing.go`
- Modify: related billing/adaptor tests.

**Interfaces:**
- Produces: `ImagineModelFamily`, `ValidateImagineModelMapping`, requested/billed model snapshots.

- [ ] **Step 1: Write mapping matrix tests**

Reject Seedance standard→fast, Grok basic→quality, image→video, legacy→1.5, and both directions between 1.5 and 1.5-preview. Every Imagine model ID may map only to itself; prove unrelated channel behavior is unchanged.

- [ ] **Step 2: Verify failures**

Run: `go test ./constant ./relay/helper ./controller -run 'ImagineModel|ModelMapping' -count=1`

- [ ] **Step 3: Centralize family validation after mapping**

Call validation after `UpstreamModelName` is final and before billing. Validate channel mapping JSON on save. Snapshot both requested and billed model; price lookup uses billed model while user-facing logs retain requested model.

- [ ] **Step 4: Run mapping and billing suites**

Run: `go test ./constant ./relay/helper ./controller ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./relay/channel/task/starai ./service -run 'Mapping|Mapped|BillingSnapshot|Price' -count=1`

- [ ] **Step 5: Commit Task 6**

```bash
git add constant/imagine_model_family.go constant/imagine_model_family_test.go relay/helper/model_mapped.go relay/helper/model_mapped_test.go controller/channel.go controller/channel_test_starai_test.go controller/channel_test_molii_grok_test.go relay/channel/task/starai relay/channel/moliigrok service/grok_video_billing.go service/grok_video_billing_test.go
git commit -m "fix: align mapped Imagine models and billing"
```

### Task 7: Stop inferring Grok edit resolution from provider cost

**Files:**
- Modify: `relay/channel/task/moliigrok/dto.go`
- Modify: `relay/channel/task/moliigrok/adaptor.go`
- Modify: `relay/channel/task/moliigrok/adaptor_test.go`
- Modify: `relay/common/relay_info.go`
- Modify: `model/task.go`
- Modify: `service/grok_video_billing.go`
- Modify: `service/grok_video_billing_test.go`
- Modify: `service/task_polling_molii_grok_test.go`

**Interfaces:**
- Produces: explicit `ActualResolution`, `ResolutionSource`, and review-required intent when resolution is unknown.

- [ ] **Step 1: Replace inference expectations with explicit-source tests**

Test 480p/720p response parsing, provider-cost changes not affecting resolution, missing/unknown resolution producing review-required, and valid resolution settling normally.

- [ ] **Step 2: Verify current inference fails the tests**

Run: `go test ./relay/channel/task/moliigrok ./service -run 'Resolution|ProviderCost|ReviewRequired' -count=1`

- [ ] **Step 3: Implement explicit resolution handling**

Parse `video.resolution`, normalize against supported values, persist the source, delete cost inference, and return an indeterminate billing intent when no trusted resolution exists. Deliver successful video output even when billing needs review; keep pre-consumed quota unchanged.

- [ ] **Step 4: Run and commit**

Run: `go test ./relay/channel/task/moliigrok ./service -run 'Grok|Resolution|Billing' -count=1`

```bash
git add relay/channel/task/moliigrok/dto.go relay/channel/task/moliigrok/adaptor.go relay/channel/task/moliigrok/adaptor_test.go relay/common/relay_info.go model/task.go service/grok_video_billing.go service/grok_video_billing_test.go service/task_polling_molii_grok_test.go
git commit -m "fix: use explicit Grok video resolution"
```

### Task 8: Reject unsupported Grok file IDs before billing

**Files:**
- Modify: `relay/channel/moliigrok/adaptor.go`
- Modify: `relay/channel/moliigrok/adaptor_test.go`
- Modify: `relay/channel/task/moliigrok/adaptor.go`
- Modify: `relay/channel/task/moliigrok/adaptor_test.go`
- Modify: `tools/molii-aigc-demo/internal/upstream/requests.go`
- Modify: `tools/molii-aigc-demo/internal/upstream/requests_test.go`

**Interfaces:**
- Produces: HTTP 400 code `file_id_not_supported` for Grok, while URL/Data URL inputs continue to work.

- [ ] **Step 1: Write failing image/video file ID tests**

Assert single/multiple image `file_id`, video `file_id`, and mixed media fail before transport and pre-consume; URL string/object and Data URL remain accepted.

- [ ] **Step 2: Verify failures**

Run: `go test ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./tools/molii-aigc-demo/internal/upstream -run 'FileID|Media' -count=1`

- [ ] **Step 3: Implement provider-scoped rejection**

Keep decode fields so the request is not silently ignored, return a sentinel error mapped to provider-neutral `file_id_not_supported`, remove file ID branches from Grok request builders and Demo previews, and do not change shared DTO behavior for other channels.

- [ ] **Step 4: Run and commit**

Run: `go test ./relay/channel/moliigrok ./relay/channel/task/moliigrok -count=1 && (cd tools/molii-aigc-demo && GOWORK=off go test ./...)`

```bash
git add relay/channel/moliigrok relay/channel/task/moliigrok tools/molii-aigc-demo/internal/upstream
git commit -m "fix: reject unsupported Grok file IDs"
```

### Task 9: Full backend verification

**Files:**
- Create: `.ccg/tasks/build-molii-docusaurus-docs/review-backend.md`

- [ ] **Step 1: Format and inspect the scoped diff**

Run: `gofmt -w <all changed Go files>` then `git diff --check`.

- [ ] **Step 2: Run focused race tests**

Run: `go test -race ./model ./service ./controller ./relay/helper ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./relay/channel/task/starai`

- [ ] **Step 3: Run root verification**

Run: `go test ./...` and `go vet ./...`.

- [ ] **Step 4: Verify relaykit independence if affected**

Run: `(cd relaykit && GOWORK=off go build ./...)`.

- [ ] **Step 5: Record review**

Write Critical/Warning/Info findings, exact commands, and remaining manual production migration checks to `.ccg/tasks/build-molii-docusaurus-docs/review-backend.md`.
