import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PricingModel, PricingVendor } from '@/features/pricing/types'

import { buildHomeModelCatalog, searchHomeModels } from '../home-model-catalog'

function model(
  modelName: string,
  overrides: Partial<PricingModel> = {}
): PricingModel {
  return {
    id: overrides.id ?? 1,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

const vendors: PricingVendor[] = [
  { id: 1, name: 'DeepSeek', icon: 'DeepSeek', description: 'LLM vendor' },
  { id: 2, name: 'xAI', icon: 'Grok', description: 'Imagine models' },
  { id: 3, name: 'Unused', icon: 'OpenAI' },
]

describe('home model catalog', () => {
  test('derives only enabled vendors and real catalog counts', () => {
    const catalog = buildHomeModelCatalog(
      [
        model('deepseek-v4', {
          id: 11,
          vendor_id: 1,
          vendor_name: 'DeepSeek',
          output_modalities: ['text'],
        }),
        model('grok-imagine-image', {
          id: 12,
          vendor_id: 2,
          vendor_name: 'xAI',
          output_modalities: ['image'],
        }),
        model('grok-imagine-video', {
          id: 13,
          vendor_id: 2,
          vendor_name: 'xAI',
          output_modalities: ['video'],
        }),
      ],
      vendors
    )

    assert.equal(catalog.modelCount, 3)
    assert.equal(catalog.vendorCount, 2)
    assert.equal(catalog.capabilityCategoryCount, 3)
    assert.deepEqual(
      catalog.vendors.map((vendor) => vendor.name),
      ['DeepSeek', 'xAI']
    )
  })

  test('uses model metadata when vendor rows are absent and deduplicates names', () => {
    const catalog = buildHomeModelCatalog(
      [
        model('qwen-a', {
          vendor_name: 'Qwen',
          vendor_icon: 'Qwen.Color',
          vendor_description: 'Alibaba model family',
        }),
        model('qwen-b', {
          vendor_name: ' qwen ',
          vendor_icon: 'Qwen',
        }),
      ],
      []
    )

    assert.equal(catalog.vendorCount, 1)
    assert.deepEqual(catalog.vendors[0], {
      id: null,
      name: 'Qwen',
      icon: 'Qwen.Color',
      description: 'Alibaba model family',
      modelCount: 2,
    })
  })

  test('returns the newest three valid dated models and excludes undated rows', () => {
    const catalog = buildHomeModelCatalog(
      [
        model('undated'),
        model('invalid', { release_date: 'not-a-date' }),
        model('oldest', { release_date: '2026-05-01' }),
        model('second', { release_date: '2026-08-12' }),
        model('newest-b', { release_date: '2026-08-13' }),
        model('newest-a', { release_date: '2026-08-13' }),
      ],
      []
    )

    assert.deepEqual(
      catalog.latestModels.map((item) => item.model_name),
      ['newest-a', 'newest-b', 'second']
    )
  })
})

describe('home model search', () => {
  const models = [
    model('deepseek-v4-pro', {
      vendor_name: 'DeepSeek',
      description: 'Long-context reasoning and coding model',
      capabilities: ['reasoning', 'tools'],
      input_modalities: ['text'],
      output_modalities: ['text'],
    }),
    model('doubao-seedance-2-0', {
      vendor_name: 'ByteDance',
      description: '视频生成模型',
      capabilities: ['video_generation'],
      input_modalities: ['text', 'image', 'video'],
      output_modalities: ['video'],
    }),
    model('grok-imagine-image', {
      vendor_name: 'xAI',
      capabilities: ['image_generation', 'image_editing'],
      output_modalities: ['image'],
    }),
  ]

  test('matches model id, vendor, description, capability, and modalities', () => {
    assert.deepEqual(
      searchHomeModels(models, 'DEEPSEEK').map((item) => item.model_name),
      ['deepseek-v4-pro']
    )
    assert.deepEqual(
      searchHomeModels(models, '视频生成').map((item) => item.model_name),
      ['doubao-seedance-2-0']
    )
    assert.deepEqual(
      searchHomeModels(models, 'image_generation').map(
        (item) => item.model_name
      ),
      ['grok-imagine-image']
    )
  })

  test('normalizes whitespace and respects the result limit', () => {
    const repeated = Array.from({ length: 8 }, (_, index) =>
      model(`qwen-model-${index}`, { vendor_name: 'Qwen' })
    )

    assert.equal(searchHomeModels(repeated, '  qWen  ', 5).length, 5)
    assert.deepEqual(searchHomeModels(models, '   '), [])
  })
})
