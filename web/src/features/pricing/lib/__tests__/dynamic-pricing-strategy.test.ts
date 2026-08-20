import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getDynamicPricingStrategy } from '../dynamic-price'

describe('dynamic pricing strategy', () => {
  test('derives per-request complete-input ranges from len tier conditions', () => {
    const strategy = getDynamicPricingStrategy(
      'len <= 128000 ? tier("short", p * 1 + c * 2) : len <= 256000 ? tier("medium", p * 2 + c * 4) : tier("long", p * 3 + c * 6)'
    )

    assert.deepEqual(strategy, {
      kind: 'input_length',
      tierRanges: ['≤ 128K', '128K–256K', '> 256K'],
      timeRules: [],
    })
  })

  test('derives request-time windows, timezone, and multipliers', () => {
    const strategy = getDynamicPricingStrategy(
      '(tier("base", p * 1.5 + c * 4.5)) * (hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12 ? 2 : 1) * (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18 ? 2 : 1)'
    )

    assert.deepEqual(strategy, {
      kind: 'time_window',
      tierRanges: [],
      timeRules: [
        {
          label: '09:00–12:00',
          timezone: 'Asia/Shanghai',
          multiplier: '2',
        },
        {
          label: '14:00–18:00',
          timezone: 'Asia/Shanghai',
          multiplier: '2',
        },
      ],
    })
  })

  test('keeps unsupported request conditions generic', () => {
    const strategy = getDynamicPricingStrategy(
      'tier("base", p * 1 + c * 2) * (param("service_tier") == "priority" ? 2 : 1)'
    )

    assert.equal(strategy.kind, 'request_conditions')
  })
})
