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

import { describe, test } from 'vitest'

import type { PricingModel } from '@/features/pricing/types'

import {
  buildQuotationSnapshot,
  calculateSuggestedGroupRatio,
  normalizeDiscount,
  resolveEffectiveDiscount,
  validateQuotation,
} from '../quotation-math'

function pricingModel(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    vendor_id: 10,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    enable_groups: ['default', 'vip'],
    ...overrides,
  }
}

function buildSnapshot(
  models: PricingModel[],
  overrides: Partial<Parameters<typeof buildQuotationSnapshot>[0]> = {}
) {
  return buildQuotationSnapshot({
    draft: {
      title: 'Acme quotation',
      customer: 'Acme',
      quotedBy: 'Molii',
      quoteDate: '2026-09-08',
      globalDiscount: 8,
      priceBasis: { type: 'raw' },
      selectedModelIds: models.map((model) => model.model_name),
      providerOverrides: {},
    },
    models,
    vendors: [{ id: 10, name: 'Provider A' }],
    groupRatio: { default: 1, vip: 1.5 },
    pricingVersion: 'pricing-v3',
    fetchedAt: '2026-09-08T10:00:00.000Z',
    ...overrides,
  })
}

describe('quotation discounts and group ratio calculator', () => {
  test('normalizes valid Chinese discounts exactly once', () => {
    assert.equal(normalizeDiscount(8), 0.8)
    assert.equal(normalizeDiscount(5), 0.5)
    assert.equal(normalizeDiscount(10), 1)
  })

  test('rejects non-finite and out-of-range discounts', () => {
    assert.equal(normalizeDiscount(Number.NaN), null)
    assert.equal(normalizeDiscount(Number.POSITIVE_INFINITY), null)
    assert.equal(normalizeDiscount(0), null)
    assert.equal(normalizeDiscount(-1), null)
    assert.equal(normalizeDiscount(10.01), null)
  })

  test('prefers a valid provider override and otherwise uses the global discount', () => {
    assert.equal(resolveEffectiveDiscount(8, 5), 0.5)
    assert.equal(resolveEffectiveDiscount(8, 0), 0.8)
    assert.equal(resolveEffectiveDiscount(8, null), 0.8)
    assert.equal(resolveEffectiveDiscount(0, null), null)
  })

  test('calculates 6.8 x 0.5 / 7 without rounding the canonical value', () => {
    assert.equal(calculateSuggestedGroupRatio(6.8, 5, 7), 0.4857142857142857)
  })

  test('rejects non-finite, zero, and negative calculator rates', () => {
    assert.equal(calculateSuggestedGroupRatio(Number.NaN, 5, 7), null)
    assert.equal(calculateSuggestedGroupRatio(6.8, Number.NaN, 7), null)
    assert.equal(calculateSuggestedGroupRatio(6.8, 5, 0), null)
    assert.equal(calculateSuggestedGroupRatio(-6.8, 5, 7), null)
  })
})

