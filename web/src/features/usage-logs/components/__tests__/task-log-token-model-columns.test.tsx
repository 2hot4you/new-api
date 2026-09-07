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

import { Window } from 'happy-dom'
import type { ComponentType } from 'react'
import { afterAll as after, describe, test } from 'vitest'

import type { TaskLog } from '../../types'

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
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { useTaskLogsColumns } = await import('../columns/task-logs-columns')
const { UsageLogsProvider } = await import('../usage-logs-provider')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const task: TaskLog = {
  id: 1,
  user_id: 7,
  platform: '62',
  task_id: 'task_public_123',
  action: 'TEXT_TO_VIDEO',
  channel_id: 2,
  token_name: 'Video production',
  group: 'xAI Video',
  submit_time: 1,
  status: 'IN_PROGRESS',
  properties: {
    origin_model_name: 'grok-imagine-video-1.5',
    upstream_model_name: 'grok-video-v2-internal',
  },
}

function CellProbe({ accessorKey }: { accessorKey: string }) {
  const columns = useTaskLogsColumns(false)
  const column = columns.find(
    (candidate) =>
      'accessorKey' in candidate && candidate.accessorKey === accessorKey
  )
  assert.ok(column, `missing ${accessorKey} column`)
  const Cell = column.cell as ComponentType<{
    row: { original: TaskLog; getValue: (key: string) => unknown }
  }>
  return (
    <Cell
      row={{
        original: task,
        getValue: (key) => task[key as keyof TaskLog],
      }}
    />
  )
}

async function renderCell(accessorKey: string) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <UsageLogsProvider>
          <CellProbe accessorKey={accessorKey} />
        </UsageLogsProvider>
      </I18nextProvider>
    )
  })
  return { container, root }
}

describe('video task token and model columns', () => {
  after(() => domWindow.close())

  test('renders the token name in the task list', async () => {
    const rendered = await renderCell('token_name')
    assert.equal(
      rendered.container.textContent?.includes('Video production'),
      true
    )
    assert.ok(rendered.container.querySelector('.lucide-key-round'))
    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('renders the requested model ID with its model icon', async () => {
    const rendered = await renderCell('model')
    assert.equal(
      rendered.container.textContent?.includes('grok-imagine-video-1.5'),
      true
    )
    assert.ok(rendered.container.querySelector('[aria-label="Grok"]'))
    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })
})
