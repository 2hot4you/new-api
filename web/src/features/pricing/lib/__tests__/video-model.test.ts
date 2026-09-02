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
import { describe, test } from 'vitest'

import {
  formatVideoDuration,
  getVideoResolutions,
  isOpenAIVideoModel,
} from '../video-model'

describe('video model helpers', () => {
  test('identifies video models from their endpoint instead of their name', () => {
    assert.equal(
      isOpenAIVideoModel({
        id: 1,
        model_name: 'doubao-seedance-2-0-260128',
        quota_type: 0,
        model_ratio: 23,
        completion_ratio: 1,
        enable_groups: ['default'],
        supported_endpoint_types: ['openai-video'],
      }),
      true
    )
  })

  test('uses configured pricing rows as the supported resolution list', () => {
    const resolutions = getVideoResolutions({
      id: 1,
      model_name: 'doubao-seedance-2-0-fast-260128',
      quota_type: 0,
      model_ratio: 18.5,
      completion_ratio: 1,
      enable_groups: ['default'],
      video_pricing: {
        unit: 'cny_per_million_tokens',
        fps: 24,
        extra_frames: 1,
        rows: [
          {
            resolutions: ['480p', '720p'],
            without_video: 37,
            with_video: 22,
          },
        ],
      },
    })

    assert.deepEqual(resolutions, ['480p', '720p'])
  })

  test('formats long video generation durations without abbreviation loss', () => {
    assert.equal(formatVideoDuration(59), '59s')
    assert.equal(formatVideoDuration(600), '10m')
    assert.equal(formatVideoDuration(661), '11m 1s')
  })
})
