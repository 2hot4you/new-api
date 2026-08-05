# Molii AIGC Pricing Unit Input Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move all Molii AIGC pricing units into the right edge of their inputs and provide complete Chinese text for the Grok pricing form.

**Architecture:** Add one small presentational input-group component shared by the two billing forms. Keep form state and option persistence untouched; translate labels and units at the form boundary through the existing i18next locale files.

**Tech Stack:** React 19, TypeScript, React Hook Form, i18next, Base UI input groups, Bun node:test runner.

---

### Task 1: Add the pricing unit input contract

**Files:**
- Create: `web/src/features/system-settings/billing/pricing-unit-input.tsx`
- Create: `web/src/features/system-settings/billing/__tests__/pricing-unit-input.test.tsx`

- [ ] **Step 1: Write the failing layout test**

Render `PricingUnitInput` with a numeric value and `¥ / 张`, then assert that the input and the addon share the same `data-slot="input-group"` ancestor and that the addon has `data-align="inline-end"`.

- [ ] **Step 2: Run the test and verify RED**

Run: `bun test src/features/system-settings/billing/__tests__/pricing-unit-input.test.tsx`

Expected: FAIL because `pricing-unit-input.tsx` does not exist.

- [ ] **Step 3: Implement the minimal shared component**

Compose `InputGroup`, `InputGroupInput`, and `InputGroupAddon`. Forward all native input props to `InputGroupInput` and render `props.unit` only in the inline-end addon.

- [ ] **Step 4: Run the test and verify GREEN**

Run: `bun test src/features/system-settings/billing/__tests__/pricing-unit-input.test.tsx`

Expected: PASS.

### Task 2: Integrate translated suffixes into both pricing forms

**Files:**
- Modify: `web/src/features/system-settings/billing/molii-grok-pricing-section.tsx`
- Modify: `web/src/features/system-settings/billing/starai-video-pricing-section.tsx`
- Modify: `web/src/features/system-settings/billing/__tests__/pricing-unit-input.test.tsx`

- [ ] **Step 1: Extend the test with source form contracts**

Assert that Grok uses per-image, per-second, per-thousand-calls and per-completed-image translation keys, and Seedance uses the per-million-tokens key without appending a unit to `FormDescription`.

- [ ] **Step 2: Run the test and verify RED**

Expected: FAIL because both forms still use a plain `Input` and bottom descriptions for units.

- [ ] **Step 3: Replace plain inputs with `PricingUnitInput`**

Pass `t(item.unit)` to Grok and `t('pricing.unit.perMillionTokens')` to Seedance. Keep all numeric form props unchanged. Remove Grok's unit-only `FormDescription` and keep only Seedance's translated reference-video description.

- [ ] **Step 4: Run the test and verify GREEN**

Expected: PASS.

### Task 3: Add locale coverage

**Files:**
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/features/system-settings/billing/__tests__/pricing-unit-input.test.tsx`

- [ ] **Step 1: Write failing locale assertions**

Assert the five unit keys and all Grok pricing labels resolve to Chinese in `zh` and `zh-TW` rather than falling back to English.

- [ ] **Step 2: Run the test and verify RED**

Expected: FAIL because the keys are currently absent.

- [ ] **Step 3: Add translations**

Add English source values plus concise Simplified and Traditional Chinese translations. Use `¥ / 张`, `¥ / 秒`, `¥ / 千次调用`, `¥ / 张完成图片`, and `¥ / 百万 Token` in Simplified Chinese.

- [ ] **Step 4: Run the test and verify GREEN**

Expected: PASS.

### Task 4: Verify and deploy locally

**Files:**
- Modify only if verification finds a defect in the files above.

- [ ] **Step 1: Format and lint touched files**

Run targeted `oxfmt` and `oxlint` commands for the touched TS/TSX files, and validate the three JSON locale files.

- [ ] **Step 2: Run frontend verification**

Run the affected test, `bun run typecheck`, and `bun run build` from `web/`.

- [ ] **Step 3: Run repository verification**

Run `go test ./...` after the frontend build has completed.

- [ ] **Step 4: Restart and inspect the local service**

Rebuild the local Go binary, restart `com.molii.new-api`, verify `http://127.0.0.1:3000` responds, and inspect both pricing tabs in Chinese.

- [ ] **Step 5: Review the final diff**

Confirm that only the shared input, two billing forms, locale resources, tests, design/plan, and CCG task records changed.
