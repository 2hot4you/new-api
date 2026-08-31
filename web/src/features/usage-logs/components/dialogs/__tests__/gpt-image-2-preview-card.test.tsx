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
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

import type { UsageLog } from '../../../data/schema'

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

const react = await import('react')
Object.defineProperty(globalThis, 'React', {
  configurable: true,
  value: react,
})
const { act } = react
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { api } = await import('@/lib/api')
const { DetailsDialog } = await import('../details-dialog')
const { GPTImage2PreviewCard } = await import('../gpt-image-2-preview-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'zh',
  resources: {
    zh: {
      translation: {
        'Model ID': '模型 ID',
        Operation: '操作',
        Quality: '质量',
        Background: '背景',
        'Output Format': '输出格式',
        Moderation: '内容审核',
        Size: '尺寸',
        User: '用户',
        'Requested Outputs': '请求输出数',
        'Actual Outputs': '实际输出数',
        'Total Duration': '总耗时',
      },
    },
  },
})
;(
  globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }
).IS_REACT_ACT_ENVIRONMENT = true

const log: UsageLog = {
  id: 1,
  user_id: 7,
  created_at: 1,
  type: 2,
  content: '',
  username: '',
  token_name: '',
  model_name: 'gpt-image-2',
  quota: 576,
  prompt_tokens: 120,
  completion_tokens: 300,
  use_time: 12.5,
  is_stream: false,
  channel: 1,
  channel_name: '',
  token_id: 1,
  group: '',
  ip: '',
  request_id: 'req-image-2',
  upstream_request_id: '',
  other: JSON.stringify({
    gpt_image_2_preview_available: true,
    model_ratio: 1,
    completion_ratio: 2,
    group_ratio: 0.8,
    user_group_ratio: -1,
    gpt_image_2: {
      version: 1,
      model: 'gpt-image-2',
      operation: 'generation',
      quality: 'high',
      background: 'transparent',
      output_format: 'webp',
      moderation: 'low',
      size: '1536x1024',
      user: 'customer-42',
      requested_output_count: 3,
      output_count: 2,
    },
  }),
}

after(() => domWindow.close())

test('renders adaptive bilingual parameters, billing, and an authenticated download', async () => {
  const apiCalls: Array<{
    url: string
    config?: Record<string, unknown>
  }> = []
  const apiClient = api as unknown as {
    get: (
      url: string,
      config?: Record<string, unknown>
    ) => Promise<{ data: unknown }>
  }
  const originalGet = apiClient.get
  apiClient.get = async (url, config) => {
    apiCalls.push({ url, config })
    if (url.includes('/download/')) {
      return { data: new Blob(['image'], { type: 'image/webp' }) }
    }
    return {
      data: {
        success: true,
        data: {
          urls: [
            'https://cos.example/one.webp',
            'https://cos.example/two.webp',
          ],
        },
      },
    }
  }
  const container = domWindow.document.createElement('div')
  domWindow.document.body.append(container)
  const root = createRoot(
    container as unknown as Parameters<typeof createRoot>[0]
  )
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <GPTImage2PreviewCard log={log} quotaPerUnit={500000} />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })

  const text = container.textContent ?? ''
  for (const expected of [
    'GPT Image 2 Preview',
    '24 hours',
    'Model ID（模型 ID）',
    'Operation（操作）',
    'Quality（质量）',
    'high',
    'Background（背景）',
    'transparent',
    'Output Format（输出格式）',
    'WEBP',
    'Moderation（内容审核）',
    'low',
    'Size（尺寸）',
    '1536x1024',
    'User（用户）',
    'customer-42',
    'Requested Outputs（请求输出数）',
    'Actual Outputs（实际输出数）',
    'Total Duration（总耗时）',
    'Billing Rules and Formula',
    'Input Tokens',
    '120',
    'Output Tokens',
    '300',
    '0.8000x',
    '20.00%',
  ]) {
    assert.match(text, new RegExp(expected))
  }
  assert.equal(container.querySelectorAll('img').length, 3)

  const mainImage = container.querySelector(
    '[data-gpt-image-2-main]'
  ) as unknown as HTMLImageElement | null
  assert.ok(mainImage)
  Object.defineProperty(mainImage, 'naturalWidth', { value: 1024 })
  Object.defineProperty(mainImage, 'naturalHeight', { value: 1536 })
  await act(async () =>
    mainImage.dispatchEvent(new domWindow.Event('load') as unknown as Event)
  )
  assert.equal(
    container
      .querySelector('[data-gpt-image-2-layout]')
      ?.getAttribute('data-media-orientation'),
    'portrait'
  )

  const download = container.querySelector(
    '[data-gpt-image-2-download]'
  ) as unknown as HTMLButtonElement | null
  assert.ok(download)
  assert.equal(download.tagName, 'BUTTON')
  await act(async () => {
    download.dispatchEvent(
      new domWindow.MouseEvent('click', { bubbles: true }) as unknown as Event
    )
    await new Promise((resolve) => setTimeout(resolve, 0))
  })
  const downloadCall = apiCalls.find(({ url }) => url.includes('/download/'))
  assert.deepEqual(downloadCall, {
    url: '/api/log/gpt-image-2-preview/7/req-image-2/download/0',
    config: {
      disableDuplicate: true,
      responseType: 'blob',
    },
  })

  await act(async () => root.unmount())
  queryClient.clear()
  container.remove()
  apiClient.get = originalGet
})

test('replaces the legacy content summary with structured billing in shared log details', async () => {
  const apiClient = api as unknown as {
    get: () => Promise<{ data: unknown }>
  }
  const originalGet = apiClient.get
  apiClient.get = async () => ({
    data: {
      success: true,
      data: { urls: ['https://cos.example/one.webp'] },
    },
  })
  const container = domWindow.document.createElement('div')
  domWindow.document.body.append(container)
  const root = createRoot(
    container as unknown as Parameters<typeof createRoot>[0]
  )
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <DetailsDialog
            log={{ ...log, content: 'LEGACY_GPT_IMAGE_2_SUMMARY' }}
            isAdmin={false}
            open
            onOpenChange={() => undefined}
          />
        </I18nextProvider>
      </QueryClientProvider>
    )
  })
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 0))
  })

  const text = domWindow.document.body.textContent ?? ''
  assert.match(text, /Billing Rules and Formula/)
  assert.doesNotMatch(text, /LEGACY_GPT_IMAGE_2_SUMMARY/)

  await act(async () => root.unmount())
  queryClient.clear()
  container.remove()
  apiClient.get = originalGet
})
