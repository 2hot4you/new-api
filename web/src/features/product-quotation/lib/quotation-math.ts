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
import {
  BILLING_PRICING_VARS,
  splitBillingExprAndRequestRules,
  type ParsedTaskTier,
  type ParsedTier,
  type TierCondition,
} from '@/features/pricing/lib/billing-expr'
import {
  getDynamicPricingTiers,
  isDynamicPricingModel,
  isTaskUsagePricingModel,
  isUnconfiguredTaskUsageModel,
} from '@/features/pricing/lib/dynamic-price'
import type { PricingModel } from '@/features/pricing/types'

import type {
  BuildQuotationSnapshotInput,
  QuoteCurrency,
  QuoteModelSection,
  QuotePriceDimension,
  QuotePriceSource,
  QuoteProviderSection,
  QuotationSnapshot,
  QuotationValidation,
} from '../types'

const TOKEN_UNIT = '1M token'
const NUMBER_PATTERN = String.raw`(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?`
const PRICE_VARIABLE_PATTERN = String.raw`(?:p|c|cr|cc|cc1h|img|img_o|ai|ao)`
const PRICE_TERM_PATTERN = String.raw`${PRICE_VARIABLE_PATTERN}\s*\*\s*${NUMBER_PATTERN}`
const TIER_BODY_PATTERN = String.raw`${PRICE_TERM_PATTERN}(?:\s*\+\s*${PRICE_TERM_PATTERN})*`
const TIER_PATTERN = String.raw`tier\("[^"\r\n]*",\s*${TIER_BODY_PATTERN}\s*\)`
const TIER_CONDITION_PATTERN = String.raw`(?:p|c|len)\s*(?:<=|>=|<|>)\s*${NUMBER_PATTERN}`
const TIER_CONDITIONS_PATTERN = String.raw`${TIER_CONDITION_PATTERN}(?:\s*&&\s*${TIER_CONDITION_PATTERN})*`
const LOSSLESS_DYNAMIC_EXPRESSION = new RegExp(
  String.raw`^(?:v\d+:)?\s*(?:${TIER_CONDITIONS_PATTERN}\s*\?\s*${TIER_PATTERN}\s*:\s*)*${TIER_PATTERN}\s*$`
)
const DYNAMIC_TIER_TERMS = new RegExp(
  String.raw`tier\("[^"\r\n]*",\s*(${TIER_BODY_PATTERN})\s*\)`,
  'g'
)
const DYNAMIC_PRICE_TERM = new RegExp(
  String.raw`(${PRICE_VARIABLE_PATTERN})\s*\*\s*(${NUMBER_PATTERN})`,
  'g'
)

function parseLosslessDynamicTierTerms(
  expression: string
): Map<string, number>[] | null {
  if (!LOSSLESS_DYNAMIC_EXPRESSION.test(expression)) return null

  const tiers: Map<string, number>[] = []
  for (const tierMatch of expression.matchAll(DYNAMIC_TIER_TERMS)) {
    const terms = new Map<string, number>()
    for (const termMatch of tierMatch[1].matchAll(DYNAMIC_PRICE_TERM)) {
      const variable = termMatch[1]
      if (terms.has(variable)) return null
      terms.set(variable, Number(termMatch[2]))
    }
    tiers.push(terms)
  }
  return tiers.length > 0 ? tiers : null
}

function modelCurrency(model: PricingModel): QuoteCurrency {
  return model.billing_currency === 'CNY' ? 'CNY' : 'USD'
}

export function normalizeDiscount(
  discountInZhe: number | null | undefined
): number | null {
  if (
    typeof discountInZhe !== 'number' ||
    !Number.isFinite(discountInZhe) ||
    discountInZhe <= 0 ||
    discountInZhe > 10
  ) {
    return null
  }
  return discountInZhe / 10
}

export function resolveEffectiveDiscount(
  globalDiscount: number | null | undefined,
  providerDiscount: number | null | undefined
): number | null {
  return (
    normalizeDiscount(providerDiscount) ?? normalizeDiscount(globalDiscount)
  )
}

export function calculateSuggestedGroupRatio(
  actualRate: number,
  discountInZhe: number,
  fixedRate: number
): number | null {
  const discount = normalizeDiscount(discountInZhe)
  if (
    discount === null ||
    !Number.isFinite(actualRate) ||
    actualRate <= 0 ||
    !Number.isFinite(fixedRate) ||
    fixedRate <= 0
  ) {
    return null
  }
  return (actualRate * discount) / fixedRate
}