describe('canonical quotation snapshot', () => {
  test('expands every configured fixed-token dimension and applies discount once', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'fixed-token',
        model_ratio: 3,
        completion_ratio: 2,
        cache_ratio: 0.25,
        create_cache_ratio: 0.5,
        image_ratio: 1.5,
        audio_ratio: 2,
        audio_completion_ratio: 3,
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.deepEqual(
      dimensions.map((dimension) => [
        dimension.key,
        dimension.sourceAmount,
        dimension.quoteAmount,
        dimension.currency,
        dimension.unit,
      ]),
      [
        ['input', 6, 4.800000000000001, 'USD', '1M token'],
        ['output', 12, 9.600000000000001, 'USD', '1M token'],
        ['cache_read', 1.5, 1.2000000000000002, 'USD', '1M token'],
        ['cache_write', 3, 2.4000000000000004, 'USD', '1M token'],
        ['image_input', 9, 7.2, 'USD', '1M token'],
        ['audio_input', 12, 9.600000000000001, 'USD', '1M token'],
        ['audio_output', 36, 28.8, 'USD', '1M token'],
      ]
    )
  })

  test('preserves explicit zero request pricing but marks missing request pricing for confirmation', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'free-request',
        quota_type: 1,
        model_price: 0,
      }),
      pricingModel({
        id: 2,
        model_name: 'unknown-request',
        quota_type: 1,
        model_price: undefined,
      }),
    ])

    const [freeModel, unknownModel] = snapshot.providers[0]?.models ?? []
    assert.deepEqual(freeModel?.dimensions[0], {
      key: 'request',
      label: 'Request',
      sourceType: 'request',
      catalogAmount: 0,
      sourceAmount: 0,
      quoteAmount: 0,
      currency: 'USD',
      unit: 'request',
      condition: null,
      status: 'ready',
    })
    assert.equal(unknownModel?.dimensions[0]?.sourceAmount, null)
    assert.equal(unknownModel?.dimensions[0]?.quoteAmount, null)
    assert.equal(unknownModel?.dimensions[0]?.status, 'needs_confirmation')
  })

  test('uses the explicit group basis and provider discount without double applying either', () => {
    const model = pricingModel({ model_name: 'vip-model', model_ratio: 2 })
    const snapshot = buildSnapshot([model], {
      draft: {
        title: 'VIP quote',
        customer: '',
        quotedBy: '',
        quoteDate: '2026-09-08',
        globalDiscount: 8,
        priceBasis: { type: 'group', group: 'vip' },
        selectedModelIds: ['vip-model'],
        providerOverrides: {
          '10': { discount: 5, note: 'Partner price' },
        },
      },
    })

    const input = snapshot.providers[0]?.models[0]?.dimensions[0]
    assert.equal(snapshot.priceBasis.ratio, 1.5)
    assert.equal(snapshot.providers[0]?.discountCoefficient, 0.5)
    assert.equal(input?.catalogAmount, 4)
    assert.equal(input?.sourceAmount, 6)
    assert.equal(input?.quoteAmount, 3)
  })

  test('preserves tiny non-zero and direct CNY dynamic prices across every tier', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'dynamic-cny',
        billing_mode: 'tiered_expr',
        billing_currency: 'CNY',
        billing_expr:
          'len < 1000 ? tier("small", p * 0.00000002 + c * 0.4) : tier("large", p * 0.00000001 + c * 0.3)',
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.equal(dimensions.length, 4)
    assert.deepEqual(
      dimensions.map((dimension) => [
        dimension.key,
        dimension.catalogAmount,
        dimension.currency,
        dimension.condition,
      ]),
      [
        ['tier-0-inputPrice', 0.00000002, 'CNY', 'small; len < 1000'],
        ['tier-0-outputPrice', 0.4, 'CNY', 'small; len < 1000'],
        ['tier-1-inputPrice', 0.00000001, 'CNY', 'large'],
        ['tier-1-outputPrice', 0.3, 'CNY', 'large'],
      ]
    )
  })

  test('preserves task base charges, units, enum conditions, and every tier', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'task-model',
        billing_mode: 'tiered_expr',
        billing_expr:
          'u("mode") == "pro" ? tier("pro", 0.2 + u("seconds") * 0.8 + u("clips") * 0.1 + u("tokens") * 9 / 1000000 + u("credits") * 0.05) : tier("base", 0.1 + u("seconds") * 0.4 + u("clips") * 0.05 + u("tokens") * 4 / 1000000 + u("credits") * 0.02)',
        billing_usage_schema: {
          mode: { enum: ['base', 'pro'] },
          seconds: { type: 'number', unit: 'second' },
          clips: { type: 'number', unit: 'count' },
          tokens: { type: 'number', unit: 'token' },
          credits: { type: 'number', unit: 'credit' },
        },
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.equal(dimensions.length, 10)
    assert.deepEqual(
      dimensions.map((dimension) => [
        dimension.key,
        dimension.catalogAmount,
        dimension.unit,
        dimension.condition,
      ]),
      [
        ['task-tier-0-base', 0.2, 'request', 'pro; mode = pro'],
        ['task-tier-0-clips', 0.1, 'count', 'pro; mode = pro'],
        ['task-tier-0-credits', 0.05, 'credit', 'pro; mode = pro'],
        ['task-tier-0-seconds', 0.8, 'second', 'pro; mode = pro'],
        ['task-tier-0-tokens', 9, '1M token', 'pro; mode = pro'],
        ['task-tier-1-base', 0.1, 'request', 'base'],
        ['task-tier-1-clips', 0.05, 'count', 'base'],
        ['task-tier-1-credits', 0.02, 'credit', 'base'],
        ['task-tier-1-seconds', 0.4, 'second', 'base'],
        ['task-tier-1-tokens', 4, '1M token', 'base'],
      ]
    )
  })

  test('preserves every task usage example and its facts in the snapshot', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'task-examples',
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("base", u("seconds") * 0.4)',
        billing_usage_schema: {
          seconds: { type: 'number', unit: 'second' },
          mode: { enum: ['standard', 'pro'] },
        },
        billing_usage_examples: [
          {
            label: 'Short standard job',
            facts: { seconds: 10, mode: 'standard' },
          },
          { label: 'Long pro job', facts: { seconds: 120, mode: 'pro' } },
        ],
      }),
    ])

    assert.deepEqual(snapshot.providers[0]?.models[0]?.usageExamples, [
      {
        label: 'Short standard job',
        facts: { seconds: 10, mode: 'standard' },
      },
      { label: 'Long pro job', facts: { seconds: 120, mode: 'pro' } },
    ])
  })

  test('marks an unconfigured task-usage model for confirmation instead of inventing token prices', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'unconfigured-task',
        billing_usage_schema: {
          seconds: { type: 'number', unit: 'second' },
        },
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.equal(dimensions.length, 1)
    assert.equal(dimensions[0]?.sourceType, 'task_usage')
    assert.equal(dimensions[0]?.sourceAmount, null)
    assert.equal(dimensions[0]?.unit, 'variable')
    assert.equal(dimensions[0]?.status, 'needs_confirmation')
    assert.equal(validateQuotation(snapshot).valid, false)
  })

  test('expands every Seedance and Grok catalog row in direct CNY', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'seedance',
        video_pricing: {
          unit: 'cny_per_million_tokens',
          fps: 24,
          extra_frames: 1,
          rows: [
            { resolutions: ['480p', '720p'], without_video: 2, with_video: 3 },
            { resolutions: ['1080p'], without_video: 4, with_video: 5 },
          ],
        },
      }),
      pricingModel({
        id: 2,
        model_name: 'grok-video',
        molii_grok_pricing: {
          kind: 'video',
          output_unit: 'second',
          output_prices: { 'standard/480p': 0.2, 'standard/720p': 0.3 },
          image_input_unit: 'image',
          image_input_price: 0.1,
          video_input_unit: 'second',
          video_input_price: 0.15,
        },
      }),
      pricingModel({
        id: 3,
        model_name: 'seedance-usd',
        video_pricing: {
          unit: 'usd_per_million_tokens',
          fps: 24,
          extra_frames: 1,
          rows: [{ resolutions: ['720p'], without_video: 6, with_video: 4 }],
        },
      }),
    ])

    const [video, grok, usdVideo] = snapshot.providers[0]?.models ?? []
    assert.equal(video?.dimensions.length, 4)
    assert.ok(
      video?.dimensions.every(
        (dimension) =>
          dimension.currency === 'CNY' &&
          dimension.unit === '1M token' &&
          dimension.condition?.includes('fps 24') &&
          dimension.condition?.includes('extra frames 1')
      )
    )
    assert.deepEqual(
      grok?.dimensions.map((dimension) => [
        dimension.key,
        dimension.catalogAmount,
        dimension.unit,
      ]),
      [
        ['grok-output-standard/480p', 0.2, 'second'],
        ['grok-output-standard/720p', 0.3, 'second'],
        ['grok-image-input', 0.1, 'image'],
        ['grok-video-input', 0.15, 'second'],
      ]
    )
    assert.ok(
      usdVideo?.dimensions.every(
        (dimension) =>
          dimension.currency === 'USD' && dimension.unit === '1M token'
      )
    )
  })

  test('keeps an unparseable dynamic expression as a confirmation row instead of zero', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'custom-dynamic',
        billing_mode: 'tiered_expr',
        billing_expr: 'unsupported(price) + custom()',
      }),
    ])

    const dimension = snapshot.providers[0]?.models[0]?.dimensions[0]
    assert.equal(dimension?.sourceAmount, null)
    assert.equal(dimension?.quoteAmount, null)
    assert.equal(dimension?.condition, 'unsupported(price) + custom()')
    assert.equal(dimension?.status, 'needs_confirmation')
    assert.equal(validateQuotation(snapshot).valid, false)
  })

  test('rejects a partially parsed dynamic tier instead of dropping an unsupported price term', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'partially-understood-dynamic',
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("base", p * 2 + custom())',
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.equal(dimensions.length, 1)
    assert.equal(dimensions[0]?.catalogAmount, null)
    assert.equal(dimensions[0]?.sourceAmount, null)
    assert.equal(dimensions[0]?.condition, 'tier("base", p * 2 + custom())')
    assert.equal(dimensions[0]?.status, 'needs_confirmation')
  })

  test('rejects duplicate recognized dynamic variables instead of quoting only the first coefficient', () => {
    const expression = 'tier("base", p * 2 + p * 3 + c * 0 + c * 4)'
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'duplicate-dynamic-variables',
        billing_mode: 'tiered_expr',
        billing_expr: expression,
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.equal(dimensions.length, 1)
    assert.equal(dimensions[0]?.catalogAmount, null)
    assert.equal(dimensions[0]?.sourceAmount, null)
    assert.equal(dimensions[0]?.quoteAmount, null)
    assert.equal(dimensions[0]?.condition, expression)
    assert.equal(dimensions[0]?.status, 'needs_confirmation')
    assert.equal(validateQuotation(snapshot).valid, false)
  })

  test('preserves an explicit zero dynamic dimension alongside a positive term', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'free-input-dynamic',
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("base", p * 0 + c * 4)',
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.deepEqual(
      dimensions.map((dimension) => [
        dimension.key,
        dimension.catalogAmount,
        dimension.sourceAmount,
        dimension.quoteAmount,
        dimension.status,
      ]),
      [
        ['tier-0-inputPrice', 0, 0, 0, 'ready'],
        ['tier-0-outputPrice', 4, 4, 3.2, 'ready'],
      ]
    )
    assert.equal(validateQuotation(snapshot).valid, true)
  })

  test('marks request-rule multipliers for confirmation rather than quoting only the default amount', () => {
    const expression =
      '(tier("base", p * 2)) * (header("x-priority") == "high" ? 2 : 1)'
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'request-rule-dynamic',
        billing_mode: 'tiered_expr',
        billing_expr: expression,
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.equal(dimensions.length, 1)
    assert.equal(dimensions[0]?.catalogAmount, null)
    assert.equal(dimensions[0]?.quoteAmount, null)
    assert.equal(dimensions[0]?.condition, expression)
    assert.equal(dimensions[0]?.status, 'needs_confirmation')
  })

  test('marks time-rule multipliers for confirmation rather than omitting the conditional amount', () => {
    const expression =
      '(tier("base", p * 3)) * (hour("UTC") >= 9 && hour("UTC") < 18 ? 1.5 : 1)'
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'time-rule-dynamic',
        billing_mode: 'tiered_expr',
        billing_expr: expression,
      }),
    ])

    const dimension = snapshot.providers[0]?.models[0]?.dimensions[0]
    assert.equal(dimension?.catalogAmount, null)
    assert.equal(dimension?.sourceAmount, null)
    assert.equal(dimension?.quoteAmount, null)
    assert.equal(dimension?.condition, expression)
    assert.equal(dimension?.status, 'needs_confirmation')
  })

  test('preserves a sole explicit zero term when the dynamic expression is lossless', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'ambiguous-zero-dynamic',
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("base", p * 0)',
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.equal(dimensions.length, 1)
    assert.equal(dimensions[0]?.catalogAmount, 0)
    assert.equal(dimensions[0]?.sourceAmount, 0)
    assert.equal(dimensions[0]?.quoteAmount, 0)
    assert.equal(dimensions[0]?.status, 'ready')
  })

  test('retains a selected model that is unavailable for the selected group and blocks export', () => {
    const snapshot = buildSnapshot(
      [
        pricingModel({
          model_name: 'default-only',
          enable_groups: ['default'],
        }),
      ],
      {
        draft: {
          title: 'VIP quote',
          customer: '',
          quotedBy: '',
          quoteDate: '2026-09-08',
          globalDiscount: 8,
          priceBasis: { type: 'group', group: 'vip' },
          selectedModelIds: ['default-only'],
          providerOverrides: {},
        },
      }
    )

    const model = snapshot.providers[0]?.models[0]
    assert.equal(model?.available, false)
    assert.equal(model?.dimensions.length, 0)
    const validation = validateQuotation(snapshot)
    assert.equal(validation.valid, false)
    assert.ok(
      validation.errors.some((error) => error.code === 'model_unavailable')
    )
  })

  test('treats the backend all group wildcard as available for an explicit group', () => {
    const snapshot = buildSnapshot(
      [pricingModel({ model_name: 'all-groups', enable_groups: ['all'] })],
      {
        draft: {
          title: 'VIP quote',
          customer: '',
          quotedBy: '',
          quoteDate: '2026-09-08',
          globalDiscount: 10,
          priceBasis: { type: 'group', group: 'vip' },
          selectedModelIds: ['all-groups'],
          providerOverrides: {},
        },
      }
    )

    const model = snapshot.providers[0]?.models[0]
    assert.equal(model?.available, true)
    assert.equal(model?.dimensions[0]?.sourceAmount, 3)
    assert.equal(validateQuotation(snapshot).valid, true)
  })

  test('omits audio output when only the audio input ratio is configured', () => {
    const snapshot = buildSnapshot([
      pricingModel({
        model_name: 'audio-input-only',
        audio_ratio: 2,
        audio_completion_ratio: undefined,
      }),
    ])

    const dimensions = snapshot.providers[0]?.models[0]?.dimensions ?? []
    assert.ok(dimensions.some((dimension) => dimension.key === 'audio_input'))
    assert.ok(!dimensions.some((dimension) => dimension.key === 'audio_output'))
    assert.equal(validateQuotation(snapshot).valid, true)
  })

  test('uses direct CNY consistently for fixed-token and request models', () => {
    const snapshot = buildSnapshot([
      pricingModel({ model_name: 'fixed-cny', billing_currency: 'CNY' }),
      pricingModel({
        id: 2,
        model_name: 'request-cny',
        quota_type: 1,
        model_price: 2.5,
        billing_currency: 'CNY',
      }),
    ])

    const [fixed, request] = snapshot.providers[0]?.models ?? []
    assert.ok(
      fixed?.dimensions.every((dimension) => dimension.currency === 'CNY')
    )
    assert.equal(request?.dimensions[0]?.currency, 'CNY')
  })
})
