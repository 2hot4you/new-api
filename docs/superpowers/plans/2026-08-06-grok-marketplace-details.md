# Grok Marketplace Details Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the four Molii Grok Imagine models accurate descriptions, direct prices, non-token performance metrics, and complete API examples based on the real Molii backend contract.

**Architecture:** Expose a model-shaped Grok catalog-pricing payload and request counts from existing backend data, route video metrics to the correct task platform, then add small Grok-specific frontend helpers/components selected by the existing model-details shell. Seedance and generic text-model components remain unchanged.

**Tech Stack:** Go, GORM, React 19, TypeScript, TanStack Query, node:test, happy-dom, i18next.

---

### Task 1: Grok catalog identity and direct pricing payload

**Files:**
- Modify: `model/pricing_default.go`
- Modify: `model/pricing_default_test.go`
- Modify: `setting/ratio_setting/molii_grok_price.go`
- Modify: `setting/ratio_setting/molii_grok_price_test.go`
- Modify: `model/pricing.go`
- Create: `model/pricing_molii_grok_test.go`

- [ ] **Step 1: Write failing description and catalog-price tests**

Extend `model/pricing_default_test.go` to assert stable keys for all four Grok models. Add a ratio-setting test that expects model-shaped pricing:

```go
pricing, ok := GetMoliiGrokCatalogPricing("grok-imagine-image-quality")
require.True(t, ok)
assert.Equal(t, "image", pricing.Kind)
assert.Equal(t, "cny_per_image", pricing.OutputUnit)
assert.Equal(t, map[string]float64{"1k": 0.05, "2k": 0.07}, pricing.OutputPrices)
assert.Equal(t, 0.01, pricing.ImageInputPrice)
```

Add equivalent assertions for `grok-imagine-video`, including `cny_per_second`, 480p/720p output prices, per-image input price and per-second video input price.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./model ./setting/ratio_setting -run 'TestDefaultGrok|TestMoliiGrokCatalog' -count=1`

Expected: failures because the keys and `GetMoliiGrokCatalogPricing` do not exist.

- [ ] **Step 3: Add the catalog-pricing type and getter**

Define:

```go
type MoliiGrokCatalogPricing struct {
    Kind                string             `json:"kind"`
    OutputUnit          string             `json:"output_unit"`
    OutputPrices        map[string]float64 `json:"output_prices"`
    ImageInputUnit      string             `json:"image_input_unit,omitempty"`
    ImageInputPrice     float64            `json:"image_input_price"`
    VideoInputUnit      string             `json:"video_input_unit,omitempty"`
    VideoInputPrice     float64            `json:"video_input_price"`
}
```

Build each model's payload from the current registered `molii_grok_price` values; never read `ModelPrice` because it is the internal `¥1` quota anchor.

- [ ] **Step 4: Expose pricing through `/api/pricing`**

Add `MoliiGrokPricing *ratio_setting.MoliiGrokCatalogPricing` with JSON name `molii_grok_pricing` to `model.Pricing`. During `updatePricing`, set it only when `GetMoliiGrokCatalogPricing(model)` returns true.

The model-layer test must seed an enabled Molii Grok ability, call `GetPricing`, find the model, and assert that direct prices are present while the payload remains absent for an unrelated model.

- [ ] **Step 5: Run focused and package tests**

Run: `go test ./model ./setting/ratio_setting -count=1`

Expected: PASS.

- [ ] **Step 6: Commit direct catalog pricing**

```bash
git add model/pricing_default.go model/pricing_default_test.go model/pricing.go model/pricing_molii_grok_test.go setting/ratio_setting/molii_grok_price.go setting/ratio_setting/molii_grok_price_test.go
git commit -m "feat: expose Grok marketplace pricing"
```

### Task 2: Request-count and platform-correct performance APIs

**Files:**
- Create: `pkg/perf_metrics/metrics_test.go`
- Modify: `pkg/perf_metrics/types.go`
- Modify: `pkg/perf_metrics/metrics.go`
- Modify: `model/video_performance.go`
- Modify: `model/video_performance_test.go`
- Modify: `controller/perf_metrics.go`

- [ ] **Step 1: Write the failing request-count serialization test**

Build a `counters{requestCount: 3, successCount: 2, totalLatencyMs: 900}` bucket and assert both the group and series point expose `RequestCount == 3` and JSON contains `"request_count":3`.

- [ ] **Step 2: Write the failing Grok platform test**

Seed one matching Grok task on platform 62 and one same-model task on platform 61, call the generalized video performance function, and assert only platform 62 contributes. Retain the existing Seedance platform-61 test.

- [ ] **Step 3: Run tests and verify RED**

Run: `go test ./pkg/perf_metrics ./model -run 'Test.*RequestCount|TestGetVideoPerformance.*Grok' -count=1`

Expected: failures because request counts are not serialized and the video query is fixed to platform 61.

- [ ] **Step 4: Expose existing request counters**

Add `RequestCount int64 \`json:"request_count"\`` to `BucketPoint` and `GroupResult`, populate both from the existing counters in `bucketPoint` and `buildQueryResult`, and update the frontend types later in Task 4. No database migration is required.