type DimensionInput = {
  key: string
  label: string
  sourceType: QuotePriceSource
  amount: number | null | undefined
  currency: QuoteCurrency
  unit: string
  condition?: string | null
}

function createDimension(
  input: DimensionInput,
  basisRatio: number | null,
  discountCoefficient: number | null
): QuotePriceDimension {
  const amountIsKnown =
    typeof input.amount === 'number' &&
    Number.isFinite(input.amount) &&
    input.amount >= 0
  const canCalculate =
    amountIsKnown &&
    basisRatio !== null &&
    Number.isFinite(basisRatio) &&
    basisRatio >= 0 &&
    discountCoefficient !== null
  const catalogAmount = amountIsKnown ? Number(input.amount) : null
  const sourceAmount =
    catalogAmount !== null && basisRatio !== null
      ? catalogAmount * basisRatio
      : null

  return {
    key: input.key,
    label: input.label,
    sourceType: input.sourceType,
    catalogAmount,
    sourceAmount,
    quoteAmount:
      canCalculate && sourceAmount !== null && discountCoefficient !== null
        ? sourceAmount * discountCoefficient
        : null,
    currency: input.currency,
    unit: input.unit,
    condition: input.condition ?? null,
    status: canCalculate ? 'ready' : 'needs_confirmation',
  }
}

function fixedTokenDimensions(
  model: PricingModel,
  basisRatio: number | null,
  discountCoefficient: number | null
): QuotePriceDimension[] {
  const base = Number(model.model_ratio) * 2
  const currency = modelCurrency(model)
  const optionalDimension = (
    key: string,
    label: string,
    ratio: number | null | undefined,
    multiplier = 1
  ): QuotePriceDimension[] => {
    if (
      ratio === null ||
      ratio === undefined ||
      !Number.isFinite(ratio) ||
      !Number.isFinite(multiplier)
    ) {
      return []
    }
    return [
      createDimension(
        {
          key,
          label,
          sourceType: 'fixed_token',
          amount: base * ratio * multiplier,
          currency,
          unit: TOKEN_UNIT,
        },
        basisRatio,
        discountCoefficient
      ),
    ]
  }

  return [
    createDimension(
      {
        key: 'input',
        label: 'Input',
        sourceType: 'fixed_token',
        amount: base,
        currency,
        unit: TOKEN_UNIT,
      },
      basisRatio,
      discountCoefficient
    ),
    createDimension(
      {
        key: 'output',
        label: 'Output',
        sourceType: 'fixed_token',
        amount: base * Number(model.completion_ratio),
        currency,
        unit: TOKEN_UNIT,
      },
      basisRatio,
      discountCoefficient
    ),
    ...optionalDimension('cache_read', 'Cache read', model.cache_ratio),
    ...optionalDimension(
      'cache_write',
      'Cache write',
      model.create_cache_ratio
    ),
    ...optionalDimension('image_input', 'Image input', model.image_ratio),
    ...optionalDimension('audio_input', 'Audio input', model.audio_ratio),
    ...optionalDimension(
      'audio_output',
      'Audio output',
      model.audio_ratio,
      model.audio_completion_ratio ?? Number.NaN
    ),
  ]
}

function genericTierCondition(tier: ParsedTier, requestRules: string): string {
  const tierConditions = tier.conditions
    .map(
      (condition: TierCondition) =>
        `${condition.var} ${condition.op} ${condition.value}`
    )
    .join(' and ')
  return [tier.label, tierConditions, requestRules].filter(Boolean).join('; ')
}

function taskTierCondition(tier: ParsedTaskTier, requestRules: string): string {
  const tierConditions = tier.conditions
    .map((condition) => `${condition.field} = ${condition.value}`)
    .join(' and ')
  return [tier.label, tierConditions, requestRules].filter(Boolean).join('; ')
}

