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
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
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
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { ApiKeyTimestampCell } = await import('../api-key-timestamp-cell')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('API key timestamp table cell', () => {
  after(() => {
    domWindow.close()
  })

  test('shows the exact local date and time when absolute display is requested', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const timestamp = new Date(2026, 7, 26, 9, 10, 11).getTime() / 1000

    await act(async () =>
      root.render(
        <ApiKeyTimestampCell
          timestamp={timestamp}
          now={new Date(2026, 7, 26, 12, 10, 11).getTime()}
          justNowLabel='Just now'
          display='absolute'
        />
      )
    )

    assert.equal(container.textContent, '2026-08-26 09:10:11')
    assert.equal(container.textContent?.includes('ago'), false)
    assert.equal(
      container.querySelector('time')?.getAttribute('dateTime'),
      new Date(timestamp * 1000).toISOString()
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps the existing relative display when absolute mode is not requested', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const timestamp = Math.floor(Date.now() / 1000) - 3 * 60 * 60

    await act(async () =>
      root.render(
        <ApiKeyTimestampCell
          timestamp={timestamp}
          now={timestamp * 1000 + 3 * 60 * 60 * 1000}
          locale='en'
          justNowLabel='Just now'
        />
      )
    )

    assert.equal(container.textContent, '3 hours ago')
    assert.equal(container.textContent?.includes('2026-'), false)

    await act(async () => root.unmount())
    container.remove()
  })
})
