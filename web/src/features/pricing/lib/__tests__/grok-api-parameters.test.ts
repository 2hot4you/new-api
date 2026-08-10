import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildGrokApiParameters } from '../grok-api-parameters'

function parameter(
  modelName: string,
  operation: 'generate' | 'edit' | 'status' | 'download',
  name: string
) {
  return buildGrokApiParameters(modelName, operation).find(
    (item) => item.name === name
  )
}

describe('Grok Imagine API parameters', () => {
  test('describes image generation limits and defaults', () => {
    const params = buildGrokApiParameters('grok-imagine-image', 'generate')

    assert.deepEqual(
      params.map((item) => item.name),
      ['model', 'prompt', 'aspect_ratio', 'resolution', 'n']
    )
    assert.equal(
      parameter('grok-imagine-image', 'generate', 'model')?.required,
      true
    )
    assert.equal(
      parameter('grok-imagine-image', 'generate', 'prompt')?.required,
      true
    )
    assert.deepEqual(
      parameter('grok-imagine-image', 'generate', 'resolution')?.enumValues,
      ['1k', '2k']
    )
    assert.equal(
      parameter('grok-imagine-image', 'generate', 'n')?.defaultValue,
      1
    )
    assert.equal(parameter('grok-imagine-image', 'generate', 'n')?.range, '1–4')
  })

  test('adds both accepted image edit inputs and their collective limit', () => {
    const params = buildGrokApiParameters('grok-imagine-image-quality', 'edit')

    assert.deepEqual(
      params.map((item) => item.name),
      ['model', 'prompt', 'image', 'images', 'aspect_ratio', 'resolution', 'n']
    )
    assert.equal(
      parameter('grok-imagine-image-quality', 'edit', 'image')?.range,
      '1–3 input images in total'
    )
    assert.equal(
      parameter('grok-imagine-image-quality', 'edit', 'images')?.range,
      '1–3 input images in total'
    )
  })

  test('distinguishes legacy video and video 1.5 generation', () => {
    const legacyImage = parameter('grok-imagine-video', 'generate', 'image')
    const video15Image = parameter(
      'grok-imagine-video-1.5',
      'generate',
      'image'
    )
    assert.equal(legacyImage?.required, false)
    assert.equal(video15Image?.required, true)
    assert.deepEqual(
      parameter('grok-imagine-video', 'generate', 'resolution')?.enumValues,
      ['480p', '720p']
    )
    assert.deepEqual(
      parameter('grok-imagine-video-1.5', 'generate', 'resolution')?.enumValues,
      ['480p', '720p', '1080p']
    )
    assert.equal(
      parameter('grok-imagine-video-1.5', 'generate', 'duration')?.range,
      '1–15 seconds'
    )
  })

  test('video editing only exposes fields accepted by the backend', () => {
    const params = buildGrokApiParameters('grok-imagine-video', 'edit')

    assert.deepEqual(
      params.map((item) => item.name),
      ['model', 'prompt', 'video']
    )
    assert.equal(
      parameter('grok-imagine-video', 'edit', 'video')?.required,
      true
    )
  })

  test('task status and download only require the public task id', () => {
    for (const operation of ['status', 'download'] as const) {
      assert.deepEqual(
        buildGrokApiParameters('grok-imagine-video', operation),
        [
          {
            name: 'task_id',
            type: 'string',
            descriptionKey:
              'Molii public task ID returned by the creation request',
            required: true,
          },
        ]
      )
    }
  })
})
