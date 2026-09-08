import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import type { PricingModel } from '../../types'
import { getCardExamplePrice, getDynamicPricingSummary } from '../dynamic-price'

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

  test('propagates backend CNY through task ranges and evaluated examples', () => {
    const taskModel: PricingModel = {
      ...model,
      model_name: 'task-cny',
      billing_expr:
        'u("mode") == "pro" ? tier("pro", 0.2 + u("seconds") * 0.8) : tier("base", 0.1 + u("seconds") * 0.4)',
      billing_usage_schema: {
        mode: { enum: ['base', 'pro'] },
        seconds: { type: 'number', unit: 'second' },
      },
      billing_usage_examples: [
        { label: 'Pro · 5s', facts: { mode: 'pro', seconds: 5 } },
      ],
    }
    const options = {
      tokenUnit: 'M' as const,
      priceRate: 3,
      usdExchangeRate: 7,
      showRechargePrice: true,
    }

    const summary = getDynamicPricingSummary(taskModel, options)
    assert.equal(summary?.primaryEntries[0]?.formattedRange, '¥0.4 – ¥0.8')
    assert.deepEqual(getCardExamplePrice(taskModel, options), {
      label: 'Pro · 5s',
      formatted: '¥4.2',
    })
  })

  test('omits the CNY symbol when the caller supplies a separate caption', () => {
    const summary = getDynamicPricingSummary(model, {
      tokenUnit: 'M',
      showCurrencySymbol: false,
    })

    assert.equal(summary?.primaryEntries[0]?.formatted, '0.2')
    assert.equal(summary?.primaryEntries[1]?.formatted, '2')
  })
})
