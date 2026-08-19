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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

import type { UsageLog } from '../../../data/schema'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
const testDocument = domWindow.document
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
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { notifyManager } = await import('@tanstack/query-core')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { getGrokImagePreview } = await import('../../../api')
const { ImageDialog } = await import('../image-dialog')
const { GrokImagePreviewCard } = await import('../grok-image-preview-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true
notifyManager.setNotifyFunction((callback) => act(callback))

const baseLog: UsageLog = {
  id: 1,
  user_id: 7,
  created_at: 1,
  type: 2,
  content: '',
  username: '',
  token_name: '',
  model_name: 'grok-imagine-image-2.0',
  quota: 1,
  prompt_tokens: 1,
  completion_tokens: 0,
  use_time: 1,
  is_stream: false,
  channel: 1,
  channel_name: '',
  token_id: 1,
  group: '',
  ip: '',
  other: '',
  request_id: 'reqABC123',
  upstream_request_id: '',
}

type PreviewApi = {
  get: (
    url: string,
    config?: { skipBusinessError?: boolean; skipErrorHandler?: boolean }
  ) => Promise<{ data: unknown }>
}

const apiClient = api as unknown as PreviewApi
const originalGet = apiClient.get

afterEach(() => {
  apiClient.get = originalGet
})

after(() => domWindow.close())

async function renderCard(
  log: UsageLog,
  withQueryClient = true,
  cachedData?: unknown
) {
  const container = testDocument.createElement('div')
  testDocument.body.append(container)
  const root = createRoot(
    container as unknown as Parameters<typeof createRoot>[0]
  )
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  if (cachedData !== undefined) {
    queryClient.setQueryData(
      ['grok-image-preview', log.user_id, log.request_id],
      cachedData
    )
  }
  await act(async () => {
    const content = (
      <I18nextProvider i18n={i18n}>
        <GrokImagePreviewCard log={log} quotaPerUnit={500000} />
      </I18nextProvider>
    )
    root.render(
      withQueryClient ? (
        <QueryClientProvider client={queryClient}>
          {content}
        </QueryClientProvider>
      ) : (
        content
      )
    )
  })
  return { container, queryClient, root }
}

async function renderImageDialog() {
  const container = testDocument.createElement('div')
  testDocument.body.append(container)
  const root = createRoot(
    container as unknown as Parameters<typeof createRoot>[0]
  )
  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <ImageDialog
          imageUrl='https://example.com/midjourney-image.png'
          taskId='midjourney-task'
          open
          onOpenChange={() => undefined}
        />
      </I18nextProvider>
    )
  })
  return { container, root }
}

function deferred<T>() {
  let reject!: (reason?: unknown) => void
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, reject, resolve }
}

function axiosError(status: number, rawMessage: string) {
  const error = new Error(rawMessage) as Error & {
    isAxiosError: boolean
    response: { status: number; data: { message: string } }
  }
  error.isAxiosError = true
  error.response = { status, data: { message: rawMessage } }
  return error
}

async function waitForCondition(
  condition: () => boolean,
  failureMessage: string
): Promise<void> {
  const deadline = Date.now() + 1500
  while (Date.now() < deadline) {
    if (condition()) return
    await new Promise((resolve) => setTimeout(resolve, 10))
  }
  throw new Error(`${failureMessage}: ${testDocument.body.textContent}`)
}

type PreviewResponse = {
  data: { success: boolean; data: { urls: string[] } }
}

