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
const { GPTImage2PreviewCard } = await import('../gpt-image-2-preview-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })
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
  quota: 1,
  prompt_tokens: 0,
  completion_tokens: 0,
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

test('renders complete parameters and Molii-owned 24-hour previews', async () => {
  const apiClient = api as unknown as {
    get: () => Promise<{ data: unknown }>
  }
  const originalGet = apiClient.get
  apiClient.get = async () => ({
    data: {
      success: true,
      data: {
        urls: ['https://cos.example/one.webp', 'https://cos.example/two.webp'],
      },
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
          <GPTImage2PreviewCard log={log} />
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
    'Quality',
    'high',
    'Background',
    'transparent',
    'Output Format',
    'WEBP',
    'Moderation',
    'low',
    'Size',
    '1536x1024',
    'User',
    'customer-42',
    'Requested Outputs',
    'Actual Outputs',
    'Total Duration',
  ]) {
    assert.match(text, new RegExp(expected))
  }
  assert.equal(container.querySelectorAll('img').length, 3)

  await act(async () => root.unmount())
  queryClient.clear()
  container.remove()
  apiClient.get = originalGet
})
