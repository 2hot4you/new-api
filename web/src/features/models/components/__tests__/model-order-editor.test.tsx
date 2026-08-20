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

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Window } from 'happy-dom'

import type { Model, Vendor } from '../../types'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
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
  'PointerEvent',
  'KeyboardEvent',
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
Object.defineProperty(domWindow, 'scrollTo', {
  configurable: true,
  value: () => undefined,
})
Object.defineProperty(globalThis, 'scrollTo', {
  configurable: true,
  value: domWindow.scrollTo,
})

const React = await import('react')
const { act } = React
const { createRoot } = await import('react-dom/client')
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
const { modelsQueryKeys } = await import('../../lib')
const Motion = await import('motion/react')

const dragStartEvents: Event[] = []
const reorderItemDragValues: (false | 'y')[] = []
let reorderGroupProps: {
  axis?: string
  onReorder?: (items: Model[]) => void
} | null = null

mock.module('motion/react', () => ({
  ...Motion,
  Reorder: {
    Group: ({
      children,
      axis,
      onReorder,
      values: _values,
      ...props
    }: {
      children: React.ReactNode
      axis: string
      onReorder: (items: Model[]) => void
      values: Model[]
    }) => {
      reorderGroupProps = { axis, onReorder }
      return React.createElement('ul', props, children)
    },
    Item: ({
      children,
      value: _value,
      drag,
      dragListener: _dragListener,
      dragControls: _dragControls,
      ...props
    }: {
      children: React.ReactNode
      value: Model
      drag: false | 'y'
      dragListener: boolean
      dragControls: unknown
    }) => {
      reorderItemDragValues.push(drag)
      return React.createElement('li', props, children)
    },
  },
  useDragControls: () => ({
    start: (event: Event) => dragStartEvents.push(event),
  }),
}))

const { ModelOrderEditor } = await import('../model-order-editor')
const { ModelsPrimaryButtons } = await import('../models-primary-buttons')
const { ModelsProvider, useModels } = await import('../models-provider')
const { Models } = await import('../../index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiClient = {
  get: (url: string) => Promise<{ data: unknown }>
  put: (url: string, data: unknown) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get
const originalPut = apiClient.put

const models: Model[] = [
  {
    id: 1,
    model_name: 'model-a',
    vendor_id: 7,
    status: 1,
    created_time: 1,
    updated_time: 1,
    name_rule: 0,
  },
  {
    id: 2,
    model_name: 'model-b',
    vendor_id: 8,
    status: 0,
    created_time: 1,
    updated_time: 1,
    name_rule: 0,
  },
]

const vendors: Vendor[] = [
  { id: 7, name: 'Vendor A', status: 1, created_time: 1, updated_time: 1 },
  { id: 8, name: 'Vendor B', status: 1, created_time: 1, updated_time: 1 },
]

function response(data: unknown) {
  return { data: { success: true, data } }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
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

async function waitFor(condition: () => boolean, message: string) {
  const deadline = Date.now() + 1500
  while (Date.now() < deadline) {
    if (condition()) return
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10))
    })
  }
  throw new Error(message)
}

function findButton(name: string): HTMLButtonElement {
  const button = [...document.querySelectorAll('button')].find(
    (candidate) =>
      candidate.textContent?.trim() === name ||
      candidate.getAttribute('aria-label') === name
  )
  assert.ok(button, `Expected button ${name}`)
  return button
}

async function click(element: Element): Promise<void> {
  await act(async () => {
    element.dispatchEvent(
      new domWindow.MouseEvent('click', { bubbles: true }) as unknown as Event
    )
  })
}

async function renderEditor({
  onSaved = () => undefined,
  onCancel = () => undefined,
  onSavingChange,
}: {
  onSaved?: () => void
  onCancel?: () => void
  onSavingChange?: (isSaving: boolean) => void
} = {}) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <ModelOrderEditor
            onSaved={onSaved}
            onCancel={onCancel}
            onSavingChange={onSavingChange}
          />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })
  return { container, root, queryClient }
}

