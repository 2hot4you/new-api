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

import type { MoliiGrokPricing } from '../../types'
import { buildGrokPricingRows } from '../grok-pricing-table'

describe('Grok marketplace pricing rows', () => {
  test('sorts image resolutions and repeats the per-image input price', () => {
    const pricing: MoliiGrokPricing = {
      kind: 'image',
      output_unit: 'image',
      output_prices: { '2k': 0.07, '1k': 0.05 },
      image_input_unit: 'image',
      image_input_price: 0.01,
    }

    assert.deepEqual(buildGrokPricingRows(pricing), [
      {
        resolution: '1k',
        outputPrice: 0.05,
        imageInputPrice: 0.01,
      },
      {
        resolution: '2k',
        outputPrice: 0.07,
        imageInputPrice: 0.01,
      },
    ])
  })

  test('keeps both image and video input prices for legacy video', () => {
    const pricing: MoliiGrokPricing = {
      kind: 'video',
      output_unit: 'second',
      output_prices: { '720p': 0.07, '480p': 0.05 },
      image_input_unit: 'image',
      image_input_price: 0.002,
      video_input_unit: 'second',
      video_input_price: 0.01,
    }

    assert.deepEqual(buildGrokPricingRows(pricing), [
      {
        resolution: '480p',
        outputPrice: 0.05,
        imageInputPrice: 0.002,
        videoInputPrice: 0.01,
      },
      {
        resolution: '720p',
        outputPrice: 0.07,
        imageInputPrice: 0.002,
        videoInputPrice: 0.01,
      },
    ])
  })

  test('orders image 2.0 quality and resolution tiers', () => {
    const pricing: MoliiGrokPricing = {
      kind: 'image',
      output_unit: 'image',
      output_prices: {
        'medium/2k': 0.08,
        'low/2k': 0.06,
        'medium/1k': 0.06,
        'low/1k': 0.04,
      },
      image_input_unit: 'image',
      image_input_price: 0.01,
    }

    assert.deepEqual(
      buildGrokPricingRows(pricing).map((row) => row.resolution),
      ['Low · 1K', 'Low · 2K', 'Medium · 1K', 'Medium · 2K']
    )
  })

  test('orders every Video 1.5 resolution without inventing video input pricing', () => {
    const pricing: MoliiGrokPricing = {
      kind: 'video',
      output_unit: 'second',
      output_prices: { '1080p': 0.25, '480p': 0.08, '720p': 0.14 },
      image_input_unit: 'image',
      image_input_price: 0.01,
    }

    const rows = buildGrokPricingRows(pricing)

    assert.deepEqual(
      rows.map((row) => row.resolution),
      ['480p', '720p', '1080p']
    )
    assert.equal(rows[0].imageInputPrice, 0.01)
    assert.equal(rows[0].videoInputPrice, undefined)
  })

  test('returns no rows when the backend exposes no output prices', () => {
    const pricing: MoliiGrokPricing = {
      kind: 'video',
      output_unit: 'second',
      output_prices: {},
    }

    assert.deepEqual(buildGrokPricingRows(pricing), [])
  })
})
