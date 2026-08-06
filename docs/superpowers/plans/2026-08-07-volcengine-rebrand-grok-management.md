# Volcengine Rebrand and Grok Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename every user-visible Molii AIGC label to Molii Volcengine Imagine API and complete Molii Grok Imagine API reachability, balance, and model discovery without paid upstream calls.

**Architecture:** Keep channel type IDs, internal Go identifiers, database records, and model owner values stable. Add an environment-configured New API management client for Grok balance, use the saved channel Key for `/v1/models`, and share the existing TCP-only reachability pattern. Split backend and frontend ownership so both can be implemented in parallel without overlapping files.

**Tech Stack:** Go 1.x, Gin, GORM, `net.Dialer`, React/TypeScript, Bun, Vitest-compatible Node tests, Docker Compose.

---

### Task 1: Grok New API management configuration and balance

**Files:**
- Modify: `constant/env.go`
- Modify: `common/init.go`
- Create: `controller/molii_grok_management.go`
- Create: `controller/molii_grok_management_test.go`
- Modify: `controller/channel-billing.go`
- Modify: `.env.example`
- Modify: `deploy/.env.example`
- Modify: `deploy/docker-compose.yml`

- [ ] **Step 1: Write failing management-client tests**

Create tests backed by `httptest.Server` that set temporary constants, require `Authorization: Bearer <PAT>` and `User-id: 2205`, return `/api/status` with `quota_per_unit: 500000`, return `/api/user/self` with `quota: 49842`, and assert the channel balance becomes `0.099684`. Add non-2xx, malformed JSON, non-positive quota unit, missing token/user ID, and secret-redaction cases.

```go
func TestUpdateMoliiGrokBalanceUsesNewAPIManagementCredential(t *testing.T) {
    // /api/status -> {"success":true,"data":{"quota_per_unit":500000}}
    // /api/user/self -> {"success":true,"data":{"id":2205,"quota":49842}}
    balance, err := updateChannelMoliiGrokBalance(channel)
    require.NoError(t, err)
    assert.InDelta(t, 0.099684, balance, 1e-12)
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./controller -run 'TestUpdateMoliiGrokBalance' -count=1`

Expected: FAIL because `updateChannelMoliiGrokBalance` and management configuration do not exist.

- [ ] **Step 3: Add environment configuration**

Add these runtime values and initialize them without defaults for secrets:

```go
var MoliiGrokNewAPIBaseURL string
var MoliiGrokNewAPIAccessToken string
var MoliiGrokNewAPIUserID int
```

```go
constant.MoliiGrokNewAPIBaseURL = GetEnvOrDefaultString("MOLII_GROK_NEW_API_BASE_URL", "https://api.wxiai.com")
constant.MoliiGrokNewAPIAccessToken = GetEnvOrDefaultString("MOLII_GROK_NEW_API_ACCESS_TOKEN", "")
constant.MoliiGrokNewAPIUserID = GetEnvOrDefault("MOLII_GROK_NEW_API_USER_ID", 0)
```

- [ ] **Step 4: Implement balance lookup**

Implement a focused client that validates HTTPS/HTTP management base URL, builds headers without logging them, reads `/api/status` and `/api/user/self`, validates `success`, quota and `quota_per_unit`, converts `quota / quota_per_unit`, then calls `channel.UpdateBalance`.

```go
headers := http.Header{
    "Authorization": []string{"Bearer " + constant.MoliiGrokNewAPIAccessToken},
    "User-id":       []string{strconv.Itoa(constant.MoliiGrokNewAPIUserID)},
    "Accept":        []string{"application/json"},
}
balance := float64(self.Data.Quota) / status.Data.QuotaPerUnit
```

Add `ChannelTypeMoliiGrokAIGC` to `updateChannelBalance` and route it to this helper.

- [ ] **Step 5: Add safe configuration examples**

Add only placeholders to `.env.example` and `deploy/.env.example`, and pass the three variables through `deploy/docker-compose.yml`. Never write the supplied real PAT.

```dotenv
MOLII_GROK_NEW_API_BASE_URL=https://api.wxiai.com
MOLII_GROK_NEW_API_ACCESS_TOKEN=replace-with-system-access-token
MOLII_GROK_NEW_API_USER_ID=2205
```

