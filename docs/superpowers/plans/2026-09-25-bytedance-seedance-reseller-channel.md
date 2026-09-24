# ByteDance Seedance Reseller Channel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a native `ByteDance Seedance` channel that lets an independent new-api instance use a Molii Base URL and per-instance API key for the full Seedance and temporary-asset lifecycle without exposing StarAI credentials or copying generated videos.

**Architecture:** Register a new channel/platform and extract the provider-neutral Seedance wire contract from the direct StarAI adaptor. The reseller adaptor talks only to Molii public APIs, stores Molii public task IDs as private upstream IDs, keeps its own asset/user bindings and billing ledger, and proxies video content through Molii with the channel key. Existing StarAI storage, billing and diagnostics remain isolated behind the old channel type.

**Tech Stack:** Go 1.25, Gin, GORM/PostgreSQL, Redis, React 19, TypeScript, TanStack Query, Vitest, existing new-api async task and billing systems.

**Spec:** `docs/superpowers/specs/2026-09-25-bytedance-seedance-reseller-channel-design.md`

## Global Constraints

- The admin-facing channel name is exactly `ByteDance Seedance`.
- First release covers only Seedance video and `/v1/assets`; it does not add text, Claude, Gemini, image generation or Grok routing.
- Every deployed reseller instance uses its own Molii account and API key.
- Model sync imports capabilities only; it never imports Molii price, ratio, currency or channel data.
- Upstream asset IDs are passed through unchanged, while every gateway enforces its own local user binding.
- Reseller video content is streamed from Molii with channel authentication and is not copied into reseller storage.
- Existing `Molii Volcengine Imagine API` channels, tasks, assets, COS storage and billing behavior must remain unchanged.
- Do not introduce a proxy-hop limit or proprietary hop-count header.
- PostgreSQL and Redis are the supported runtime stores; SQLite is not an acceptance target.
- Never log or return API keys, raw private StarAI responses, private result URLs or StarAI task IDs from a reseller instance.
- The external CCG dual-model wrapper is currently unavailable. If it becomes available before implementation/review, run both antigravity and Claude as required; otherwise record the missing tool truthfully.

## Review Focus

- A failed or malformed `/v1/models` refresh must retain the last known channel models and must not import non-Seedance IDs.
- An asset ID copied between local users or reseller instances must fail before an upstream media request is issued.
- Nested Molii task responses must not leak StarAI IDs, keys, raw task data or private result URLs into reseller storage or responses.
- GET/HEAD/Range video content must authenticate to Molii without depending on an expired signed URL and without forwarding the end-user Authorization header.
- A successful task without reliable actual usage must enter manual review rather than settle at zero or silently keep an estimated charge.

---

### Task 1: Register the channel and extract a provider-neutral Seedance protocol core

**Files:**
- Create: `relay/channel/task/seedanceprotocol/models.go`
- Create: `relay/channel/task/seedanceprotocol/payload.go`
- Create: `relay/channel/task/seedanceprotocol/response.go`
- Create: `relay/channel/task/seedanceprotocol/protocol_test.go`
- Modify: `constant/channel.go`
- Modify: `relay/channel/task/starai/adaptor.go`
- Modify: `relay/channel/task/starai/constants.go`
- Modify: `relay/channel/task/starai/registration_test.go`
- Test: `relay/channel/task/starai/adaptor_test.go`

**Interfaces:**
- Produces: `seedanceprotocol.SupportedModels() []string`
- Produces: `seedanceprotocol.FilterSupportedModels([]string) []string`
- Produces: `seedanceprotocol.Payload` and `seedanceprotocol.ResponseEnvelope`
- Produces: `seedanceprotocol.DecodeResponse([]byte, any) error`
- Produces: `seedanceprotocol.ValidatePayload(*Payload) error`
- Consumes: existing StarAI request/response fixtures and validation behavior.

- [ ] **Step 1: Write failing channel registration and shared protocol tests**

