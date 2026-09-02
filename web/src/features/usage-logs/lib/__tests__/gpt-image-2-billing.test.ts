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
import { test } from 'vitest'

import { buildGPTImage2Billing } from '../gpt-image-2-billing'

test('calculates immutable token prices, settled total, and savings', () => {
  const billing = buildGPTImage2Billing({
    promptTokens: 120,
    completionTokens: 300,
    quota: 576,
    quotaPerUnit: 500000,
    modelRatio: 1,
    completionRatio: 2,
    groupRatio: 0.8,
    userGroupRatio: -1,
    imageTokens: 0,
    imageRatio: 1,
  })

  assert.ok(billing)
  assert.equal(billing.inputUnitPriceUSD, 2)
  assert.equal(billing.outputUnitPriceUSD, 4)
  assert.equal(billing.inputCostUSD, 0.00024)
  assert.equal(billing.outputCostUSD, 0.0012)
  assert.ok(Math.abs(billing.subtotalUSD - 0.00144) < 1e-12)
  assert.equal(billing.groupRatio, 0.8)
  assert.equal(billing.finalCostUSD, 0.001152)
  assert.ok(Math.abs(billing.savingsPercent - 20) < 1e-9)
})

test('uses a finite user-specific group ratio and rejects incomplete rates', () => {
  assert.equal(
    buildGPTImage2Billing({
      promptTokens: 1,
      completionTokens: 1,
      quota: 1,
      quotaPerUnit: 500000,
      modelRatio: 1,
      completionRatio: 2,
      groupRatio: 0.8,
      userGroupRatio: 0.5,
      imageTokens: 0,
      imageRatio: 1,
    })?.groupRatio,
    0.5
  )
  assert.equal(
    buildGPTImage2Billing({
      promptTokens: 1,
      completionTokens: 1,
      quota: 1,
      quotaPerUnit: 0,
      modelRatio: 1,
      completionRatio: 2,
      groupRatio: 1,
      imageTokens: 0,
      imageRatio: 1,
    }),
    null
  )
})

test('prices image input tokens with their recorded image ratio', () => {
  const billing = buildGPTImage2Billing({
    promptTokens: 100,
    completionTokens: 200,
    imageTokens: 40,
    quota: 520,
    quotaPerUnit: 500000,
    modelRatio: 1,
    completionRatio: 2,
    imageRatio: 1.5,
    groupRatio: 1,
  })

  assert.ok(billing)
  assert.equal(billing.textInputTokens, 60)
  assert.equal(billing.imageInputTokens, 40)
  assert.equal(billing.imageInputUnitPriceUSD, 3)
  assert.equal(billing.textInputCostUSD, 0.00012)
  assert.equal(billing.imageInputCostUSD, 0.00012)
  assert.ok(Math.abs(billing.subtotalUSD - 0.00104) < 1e-12)
})
