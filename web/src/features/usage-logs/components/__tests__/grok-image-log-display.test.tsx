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

import type { UsageLog } from '../../data/schema'

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
const { GrokImageBillingCard } =
  await import('../dialogs/grok-image-billing-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })

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
  token_name: '',
  model_name: 'grok-imagine-image-quality',
  quota: 45000,
  prompt_tokens: 1,
  completion_tokens: 0,
  use_time: 1,
  is_stream: false,
  channel: 1,
  channel_name: '',
  token_id: 1,
  group: '',
  ip: '',
  other: '',
  request_id: '',
  upstream_request_id: '',
}

async function renderCard(log: UsageLog) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <GrokImageBillingCard log={log} quotaPerUnit={500000} />
      </I18nextProvider>
    )
  })
  return { container, root }
}

describe('Grok image billing display', () => {
  after(() => domWindow.close())

  test('renders the full edit snapshot without generic billing anchors', async () => {
    const other = {
      model_price: 1,
      grok_image_billing: {
        version: 1,
        model: 'grok-imagine-image-quality',
        operation: 'edit',
        resolution: '2k',
        aspect_ratio: '16:9',
        requested_output_count: 2,
        output_count: 1,
        input_image_count: 2,
        output_unit_price: 0.07,
        input_unit_price: 0.01,
        output_cost: 0.07,
        input_cost: 0.02,
        subtotal: 0.09,
        group_ratio: 1,
        final_cost: 0.09,
      },
    }
    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify(other),
    })
    const text = rendered.container.textContent ?? ''

    for (const expected of [
      'grok-imagine-image-quality',
      'Image Editing',
      '2K',
      '16:9',
      'Requested Outputs',
      'Actual Outputs',
      'Input Images',
      '¥0.070000',
      '¥0.010000',
      '¥0.090000',
      'Final Charge',
    ]) {
      assert.equal(text.includes(expected), true, expected)
    }
    for (const forbidden of [
      'Per-call',
      'Model Price',
      'Token Breakdown',
      '1 / 0',
      '¥1.000000',
    ]) {
      assert.equal(text.includes(forbidden), false, forbidden)
    }

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('renders a safe historical fallback without guessed fields', async () => {
    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify({ model_price: 1, group_ratio: 1 }),
    })
    const text = rendered.container.textContent ?? ''

    assert.equal(text.includes('grok-imagine-image-quality'), true)
    assert.equal(
      text.includes('Historical billing breakdown unavailable'),
      true
    )
    assert.equal(text.includes('Final Charge'), true)
    for (const forbidden of [
      'Resolution',
      'Aspect Ratio',
      'Per-call',
      'Model Price',
      'Token Breakdown',
      '1 / 0',
      '¥1.000000',
    ]) {
      assert.equal(text.includes(forbidden), false, forbidden)
    }

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('renders the Image 2.0 quality tier used for billing', async () => {
    const rendered = await renderCard({
      ...baseLog,
      model_name: 'grok-imagine-image-2.0',
      other: JSON.stringify({
        grok_image_billing: {
          version: 1,
          model: 'grok-imagine-image-2.0',
          operation: 'generation',
          resolution: '2k',
          quality: 'medium',
          aspect_ratio: '16:9',
          requested_output_count: 1,
          output_count: 1,
          input_image_count: 0,
          output_unit_price: 0.08,
          input_unit_price: 0.01,
          output_cost: 0.08,
          input_cost: 0,
          subtotal: 0.08,
          group_ratio: 1,
          final_cost: 0.08,
        },
      }),
    })
    const text = rendered.container.textContent ?? ''

    assert.equal(text.includes('grok-imagine-image-2.0'), true)
    assert.equal(text.includes('Quality'), true)
    assert.equal(text.includes('MEDIUM'), true)
    assert.equal(text.includes('¥0.080000'), true)

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })
})