```go
func TestByteDanceSeedanceChannelRegistration(t *testing.T) {
    assert.Equal(t, "ByteDance Seedance", constant.GetChannelTypeName(constant.ChannelTypeByteDanceSeedance))
    assert.Empty(t, constant.GetChannelBaseURL(constant.ChannelTypeByteDanceSeedance))
    assert.Greater(t, constant.ChannelTypeDummy, constant.ChannelTypeByteDanceSeedance)
}

func TestFilterSupportedModelsRejectsUnrelatedAndUnknownModels(t *testing.T) {
    got := seedanceprotocol.FilterSupportedModels([]string{
        "gpt-5.6-sol",
        "doubao-seedance-2-0-260128",
        "doubao-seedance-unknown",
    })
    assert.Equal(t, []string{"doubao-seedance-2-0-260128"}, got)
}
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `go test ./constant ./relay/channel/task/seedanceprotocol ./relay/channel/task/starai`

Expected: FAIL because the new constant and protocol package do not exist.

- [ ] **Step 3: Add the channel constant without reusing StarAI identity**

```go
const (
    ChannelTypeStarAI              = 61
    ChannelTypeMoliiGrokAIGC       = 62
    ChannelTypeTaskPlugin          = 63
    ChannelTypeByteDanceSeedance   = 64
    ChannelTypeDummy               = 65
)
```

Append an empty default Base URL and add `ChannelTypeByteDanceSeedance: "ByteDance Seedance"` to `ChannelTypeNames`.

- [ ] **Step 4: Extract shared DTO, decoder and validation behavior**

Move only provider-neutral Seedance definitions and logic into `seedanceprotocol`. Keep StarAI branding, direct-provider billing and private diagnostics in `task/starai`.

```go
var modelSet = map[string]struct{}{
    "doubao-seedance-2-0-260128":      {},
    "doubao-seedance-2-0-fast-260128": {},
    "doubao-seedance-2-0-mini-260615": {},
    "doubao-seedance-2-5-260628":      {},
}

func FilterSupportedModels(models []string) []string {
    result := make([]string, 0, len(models))
    seen := make(map[string]struct{}, len(models))
    for _, raw := range models {
        model := strings.TrimSpace(raw)
        if _, supported := modelSet[model]; !supported {
            continue
        }
        if _, duplicate := seen[model]; duplicate {
            continue
        }
        seen[model] = struct{}{}
        result = append(result, model)
    }
    return result
}
```

- [ ] **Step 5: Adapt the direct StarAI adaptor to the shared package**

Keep its public behavior and test fixtures byte-compatible. Do not change StarAI URLs, pricing, error brand, asset resolution, timing capture or COS behavior.

- [ ] **Step 6: Run focused and regression tests**

Run: `go test ./constant ./relay/channel/task/seedanceprotocol ./relay/channel/task/starai`

Expected: PASS, including all existing StarAI tests.

- [ ] **Step 7: Commit the protocol foundation**

```bash
git add constant/channel.go relay/channel/task/seedanceprotocol relay/channel/task/starai
git commit -m "refactor: extract shared seedance protocol core"
```

### Task 2: Implement the native ByteDance Seedance task adaptor

**Files:**
- Create: `relay/channel/task/bytedanceseedance/adaptor.go`
- Create: `relay/channel/task/bytedanceseedance/constants.go`
- Create: `relay/channel/task/bytedanceseedance/adaptor_test.go`
- Create: `relay/channel/task/bytedanceseedance/registration_test.go`
- Modify: `relay/relay_adaptor.go`
- Modify: `controller/model.go`
- Modify: `service/channel_select.go`
- Modify: `common/channel.go` or the existing endpoint registration file that owns channel endpoint types

**Interfaces:**
- Consumes: `seedanceprotocol.Payload`, `ResponseEnvelope`, validation and model list.
- Produces: native task adaptor for platform `strconv.Itoa(ChannelTypeByteDanceSeedance)`.
- Produces: submit upstream ID equal to the Molii public task ID.
- Produces: parsed task facts without raw StarAI diagnostics.

- [ ] **Step 1: Write failing submit, poll and sanitization tests**

```go
func TestSubmitStoresOnlyMoliiPublicTaskID(t *testing.T) {
    adaptor := &TaskAdaptor{}
    ctx, recorder := gin.CreateTestContext(httptest.NewRecorder())
    response := &http.Response{
        StatusCode: http.StatusOK,
        Header:     http.Header{"Content-Type": []string{"application/json"}},
        Body:       io.NopCloser(strings.NewReader(`{"id":"task_molii_public"}`)),
    }
    parsed, taskErr := adaptor.ParseResponse(ctx, response, &relaycommon.RelayInfo{
        PublicTaskID:   "task_reseller_public",
        OriginModelName: "doubao-seedance-2-5-260628",
    })
    require.Nil(t, taskErr)
    require.NotNil(t, parsed)
    assert.Equal(t, "task_molii_public", parsed.UpstreamTaskID)
    assert.NotContains(t, recorder.Body.String(), "task_molii_public")
}