- [ ] **Step 5: Generalize video task platform selection**

Rename `GetStarAIVideoPerformance` to `GetVideoPerformance`. Add a private model-to-platform resolver that returns platform 62 for `grok-imagine-video` and `grok-imagine-video-1.5`, platform 61 for both Seedance models, and no platform for unknown models. Return an initialized empty result when no platform is known. Update the controller call.

- [ ] **Step 6: Run focused and package tests**

Run: `go test ./pkg/perf_metrics ./model ./controller -count=1`

Expected: PASS.

- [ ] **Step 7: Commit performance API changes**

```bash
git add pkg/perf_metrics model/video_performance.go model/video_performance_test.go controller/perf_metrics.go
git commit -m "fix: expose media request metrics for Grok"
```

### Task 3: Grok model contract and API sample builders

**Files:**
- Create: `web/src/features/pricing/lib/grok-model.ts`
- Create: `web/src/features/pricing/lib/grok-api-sample.ts`
- Create: `web/src/features/pricing/lib/__tests__/grok-model.test.ts`
- Create: `web/src/features/pricing/components/__tests__/grok-api-details.test.ts`
- Modify: `web/src/features/pricing/types.ts`

- [ ] **Step 1: Write failing model-classification tests**

Assert exact classification for the four IDs and false for `grok-4` and Seedance:

```ts
assert.equal(isMoliiGrokImageModel('grok-imagine-image'), true)
assert.equal(isMoliiGrokVideoModel('grok-imagine-video-1.5'), true)
assert.equal(isMoliiGrokModel('grok-4'), false)
```

- [ ] **Step 2: Write failing sample and parameter tests**

For image generation/edit samples, assert `/v1/images/generations`, `/v1/images/edits`, `aspect_ratio`, `resolution`, `n`, and `image`; reject `size`, `quality` and `content`. For video samples, assert `/v1/videos`, status lookup, `/content` download, and `/v1/videos/edits` only for the legacy model. Run the assertions for cURL, Python, TypeScript and JavaScript.

- [ ] **Step 3: Run tests and verify RED**

Run from `web`: `bun test src/features/pricing/lib/__tests__/grok-model.test.ts src/features/pricing/components/__tests__/grok-api-details.test.ts`

Expected: module-not-found failures.

- [ ] **Step 4: Implement exact model contracts**

Export immutable model sets plus a `getMoliiGrokContract(modelName)` object containing:

```ts
{
  kind: 'image' | 'video',
  resolutions: ['1k', '2k'] | ['480p', '720p'] | ['480p', '720p', '1080p'],
  supportsImageEdit: boolean,
  supportsVideoEdit: boolean,
  requiresImage: boolean,
  durationRange?: [1, 15],
}
```

- [ ] **Step 5: Implement operation-specific samples**

`buildMoliiGrokSample(language, context, operation)` must generate only these operations: `image_generation`, `image_edit`, `video_generation`, `video_edit`, `video_status`, `video_download`. Every sample uses `${baseUrl}` plus the exact Molii route, an environment-backed API key and comments explaining asynchronous steps. Invalid model/operation combinations return no sample instead of inventing parameters.

- [ ] **Step 6: Add frontend pricing types**

Add `MoliiGrokCatalogPricing` and optional `molii_grok_pricing` to `PricingModel`, matching the backend JSON fields exactly.

- [ ] **Step 7: Run tests and commit**

Run from `web`: `bun test src/features/pricing/lib/__tests__/grok-model.test.ts src/features/pricing/components/__tests__/grok-api-details.test.ts`

Expected: PASS.

```bash
git add web/src/features/pricing/lib/grok-model.ts web/src/features/pricing/lib/grok-api-sample.ts web/src/features/pricing/lib/__tests__/grok-model.test.ts web/src/features/pricing/components/__tests__/grok-api-details.test.ts web/src/features/pricing/types.ts
git commit -m "feat: add Grok marketplace API contracts"
```

### Task 4: Grok overview, direct price matrix and non-token performance

**Files:**
- Create: `web/src/features/pricing/components/model-details-grok-overview.tsx`
- Create: `web/src/features/pricing/components/model-details-grok-pricing.tsx`
- Create: `web/src/features/pricing/components/model-details-grok-image-performance.tsx`
- Create: `web/src/features/pricing/components/__tests__/grok-model-details.test.tsx`
- Modify: `web/src/features/pricing/components/model-details.tsx`
- Modify: `web/src/features/pricing/components/model-details-api.tsx`
- Modify: `web/src/features/performance-metrics/types.ts`

- [ ] **Step 1: Write a failing rendered details test**

Render a Grok image details body with request-count metrics and direct pricing. Assert it contains `Requests`, `Average response time`, `Success rate`, `1K`, `2K` and the configured prices, while excluding `TPS`, `TTFT`, `Token`, and `¥1`. Render both video models and assert their capability/resolution differences and absence of frame-rate/Seedance fields.

- [ ] **Step 2: Run the rendered test and verify RED**