- [ ] **Step 6: Verify GREEN**

Run: `gofmt -w constant/env.go common/init.go controller/molii_grok_management.go controller/molii_grok_management_test.go controller/channel-billing.go`

Run: `go test ./controller -run 'TestUpdateMoliiGrokBalance' -count=1`

Expected: PASS, with no secret in test output.

### Task 2: Grok model discovery and TCP reachability

**Files:**
- Modify: `controller/molii_grok_management.go`
- Modify: `controller/molii_grok_management_test.go`
- Modify: `controller/channel_upstream_update.go`
- Modify: `controller/channel-test.go`
- Modify: `controller/channel_test_molii_grok_test.go`
- Modify: `controller/channel_test_starai_test.go`

- [ ] **Step 1: Write failing model discovery tests**

Use `httptest.Server` to require the saved channel Key at root `/v1/models`, return duplicate model IDs, and assert normalized unique IDs. Assert the request never uses the generation `/xai` prefix and never sends the management PAT.

```go
models, err := fetchMoliiGrokNewAPIModels(channel)
require.NoError(t, err)
assert.Equal(t, []string{"grok-imagine-image", "grok-imagine-video"}, models)
```

- [ ] **Step 2: Write failing reachability tests**

Start a local TCP listener and require `testChannel` for type 62 to connect successfully without sending bytes. Close the listener and assert a safe failure. Keep missing-Key and invalid-URL coverage.

```go
result := testChannel(context.Background(), channel, 1, "", "", false)
require.NoError(t, result.localErr)
assert.Equal(t, "可达性测试通过，未发送付费请求", result.successMessage)
```

- [ ] **Step 3: Run tests and verify RED**

Run: `go test ./controller -run 'TestMoliiGrok' -count=1`

Expected: FAIL because type 62 still performs configuration-only validation and generic model fetching uses the wrong prefixed base URL.

- [ ] **Step 4: Implement model discovery**

Special-case type 62 before generic URL construction in `fetchChannelUpstreamModelIDs`. Call the management root `/v1/models`, authorize with `channel.GetNextEnabledKey()`, reuse error sanitization, parse OpenAI model objects, and normalize IDs.

- [ ] **Step 5: Implement shared TCP reachability**

Extract the URL parsing and `net.Dialer` logic into a helper accepting a display name. Use it from both type 61 and type 62. Type 62 first validates Key, then checks the fixed generation URL host; no TLS or HTTP request is allowed.

- [ ] **Step 6: Verify GREEN**

Run: `gofmt -w controller/molii_grok_management.go controller/molii_grok_management_test.go controller/channel_upstream_update.go controller/channel-test.go controller/channel_test_molii_grok_test.go controller/channel_test_starai_test.go`

Run: `go test ./controller -run 'TestMoliiGrok|TestStarAIReachability' -count=1`

Expected: PASS.

### Task 3: Complete user-visible Volcengine rename

**Files:**
- Modify: `constant/channel.go`
- Modify: `controller/channel-billing.go`
- Modify: `controller/channel-test.go`
- Modify: `controller/starai_asset.go`
- Modify: `controller/video_proxy.go`
- Modify: `relay/channel/task/starai/adaptor.go`
- Modify: `service/starai_result_security.go`
- Modify: `service/task_polling.go`
- Modify: `controller/channel_billing_test.go`
- Modify: `controller/starai_asset_test.go`
- Modify: `controller/video_proxy_starai_test.go`
- Modify: `relay/channel/task/starai/adaptor_test.go`
- Modify: `service/starai_result_security_test.go`
- Modify: `Dockerfile`

- [ ] **Step 1: Update failing brand expectations first**

