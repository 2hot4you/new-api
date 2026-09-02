import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import {
  MODEL_CAPABILITY_OPTIONS,
  MODEL_MODALITY_OPTIONS,
  MODEL_OUTPUT_FORMAT_OPTIONS,
  MODEL_PARAMETER_OPTIONS,
  modelFormSchema,
  transformFormDataToModelPayload,
  transformModelToFormDefaults,
} from '../model-form'

describe('model catalog metadata form mapping', () => {
  test('round-trips every local marketplace field without editable provenance', () => {
    const defaults = transformModelToFormDefaults({
      id: 7,
      model_name: 'catalog-model',
      display_name: 'Catalog Model',
      description: '中文简介',
      description_en: 'English description',
      icon: 'OpenAI',
      tags: 'chat,long-context',
      vendor_id: 3,
      status: 1,
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
      marketplace_enabled: true,
      supported_parameters: ['stream', 'tools'],
      supported_resolutions: ['720p'],
      supported_aspect_ratios: ['16:9'],
      max_input_images: 0,
      output_formats: ['url'],
      min_duration: 1,
      max_duration: 15,
      reference_modalities: ['image'],
      metadata_source: 'models.dev',
      metadata_verified_at: '2026-08-13',
    })

    const payload = transformFormDataToModelPayload(defaults)

    assert.equal(payload.display_name, 'Catalog Model')
    assert.equal(payload.description, '中文简介')
    assert.equal(payload.description_en, 'English description')
    assert.equal(payload.marketplace_enabled, true)
    assert.equal(payload.context_length, 1_000_000)
    assert.equal(payload.max_output_tokens, 65_536)
    assert.equal(payload.knowledge_cutoff, '2025-04')
    assert.equal(payload.release_date, '2026-02-16')
    assert.deepEqual(payload.input_modalities, ['text', 'image'])
    assert.deepEqual(payload.output_modalities, ['text'])
    assert.deepEqual(payload.capabilities, ['streaming', 'tools'])
    assert.deepEqual(payload.supported_parameters, ['stream', 'tools'])
    assert.deepEqual(payload.supported_resolutions, ['720p'])
    assert.deepEqual(payload.supported_aspect_ratios, ['16:9'])
    assert.equal(payload.max_input_images, 0)
    assert.deepEqual(payload.output_formats, ['url'])
    assert.equal(payload.min_duration, 1)
    assert.equal(payload.max_duration, 15)
    assert.deepEqual(payload.reference_modalities, ['image'])
    assert.equal(payload.metadata_source, undefined)
    assert.equal(payload.metadata_verified_at, undefined)
  })

  test('preserves numeric zero values with nullish defaults', () => {
    const defaults = transformModelToFormDefaults({
      id: 8,
      model_name: 'zero-model',
      status: 0,
      created_time: 1,
      updated_time: 2,
      name_rule: 0,
      context_length: 0,
      max_output_tokens: 0,
      max_input_images: 0,
      min_duration: 0,
      max_duration: 0,
    })

    assert.equal(defaults.context_length, 0)
    assert.equal(defaults.max_output_tokens, 0)
    assert.equal(defaults.max_input_images, 0)
    assert.equal(defaults.min_duration, 0)
    assert.equal(defaults.max_duration, 0)
  })
})

describe('model catalog metadata validation', () => {
  const validForm = {
    model_name: 'catalog-model',
    display_name: '',
    description: '',
    description_en: '',
    icon: '',
    tags: [],
    endpoints: '',
    name_rule: 0,
    status: true,
    context_length: 0,
    max_output_tokens: 0,
    knowledge_cutoff: '',
    release_date: '',
    input_modalities: [],
    output_modalities: [],
    capabilities: [],
    marketplace_enabled: false,
    supported_parameters: [],
    supported_resolutions: [],
    supported_aspect_ratios: [],
    max_input_images: 0,
    output_formats: [],
    min_duration: 0,
    max_duration: 0,
    reference_modalities: [],
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
    assert.ok(MODEL_PARAMETER_OPTIONS.includes('reasoning_effort'))
    assert.deepEqual(MODEL_OUTPUT_FORMAT_OPTIONS, ['url', 'b64_json'])

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

  test('accepts an incomplete local draft', () => {
    const draft = modelFormSchema.safeParse(validForm)
    assert.equal(draft.success, true)
  })

  test('rejects an incomplete publication request with field-level issues', () => {
    const publish = modelFormSchema.safeParse({
      ...validForm,
      marketplace_enabled: true,
    })

    assert.equal(publish.success, false)
    if (publish.success) return
    const issuePaths = new Set(
      publish.error.issues.map((issue) => issue.path.join('.'))
    )
    assert.ok(issuePaths.has('display_name'))
    assert.ok(issuePaths.has('description'))
    assert.ok(issuePaths.has('vendor_id'))
    assert.ok(issuePaths.has('supported_parameters'))
    assert.ok(issuePaths.has('context_length'))
    assert.ok(issuePaths.has('max_output_tokens'))
  })
})