Run from `web`: `bun test src/features/pricing/components/__tests__/grok-model-details.test.tsx`

Expected: failure because the Grok-specific components do not exist.

- [ ] **Step 3: Build the Grok overview component**

Use the contract helper to render image/video modality, resolution, duration and edit-support cards. Do not query performance from the overview component; keep overview capability data deterministic and let the performance tab own metrics fetching.

- [ ] **Step 4: Build the direct price matrix**

Render `output_prices` by resolution with `¥ / image` or `¥ / second`, plus image/video input rows when present. If `molii_grok_pricing` is absent, display the existing “pricing unavailable” state; never fall back to `model_price` for a Grok model.

- [ ] **Step 5: Build the image performance component**

Use `getPerfMetrics(model, 24)` and the newly exposed request counts. Summary cards show total requests, weighted average response latency and weighted success rate. The group table contains only group, requests, average response time and success rate. Trend charts use `avg_latency_ms` and `success_rate`; no code path reads `avg_ttft_ms` or `avg_tps`.

- [ ] **Step 6: Route Grok details through the existing shell**

In `ModelDetailsContent`, choose Grok overview/pricing/performance components only when `isMoliiGrokModel(model.model_name)` is true. Keep `ModelDetailsVideoOverview`, `ModelDetailsVideoPerformance`, `OverviewSummaryGrid`, `PriceSection`, and `ModelDetailsPerformance` unchanged for non-Grok models.

In `ModelDetailsApi`, render a Grok operation selector, language selector, exact parameter table and async lifecycle guidance for Grok models; retain the existing generic and Seedance API paths for all others.

- [ ] **Step 7: Update performance response types**

Add `request_count` to `PerformanceGroup` and `PerformanceSeriesPoint` to match Task 2.

- [ ] **Step 8: Run tests and commit**

Run from `web`:

```bash
bun test src/features/pricing
pnpm typecheck
pnpm exec oxlint -c .oxlintrc.json src/features/pricing src/features/performance-metrics
```

Expected: PASS.

```bash
git add web/src/features/pricing/components web/src/features/performance-metrics/types.ts
git commit -m "feat: complete Grok marketplace details"
```

### Task 5: Localized Grok descriptions and labels

**Files:**
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/features/pricing/lib/__tests__/model-description.test.ts`

- [ ] **Step 1: Add failing translation-key assertions**

Assert the four backend keys resolve to non-empty, distinct English and Chinese descriptions and the Grok-specific labels used by the new components exist in both locale files.

- [ ] **Step 2: Run and verify RED**

Run from `web`: `bun test src/features/pricing/lib/__tests__/model-description.test.ts`

Expected: failure because the Grok translation entries do not exist.

- [ ] **Step 3: Add concise English and Chinese content**

Describe actual Molii capabilities only: image generation/editing for the image models, text/image/video generation/editing for legacy video, and required-image generation for video 1.5. Add labels for requests, average response time, direct image/video pricing, input media and asynchronous lifecycle.

- [ ] **Step 4: Run tests and commit**

Run from `web`: `bun test src/features/pricing && pnpm format:check`

Expected: PASS.

```bash
git add web/src/i18n/locales/en.json web/src/i18n/locales/zh.json web/src/features/pricing/lib/__tests__/model-description.test.ts
git commit -m "feat: localize Grok marketplace details"
```

### Task 6: End-to-end verification

**Files:**
- No production file changes expected.

- [ ] **Step 1: Run all backend checks**

Run: `gofmt -w model/pricing_default.go model/pricing_default_test.go setting/ratio_setting/molii_grok_price.go setting/ratio_setting/molii_grok_price_test.go model/pricing.go model/pricing_molii_grok_test.go pkg/perf_metrics/types.go pkg/perf_metrics/metrics.go pkg/perf_metrics/metrics_test.go model/video_performance.go model/video_performance_test.go controller/perf_metrics.go && go test ./...`

Expected: PASS.

- [ ] **Step 2: Run all frontend checks**

Run from `web`:

```bash
bun test src/features/pricing src/features/performance-metrics
pnpm typecheck
pnpm format:check
pnpm lint
pnpm build
```

Expected: PASS.

- [ ] **Step 3: Rebuild and restart the local backend**

Build the frontend and Go binary using the existing local deployment flow, then kickstart `com.molii.new-api`. Poll `http://127.0.0.1:3000/api/status` until HTTP 200; do not use a fixed sleep.

- [ ] **Step 4: Browser-verify all four models**

At `http://127.0.0.1:3000/pricing`, open each Grok model. For every model verify page identity, meaningful content, no framework overlay, clean console, correct overview/pricing/performance/API tabs and a working operation/language interaction. Capture screenshots outside the repository.

- [ ] **Step 5: Check compatibility surfaces**

Open one Seedance model and one token-billed text model. Confirm Seedance still shows its video-specific content and the text model still shows TPS/TTFT/token pricing.

- [ ] **Step 6: Check final diff and service health**

Run: `git diff --check && git status --short && curl -fsS http://127.0.0.1:3000/api/status >/dev/null`

Expected: clean diff formatting, only planned changes, healthy service.
