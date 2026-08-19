import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'node:test'

import { buildGrokApiParameters } from '../grok-api-parameters'
import type { GrokOperation } from '../grok-api-sample'

function parameter(modelName: string, operation: GrokOperation, name: string) {
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
    assert.match(
      params.map((item) => item.descriptionKey).join(' '),
      /file_id/i
    )
  })

  test('exposes the quality tiers used by Grok Imagine Image 2.0 billing', () => {
    const quality = parameter('grok-imagine-image-2.0', 'generate', 'quality')
    assert.equal(quality?.defaultValue, 'medium')
    assert.deepEqual(quality?.enumValues, ['low', 'medium'])
    assert.equal(
      parameter('grok-imagine-image', 'generate', 'quality'),
      undefined
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
    assert.equal(
      parameter('grok-imagine-video', 'generate', 'prompt')?.required,
      true
    )
    assert.deepEqual(
      parameter('grok-imagine-video', 'generate', 'aspect_ratio')?.enumValues,
      ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3']
    )
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
    assert.match(
      buildGrokApiParameters('grok-imagine-video-1.5', 'generate')
        .map((item) => item.descriptionKey)
        .join(' '),
      /file_id/i
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
    assert.match(
      params.map((item) => item.descriptionKey).join(' '),
      /file_id/i
    )
  })

  test('video extension and reference modes expose only accepted fields', () => {
    const extension = buildGrokApiParameters('grok-imagine-video', 'extend')
    assert.deepEqual(
      extension.map((item) => item.name),
      ['model', 'prompt', 'video', 'duration']
    )
    assert.equal(
      parameter('grok-imagine-video', 'extend', 'duration')?.range,
      '2–10 seconds'
    )
    assert.equal(
      parameter('grok-imagine-video', 'extend', 'duration')?.defaultValue,
      6
    )

    const references = buildGrokApiParameters(
      'grok-imagine-video-1.5',
      'reference'
    )
    assert.deepEqual(
      references.map((item) => item.name),
      [
        'model',
        'prompt',
        'reference_images',
        'duration',
        'aspect_ratio',
        'resolution',
      ]
    )
    assert.equal(
      parameter('grok-imagine-video-1.5', 'reference', 'reference_images')
        ?.range,
      '1–7 images'
    )
    assert.deepEqual(
      parameter('grok-imagine-video-1.5', 'reference', 'resolution')
        ?.enumValues,
      ['480p', '720p']
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

  test('published locale strings do not advertise retired or unsupported Grok inputs', () => {
    for (const locale of ['en', 'zh']) {
      const document = JSON.parse(
        readFileSync(
          new URL(`../../../../i18n/locales/${locale}.json`, import.meta.url),
          'utf8'
        )
      ) as { translation: Record<string, string> }
      for (const retiredKey of [
        'Single input image; provide image or images and include url or file_id',
        'Multiple input images; provide image or images and include url or file_id',
        'Input image with url or file_id; required by grok-imagine-video-1.5',
        'Optional input image with url or file_id for image-to-video generation',
        'Input video with url or file_id to edit',
        'Grok Imagine Video 1.5 Preview model description',
      ]) {
        assert.equal(document.translation[retiredKey], undefined)
      }
    }
  })
})
