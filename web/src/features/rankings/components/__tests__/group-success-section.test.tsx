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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { RankingsSnapshot } from '../../types'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
const scrollTo = () => undefined
const matchMedia = () => ({
  matches: false,
  addEventListener: () => undefined,
  removeEventListener: () => undefined,
})
Object.defineProperty(domWindow, 'matchMedia', {
  configurable: true,
  value: matchMedia,
})
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: matchMedia,
})
Object.defineProperty(domWindow, 'scrollTo', {
  configurable: true,
  value: scrollTo,
})
Object.defineProperty(globalThis, 'scrollTo', {
  configurable: true,
  value: scrollTo,
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

const React = await import('react')
const { act } = React
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} = await import('@tanstack/react-router')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')

mock.module('@/components/layout', () => ({
  PublicLayout: ({ children }: { children: React.ReactNode }) =>
    React.createElement(React.Fragment, null, children),
}))

const { GroupSuccessSection } = await import('../group-success-section')
const { Rankings } = await import('../../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiClient = {
  get: (url: string, config?: unknown) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get

const unavailableSnapshot: RankingsSnapshot = {
  models: [],
  vendors: [],
  top_movers: [],
  top_droppers: [],
  models_history: { points: [], models: [], buckets: 0 },
  vendor_share_history: { points: [], vendors: [], buckets: 0 },
  group_success: [],
  group_success_available: false,
}

function rankingsResponse(data: RankingsSnapshot) {
  return { data: { success: true, data } }
}

async function waitForText(text: string): Promise<void> {
  const deadline = Date.now() + 1500
  while (Date.now() < deadline) {
    if (document.body.textContent?.includes(text)) return
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10))
    })
  }
  throw new Error(`Expected text ${text}: ${document.body.textContent}`)
}

async function renderRankingsPage() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  const rootRoute = createRootRoute()
  const rankingsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: 'rankings',
    component: Outlet,
  })
  const rankingsIndexRoute = createRoute({
    getParentRoute: () => rankingsRoute,
    path: '/',
    component: Rankings,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      rankingsRoute.addChildren([rankingsIndexRoute]),
    ]),
    history: createMemoryHistory({ initialEntries: ['/rankings?period=week'] }),
  })

  await router.load()
  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <RouterProvider router={router} />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })

  return { container, root, queryClient, router }
}

afterEach(() => {
  apiClient.get = originalGet
  document.body.replaceChildren()
})

describe('GroupSuccessSection', () => {
  after(() => domWindow.close())

  test('ranks measured groups by success rate then request count and preserves 0%', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <GroupSuccessSection
            period='week'
            groups={[
              { group: 'free', request_count: 0, success_rate: null },
              { group: 'standard', request_count: 80, success_rate: 90 },
              { group: 'premium', request_count: 12, success_rate: 100 },
              { group: 'trial', request_count: 3, success_rate: 0 },
              { group: 'business', request_count: 180, success_rate: 90 },
            ]}
          />
        </I18nextProvider>
      )
    })

    const rows = [...container.querySelectorAll('[data-group-success-row]')]
    assert.deepEqual(
      rows.map((row) => row.getAttribute('data-group-success-row')),
      ['premium', 'business', 'standard', 'trial', 'free']
    )
    assert.match(rows[3].textContent ?? '', /trial.*0%.*3 requests/)
    assert.match(rows[4].textContent ?? '', /free.*No requests/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('labels the selected period and distinguishes an empty configured group list', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <GroupSuccessSection period='year' groups={[]} />
        </I18nextProvider>
      )
    })

    assert.equal(
      container.querySelector('section')?.getAttribute('aria-label'),
      'Group success rates for the past year'
    )
    assert.match(container.textContent ?? '', /No configured groups/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps token rankings visible when the rankings snapshot marks group success unavailable', async () => {
    const requests: string[] = []
    apiClient.get = async (url) => {
      requests.push(url)
      if (url === '/api/rankings') {
        return rankingsResponse(unavailableSnapshot)
      }
      throw new Error(`Unexpected request: ${url}`)
    }

    const { container, root } = await renderRankingsPage()
    await waitForText('Group success rates are temporarily unavailable')

    assert.deepEqual(requests, ['/api/rankings'])
    assert.match(container.textContent ?? '', /Top Models/)
    assert.match(container.textContent ?? '', /Market Share/)
    assert.match(
      container.textContent ?? '',
      /Group success rates are temporarily unavailable/
    )

    await act(async () => root.unmount())
  })
})
