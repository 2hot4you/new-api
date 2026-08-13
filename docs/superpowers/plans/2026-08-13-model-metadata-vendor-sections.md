# Model Metadata and Vendor Sections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:test-driven-development` for each implementation task and `superpowers:verification-before-completion` before reporting completion.

**Goal:** 让数据库模型元数据成为 `/models/metadata` 与 `/pricing` 的统一资料来源，并在模型广场中以“厂商 Logo + 厂商名称 + 厂商介绍”作为每个厂商模型分组的标题区。

**Architecture:** 后端新增持久化模型规格字段和一个幂等目录对账器；对账器根据当前启用能力创建缺失的 Model/Vendor，只补空字段且尊重禁用与软删除。`/api/pricing` 改为只读地合并数据库元数据、可用能力、计费和性能，不再临时创建模型或厂商。前端管理页编辑这些持久化字段，模型广场直接使用 `/api/pricing` 返回的厂商资料渲染分组标题。

**Tech Stack:** Go 1.26.5、Gin、GORM、PostgreSQL/SQLite tests、React 19、TypeScript、TanStack Query、Zod、Bun test、Tailwind CSS。

## Global Constraints

- 保留用户已有改动，不修改与本任务无关的认证、渠道、COS、异步计费与 Docusaurus 代码。
- `/api/pricing` 是只读聚合接口，不能在 GET 请求期间写 Model 或 Vendor。
- 数据库 Model/Vendor 是模型资料唯一事实来源；前端不得新增模型或厂商硬编码目录。
- 对账只补空字段，不覆盖管理员编辑的简介、Logo、厂商、能力、规格或状态。
- 禁用或软删除模型不得被自动恢复。
- 模型广场仍只展示“启用元数据”和“启用渠道能力”的交集。
- 保留 Seedance/Grok 专用价格表、详情和 API 示例；不得重新加入 `grok-imagine-video-1.5-preview`。
- 厂商标题区必须显示 Logo、名称和介绍；介绍为空时不显示虚构占位文案。
- 厂商分组继续遵循当前筛选、排序和分页结果；跨页时在新页重复厂商标题。
- 只运行本地开发验证，不执行前端或文档生产构建。

---

## Task 1: Extend the persistent model metadata schema

**Files:**

- Modify: `model/model_meta.go`
- Modify: `web/src/features/models/types.ts`
- Test: `model/model_meta_test.go`
- Test: `web/src/features/models/lib/__tests__/model-form.test.ts`

**Interfaces:**

Extend `model.Model` and the frontend `Model` type with:

```go
ContextLength      int      `json:"context_length,omitempty"`
MaxOutputTokens    int      `json:"max_output_tokens,omitempty"`
KnowledgeCutoff    string   `json:"knowledge_cutoff,omitempty"`
ReleaseDate        string   `json:"release_date,omitempty"`
InputModalities    []string `json:"input_modalities,omitempty" gorm:"serializer:json;type:text"`
OutputModalities   []string `json:"output_modalities,omitempty" gorm:"serializer:json;type:text"`
Capabilities       []string `json:"capabilities,omitempty" gorm:"serializer:json;type:text"`
MetadataSource     string   `json:"metadata_source,omitempty"`
MetadataVerifiedAt string   `json:"metadata_verified_at,omitempty"`
```

Add a model-layer normalization/validation helper:

```go
func (mi *Model) NormalizeCatalogMetadata() error
```

It must trim and de-duplicate list values, reject negative token limits, and accept empty or `YYYY-MM-DD` metadata dates.

### Steps

1. Add failing Go tests that round-trip all new fields through SQLite and prove `Model.Update()` persists explicit zero/empty values.
2. Add failing tests for list normalization, duplicate removal, negative numeric limits, and invalid date strings.
3. Run the focused Go test and confirm it fails because fields/helper do not exist.
4. Add the GORM fields, update the explicit `Select(...)` list in `Model.Update()`, and implement normalization/validation.
5. Extend the frontend `Model` type with the same JSON field names and the existing `Modality`/capability string vocabulary.
6. Add a small frontend type/form contract test fixture that includes all fields and confirms values are not silently discarded.
7. Run focused Go and Bun tests, then `gofmt` and scoped frontend lint/format checks.

**Focused verification:**

```bash
go test ./model -run 'TestModelCatalogMetadata|TestNormalizeCatalogMetadata' -count=1
cd web && bun test src/features/models/lib/__tests__/model-form.test.ts
```

**Commit:** `feat: persist model catalog metadata`

---

## Task 2: Centralize curated model and vendor profiles

**Files:**

