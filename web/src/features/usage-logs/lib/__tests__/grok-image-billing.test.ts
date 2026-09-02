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
import { describe, test } from 'vitest'

import {
  getGrokImageBillingState,
  isGrokImageLog,
  parseGrokImageBilling,
} from '../grok-image-billing'

const generation = {
  version: 1,
  model: 'grok-imagine-image',
  operation: 'generation',
  resolution: '2k',
  aspect_ratio: '16:9',
  requested_output_count: 2,
  output_count: 1,
  input_image_count: 0,
  output_unit_price: 0.05,
  input_unit_price: 0,
  output_cost: 0.05,
  input_cost: 0,
  subtotal: 0.05,
  group_ratio: 1,
  final_cost: 0.05,
} as const

describe('Grok image billing parser', () => {
  test('strictly parses a v1 generation snapshot', () => {
    assert.deepEqual(
      parseGrokImageBilling({ grok_image_billing: generation }),
      generation
    )
  })

  test('strictly parses a v1 edit snapshot', () => {
    const edit = {
      ...generation,
      model: 'grok-imagine-image-quality',
      operation: 'edit',
      input_image_count: 2,
      input_unit_price: 0.01,
      input_cost: 0.02,
      subtotal: 0.07,
      final_cost: 0.07,
    } as const

    assert.deepEqual(parseGrokImageBilling({ grok_image_billing: edit }), edit)
  })

  test('requires and preserves the image 2.0 quality tier', () => {
    const image20 = {
      ...generation,
      model: 'grok-imagine-image-2.0',
      quality: 'medium',
      output_unit_price: 0.06,
      output_cost: 0.06,
      subtotal: 0.06,
      final_cost: 0.06,
    } as const

    assert.deepEqual(
      parseGrokImageBilling({ grok_image_billing: image20 }),
      image20
    )
    assert.equal(
      parseGrokImageBilling({
        grok_image_billing: { ...image20, quality: undefined },
      }),
      null
    )
  })

  test('rejects invalid versions, models, operations, and incomplete payloads', () => {
    const invalid = [
      { ...generation, version: 2 },
      { ...generation, model: 'grok-imagine-video' },
      { ...generation, operation: 'generate' },
      { ...generation, aspect_ratio: undefined },
      { ...generation, output_unit_price: Number.NaN },
    ]

    for (const snapshot of invalid) {
      assert.equal(
        parseGrokImageBilling({ grok_image_billing: snapshot }),
        null
      )
    }
  })

  test('recognizes historical Grok image logs without inventing parameters', () => {
    const historical = {
      model_name: 'grok-imagine-image-quality',
      other: JSON.stringify({ model_price: 1 }),
    }

    assert.equal(isGrokImageLog(historical), true)
    assert.deepEqual(getGrokImageBillingState(historical), {
      kind: 'history',
      model: 'grok-imagine-image-quality',
    })
    assert.equal(
      isGrokImageLog({ model_name: 'grok-imagine-video', other: '{}' }),
      false
    )
  })
})