func TestPollDropsNestedStarAIPrivateDiagnostics(t *testing.T) {
    body := []byte(`{"code":"success","data":{"task_id":"task_molii_public","upstream_id":"cgt-private","status":"SUCCESS","data":{"private":"secret"},"usage":{"total_tokens":288625}}}`)
    result, err := adaptor.ParseTaskResult(task, response, body)
    require.NoError(t, err)
    assert.Equal(t, 288625, result.TotalTokens)
    safeData := adaptor.SafePollingData(result)
    assert.NotContains(t, string(safeData), "cgt-private")
    assert.NotContains(t, string(safeData), "secret")
}
```

- [ ] **Step 2: Run the adaptor tests and verify failure**

Run: `go test ./relay/channel/task/bytedanceseedance ./relay`

Expected: FAIL because the adaptor is not registered.

- [ ] **Step 3: Implement submit and poll against Molii public APIs**

```go
func (a *TaskAdaptor) BuildRequestURL(*relaycommon.RelayInfo) (string, error) {
    return strings.TrimRight(a.baseURL, "/") + "/v1/video/generations", nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, task *model.Task, proxy string) (*http.Response, error) {
    endpoint := strings.TrimRight(baseURL, "/") + "/v1/video/generations/" + url.PathEscape(task.GetUpstreamTaskID())
    request, err := http.NewRequest(http.MethodGet, endpoint, nil)
    if err != nil {
        return nil, err
    }
    request.Header.Set("Authorization", "Bearer "+key)
    request.Header.Set("Accept", "application/json")
    client, err := service.GetHttpClientWithProxy(proxy)
    if err != nil {
        return nil, err
    }
    return client.Do(request)
}
```

Parse both the current Molii `TaskResponse.data` envelope and the OpenAI-video form, but project only status, timing, usage, failure, model and media counts into local task facts.

- [ ] **Step 4: Register routing and supported endpoint types**

Add the new type to `getNativeTaskAdaptor`, model lists, the unified native Seedance route selection and OpenAI-video endpoint registration. Do not add it to any StarAI-only storage predicate.

- [ ] **Step 5: Test request validation and four supported models**

Reuse the shared validation fixtures, including Seedance 2.5 limits of 30 images, 10 videos and 10 audio files and existing limits for 2.0/Fast/Mini.

- [ ] **Step 6: Run the task routing tests**

Run: `go test ./relay/channel/task/bytedanceseedance ./relay ./service ./controller -run 'ByteDanceSeedance|UnifiedNativeTask|ChannelSelect'`

Expected: PASS.

- [ ] **Step 7: Commit the native adaptor**

```bash
git add relay/channel/task/bytedanceseedance relay/relay_adaptor.go controller/model.go service/channel_select.go common
git commit -m "feat: add ByteDance Seedance task channel"
```

### Task 3: Add connection testing and authorized-model synchronization

**Files:**
- Modify: `controller/channel-test.go`
- Modify: `controller/channel_upstream_update.go`
- Modify: `controller/channel_upstream_update_test.go`
- Modify: `controller/channel_test_internal_test.go`
- Test: `controller/channel_test_starai_test.go`

**Interfaces:**
- Consumes: standard upstream model fetch infrastructure and `seedanceprotocol.FilterSupportedModels`.
- Produces: test/fetch/update behavior for `ChannelTypeByteDanceSeedance` using `{BaseURL}/v1/models`.
- Preserves: old model set on fetch failure.

- [ ] **Step 1: Write failing model filtering and failure-retention tests**

```go
func TestByteDanceSeedanceFetchModelsFiltersAuthorizationResponse(t *testing.T) {
    server := modelListServer([]string{"doubao-seedance-2-5-260628", "gpt-5.6-sol", "doubao-seedance-unknown"})
    channel := byteDanceSeedanceChannel(server.URL, "instance-key")
    got, err := fetchUpstreamModels(channel)
    require.NoError(t, err)
    assert.Equal(t, []string{"doubao-seedance-2-5-260628"}, got)
}

