import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildGrokApiSample, getGrokOperations } from '../grok-api-sample'

describe('Grok Imagine API samples', () => {
  test('uses Molii image fields instead of generic OpenAI size', () => {
    const sample = buildGrokApiSample('curl', {
      baseUrl: 'https://aigc.claudeye.com',
      modelName: 'grok-imagine-image',
      operation: 'generate',
    })
    assert.match(sample, /\/v1\/images\/generations/)
    assert.match(sample, /"resolution": "1k"/)
    assert.match(sample, /"aspect_ratio": "16:9"/)
    assert.doesNotMatch(sample, /"size"/)
  })

  test('includes the official medium quality default for image 2.0', () => {
    const sample = buildGrokApiSample('curl', {
      baseUrl: 'https://aigc.claudeye.com',
      modelName: 'grok-imagine-image-2.0',
      operation: 'generate',
    })
    assert.match(sample, /"quality": "medium"/)
  })

  test('covers video creation, editing, status and download', () => {
    assert.deepEqual(getGrokOperations('grok-imagine-video'), [
      'generate',
      'edit',
      'extend',
      'status',
      'download',
    ])
    const edit = buildGrokApiSample('python', {
      baseUrl: 'https://aigc.claudeye.com',
      modelName: 'grok-imagine-video',
      operation: 'edit',
    })
    assert.match(edit, /\/v1\/videos\/edits/)
    assert.match(edit, /video/)
    const download = buildGrokApiSample('javascript', {
      baseUrl: 'https://aigc.claudeye.com',
      modelName: 'grok-imagine-video',
      operation: 'download',
    })
    assert.match(download, /\/v1\/videos\/task_xxx\/content/)
    assert.match(download, /arrayBuffer/)

    const curlDownload = buildGrokApiSample('curl', {
      baseUrl: 'https://aigc.claudeye.com',
      modelName: 'grok-imagine-video',
      operation: 'download',
    })
    assert.match(
      curlDownload,
      /MOLII_API_KEY' \\\n {2}--output grok-result\.mp4/
    )
  })

  test('documents video extension with a Molii file id', () => {
    const extension = buildGrokApiSample('curl', {
      baseUrl: 'http://127.0.0.1:3000',
      modelName: 'grok-imagine-video',
      operation: 'extend',
    })
    assert.match(extension, /\/v1\/videos\/extensions/)
    assert.match(extension, /"file_id": "file_video_xxx"/)
    assert.match(extension, /"duration": 6/)
    assert.doesNotMatch(extension, /aspect_ratio|resolution/)
  })

  test('does not advertise video editing for video 1.5', () => {
    assert.deepEqual(getGrokOperations('grok-imagine-video-1.5'), [
      'generate',
      'reference',
      'status',
      'download',
    ])
    const video15 = buildGrokApiSample('curl', {
      baseUrl: 'https://aigc.claudeye.com',
      modelName: 'grok-imagine-video-1.5',
      operation: 'generate',
    })
    assert.match(video15, /"model": "grok-imagine-video-1\.5"/)
    assert.match(video15, /"image":/)
    assert.match(video15, /"resolution": "720p"/)

    const references = buildGrokApiSample('curl', {
      baseUrl: 'http://127.0.0.1:3000',
      modelName: 'grok-imagine-video-1.5',
      operation: 'reference',
    })
    assert.match(references, /"reference_images":/)
    assert.match(references, /"file_id": "file_reference_xxx"/)
  })
})
