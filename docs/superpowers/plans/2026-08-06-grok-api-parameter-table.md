# Grok API Parameter Table Implementation Plan

> **For Codex:** Execute this plan inline on `feat/molii-auth`; do not call external models or push.

**Goal:** Replace the Grok marketplace API parameter summary with the existing structured parameter table, driven by the selected model and operation.

**Architecture:** Keep backend-derived Grok rules in a pure TypeScript builder. Reuse the existing table renderer in `model-details-api.tsx`, so generic model behavior remains unchanged while Grok operations supply their own metadata.

**Tech Stack:** TypeScript, React 19, i18next, Node test runner, Rsbuild.

---

### Task 1: Specify Grok parameter metadata with tests

**Files:**
- Create: `web/src/features/pricing/lib/__tests__/grok-api-parameters.test.ts`
- Create: `web/src/features/pricing/lib/grok-api-parameters.ts`

1. Add failing tests for image generation/editing, both video model generations, legacy video editing, task status, and download.
2. Run `bun test web/src/features/pricing/lib/__tests__/grok-api-parameters.test.ts` and confirm the missing module failure.
3. Implement `buildGrokApiParameters(modelName, operation)` using the validated backend limits.
4. Re-run the focused test and confirm it passes.

### Task 2: Reuse the standard table for Grok operations

**Files:**
- Modify: `web/src/features/pricing/components/model-details-api.tsx`

1. Extract the existing table body into a renderer that accepts `SupportedParameter[]`.
2. Keep `SupportedParametersSection` behavior unchanged for non-Grok models.
3. Derive Grok parameters from the active operation and render them with the shared table.
4. Remove the inline `parameterSummary` JSX.

### Task 3: Add localized parameter descriptions

**Files:**
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/en.json`

1. Add concise descriptions for model, prompt, image/images, video, task ID, duration, aspect ratio, resolution, and output quantity.
2. Add localized range labels for prompt length and input image count.

### Task 4: Verify and deliver

**Files:**
- Modify: `.ccg/tasks/refine-grok-api-parameter-table/task.json`
- Create: `.ccg/tasks/refine-grok-api-parameter-table/review.md`

1. Run the Grok pricing tests.
2. Run `pnpm typecheck`, `pnpm format:check`, and `pnpm build` from `web/`.
3. Review `git diff` for scope, correctness, and translations.
4. Rebuild the embedded Go binary and restart the existing LaunchAgent.
5. Verify image/video operations in the local model marketplace.
6. Commit implementation, archive the CCG task, and commit the archive. Do not push.