function taskUsageDimensions(
  model: PricingModel,
  basisRatio: number | null,
  discountCoefficient: number | null
): QuotePriceDimension[] {
  const tiers = getDynamicPricingTiers(model) as ParsedTaskTier[]
  const { requestRuleExpr } = splitBillingExprAndRequestRules(
    model.billing_expr ?? ''
  )
  if (requestRuleExpr) {
    return [
      createConfirmationDimension(
        model,
        'task_usage',
        basisRatio,
        discountCoefficient
      ),
    ]
  }
  if (tiers.length === 0) {
    return [
      createDimension(
        {
          key: 'dynamic-unparsed',
          label: 'Dynamic pricing',
          sourceType: 'task_usage',
          amount: null,
          currency: modelCurrency(model),
          unit: 'variable',
          condition: model.billing_expr ?? null,
        },
        basisRatio,
        discountCoefficient
      ),
    ]
  }

  const currency = modelCurrency(model)
  return tiers.flatMap((tier, tierIndex) => {
    const condition = taskTierCondition(tier, requestRuleExpr)
    const dimensions: QuotePriceDimension[] = []
    if (tier.constant > 0) {
      dimensions.push(
        createDimension(
          {
            key: `task-tier-${tierIndex}-base`,
            label: 'Base charge',
            sourceType: 'task_usage',
            amount: tier.constant,
            currency,
            unit: 'request',
            condition,
          },
          basisRatio,
          discountCoefficient
        )
      )
    }
    for (const [field, amount] of Object.entries(tier.unitPrices).sort(
      ([left], [right]) => left.localeCompare(right)
    )) {
      const schemaUnit = model.billing_usage_schema?.[field]?.unit
      if (!schemaUnit) continue
      dimensions.push(
        createDimension(
          {
            key: `task-tier-${tierIndex}-${field}`,
            label: field,
            sourceType: 'task_usage',
            amount,
            currency,
            unit: schemaUnit === 'token' ? TOKEN_UNIT : schemaUnit,
            condition,
          },
          basisRatio,
          discountCoefficient
        )
      )
    }
    return dimensions
  })
}

function dynamicDimensions(
  model: PricingModel,
  basisRatio: number | null,
  discountCoefficient: number | null
): QuotePriceDimension[] {
  if (isTaskUsagePricingModel(model)) {
    return taskUsageDimensions(model, basisRatio, discountCoefficient)
  }

  const tiers = getDynamicPricingTiers(model) as ParsedTier[]
  const { billingExpr, requestRuleExpr } = splitBillingExprAndRequestRules(
    model.billing_expr ?? ''
  )
  const currency = modelCurrency(model)
  const parsedTierTerms = parseLosslessDynamicTierTerms(billingExpr)
  if (
    requestRuleExpr ||
    parsedTierTerms === null ||
    parsedTierTerms.length !== tiers.length
  ) {
    return [
      createConfirmationDimension(
        model,
        'dynamic',
        basisRatio,
        discountCoefficient
      ),
    ]
  }
  if (tiers.length === 0) {
    return [
      createDimension(
        {
          key: 'dynamic-unparsed',
          label: 'Dynamic pricing',
          sourceType: 'dynamic',
          amount: null,
          currency,
          unit: TOKEN_UNIT,
          condition: model.billing_expr ?? null,
        },
        basisRatio,
        discountCoefficient
      ),
    ]
  }

  const dimensions = tiers.flatMap((tier, tierIndex) => {
    const condition = genericTierCondition(tier, requestRuleExpr)
    const explicitTerms = parsedTierTerms[tierIndex]
    return BILLING_PRICING_VARS.flatMap((variable) => {
      if (!variable.field) return []
      if (!explicitTerms.has(variable.key)) return []
      const amount = explicitTerms.get(variable.key)
      if (amount === undefined || !Number.isFinite(amount) || amount < 0) {
        return []
      }
      return [
        createDimension(
          {
            key: `tier-${tierIndex}-${variable.field}`,
            label: variable.label,
            sourceType: 'dynamic',
            amount,
            currency,
            unit: TOKEN_UNIT,
            condition,
          },
          basisRatio,
          discountCoefficient
        ),
      ]
    })
  })
  if (dimensions.length > 0) return dimensions
  return [
    createDimension(
      {
        key: 'dynamic-unparsed',
        label: 'Dynamic pricing',
        sourceType: 'dynamic',
        amount: null,
        currency,
        unit: TOKEN_UNIT,
        condition: model.billing_expr ?? null,
      },
      basisRatio,
      discountCoefficient
    ),
  ]
}

function createConfirmationDimension(
  model: PricingModel,
  sourceType: 'dynamic' | 'task_usage',
  basisRatio: number | null,
  discountCoefficient: number | null
): QuotePriceDimension {
  return createDimension(
    {
      key: `${sourceType}-unparsed`,
      label:
        sourceType === 'task_usage' ? 'Task usage pricing' : 'Dynamic pricing',
      sourceType,
      amount: null,
      currency: modelCurrency(model),
      unit: 'variable',
      condition: model.billing_expr ?? null,
    },
    basisRatio,
    discountCoefficient
  )
}

