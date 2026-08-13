import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  advanceModelPlaceholder,
  createModelPlaceholderState,
  getModelPlaceholderDelay,
  normalizeModelIds,
} from '../use-model-placeholder-typewriter'

describe('homepage model placeholder typewriter', () => {
  test('uses the enabled catalog order while removing empty and duplicate IDs', () => {
    assert.deepEqual(
      normalizeModelIds([
        'deepseek-v4',
        '',
        'qwen3.5-plus',
        'deepseek-v4',
        '  glm-5.2  ',
      ]),
      ['deepseek-v4', 'qwen3.5-plus', 'glm-5.2']
    )
  })

  test('types, holds, deletes, and advances to the next model', () => {
    const ids = ['ab', 'xyz']
    let state = createModelPlaceholderState()

    state = advanceModelPlaceholder(state, ids)
    assert.deepEqual(state, { modelIndex: 0, phase: 'typing', text: 'a' })

    state = advanceModelPlaceholder(state, ids)
    assert.deepEqual(state, { modelIndex: 0, phase: 'holding', text: 'ab' })

    state = advanceModelPlaceholder(state, ids)
    assert.deepEqual(state, { modelIndex: 0, phase: 'deleting', text: 'ab' })

    state = advanceModelPlaceholder(state, ids)
    assert.equal(state.text, 'a')
    state = advanceModelPlaceholder(state, ids)
    assert.deepEqual(state, { modelIndex: 1, phase: 'typing', text: '' })
    state = advanceModelPlaceholder(state, ids)
    assert.equal(state.text, 'x')
  })

  test('finishes typing every model ID within three seconds', () => {
    const shortDelay = getModelPlaceholderDelay(
      { modelIndex: 0, phase: 'typing', text: '' },
      ['glm-5.2']
    )
    const longModel = 'a'.repeat(120)
    const longDelay = getModelPlaceholderDelay(
      { modelIndex: 0, phase: 'typing', text: '' },
      [longModel]
    )

    assert.ok(shortDelay * 'glm-5.2'.length <= 3000)
    assert.ok(longDelay * longModel.length <= 3000)
  })

  test('deletes faster than it types and keeps a readable hold phase', () => {
    const ids = ['deepseek-v4']
    const typing = getModelPlaceholderDelay(
      { modelIndex: 0, phase: 'typing', text: 'deep' },
      ids
    )
    const holding = getModelPlaceholderDelay(
      { modelIndex: 0, phase: 'holding', text: 'deepseek-v4' },
      ids
    )
    const deleting = getModelPlaceholderDelay(
      { modelIndex: 0, phase: 'deleting', text: 'deepseek-v4' },
      ids
    )

    assert.ok(deleting < typing)
    assert.ok(holding >= 500)
  })
})
