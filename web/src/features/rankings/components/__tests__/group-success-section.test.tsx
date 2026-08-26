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
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { after, describe, test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { Window } from 'happy-dom'

const domWindow = new Window()
const testDirectory = fileURLToPath(new URL('.', import.meta.url))
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
const { GroupSuccessSection } = await import('../group-success-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('GroupSuccessSection', () => {
  after(() => domWindow.close())

  test('ranks measured groups by success rate then request count and preserves 0%', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <GroupSuccessSection
            period='week'
            groups={[
              { group: 'free', request_count: 0, success_rate: null },
              { group: 'standard', request_count: 80, success_rate: 90 },
              { group: 'premium', request_count: 12, success_rate: 100 },
              { group: 'trial', request_count: 3, success_rate: 0 },
              { group: 'business', request_count: 180, success_rate: 90 },
            ]}
          />
        </I18nextProvider>
      )
    })

    const rows = [...container.querySelectorAll('[data-group-success-row]')]
    assert.deepEqual(
      rows.map((row) => row.getAttribute('data-group-success-row')),
      ['premium', 'business', 'standard', 'trial', 'free']
    )
    assert.match(rows[3].textContent ?? '', /trial.*0%.*3 requests/)
    assert.match(rows[4].textContent ?? '', /free.*No requests/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('labels the selected period and distinguishes an empty configured group list', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <GroupSuccessSection period='year' groups={[]} />
        </I18nextProvider>
      )
    })

    assert.equal(
      container.querySelector('section')?.getAttribute('aria-label'),
      'Group success rates for the past year'
    )
    assert.match(container.textContent ?? '', /No configured groups/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('uses the rankings snapshot instead of a second performance-summary request', () => {
    const rankingsIndex = readFileSync(
      resolve(testDirectory, '../../index.tsx'),
      'utf8'
    )

    assert.doesNotMatch(rankingsIndex, /getPerfMetricsSummary|useGroupSuccess/)
    assert.match(rankingsIndex, /snapshot\.group_success/)
    assert.equal(
      existsSync(resolve(testDirectory, '../../hooks/use-group-success.ts')),
      false
    )
  })
})