function videoDimensions(
  model: PricingModel,
  basisRatio: number | null,
  discountCoefficient: number | null
): QuotePriceDimension[] {
  const pricing = model.video_pricing
  if (!pricing) return []
  return pricing.rows.flatMap((row, rowIndex) => {
    const resolution = row.resolutions.join(' / ')
    const metadata = `${resolution}; fps ${pricing.fps}; extra frames ${pricing.extra_frames}; Token = ceil(width x height x (fps x duration + extra frames) / 1024)`
    return [
      createDimension(
        {
          key: `video-${rowIndex}-without-input`,
          label: `${resolution} without video input`,
          sourceType: 'video',
          amount: row.without_video,
          currency: 'CNY',
          unit: pricing.unit,
          condition: metadata,
        },
        basisRatio,
        discountCoefficient
      ),
      createDimension(
        {
          key: `video-${rowIndex}-with-input`,
          label: `${resolution} with video input`,
          sourceType: 'video',
          amount: row.with_video,
          currency: 'CNY',
          unit: pricing.unit,
          condition: metadata,
        },
        basisRatio,
        discountCoefficient
      ),
    ]
  })
}

function grokDimensions(
  model: PricingModel,
  basisRatio: number | null,
  discountCoefficient: number | null
): QuotePriceDimension[] {
  const pricing = model.molii_grok_pricing
  if (!pricing) return []
  const dimensions = Object.entries(pricing.output_prices).map(
    ([tier, amount]) =>
      createDimension(
        {
          key: `grok-output-${tier}`,
          label: `${tier} output`,
          sourceType: 'grok',
          amount,
          currency: 'CNY',
          unit: pricing.output_unit,
          condition: tier,
        },
        basisRatio,
        discountCoefficient
      )
  )
  if (pricing.image_input_price !== undefined) {
    dimensions.push(
      createDimension(
        {
          key: 'grok-image-input',
          label: 'Image input',
          sourceType: 'grok',
          amount: pricing.image_input_price,
          currency: 'CNY',
          unit: pricing.image_input_unit ?? 'image',
        },
        basisRatio,
        discountCoefficient
      )
    )
  }
  if (pricing.video_input_price !== undefined) {
    dimensions.push(
      createDimension(
        {
          key: 'grok-video-input',
          label: 'Video input',
          sourceType: 'grok',
          amount: pricing.video_input_price,
          currency: 'CNY',
          unit: pricing.video_input_unit ?? 'second',
        },
        basisRatio,
        discountCoefficient
      )
    )
  }
  return dimensions
}

function modelDimensions(
  model: PricingModel,
  basisRatio: number | null,
  discountCoefficient: number | null
): QuotePriceDimension[] {
  if (model.video_pricing) {
    return videoDimensions(model, basisRatio, discountCoefficient)
  }
  if (model.molii_grok_pricing) {
    return grokDimensions(model, basisRatio, discountCoefficient)
  }
  if (isDynamicPricingModel(model)) {
    return dynamicDimensions(model, basisRatio, discountCoefficient)
  }
  if (isUnconfiguredTaskUsageModel(model)) {
    return [
      createConfirmationDimension(
        model,
        'task_usage',
        basisRatio,
        discountCoefficient
      ),
    ]
  }
  if (model.quota_type === 1) {
    return [
      createDimension(
        {
          key: 'request',
          label: 'Request',
          sourceType: 'request',
          amount: model.model_price,
          currency: modelCurrency(model),
          unit: 'request',
        },
        basisRatio,
        discountCoefficient
      ),
    ]
  }
  return fixedTokenDimensions(model, basisRatio, discountCoefficient)
}

function normalizeFetchedAt(value: string | number | Date): string {
  if (typeof value === 'string') return value
  return new Date(value).toISOString()
}

