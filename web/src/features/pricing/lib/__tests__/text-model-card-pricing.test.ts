/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import type { PricingModel, TokenUnit } from '../../types'
import * as dynamicPrice from '../dynamic-price'

type TextModelCardPricing = {
  kind: 'fixed' | 'tiered'
  explanationKey: string
  unitLabel: string
  rows: Array<{
    label: string
    input: string
    output: string
    cache: string
  }>
}

type GetTextModelCardPricing = (
  model: PricingModel,
  options: { tokenUnit: TokenUnit }
) => TextModelCardPricing | null

function pricingModel(modelName: string): PricingModel {
  return {
    id: 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    cache_ratio: 0.1,
    enable_groups: ['default'],
  }
}

const getTextModelCardPricing = (
  dynamicPrice as unknown as {
    getTextModelCardPricing?: GetTextModelCardPricing
  }
).getTextModelCardPricing

describe('text model marketplace card pricing', () => {
  test('exposes a billing explanation only for the four selected fixed-price models', () => {
    assert.equal(typeof getTextModelCardPricing, 'function')
    if (!getTextModelCardPricing) return

    for (const modelName of [
      'deepseek-v4-flash-202605',
      'deepseek-v4-pro-202606',
      'glm-5.2',
      'kimi-k3',
    ]) {
      assert.deepEqual(
        getTextModelCardPricing(pricingModel(modelName), { tokenUnit: 'M' }),
        {
          kind: 'fixed',
          explanationKey: 'Billed by input, output, and cached Token usage',
          unitLabel: '1M',
          rows: [],
        },
        modelName
      )
    }

    assert.equal(
      getTextModelCardPricing(pricingModel('unrelated-model'), {
        tokenUnit: 'M',
      }),
      null
    )
    assert.equal(
      getTextModelCardPricing(pricingModel('qwen3.5-flash'), {
        tokenUnit: 'M',
      }),
      null,
      'a tiered model without a valid expression must not advertise tier pricing'
    )
  })

  test('expands all published tiers for the three dynamic-price models', () => {
    assert.equal(typeof getTextModelCardPricing, 'function')
    if (!getTextModelCardPricing) return

    const qwenFlash = pricingModel('qwen3.5-flash')
    qwenFlash.billing_mode = 'tiered_expr'
    qwenFlash.billing_currency = 'CNY'
    qwenFlash.billing_expr =
      'len <= 128000 ? tier("up_to_128k", p * 0.2 + c * 2 + cr * 0.02) : len <= 256000 ? tier("128k_to_256k", p * 0.8 + c * 8 + cr * 0.08) : tier("256k_to_1m", p * 1.2 + c * 12 + cr * 0.12)'

    assert.deepEqual(getTextModelCardPricing(qwenFlash, { tokenUnit: 'M' }), {
      kind: 'tiered',
      explanationKey: 'Tiered by full input length',
      unitLabel: '1M',
      rows: [
        { label: '≤ 128K', input: '¥0.2', output: '¥2', cache: '¥0.02' },
        { label: '128K–256K', input: '¥0.8', output: '¥8', cache: '¥0.08' },
        { label: '256K–1M', input: '¥1.2', output: '¥12', cache: '¥0.12' },
      ],
    })
  })

  test('formats tier prices using the selected Token unit', () => {
    assert.equal(typeof getTextModelCardPricing, 'function')
    if (!getTextModelCardPricing) return

    const minimax = pricingModel('minimax-m3')
    minimax.billing_mode = 'tiered_expr'
    minimax.billing_currency = 'CNY'
    minimax.billing_expr =
      'len <= 512000 ? tier("up_to_512k", p * 2.1 + c * 8.4 + cr * 0.42) : tier("over_512k", p * 4.2 + c * 16.8 + cr * 0.84)'

    assert.deepEqual(
      getTextModelCardPricing(minimax, { tokenUnit: 'K' })?.rows[0],
      {
        label: '≤ 512K',
        input: '¥0.0021',
        output: '¥0.0084',
        cache: '¥0.00042',
      }
    )
  })
})