func TestByteDanceSeedanceFailedRefreshKeepsExistingModels(t *testing.T) {
    channel := byteDanceSeedanceChannel(unreachableURL, "instance-key")
    channel.Models = "doubao-seedance-2-0-260128"
    err := checkAndPersistChannelUpstreamModelUpdates(channel, &settings, true, true)
    require.Error(t, err)
    assert.Equal(t, "doubao-seedance-2-0-260128", reloadChannel(t, channel.Id).Models)
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./controller -run 'ByteDanceSeedance.*(FetchModels|Refresh|ChannelTest)'`

- [ ] **Step 3: Add channel test behavior**

Validate an absolute Base URL, reject a direct self URL, call `/v1/models` with the configured Bearer key, and require at least one supported Seedance model. Do not submit a generation.

- [ ] **Step 4: Integrate with existing manual and scheduled sync**

Make the standard fetch-model endpoints and scheduled detector recognize the type. Filter before calculating add/remove sets so unrelated Molii models never enter the channel.

- [ ] **Step 5: Test revoked and locally disabled models**

Add tests proving an upstream removal is detected, a local explicit disable is not auto-reenabled, and prices/options are not touched by model sync.

- [ ] **Step 6: Run controller tests**

Run: `go test ./controller -run 'Channel.*(Model|Fetch|Update|Test)|ByteDanceSeedance'`

Expected: PASS.

- [ ] **Step 7: Commit model synchronization**

```bash
git add controller/channel-test.go controller/channel_upstream_update.go controller/*test.go
git commit -m "feat: sync authorized Seedance models"
```

### Task 4: Generalize temporary assets for direct and reseller upstreams

**Files:**
- Create: `controller/temporary_asset_upstream.go`
- Create: `controller/temporary_asset_upstream_test.go`
- Modify: `controller/starai_asset.go`
- Modify: `controller/starai_asset_test.go`
- Modify: `service/starai_asset.go`
- Modify: `service/starai_asset_test.go`
- Modify: `model/channel.go` only if an existing channel query cannot select the new type without schema changes

**Interfaces:**
- Produces: `resolveTemporaryAssetChannel(binding *service.StarAIAssetBinding) (*model.Channel, error)`.
- Produces: provider-neutral create/query request helper using the selected channel Base URL and key.
- Extends Redis binding JSON with a backward-compatible provider/channel type field; old missing values mean direct StarAI.

- [ ] **Step 1: Write failing ownership and provider-selection tests**

```go
func TestTemporaryAssetRejectsDifferentLocalUserBeforeUpstream(t *testing.T) {
    binding := saveBinding(t, userA, "asset-upstream")
    upstreamCalls := atomic.Int32{}
    _, err := service.ResolveStarAIAssetURI(ctx, "asset://"+binding.ID, userB, configWithCounter(&upstreamCalls))
    require.ErrorIs(t, err, service.ErrStarAIAssetNotFound)
    assert.Zero(t, upstreamCalls.Load())
}

func TestTemporaryAssetUsesByteDanceSeedanceChannelOnReseller(t *testing.T) {
    seedChannel(t, constant.ChannelTypeByteDanceSeedance, moliiURL, "instance-key")
    channel, err := resolveTemporaryAssetChannel(nil)
    require.NoError(t, err)
    assert.Equal(t, constant.ChannelTypeByteDanceSeedance, channel.Type)
}
```

- [ ] **Step 2: Run asset tests and verify failure**

Run: `go test ./controller ./service -run 'TemporaryAsset|StarAIAsset'`

- [ ] **Step 3: Introduce the provider-neutral upstream resolver**

Preserve old StarAI bindings and routes. New bindings record the chosen provider type and channel ID. On a reseller installation, choose the enabled ByteDance Seedance channel; otherwise preserve direct StarAI fallback.

- [ ] **Step 4: Forward create/query and retain raw asset IDs**

Use the selected channel for `POST /v1/assets` and `GET /v1/assets/{id}`. Save `binding.ID == binding.UpstreamID`. Refresh status and the upstream expiry immediately after creation when the create response does not include expiry.

- [ ] **Step 5: Preserve local authorization on query, delete and generation**

Every operation must load the local binding by `(assetID, userID)` before contacting Molii. DELETE removes only the current local binding and local COS object, matching the existing contract.

- [ ] **Step 6: Test key rotation and cross-instance behavior**

Use two upstream test users/tokens. Prove a new key owned by the same upstream account can verify the asset, while a key belonging to another account receives not-found/forbidden and the local request becomes a safe verification error.

- [ ] **Step 7: Run all asset tests**

Run: `go test ./controller ./service -run 'Asset|COSUpload'`

Expected: PASS, including legacy StarAI asset tests.

- [ ] **Step 8: Commit the asset abstraction**

```bash
git add controller/temporary_asset_upstream.go controller/starai_asset.go controller/*asset*test.go service/starai_asset.go service/starai_asset_test.go
git commit -m "feat: proxy Seedance temporary assets through Molii"
```

### Task 5: Integrate durable polling, sanitized diagnostics and reseller billing

**Files:**
- Modify: `service/task_polling.go`
- Modify: `service/task_polling_test.go`
- Modify: `service/task_billing.go`
- Modify: `service/task_billing_test.go`
- Modify: `controller/relay.go`
- Modify: `controller/task.go`
- Modify: `relay/relay_task.go`
- Test: `controller/task_log_view_test.go`

**Interfaces:**
- Consumes: ByteDance adaptor task facts and actual token usage.
- Produces: durable settlement/refund jobs using local price snapshots.
- Produces: reseller admin diagnostics containing only local and Molii public task IDs.

- [ ] **Step 1: Write failing billing and privacy tests**

```go
func TestByteDanceSeedanceSuccessWithoutUsageRequiresReview(t *testing.T) {
    task := successfulByteDanceSeedanceTaskWithReservedQuota(718937)
    job := service.BuildTerminalTaskBillingJob(ctx, adaptor, task, &relaycommon.TaskInfo{Status: model.TaskStatusSuccess})
    assert.Nil(t, job.TargetQuota)
}

func TestByteDanceSeedanceAdminDiagnosticsHideStarAIID(t *testing.T) {
    task := byteDanceTaskWithUpstream("task_molii_public", `{"upstream_id":"cgt-private"}`)
    view := buildAdminTaskView(task)
    assert.Equal(t, "task_molii_public", view.RootInfo.UpstreamTaskID)
    assert.NotContains(t, marshal(view), "cgt-private")
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./service ./controller ./relay -run 'ByteDanceSeedance.*(Billing|Diagnostics|Polling)'`

- [ ] **Step 3: Register the platform with generic durable polling**

Do not add ByteDance Seedance to `isNativeMoliiTask`, StarAI COS persistence or StarAI direct timing predicates. Let it use its adaptor to poll the Molii public task ID.

- [ ] **Step 4: Recalculate local quota from actual usage**

On terminal success, use `total_tokens`, falling back to `completion_tokens`, with the local task billing snapshot. On terminal failure, enqueue the existing refund operation. If success has no trustworthy usage, set `TargetQuota=nil` so reconciliation exposes `review_required`.

- [ ] **Step 5: Sanitize stored and presented task data**

Store only the Molii public task ID in `PrivateData.UpstreamTaskID`. Persist normalized task facts rather than raw upstream JSON. Ordinary responses omit upstream IDs; reseller admin views expose the Molii task ID only.

- [ ] **Step 6: Test retries and restart recovery**

Add tests for 429/502/503/504 and transport errors retaining the current task state, explicit upstream failure refunding once, and a restarted poller reusing the durable billing job without duplicate settlement.

- [ ] **Step 7: Run polling and billing suites**

Run: `go test ./service ./controller ./relay -run 'Task(Billing|Polling)|ByteDanceSeedance|TaskLog'`

Expected: PASS.

- [ ] **Step 8: Commit polling and billing**

```bash
git add service/task_polling.go service/task_polling_test.go service/task_billing.go service/task_billing_test.go controller/relay.go controller/task.go controller/task_log_view_test.go relay/relay_task.go
git commit -m "feat: settle reseller Seedance tasks durably"
```

### Task 6: Proxy generated video content through authenticated Molii content requests

**Files:**
- Modify: `relay/channel/task/bytedanceseedance/adaptor.go`
- Modify: `relay/channel/task/bytedanceseedance/adaptor_test.go`
- Modify: `controller/video_proxy.go`
- Create: `controller/video_proxy_bytedance_seedance_test.go`
- Modify: `relay/channel/adapter.go` only if the existing content request interface lacks a required safe field

**Interfaces:**
- Produces: `BuildContentRequest(task, artifactKey, clientRequest) (*channel.TaskContentRequest, error)`.
- Consumes: `task.GetUpstreamTaskID()` as the Molii public task ID.
- Produces: authenticated GET/HEAD to `{BaseURL}/v1/videos/{MoliiTaskID}/content`.

- [ ] **Step 1: Write failing GET, HEAD, Range and credential tests**

```go
func TestByteDanceSeedanceVideoProxyUsesChannelCredential(t *testing.T) {
    upstream := newRangeAwareMoliiContentServer(t, func(r *http.Request) {
        assert.Equal(t, "Bearer instance-key", r.Header.Get("Authorization"))
        assert.Equal(t, "bytes=100-199", r.Header.Get("Range"))
        assert.NotEqual(t, "Bearer end-user-key", r.Header.Get("Authorization"))
    })
    task := completedByteDanceTask("task_reseller_public", "task_molii_public", upstream.URL)
    recorder := serveLocalVideoContent(t, task, "Bearer end-user-key", "bytes=100-199")
    assert.Equal(t, http.StatusPartialContent, recorder.Code)
    assert.Equal(t, "bytes 100-199/300", recorder.Header().Get("Content-Range"))
    assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
}

func TestByteDanceSeedanceVideoProxyDoesNotUseExpiredResultURL(t *testing.T) {
    requestedPath := ""
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestedPath = r.URL.Path
        w.Header().Set("Content-Type", "video/mp4")
        io.WriteString(w, "video")
    }))
    defer upstream.Close()
    task := completedByteDanceTaskWithLegacyResult(
        "task_reseller_public",
        "task_molii_public",
        upstream.URL,
        "https://expired.example/signed.mp4",
    )
    recorder := serveLocalVideoContent(t, task, "Bearer end-user-key", "")
    assert.Equal(t, http.StatusOK, recorder.Code)
    assert.Equal(t, "/v1/videos/task_molii_public/content", requestedPath)
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./controller ./relay/channel/task/bytedanceseedance -run 'ByteDanceSeedance.*(Content|VideoProxy)'`

- [ ] **Step 3: Implement the adaptor content request provider**

```go
func (a *TaskAdaptor) BuildContentRequest(task *model.Task, _ string, client channel.TaskArtifactClientRequest) (*channel.TaskContentRequest, error) {
    return &channel.TaskContentRequest{
        URL:     strings.TrimRight(a.baseURL, "/") + "/v1/videos/" + url.PathEscape(task.GetUpstreamTaskID()) + "/content",
        Method:  client.Method,
        Headers: safeMoliiContentHeaders(a.apiKey, client.Headers),
    }, nil
}
```

Only copy Range and If-Range from the client. Always replace Authorization with the channel key.

- [ ] **Step 4: Let native adaptors provide content descriptors**

Refactor `VideoProxy` so any task adaptor implementing `TaskContentRequestProvider` can supply the upstream request, not only JS plugin tasks. Keep direct StarAI tasks on stored COS and preserve existing Grok behavior.

- [ ] **Step 5: Test failure mapping and no task mutation**

Prove an upstream 502/503 returns a safe media proxy error while the completed task and billing job remain unchanged.

- [ ] **Step 6: Run all video proxy tests**

Run: `go test ./controller -run 'VideoProxy|TaskMedia' && go test ./relay/channel/task/bytedanceseedance`

Expected: PASS.

- [ ] **Step 7: Commit content proxying**

```bash
git add relay/channel/task/bytedanceseedance controller/video_proxy.go controller/video_proxy_bytedance_seedance_test.go
git commit -m "feat: proxy reseller Seedance video content"
```

### Task 7: Add the admin UI and neutral temporary-asset presentation

**Files:**
- Modify: `web/src/features/channels/constants.ts`
- Modify: `web/src/features/channels/lib/channel-utils.ts`
- Modify: `web/src/features/channels/lib/channel-type-config.ts`
- Modify: `web/src/features/channels/lib/channel-form.ts`
- Modify: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- Create: `web/src/features/channels/lib/__tests__/bytedance-seedance-channel.test.ts`
- Modify: `web/src/features/temporary-assets/components/create-asset-card.tsx`
- Modify: relevant temporary asset component tests
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/*.json` through the repository i18n sync workflow

**Interfaces:**
- Consumes: existing fetch-model API and auto-sync settings.
- Produces: channel option 64/assigned constant label `ByteDance Seedance` with ByteDance icon.
- Produces: required Base URL and Key form behavior.

- [ ] **Step 1: Write failing UI configuration tests**

```ts
test('ByteDance Seedance requires base URL and key and supports upstream model fetch', () => {
  const config = getChannelTypeConfig(CHANNEL_TYPE_BYTEDANCE_SEEDANCE)
  expect(config.name).toBe('ByteDance Seedance')
  expect(config.requiresBaseUrl).toBe(true)
  expect(config.requiresKey).toBe(true)
  expect(config.supportsFetchModels).toBe(true)
})

test('temporary asset submission uses provider-neutral copy', () => {
  render(<CreateAssetCard />)
  expect(screen.queryByText(/Molii Volcengine Imagine API/)).not.toBeInTheDocument()
})
```

- [ ] **Step 2: Run the focused Vitest tests and verify failure**

Run: `cd web && pnpm test --run src/features/channels/lib/__tests__/bytedance-seedance-channel.test.ts src/features/temporary-assets`

- [ ] **Step 3: Add the channel option and form behavior**

Reuse the existing ByteDance/Doubao icon. Show Base URL, Key, model fetch, manual refresh, auto-sync and last-sync state. Do not render StarAI pricing sections or direct-provider copy.

- [ ] **Step 4: Make temporary-asset copy provider-neutral**

Replace submission/status copy that names `Molii Volcengine Imagine API` with neutral text such as `Submitting temporary asset...`, while leaving API errors and existing translations structurally intact.

- [ ] **Step 5: Sync translations and run UI tests**

Run:

```bash
cd web
pnpm i18n:sync
pnpm i18n:check
pnpm test --run src/features/channels src/features/temporary-assets
pnpm typecheck
pnpm lint
```

Expected: all commands PASS.

- [ ] **Step 6: Commit the admin UI**

```bash
git add web/src/features/channels web/src/features/temporary-assets web/src/i18n
git commit -m "feat: configure ByteDance Seedance channels"
```

### Task 8: Document the reseller setup and verify the complete feature

**Files:**
- Create: `docs/ByteDance-Seedance-代理渠道配置指南.md`
- Modify: `docs/Molii Seedance API 接口文档.md`
- Modify: `docs/Molii-Volcengine-Imagine-API-客户交付指南.md` only to distinguish direct-provider and reseller channel configuration; do not rewrite the public API contract
- Modify: any Docusaurus Seedance/channel page that documents channel setup, after locating its exact source through `rg`
- Modify: `.ccg/tasks/design-reseller-upstream-channel/review.md`

**Interfaces:**
- Documents: Base URL + per-instance Key setup, model sync, local pricing, asset lifecycle, task ID layers, content proxy and troubleshooting.
- Verifies: direct StarAI regression plus reseller end-to-end behavior.

- [ ] **Step 1: Write the operator guide with complete configuration examples**

Include a safe example:

```text
渠道类型：ByteDance Seedance
Base URL：https://aigc.ixiaozu.cn
API Key：由 Molii 为该部署实例单独签发
模型：通过“测试连接并获取模型”自动同步
价格：在代理商实例本地配置，不从 Molii 同步
```

Document that local file upload requires the reseller's own COS, while public URL assets do not.

- [ ] **Step 2: Add a backend integration test covering the full chain**

Use nested `httptest.Server` fixtures to represent Molii and StarAI. Exercise asset create/query, asset-backed submit, poll success with usage, settlement and authenticated Range content. Assert no response contains either fixture's private key or StarAI task ID.

- [ ] **Step 3: Run the complete affected Go test set**

Run:

```bash
go test ./constant ./common ./relay/... ./controller ./service ./model
go test -race ./relay/channel/task/seedanceprotocol ./relay/channel/task/bytedanceseedance ./controller ./service
```

Expected: PASS with PostgreSQL/Redis-dependent integration tests using the project test harness; no SQLite acceptance claim.

- [ ] **Step 4: Run complete affected frontend verification**

Run:

```bash
cd web
pnpm test --run
pnpm typecheck
pnpm lint
pnpm build
```

Expected: PASS.

- [ ] **Step 5: Perform security and compatibility checks**

Run repository searches and tests proving:

```bash
rg -n 'ChannelTypeStarAI|isNativeMoliiTask|upstream_task_id|Authorization' controller service relay
git diff --check
git status --short
```

Review every new occurrence so ByteDance Seedance is not accidentally added to StarAI-only storage or private diagnostic paths. Confirm `git diff` contains no keys, private URLs or unrelated changes.

- [ ] **Step 6: Run the required review gate**

If `/Users/naf/.claude/bin/codeagent-wrapper` is available, run antigravity and Claude reviewers in parallel against the complete diff and record Critical/Warning/Info findings in `.ccg/tasks/design-reseller-upstream-channel/review.md`. If unavailable, record the exact missing-tool result and perform an explicit manual review of protocol privacy, billing idempotency, asset ownership, SSRF and regression tests.

- [ ] **Step 7: Commit documentation and final verification changes**

```bash
git add docs .ccg/tasks/design-reseller-upstream-channel/review.md
git commit -m "docs: add ByteDance Seedance reseller guide"
```

## Self-review result

- Spec coverage: every confirmed design section maps to Tasks 1–8.
- Placeholder scan: every implementation and test step names concrete behavior, commands and expected results.
- Type consistency: the shared protocol, native adaptor, asset resolver and content request interfaces are introduced before their consumers.
- Review focus: model refresh retention is in Task 3; asset isolation is in Task 4; diagnostic sanitization and missing usage are in Task 5; authenticated Range content is in Task 6.
- Scope: the plan does not add other Molii protocols, upstream price sync, video copying, proxy-hop limits or database schema migrations.
