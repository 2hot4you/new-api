/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import assert from 'node:assert/strict'
import { after, afterEach, describe, test } from 'node:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Window } from 'happy-dom'

import type { Vendor } from '../../../types'

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

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { modelsQueryKeys } = await import('../../../lib')
const { VendorManagement } = await import('../vendor-management')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiClient = {
  get: (url: string, config?: unknown) => Promise<{ data: unknown }>
  put: (url: string, data: unknown) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get
const originalPut = apiClient.put

const vendorA: Vendor = {
  id: 1,
  name: 'Vendor A',
  description: 'First vendor',
  icon: 'OpenAI.Color',
  status: 1,
  created_time: 1,
  updated_time: 1,
}

const vendorB: Vendor = {
  id: 2,
  name: 'Vendor B',
  description: 'Second vendor',
  icon: 'Anthropic.Color',
  status: 1,
  created_time: 1,
  updated_time: 1,
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.put = originalPut
  document.body.replaceChildren()
})

after(() => domWindow.close())

function vendorListResponse(items: Vendor[]) {
  return {
    data: {
      success: true,
      data: { items, total: items.length, page: 1, page_size: 1000 },
    },
  }
}

function vendorOrderResponse(items: Vendor[]) {
  return {
    data: {
      success: true,
      data: items,
    },
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function findButton(name: string): HTMLButtonElement {
  const button = [...document.querySelectorAll('button')].find(
    (candidate) =>
      candidate.textContent?.trim() === name ||
      candidate.getAttribute('aria-label') === name
  ) as HTMLButtonElement | undefined
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

async function waitForButton(name: string): Promise<HTMLButtonElement> {
  const deadline = Date.now() + 1500
  while (Date.now() < deadline) {
    const button = [...document.querySelectorAll('button')].find(
      (candidate) => candidate.getAttribute('aria-label') === name
    ) as HTMLButtonElement | undefined
    if (button) return button
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 10))
    })
  }
  throw new Error(`Expected button ${name}: ${document.body.textContent}`)
}

function visibleVendorNames(): string[] {
  return [...document.querySelectorAll('[data-vendor-name]')].map(
    (element) => element.textContent?.trim() ?? ''
  )
}

async function renderManagement(props?: {
  items?: Vendor[]
  orderedItems?: Vendor[]
}) {
  const items = props?.items ?? [vendorA, vendorB]
  const orderedItems = props?.orderedItems ?? items
  const getCalls: string[] = []
  apiClient.get = async (url) => {
    getCalls.push(url)
    if (url === '/api/vendors/order') {
      return vendorOrderResponse(orderedItems)
    }
    return vendorListResponse(items)
  }
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })

  function Harness() {
    const [open, setOpen] = useState(true)
    return (
      <>
        <button type='button' onClick={() => setOpen(true)}>
          Reopen vendor manager
        </button>
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <VendorManagement open={open} onOpenChange={setOpen} />
          </I18nextProvider>
        </QueryClientProvider>
      </>
    )
  }

  await act(async () => root.render(<Harness />))
  await waitForText(items[0]?.name ?? 'No vendors found')
  return { getCalls, queryClient, root }
}

async function enterOrderModeAndMoveSecondVendorFirst() {
  await click(findButton('Reorder Vendors'))
  await waitForButton('Drag Vendor B to reorder')
  await click(findButton('Move Vendor B up'))
  assert.deepEqual(visibleVendorNames(), ['Vendor B', 'Vendor A'])
}