Change tests to require `Molii Volcengine Imagine API` in the channel type map, sanitized upstream errors, asset errors, video proxy errors and task messages. Keep `molii-aigc`, `ChannelTypeStarAI`, package names, type ID 61 and historical docs unchanged.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./controller ./relay/channel/task/starai ./service -count=1`

Expected: FAIL on old user-visible text.

- [ ] **Step 3: Replace active user-visible backend strings**

Replace every active-source `Molii AIGC` label/message with `Molii Volcengine Imagine API`. Do not rename identifiers or model owner values. Update the Docker image description only.

- [ ] **Step 4: Verify backend brand cleanup**

Run: `rg -n 'Molii AIGC' constant controller relay/channel/task/starai service Dockerfile --glob '*.{go}'`

Expected: no active user-visible occurrence; allowed internal `molii-aigc` owner values remain.

Run: `go test ./controller ./relay/channel/task/starai ./service -count=1`

Expected: PASS.

### Task 4: Frontend labels, pricing, assets and test action

**Files:**
- Modify: `web/src/features/channels/constants.ts`
- Modify: `web/src/features/channels/lib/channel-utils.ts`
- Modify: `web/src/features/channels/lib/channel-type-config.ts`
- Modify: `web/src/features/channels/lib/__tests__/starai-channel.test.ts`
- Modify: `web/src/features/channels/lib/__tests__/molii-grok-channel.test.ts`
- Modify: `web/src/features/system-settings/billing/section-registry.tsx`
- Modify: `web/src/features/system-settings/billing/starai-video-pricing-section.tsx`
- Modify: `web/src/features/system-settings/operations/object-storage-section.tsx`
- Modify: `web/src/features/system-settings/billing/__tests__/molii-aigc-pricing-tabs.test.tsx`
- Modify: `web/src/features/system-settings/billing/__tests__/pricing-unit-input.test.tsx`
- Modify: `web/src/features/temporary-assets/components/create-asset-card.tsx`
- Modify: `web/src/features/usage-logs/constants.ts`
- Modify: `web/src/features/usage-logs/lib/__tests__/task-video-preview.test.ts`
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/*.json`

- [ ] **Step 1: Update frontend tests to the new contract**

Require channel 61 to display `Molii Volcengine Imagine API`, channel 62 to return `{direct: true, label: 'Reachability Test'}`, and its hint to state that only TCP reachability is checked without paid requests.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `cd web && bun test src/features/channels/lib/__tests__/starai-channel.test.ts src/features/channels/lib/__tests__/molii-grok-channel.test.ts`

Expected: FAIL on the old labels and configuration-only action.

- [ ] **Step 3: Update all active frontend strings and translations**

Replace channel, pricing, asset upload, usage-log and status strings with `Molii Volcengine Imagine API`. Change Grok `Configuration Check` to `Reachability Test` and update its help text. Use `bun run i18n:sync` if translation key synchronization requires it.

- [ ] **Step 4: Verify frontend brand cleanup and tests**

Run: `rg -n 'Molii AIGC' web/src --glob '*.{ts,tsx,json}'`

Expected: no active old user-visible labels.

Run: `cd web && bun test src/features/channels/lib/__tests__/starai-channel.test.ts src/features/channels/lib/__tests__/molii-grok-channel.test.ts`

Expected: PASS.

### Task 5: Integration verification and local deployment

**Files:**
- Modify: `.ccg/tasks/rename-molii-aigc-channel/review.md`

- [ ] **Step 1: Run backend verification**

Run: `go test ./...`

Run: `go vet ./...`

Run: `gofmt -d` for every changed Go file and `git diff --check`.

Expected: all pass with no formatting output.

- [ ] **Step 2: Run frontend verification**

Run: `cd web && bun run typecheck`

Run: `cd web && bun run lint`

Run: `cd web && bun run format:check`

Run: `cd web && bun run build`

Expected: all pass.

- [ ] **Step 3: Verify secrets are absent**

Run focused searches for the supplied PAT and ensure it appears nowhere in tracked or untracked workspace files. Inspect `git diff` for Authorization values and `.env` changes.

Expected: only placeholders are present.

- [ ] **Step 4: Build and restart the local service**

Build a temporary binary, atomically replace `/Users/naf/Library/Application Support/Molii/new-api/new-api`, and kick `com.molii.new-api`. Do not persist the real management PAT unless the user explicitly asks to configure local secrets.

- [ ] **Step 5: Browser verification**

Verify channel 61 labels, pricing title, asset prompt, type 62 reachability action, and successful model fetching. Balance should show a clear missing-management-config error until the three environment variables are configured locally.

- [ ] **Step 6: Self-review, commit and archive**

Review the complete diff for compatibility and secret leakage. Commit implementation by coherent scope, write `.ccg/tasks/rename-molii-aigc-channel/review.md`, mark the task completed, archive it under `.ccg/tasks/archive/2026-08`, and do not push.
