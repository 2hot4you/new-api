/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// @ts-expect-error Bun's test mock module has no repository TypeScript declaration.
import { mock } from 'bun:test'
import assert from 'node:assert/strict'
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
const matchMedia = () => ({
  matches: false,
  addEventListener: () => undefined,
  removeEventListener: () => undefined,
})
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: matchMedia,
})
Object.defineProperty(globalThis, 'scrollTo', {
  configurable: true,
  value: () => undefined,
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
  'customElements',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

type CapturedSpec = {
  data?: Array<{ id?: string }>
  tooltip?: {
    updateElement?: (
      tooltipElement: HTMLElement,
      actualTooltip: { content?: Array<{ key?: string }> }
    ) => void
  }
}

const specs = new Map<string, CapturedSpec>()

mock.module('@visactor/react-vchart', () => ({
  VChart: ({ spec }: { spec: CapturedSpec }) => {
    const id = spec.data?.[0]?.id
    if (id) specs.set(id, spec)
    return <div data-vchart={id} />
  },
}))
mock.module('@/lib/use-chart-theme', () => ({
  useChartTheme: () => ({
    resolvedTheme: 'light',
    themeReady: true,
  }),
}))

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} = await import('@tanstack/react-router')
const { MarketShareSection } = await import('../market-share-section')
const { ModelsSection } = await import('../models-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

after(() => domWindow.close())

function tooltipWithOneShape(): HTMLElement {
  const tooltip = document.createElement('div')
  tooltip.innerHTML = '<div data-col="shape"><div></div></div>'
  return tooltip
}

test('uses configured model and vendor icons in ranking tooltips and vendor rows', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  function RankingIconFixtures() {
    return (
      <I18nextProvider i18n={i18n}>
        <div>
          <ModelsSection
            period='week'
            rows={[
              {
                rank: 1,
                model_name: 'claude-sonnet-4-6',
                model_icon: 'Claude.Color',
                vendor: 'Anthropic',
                vendor_icon: 'Anthropic.Color',
                category: 'all',
                total_tokens: 100,
                share: 1,
                growth_pct: 0,
              },
            ]}
            history={{
              buckets: 1,
              models: [
                { name: 'claude-sonnet-4-6', vendor: 'Anthropic', total: 100 },
              ],
              points: [
                {
                  ts: '1',
                  label: 'Aug 26',
                  model: 'claude-sonnet-4-6',
                  vendor: 'Anthropic',
                  tokens: 100,
                },
              ],
            }}
          />
          <MarketShareSection
            period='week'
            rows={[
              {
                rank: 1,
                vendor: 'Anthropic',
                vendor_icon: 'Anthropic.Color',
                total_tokens: 100,
                share: 1,
                growth_pct: 0,
                models_count: 1,
                top_model: 'claude-sonnet-4-6',
              },
            ]}
            history={{
              buckets: 1,
              vendors: [{ name: 'Anthropic', total: 100, share: 1 }],
              points: [
                {
                  ts: '1',
                  label: 'Aug 26',
                  vendor: 'Anthropic',
                  share: 1,
                  tokens: 100,
                },
              ],
            }}
          />
        </div>
      </I18nextProvider>
    )
  }

  const rootRoute = createRootRoute()
  const fixtureRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/',
    component: RankingIconFixtures,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([fixtureRoute]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()

  await act(async () => {
    root.render(<RouterProvider router={router} />)
  })

  const modelTooltip = tooltipWithOneShape()
  specs.get('models-history')?.tooltip?.updateElement?.(modelTooltip, {
    content: [{ key: 'claude-sonnet-4-6' }],
  })
  assert.equal(
    modelTooltip
      .querySelector('[data-ranking-tooltip-icon]')
      ?.getAttribute('data-ranking-tooltip-icon'),
    'Claude.Color'
  )

  const vendorTooltip = tooltipWithOneShape()
  specs.get('vendor-share')?.tooltip?.updateElement?.(vendorTooltip, {
    content: [{ key: 'Anthropic' }],
  })
  assert.equal(
    vendorTooltip
      .querySelector('[data-ranking-tooltip-icon]')
      ?.getAttribute('data-ranking-tooltip-icon'),
    'Anthropic.Color'
  )
  assert.equal(
    container
      .querySelector('[data-ranking-vendor-icon]')
      ?.getAttribute('data-ranking-vendor-icon'),
    'Anthropic.Color'
  )
  const vendorIdentity = container.querySelector(
    '[data-ranking-vendor-identity]'
  )
  assert.ok(vendorIdentity)
  assert.ok(vendorIdentity.querySelector('[data-ranking-vendor-icon]'))
  assert.equal(vendorIdentity.textContent?.includes('Anthropic'), true)

  await act(async () => root.unmount())
  container.remove()
})
