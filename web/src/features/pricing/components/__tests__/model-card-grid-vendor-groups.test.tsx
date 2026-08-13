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
  vendorId: number,
  vendorName: string
): PricingModel {
  return {
    id,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    vendor_id: vendorId,
    vendor_name: vendorName,
  }
}

describe('model marketplace vendor grid rows', () => {
  after(() => domWindow.close())

  test('renders each vendor in its own responsive grid without a heading', async () => {
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
                model(1, 'vendor-a-first', 10, 'Vendor A'),
                model(2, 'vendor-b-first', 20, 'Vendor B'),
                model(3, 'vendor-a-second', 10, 'Vendor A'),
              ]}
              onModelClick={() => undefined}
            />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    const groups = [...container.querySelectorAll('[data-model-vendor-group]')]
    assert.equal(groups.length, 2)
    assert.match(groups[0]?.textContent ?? '', /vendor-a-first/)
    assert.match(groups[0]?.textContent ?? '', /vendor-a-second/)
    assert.doesNotMatch(groups[0]?.textContent ?? '', /vendor-b-first/)
    assert.match(groups[1]?.textContent ?? '', /vendor-b-first/)
    for (const group of groups) {
      assert.match(group.className, /grid-cols-1/)
      assert.match(group.className, /md:grid-cols-2/)
      assert.match(group.className, /2xl:grid-cols-3/)
    }
    assert.equal(container.querySelector('[data-model-vendor-heading]'), null)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })
})