async function renderOpenPreviewWithPendingRefetch() {
  const initial = deferred<PreviewResponse>()
  const refetch = deferred<PreviewResponse>()
  let requestCount = 0
  apiClient.get = async () => {
    requestCount += 1
    return requestCount === 1 ? initial.promise : refetch.promise
  }
  const log = {
    ...baseLog,
    other: JSON.stringify({ grok_image_preview_available: true }),
  }
  const rendered = await renderCard(log)
  const oldUrl = 'https://imgen.x.ai/open-before-refetch'
  await act(async () => {
    initial.resolve({ data: { success: true, data: { urls: [oldUrl] } } })
    await initial.promise
  })
  await waitForCondition(
    () => rendered.container.querySelectorAll('img').length === 1,
    'Expected the initial preview image'
  )

  const previewButton = rendered.container.querySelector('button')
  assert.ok(previewButton)
  await act(async () => {
    previewButton.dispatchEvent(
      new domWindow.MouseEvent('click', {
        bubbles: true,
      }) as unknown as Parameters<typeof previewButton.dispatchEvent>[0]
    )
  })
  assert.equal(
    [...testDocument.querySelectorAll('img')].filter(
      (image) => image.getAttribute('src') === oldUrl
    ).length,
    2
  )

  await act(async () => {
    void rendered.queryClient.invalidateQueries({
      queryKey: ['grok-image-preview', log.user_id, log.request_id],
    })
  })
  await waitForCondition(
    () => rendered.container.querySelector('[aria-busy="true"]') !== null,
    'Expected preview refetch to start'
  )

  return { log, oldUrl, refetch, rendered }
}

