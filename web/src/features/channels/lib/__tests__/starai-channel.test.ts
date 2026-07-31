/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

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
  CHANNEL_TYPES,
  CHANNEL_TYPE_OPTIONS,
  CHANNEL_TYPE_STARAI,
  MODEL_FETCHABLE_TYPES,
  STARAI_MODELS,
} from '../../constants'
import { getChannelTypeConfig } from '../channel-type-config'
import {
  getChannelTypeIcon,
  getKeyPromptForType,
  getRelatedModelsForChannelType,
} from '../channel-utils'

describe('StarAI channel', () => {
  test('registers channel 61 as a video provider with Doubao icon metadata', () => {
    assert.equal(CHANNEL_TYPE_STARAI, 61)
    assert.equal(CHANNEL_TYPES[CHANNEL_TYPE_STARAI], 'StarAI')
    assert.deepEqual(
      CHANNEL_TYPE_OPTIONS.find((item) => item.value === CHANNEL_TYPE_STARAI),
      { value: CHANNEL_TYPE_STARAI, label: 'StarAI' }
    )
    assert.equal(
      CHANNEL_TYPE_OPTIONS.findIndex(
        (item) => item.value === CHANNEL_TYPE_STARAI
      ),
      CHANNEL_TYPE_OPTIONS.findIndex((item) => item.value === 54) + 1
    )
    assert.equal(getChannelTypeIcon(CHANNEL_TYPE_STARAI), 'Doubao')
    assert.equal(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_STARAI), false)
  })

  test('registers the default URL, key hint, and exactly two Seedance models', () => {
    const config = getChannelTypeConfig(CHANNEL_TYPE_STARAI)

    assert.equal(config.defaultBaseUrl, 'https://ai-api.lfxqai.com')
    assert.equal(config.icon, 'Doubao')
    assert.equal(
      getKeyPromptForType(CHANNEL_TYPE_STARAI),
      'Enter API key for this channel'
    )
    assert.deepEqual(config.supportedModels, [...STARAI_MODELS])
    assert.deepEqual(
      getRelatedModelsForChannelType(CHANNEL_TYPE_STARAI, [
        'gpt-5',
        'unrelated-video-model',
      ]),
      [...STARAI_MODELS]
    )
  })

  test('keeps existing related-model behavior for other channel types', () => {
    const allModels = ['gpt-5', 'text-embedding-3-large', 'claude-opus-4-1']

    assert.deepEqual(getRelatedModelsForChannelType(1, allModels), [
      'gpt-5',
      'text-embedding-3-large',
    ])
    assert.equal(getRelatedModelsForChannelType(14, allModels), allModels)
  })
})
