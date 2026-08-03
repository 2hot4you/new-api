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

import { buildSupportedParameters } from '../../lib/mock-stats'
import { buildVideoSample } from '../../lib/video-api-sample'
import type { PricingModel } from '../../types'

const seedanceModel: PricingModel = {
  id: 1,
  model_name: 'doubao-seedance-2-0-fast-260128',
  quota_type: 0,
  model_ratio: 18.5,
  completion_ratio: 1,
  enable_groups: ['default'],
  supported_endpoint_types: ['openai-video'],
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
    unsupported_resolutions: ['1080p', '4K'],
  },
}

describe('Seedance API details', () => {
  test('builds an asynchronous video cURL sample instead of a chat request', () => {
    const sample = buildVideoSample('curl', {
      baseUrl: 'https://api.molii.co',
      apiKeyEnv: 'MOLII_TOKEN',
      modelName: seedanceModel.model_name,
      endpointPath: '/v1/video/generations',
    })

    assert.match(sample, /POST|curl -sS/)
    assert.match(sample, /"content"/)
    assert.match(sample, /"resolution": "720p"/)
    assert.match(sample, /TASK_ID/)
    assert.match(sample, /video\/generations\/\$TASK_ID/)
    assert.doesNotMatch(sample, /"messages"/)
    assert.doesNotMatch(sample, /temperature/)
  })

  test('annotates each asynchronous video workflow step', () => {
    const context = {
      baseUrl: 'https://api.molii.co',
      apiKeyEnv: 'MOLII_TOKEN',
      modelName: seedanceModel.model_name,
      endpointPath: '/v1/video/generations',
    }

    const curl = buildVideoSample('curl', context)
    const python = buildVideoSample('python', context)
    const typescript = buildVideoSample('typescript', context)
    const javascript = buildVideoSample('javascript', context)

    assert.match(curl, /# Create an asynchronous video task/)
    assert.match(curl, /# Query the task status with the public task ID/)
    assert.match(python, /# Create an asynchronous video task/)
    assert.match(python, /# Query the task status with the public task ID/)
    assert.match(typescript, /\/\/ Create an asynchronous video task/)
    assert.match(
      typescript,
      /\/\/ Query the task status with the public task ID/
    )
    assert.match(javascript, /\/\/ Create an asynchronous video task/)
    assert.match(
      javascript,
      /\/\/ Query the task status with the public task ID/
    )
  })

  test('shows only resolutions supported by the selected Seedance model', () => {
    const parameters = buildSupportedParameters(seedanceModel)
    const names = parameters.map((parameter) => parameter.name)
    const resolution = parameters.find(
      (parameter) => parameter.name === 'resolution'
    )
    const content = parameters.find((parameter) => parameter.name === 'content')
    const audio = parameters.find(
      (parameter) => parameter.name === 'generate_audio'
    )
    const duration = parameters.find(
      (parameter) => parameter.name === 'duration'
    )
    const ratio = parameters.find((parameter) => parameter.name === 'ratio')
    const watermark = parameters.find(
      (parameter) => parameter.name === 'watermark'
    )
    const tools = parameters.find((parameter) => parameter.name === 'tools')

    assert.deepEqual(names, [
      'model',
      'content',
      'generate_audio',
      'resolution',
      'duration',
      'ratio',
      'watermark',
      'tools',
    ])
    assert.deepEqual(resolution?.enumValues, ['480p', '720p'])
    assert.equal(resolution?.defaultValue, '720p')
    assert.equal(
      content?.range,
      'At least 1 item; up to 9 images, 3 videos, and 3 audio files'
    )
    assert.deepEqual(audio?.enumValues, ['true', 'false'])
    assert.equal(duration?.range, '-1 for smart duration, or 4–15 seconds')
    assert.deepEqual(ratio?.enumValues, [
      'adaptive',
      '16:9',
      '4:3',
      '1:1',
      '3:4',
      '9:16',
      '21:9',
    ])
    assert.deepEqual(watermark?.enumValues, ['true', 'false'])
    assert.equal(tools?.range, '0 or more items; each type must be web_search')
    assert.deepEqual(tools?.enumValues, ['web_search'])
  })

  test('uses the lowercase 4k value accepted by the video API', () => {
    const videoPricing = seedanceModel.video_pricing
    assert.ok(videoPricing)
    const parameters = buildSupportedParameters({
      ...seedanceModel,
      video_pricing: {
        ...videoPricing,
        rows: [
          ...videoPricing.rows,
          {
            resolutions: ['4K'],
            without_video: 26,
            with_video: 16,
          },
        ],
      },
    })
    const resolution = parameters.find(
      (parameter) => parameter.name === 'resolution'
    )

    assert.deepEqual(resolution?.enumValues, ['480p', '720p', '4k'])
  })
})
