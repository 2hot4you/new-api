/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

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

import type { VideoPricing } from '../../types'

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
const { VideoPricingMatrix } = await import('../video-pricing-matrix')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const pricing: VideoPricing = {
  unit: 'cny_per_million_tokens',
  fps: 24,
  extra_frames: 1,
  rows: [
    { resolutions: ['480p', '720p'], without_video: 46, with_video: 28 },
    { resolutions: ['1080p'], without_video: 51, with_video: 31 },
    { resolutions: ['4K'], without_video: 26, with_video: 16 },
  ],
}

describe('Seedance video pricing matrix', () => {
  after(() => domWindow.close())

  test('shows every resolution and video-input price without generic input/output pricing', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <VideoPricingMatrix pricing={pricing} showFormula />
        </I18nextProvider>
      )
    })

    const rows = container.querySelectorAll('tbody tr')
    assert.equal(rows.length, 3)
    assert.match(rows[0].textContent ?? '', /480p \/ 720p.*¥46\.00.*¥28\.00/)
    assert.match(rows[1].textContent ?? '', /1080p.*¥51\.00.*¥31\.00/)
    assert.match(rows[2].textContent ?? '', /4K.*¥26\.00.*¥16\.00/)
    assert.match(container.textContent ?? '', /1,000,000 Token/)
    assert.match(container.textContent ?? '', /24.*\+ 1.*1024/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('marks unsupported Fast resolutions explicitly', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <VideoPricingMatrix
            pricing={{
              ...pricing,
              rows: [pricing.rows[0]],
              unsupported_resolutions: ['1080p', '4K'],
            }}
            compact
          />
        </I18nextProvider>
      )
    })

    const rows = container.querySelectorAll('tbody tr')
    assert.equal(rows.length, 3)
    assert.match(rows[1].textContent ?? '', /1080p.*Not supported/)
    assert.match(rows[2].textContent ?? '', /4K.*Not supported/)
    assert.doesNotMatch(container.textContent ?? '', /Token =/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('converts the displayed prices when switching from 1M to 1K tokens', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <VideoPricingMatrix pricing={pricing} tokenUnit='K' showFormula />
        </I18nextProvider>
      )
    })

    const rows = container.querySelectorAll('tbody tr')
    assert.match(rows[0].textContent ?? '', /¥0\.046.*¥0\.028/)
    assert.match(rows[1].textContent ?? '', /¥0\.051.*¥0\.031/)
    assert.match(container.textContent ?? '', /¥ \/ 1,000 Token/)
    assert.match(container.textContent ?? '', /对应档位单价|tier price/)
    assert.doesNotMatch(container.textContent ?? '', /1,000,000 Token/)

    await act(async () => root.unmount())
    container.remove()
  })
})
