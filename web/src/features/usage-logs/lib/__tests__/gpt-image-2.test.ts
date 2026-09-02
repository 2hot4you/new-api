import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import {
  getGPTImage2ListSummary,
  getGPTImage2LogState,
  isGPTImage2Log,
  parseGPTImage2Log,
} from '../gpt-image-2'

const snapshot = {
  version: 1,
  model: 'gpt-image-2',
  operation: 'generation',
  quality: 'high',
  background: 'transparent',
  output_format: 'webp',
  moderation: 'low',
  size: '1536x1024',
  user: 'customer-42',
  requested_output_count: 3,
  output_count: 2,
}

describe('GPT Image 2 log contract', () => {
  test('parses the complete versioned snapshot', () => {
    assert.deepEqual(
      parseGPTImage2Log(JSON.stringify({ gpt_image_2: snapshot })),
      snapshot
    )
  })

  test('rejects malformed snapshots instead of synthesizing fields', () => {
    assert.equal(
      parseGPTImage2Log(
        JSON.stringify({ gpt_image_2: { ...snapshot, version: 2 } })
      ),
      null
    )
    assert.equal(
      parseGPTImage2Log(
        JSON.stringify({
          gpt_image_2: { ...snapshot, output_count: 0 },
        })
      ),
      null
    )
  })

  test('preserves an upstream output count that exceeds the request', () => {
    const upstreamExpanded = { ...snapshot, output_count: 4 }
    assert.deepEqual(
      parseGPTImage2Log(JSON.stringify({ gpt_image_2: upstreamExpanded })),
      upstreamExpanded
    )
  })

  test('distinguishes current, historical, and unrelated logs', () => {
    assert.equal(
      getGPTImage2LogState({
        model_name: 'gpt-image-2',
        other: JSON.stringify({ gpt_image_2: snapshot }),
      }).kind,
      'current'
    )
    assert.deepEqual(
      getGPTImage2LogState({ model_name: 'gpt-image-2', other: '{}' }),
      { kind: 'history', model: 'gpt-image-2' }
    )
    assert.equal(isGPTImage2Log({ model_name: 'gpt-image-1' }), false)
  })

  test('formats a compact list summary from actual output data', () => {
    assert.equal(
      getGPTImage2ListSummary({
        model_name: 'gpt-image-2',
        other: JSON.stringify({ gpt_image_2: snapshot }),
      }),
      'HIGH · 1536x1024 · WEBP · 2 images'
    )
  })
})
