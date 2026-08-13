import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
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
const { ModelCardGrid } = await import('../model-card-grid')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function model(
  id: number,
  modelName: string,
  vendorName: string,
  releaseDate: string
): PricingModel {
  return {
    id,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    cache_ratio: 0.1,
    enable_groups: ['default'],
    vendor_name: vendorName,
    vendor_icon: vendorName,
    description: `${modelName} description`,
    release_date: releaseDate,
    context_length: 1_000_000,
    max_output_tokens: 64_000,
    input_modalities: ['text'],
    output_modalities: ['text'],
    capabilities: ['reasoning', 'tools'],
    supported_endpoint_types: ['openai'],
  }
}

describe('high-density model directory grid', () => {
  after(() => domWindow.close())

  test('renders one continuous responsive grid without vendor group sections', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { enabled: false } },
    })

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <ModelCardGrid
              models={[
                model(1, 'new-model', 'DeepSeek', '2026-08-13'),
                model(2, 'older-model', 'Qwen', '2026-07-01'),
              ]}
              onModelClick={() => undefined}
            />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    const grid = container.querySelector('[data-model-directory-grid]')
    assert.ok(grid)
    assert.match(grid.className, /grid-cols-1/)
    assert.match(grid.className, /md:grid-cols-2/)
    assert.match(grid.className, /xl:grid-cols-3/)
    assert.equal(
      container.querySelectorAll('[data-model-vendor-section]').length,
      0
    )
    assert.equal(container.querySelectorAll('[data-model-card]').length, 2)
    assert.match(container.textContent ?? '', /DeepSeek/)
    assert.match(container.textContent ?? '', /new-model description/)
    assert.match(container.textContent ?? '', /1M/)
    assert.match(container.textContent ?? '', /64K/)
    assert.match(container.textContent ?? '', /2026-08-13/)
    assert.match(container.textContent ?? '', /OpenAI/)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })
})
