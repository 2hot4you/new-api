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
import { describe, test } from 'node:test'

import {
  formatGrokVideoFormula,
  getGrokVideoBillingState,
  getGrokVideoListSummary,
  isGrokVideoBillingLog,
  isGrokVideoModel,
  parseGrokVideoBilling,
} from '../grok-video-billing'

const videoEdit = {
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
  image_input_unit_price: 0,
  video_input_unit_price: 0.01,
  output_cost: 0.3,
  image_input_cost: 0,
  video_input_cost: 0.06,
  subtotal: 0.36,
  group_ratio: 1,
  final_cost: 0.36,
} as const

describe('Grok video billing parser', () => {
  test('strictly parses the complete v1 snapshot and preserves numeric zero', () => {
    assert.deepEqual(
      parseGrokVideoBilling({ grok_video_billing: videoEdit }),
      videoEdit
    )
    assert.equal(
      parseGrokVideoBilling({ grok_video_billing: videoEdit })
        ?.image_input_unit_price,
      0
    )
  })

  test('parses JSON other payloads', () => {
    assert.deepEqual(
      parseGrokVideoBilling(JSON.stringify({ grok_video_billing: videoEdit })),
      videoEdit
    )
  })

  test('rejects unsupported, malformed, negative, non-finite, and incomplete snapshots', () => {
    const invalid = [
      { ...videoEdit, version: 2 },
      { ...videoEdit, model: 'grok-imagine-image' },
      { ...videoEdit, operation: 'generate' },
      { ...videoEdit, input_type: 'audio' },
      { ...videoEdit, input_type: 'image' },
      { ...videoEdit, actual_resolution: '' },
      { ...videoEdit, actual_resolution: undefined },
      { ...videoEdit, input_image_count: 0.5 },
      { ...videoEdit, actual_duration_seconds: -1 },
      { ...videoEdit, output_unit_price: Number.NaN },
      { ...videoEdit, final_cost: Number.POSITIVE_INFINITY },
    ]

    for (const snapshot of invalid) {
      assert.equal(
        parseGrokVideoBilling({ grok_video_billing: snapshot }),
        null
      )
    }
  })

  test('requires the snapshot model to match the log model', () => {
    assert.deepEqual(
      getGrokVideoBillingState({
        model_name: 'grok-imagine-video-1.5',
        other: { grok_video_billing: videoEdit },
      }),
      { kind: 'history', model: 'grok-imagine-video-1.5' }
    )
  })

  test('recognizes only the two supported Grok video models', () => {
    assert.equal(isGrokVideoModel('grok-imagine-video'), true)
    assert.equal(isGrokVideoModel('grok-imagine-video-1.5'), true)
    assert.equal(isGrokVideoModel('grok-imagine-image'), false)
  })

  test('routes only consumption records through the billing UI', () => {
    assert.equal(
      isGrokVideoBillingLog({ type: 2, model_name: 'grok-imagine-video' }),
      true
    )
    assert.equal(
      isGrokVideoBillingLog({ type: 5, model_name: 'grok-imagine-video' }),
      false
    )
  })

  test('formats operation-specific formulas', () => {
    assert.equal(
      formatGrokVideoFormula({
        ...videoEdit,
        operation: 'text_to_video',
        input_type: 'text',
        video_input_billed_seconds: 0,
        video_input_cost: 0,
        subtotal: 0.3,
        final_cost: 0.3,
      }),
      '(¥0.050000 × 6) × 1.0000 = ¥0.300000'
    )
    assert.equal(
      formatGrokVideoFormula({
        ...videoEdit,
        operation: 'image_to_video',
        input_type: 'image',
        input_image_count: 1,
        image_input_unit_price: 0.002,
        image_input_cost: 0.002,
        video_input_billed_seconds: 0,
        video_input_cost: 0,
        subtotal: 0.302,
        final_cost: 0.302,
      }),
      '(¥0.050000 × 6 + ¥0.002000 × 1) × 1.0000 = ¥0.302000'
    )
    assert.equal(
      formatGrokVideoFormula(videoEdit),
      '(¥0.050000 × 6 + ¥0.010000 × 6) × 1.0000 = ¥0.360000'
    )
  })

  test('builds current and historical list summaries safely', () => {
    assert.equal(
      getGrokVideoListSummary({
        model_name: 'grok-imagine-video',
        other: { grok_video_billing: videoEdit },
      }),
      '480P · 6s · video_edit'
    )
    assert.equal(
      getGrokVideoListSummary({
        model_name: 'grok-imagine-video-1.5',
        other: '{}',
      }),
      'grok-imagine-video-1.5'
    )
    assert.equal(
      getGrokVideoListSummary({ model_name: 'seedance', other: '{}' }),
      null
    )
  })
})
