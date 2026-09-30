import assert from 'node:assert/strict'

import { afterEach, describe, test } from 'vitest'

import {
  formatSourceBillingAmount,
  sourceAmountToUSD,
} from '@/lib/currency'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

afterEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

describe('source billing currency conversion', () => {
  test('normalizes source amounts to USD', () => {
    assert.equal(sourceAmountToUSD(5, 'USD', 7), 5)
    assert.equal(sourceAmountToUSD(14, 'CNY', 7), 2)
    assert.ok(Number.isNaN(sourceAmountToUSD(14, 'CNY', 0)))
  })

  test('renders both USD and CNY source prices in the CNY site currency', () => {
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7,
      },
    })

    assert.equal(formatSourceBillingAmount(5, 'USD', { cnyPerUSD: 7 }), '¥35')
    assert.equal(formatSourceBillingAmount(2, 'CNY', { cnyPerUSD: 7 }), '¥2')
  })

  test('renders a CNY source price in the USD site currency', () => {
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'USD',
        usdExchangeRate: 7,
      },
    })

    assert.equal(
      formatSourceBillingAmount(2, 'CNY', {
        cnyPerUSD: 7,
        digitsSmall: 6,
      }),
      '$0.285714'
    )
  })

  test('rejects an invalid CNY exchange rate instead of guessing', () => {
    assert.equal(formatSourceBillingAmount(2, 'CNY', { cnyPerUSD: 0 }), '-')
  })
})
