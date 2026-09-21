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
import { afterAll as after, describe, test } from 'vitest'

const domWindow = new Window({ url: 'https://claudeye.test/' })
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLImageElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
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

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { CLAUDEYE_WORDMARK_FALLBACK, ClaudeyeWordmark, claudeyeWordmarkUrl } =
  await import('../claudeye-wordmark')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('ClaudeyeWordmark', () => {
  after(() => domWindow.close())

  test('builds the dynamic endpoint for both supported surfaces', () => {
    assert.equal(
      claudeyeWordmarkUrl('light'),
      '/api/branding/claudeye/wordmark.svg?surface=light'
    )
    assert.equal(
      claudeyeWordmarkUrl('dark'),
      '/api/branding/claudeye/wordmark.svg?surface=dark'
    )
  })

  test('uses the light dynamic wordmark and falls back once', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(<ClaudeyeWordmark surface='light' alt='claudeye' />)
    })

    const image = container.querySelector('img')
    assert.ok(image)
    assert.equal(
      image.getAttribute('src'),
      '/api/branding/claudeye/wordmark.svg?surface=light'
    )

    await act(async () => image.dispatchEvent(new Event('error')))
    assert.equal(image.getAttribute('src'), CLAUDEYE_WORDMARK_FALLBACK)

    await act(async () => image.dispatchEvent(new Event('error')))
    assert.equal(image.getAttribute('src'), CLAUDEYE_WORDMARK_FALLBACK)

    await act(async () => root.unmount())
    container.remove()
  })
})
