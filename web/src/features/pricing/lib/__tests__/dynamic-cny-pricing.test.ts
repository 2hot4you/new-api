import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PricingModel } from '../../types'
import { getDynamicPricingSummary } from '../dynamic-price'

const model: PricingModel = {
  id: 1,
  model_name: 'qwen3.5-flash',
  quota_type: 0,
  model_ratio: 0,
  completion_ratio: 0,
  enable_groups: ['default'],
  billing_mode: 'tiered_expr',
  billing_currency: 'CNY',
  billing_expr:
    'len <= 128000 ? tier("up_to_128k", p * 0.2 + c * 2 + cr * 0.02) : tier("128k_to_256k", p * 0.8 + c * 8 + cr * 0.08)',
}

describe('CNY dynamic pricing', () => {
  test('keeps provider-published CNY tier prices out of USD display conversion', () => {
    const summary = getDynamicPricingSummary(model, {
      tokenUnit: 'M',
      priceRate: 3,
      usdExchangeRate: 7,
      showRechargePrice: true,
    })

    assert.equal(summary?.primaryEntries[0]?.formatted, '¥0.2')
    assert.equal(summary?.primaryEntries[1]?.formatted, '¥2')
    assert.equal(
      summary?.entries.find((entry) => entry.field === 'cacheReadPrice')
        ?.formatted,
      '¥0.02'
    )
  })
})
