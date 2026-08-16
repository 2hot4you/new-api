import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { SORT_OPTIONS } from '../../constants'
import type { PricingModel } from '../../types'
import { filterByQuotaType, sortModels } from '../filters'
import {
  filterModelsByDirectory,
  getContextBucketId,
  getContextBuckets,
  getModelCategories,
  getModelCategory,
  getModelInputModalities,
  sortModelsByReleaseDate,
} from '../model-directory'

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

describe('model directory release ordering', () => {
  test('sorts dated models newest first and keeps undated models last', () => {
    const sorted = sortModelsByReleaseDate([
      model('undated-z'),
      model('older', { release_date: '2026-07-01' }),
      model('newer', { release_date: '2026-08-13' }),
      model('invalid-a', { release_date: 'not-a-date' }),
    ])

    assert.deepEqual(
      sorted.map((item) => item.model_name),
      ['newer', 'older', 'invalid-a', 'undated-z']
    )
  })

  test('uses model id as a stable tie breaker for equal release dates', () => {
    const sorted = sortModelsByReleaseDate([
      model('z-model', { release_date: '2026-08-13' }),
      model('a-model', { release_date: '2026-08-13' }),
    ])

    assert.deepEqual(
      sorted.map((item) => item.model_name),
      ['a-model', 'z-model']
    )
  })

  test('exposes release ordering through the marketplace sort option', () => {
    const sorted = sortModels(
      [
        model('undated'),
        model('newer', { release_date: '2026-08-13' }),
        model('older', { release_date: '2026-07-01' }),
      ],
      SORT_OPTIONS.RELEASE_DATE
    )

    assert.deepEqual(
      sorted.map((item) => item.model_name),
      ['newer', 'older', 'undated']
    )
  })
})

describe('model directory billing filters', () => {
  test('keeps fixed token, request, and dynamic pricing mutually exclusive', () => {
    const models = [
      model('token'),
      model('request', { quota_type: 1, model_price: 1 }),
      model('dynamic', {
        billing_mode: 'tiered_expr',
        billing_expr: 'input * 2',
      }),
    ]

    assert.deepEqual(
      filterByQuotaType(models, 'token').map((item) => item.model_name),
      ['token']
    )
    assert.deepEqual(
      filterByQuotaType(models, 'request').map((item) => item.model_name),
      ['request']
    )
    assert.deepEqual(
      filterByQuotaType(models, 'dynamic').map((item) => item.model_name),
      ['dynamic']
    )
  })
})

describe('model directory categories and modalities', () => {
  test('prefers output metadata and falls back to capabilities and endpoints', () => {
    assert.equal(
      getModelCategory(
        model('video', {
          output_modalities: ['video'],
          supported_endpoint_types: ['openai'],
        })
      ),
      'video'
    )
    assert.equal(
      getModelCategory(
        model('image', {
          capabilities: ['image_generation'],
          supported_endpoint_types: ['openai'],
        })
      ),
      'image'
    )
    assert.equal(
      getModelCategory(
        model('embedding', { supported_endpoint_types: ['embeddings'] })
      ),
      'embedding'
    )
    assert.equal(
      getModelCategory(model('chat', { supported_endpoint_types: ['openai'] })),
      'text'
    )
  })

  test('returns only non-empty categories in product order', () => {
    const categories = getModelCategories([
      model('chat', { supported_endpoint_types: ['openai'] }),
      model('image', { supported_endpoint_types: ['image-generation'] }),
      model('video', { supported_endpoint_types: ['openai-video'] }),
    ])

    assert.deepEqual(
      categories.map((item) => [item.id, item.count]),
      [
        ['all', 3],
        ['text', 1],
        ['image', 1],
        ['video', 1],
      ]
    )
  })

  test('uses only persisted input metadata', () => {
    assert.deepEqual(
      getModelInputModalities(
        model('vision-chat', {
          input_modalities: ['text', 'image', 'file'],
          supported_endpoint_types: ['openai'],
        })
      ),
      ['text', 'image', 'file']
    )
    assert.deepEqual(
      getModelInputModalities(
        model('plain-chat', { supported_endpoint_types: ['openai'] })
      ),
      []
    )
  })
})

describe('model directory context filters', () => {
  test('assigns model context to stable display buckets', () => {
    assert.equal(getContextBucketId(32_000), 'lte-128k')
    assert.equal(getContextBucketId(128_000), 'lte-128k')
    assert.equal(getContextBucketId(256_000), 'lte-256k')
    assert.equal(getContextBucketId(1_000_000), 'lte-1m')
    assert.equal(getContextBucketId(2_000_000), 'gt-1m')
    assert.equal(getContextBucketId(undefined), null)
  })

  test('omits empty context buckets', () => {
    assert.deepEqual(
      getContextBuckets([
        model('small', { context_length: 64_000 }),
        model('large', { context_length: 1_000_000 }),
        model('unknown'),
      ]).map((bucket) => [bucket.id, bucket.count]),
      [
        ['lte-128k', 1],
        ['lte-1m', 1],
      ]
    )
  })
})

describe('model directory combined filters', () => {
  test('combines vendor, category, input, context, capability, endpoint, billing, group, and tag filters', () => {
    const matching = model('matching', {
      vendor_name: 'DeepSeek',
      input_modalities: ['text'],
      output_modalities: ['text'],
      context_length: 1_000_000,
      capabilities: ['reasoning', 'tools'],
      supported_endpoint_types: ['openai'],
      billing_mode: 'tiered_expr',
      tags: 'agent context',
      enable_groups: ['vip'],
    })
    const wrongVendor = model('wrong-vendor', {
      ...matching,
      model_name: 'wrong-vendor',
      vendor_name: 'Qwen',
    })

    const filtered = filterModelsByDirectory([matching, wrongVendor], {
      vendor: 'DeepSeek',
      category: 'text',
      inputModality: 'text',
      contextBucket: 'lte-1m',
      capability: 'reasoning',
      endpointType: 'openai',
      billingType: 'dynamic',
      group: 'vip',
      tag: 'agent',
    })

    assert.deepEqual(
      filtered.map((item) => item.model_name),
      ['matching']
    )
  })
})
