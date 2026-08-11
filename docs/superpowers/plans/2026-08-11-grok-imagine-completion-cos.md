# Grok Imagine Completion and COS Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐 Molii Grok Imagine 的正式视频协议、精确媒体计费、本地 Files API，并将图片、视频和文件结果在现有腾讯云 COS 中安全保留 24 小时。

**Architecture:** 在适配器之外新增通用媒体探测、COS 对象存储和用户文件服务。同步图片在响应前完成 COS 持久化；异步视频在写 SUCCESS 终态与账务作业前完成幂等复制。公开 `file_id` 只解析 Molii 自有文件，不透传上游 ID。

**Tech Stack:** Go、Gin、GORM、PostgreSQL、Redis、Tencent COS SDK、`github.com/abema/go-mp4`、现有 billing outbox。

## Global Constraints

- 不支持 `grok-imagine-video-1.5-preview`。
- 不新增渠道切换、渠道亲和或模型映射逻辑。
- 只复用现有 COS Bucket 和凭据；对象前缀固定为 `grok-results/` 与 `grok-files/`。
- 每个 Grok 对象固定保留 24 小时；应用层过期是主保障，COS 生命周期规则由管理员手工配置。
- 不保存真实密钥、上游签名 URL 或原始敏感响应。
- 不自动重试付费 POST；查询、COS 复制和清理必须幂等。
- 不构建 Docker、前端生产包或部署产物；只运行测试、格式和本地开发环境。

---

### Task 1: P0 官方路由与安全 MP4 探测

**Files:**
- Modify: `router/video-router.go`
- Modify: `relay/common/relay_info.go`
- Create: `service/media_probe.go`
- Create: `service/media_probe_test.go`
- Modify: `relay/channel/task/moliigrok/adaptor.go`
- Modify: `relay/channel/task/moliigrok/adaptor_test.go`
- Modify: `service/grok_video_billing.go`
- Modify: `service/grok_video_billing_test.go`

**Interfaces:**
- Produces: `type MediaProbeResult struct { DurationSeconds float64; Width int; Height int; ResolutionTier string; MIMEType string }`
- Produces: `ProbeUserVideo(ctx context.Context, source MediaSource) (*MediaProbeResult, error)`
- Produces: `POST /v1/videos/generations` as an alias of the existing submit handler.
- Consumes: existing SSRF-protected HTTP client and `github.com/abema/go-mp4`.

- [ ] **Step 1: Write failing routing and media-probe tests**

Add tests proving the official route is registered, valid MP4 metadata returns duration/size/480p or 720p, private URLs and unsafe redirects are rejected, oversized or invalid MIME bodies fail, and timeout cancellation is honored.

- [ ] **Step 2: Run RED tests**

```bash
go test ./router ./service ./relay/channel/task/moliigrok -run 'Grok|MediaProbe|VideoGenerationRoute' -count=1
```

Expected: failure because the route and probe interfaces do not exist.

- [ ] **Step 3: Implement the route alias and bounded probe**

Use the existing SSRF client, a context deadline, bounded reader, MP4 MIME/extension checks and `go-mp4` parsing. Normalize the output billing tier to `480p` when input height is at most 480, otherwise cap it at `720p`, matching the official edit/extension output contract.

- [ ] **Step 4: Connect edit billing to immutable input metadata**

Persist `duration_seconds`, `resolution_tier` and `resolution_source: input_probe_v1` in the Grok video billing snapshot before precharge. Keep polling resolution only as an optional consistency check; never use provider cost to infer a tier.

- [ ] **Step 5: Run GREEN and regression tests**

```bash
go test ./router ./service ./relay/channel/task/moliigrok -run 'Grok|MediaProbe|VideoGenerationRoute' -count=1
go test ./service ./relay/channel/task/moliigrok -count=1
gofmt -w router/video-router.go relay/common/relay_info.go service/media_probe.go service/media_probe_test.go relay/channel/task/moliigrok/adaptor.go relay/channel/task/moliigrok/adaptor_test.go service/grok_video_billing.go service/grok_video_billing_test.go
```

### Task 2: Provider-neutral COS primitives and 24-hour object metadata

