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

function parseOutputTier(key: string): {
  quality: string
  resolution: string
  label: string
} {
  const [quality, resolution] = key.includes('/')
    ? key.split('/', 2)
    : ['', key]
  if (!quality) return { quality: '', resolution, label: resolution }
  const qualityLabel = quality.charAt(0).toUpperCase() + quality.slice(1)
  return {
    quality,
    resolution,
    label: `${qualityLabel} · ${resolution.toUpperCase()}`,
  }
}

export function buildGrokPricingRows(
  pricing: MoliiGrokPricing
): GrokPricingTableRow[] {
  return Object.entries(pricing.output_prices)
    .sort(([left], [right]) => {
      const leftTier = parseOutputTier(left)
      const rightTier = parseOutputTier(right)
      const qualityDifference = leftTier.quality.localeCompare(
        rightTier.quality
      )
      const rankDifference =
        resolutionRank(leftTier.resolution) -
        resolutionRank(rightTier.resolution)
      return qualityDifference || rankDifference || left.localeCompare(right)
    })
    .map(([key, outputPrice]) => {
      const tier = parseOutputTier(key)
      return {
        resolution: tier.label,
        outputPrice,
        ...(pricing.image_input_price != null
          ? { imageInputPrice: pricing.image_input_price }
          : {}),
        ...(pricing.video_input_price != null
          ? { videoInputPrice: pricing.video_input_price }
          : {}),
      }
    })
}
