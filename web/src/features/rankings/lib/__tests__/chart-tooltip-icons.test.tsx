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

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
const matchMedia = () => ({
  matches: false,
  addEventListener: () => undefined,
  removeEventListener: () => undefined,
})
Object.defineProperty(domWindow, 'matchMedia', {
  configurable: true,
  value: matchMedia,
})
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: matchMedia,
})
for (const key of ['window', 'document', 'DOMParser', 'Node'] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { decorateChartTooltipIcons } = await import('../chart-tooltip-icons')

after(() => domWindow.close())

test('replaces chart markers with configured icons while preserving summary rows', () => {
  const tooltip = document.createElement('div')
  tooltip.innerHTML = `
    <div data-col="shape">
      <div><svg data-original-marker="total"></svg></div>
      <div><svg data-original-marker="model"></svg></div>
      <div><svg data-original-marker="others"></svg></div>
    </div>
  `

  decorateChartTooltipIcons(
    tooltip,
    {
      content: [
        { key: 'Total:' },
        { key: 'claude-sonnet-4-6' },
        { key: 'Others' },
      ],
    },
    new Map([['claude-sonnet-4-6', 'Claude.Color']])
  )

  const rows = tooltip.querySelector('[data-col="shape"]')?.children
  assert.equal(
    rows?.[0]?.querySelector('[data-original-marker]') !== null,
    true
  )
  assert.equal(
    rows?.[1]
      ?.querySelector('[data-ranking-tooltip-icon]')
      ?.getAttribute('data-ranking-tooltip-icon'),
    'Claude.Color'
  )
  assert.equal(
    rows?.[2]?.querySelector('[data-original-marker]') !== null,
    true
  )
})

test('uses the existing Lobe icon fallback for a configured item without an icon key', () => {
  const tooltip = document.createElement('div')
  tooltip.innerHTML = '<div data-col="shape"><div></div></div>'

  decorateChartTooltipIcons(
    tooltip,
    { content: [{ key: 'custom-model' }] },
    new Map([['custom-model', undefined]])
  )

  const icon = tooltip.querySelector('[data-ranking-tooltip-icon]')
  assert.equal(icon?.getAttribute('data-ranking-tooltip-icon'), '')
  assert.equal(icon?.textContent, '?')
})