**Files:**
- Create: `service/object_storage_cos.go`
- Create: `service/object_storage_cos_test.go`
- Modify: `service/starai_cos.go`
- Modify: `model/task.go`
- Create: `service/grok_result_storage.go`
- Create: `service/grok_result_storage_test.go`
- Modify: `main.go`

**Interfaces:**
- Produces: `type StoredObject struct { ObjectKey string; MIMEType string; Size int64; ExpiresAt int64 }`
- Produces: `CopyRemoteObjectToCOS(ctx, sourceURL string, key ObjectKeySpec) (*StoredObject, error)`
- Produces: `SignCOSObjectURL(ctx, objectKey string, ttl time.Duration) (string, error)`
- Produces: `DeleteCOSObject(ctx, objectKey string) error`
- Produces: `EnqueueGrokObjectCleanup(objectKey string, expiresAt int64) error`
- Preserves: all existing `StarAICOS*` wrappers and behavior.

- [ ] **Step 1: Write failing prefix, expiry and idempotency tests**

Assert keys match `PathPrefix/grok-results/{userID}/{image|video}/YYYY/MM/...` or `PathPrefix/grok-files/{userID}/YYYY/MM/...`, expiry is exactly 24 hours, repeated copy for one idempotency key reuses the same object, and cleanup is provider-separated from Seedance.

- [ ] **Step 2: Run RED tests**

```bash
go test ./service -run 'ObjectStorageCOS|GrokResultStorage' -count=1
```

- [ ] **Step 3: Extract generic COS operations without changing Seedance**

Move only shared client/sign/upload/delete behavior behind provider-neutral functions. Keep existing Seedance public functions as wrappers and keep its Redis keys and paths unchanged.

- [ ] **Step 4: Add Grok result metadata and cleanup worker**

Extend `TaskPrivateData` with optional stored-result fields and add a Grok cleanup queue. The worker must treat COS delete-not-found as success and remove queue entries only after successful or already-absent deletion.

- [ ] **Step 5: Run GREEN and Seedance regressions**

```bash
go test ./service -run 'ObjectStorageCOS|GrokResultStorage|StarAICOS' -count=1
go test ./service ./model -count=1
gofmt -w service/object_storage_cos.go service/object_storage_cos_test.go service/starai_cos.go model/task.go service/grok_result_storage.go service/grok_result_storage_test.go main.go
```

### Task 3: Synchronous Grok image result persistence

**Files:**
- Modify: `relay/channel/moliigrok/dto.go`
- Modify: `relay/channel/moliigrok/adaptor.go`
- Modify: `relay/channel/moliigrok/adaptor_test.go`
- Modify: `service/grok_image_billing.go`
- Modify: `service/grok_image_billing_test.go`
- Modify: `model/log.go`
- Modify: `model/log_grok_image_test.go`

**Interfaces:**
- Consumes: Task 2 `CopyRemoteObjectToCOS` and stored object metadata.
- Produces: Molii-owned result URLs for every successful image.
- Preserves: configured CNY billing as the only user charge authority.

- [ ] **Step 1: Write failing all-or-fail persistence tests**

Cover one image, multiple images, `mime_type`, `revised_prompt`, second-image copy failure cleanup, upstream URL removal from logs, and refund behavior when persistence fails.

- [ ] **Step 2: Run RED tests**

```bash
go test ./relay/channel/moliigrok ./service ./model -run 'GrokImage|ImageResultPersistence' -count=1
```

- [ ] **Step 3: Persist all image outputs before responding**

Copy outputs sequentially or with bounded concurrency. If any copy fails, delete objects already created in this request and return the existing sanitized bad-response envelope. Only after all copies succeed may response conversion and final billing complete.

- [ ] **Step 4: Preserve safe metadata and redact provider data**

Return `mime_type` and `revised_prompt` when present. Keep upstream `cost_in_usd_ticks` in admin-only transient audit data; do not write it to the public response or normal user logs.

- [ ] **Step 5: Run GREEN and regressions**

```bash
go test ./relay/channel/moliigrok ./service ./model -run 'GrokImage|ImageResultPersistence' -count=1
go test ./relay/channel/moliigrok ./service ./model -count=1
gofmt -w relay/channel/moliigrok/dto.go relay/channel/moliigrok/adaptor.go relay/channel/moliigrok/adaptor_test.go service/grok_image_billing.go service/grok_image_billing_test.go model/log.go model/log_grok_image_test.go
```