async function renderModelsPage() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  const rootRoute = createRootRoute()
  const authenticatedRoute = createRoute({
    getParentRoute: () => rootRoute,
    id: '_authenticated',
    component: Outlet,
  })
  const modelsRoute = createRoute({
    getParentRoute: () => authenticatedRoute,
    path: 'models/$section',
    component: Models,
  })
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      authenticatedRoute.addChildren([modelsRoute]),
    ]),
    history: createMemoryHistory({ initialEntries: ['/models/metadata'] }),
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

function OrderingProbe() {
  const { isOrderingModels, stopModelOrdering } = useModels()
  return isOrderingModels ? (
    <ModelOrderEditor
      onSaved={stopModelOrdering}
      onCancel={stopModelOrdering}
    />
  ) : (
    <p>Page 1 of 1</p>
  )
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.put = originalPut
  dragStartEvents.splice(0)
  reorderItemDragValues.splice(0)
  reorderGroupProps = null
  document.body.replaceChildren()
})

after(() => domWindow.close())

describe('model order editor', () => {
  test('edit order enables row handles while normal mode has none', async () => {
    apiClient.get = async (url) =>
      url === '/api/models/order'
        ? response(models)
        : response({ items: vendors, total: vendors.length })
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    })
    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <ModelsProvider>
              <ModelsPrimaryButtons />
              <OrderingProbe />
            </ModelsProvider>
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    assert.equal(document.querySelector('[aria-label^="Drag model"]'), null)
    assert.match(document.body.textContent ?? '', /Page 1 of/)
    await click(findButton('Edit order'))
    await waitForText('model-a')
    assert.equal(findButton('Drag model-a to reorder').disabled, false)
    assert.doesNotMatch(document.body.textContent ?? '', /Page 1 of/)
    await act(async () => root.unmount())
  })

  test('loads the full order and supports ArrowUp and ArrowDown movement', async () => {
    apiClient.get = async (url) => {
      if (url === '/api/models/order') return response(models)
      return response({ items: vendors, total: vendors.length })
    }
    const rendered = await renderEditor()
    await waitForText('model-a')

    const firstHandle = findButton('Drag model-a to reorder')
    assert.equal(firstHandle.disabled, false)
    assert.match(rendered.container.textContent ?? '', /Vendor A/)
    assert.match(rendered.container.textContent ?? '', /Enabled/)
    assert.match(rendered.container.textContent ?? '', /Disabled/)

    await act(async () => {
      firstHandle.dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          key: 'ArrowDown',
          bubbles: true,
        }) as unknown as Event
      )
    })
    const orderedRows = [
      ...rendered.container.querySelectorAll('[data-model-order-item]'),
    ]
    assert.equal(orderedRows[0]?.getAttribute('data-model-id'), '2')

    const movedHandle = findButton('Drag model-a to reorder')
    await act(async () => {
      movedHandle.dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          key: 'ArrowUp',
          bubbles: true,
        }) as unknown as Event
      )
    })
    assert.equal(
      rendered.container
        .querySelector('[data-model-order-item]')
        ?.getAttribute('data-model-id'),
      '1'
    )
    await act(async () => rendered.root.unmount())
  })

  test('starts pointer dragging through Motion controls and reorders vertically', async () => {
    apiClient.get = async (url) =>
      url === '/api/models/order'
        ? response(models)
        : response({ items: vendors, total: vendors.length })
    const rendered = await renderEditor()
    await waitForText('model-a')

    const list = rendered.container.querySelector('[data-model-order-list]')
    assert.equal(list?.tagName, 'UL')
    assert.equal(list?.firstElementChild?.tagName, 'LI')
    await act(async () => {
      findButton('Drag model-a to reorder').dispatchEvent(
        new domWindow.PointerEvent('pointerdown', {
          bubbles: true,
        }) as unknown as Event
      )
    })
    assert.ok(list)
    assert.equal(dragStartEvents.length, 1)
    assert.equal(reorderGroupProps?.axis, 'y')
    assert.deepEqual(reorderItemDragValues, ['y', 'y'])
    const reorder = reorderGroupProps?.onReorder
    assert.ok(reorder, 'Expected the Reorder.Group callback')
    await act(async () => reorder([models[1], models[0]]))

    assert.equal(
      rendered.container
        .querySelector('[data-model-order-item]')
        ?.getAttribute('data-model-id'),
      '2'
    )
    await act(async () => rendered.root.unmount())
  })

  test('the routed models page replaces table controls and cleans ordering state on navigation', async () => {
    apiClient.get = async (url) => {
      if (url === '/api/models/order') return response(models)
      if (url === '/api/models/') {
        return response({
          items: models,
          total: models.length,
          page: 1,
          page_size: 20,
        })
      }
      return response({ items: vendors, total: vendors.length, page: 1 })
    }
    const rendered = await renderModelsPage()
    await waitForText('model-a')

    assert.ok(findButton('Add Model'))
    assert.ok(findButton('Edit order'))
    assert.match(rendered.container.textContent ?? '', /Rows per page/)
    await click(findButton('Edit order'))
    await waitForText('Edit model order')
    assert.equal(
      [...rendered.container.querySelectorAll('button')].some(
        (button) => button.textContent?.trim() === 'Add Model'
      ),
      false
    )
    assert.doesNotMatch(rendered.container.textContent ?? '', /Rows per page/)

    await click(findButton('Deployments'))
    await waitForText('Create deployment')
    await click(findButton('Metadata'))
    await waitForText('Edit order')
    assert.ok(findButton('Add Model'))
    assert.doesNotMatch(
      rendered.container.textContent ?? '',
      /Edit model order/
    )

    const saveRequest = deferred<{ data: unknown }>()
    apiClient.put = async () => saveRequest.promise
    await click(findButton('Edit order'))
    await waitForText('Edit model order')
    await click(findButton('Save'))
    await waitForText('Saving...')
    const deploymentsTab = findButton('Deployments')
    assert.match(deploymentsTab.outerHTML, /disabled|aria-disabled="true"/)
    await click(deploymentsTab)
    assert.match(rendered.container.textContent ?? '', /Edit model order/)
    await act(async () => {
      saveRequest.resolve({ data: { success: true } })
      await saveRequest.promise
    })
    await waitForText('Edit order')
    await act(async () => rendered.root.unmount())
  })

  test('saves the complete reordered IDs and invalidates order, model, and pricing caches', async () => {
    const saved: unknown[] = []
    let savedCount = 0
    apiClient.get = async (url) =>
      url === '/api/models/order'
        ? response(models)
        : response({ items: vendors, total: vendors.length })
    apiClient.put = async (_url, data) => {
      saved.push(data)
      return { data: { success: true } }
    }
    const rendered = await renderEditor({
      onSaved: () => {
        savedCount += 1
      },
    })
    const invalidated: unknown[] = []
    const originalInvalidate = rendered.queryClient.invalidateQueries.bind(
      rendered.queryClient
    )
    rendered.queryClient.invalidateQueries = (async (filters) => {
      invalidated.push(filters?.queryKey)
      return originalInvalidate(filters)
    }) as typeof rendered.queryClient.invalidateQueries
    await waitForText('model-a')
    await click(findButton('Move model-a down'))
    await click(findButton('Save'))

    assert.deepEqual(saved, [{ ordered_ids: [2, 1] }])
    await waitFor(() => savedCount === 1, 'Expected onSaved to be called')
    assert.deepEqual(invalidated, [
      [...modelsQueryKeys.all, 'order'],
      modelsQueryKeys.lists(),
      ['pricing'],
    ])
    await act(async () => rendered.root.unmount())
  })

  test('cancel does not request a save', async () => {
    let saveCount = 0
    apiClient.get = async (url) =>
      url === '/api/models/order'
        ? response(models)
        : response({ items: vendors, total: vendors.length })
    apiClient.put = async () => {
      saveCount += 1
      return { data: { success: true } }
    }
    let cancelCount = 0
    const rendered = await renderEditor({
      onCancel: () => {
        cancelCount += 1
      },
    })
    await waitForText('model-a')
    await click(findButton('Cancel'))
    assert.equal(saveCount, 0)
    assert.equal(cancelCount, 1)
    await act(async () => rendered.root.unmount())
  })

  test('locks cancellation and reordering while a save is in flight', async () => {
    const saveRequest = deferred<{ data: unknown }>()
    const savingStates: boolean[] = []
    let cancelCount = 0
    let savedCount = 0
    apiClient.get = async (url) =>
      url === '/api/models/order'
        ? response(models)
        : response({ items: vendors, total: vendors.length })
    apiClient.put = async () => saveRequest.promise
    const rendered = await renderEditor({
      onSaved: () => {
        savedCount += 1
      },
      onCancel: () => {
        cancelCount += 1
      },
      onSavingChange: (isSaving) => savingStates.push(isSaving),
    })
    await waitForText('model-a')
    await click(findButton('Save'))
    await waitForText('Saving...')

    assert.equal(findButton('Cancel').disabled, true)
    assert.equal(findButton('Drag model-a to reorder').disabled, true)
    assert.equal(findButton('Move model-a down').disabled, true)
    await click(findButton('Cancel'))
    assert.equal(cancelCount, 0)
    await act(async () => {
      findButton('Drag model-a to reorder').dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          key: 'ArrowDown',
          bubbles: true,
        }) as unknown as Event
      )
    })
    assert.equal(
      rendered.container
        .querySelector('[data-model-order-item]')
        ?.getAttribute('data-model-id'),
      '1'
    )

    await act(async () => {
      saveRequest.resolve({ data: { success: true } })
      await saveRequest.promise
    })
    await waitFor(() => savedCount === 1, 'Expected saved callback')
    assert.deepEqual(savingStates, [true, false])
    await act(async () => rendered.root.unmount())
  })

  test('retains the draft and shows an error when saving fails', async () => {
    apiClient.get = async (url) =>
      url === '/api/models/order'
        ? response(models)
        : response({ items: vendors, total: vendors.length })
    apiClient.put = async () => ({
      data: { success: false, message: 'Unable to save order' },
    })
    const rendered = await renderEditor()
    await waitForText('model-a')
    await click(findButton('Move model-a down'))
    await click(findButton('Save'))
    await waitForText('Unable to save order')
    const orderedRows = [
      ...rendered.container.querySelectorAll('[data-model-order-item]'),
    ]
    assert.equal(orderedRows[0]?.getAttribute('data-model-id'), '2')
    await act(async () => rendered.root.unmount())
  })

  test('renders loading, empty, and retryable error states', async () => {
    const request = deferred<ReturnType<typeof response>>()
    apiClient.get = async (url) => {
      if (url === '/api/models/order') return request.promise
      return response({ items: vendors, total: vendors.length })
    }
    const rendered = await renderEditor()
    assert.ok(rendered.container.querySelector('[aria-busy="true"]'))
    let sawEmptyState = false
    const observer = new domWindow.MutationObserver(() => {
      sawEmptyState ||= rendered.container.textContent?.includes(
        'No models available to order'
      )
    })
    ;(observer as unknown as MutationObserver).observe(rendered.container, {
      childList: true,
      subtree: true,
    })
    await act(async () => {
      request.resolve(response(models))
      await request.promise
    })
    await waitForText('model-a')
    observer.disconnect()
    assert.equal(sawEmptyState, false)
    await act(async () => rendered.root.unmount())

    apiClient.get = async (url) => {
      if (url === '/api/models/order') return response([])
      return response({ items: vendors, total: vendors.length })
    }
    const empty = await renderEditor()
    await waitForText('No models available to order')
    await act(async () => empty.root.unmount())

    let attempts = 0
    apiClient.get = async (url) => {
      if (url !== '/api/models/order') return response({ items: vendors })
      attempts += 1
      if (attempts === 1) throw new Error('Order service unavailable')
      return response([])
    }
    const failed = await renderEditor()
    await waitForText('Order service unavailable')
    await click(findButton('Retry'))
    await waitForText('No models available to order')
    assert.equal(attempts, 2)
    await act(async () => failed.root.unmount())
  })
})