describe('Grok image preview card', () => {
  test('encodes both authenticated preview path parameters', async () => {
    const requestedPaths: string[] = []
    const requestConfigs: Array<{
      skipBusinessError?: boolean
      skipErrorHandler?: boolean
    }> = []
    apiClient.get = async (path, config) => {
      requestedPaths.push(path)
      requestConfigs.push(config ?? {})
      return { data: { success: true, data: { urls: [] } } }
    }

    await getGrokImagePreview(7, 'reqABC123')

    assert.deepEqual(requestedPaths, [
      '/api/log/grok-image-preview/7/reqABC123',
    ])
    assert.deepEqual(requestConfigs, [
      { skipBusinessError: true, skipErrorHandler: true },
    ])
  })

  test('does not request or render a preview when the log has no availability flag', async () => {
    const requestedPaths: string[] = []
    apiClient.get = async (path) => {
      requestedPaths.push(path)
      return { data: { success: true, data: { urls: [] } } }
    }

    const rendered = await renderCard(baseLog, false)

    assert.deepEqual(requestedPaths, [])
    assert.equal(rendered.container.textContent, '')

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('renders the selected image, parameters, billing, and temporary-result download action in a two-column panel', async () => {
    const url = 'https://imgen.x.ai/result.png?token=private'
    apiClient.get = async () => ({
      data: { success: true, data: { urls: [url] } },
    })
    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({
        grok_image_preview_available: true,
        grok_image_billing: {
          version: 1,
          model: 'grok-imagine-image-2.0',
          operation: 'generation',
          resolution: '2k',
          quality: 'medium',
          aspect_ratio: '16:9',
          requested_output_count: 1,
          output_count: 1,
          input_image_count: 0,
          output_unit_price: 0.06,
          input_unit_price: 0,
          output_cost: 0.06,
          input_cost: 0,
          subtotal: 0.06,
          group_ratio: 1,
          final_cost: 0.06,
        },
      }),
    })
    await waitForCondition(
      () => rendered.container.querySelector('[data-grok-image-main]') !== null,
      'Expected the large selected image'
    )

    const panel = rendered.container.querySelector('[data-grok-image-layout]')
    const download = rendered.container.querySelector(
      'a[data-grok-image-download]'
    ) as HTMLAnchorElement | null
    const text = rendered.container.textContent ?? ''
    assert.ok(panel)
    assert.ok(panel.className.includes('lg:grid-cols'))
    assert.equal(
      download?.getAttribute('href'),
      'https://imgen.x.ai/result.png?token=private'
    )
    assert.equal(download?.getAttribute('target'), '_blank')
    assert.equal(download?.getAttribute('referrerpolicy'), 'no-referrer')
    for (const expected of [
      'grok-imagine-image-2.0',
      '2K',
      '16:9',
      'MEDIUM',
      'Billing Formula',
      'Final Charge',
    ]) {
      assert.equal(text.includes(expected), true, expected)
    }

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('shows the temporary-link warning with up to four no-referrer images', async () => {
    const urls = [
      'https://imgen.x.ai/a',
      'https://imgen.x.ai/b',
      'https://imgen.x.ai/c',
      'https://imgen.x.ai/d',
      'https://imgen.x.ai/e',
    ]
    const response = deferred<{
      data: { success: boolean; data: { urls: string[] } }
    }>()
    let requestStarted = false
    apiClient.get = async () => {
      requestStarted = true
      return response.promise
    }

    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    })
    assert.ok(rendered.container.querySelector('[aria-busy="true"]'))
    assert.equal(requestStarted, true)
    await act(async () => {
      response.resolve({ data: { success: true, data: { urls } } })
      await response.promise
    })
    await waitForCondition(
      () => rendered.container.querySelectorAll('img').length === 5,
      'Expected the preview gallery to render four images'
    )

    const alert = rendered.container.querySelector('[role="alert"]')
    const images = rendered.container.querySelectorAll('img')
    assert.ok(alert)
    assert.equal(images.length, 5)
    for (const image of images) {
      assert.equal(image.referrerPolicy, 'no-referrer')
    }

    const firstPreview = images[0].closest('button')
    assert.ok(firstPreview)
    await act(async () => {
      firstPreview.dispatchEvent(
        new domWindow.MouseEvent('click', {
          bubbles: true,
        }) as unknown as Parameters<typeof firstPreview.dispatchEvent>[0]
      )
    })
    const selectedImages = [...testDocument.querySelectorAll('img')].filter(
      (image) => image.getAttribute('src') === urls[0]
    )
    assert.equal(selectedImages.length, 3)
    for (const image of selectedImages) {
      assert.equal(image.referrerPolicy, 'no-referrer')
    }

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('shows a safe expired state when no temporary links remain', async () => {
    const response = deferred<{
      data: { success: boolean; data: { urls: string[] } }
    }>()
    apiClient.get = async () => response.promise

    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    })
    await act(async () => {
      response.resolve({ data: { success: true, data: { urls: [] } } })
      await response.promise
    })
    await waitForCondition(
      () =>
        (rendered.container.textContent ?? '').includes(
          'Image preview expired'
        ),
      'Expected the expired preview state'
    )

    assert.equal(
      (rendered.container.textContent ?? '').includes('Image preview expired'),
      true
    )

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('maps an Axios 404 to the safe expired state without exposing its body', async () => {
    const rawTemporaryUrl = 'https://imgen.x.ai/private-expired-result'
    apiClient.get = async () => {
      throw axiosError(404, rawTemporaryUrl)
    }

    const preview = await getGrokImagePreview(7, 'reqABC123')
    assert.deepEqual(preview, { success: false, expired: true })

    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    })
    await waitForCondition(
      () =>
        (rendered.container.textContent ?? '').includes(
          'Image preview expired'
        ),
      'Expected the Axios 404 preview state to be expired'
    )

    const text = rendered.container.textContent ?? ''
    assert.equal(text.includes('Image preview expired'), true)
    assert.equal(text.includes('Image preview is unavailable'), false)
    assert.equal(text.includes('private-expired-result'), false)
    assert.equal(rendered.container.querySelectorAll('img').length, 0)

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('does not reveal Axios 500 response error content when the preview query fails', async () => {
    const response = deferred<never>()
    apiClient.get = async () => response.promise

    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    })
    await act(async () => {
      response.reject(
        axiosError(500, 'https://imgen.x.ai/private-temporary-result')
      )
      try {
        await response.promise
      } catch {
        // React Query converts the rejected request into the component state.
      }
    })
    await waitForCondition(
      () =>
        (rendered.container.textContent ?? '').includes(
          'Image preview is unavailable'
        ),
      'Expected the unavailable preview state'
    )

    const text = rendered.container.textContent ?? ''
    assert.equal(text.includes('Image preview is unavailable'), true)
    assert.equal(text.includes('private-temporary-result'), false)

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('does not render URLs from an unsuccessful preview response', async () => {
    const response = deferred<{
      data: {
        success: boolean
        message: string
        data: { urls: string[] }
      }
    }>()
    apiClient.get = async () => response.promise

    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    })
    await act(async () => {
      response.resolve({
        data: {
          success: false,
          message: 'https://imgen.x.ai/private-temporary-result',
          data: { urls: ['https://imgen.x.ai/private-temporary-result'] },
        },
      })
      await response.promise
    })
    await waitForCondition(
      () =>
        (rendered.container.textContent ?? '').includes(
          'Image preview is unavailable'
        ),
      'Expected the unavailable preview state'
    )

    const text = rendered.container.textContent ?? ''
    assert.equal(text.includes('private-temporary-result'), false)
    assert.equal(rendered.container.querySelectorAll('img').length, 0)

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('treats a successful response without a URL envelope as expired', async () => {
    const response = deferred<{ data: { success: boolean } }>()
    apiClient.get = async () => response.promise

    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    })
    await act(async () => {
      response.resolve({ data: { success: true } })
      await response.promise
    })
    await waitForCondition(
      () =>
        (rendered.container.textContent ?? '').includes(
          'Image preview expired'
        ),
      'Expected the missing URL envelope to be treated as expired'
    )

    assert.equal(rendered.container.querySelectorAll('img').length, 0)

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('closes the selected preview when the log identity changes', async () => {
    const response = deferred<{
      data: { success: boolean; data: { urls: string[] } }
    }>()
    apiClient.get = async () => response.promise
    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    })
    await act(async () => {
      response.resolve({
        data: { success: true, data: { urls: ['https://imgen.x.ai/old'] } },
      })
      await response.promise
    })
    await waitForCondition(
      () => rendered.container.querySelectorAll('img').length === 1,
      'Expected the initial preview image'
    )
    const previewButton = rendered.container.querySelector('button')
    assert.ok(previewButton)
    await act(async () => {
      previewButton.dispatchEvent(
        new domWindow.MouseEvent('click', {
          bubbles: true,
        }) as unknown as Parameters<typeof previewButton.dispatchEvent>[0]
      )
    })

    await act(async () => {
      rendered.root.render(
        <QueryClientProvider client={rendered.queryClient}>
          <I18nextProvider i18n={i18n}>
            <GrokImagePreviewCard
              quotaPerUnit={500000}
              log={{
                ...baseLog,
                user_id: 8,
                request_id: 'a-new-request',
                other: JSON.stringify({ grok_image_preview_available: true }),
              }}
            />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    assert.equal(
      [...testDocument.querySelectorAll('img')].some(
        (image) => image.getAttribute('src') === 'https://imgen.x.ai/old'
      ),
      false
    )

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('configures inactive preview cache retention for one minute', async () => {
    const response = deferred<never>()
    apiClient.get = async () => response.promise
    const log = {
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    }
    const rendered = await renderCard(log)

    const previewQuery = rendered.queryClient.getQueryCache().find({
      queryKey: ['grok-image-preview', log.user_id, log.request_id],
    })
    assert.equal(previewQuery?.options.gcTime, 60_000)

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('does not show cached temporary URLs while the preview is refetching', async () => {
    const response = deferred<{
      data: { success: boolean; data: { urls: string[] } }
    }>()
    apiClient.get = async () => response.promise
    const log = {
      ...baseLog,
      other: JSON.stringify({ grok_image_preview_available: true }),
    }
    const rendered = await renderCard(log, true, {
      success: true,
      data: { urls: ['https://imgen.x.ai/cached-temporary-result'] },
    })

    assert.ok(rendered.container.querySelector('[aria-busy="true"]'))
    assert.equal(rendered.container.querySelectorAll('img').length, 0)

    await act(async () => {
      response.resolve({
        data: {
          success: true,
          data: { urls: ['https://imgen.x.ai/refreshed-temporary-result'] },
        },
      })
      await response.promise
    })
    await waitForCondition(
      () => rendered.container.querySelectorAll('img').length === 1,
      'Expected the refreshed preview image'
    )

    const image = rendered.container.querySelector('img')
    assert.equal(
      image?.getAttribute('src'),
      'https://imgen.x.ai/refreshed-temporary-result'
    )

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('closes an open image dialog as soon as its preview refetch starts', async () => {
    const { oldUrl, refetch, rendered } =
      await renderOpenPreviewWithPendingRefetch()
    try {
      assert.equal(
        [...testDocument.querySelectorAll('img')].some(
          (image) => image.getAttribute('src') === oldUrl
        ),
        false
      )
    } finally {
      refetch.resolve({
        data: {
          success: true,
          data: { urls: ['https://imgen.x.ai/refreshed-after-dialog'] },
        },
      })
      await act(async () => rendered.root.unmount())
      rendered.container.remove()
    }
  })

  test('keeps an open image dialog closed after a refetch returns Axios 404', async () => {
    const { oldUrl, refetch, rendered } =
      await renderOpenPreviewWithPendingRefetch()
    try {
      await act(async () => {
        refetch.reject(axiosError(404, oldUrl))
        try {
          await refetch.promise
        } catch {
          // The preview helper maps Axios 404 to its safe expired result.
        }
      })
      await waitForCondition(
        () =>
          (rendered.container.textContent ?? '').includes(
            'Image preview expired'
          ),
        'Expected the expired refetch state'
      )

      assert.equal(
        [...testDocument.querySelectorAll('img')].some(
          (image) => image.getAttribute('src') === oldUrl
        ),
        false
      )
    } finally {
      await act(async () => rendered.root.unmount())
      rendered.container.remove()
    }
  })

  test('keeps an open image dialog closed after a refetch returns Axios 500', async () => {
    const { oldUrl, refetch, rendered } =
      await renderOpenPreviewWithPendingRefetch()
    try {
      await act(async () => {
        refetch.reject(axiosError(500, oldUrl))
        try {
          await refetch.promise
        } catch {
          // React Query retains the failed request state for the fixed error UI.
        }
      })
      await waitForCondition(
        () =>
          (rendered.container.textContent ?? '').includes(
            'Image preview is unavailable'
          ),
        'Expected the unavailable refetch state'
      )

      const text = rendered.container.textContent ?? ''
      assert.equal(text.includes(oldUrl), false)
      assert.equal(
        [...testDocument.querySelectorAll('img')].some(
          (image) => image.getAttribute('src') === oldUrl
        ),
        false
      )
    } finally {
      await act(async () => rendered.root.unmount())
      rendered.container.remove()
    }
  })

  test('keeps the existing task image dialog preview safe with no referrer', async () => {
    const rendered = await renderImageDialog()
    const image = [...testDocument.querySelectorAll('img')].find(
      (candidate) =>
        candidate.getAttribute('src') ===
        'https://example.com/midjourney-image.png'
    )
    assert.ok(image)
    assert.equal(image.referrerPolicy, 'no-referrer')

    await act(async () => {
      image.dispatchEvent(
        new domWindow.Event('load') as unknown as Parameters<
          typeof image.dispatchEvent
        >[0]
      )
      image.dispatchEvent(
        new domWindow.Event('error') as unknown as Parameters<
          typeof image.dispatchEvent
        >[0]
      )
    })

    const text = testDocument.body.textContent ?? ''
    assert.equal(text.includes('midjourney-task'), true)
    assert.equal(text.includes('Failed to load image'), true)

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })
})
