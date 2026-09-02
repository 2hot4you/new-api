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
import { afterAll as after, describe, test } from 'vitest'

import { Window } from 'happy-dom'
import type { ComponentType } from 'react'

import type { UsageLog } from '../../data/schema'

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
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { useCommonLogsColumns } = await import('../columns/common-logs-columns')
const { UsageLogsProvider } = await import('../usage-logs-provider')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Model test': 'Model test',
        'API playground': 'API playground',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const baseLog: UsageLog = {
  id: 1,
  user_id: 1,
  created_at: 1,
  type: 2,
  content: '',
  username: '',
  token_name: 'customer-key',
  model_name: 'claude-sonnet-4-6',
  quota: 1,
  prompt_tokens: 1,
  completion_tokens: 1,
  use_time: 1,
  is_stream: false,
  channel: 1,
  channel_name: '',
  token_id: 1,
  group: 'Anthropic',
  ip: '',
  other: '',
  request_id: '',
  upstream_request_id: '',
}

function TokenCellProbe({ log }: { log: UsageLog }) {
  const columns = useCommonLogsColumns(false)
  const column = columns.find(
    (candidate) =>
      'accessorKey' in candidate && candidate.accessorKey === 'token_name'
  )
  const Cell = column?.cell as ComponentType<{
    row: { original: UsageLog }
  }>

  return <Cell row={{ original: log }} />
}

async function renderTokenCell(log: UsageLog) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(['user-groups'], {
    success: true,
    data: {
      Anthropic: {
        desc: 'Official Claude API',
        ratio: 1,
        icon: 'Anthropic.Color',
      },
    },
  })

  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <UsageLogsProvider>
            <TokenCellProbe log={log} />
          </UsageLogsProvider>
        </I18nextProvider>
      </QueryClientProvider>
    )
  })
  return { container, root, queryClient }
}

describe('usage log token context display', () => {
  after(() => domWindow.close())

  test('renders the configured icon beside a normal group', async () => {
    const rendered = await renderTokenCell(baseLog)
    const icon = rendered.container.querySelector(
      '[data-usage-log-group-icon="Anthropic"]'
    )

    assert.ok(icon)
    assert.equal(icon.getAttribute('data-icon-key'), 'Anthropic.Color')

    await act(async () => rendered.root.unmount())
    rendered.queryClient.clear()
  })

  test('labels model tests instead of exposing the default group name', async () => {
    const rendered = await renderTokenCell({
      ...baseLog,
      token_name: '模型测试',
      group: 'default',
    })
    const source = rendered.container.querySelector(
      '[data-usage-log-token-context="model-test"]'
    )

    assert.equal(source?.textContent, 'Model test')
    assert.ok(source?.querySelector('.lucide-gauge'))
    assert.doesNotMatch(source?.textContent ?? '', /default/i)

    await act(async () => rendered.root.unmount())
    rendered.queryClient.clear()
  })

  test('labels playground requests instead of showing the playground group', async () => {
    const rendered = await renderTokenCell({
      ...baseLog,
      token_name: 'playground-Anthropic',
      group: 'playground',
    })
    const source = rendered.container.querySelector(
      '[data-usage-log-token-context="playground"]'
    )

    assert.equal(source?.textContent, 'API playground')
    assert.ok(source?.querySelector('.lucide-message-square-code'))
    assert.doesNotMatch(source?.textContent ?? '', /^playground$/i)

    await act(async () => rendered.root.unmount())
    rendered.queryClient.clear()
  })
})
