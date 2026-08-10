import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { TaskBillingSummary } from '../../types'
import {
  formatTaskBillingCny,
  formatTaskBillingFormula,
  getTaskBillingDisplay,
} from '../task-billing.ts'

describe('generation record billing', () => {
  test('formats the settled Seedance formula from the final task snapshot', () => {
    const billing: TaskBillingSummary = {
      state: 'settled',
      mode: 'seedance',
      model: 'doubao-seedance-2-0-260128',
      final_cost: 0.555,
      group_ratio: 1.5,
      detail_available: true,
      seedance: {
        actual_tokens: 10000,
        resolution: '720p',
        ratio: '16:9',
        seconds: 5,
        has_video: false,
        unit_price: 37,
      },
    }

    assert.equal(formatTaskBillingCny(billing.final_cost), '¥0.555000')
    assert.equal(
      formatTaskBillingFormula(billing),
      '10000 × ¥37.000000 ÷ 1,000,000 × 1.5000 = ¥0.555000'
    )
  })

  test('does not expose a precharge formula before terminal settlement', () => {
    const billing: TaskBillingSummary = {
      state: 'pending',
      mode: 'seedance',
      final_cost: 1,
      group_ratio: 1,
      detail_available: false,
    }
    assert.equal(formatTaskBillingFormula(billing), null)
  })

  test('never renders an unavailable settlement as a zero charge', () => {
    const billing: TaskBillingSummary = {
      state: 'unavailable',
      mode: 'grok_video',
      final_cost: 0,
      group_ratio: 1,
      detail_available: false,
    }

    assert.deepEqual(getTaskBillingDisplay(billing), { kind: 'unavailable' })
    assert.equal(formatTaskBillingFormula(billing), null)
  })

  test('only settled billing is rendered as a charge', () => {
    const billing: TaskBillingSummary = {
      state: 'settled',
      mode: 'seedance',
      final_cost: 0.125,
      group_ratio: 1,
      detail_available: false,
    }

    assert.deepEqual(getTaskBillingDisplay(billing), {
      kind: 'settled',
      amount: 0.125,
    })
  })

  test('uses the finalized Grok material and output costs', () => {
    const billing: TaskBillingSummary = {
      state: 'settled',
      mode: 'grok_video',
      final_cost: 0.36,
      group_ratio: 1,
      detail_available: true,
      grok_video: {
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

    assert.equal(
      formatTaskBillingFormula(billing),
      '(¥0.050000 × 6 + ¥0.010000 × 6) × 1.0000 = ¥0.360000'
    )
  })
})
