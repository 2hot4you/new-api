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
  getCardExamplePrice,
  getDynamicPriceUnitLabelKey,
  getDynamicPricingSummary,
  getDynamicPricingStrategy,
  hasTaskUsageSchema,
  type DynamicPricingStrategy,
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
      kind: 'task'
      label: string
      description?: string | Record<string, string>
      value: string
      unit: string
      example?: string
    }
  | { kind: 'task-unconfigured' }
  | { kind: 'special'; expression: string }
  | {
      kind: 'tiered'
      label:
        | 'Tiered pricing'
        | 'Tiered by per-request input Tokens'
        | 'Priced by request time'
        | 'Priced by input Tokens and request time'
      detail?: string
      noteKey?: 'Other times use the base price'
      from?: string
      unit: '1M' | '1K' | 'image' | 'second'
    }

function formatDirectCNY(value: number): string {
  const digits = Math.abs(value) >= 1 ? 4 : 6
  return `¥${Number(value.toFixed(digits))}`
}

function pricingStrategyLabel(
  strategy: DynamicPricingStrategy
): Extract<CompactPricingSummary, { kind: 'tiered' }>['label'] {
  if (strategy.kind === 'input_length') {
    return 'Tiered by per-request input Tokens'
  }
  if (strategy.kind === 'time_window') return 'Priced by request time'
  if (strategy.kind === 'input_length_and_time') {
    return 'Priced by input Tokens and request time'
  }
  return 'Tiered pricing'
}

function formatTimeRuleDetail(strategy: DynamicPricingStrategy): string {
  if (strategy.timeRules.length === 0) return ''
  const first = strategy.timeRules[0]
  const sameRuleBasis = strategy.timeRules.every(
    (rule) =>
      rule.timezone === first.timezone && rule.multiplier === first.multiplier
  )
  if (sameRuleBasis) {
    return `${strategy.timeRules.map((rule) => rule.label).join(', ')} (${first.timezone}) ×${first.multiplier}`
  }
  return strategy.timeRules
    .map((rule) => `${rule.label} (${rule.timezone}) ×${rule.multiplier}`)
    .join('; ')
}

function pricingStrategyDetail(strategy: DynamicPricingStrategy): string {
  const parts: string[] = []
  if (strategy.tierRanges.length > 0) {
    parts.push(strategy.tierRanges.join(' / '))
  }
  const timeDetail = formatTimeRuleDetail(strategy)
  if (timeDetail) parts.push(timeDetail)
  return parts.join(' · ')
}

export function getCompactPricingSummary(
  model: PricingModel,
  options: CompactPricingOptions
): CompactPricingSummary {
  const tokenUnit = options.tokenUnit
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
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
    if (dynamic.isSpecialExpression) {
      return { kind: 'special', expression: dynamic.rawExpression }
    }
    if (dynamic.isTaskUsage) {
      const entry = dynamic.primaryEntries[0]
      if (entry) {
        const cardExample = getCardExamplePrice(model, {
          tokenUnit,
          showRechargePrice,
          priceRate,
          usdExchangeRate,
          groupRatioMultiplier: getDynamicDisplayGroupRatio(
            model,
            options.selectedGroup
          ),
        })
        return {
          kind: 'task',
          label: entry.label,
          description: entry.description,
          value: entry.formattedRange ?? entry.formatted,
          unit: getDynamicPriceUnitLabelKey(entry) ?? 'unit',
          ...(cardExample
            ? { example: `${cardExample.label} ≈ ${cardExample.formatted}` }
            : {}),
        }
      }
    } else {
      const items = dynamic.primaryEntries
        .filter(
          (entry) =>
            entry.field === 'inputPrice' ||
            entry.field === 'outputPrice' ||
            entry.field === 'cacheReadPrice'
        )
        .map((entry) => {
          let label: 'Input' | 'Output' | 'Cached' = 'Cached'
          if (entry.field === 'inputPrice') {
            label = 'Input'
          } else if (entry.field === 'outputPrice') {
            label = 'Output'
          }
          return { label, value: entry.formatted }
        })
      if (items.length > 0) {
        return { kind: 'token', items, unit: tokenUnitLabel }
      }
    }
    const strategy = getDynamicPricingStrategy(dynamic.rawExpression)
    const detail = pricingStrategyDetail(strategy)
    return {
      kind: 'tiered',
      label: pricingStrategyLabel(strategy),
      ...(detail ? { detail } : {}),
      ...(strategy.timeRules.length > 0
        ? { noteKey: 'Other times use the base price' as const }
        : {}),
      ...(dynamic.primaryEntries[0]?.formatted
        ? { from: dynamic.primaryEntries[0].formatted }
        : {}),
      unit: tokenUnitLabel,
    }
  }

  if (hasTaskUsageSchema(model)) {
    return { kind: 'task-unconfigured' }
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
