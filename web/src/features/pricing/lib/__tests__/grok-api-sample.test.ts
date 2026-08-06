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

  test('covers video creation, editing, status and download', () => {
    assert.deepEqual(getGrokOperations('grok-imagine-video'), [
      'generate',
      'edit',
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
    assert.match(curlDownload, /MOLII_API_KEY' \\\n  --output grok-result\.mp4/)
  })

  test('does not advertise video editing for video 1.5', () => {
    assert.deepEqual(getGrokOperations('grok-imagine-video-1.5'), [
      'generate',
      'status',
      'download',
    ])
  })
})
