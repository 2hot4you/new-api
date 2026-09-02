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
import type {
  BillingUsageSchema,
  BillingUsageUnit,
  PricingModel,
  TokenUnit,
} from '../types'
import {
  BILLING_PRICING_VARS,
  parseTaskTiersFromExpr,
  parseTiersFromExpr,
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
  type BillingVar,
  type ParsedTaskTier,
  type ParsedTier,
  type RequestRuleGroup,
  type TierCondition,
} from './billing-expr'
import { getDisplayGroupRatio } from './model-helpers'
import {
  evaluateTaskVisualConfig,
  getTaskNumberFields,
  tryParseTaskVisualConfig,
} from './task-expr'

type DynamicPriceOptions = {
  tokenUnit: TokenUnit
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
  groupRatioMultiplier?: number
  billingCurrency?: PricingModel['billing_currency']
  usageSchema?: BillingUsageSchema
}

export type DynamicPriceLabelKind = 'i18n' | 'schema'

export type DynamicPriceEntry = {
  key: string
  field: string
  label: string
  shortLabel: string
  labelKind?: DynamicPriceLabelKind
  value: number
  formatted: string
  formattedRange?: string
  unit?: 'token' | BillingUsageUnit | 'request'
  variable?: BillingVar
  description?: string | Record<string, string>
}

export type CardExamplePrice = {
  label: string
  formatted: string
}

export type DynamicPricingTier = ParsedTier | ParsedTaskTier

export type DynamicPricingSummary = {
  tiers: DynamicPricingTier[]
  tier: DynamicPricingTier | null
  tierCount: number
  hasRequestRules: boolean
  isSpecialExpression: boolean
  rawExpression: string
  entries: DynamicPriceEntry[]
  primaryEntries: DynamicPriceEntry[]
  secondaryEntries: DynamicPriceEntry[]
  isTaskUsage: boolean
}

export type DynamicPricingStrategyKind =
  | 'generic'
  | 'input_length'
  | 'time_window'
  | 'input_length_and_time'
  | 'request_conditions'

export type DynamicPricingTimeRule = {
  label: string
  timezone: string
  multiplier: string
}

export type DynamicPricingStrategy = {
  kind: DynamicPricingStrategyKind
  tierRanges: string[]
  timeRules: DynamicPricingTimeRule[]
}

export type DynamicPricingTierPresentation =
  | { kind: 'input_length'; range: string }
  | { kind: 'base_period' }
  | { kind: 'default_price' }
  | { kind: 'custom'; label: string }

type TierLabelTranslator = (
  key: string,
  options?: {
    range: string
    interpolation?: { escapeValue: boolean }
  }
) => string

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

function isTaskPricingTier(tier: DynamicPricingTier): tier is ParsedTaskTier {
  return (
    Object.hasOwn(tier, 'unitPrices') &&
    typeof (tier as ParsedTaskTier).unitPrices === 'object'
  )
}

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

export function hasTaskUsageSchema(model: PricingModel): boolean {
  return Object.keys(model.billing_usage_schema ?? {}).length > 0
}

export function isTaskUsagePricingModel(model: PricingModel): boolean {
  return model.billing_mode === 'tiered_expr' && hasTaskUsageSchema(model)
}

export function isUnconfiguredTaskUsageModel(model: PricingModel): boolean {
  return (
    model.quota_type !== 1 &&
    hasTaskUsageSchema(model) &&
    !isDynamicPricingModel(model)
  )
}

export function getTaskPricingUnit(
  model: PricingModel
): BillingUsageUnit | null {
  const primaryField = getTaskNumberFields(model.billing_usage_schema)[0]
  return primaryField?.[1].unit ?? null
}

export function getTaskUsageQuantityUnitLabelKey(
  unit: BillingUsageUnit | undefined
): string {
  if (unit === 'second') return 's'
  if (unit === 'token') return 'token (unit)'
  if (unit === 'credit') return 'credit'
  return 'unit'
}

