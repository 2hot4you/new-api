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
import { afterEach, describe, expect, test } from 'vitest'

import { useSystemConfigStore } from '@/stores/system-config-store'

import { SORT_OPTIONS } from '../constants'
import { sortModels } from '../lib/filters'
import type { PricingModel } from '../types'

function pricingModel(
  modelName: string,
  overrides: Partial<PricingModel>
): PricingModel {
  return {
    id: 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

const models = [
  pricingModel('legacy-ratio', { model_ratio: 1, completion_ratio: 4 }),
  pricingModel('legacy-per-call', { quota_type: 1, model_price: 0.05 }),
  pricingModel('expr-tokens', {
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("base", p * 1 + c * 10)',
  }),
  pricingModel('expr-long-context', {
    billing_mode: 'tiered_expr',
    billing_expr:
      'len <= 200000 ? tier("standard", p * 3 + c * 15) : tier("long", p * 6 + c * 22.5)',
  }),
  pricingModel('expr-input-only', {
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("base", p * 0.1)',
  }),
  pricingModel('expr-fixed', {
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("request", fixed(0.02))',
  }),
  pricingModel('expr-task', {
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("base", u("seconds") * 0.1)',
    billing_usage_schema: { seconds: { type: 'number', unit: 'second' } },
    billing_usage_examples: [{ label: '5s', facts: { seconds: 5 } }],
  }),
]

describe('price sorting', () => {
  const initialConfig = useSystemConfigStore.getState().config
  afterEach(() => useSystemConfigStore.setState({ config: initialConfig }))

  test.each([
    [SORT_OPTIONS.INPUT_PRICE_LOW, 'tier("base", p * PRICE + c * PRICE)', {}],
    [SORT_OPTIONS.OUTPUT_PRICE_LOW, 'tier("base", p * PRICE + c * PRICE)', {}],
    [SORT_OPTIONS.REQUEST_PRICE_LOW, 'tier("base", fixed(PRICE))', {}],
    [
      SORT_OPTIONS.REQUEST_PRICE_LOW,
      'tier("base", u("seconds") * PRICE)',
      {
        billing_usage_schema: { seconds: { type: 'number', unit: 'second' } },
        billing_usage_examples: [{ label: '1s', facts: { seconds: 1 } }],
      },
    ],
  ] satisfies [string, string, Partial<PricingModel>][])(
    '%s compares CNY and USD %s in the same currency',
    (sortBy, expression, taskFields) => {
      useSystemConfigStore.setState({
        config: {
          ...initialConfig,
          currency: { ...initialConfig.currency, usdExchangeRate: 7 },
        },
      })
      const quote = (name: string, currency: 'CNY' | 'USD', price: number) =>
        pricingModel(name, {
          billing_mode: 'tiered_expr',
          billing_currency: currency,
          billing_expr: expression.replaceAll('PRICE', String(price)),
          ...taskFields,
        })
      const quotes = [
        quote('usd-expensive', 'USD', 2),
        quote('cny-cheap', 'CNY', 7),
        quote('usd-equal', 'USD', 1),
      ]
      expect(
        sortModels(quotes, sortBy).map((model) => model.model_name)
      ).toEqual(['cny-cheap', 'usd-equal', 'usd-expensive'])
      expect(
        sortModels(quotes, sortBy.replace('low', 'high')).map(
          (model) => model.model_name
        )
      ).toEqual(['usd-expensive', 'cny-cheap', 'usd-equal'])
      expect(quotes[1].billing_currency).toBe('CNY')
      expect(quotes[1].billing_expr).toContain('7')
    }
  )

  test.each([
    [
      SORT_OPTIONS.INPUT_PRICE_LOW,
      [
        'expr-input-only',
        'expr-tokens',
        'legacy-ratio',
        'expr-long-context',
        'legacy-per-call',
        'expr-fixed',
        'expr-task',
      ],
    ],
    [
      SORT_OPTIONS.INPUT_PRICE_HIGH,
      [
        'expr-long-context',
        'legacy-ratio',
        'expr-tokens',
        'expr-input-only',
        'legacy-per-call',
        'expr-fixed',
        'expr-task',
      ],
    ],
    [
      SORT_OPTIONS.OUTPUT_PRICE_LOW,
      [
        'legacy-ratio',
        'expr-tokens',
        'expr-long-context',
        'legacy-per-call',
        'expr-input-only',
        'expr-fixed',
        'expr-task',
      ],
    ],
    [
      SORT_OPTIONS.REQUEST_PRICE_HIGH,
      [
        'expr-task',
        'legacy-per-call',
        'expr-fixed',
        'legacy-ratio',
        'expr-tokens',
        'expr-long-context',
        'expr-input-only',
      ],
    ],
  ])(
    '%s compares base prices across legacy and expression billing and keeps unpriced models last',
    (sortBy, expected) => {
      expect(sortModels(models, sortBy).map((m) => m.model_name)).toEqual(
        expected
      )
    }
  )
})