- Modify: `model/pricing_catalog.go`
- Modify: `model/pricing_default.go`
- Add: `model/model_catalog_profiles_test.go`

**Interfaces:**

Replace separate facts spread across `exactMarketplaceCatalogMetadata`, `defaultModelDescriptionI18nKeys`, vendor rules, and vendor icons with one backend profile registry:

```go
type CatalogVendorProfile struct {
    Name        string
    Icon        string
    Description string
}

type CatalogModelProfile struct {
    ModelName          string
    VendorName         string
    Description        string
    Icon               string
    Tags               string
    ContextLength      int
    MaxOutputTokens    int
    KnowledgeCutoff    string
    ReleaseDate        string
    InputModalities    []string
    OutputModalities   []string
    Capabilities       []string
    MetadataSource     string
    MetadataVerifiedAt string
}

func GetCatalogModelProfile(modelName string) (CatalogModelProfile, bool)
func GetCatalogVendorProfile(vendorName string) (CatalogVendorProfile, bool)
func InferCatalogVendorName(modelName string) string
```

The known vendor descriptions seeded by this registry are:

- MiniMax: `专注于通用大模型与多模态智能能力，为对话、编程、Agent 和内容创作提供模型服务。`
- 阿里巴巴: `通义系列模型提供通用对话、推理、编程与多模态能力，覆盖高性能与高性价比场景。`
- 字节跳动: `豆包模型系列覆盖文本、图像与视频生成，面向多模态内容创作和企业应用。`
- xAI: `xAI 的 Grok Imagine 系列面向高质量图片与视频生成、编辑和创意工作流。`
- DeepSeek: `DeepSeek 提供面向推理、编程和 Agent 工作流的高性能大语言模型。`
- 智谱: `智谱 GLM 系列覆盖通用对话、推理、编程和长上下文应用。`
- Moonshot: `Moonshot 的 Kimi 系列专注于长上下文理解、复杂推理和智能 Agent 场景。`

### Steps

1. Write failing table-driven tests for the seven LLM IDs, two Seedance IDs, and four supported Grok IDs.
2. Assert every curated model resolves to the expected vendor, has a non-empty description, and contains only supported modality/capability values.
3. Assert `grok-imagine-video-1.5-preview` has no profile and is not inferred as a published model.
4. Assert each current vendor profile has a non-empty icon and the approved introduction above.
5. Refactor the existing maps into the unified registry while preserving generic vendor inference for unknown enabled model IDs.
6. Keep billing currency and prices outside the metadata profile; those remain in billing settings.
7. Run focused tests and ensure no frontend file contains a parallel vendor-description lookup table.

**Focused verification:**

```bash
go test ./model -run 'TestCatalog(Model|Vendor)Profiles|TestInferCatalogVendorName' -count=1
rg -n "MiniMax:|Moonshot:|Grok Imagine 系列" web/src/features/pricing
```

**Commit:** `refactor: centralize model catalog profiles`

---

## Task 3: Add idempotent metadata reconciliation

**Files:**

- Add: `model/model_catalog_reconcile.go`
- Add: `model/model_catalog_reconcile_test.go`
- Modify: `model/channel_cache.go`
- Modify: `main.go`
- Test: `main_channel_cache_test.go`

**Interfaces:**

```go
type CatalogReconcileSummary struct {
    CreatedModels  int
    UpdatedModels  int
    CreatedVendors int
    UpdatedVendors int
}

func ReconcileEnabledModelMetadata() (CatalogReconcileSummary, error)
```

### Reconciliation contract

- Query the exact IDs from enabled channel abilities.
- Run the vendor/model writes in one database transaction.
- Use `Unscoped()` lookup for Model to distinguish missing rows from soft-delete tombstones.
- Skip soft-deleted models and never change an existing model's `Status`.
- Create missing exact-name models with `Status=1`, `NameRule=NameRuleExact`, and curated fields when known.
- Unknown models receive only model ID plus a safely inferred vendor; no invented capability/description.
- Existing rows receive only currently empty fields; non-empty admin values win.
- Existing vendor rows receive only empty icon/description fields; vendor status is never changed.
- Multiple consecutive runs are no-ops after the first successful run.

### Steps

