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
import { describe, test } from 'node:test'

import type { TFunction } from 'i18next'

import type { PricingModel } from '../../types'
import { getPricingModelDescription } from '../model-description'

const translate = ((key: string) => `translated:${key}`) as TFunction

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
  test('uses a custom catalog description before the localized fallback', () => {
    const description = getPricingModelDescription(
      makeModel({
        description: 'Custom description',
        description_i18n_key: 'Fallback description',
      }),
      translate
    )

    assert.equal(description, 'Custom description')
  })

  test('translates the backend-provided fallback key when no custom description exists', () => {
    const description = getPricingModelDescription(
      makeModel({ description_i18n_key: 'Seedance description' }),
      translate
    )

    assert.equal(description, 'translated:Seedance description')
  })
})
