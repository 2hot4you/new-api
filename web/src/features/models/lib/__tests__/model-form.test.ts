import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  MODEL_CAPABILITY_OPTIONS,
  MODEL_MODALITY_OPTIONS,
  modelFormSchema,
  transformFormDataToModelPayload,
  transformModelToFormDefaults,
} from '../model-form'

describe('model catalog metadata form mapping', () => {
  test('round-trips every persisted catalog field', () => {
    const defaults = transformModelToFormDefaults({
      id: 7,
      model_name: 'catalog-model',
      status: 1,
      sync_official: 1,
      created_time: 1,
      updated_time: 2,
      name_rule: 0,
      context_length: 1_000_000,
      max_output_tokens: 65_536,
      knowledge_cutoff: '2025-04',
      release_date: '2026-02-16',
      input_modalities: ['text', 'image'],
      output_modalities: ['text'],
      capabilities: ['streaming', 'tools'],
      metadata_source: 'models.dev',
      metadata_verified_at: '2026-08-13',
    })

    const payload = transformFormDataToModelPayload(defaults)

    assert.equal(payload.context_length, 1_000_000)
    assert.equal(payload.max_output_tokens, 65_536)
    assert.equal(payload.knowledge_cutoff, '2025-04')
    assert.equal(payload.release_date, '2026-02-16')
    assert.deepEqual(payload.input_modalities, ['text', 'image'])
    assert.deepEqual(payload.output_modalities, ['text'])
    assert.deepEqual(payload.capabilities, ['streaming', 'tools'])
    assert.equal(payload.metadata_source, 'models.dev')
    assert.equal(payload.metadata_verified_at, '2026-08-13')
  })
})

describe('model catalog metadata validation', () => {
  const validForm = {
    model_name: 'catalog-model',
    description: '',
    icon: '',
    tags: [],
    endpoints: '',
    name_rule: 0,
    status: true,
    sync_official: true,
    context_length: 0,
    max_output_tokens: 0,
    knowledge_cutoff: '',
    release_date: '',
    input_modalities: [],
    output_modalities: [],
    capabilities: [],
    metadata_source: '',
    metadata_verified_at: '',
    enable_groups: [],
    quota_types: [],
  }

  test('accepts only supported modality and capability values', () => {
    assert.deepEqual(MODEL_MODALITY_OPTIONS, [
      'text',
      'image',
      'audio',
      'video',
      'file',
    ])
    assert.ok(MODEL_CAPABILITY_OPTIONS.includes('streaming'))
    assert.ok(MODEL_CAPABILITY_OPTIONS.includes('video_generation'))

    const invalid = modelFormSchema.safeParse({
      ...validForm,
      model_name: 'invalid-options',
      input_modalities: ['imaginary'],
      capabilities: ['unknown-capability'],
    })
    assert.equal(invalid.success, false)
  })

  test('rejects negative token limits', () => {
    const invalid = modelFormSchema.safeParse({
      ...validForm,
      model_name: 'invalid-limits',
      context_length: -1,
      max_output_tokens: -1,
    })
    assert.equal(invalid.success, false)
  })
})
