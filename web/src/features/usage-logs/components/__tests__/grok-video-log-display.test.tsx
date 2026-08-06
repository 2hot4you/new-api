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
const { GrokVideoBillingCard } =
  await import('../dialogs/grok-video-billing-card')
const { DetailsDialog } = await import('../dialogs/details-dialog')
const { getCommonLogDetailPreviewText } =
  await import('../columns/common-logs-columns')

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
  model_name: 'grok-imagine-video',
  quota: 180000,
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
        <GrokVideoBillingCard log={log} quotaPerUnit={500000} />
      </I18nextProvider>
    )
  })
  return { container, root }
}

async function renderDetails(log: UsageLog) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <DetailsDialog
          log={log}
          isAdmin={false}
          open
          onOpenChange={() => undefined}
        />
      </I18nextProvider>
    )
  })
  return { container, root }
}

describe('Grok video billing display', () => {
  after(() => domWindow.close())

  test('renders a complete video-edit snapshot without generic billing anchors', async () => {
    const other = {
      model_price: 1,
      grok_video_billing: {
        version: 1,
        model: 'grok-imagine-video',
        operation: 'video_edit',
        input_type: 'video',
        requested_duration_seconds: 0,
        estimated_duration_seconds: 8.7,
        actual_duration_seconds: 6,
        requested_resolution: '',
        estimated_resolution: '720p',
        actual_resolution: '480p',
        aspect_ratio: '',
        input_image_count: 0,
        video_input_billed_seconds: 6,
        output_unit_price: 0.05,
        image_input_unit_price: 0.002,
        video_input_unit_price: 0.01,
        output_cost: 0.3,
        image_input_cost: 0,
        video_input_cost: 0.06,
        subtotal: 0.36,
        group_ratio: 1,
        final_cost: 0.36,
      },
    }
    const rendered = await renderCard({
      ...baseLog,
      other: JSON.stringify(other),
    })
    const text = rendered.container.textContent ?? ''

    for (const expected of [
      'Grok Video Billing',
      'grok-imagine-video',
      'Video Editing',
      'Video',
      'Billing Duration',
      '6s',
      'Billing Resolution',
      '480P',
      'Video Input Billed Seconds',
      'Output Unit Price',
      'Video Input Unit Price',
      'Output Subtotal',
      'Video Input Subtotal',
      'Group Ratio',
      'Billing Formula',
      '(¥0.050000 × 6 + ¥0.010000 × 6) × 1.0000 = ¥0.360000',
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
      'Requested Duration',
      'Estimated Duration',
      'Actual Duration',
      '8.7s',
      'Requested Resolution',
      'Estimated Resolution',
      'Actual Resolution',
      '720P',
    ]) {
      assert.equal(text.includes(forbidden), false, forbidden)
    }

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('renders image input fields for image-to-video billing', async () => {
    const snapshot = {
      version: 1,
      model: 'grok-imagine-video-1.5',
      operation: 'image_to_video',
      input_type: 'image',
      requested_duration_seconds: 10,
      estimated_duration_seconds: 10,
      actual_duration_seconds: 10,
      requested_resolution: '720p',
      estimated_resolution: '720p',
      actual_resolution: '720p',
      aspect_ratio: '16:9',
      input_image_count: 1,
      video_input_billed_seconds: 0,
      output_unit_price: 0.05,
      image_input_unit_price: 0.002,
      video_input_unit_price: 0,
      output_cost: 0.5,
      image_input_cost: 0.002,
      video_input_cost: 0,
      subtotal: 0.502,
      group_ratio: 1,
      final_cost: 0.502,
    }
    const rendered = await renderCard({
      ...baseLog,
      model_name: 'grok-imagine-video-1.5',
      quota: 251000,
      other: JSON.stringify({ grok_video_billing: snapshot }),
    })
    const text = rendered.container.textContent ?? ''

    for (const expected of [
      'Image to Video',
      'Input Images',
      'Image Input Unit Price',
      'Image Input Subtotal',
      'Requested Duration',
      'Requested Resolution',
      'Billing Duration',
      'Billing Resolution',
      '16:9',
      '¥0.002000',
    ]) {
      assert.equal(text.includes(expected), true, expected)
    }
    assert.equal(text.includes('Video Input Billed Seconds'), false)
    assert.equal(text.includes('Estimated Duration'), false)
    assert.equal(text.includes('Estimated Resolution'), false)
    assert.equal(text.includes('Actual Duration'), false)
    assert.equal(text.includes('Actual Resolution'), false)

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })

  test('renders a safe historical fallback with model and final cost', async () => {
    const historicalLog = {
      ...baseLog,
      model_name: 'grok-imagine-video-1.5',
      other: JSON.stringify({ model_price: 1, group_ratio: 1 }),
    }
    const rendered = await renderCard(historicalLog)
    const text = rendered.container.textContent ?? ''

    assert.equal(text.includes('grok-imagine-video-1.5'), true)
    assert.equal(
      text.includes('Historical billing breakdown unavailable'),
      true
    )
    assert.equal(text.includes('Final Charge'), true)
    assert.equal(text.includes('¥0.360000'), true)
    for (const forbidden of [
      'Requested Duration',
      'Actual Resolution',
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

    const details = await renderDetails(historicalLog)
    const detailsText = document.body.textContent ?? ''
    assert.equal(detailsText.includes('Grok Video Billing'), true)
    assert.equal(
      detailsText.includes('Historical billing breakdown unavailable'),
      true
    )
    for (const forbidden of ['Token Breakdown', 'Model Price', '¥1.000000']) {
      assert.equal(detailsText.includes(forbidden), false, forbidden)
    }

    await act(async () => details.root.unmount())
    details.container.remove()
  })

  test('keeps sanitized failure content on the normal error-log path', async () => {
    const failedLog = {
      ...baseLog,
      type: 5,
      content: 'Grok video task failed: upstream rejected the request',
      other: JSON.stringify({ is_task: true }),
    }

    assert.equal(
      getCommonLogDetailPreviewText(
        failedLog,
        { is_task: true },
        (key: string) => key,
        false
      ),
      failedLog.content
    )

    const rendered = await renderDetails(failedLog)
    const text = document.body.textContent ?? ''
    assert.equal(text.includes(failedLog.content), true)
    assert.equal(text.includes('Grok Video Billing'), false)
    assert.equal(
      text.includes('Historical billing breakdown unavailable'),
      false
    )

    await act(async () => rendered.root.unmount())
    rendered.container.remove()
  })
})
