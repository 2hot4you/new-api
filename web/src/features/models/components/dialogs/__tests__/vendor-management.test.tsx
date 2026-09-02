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
import assert from 'node:assert/strict'
import { afterAll as after, afterEach, describe, test } from 'vitest'

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

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { ModelsDialogs } = await import('../../models-dialogs')
const { ModelsPrimaryButtons } = await import('../../models-primary-buttons')
const { ModelsProvider } = await import('../../models-provider')
const { VendorManagement } = await import('../vendor-management')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ApiClient = {
  get: (url: string) => Promise<{ data: unknown }>
  delete: (url: string) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as ApiClient
const originalGet = apiClient.get
const originalDelete = apiClient.delete

const deepSeekVendor: Vendor = {
  id: 6,
  name: 'DeepSeek',
  description: 'Open model provider',
  icon: 'DeepSeek.Color',
  status: 1,
  created_time: 1,
  updated_time: 1,
}

afterEach(() => {
  apiClient.get = originalGet
  apiClient.delete = originalDelete
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

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
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

function findMenuItem(name: string): Element {
  const item = [...document.querySelectorAll('[role="menuitem"]')].find(
    (candidate) => candidate.textContent?.includes(name)
  )
  assert.ok(item, `Expected menu item ${name}`)
  return item
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

async function renderManagement(items: Vendor[] | null = [deepSeekVendor]) {
  if (items) {
    apiClient.get = async () => vendorListResponse(items)
  }
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
          <VendorManagement open onOpenChange={() => undefined} />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })
  return { container, queryClient, root }
}

describe('vendor management', () => {
  test('the models menu opens vendor management instead of the create form', async () => {
    apiClient.get = async () => vendorListResponse([])
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <ModelsProvider>
              <ModelsPrimaryButtons />
              <ModelsDialogs />
            </ModelsProvider>
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    await click(findButton('More model actions'))
    await waitForText('Manage Vendors')
    await click(findMenuItem('Manage Vendors'))
    await waitForText('No vendors found')

    assert.match(document.body.textContent ?? '', /Manage Vendors/)
    assert.doesNotMatch(document.body.textContent ?? '', /Create Vendor/)
    await act(async () => root.unmount())
  })

  test('shows vendor details, the original icon key, and a rendered logo preview', async () => {
    const rendered = await renderManagement()
    await waitForText('DeepSeek.Color')

    assert.match(document.body.textContent ?? '', /Open model provider/)
    assert.ok(document.querySelector('[data-vendor-logo="DeepSeek.Color"]'))
    assert.ok(document.querySelector('svg'))
    await act(async () => rendered.root.unmount())
  })

  test('shows a loading state while the vendor request is pending', async () => {
    const request = deferred<ReturnType<typeof vendorListResponse>>()
    apiClient.get = async () => request.promise
    const rendered = await renderManagement(null)

    assert.ok(document.querySelector('[aria-busy="true"]'))
    await act(async () => {
      request.resolve(vendorListResponse([]))
      await request.promise
    })
    await waitForText('No vendors found')
    await act(async () => rendered.root.unmount())
  })

  test('shows an empty state when no vendors exist', async () => {
    const rendered = await renderManagement([])
    await waitForText('No vendors found')

    assert.match(
      document.body.textContent ?? '',
      /Add a vendor to start organizing marketplace models\./
    )
    await act(async () => rendered.root.unmount())
  })

  test('shows a retry action when loading vendors fails', async () => {
    let requestCount = 0
    apiClient.get = async () => {
      requestCount += 1
      if (requestCount === 1) throw new Error('Vendor service unavailable')
      return vendorListResponse([])
    }
    const rendered = await renderManagement(null)
    await waitForText('Vendor service unavailable')

    await click(findButton('Retry'))
    await waitForText('No vendors found')
    assert.equal(requestCount, 2)
    await act(async () => rendered.root.unmount())
  })

  test('add and edit forms return to the vendor list when cancelled', async () => {
    const rendered = await renderManagement()
    await waitForText('DeepSeek.Color')

    await click(findButton('Add Vendor'))
    await waitForText('Create Vendor')
    const createNameInput = document.querySelector(
      'input[name="name"]'
    ) as HTMLInputElement | null
    assert.equal(createNameInput?.value, '')
    await click(findButton('Cancel'))
    await waitForText('DeepSeek.Color')

    await click(findButton('Edit vendor DeepSeek'))
    await waitForText('Edit Vendor')
    const nameInput = document.querySelector(
      'input[name="name"]'
    ) as HTMLInputElement | null
    assert.equal(nameInput?.value, 'DeepSeek')
    await click(findButton('Cancel'))
    await waitForText('DeepSeek.Color')

    await act(async () => rendered.root.unmount())
  })

  test('passes the full OpenAI.Color key to the live form preview', async () => {
    const rendered = await renderManagement()
    await waitForText('DeepSeek.Color')
    await click(findButton('Edit vendor DeepSeek'))
    await waitForText('Edit Vendor')

    const iconInput = document.querySelector(
      'input[name="icon"]'
    ) as HTMLInputElement | null
    assert.equal(iconInput?.value, 'DeepSeek.Color')
    await act(async () => {
      assert.ok(iconInput)
      const setInputValue = Object.getOwnPropertyDescriptor(
        domWindow.HTMLInputElement.prototype,
        'value'
      )?.set
      assert.ok(setInputValue)
      setInputValue.call(iconInput, 'OpenAI.Color')
      iconInput.dispatchEvent(
        new domWindow.Event('input', { bubbles: true }) as unknown as Event
      )
    })
    assert.equal(iconInput?.value, 'OpenAI.Color')
    assert.ok(
      document.querySelector('[data-vendor-icon-preview="OpenAI.Color"]')
    )
    await act(async () => rendered.root.unmount())
  })

  test('deletes only after confirmation and refreshes the vendor list', async () => {
    let listRequests = 0
    let deleteRequests = 0
    apiClient.get = async () => {
      listRequests += 1
      return vendorListResponse(listRequests === 1 ? [deepSeekVendor] : [])
    }
    apiClient.delete = async () => {
      deleteRequests += 1
      return { data: { success: true } }
    }
    const rendered = await renderManagement(null)
    await waitForText('DeepSeek.Color')

    await click(findButton('Delete vendor DeepSeek'))
    await waitForText(
      'Are you sure you want to delete vendor "DeepSeek"? This action cannot be undone.'
    )
    assert.equal(deleteRequests, 0)
    await click(findButton('Delete'))
    await waitForText('No vendors found')

    assert.equal(deleteRequests, 1)
    assert.equal(listRequests >= 2, true)
    await act(async () => rendered.root.unmount())
  })

  test('keeps the confirmation open and shows the backend error when deletion fails', async () => {
    apiClient.delete = async () => ({
      data: { success: false, message: 'Vendor is still in use' },
    })
    const rendered = await renderManagement()
    await waitForText('DeepSeek.Color')

    await click(findButton('Delete vendor DeepSeek'))
    await click(findButton('Delete'))
    await waitForText('Vendor is still in use')

    assert.match(
      document.body.textContent ?? '',
      /Are you sure you want to delete vendor "DeepSeek"/
    )
    await act(async () => rendered.root.unmount())
  })

  test('clears the selected vendor after the management flow is closed', async () => {
    apiClient.get = async () => vendorListResponse([deepSeekVendor])
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const renderFlow = async (open: boolean) => {
      await act(async () => {
        root.render(
          <QueryClientProvider client={queryClient}>
            <I18nextProvider i18n={i18n}>
              <VendorManagement open={open} onOpenChange={() => undefined} />
            </I18nextProvider>
          </QueryClientProvider>
        )
      })
    }

    await renderFlow(true)
    await waitForText('DeepSeek.Color')
    await click(findButton('Edit vendor DeepSeek'))
    await waitForText('Edit Vendor')
    await renderFlow(false)
    await renderFlow(true)
    await waitForText('DeepSeek.Color')

    assert.doesNotMatch(document.body.textContent ?? '', /Edit Vendor/)
    await act(async () => root.unmount())
  })
})
