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
import type { PricingModel, TokenUnit } from '../types'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from './dynamic-price'
import { formatPrice, formatRequestPrice } from './price'
import { formatVideoPrice } from './video-pricing'

type CompactPricingOptions = {
  tokenUnit: TokenUnit
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
  selectedGroup?: string
}

export type CompactPricingSummary =
  | {
      kind: 'token'
      items: Array<{ label: 'Input' | 'Output' | 'Cached'; value: string }>
      unit: string
    }
  | {
      kind: 'request'
      value: string
      unit: 'request'
    }
  | {
      kind: 'tiered'
      label: 'Tiered pricing'
      from?: string
      unit: '1,000,000 Token' | '1,000 Token' | 'image' | 'second'
    }

function formatDirectCNY(value: number): string {
  const digits = Math.abs(value) >= 1 ? 4 : 6
  return `¥${Number(value.toFixed(digits))}`
}

export function getCompactPricingSummary(
  model: PricingModel,
  options: CompactPricingOptions
): CompactPricingSummary {
  const tokenUnit = options.tokenUnit
  const tokenUnitLabel = tokenUnit === 'K' ? '1,000 Token' : '1,000,000 Token'
  const showRechargePrice = options.showRechargePrice ?? false
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1

  if (model.molii_grok_pricing) {
    const prices = Object.values(model.molii_grok_pricing.output_prices).filter(
      (price) => Number.isFinite(price) && price >= 0
    )
    const minimum = prices.length > 0 ? Math.min(...prices) : undefined
    return {
      kind: 'tiered',
      label: 'Tiered pricing',
      ...(minimum == null ? {} : { from: formatDirectCNY(minimum) }),
      unit: model.molii_grok_pricing.output_unit,
    }
  }

  if (model.video_pricing) {
    const prices = model.video_pricing.rows.flatMap((row) => [
      row.without_video,
      row.with_video,
    ])
    const minimum = prices.length > 0 ? Math.min(...prices) : undefined
    return {
      kind: 'tiered',
      label: 'Tiered pricing',
      ...(minimum == null
        ? {}
        : { from: formatVideoPrice(minimum, tokenUnit) }),
      unit: tokenUnitLabel,
    }
  }

  const dynamic = getDynamicPricingSummary(model, {
    tokenUnit,
    showRechargePrice,
    priceRate,
    usdExchangeRate,
    groupRatioMultiplier: getDynamicDisplayGroupRatio(
      model,
      options.selectedGroup
    ),
  })
  if (dynamic) {
    return {
      kind: 'tiered',
      label: 'Tiered pricing',
      ...(dynamic.primaryEntries[0]?.formatted
        ? { from: dynamic.primaryEntries[0].formatted }
        : {}),
      unit: tokenUnitLabel,
    }
  }

  if (model.quota_type === 1) {
    return {
      kind: 'request',
      value: formatRequestPrice(
        model,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        options.selectedGroup
      ),
      unit: 'request',
    }
  }

  const items: Array<{
    label: 'Input' | 'Output' | 'Cached'
    value: string
  }> = [
    {
      label: 'Input',
      value: formatPrice(
        model,
        'input',
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        options.selectedGroup
      ),
    },
    {
      label: 'Output',
      value: formatPrice(
        model,
        'output',
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        options.selectedGroup
      ),
    },
  ]
  if (model.cache_ratio != null) {
    items.push({
      label: 'Cached',
      value: formatPrice(
        model,
        'cache',
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        options.selectedGroup
      ),
    })
  }

  return { kind: 'token', items, unit: tokenUnitLabel }
}