export function buildQuotationSnapshot(
  input: BuildQuotationSnapshotInput
): QuotationSnapshot {
  const selectedGroup =
    input.draft.priceBasis.type === 'group'
      ? input.draft.priceBasis.group
      : null
  const configuredRatio = selectedGroup ? input.groupRatio[selectedGroup] : 1
  const basisRatio =
    typeof configuredRatio === 'number' &&
    Number.isFinite(configuredRatio) &&
    configuredRatio >= 0
      ? configuredRatio
      : null
  const vendorById = new Map(input.vendors.map((vendor) => [vendor.id, vendor]))
  const modelById = new Map(
    input.models.map((model) => [model.model_name, model])
  )
  const providers = new Map<string, QuoteProviderSection>()

  for (const selectedModelId of input.draft.selectedModelIds) {
    const model = modelById.get(selectedModelId)
    const providerId = model?.vendor_id ?? null
    const providerKey = providerId === null ? 'unknown' : String(providerId)
    const providerOverride = input.draft.providerOverrides[providerKey]
    const discountCoefficient = resolveEffectiveDiscount(
      input.draft.globalDiscount,
      providerOverride?.discount
    )
    let provider = providers.get(providerKey)
    if (!provider) {
      provider = {
        providerId,
        providerName:
          (providerId === null ? null : vendorById.get(providerId)?.name) ??
          model?.vendor_name ??
          'Unknown provider',
        note: providerOverride?.note ?? '',
        discount: providerOverride?.discount ?? null,
        discountCoefficient,
        models: [],
      }
      providers.set(providerKey, provider)
    }

    if (!model) {
      provider.models.push({
        modelId: selectedModelId,
        displayName: selectedModelId,
        available: false,
        unavailableReason: 'missing',
        dimensions: [],
        usageExamples: [],
      })
      continue
    }

    const groupIsAvailable =
      selectedGroup === null ||
      model.enable_groups.includes('all') ||
      model.enable_groups.includes(selectedGroup)
    const available = groupIsAvailable && basisRatio !== null
    const modelSection: QuoteModelSection = {
      modelId: model.model_name,
      displayName: model.display_name || model.model_name,
      available,
      unavailableReason: available ? null : 'group_unavailable',
      dimensions: available
        ? modelDimensions(model, basisRatio, discountCoefficient)
        : [],
      usageExamples: (model.billing_usage_examples ?? []).map((example) => ({
        label: example.label,
        facts: { ...example.facts },
      })),
    }
    provider.models.push(modelSection)
  }

  return {
    title: input.draft.title,
    customer: input.draft.customer,
    quotedBy: input.draft.quotedBy,
    quoteDate: input.draft.quoteDate,
    pricingVersion: input.pricingVersion ?? null,
    fetchedAt: normalizeFetchedAt(input.fetchedAt),
    globalDiscount: input.draft.globalDiscount,
    priceBasis: {
      type: input.draft.priceBasis.type,
      group: selectedGroup,
      ratio: basisRatio,
    },
    providers: [...providers.values()],
  }
}

export function validateQuotation(
  snapshot: QuotationSnapshot
): QuotationValidation {
  const errors: QuotationValidation['errors'] = []
  if (!snapshot.title.trim()) {
    errors.push({
      code: 'missing_title',
      message: 'Quotation title is required.',
    })
  }
  if (!snapshot.quoteDate.trim()) {
    errors.push({
      code: 'missing_quote_date',
      message: 'Quote date is required.',
    })
  }
  if (snapshot.providers.every((provider) => provider.models.length === 0)) {
    errors.push({ code: 'no_models', message: 'Select at least one model.' })
  }
  if (normalizeDiscount(snapshot.globalDiscount) === null) {
    errors.push({
      code: 'invalid_discount',
      message: 'Global discount must be greater than 0 and at most 10.',
    })
  }
  if (snapshot.priceBasis.ratio === null) {
    errors.push({
      code: 'invalid_price_basis',
      message: 'The selected price basis has no valid ratio.',
    })
  }

  for (const provider of snapshot.providers) {
    if (
      provider.discount !== null &&
      normalizeDiscount(provider.discount) === null
    ) {
      errors.push({
        code: 'invalid_provider_discount',
        message: 'Provider discount must be greater than 0 and at most 10.',
        providerId: provider.providerId,
      })
    }
    for (const model of provider.models) {
      if (!model.available) {
        errors.push({
          code: 'model_unavailable',
          message: `Model ${model.modelId} is unavailable for this price basis.`,
          providerId: provider.providerId,
          modelId: model.modelId,
        })
      }
      for (const dimension of model.dimensions) {
        if (dimension.status === 'needs_confirmation') {
          errors.push({
            code: 'price_needs_confirmation',
            message: `Price ${dimension.label} for ${model.modelId} needs confirmation.`,
            providerId: provider.providerId,
            modelId: model.modelId,
            dimensionKey: dimension.key,
          })
        }
      }
    }
  }

  return { valid: errors.length === 0, errors }
}
