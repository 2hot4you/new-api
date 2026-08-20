import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PricingModel } from '../../types'
import { getCompactPricingSummary } from '../model-card-summary'

function model(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    cache_ratio: 0.1,
    enable_groups: ['default'],
    ...overrides,
  }
}

const options = {
  tokenUnit: 'M' as const,
  showRechargePrice: false,
  priceRate: 1,
  usdExchangeRate: 1,
}

describe('compact model directory pricing summary', () => {
  test('summarizes fixed token prices without a full table', () => {
    const summary = getCompactPricingSummary(model(), options)

    assert.deepEqual(summary, {
      kind: 'token',
      items: [
        { label: 'Input', value: '$2' },
        { label: 'Output', value: '$4' },
        { label: 'Cached', value: '$0.2' },
      ],
      unit: '1,000,000 Token',
    })
  })

  test('uses the first published dynamic tier as the starting price', () => {
    const summary = getCompactPricingSummary(
      model({
        model_name: 'qwen3.5-flash',
        billing_mode: 'tiered_expr',
        billing_currency: 'CNY',
        billing_expr:
          'len <= 128000 ? tier("up_to_128k", p * 0.2 + c * 2 + cr * 0.02) : tier("128k_to_256k", p * 0.8 + c * 8 + cr * 0.08)',
      }),
      options
    )

    assert.deepEqual(summary, {
      kind: 'tiered',
      label: 'Tiered by per-request input Tokens',
      detail: '≤ 128K / > 128K',
      from: '¥0.2',
      unit: '1,000,000 Token',
    })
  })

  test('describes time-window pricing from the expression instead of the model ID', () => {
    const summary = getCompactPricingSummary(
      model({
        model_name: 'deepseek-custom-name',
        billing_mode: 'tiered_expr',
        billing_expr:
          '(tier("base", p * 1.5 + c * 4.5 + cr * 0.05)) * (hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12 ? 2 : 1) * (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18 ? 2 : 1)',
      }),
      options
    )

    assert.deepEqual(summary, {
      kind: 'tiered',
      label: 'Priced by request time',
      detail: '09:00–12:00, 14:00–18:00 (Asia/Shanghai) ×2',
      noteKey: 'Other times use the base price',
      from: '$1.5',
      unit: '1,000,000 Token',
    })
  })

  test('uses the lowest Grok output tier with its actual output unit', () => {
    const summary = getCompactPricingSummary(
      model({
        quota_type: 1,
        molii_grok_pricing: {
          kind: 'video',
          output_unit: 'second',
          output_prices: { '480p': 0.05, '720p': 0.07 },
          image_input_unit: 'image',
          image_input_price: 0.002,
        },
      }),
      options
    )

    assert.deepEqual(summary, {
      kind: 'tiered',
      label: 'Tiered pricing',
      from: '¥0.05',
      unit: 'second',
    })
  })

  test('uses the lowest Seedance tier per one million tokens', () => {
    const summary = getCompactPricingSummary(
      model({
        video_pricing: {
          unit: 'million_tokens',
          fps: 24,
          extra_frames: 1,
          rows: [
            {
              resolutions: ['480p', '720p'],
              without_video: 46,
              with_video: 28,
            },
            {
              resolutions: ['1080p'],
              without_video: 51,
              with_video: 31,
            },
          ],
        },
      }),
      options
    )

    assert.deepEqual(summary, {
      kind: 'tiered',
      label: 'Tiered pricing',
      from: '¥28.00',
      unit: '1,000,000 Token',
    })
  })
})
