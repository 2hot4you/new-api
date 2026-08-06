/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  GENERATION_LOG_META,
  GENERATION_LOG_SOURCES,
  getVideoPlatformForSource,
  resolveUsageLogSource,
  resolveVideoLogSource,
} from '../../source-registry'

describe('generation log source registry', () => {
  test('exposes product-facing page and section labels', () => {
    assert.deepEqual(GENERATION_LOG_META, {
      titleKey: 'Generation Records',
      sections: {
        drawing: { labelKey: 'Image Generation' },
        task: { labelKey: 'Video Generation' },
      },
    })

    const sourceLabels: string[] = Object.values(GENERATION_LOG_SOURCES)
      .flat()
      .map((source) => source.labelKey)
    assert.equal(
      sourceLabels.some((label) => label === 'Image API'),
      false
    )
    assert.equal(
      sourceLabels.some((label) => label === 'Midjourney'),
      false
    )
  })

  test('declares the current image and video model families in display order', () => {
    assert.deepEqual(GENERATION_LOG_SOURCES.drawing, [
      { id: 'grok-image', labelKey: 'Grok Image' },
    ])
    assert.deepEqual(GENERATION_LOG_SOURCES.task, [
      { id: 'grok-video', labelKey: 'Grok Video', platform: '62' },
      { id: 'seedance', labelKey: 'Seedance', platform: '61' },
    ])
  })

  test('resolves missing, invalid, and cross-section sources to section defaults', () => {
    assert.equal(resolveUsageLogSource('drawing'), 'grok-image')
    assert.equal(resolveUsageLogSource('drawing', 'seedance'), 'grok-image')
    assert.equal(resolveUsageLogSource('task'), 'grok-video')
    assert.equal(resolveUsageLogSource('task', 'grok-image'), 'grok-video')
    assert.equal(resolveUsageLogSource('task', 'seedance'), 'seedance')
  })

  test('maps video model families to stable backend task platforms', () => {
    assert.equal(resolveVideoLogSource('seedance'), 'seedance')
    assert.equal(resolveVideoLogSource('grok-image'), 'grok-video')
    assert.equal(getVideoPlatformForSource('grok-video'), '62')
    assert.equal(getVideoPlatformForSource('seedance'), '61')
    assert.equal(getVideoPlatformForSource('grok-image'), undefined)
  })
})