describe('vendor ordering', () => {
  test('activates order mode with accessible drag handles and movement controls', async () => {
    const rendered = await renderManagement()

    await click(findButton('Reorder Vendors'))
    await waitForText('Save order')

    assert.equal(findButton('Drag Vendor A to reorder').type, 'button')
    assert.equal(findButton('Move Vendor A up').disabled, true)
    assert.equal(findButton('Move Vendor B down').disabled, true)
    assert.equal(findButton('Add Vendor').disabled, true)
    assert.equal(
      [...document.querySelectorAll('button')].some(
        (button) => button.getAttribute('aria-label') === 'Edit vendor Vendor A'
      ),
      false
    )

    await act(async () => {
      findButton('Drag Vendor B to reorder').dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          key: 'ArrowUp',
          bubbles: true,
        }) as unknown as KeyboardEvent
      )
    })
    assert.deepEqual(visibleVendorNames(), ['Vendor B', 'Vendor A'])

    await click(findButton('Move Vendor A up'))
    assert.deepEqual(visibleVendorNames(), ['Vendor A', 'Vendor B'])
    await act(async () => rendered.root.unmount())
  })

  test('loads the persisted complete order from the dedicated endpoint', async () => {
    const vendorC = { ...vendorA, id: 3, name: 'Vendor C' }
    const rendered = await renderManagement({
      items: [vendorA, vendorB],
      orderedItems: [vendorB, vendorC, vendorA],
    })

    await click(findButton('Reorder Vendors'))
    await waitForButton('Drag Vendor C to reorder')

    assert.deepEqual(visibleVendorNames(), ['Vendor B', 'Vendor C', 'Vendor A'])
    assert.equal(rendered.getCalls.includes('/api/vendors/order'), true)
    await act(async () => rendered.root.unmount())
  })

  test('does not truncate the persisted order to the management list page', async () => {
    const completeOrder = Array.from({ length: 101 }, (_, index) => ({
      ...vendorA,
      id: index + 1,
      name: `Vendor ${index + 1}`,
    }))
    const rendered = await renderManagement({
      items: completeOrder.slice(0, 2),
      orderedItems: completeOrder,
    })

    await click(findButton('Reorder Vendors'))
    await waitForButton('Drag Vendor 101 to reorder')

    assert.equal(visibleVendorNames().length, 101)
    await act(async () => rendered.root.unmount())
  })

  test('saves the complete reordered ID sequence and invalidates marketplace queries', async () => {
    const savedOrders: unknown[] = []
    apiClient.put = async (url, data) => {
      assert.equal(url, '/api/vendors/order')
      savedOrders.push(data)
      return { data: { success: true } }
    }
    const rendered = await renderManagement()
    rendered.queryClient.setQueryData(modelsQueryKeys.list({}), ['model'])
    rendered.queryClient.setQueryData(['pricing'], ['pricing'])

    await enterOrderModeAndMoveSecondVendorFirst()
    await click(findButton('Save order'))
    await waitForText('Reorder Vendors')

    assert.deepEqual(savedOrders, [{ ordered_ids: [2, 1] }])
    assert.equal(
      rendered.queryClient.getQueryCache().find({
        queryKey: modelsQueryKeys.list({}),
      })?.state.isInvalidated,
      true
    )
    assert.equal(
      rendered.queryClient.getQueryCache().find({ queryKey: ['pricing'] })
        ?.state.isInvalidated,
      true
    )
    await act(async () => rendered.root.unmount())
  })

  test('keeps the reordered draft and error visible when saving fails', async () => {
    apiClient.put = async () => ({
      data: { success: false, message: 'Order is stale' },
    })
    const rendered = await renderManagement()

    await enterOrderModeAndMoveSecondVendorFirst()
    await click(findButton('Save order'))
    await waitForText('Order is stale')

    assert.deepEqual(visibleVendorNames(), ['Vendor B', 'Vendor A'])
    assert.ok(findButton('Save order'))
    await act(async () => rendered.root.unmount())
  })

  test('locks ordering and keeps the dialog open while a save is pending', async () => {
    const request = deferred<{ data: unknown }>()
    apiClient.put = async () => request.promise
    const rendered = await renderManagement()

    await enterOrderModeAndMoveSecondVendorFirst()
    await click(findButton('Save order'))
    await waitForText('Saving...')

    assert.equal(findButton('Drag Vendor B to reorder').disabled, true)
    assert.equal(findButton('Move Vendor B down').disabled, true)
    assert.equal(findButton('Cancel').disabled, true)
    await click(findButton('Move Vendor B down'))
    await act(async () => {
      findButton('Drag Vendor B to reorder').dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          key: 'ArrowDown',
          bubbles: true,
        }) as unknown as KeyboardEvent
      )
    })
    await click(findButton('Close'))

    assert.deepEqual(visibleVendorNames(), ['Vendor B', 'Vendor A'])
    assert.ok(findButton('Saving...'))
    await act(async () => {
      request.resolve({ data: { success: true } })
      await request.promise
    })
    await waitForText('Reorder Vendors')
    await act(async () => rendered.root.unmount())
  })

  test('cancelling discards the unsaved vendor order', async () => {
    const rendered = await renderManagement()

    await enterOrderModeAndMoveSecondVendorFirst()
    await click(findButton('Cancel'))
    await waitForText('Reorder Vendors')

    assert.deepEqual(visibleVendorNames(), ['Vendor A', 'Vendor B'])
    await act(async () => rendered.root.unmount())
  })

  test('closing and reopening discards unsaved vendor order', async () => {
    const rendered = await renderManagement()

    await enterOrderModeAndMoveSecondVendorFirst()
    await click(findButton('Close'))
    await click(findButton('Reopen vendor manager'))
    await waitForText('Reorder Vendors')

    assert.deepEqual(visibleVendorNames(), ['Vendor A', 'Vendor B'])
    await act(async () => rendered.root.unmount())
  })
})