### Task 4: Asynchronous Grok video result persistence

**Files:**
- Modify: `service/task_polling.go`
- Modify: `service/task_polling_molii_grok_test.go`
- Modify: `controller/video_proxy.go`
- Modify: `controller/video_proxy_molii_grok_test.go`
- Modify: `service/video_playback_url.go`
- Modify: `service/video_playback_url_test.go`

**Interfaces:**
- Consumes: Task 2 Grok result storage and existing billing outbox finalization.
- Produces: task SUCCESS only after COS persistence; content route streams COS with Range.

- [ ] **Step 1: Write failing terminal-order and proxy tests**

Assert COS copy occurs before terminal CAS, copy failure leaves the task retryable and creates no settle job, repeated poll uses one object, expired objects return 410, fresh objects support Range, and no upstream signed URL reaches task JSON or logs.

- [ ] **Step 2: Run RED tests**

```bash
go test ./service ./controller -run 'Grok.*Persistence|VideoProxyMoliiGrok|VideoPlayback' -count=1
```

- [ ] **Step 3: Insert idempotent persistence before terminal finalization**

For Grok SUCCESS results, obtain or create the deterministic COS object, update private stored-result metadata, then call the existing terminal CAS + billing job transaction. A retry after copy but before CAS must rediscover the same object.

- [ ] **Step 4: Stream stored objects through the existing content route**

Resolve only the current user's task, enforce expiry, sign a short-lived COS URL internally and proxy it with safe headers and Range behavior. Bearer/session responses use private/no-store cache policy.

- [ ] **Step 5: Run GREEN, race and regression tests**

```bash
go test ./service ./controller -run 'Grok.*Persistence|VideoProxyMoliiGrok|VideoPlayback' -count=1
go test -race ./service ./controller -run 'Grok.*Persistence|VideoProxyMoliiGrok' -count=3
go test ./service ./controller -count=1
gofmt -w service/task_polling.go service/task_polling_molii_grok_test.go controller/video_proxy.go controller/video_proxy_molii_grok_test.go service/video_playback_url.go service/video_playback_url_test.go
```

### Task 5: P1 video extension and reference images

**Files:**
- Modify: `router/video-router.go`
- Modify: `relay/common/relay_info.go`
- Modify: `relay/channel/task/moliigrok/dto.go`
- Modify: `relay/channel/task/moliigrok/adaptor.go`
- Modify: `relay/channel/task/moliigrok/adaptor_test.go`
- Modify: `service/grok_video_billing.go`
- Modify: `service/grok_video_billing_test.go`

**Interfaces:**
- Produces: `POST /v1/videos/extensions`.
- Produces: `reference_images` with 1–7 media references on `grok-imagine-video-1.5`.
- Consumes: Task 1 media probe and Task 2 stored media resolver.

- [ ] **Step 1: Write failing action, validation and billing tests**

Cover extension model/prompt/video, 2–15 second input, 2–10 second extension default 6, unsupported resolution/ratio, reference count 1–7, 720p maximum, and all mutual exclusions.

- [ ] **Step 2: Run RED tests**

```bash
go test ./router ./relay/channel/task/moliigrok ./service -run 'Grok.*Extension|ReferenceImages' -count=1
```

- [ ] **Step 3: Add explicit operations and request transformation**

Do not infer extension/reference from loose fields. Assign an internal operation during validation, build the official upstream path/body, and reject unsupported combinations before precharge.

- [ ] **Step 4: Add immutable extension/reference billing snapshots**

Extension uses probed input seconds and output duration; reference generation records reference count/input prices and the selected output tier. Continue using configured Molii CNY prices and tool invocation prices.

- [ ] **Step 5: Run GREEN and full Grok adaptor regressions**

```bash
go test ./router ./relay/channel/task/moliigrok ./service -run 'Grok.*Extension|ReferenceImages' -count=1
go test ./relay/channel/task/moliigrok ./service -count=1
gofmt -w router/video-router.go relay/common/relay_info.go relay/channel/task/moliigrok/dto.go relay/channel/task/moliigrok/adaptor.go relay/channel/task/moliigrok/adaptor_test.go service/grok_video_billing.go service/grok_video_billing_test.go
```

### Task 6: P2 user-owned Files API

