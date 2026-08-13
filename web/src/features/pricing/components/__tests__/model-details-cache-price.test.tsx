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
const { PriceSection } = await import('../model-details')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function fixedModel(cacheRatio?: number): PricingModel {
  return {
    id: 1,
    model_name: 'fixed-llm',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    cache_ratio: cacheRatio,
    enable_groups: ['default'],
  }
}

function dynamicModel(): PricingModel {
  return {
    ...fixedModel(),
    model_name: 'dynamic-llm',
    billing_mode: 'tiered_expr',
    billing_currency: 'CNY',
    billing_expr: 'tier("default", p * 0.2 + c * 2 + cr * 0.02 + cc * 0.04)',
  }
}

async function renderPriceSection(model: PricingModel) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <PriceSection
          model={model}
          priceRate={1}
          usdExchangeRate={1}
          tokenUnit='M'
          showRechargePrice={false}
        />
      </I18nextProvider>
    )
  })

  return { container, root }
}

describe('LLM model-detail cache price visibility', () => {
  after(() => domWindow.close())

  test('renders input, output, and cached input as equal primary cards', async () => {
    const { container, root } = await renderPriceSection(fixedModel(0.2))

    const grid = container.querySelector('[data-base-price-primary-grid]')
    assert.ok(grid)
    assert.deepEqual(
      [...grid.children].map((card) =>
        card.getAttribute('data-base-price-card-type')
      ),
      ['input', 'output', 'cache']
    )
    assert.match(grid.className, /sm:grid-cols-3/)
    assert.equal(container.querySelector('[data-base-price-secondary]'), null)

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps input and output in a two-column grid without cache pricing', async () => {
    const { container, root } = await renderPriceSection(fixedModel())

    const grid = container.querySelector('[data-base-price-primary-grid]')
    assert.ok(grid)
    assert.deepEqual(
      [...grid.children].map((card) =>
        card.getAttribute('data-base-price-card-type')
      ),
      ['input', 'output']
    )
    assert.match(grid.className, /grid-cols-2/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('promotes dynamic cache read while keeping cache write secondary', async () => {
    const { container, root } = await renderPriceSection(dynamicModel())

    const grid = container.querySelector('[data-base-price-primary-grid]')
    assert.ok(grid)
    assert.deepEqual(
      [...grid.children].map((card) =>
        card.getAttribute('data-base-price-card-field')
      ),
      ['inputPrice', 'outputPrice', 'cacheReadPrice']
    )
    assert.match(grid.className, /sm:grid-cols-3/)

    const secondary = container.querySelector('[data-base-price-secondary]')
    assert.ok(secondary)
    assert.match(secondary.textContent ?? '', /Cache Write/)
    assert.doesNotMatch(secondary.textContent ?? '', /Cache Read/)

    await act(async () => root.unmount())
    container.remove()
  })
})