export function getTaskUsagePriceUnitLabelKey(
  unit: BillingUsageUnit | undefined
): string {
  if (unit === 'second') return 'second'
  if (unit === 'token') return '1M token'
  if (unit === 'credit') return 'credit'
  return 'unit'
}

export function getDynamicPriceUnitLabelKey(
  entry: DynamicPriceEntry
): string | null {
  if (entry.unit === 'second') return 's'
  if (entry.unit === 'count') return 'unit'
  if (entry.unit === 'credit') return 'credit'
  if (entry.unit === 'token' && !entry.variable) return '1M token'
  if (entry.unit === 'request') return 'request'
  return null
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

export function formatTaskUsageUnitPrice(
  valuePerUnit: number,
  options: DynamicPriceOptions
): string {
  const groupRatio = options.groupRatioMultiplier ?? 1
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const priceUSD = valuePerUnit * groupRatio
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

function formatTokenBoundary(value: number): string {
  if (value >= 1_000_000) {
    return `${Number((value / 1_000_000).toFixed(1))}M`
  }
  if (value >= 1000) return `${Number((value / 1000).toFixed(1))}K`
  return String(value)
}

function upperLengthCondition(
  conditions: TierCondition[]
): TierCondition | undefined {
  return conditions.find(
    (condition) =>
      condition.var === 'len' && (condition.op === '<' || condition.op === '<=')
  )
}

function lowerLengthCondition(
  conditions: TierCondition[]
): TierCondition | undefined {
  return conditions.find(
    (condition) =>
      condition.var === 'len' && (condition.op === '>' || condition.op === '>=')
  )
}

function formatLengthTierRanges(tiers: ParsedTier[]): string[] {
  let previousUpper: TierCondition | undefined

  return tiers.map((tier) => {
    const lower = lowerLengthCondition(tier.conditions)
    const upper = upperLengthCondition(tier.conditions)
    let label = tier.label

    if (lower && upper) {
      label = `${formatTokenBoundary(lower.value)}–${formatTokenBoundary(upper.value)}`
    } else if (upper && previousUpper) {
      label = `${formatTokenBoundary(previousUpper.value)}–${formatTokenBoundary(upper.value)}`
    } else if (upper) {
      label = `${upper.op === '<=' ? '≤' : '<'} ${formatTokenBoundary(upper.value)}`
    } else if (lower) {
      label = `${lower.op === '>=' ? '≥' : '>'} ${formatTokenBoundary(lower.value)}`
    } else if (previousUpper) {
      label = `${previousUpper.op === '<=' ? '>' : '≥'} ${formatTokenBoundary(previousUpper.value)}`
    }

    if (upper) previousUpper = upper
    return label
  })
}

function padHour(value: string): string {
  const hour = Number(value)
  if (!Number.isInteger(hour) || hour < 0 || hour > 24) return value
  return String(hour).padStart(2, '0')
}

function parseTimeRule(group: RequestRuleGroup): DynamicPricingTimeRule | null {
  if (group.conditions.length === 0) return null
  if (group.conditions.some((condition) => condition.source !== 'time')) {
    return null
  }

  const conditions = group.conditions.filter(
    (condition) => condition.source === 'time'
  )
  const first = conditions[0]
  if (!first || first.timeFunc !== 'hour') return null
  if (
    conditions.some(
      (condition) =>
        condition.timeFunc !== 'hour' || condition.timezone !== first.timezone
    )
  ) {
    return null
  }

  const range = conditions.find((condition) => condition.mode === 'range')
  if (range) {
    return {
      label: `${padHour(range.rangeStart)}:00–${padHour(range.rangeEnd)}:00`,
      timezone: range.timezone || 'UTC',
      multiplier: group.multiplier,
    }
  }

  const start = conditions.find(
    (condition) => condition.mode === 'gte' || condition.mode === 'gt'
  )
  const end = conditions.find(
    (condition) => condition.mode === 'lt' || condition.mode === 'lte'
  )
  if (!start || !end) return null

  return {
    label: `${padHour(start.value)}:00–${padHour(end.value)}:00`,
    timezone: first.timezone || 'UTC',
    multiplier: group.multiplier,
  }
}

export function getDynamicPricingStrategy(
  expression: string
): DynamicPricingStrategy {
  const split = splitBillingExprAndRequestRules(expression || '')
  const tiers = parseTiersFromExpr(split.billingExpr)
  const ruleGroups = tryParseRequestRuleExpr(split.requestRuleExpr || '') || []
  const hasInputLength = tiers.some((tier) =>
    tier.conditions.some((condition) => condition.var === 'len')
  )
  const hasTimeRules = ruleGroups.some((group) =>
    group.conditions.some((condition) => condition.source === 'time')
  )
  const hasOtherRequestRules = ruleGroups.some((group) =>
    group.conditions.some((condition) => condition.source !== 'time')
  )
  const timeRules = ruleGroups.flatMap((group) => {
    const rule = parseTimeRule(group)
    return rule ? [rule] : []
  })

  let kind: DynamicPricingStrategyKind = 'generic'
  if (hasInputLength && hasTimeRules && !hasOtherRequestRules) {
    kind = 'input_length_and_time'
  } else if (hasInputLength && ruleGroups.length === 0) {
    kind = 'input_length'
  } else if (hasTimeRules && !hasOtherRequestRules) {
    kind = 'time_window'
  } else if (ruleGroups.length > 0) {
    kind = 'request_conditions'
  }

  return {
    kind,
    tierRanges: hasInputLength ? formatLengthTierRanges(tiers) : [],
    timeRules,
  }
}

export function getDynamicPricingTierPresentation(
  expression: string,
  tier: Pick<ParsedTier, 'label'>,
  tierIndex: number
): DynamicPricingTierPresentation {
  const strategy = getDynamicPricingStrategy(expression)
  const range = strategy.tierRanges[tierIndex]
  if (range) return { kind: 'input_length', range }

  const internalLabel = tier.label.trim().toLocaleLowerCase()
  if (
    !internalLabel ||
    internalLabel === 'base' ||
    internalLabel === 'default'
  ) {
    return strategy.timeRules.length > 0
      ? { kind: 'base_period' }
      : { kind: 'default_price' }
  }

  return { kind: 'custom', label: tier.label }
}

export function formatDynamicPricingTierLabel(
  expression: string,
  tier: Pick<ParsedTier, 'label'>,
  tierIndex: number,
  t: TierLabelTranslator
): string {
  const presentation = getDynamicPricingTierPresentation(
    expression,
    tier,
    tierIndex
  )
  if (presentation.kind === 'input_length') {
    return t('Single-request input {{range}} Tokens', {
      range: presentation.range,
      interpolation: { escapeValue: false },
    })
  }
  if (presentation.kind === 'base_period') return t('Base-period price')
  if (presentation.kind === 'default_price') return t('Default price')
  return presentation.label
}

export function getDynamicPricingTiers(
  model: PricingModel
): DynamicPricingTier[] {
  if (!isDynamicPricingModel(model)) return []
  const { billingExpr } = splitBillingExprAndRequestRules(
    model.billing_expr || ''
  )
  if (isTaskUsagePricingModel(model)) {
    return parseTaskTiersFromExpr(billingExpr, model.billing_usage_schema)
  }
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
  tier: DynamicPricingTier | null,
  options: DynamicPriceOptions
): DynamicPriceEntry[] {
  if (!tier) return []

  if (isTaskPricingTier(tier) && options.usageSchema) {
    const usageEntries: DynamicPriceEntry[] = getTaskNumberFields(
      options.usageSchema
    ).flatMap(([field, definition]) => {
      const value = Number(tier.unitPrices[field])
      if (!Number.isFinite(value) || value <= 0 || !definition.unit) return []
      return [
        {
          key: field,
          field,
          label: field,
          shortLabel: field,
          labelKind: 'schema' as const,
          value,
          formatted: formatTaskUsageUnitPrice(value, options),
          unit: definition.unit,
          description: definition.description,
        },
      ]
    })
    if (tier.constant > 0) {
      usageEntries.push({
        key: 'constant',
        field: 'constant',
        label: 'Base charge',
        shortLabel: 'Base',
        labelKind: 'i18n',
        value: tier.constant,
        formatted: formatTaskUsageUnitPrice(tier.constant, options),
        unit: 'request',
      })
    }
    return usageEntries
  }

  return BILLING_PRICING_VARS.flatMap((variable) => {
    if (!variable.field) return []
    const value = Number((tier as ParsedTier)[variable.field])
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
        labelKind: 'i18n' as const,
        unit: 'token' as const,
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
  const isTaskUsage = isTaskUsagePricingModel(model)
  const tier = isTaskUsage ? (tiers.at(-1) ?? null) : (tiers[0] ?? null)
  let entries = getDynamicPriceEntries(tier, {
    ...options,
    billingCurrency: model.billing_currency,
    usageSchema: model.billing_usage_schema,
  })

  if (isTaskUsage) {
    const priceRanges = new Map<string, { min: number; max: number }>()
    for (const [field] of getTaskNumberFields(model.billing_usage_schema)) {
      let min = Number.POSITIVE_INFINITY
      let max = Number.NEGATIVE_INFINITY
      for (const taskTier of tiers) {
        if (!isTaskPricingTier(taskTier)) continue
        const value = Number(taskTier.unitPrices[field])
        if (!Number.isFinite(value) || value <= 0) continue
        min = Math.min(min, value)
        max = Math.max(max, value)
      }
      if (Number.isFinite(min) && Number.isFinite(max)) {
        priceRanges.set(field, { min, max })
      }
    }
    entries = entries.map((entry) => {
      const range = priceRanges.get(entry.field)
      if (!range || range.min === range.max) return entry
      return {
        ...entry,
        formattedRange: `${formatTaskUsageUnitPrice(range.min, options)}–${formatTaskUsageUnitPrice(range.max, options)}`,
      }
    })
  }
  const rawExpression = model.billing_expr || ''

  return {
    tiers,
    tier,
    tierCount: tiers.length,
    hasRequestRules: hasDynamicRequestRules(model),
    isSpecialExpression: rawExpression.trim().length > 0 && tiers.length === 0,
    rawExpression,
    entries,
    primaryEntries: isTaskUsage
      ? entries.filter((entry) => entry.unit !== 'request')
      : entries.filter((entry) => PRIMARY_DYNAMIC_FIELDS.has(entry.field)),
    secondaryEntries: isTaskUsage
      ? entries.filter((entry) => entry.unit === 'request')
      : entries.filter((entry) => !PRIMARY_DYNAMIC_FIELDS.has(entry.field)),
    isTaskUsage,
  }
}

export function getCardExamplePrice(
  model: PricingModel,
  options: DynamicPriceOptions
): CardExamplePrice | null {
  if (!isTaskUsagePricingModel(model)) return null
  const schema = model.billing_usage_schema
  const firstExample = model.billing_usage_examples?.[0]
  if (!schema || !firstExample) return null
  const { billingExpr } = splitBillingExprAndRequestRules(
    model.billing_expr || ''
  )
  const config = tryParseTaskVisualConfig(billingExpr, schema)
  if (!config) return null
  const result = evaluateTaskVisualConfig(config, firstExample.facts, schema)
  if (!result) return null
  return {
    label: firstExample.label,
    formatted: formatTaskUsageUnitPrice(result.total, options),
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
    !isDynamicPricingModel(model) ||
    isTaskUsagePricingModel(model)
  ) {
    return null
  }

  const rows = (getDynamicPricingTiers(model) as ParsedTier[]).map((tier) => {
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