**Files:**
- Create: `model/file.go`
- Create: `model/file_test.go`
- Create: `dto/file.go`
- Create: `service/file.go`
- Create: `service/file_test.go`
- Create: `controller/file.go`
- Create: `controller/file_test.go`
- Modify: `router/relay-router.go`
- Modify: `model/main.go`
- Create: `deploy/migrations/20260811_molii_files.sql`
- Create: `deploy/migrations/20260811_molii_files_test.sh`
- Modify: `relay/channel/moliigrok/adaptor.go`
- Modify: `relay/channel/task/moliigrok/adaptor.go`

**Interfaces:**
- Produces: `POST/GET /v1/files`, `GET/DELETE /v1/files/{id}`, `GET /v1/files/{id}/content`.
- Produces: `ResolveUserFile(ctx, userID, fileID) (*File, string, error)` returning metadata plus a short COS URL.
- Consumes: Task 1 media probe and Task 2 generic COS storage.

- [ ] **Step 1: Write failing model, ownership and expiry tests**

Cover create/list/get/content/delete, same-user access, cross-user 404, expired 410, repeated delete, invalid purpose/MIME/size, and migration idempotency on PostgreSQL 15.

- [ ] **Step 2: Run RED tests**

```bash
go test ./model ./service ./controller -run 'MoliiFile|FilesAPI' -count=1
bash deploy/migrations/20260811_molii_files_test.sh
```

- [ ] **Step 3: Implement the local file lifecycle**

Use public IDs prefixed `file_`, store only UserID-scoped metadata and COS object keys, fix expiry at 24 hours, and enqueue cleanup on create/delete. List only active current-user files.

- [ ] **Step 4: Replace 501 routes and resolve file_id in Grok adaptors**

Every file route uses token authentication and current user ownership. Image/video/reference adapters accept Molii file IDs only, resolve them to short COS URLs before building upstream requests, and keep rejecting unknown or foreign IDs before precharge.

- [ ] **Step 5: Run GREEN, migration and cross-user tests**

```bash
go test ./model ./service ./controller ./relay/channel/moliigrok ./relay/channel/task/moliigrok -run 'MoliiFile|FilesAPI|FileID' -count=1
bash deploy/migrations/20260811_molii_files_test.sh
gofmt -w model/file.go model/file_test.go dto/file.go service/file.go service/file_test.go controller/file.go controller/file_test.go router/relay-router.go model/main.go relay/channel/moliigrok/adaptor.go relay/channel/task/moliigrok/adaptor.go
```

### Task 7: Public contract, model catalog and final backend verification

**Files:**
- Modify: `docs-site/openapi/relay.public.template.yaml`
- Modify: `docs-site/openapi/public-api-surface.json`
- Modify: `docs-site/docs/models/grok-imagine-video.mdx`
- Modify: `docs-site/docs/api-basics/media-inputs.mdx`
- Modify: `docs-site/scripts/content-contract.test.ts`
- Modify: `web/src/features/pricing/lib/grok-api-parameters.ts`
- Modify: `web/src/features/pricing/lib/grok-api-sample.ts`
- Modify: `web/src/features/pricing/lib/grok-model.ts`

**Interfaces:**
- Consumes: Tasks 1–6 final runtime contract.
- Produces: user-only API reference and model-square samples with no admin/channel details.

- [ ] **Step 1: Write failing contract tests**

Assert public docs include generation alias, extension, reference images and local Files API; exclude preview, channel management, upstream IDs, provider pricing and real secrets.

- [ ] **Step 2: Run RED tests**

```bash
cd docs-site && bun test scripts/content-contract.test.ts
```

- [ ] **Step 3: Update the public allowlist, schemas, examples and model pages**

Document exact required/default/enum/range/conditional rules and 24-hour result semantics. Use `$MOLII_API_KEY`, `file_xxx` and `task_xxx` placeholders only.

- [ ] **Step 4: Run local documentation and frontend checks without building**

```bash
cd docs-site && bun test scripts/content-contract.test.ts && bun run api:lint
cd ../web && bun test && bun run typecheck
```

- [ ] **Step 5: Run final backend quality gates**

```bash
go test ./... -count=1
go vet ./...
git diff --check
```

Expected: all tests and checks pass, no image/container/site production build is executed.
