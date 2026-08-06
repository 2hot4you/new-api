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
import type { MoliiGrokPricing } from '../types'

export type GrokPricingTableRow = {
  resolution: string
  outputPrice: number
  imageInputPrice?: number
  videoInputPrice?: number
}

const RESOLUTION_ORDER = new Map([
  ['1k', 0],
  ['2k', 1],
  ['480p', 0],
  ['720p', 1],
  ['1080p', 2],
])

function resolutionRank(resolution: string): number {
  return RESOLUTION_ORDER.get(resolution.toLowerCase()) ?? 100
}

export function buildGrokPricingRows(
  pricing: MoliiGrokPricing
): GrokPricingTableRow[] {
  return Object.entries(pricing.output_prices)
    .sort(([left], [right]) => {
      const rankDifference = resolutionRank(left) - resolutionRank(right)
      return rankDifference || left.localeCompare(right)
    })
    .map(([resolution, outputPrice]) => ({
      resolution,
      outputPrice,
      ...(pricing.image_input_price != null
        ? { imageInputPrice: pricing.image_input_price }
        : {}),
      ...(pricing.video_input_price != null
        ? { videoInputPrice: pricing.video_input_price }
        : {}),
    }))
}
