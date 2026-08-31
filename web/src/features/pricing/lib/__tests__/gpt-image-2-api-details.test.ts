import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PricingModel } from '../../types'
import { buildSupportedParameters } from '../mock-stats'

const gptImage2: PricingModel = {
  id: 1,
  model_name: 'gpt-image-2',
  quota_type: 1,
  model_ratio: 0,
  completion_ratio: 0,
  enable_groups: ['default'],
  supported_endpoint_types: ['image-generation'],
  input_modalities: ['text', 'image'],
  output_modalities: ['image'],
  capabilities: ['image_generation', 'image_editing', 'streaming'],
}

describe('gpt-image-2 API details', () => {
  test('shows the complete generation and editing parameter contract', () => {
    const parameters = buildSupportedParameters(gptImage2)
    const byName = new Map(
      parameters.map((parameter) => [parameter.name, parameter])
    )

    assert.deepEqual(
      parameters.map((parameter) => parameter.name),
      [
        'model',
        'prompt',
        'n',
        'size',
        'quality',
        'background',
        'output_format',
        'output_compression',
        'moderation',
        'user',
        'stream',
        'partial_images',
        'images / image[]',
        'mask',
      ]
    )

    assert.equal(byName.get('model')?.required, true)
    assert.deepEqual(byName.get('model')?.enumValues, ['gpt-image-2'])
    assert.equal(byName.get('prompt')?.required, true)
    assert.equal(byName.get('prompt')?.range, '1 ~ 32,000 characters')
    assert.equal(byName.get('n')?.defaultValue, 1)
    assert.equal(byName.get('n')?.range, '1 ~ 10')
    assert.equal(byName.get('size')?.defaultValue, 'auto')
    assert.equal(byName.get('size')?.range, 'auto or WIDTHxHEIGHT')
    assert.deepEqual(byName.get('quality')?.enumValues, [
      'auto',
      'low',
      'medium',
      'high',
    ])
    assert.deepEqual(byName.get('background')?.enumValues, [
      'auto',
      'opaque',
      'transparent',
    ])
    assert.deepEqual(byName.get('output_format')?.enumValues, [
      'png',
      'jpeg',
      'webp',
    ])
    assert.equal(byName.get('output_compression')?.range, '0 ~ 100')
    assert.deepEqual(byName.get('moderation')?.enumValues, ['auto', 'low'])
    assert.equal(byName.get('stream')?.defaultValue, false)
    assert.equal(byName.get('partial_images')?.range, '0 ~ 3')
    assert.equal(byName.get('images / image[]')?.range, 'Up to 16 images')
  })

  test('keeps legacy image parameters for other image models', () => {
    const parameters = buildSupportedParameters({
      ...gptImage2,
      model_name: 'dall-e-3',
    })

    assert.deepEqual(
      parameters.map((parameter) => parameter.name),
      ['prompt', 'size', 'quality', 'style', 'n', 'response_format']
    )
  })
})
