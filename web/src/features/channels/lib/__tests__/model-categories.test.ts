import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { categorizeModels, getModelCategory } from '../model-categories'

describe('fetched model categories', () => {
  test('recognizes the Molii model catalog vendors', () => {
    assert.equal(getModelCategory('grok-imagine-video'), 'xAI')
    assert.equal(getModelCategory('doubao-seedance-2-0-260128'), 'Doubao')
    assert.equal(getModelCategory('minimax-m3'), 'MiniMax')
    assert.equal(getModelCategory('qwen3.5-plus'), 'Qwen')
    assert.equal(getModelCategory('glm-5.2'), 'Zhipu')
  })

  test('preserves source order inside each category', () => {
    assert.deepEqual(
      categorizeModels(['qwen3.5-flash', 'grok-imagine-video', 'qwen3.5-plus']),
      {
        Qwen: ['qwen3.5-flash', 'qwen3.5-plus'],
        xAI: ['grok-imagine-video'],
      }
    )
  })
})
