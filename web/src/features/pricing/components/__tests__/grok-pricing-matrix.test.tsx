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

import type { MoliiGrokPricing } from '../../types'

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
const { GrokPricingMatrix } = await import('../grok-pricing-matrix')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function renderPricing(pricing: MoliiGrokPricing) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <GrokPricingMatrix pricing={pricing} />
      </I18nextProvider>
    )
  })

  return { container, root }
}

describe('Grok marketplace pricing matrix', () => {
  after(() => domWindow.close())

  test('shows image output and input prices in a three-column table', async () => {
    const { container, root } = await renderPricing({
      kind: 'image',
      output_unit: 'image',
      output_prices: { '2k': 0.07, '1k': 0.05 },
      image_input_unit: 'image',
      image_input_price: 0.01,
    })

    assert.equal(container.querySelectorAll('thead th').length, 3)
    assert.match(
      container.querySelector('thead')?.textContent ?? '',
      /Output resolution.*Image output.*Image input/
    )
    const rows = container.querySelectorAll('tbody tr')
    assert.equal(rows.length, 2)
    assert.match(
      rows[0].textContent ?? '',
      /1k.*¥0\.05 \/ image.*¥0\.01 \/ image/
    )
    assert.match(
      rows[1].textContent ?? '',
      /2k.*¥0\.07 \/ image.*¥0\.01 \/ image/
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows both media input charges for legacy video', async () => {
    const { container, root } = await renderPricing({
      kind: 'video',
      output_unit: 'second',
      output_prices: { '720p': 0.07, '480p': 0.05 },
      image_input_unit: 'image',
      image_input_price: 0.002,
      video_input_unit: 'second',
      video_input_price: 0.01,
    })

    assert.match(
      container.querySelector('thead')?.textContent ?? '',
      /Video output.*Media input/
    )
    const firstRow = container.querySelector('tbody tr')?.textContent ?? ''
    assert.match(firstRow, /480p.*¥0\.05 \/ second/)
    assert.match(firstRow, /Image input.*¥0\.002 \/ image/)
    assert.match(firstRow, /Video input.*¥0\.01 \/ second/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows image 2.0 quality tiers as separate pricing rows', async () => {
    const { container, root } = await renderPricing({
      kind: 'image',
      output_unit: 'image',
      output_prices: {
        'low/1k': 0.04,
        'low/2k': 0.06,
        'medium/1k': 0.06,
        'medium/2k': 0.08,
      },
      image_input_unit: 'image',
      image_input_price: 0.01,
    })

    const rows = container.querySelectorAll('tbody tr')
    assert.equal(rows.length, 4)
    assert.match(rows[0].textContent ?? '', /Low · 1K.*¥0\.04 \/ image/)
    assert.match(rows[3].textContent ?? '', /Medium · 2K.*¥0\.08 \/ image/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows three Video 1.5 output rows without a video input charge', async () => {
    const { container, root } = await renderPricing({
      kind: 'video',
      output_unit: 'second',
      output_prices: { '1080p': 0.25, '480p': 0.08, '720p': 0.14 },
      image_input_unit: 'image',
      image_input_price: 0.01,
    })

    const rows = container.querySelectorAll('tbody tr')
    assert.equal(rows.length, 3)
    assert.match(rows[2].textContent ?? '', /1080p.*¥0\.25 \/ second/)
    assert.doesNotMatch(container.textContent ?? '', /Video input/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows an unavailable state instead of an empty table', async () => {
    const { container, root } = await renderPricing({
      kind: 'video',
      output_unit: 'second',
      output_prices: {},
    })

    assert.equal(container.querySelector('table'), null)
    assert.match(
      container.textContent ?? '',
      /Pricing is temporarily unavailable\./
    )

    await act(async () => root.unmount())
    container.remove()
  })
})