1. Add failing transaction-backed tests for creating missing vendors and models from enabled abilities.
2. Add tests for second-run idempotency and unique model/vendor records.
3. Add tests proving admin descriptions, icons, vendor assignment, capabilities, and disabled status are preserved.
4. Add tests proving soft-deleted model tombstones are not restored.
5. Add a test proving unknown models receive minimal metadata with no fake description/specification.
6. Implement the reconciler with explicit “fill if blank/zero/nil” merge helpers inside one transaction.
7. Integrate reconciliation with channel-cache initialization after the cache lock is released and before pricing-cache invalidation, including `MemoryCacheEnabled=false`.
8. Keep startup non-fatal: log a structured warning if reconciliation fails, then allow the service to continue with the existing safe catalog.
9. Add a startup test proving reconciliation executes before the first pricing warm-up without starting extra background workers.
10. Run focused tests and race tests for cache/reconciliation paths.

**Focused verification:**

```bash
go test ./model -run 'TestReconcileEnabledModelMetadata' -count=1
go test ./... -run 'TestInitializeChannelCacheAtStartup|TestInitChannelCache' -count=1
go test -race ./model ./... -run 'TestReconcileEnabledModelMetadata|TestInitializeChannelCacheAtStartup' -count=3
```

**Commit:** `feat: reconcile enabled model metadata`

---

## Task 4: Make `/api/pricing` a read-only metadata aggregate

**Files:**

- Modify: `model/pricing.go`
- Modify: `model/pricing_default.go`
- Modify: `model/pricing_catalog.go`
- Modify: `model/pricing_test.go`
- Add: `model/pricing_metadata_intersection_test.go`
- Modify: `controller/vendor_meta.go`
- Add: `controller/vendor_meta_pricing_test.go`

**Behavior:**

- `updatePricing()` must never call `Insert`, `Update`, or `Create`.
- Exact Model rows supply description, icon, tags, vendor, specifications, modalities, capabilities, source, and verification date.
- Enabled non-exact rules remain usable as explicit administrator rules, but must not produce temporary exact Model records.
- Models missing an enabled exact/rule metadata match are omitted until reconciliation creates their exact row.
- A Model with `Status != 1` is omitted.
- A model whose referenced Vendor has `Status != 1` is omitted.
- `vendorsList` includes only enabled vendors referenced by the currently published pricing models.
- Vendor create/update/delete invalidates pricing cache exactly as Model mutations already do.

### Steps

1. Write a failing test that snapshots Model/Vendor row counts before and after `GetPricing()` and proves no database writes occur.
2. Write failing tests for the Model-enabled/ability-enabled intersection and exclusion of disabled models/vendors.
3. Write a failing test that persisted context, modality, capability, source, description, and vendor data are copied into `Pricing`.
4. Write a failing test that only referenced vendors appear in `/api/pricing.vendors`.
5. Remove `initDefaultVendorMapping()` and `getOrCreateVendor()` from the pricing read path.
6. Remove runtime injection from `exactMarketplaceCatalogMetadata`; populate all public metadata from the selected persistent Model.
7. Preserve existing endpoint inference, billing expressions, special Seedance/Grok price objects, group ratios, and performance merge contracts.
8. Add `model.RefreshPricing()` after successful vendor create/update/delete.
9. Add controller tests proving vendor introduction/logo edits are visible after cache refresh.
10. Run focused model/controller tests and compare the response schema with existing pricing frontend types.

**Focused verification:**

```bash
go test ./model -run 'TestPricing(Metadata|Intersection|ReadOnly|Vendors)' -count=1
go test ./controller -run 'TestVendorMetaRefreshesPricing' -count=1
```

**Commit:** `refactor: source pricing metadata from catalog`

---

## Task 5: Expose catalog fields in `/models/metadata`

**Files:**

- Modify: `controller/model_meta.go`
- Add: `controller/model_meta_catalog_test.go`
- Modify: `web/src/features/models/lib/model-form.ts`
- Modify: `web/src/features/models/components/drawers/model-mutate-drawer.tsx`
- Modify: `web/src/features/models/types.ts`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Test: `web/src/features/models/lib/__tests__/model-form.test.ts`

**UI fields:**

- Context length and maximum output tokens: non-negative integer inputs.
- Knowledge cutoff, release date, metadata source, verified date: text/date inputs.
- Input/output modalities: tag/multi-select values limited to `text`, `image`, `audio`, `video`, `file`.
- Capabilities: tag/multi-select values limited to the existing frontend `ModelCapability` vocabulary.
- Existing model name, description, icon, vendor, tags, endpoints, status, sync, and pricing controls remain unchanged.

### Steps

