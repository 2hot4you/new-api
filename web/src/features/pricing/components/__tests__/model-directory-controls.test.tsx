import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import { getModelCategories } from '../../lib/model-directory'
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
  'MouseEvent',
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
const { ModelCategoryBar } = await import('../model-category-bar')
const { PricingSidebar } = await import('../pricing-sidebar')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function model(
  id: number,
  modelName: string,
  overrides: Partial<PricingModel> = {}
): PricingModel {
  return {
    id,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

const models = [
  model(1, 'chat', {
    vendor_name: 'DeepSeek',
    vendor_icon: 'DeepSeek',
    input_modalities: ['text', 'image'],
    output_modalities: ['text'],
    context_length: 1_000_000,
    capabilities: ['reasoning', 'tools'],
    supported_endpoint_types: ['openai'],
    tags: 'agent',
  }),
  model(2, 'image', {
    vendor_name: 'xAI',
    vendor_icon: 'Grok',
    input_modalities: ['text', 'image'],
    output_modalities: ['image'],
    capabilities: ['image_generation'],
    supported_endpoint_types: ['image-generation'],
    quota_type: 1,
    model_price: 0.02,
  }),
  model(3, 'video', {
    vendor_name: 'ByteDance',
    vendor_icon: 'Doubao',
    input_modalities: ['text', 'video'],
    output_modalities: ['video'],
    capabilities: ['video_generation'],
    supported_endpoint_types: ['openai-video'],
    billing_mode: 'tiered_expr',
    billing_expr: 'duration * 1',
  }),
]

describe('model directory category and filter controls', () => {
  after(() => domWindow.close())

  test('renders only non-empty model categories and changes the selected category', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let selected = 'all'

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelCategoryBar
            categories={getModelCategories(models)}
            value={selected}
            onChange={(value) => {
              selected = value
            }}
          />
        </I18nextProvider>
      )
    })

    assert.equal(container.querySelectorAll('[data-model-category]').length, 4)
    assert.ok(container.querySelector('[data-model-category="all"]'))
    assert.ok(container.querySelector('[data-model-category="text"]'))
    assert.ok(container.querySelector('[data-model-category="image"]'))
    assert.ok(container.querySelector('[data-model-category="video"]'))
    assert.equal(container.querySelector('[data-model-category="audio"]'), null)

    await act(async () => {
      container
        .querySelector<HTMLButtonElement>('[data-model-category="video"]')
        ?.click()
    })
    assert.equal(selected, 'video')

    await act(async () => root.unmount())
    container.remove()
  })

  test('renders every supported directory filter section from real model data', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const noop = () => undefined

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <PricingSidebar
            quotaTypeFilter='all'
            endpointTypeFilter='all'
            vendorFilter='all'
            groupFilter='all'
            tagFilter='all'
            inputModalityFilter='all'
            contextFilter='all'
            capabilityFilter='all'
            onQuotaTypeChange={noop}
            onEndpointTypeChange={noop}
            onVendorChange={noop}
            onGroupChange={noop}
            onTagChange={noop}
            onInputModalityChange={noop}
            onContextChange={noop}
            onCapabilityChange={noop}
            vendors={[
              { id: 1, name: 'DeepSeek', icon: 'DeepSeek' },
              { id: 2, name: 'xAI', icon: 'Grok' },
              { id: 3, name: 'ByteDance', icon: 'Doubao' },
            ]}
            groups={['default']}
            tags={['agent']}
            models={models}
            hasActiveFilters={false}
            onClearFilters={noop}
          />
        </I18nextProvider>
      )
    })

    for (const section of [
      'input-types',
      'context-length',
      'vendors',
      'capabilities',
      'endpoint-types',
      'pricing-types',
      'groups',
      'tags',
    ]) {
      assert.ok(
        container.querySelector(`[data-filter-section="${section}"]`),
        `missing ${section}`
      )
    }
    assert.match(container.textContent ?? '', /DeepSeek/)
    assert.match(container.textContent ?? '', /Reasoning/)
    assert.match(container.textContent ?? '', /1M/)

    await act(async () => root.unmount())
    container.remove()
  })
})
