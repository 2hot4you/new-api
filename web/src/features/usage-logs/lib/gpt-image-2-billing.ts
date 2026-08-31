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

export interface GPTImage2BillingInput {
  promptTokens: number
  completionTokens: number
  imageTokens?: number
  quota: number
  quotaPerUnit: number
  modelRatio: number | undefined
  completionRatio: number | undefined
  imageRatio?: number
  groupRatio: number | undefined
  userGroupRatio?: number
}

export interface GPTImage2BillingDetails {
  inputTokens: number
  textInputTokens: number
  imageInputTokens: number
  outputTokens: number
  inputUnitPriceUSD: number
  imageInputUnitPriceUSD: number
  outputUnitPriceUSD: number
  textInputCostUSD: number
  imageInputCostUSD: number
  inputCostUSD: number
  outputCostUSD: number
  subtotalUSD: number
  groupRatio: number
  finalCostUSD: number
  savingsPercent: number
}

function isNonNegativeFinite(value: number | undefined): value is number {
  return value != null && Number.isFinite(value) && value >= 0
}

export function buildGPTImage2Billing(
  input: GPTImage2BillingInput
): GPTImage2BillingDetails | null {
  const imageTokens = input.imageTokens ?? 0
  const imageRatio = input.imageRatio ?? 1
  if (
    !isNonNegativeFinite(input.promptTokens) ||
    !isNonNegativeFinite(input.completionTokens) ||
    !isNonNegativeFinite(input.quota) ||
    !isNonNegativeFinite(imageTokens) ||
    imageTokens > input.promptTokens ||
    !isNonNegativeFinite(imageRatio) ||
    !Number.isFinite(input.quotaPerUnit) ||
    input.quotaPerUnit <= 0 ||
    !Number.isFinite(input.modelRatio) ||
    (input.modelRatio ?? 0) <= 0 ||
    !Number.isFinite(input.completionRatio) ||
    (input.completionRatio ?? 0) < 0
  ) {
    return null
  }

  const inputUnitPriceUSD = (input.modelRatio as number) * 2
  const imageInputUnitPriceUSD = inputUnitPriceUSD * imageRatio
  const outputUnitPriceUSD =
    inputUnitPriceUSD * (input.completionRatio as number)
  const textInputTokens = input.promptTokens - imageTokens
  const textInputCostUSD = (textInputTokens * inputUnitPriceUSD) / 1_000_000
  const imageInputCostUSD = (imageTokens * imageInputUnitPriceUSD) / 1_000_000
  const inputCostUSD = textInputCostUSD + imageInputCostUSD
  const outputCostUSD =
    (input.completionTokens * outputUnitPriceUSD) / 1_000_000
  const subtotalUSD = inputCostUSD + outputCostUSD
  const hasUserRatio =
    Number.isFinite(input.userGroupRatio) && input.userGroupRatio !== -1
  const groupRatio = hasUserRatio
    ? (input.userGroupRatio as number)
    : input.groupRatio
  if (!isNonNegativeFinite(groupRatio)) return null

  const finalCostUSD = input.quota / input.quotaPerUnit
  const savingsPercent =
    subtotalUSD > 0 ? (1 - finalCostUSD / subtotalUSD) * 100 : 0

  return {
    inputTokens: input.promptTokens,
    textInputTokens,
    imageInputTokens: imageTokens,
    outputTokens: input.completionTokens,
    inputUnitPriceUSD,
    imageInputUnitPriceUSD,
    outputUnitPriceUSD,
    textInputCostUSD,
    imageInputCostUSD,
    inputCostUSD,
    outputCostUSD,
    subtotalUSD,
    groupRatio,
    finalCostUSD,
    savingsPercent,
  }
}