1. Add failing controller tests for valid metadata create/update and invalid numeric/date/list values.
2. Call `NormalizeCatalogMetadata()` before insert/update and return a stable field-validation error without modifying the existing row.
3. Extend the shared frontend form schema and transform helpers; remove or stop using the duplicate model schema in `types.ts` so the form contract has one source.
4. Add failing frontend tests for load → edit → payload round-trips of every new field.
5. Extend `extendedModelFormSchema` from the shared model schema instead of copying the base model fields again.
6. Add a dedicated “模型能力与规格” form section with clear optional-field labels and multi-select/tag inputs.
7. Add only the required locale strings; do not add these fields to the already-dense models table.
8. Verify an administrator can edit a reconciled DeepSeek/GLM/Kimi/Seedance/Grok record and see the persisted result after reopening the drawer.

**Focused verification:**

```bash
go test ./controller -run 'Test(ModelMetaCatalog|CreateModelMeta|UpdateModelMeta)' -count=1
cd web && bun test src/features/models/lib/__tests__/model-form.test.ts
cd web && bun run typecheck
```

**Commit:** `feat: edit catalog metadata in model manager`

---

## Task 6: Render vendor logo, name, and introduction as section titles

**Files:**

- Modify: `web/src/features/pricing/components/model-card-grid.tsx`
- Modify: `web/src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx`
- Modify: `web/src/features/pricing/lib/vendor-model-groups.ts`
- Modify: `web/src/features/pricing/lib/__tests__/vendor-model-groups.test.ts`

**Component contract:**

For each paginated vendor group, render:

```tsx
<section data-model-vendor-section>
  <header data-model-vendor-heading>
    {/* 使用现有 getLobeIcon 厂商图标解析器 */}
    <div>
      <h2>{vendor_name}</h2>
      {vendor_description ? <p>{vendor_description}</p> : null}
    </div>
  </header>
  <div data-model-vendor-group>{/* existing model cards */}</div>
</section>
```

Use `vendor_icon`, `vendor_name`, and `vendor_description` already joined from `/api/pricing`; do not fetch `/models/metadata` separately and do not introduce frontend vendor profiles.

### Steps

1. Replace the existing test assertion “without a heading” with failing assertions for one heading per vendor group.
2. Assert each heading includes the correct vendor name, icon marker, and exact introduction returned by the backend.
3. Assert an empty description omits the paragraph and does not show “暂无介绍” or other fabricated copy.
4. Assert group order remains first appearance order and non-contiguous same-vendor models remain in one grid on the page.
5. Assert a vendor split by pagination receives a heading on each page where its models appear.
6. Wrap each existing grid in a semantic `section` with a compact title header; retain the current one/two/three-column card grid unchanged.
7. Reuse the existing model/vendor icon component and fallback behavior; do not add a new icon registry.
8. Keep filter, sort, detail drawer, copy button, pricing tables, and pagination behavior unchanged.
9. Run the focused component/grouping tests, typecheck, and scoped lint/format checks.

**Focused verification:**

```bash
cd web && bun test src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx src/features/pricing/lib/__tests__/vendor-model-groups.test.ts
cd web && bun run typecheck
```

**Commit:** `feat: add vendor headers to model marketplace`

---

## Task 7: Integration verification and local development handoff

**Files:**

- Modify: `.ccg/tasks/model-metadata-vendor-sections/task.json`
- Add: `.ccg/tasks/model-metadata-vendor-sections/review.md`
- Modify only if a reusable convention emerged: `.ccg/spec/backend/index.md` or `.ccg/spec/frontend/index.md`

### Steps

1. Run `gofmt` on all changed Go files and scoped frontend format/lint checks.
2. Run focused backend and frontend tests from Tasks 1–6.
3. Run `go test ./...` and `go vet ./...`.
4. Run the frontend test suite and `bun run typecheck`; do not run `bun run build`.
5. Run `git diff --check`, secret scanning already available in the repository, and inspect `git diff --stat`/`git status` for unrelated changes.
6. Restart the local New API service on `127.0.0.1:3000` using the repository's existing local-development command; do not create a Docker image.
7. Verify through local APIs that:
   - `/api/models/` contains the currently enabled models as persistent rows;
   - `/api/pricing` exposes persisted model/vendor metadata and never includes the preview Grok model;
   - vendor descriptions and logos are populated for known vendors;
   - disabled/soft-deleted models stay absent from pricing.
8. Verify in the browser that `/models/metadata` can edit the new fields and `/pricing` renders each vendor as a separate row with its Logo, name, and introduction title.
9. Record verification evidence and any residual risks in `review.md`.
10. Update the task to `completed`, archive it under `.ccg/tasks/archive/2026-08/`, and commit the archive only when the feature changes are intentionally committed.

**Final verification commands:**

```bash
go test ./...
go vet ./...
cd web && bun test
cd web && bun run typecheck
git diff --check
```

**Commit:** `test: verify unified model metadata catalog`
