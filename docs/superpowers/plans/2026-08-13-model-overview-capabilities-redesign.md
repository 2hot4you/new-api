# Model Overview Capabilities Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the fragmented LLM overview metadata blocks with one coherent model-capabilities card while preserving pricing and AIGC-specific detail views.

**Architecture:** Extract the generic LLM capability presentation from `model-details.tsx` into a focused `ModelDetailsCapabilities` component. The component consumes the existing `PricingModel` fields, renders core specifications, modalities, effective capabilities, and a low-priority source footer, while `model-details.tsx` keeps pricing and provider metadata orchestration.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, react-i18next, lucide-react, Bun test with happy-dom.

## Global Constraints

- Do not change backend catalog data or billing behavior.
- Do not change Seedance or Grok specialized overview components.
- Do not change the Performance or API tabs.
- Do not add a browser-side models.dev request.
- Use only existing design tokens and dependencies.
- Do not run a production build; use tests, type checking, linting, and local development verification.

---

### Task 1: Build the integrated model-capabilities card

**Files:**
- Create: `web/src/features/pricing/components/model-details-capabilities.tsx`
- Create: `web/src/features/pricing/components/__tests__/model-details-capabilities.test.tsx`

**Interfaces:**
- Consumes: `PricingModel` from `../types`.
- Produces: `ModelDetailsCapabilities({ model }: { model: PricingModel }): JSX.Element | null`.

- [ ] **Step 1: Write a failing component test**

Create a representative text model with `context_length`, `max_output_tokens`, `release_date`, text input/output modalities, five effective capabilities, `metadata_source`, and `metadata_verified_at`. Assert:

```ts
const card = container.querySelector('[data-model-capabilities-card]')
assert.ok(card)
assert.match(card.textContent ?? '', /1M/)
assert.match(card.textContent ?? '', /384K/)
assert.equal(container.querySelectorAll('[data-model-modalities]').length, 1)
assert.equal(container.querySelectorAll('[data-model-capability]').length, 5)
assert.match(container.querySelector('[data-model-metadata-note]')?.textContent ?? '', /models\.dev/)
```

Add a second case with optional dates, capabilities, and source omitted. It must render only existing core facts without empty labels or footer placeholders.

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
cd web
bun test src/features/pricing/components/__tests__/model-details-capabilities.test.tsx
```

Expected: failure because `model-details-capabilities.tsx` does not exist.

- [ ] **Step 3: Implement the focused component**

Implement one bordered section with these internal regions:

```tsx
<section data-model-capabilities-card>
  <header>{t('Model capabilities')}</header>
  <div data-model-core-specs>{/* context, max output, release or knowledge */}</div>
  <div data-model-modalities>{/* input → output, once */}</div>
  <div>{capabilities.map((item) => <div data-model-capability />)}</div>
  <footer data-model-metadata-note>{/* source · verified date */}</footer>
</section>
```

Use `CAPABILITY_LABEL_KEYS`, `MODALITY_LABEL_KEYS`, token/date formatters local to the new component. Render a maximum of three core specification cells. Prefer `release_date`; fall back to `knowledge_cutoff`. Return `null` when no core specification, modality, or capability exists. The metadata footer must not make an otherwise-empty card visible.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```bash
cd web
bun test src/features/pricing/components/__tests__/model-details-capabilities.test.tsx
```

Expected: all component cases pass.

### Task 2: Replace the fragmented overview blocks

**Files:**
- Modify: `web/src/features/pricing/components/model-details.tsx`
- Modify: `web/src/features/pricing/components/__tests__/model-details-capabilities.test.tsx`

**Interfaces:**
- Consumes: `ModelDetailsCapabilities` from `./model-details-capabilities`.
- Produces: generic LLM overview containing one capability card followed by the reduced model-information card.

- [ ] **Step 1: Add a failing integration assertion**

Export `ModelBackendDetailsSection` for testing and render it with a representative model. Assert:

```ts
assert.equal(container.querySelectorAll('[data-model-capabilities-card]').length, 1)
assert.equal(container.querySelectorAll('[data-model-modalities]').length, 1)
assert.equal((container.textContent?.match(/models\.dev/g) ?? []).length, 1)
assert.equal(container.querySelector('[data-model-provider-info]')?.textContent?.includes('models.dev'), false)
```

- [ ] **Step 2: Run the integration test and verify RED**

Run the focused Bun test. Expected: source remains in the provider grid and the old quick-stats/signals components still render duplicated modalities.

- [ ] **Step 3: Integrate the new component**

Remove `ModelBackendQuickStats`, `ModelBackendSignalsSection`, their now-unused icons/helpers, and the source/verification cells from `ModelBackendProviderSection`. Add `data-model-provider-info` to the provider grid. Render:

```tsx
export function ModelBackendDetailsSection({ model }: { model: PricingModel }) {
  return (
    <>
      <ModelDetailsCapabilities model={model} />
      <ModelBackendProviderSection model={model} />
    </>
  )
}
```

- [ ] **Step 4: Run focused and pricing tests**

Run:

```bash
cd web
bun test src/features/pricing/components/__tests__/model-details-capabilities.test.tsx
bun test src/features/pricing
bun run typecheck
bunx oxlint -c .oxlintrc.json \
  src/features/pricing/components/model-details-capabilities.tsx \
  src/features/pricing/components/model-details.tsx \
  src/features/pricing/components/__tests__/model-details-capabilities.test.tsx
```

Expected: all commands exit 0.

- [ ] **Step 5: Verify the local development page**

Refresh `http://127.0.0.1:3000/pricing`, open a text model detail, and verify:

- pricing is unchanged;
- exactly one model-capabilities card appears;
- modalities appear once;
- source/date appear only in the small footer;
- Seedance and Grok details are unchanged.

- [ ] **Step 6: Commit and archive**

After fresh verification, update the CCG review, archive the task, and commit the implementation without pushing.
