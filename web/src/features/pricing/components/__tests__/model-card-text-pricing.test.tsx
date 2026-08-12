import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

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

  test('renders the fixed Token billing explanation', async () => {
    const { container, root } = await renderCard(baseModel('glm-5.2'))

    const explanation = container.querySelector('[data-text-model-billing]')
    assert.ok(explanation)
    assert.match(
      explanation.textContent ?? '',
      /Billed by input, output, and cached Token usage/
    )
    assert.match(explanation.textContent ?? '', /1M/)
    const matrix = container.querySelector('[data-text-model-pricing-matrix]')
    assert.ok(matrix)
    assert.match(matrix.textContent ?? '', /Input/)
    assert.match(matrix.textContent ?? '', /Output/)
    assert.match(matrix.textContent ?? '', /Cached/)
    assert.match(matrix.textContent ?? '', /\$2/)
    assert.match(matrix.textContent ?? '', /\$4/)
    assert.match(matrix.textContent ?? '', /\$0\.4/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('renders every dynamic price tier with input, output, and cache prices', async () => {
    const model = baseModel('qwen3.5-flash')
    model.billing_mode = 'tiered_expr'
    model.billing_currency = 'CNY'
    model.billing_expr =
      'len <= 128000 ? tier("up_to_128k", p * 0.2 + c * 2 + cr * 0.02) : len <= 256000 ? tier("128k_to_256k", p * 0.8 + c * 8 + cr * 0.08) : tier("256k_to_1m", p * 1.2 + c * 12 + cr * 0.12)'
    const { container, root } = await renderCard(model)

    const matrix = container.querySelector('[data-text-model-pricing-matrix]')
    assert.ok(matrix)
    assert.match(matrix.textContent ?? '', /≤ 128K/)
    assert.match(matrix.textContent ?? '', /128K–256K/)
    assert.match(matrix.textContent ?? '', /256K–1M/)
    assert.match(matrix.textContent ?? '', /¥0\.02/)

    await act(async () => root.unmount())
    container.remove()
  })
})
