import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Window } from 'happy-dom'

import type { PricingModel } from '../../types'
import { getRelatedModels } from '../related-models'

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
  'ResizeObserver',
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

const { renderToStaticMarkup } = await import('react-dom/server')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ModelDetailsContent } = await import('../model-details')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = false

function model(
  modelName: string,
  overrides: Partial<PricingModel> = {}
): PricingModel {
  return {
    id: overrides.id ?? 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    cache_ratio: 0.1,
    enable_groups: ['default'],
    vendor_id: 10,
    vendor_name: 'DeepSeek',
    input_modalities: ['text'],
    output_modalities: ['text'],
    capabilities: ['reasoning', 'tools'],
    supported_endpoint_types: ['openai'],
    context_length: 1_000_000,
    max_output_tokens: 64_000,
    release_date: '2026-08-13',
    ...overrides,
  }
}

describe('independent model directory detail page', () => {
  after(() => domWindow.close())

  test('renders pricing, capabilities, performance, and API as one continuous page', () => {
    const container = document.createElement('div')
    const queryClient = new QueryClient({
      defaultOptions: { queries: { enabled: false, retry: false } },
    })

    container.innerHTML = renderToStaticMarkup(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ModelDetailsContent
            model={model('deepseek-v4')}
            groupRatio={{ default: 1 }}
            usableGroup={{ default: { desc: 'Default', ratio: 1 } }}
            endpointMap={{}}
            autoGroups={[]}
            priceRate={1}
            usdExchangeRate={1}
            tokenUnit='M'
          />
        </I18nextProvider>
      </QueryClientProvider>
    )

    assert.equal(container.querySelector('[role="tablist"]'), null)
    assert.ok(container.querySelector('[data-model-detail-anchor-nav]'))
    for (const id of ['pricing', 'capabilities', 'performance', 'api']) {
      assert.ok(container.querySelector(`#${id}`), `missing #${id}`)
    }

    queryClient.clear()
  })

  test('selects same-vendor related models newest first and excludes the current model', () => {
    const current = model('current', { release_date: '2026-08-10' })
    const related = getRelatedModels(current, [
      model('current', { release_date: '2026-08-10' }),
      model('older', { release_date: '2026-06-01' }),
      model('newer', { release_date: '2026-08-12' }),
      model('undated', { release_date: undefined }),
      model('other-vendor', {
        vendor_id: 20,
        vendor_name: 'Qwen',
        release_date: '2026-08-13',
      }),
    ])

    assert.deepEqual(
      related.map((item) => item.model_name),
      ['newer', 'older', 'undated']
    )
  })
})
