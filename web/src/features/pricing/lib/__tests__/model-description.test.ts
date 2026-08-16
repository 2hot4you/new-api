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

import type { PricingModel } from '../../types'
import { getPricingModelDescription } from '../model-description'

function makeModel(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: [],
    ...overrides,
  }
}

describe('pricing model descriptions', () => {
  test('uses the English description for English locales', () => {
    const description = getPricingModelDescription(
      makeModel({
        description: '中文简介',
        description_en: 'English description',
      }),
      'en-US'
    )

    assert.equal(description, 'English description')
  })

  test('uses the Chinese description for non-English locales', () => {
    const description = getPricingModelDescription(
      makeModel({
        description: '中文简介',
        description_en: 'English description',
      }),
      'zh-CN'
    )

    assert.equal(description, '中文简介')
  })

  test('falls back to Chinese when the English description is missing', () => {
    const description = getPricingModelDescription(
      makeModel({ description: '中文简介' }),
      'en'
    )

    assert.equal(description, '中文简介')
  })

  test('returns undefined when neither persisted description is configured', () => {
    const description = getPricingModelDescription(makeModel({}), 'ja')

    assert.equal(description, undefined)
  })
})
