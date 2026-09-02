import assert from 'node:assert/strict'
import { afterAll as after, describe, test } from 'vitest'

import { Window } from 'happy-dom'

import type { PricingModel } from '../../types'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
Object.defineProperty(domWindow.document, 'compatMode', {
  configurable: true,
  value: 'CSS1Compat',
})
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'matchMedia',
  'customElements',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ModelCard } = await import('../model-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function baseModel(modelName: string): PricingModel {
  return {
    id: 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    cache_ratio: 0.2,
    enable_groups: ['default'],
  }
}

async function renderCard(model: PricingModel) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <ModelCard model={model} onClick={() => undefined} />
      </I18nextProvider>
    )
  })
  return { container, root }
}

describe('selected text-model marketplace card pricing', () => {
  after(() => domWindow.close())

  test('renders compact fixed Token prices without the former full table', async () => {
    const { container, root } = await renderCard(baseModel('glm-5.2'))

    const description = container.querySelector('p')
    assert.ok(description)
    assert.doesNotMatch(
      description.className,
      /(?:^|\s)flex-1(?:\s|$)/,
      'description must not push the pricing table downward'
    )
    const pricingSummary = container.querySelector('[data-model-card-pricing]')
    assert.ok(pricingSummary)
    assert.doesNotMatch(
      pricingSummary.textContent ?? '',
      /Billed by input, output, and cached Token usage/
    )
    assert.equal(
      container.querySelector('[data-text-model-pricing-matrix]'),
      null
    )
    assert.match(pricingSummary.textContent ?? '', /Input/)
    assert.match(pricingSummary.textContent ?? '', /Output/)
    assert.match(pricingSummary.textContent ?? '', /Cached/)
    assert.match(pricingSummary.textContent ?? '', /\$2/)
    assert.match(pricingSummary.textContent ?? '', /\$4/)
    assert.match(pricingSummary.textContent ?? '', /\$0\.4/)
    assert.match(pricingSummary.textContent ?? '', /1,000,000 Token/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('renders the input-length strategy and only the starting dynamic price', async () => {
    const model = baseModel('qwen3.5-flash')
    model.billing_mode = 'tiered_expr'
    model.billing_currency = 'CNY'
    model.billing_expr =
      'len <= 128000 ? tier("up_to_128k", p * 0.2 + c * 2 + cr * 0.02) : len <= 256000 ? tier("128k_to_256k", p * 0.8 + c * 8 + cr * 0.08) : tier("256k_to_1m", p * 1.2 + c * 12 + cr * 0.12)'
    const { container, root } = await renderCard(model)

    const summary = container.querySelector('[data-model-card-pricing]')
    assert.ok(summary)
    assert.match(
      summary.textContent ?? '',
      /Tiered by per-request input Tokens/
    )
    assert.match(summary.textContent ?? '', /≤ 128K \/ 128K–256K \/ > 256K/)
    assert.match(summary.textContent ?? '', /¥0\.2/)
    assert.match(summary.textContent ?? '', /1,000,000 Token/)
    assert.doesNotMatch(summary.textContent ?? '', /¥0\.8|¥1\.2/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows a compact effective-capability summary without crowding the card', async () => {
    const model = baseModel('glm-5.2')
    model.context_length = 1_000_000
    model.capabilities = [
      'streaming',
      'system_prompt',
      'reasoning',
      'tools',
      'structured_output',
    ]
    const { container, root } = await renderCard(model)

    const specs = container.querySelector('[data-model-card-specs]')
    const capabilities = container.querySelector(
      '[data-model-card-capabilities]'
    )
    assert.ok(specs)
    assert.ok(capabilities)
    assert.match(specs.textContent ?? '', /1M/)
    assert.match(capabilities.textContent ?? '', /Reasoning/)
    assert.match(capabilities.textContent ?? '', /Tools/)
    assert.match(capabilities.textContent ?? '', /Structured output/)
    assert.doesNotMatch(capabilities.textContent ?? '', /System prompt/)

    await act(async () => root.unmount())
    container.remove()
  })
})
