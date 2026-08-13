import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
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
