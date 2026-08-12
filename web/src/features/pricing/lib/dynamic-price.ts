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
import { formatBillingCurrencyFromUSD } from '@/lib/currency'

import { TOKEN_UNIT_DIVISORS } from '../constants'
import type { PricingModel, TokenUnit } from '../types'
import {
  BILLING_PRICING_VARS,
  parseTiersFromExpr,
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
  type BillingVar,
  type ParsedTier,
} from './billing-expr'
import { getDisplayGroupRatio } from './model-helpers'

type DynamicPriceOptions = {
  tokenUnit: TokenUnit
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
  groupRatioMultiplier?: number
  billingCurrency?: PricingModel['billing_currency']
}

export type DynamicPriceEntry = {
  key: string
  field: string
  label: string
  shortLabel: string
  value: number
  formatted: string
  variable: BillingVar
}

export type DynamicPricingSummary = {
  tiers: ParsedTier[]
  tier: ParsedTier | null
  tierCount: number
  hasRequestRules: boolean
  isSpecialExpression: boolean
  rawExpression: string
  entries: DynamicPriceEntry[]
  primaryEntries: DynamicPriceEntry[]
  secondaryEntries: DynamicPriceEntry[]
}

export type TextModelCardPricingRow = {
  label: string
  input: string
  output: string
  cache: string
}

export type TextModelCardPricing = {
  kind: 'fixed' | 'tiered'
  explanationKey:
    | 'Billed by input, output, and cached Token usage'
    | 'Tiered by full input length'
  unitLabel: string
  rows: TextModelCardPricingRow[]
}

const PRIMARY_DYNAMIC_FIELDS = new Set(['inputPrice', 'outputPrice'])

const FIXED_TEXT_MODEL_CARD_IDS = new Set([
  'deepseek-v4-flash-202605',
  'deepseek-v4-pro-202606',
  'glm-5.2',
  'kimi-k3',
])

const TIERED_TEXT_MODEL_CARD_IDS = new Set([
  'minimax-m3',
  'qwen3.5-flash',
  'qwen3.5-plus',
])

const TIER_CARD_LABELS: Record<string, string> = {
  up_to_128k: '≤ 128K',
  '128k_to_256k': '128K–256K',
  '256k_to_1m': '256K–1M',
  up_to_512k: '≤ 512K',
  over_512k: '> 512K',
}

export function isDynamicPricingModel(model: PricingModel): boolean {
  return model.billing_mode === 'tiered_expr' && Boolean(model.billing_expr)
}

export function getDynamicDisplayGroupRatio(
  model: PricingModel,
  selectedGroup?: string
): number {
  return getDisplayGroupRatio(model, selectedGroup)
}

function applyRechargeRate(
  price: number,
  showWithRecharge: boolean,
  priceRate: number,
  usdExchangeRate: number
): number {
  if (!showWithRecharge) return price
  return (price * priceRate) / usdExchangeRate
}

export function formatDynamicUnitPrice(
  valuePerMillionTokens: number,
  options: DynamicPriceOptions
): string {
  const groupRatio = options.groupRatioMultiplier ?? 1
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const priceUSD =
    (valuePerMillionTokens * groupRatio) /
    TOKEN_UNIT_DIVISORS[options.tokenUnit]
  if (options.billingCurrency === 'CNY') {
    return `¥${formatCNYPrice(priceUSD)}`
  }
  const displayPrice = applyRechargeRate(
    priceUSD,
    options.showRechargePrice ?? false,
    priceRate,
    usdExchangeRate
  )

  return formatBillingCurrencyFromUSD(displayPrice, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}

function formatCNYPrice(value: number): string {
  const digits = Math.abs(value) >= 1 ? 4 : 6
  return String(Number(value.toFixed(digits)))
}

export function getDynamicPricingTiers(model: PricingModel): ParsedTier[] {
  if (!isDynamicPricingModel(model)) return []
  const { billingExpr } = splitBillingExprAndRequestRules(
    model.billing_expr || ''
  )
  return parseTiersFromExpr(billingExpr)
}

export function hasDynamicRequestRules(model: PricingModel): boolean {
  if (!isDynamicPricingModel(model)) return false
  const { requestRuleExpr } = splitBillingExprAndRequestRules(
    model.billing_expr || ''
  )
  return Boolean(tryParseRequestRuleExpr(requestRuleExpr || '')?.length)
}

export function getDynamicPriceEntries(
  tier: ParsedTier | null,
  options: DynamicPriceOptions
): DynamicPriceEntry[] {
  if (!tier) return []

  return BILLING_PRICING_VARS.flatMap((variable) => {
    if (!variable.field) return []
    const value = Number(tier[variable.field])
    if (!Number.isFinite(value) || value <= 0) return []

    return [
      {
        key: variable.key,
        field: variable.field,
        label: variable.label,
        shortLabel: variable.shortLabel,
        value,
        formatted: formatDynamicUnitPrice(value, options),
        variable,
      },
    ]
  }).sort((a, b) => {
    const aPrimary = PRIMARY_DYNAMIC_FIELDS.has(a.field)
    const bPrimary = PRIMARY_DYNAMIC_FIELDS.has(b.field)
    if (aPrimary !== bPrimary) return aPrimary ? -1 : 1
    return 0
  })
}

export function getDynamicPricingSummary(
  model: PricingModel,
  options: DynamicPriceOptions
): DynamicPricingSummary | null {
  if (!isDynamicPricingModel(model)) return null

  const tiers = getDynamicPricingTiers(model)
  const tier = tiers[0] || null
  const entries = getDynamicPriceEntries(tier, {
    ...options,
    billingCurrency: model.billing_currency,
  })
  const rawExpression = model.billing_expr || ''

  return {
    tiers,
    tier,
    tierCount: tiers.length,
    hasRequestRules: hasDynamicRequestRules(model),
    isSpecialExpression: rawExpression.trim().length > 0 && tiers.length === 0,
    rawExpression,
    entries,
    primaryEntries: entries.filter((entry) =>
      PRIMARY_DYNAMIC_FIELDS.has(entry.field)
    ),
    secondaryEntries: entries.filter(
      (entry) => !PRIMARY_DYNAMIC_FIELDS.has(entry.field)
    ),
  }
}

export function getTextModelCardPricing(
  model: PricingModel,
  options: DynamicPriceOptions
): TextModelCardPricing | null {
  const unitLabel = options.tokenUnit === 'K' ? '1K' : '1M'

  if (FIXED_TEXT_MODEL_CARD_IDS.has(model.model_name)) {
    return {
      kind: 'fixed',
      explanationKey: 'Billed by input, output, and cached Token usage',
      unitLabel,
      rows: [],
    }
  }

  if (
    !TIERED_TEXT_MODEL_CARD_IDS.has(model.model_name) ||
    !isDynamicPricingModel(model)
  ) {
    return null
  }

  const rows = getDynamicPricingTiers(model).map((tier) => {
    const entries = getDynamicPriceEntries(tier, {
      ...options,
      billingCurrency: model.billing_currency,
    })
    const priceFor = (field: string) =>
      entries.find((entry) => entry.field === field)?.formatted ?? '—'

    return {
      label: TIER_CARD_LABELS[tier.label] ?? tier.label,
      input: priceFor('inputPrice'),
      output: priceFor('outputPrice'),
      cache: priceFor('cacheReadPrice'),
    }
  })

  return {
    kind: 'tiered',
    explanationKey: 'Tiered by full input length',
    unitLabel,
    rows,
  }
}
