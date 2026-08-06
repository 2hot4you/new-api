import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getGrokModelCapabilities,
  isGrokImageModel,
  isGrokImagineModel,
  isGrokVideoModel,
} from '../grok-model'

describe('Grok Imagine model metadata', () => {
  test('recognizes exactly the four Molii Grok models', () => {
    assert.equal(isGrokImagineModel('grok-imagine-image'), true)
    assert.equal(isGrokImagineModel('grok-imagine-image-quality'), true)
    assert.equal(isGrokImagineModel('grok-imagine-video'), true)
    assert.equal(isGrokImagineModel('grok-imagine-video-1.5'), true)
    assert.equal(isGrokImagineModel('grok-4'), false)
    assert.equal(isGrokImageModel('grok-imagine-video'), false)
    assert.equal(isGrokVideoModel('grok-imagine-video'), true)
  })

  test('keeps legacy video and video 1.5 capabilities distinct', () => {
    assert.ok(
      getGrokModelCapabilities('grok-imagine-video').operations.includes(
        'Video editing'
      )
    )
    const video15 = getGrokModelCapabilities('grok-imagine-video-1.5')
    assert.deepEqual(video15.operations, ['Image-to-video'])
    assert.ok(video15.resolutions.includes('1080p'))
  })
})
