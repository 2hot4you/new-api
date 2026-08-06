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

import { buildGrokImageLogRequest, buildVideoTaskLogRequest } from '../../api'
import { shouldReuseLogPlaceholder } from '../utils'

describe('usage log data-source routing', () => {
  test('reuses cached rows only for the same category and model family', () => {
    assert.equal(
      shouldReuseLogPlaceholder(
        ['logs', 'task', 'grok-video'],
        'task',
        'grok-video'
      ),
      true
    )
    assert.equal(
      shouldReuseLogPlaceholder(
        ['logs', 'task', 'grok-video'],
        'task',
        'seedance'
      ),
      false
    )
    assert.equal(
      shouldReuseLogPlaceholder(
        ['logs', 'image', 'grok-image'],
        'task',
        'grok-video'
      ),
      false
    )
  })

  test('routes admin Grok Image logs to the common log endpoint with a category', () => {
    assert.deepEqual(buildGrokImageLogRequest({ p: 3 }, true), {
      path: '/api/log',
      params: { p: 3, log_category: 'grok_image' },
    })
  })

  test('routes self Grok Image logs to the self common log endpoint', () => {
    assert.deepEqual(buildGrokImageLogRequest({ page_size: 20 }, false), {
      path: '/api/log/self',
      params: { page_size: 20, log_category: 'grok_image' },
    })
  })

  test('routes Grok Video task logs through platform 62', () => {
    assert.deepEqual(
      buildVideoTaskLogRequest(
        { p: 2, task_id: 'task-grok' },
        true,
        'grok-video'
      ),
      {
        path: '/api/task',
        params: { p: 2, task_id: 'task-grok', platform: '62' },
      }
    )
  })

  test('routes self Seedance task logs through platform 61', () => {
    assert.deepEqual(
      buildVideoTaskLogRequest(
        { page_size: 20, task_id: 'task-seedance' },
        false,
        'seedance'
      ),
      {
        path: '/api/task/self',
        params: {
          page_size: 20,
          task_id: 'task-seedance',
          platform: '61',
        },
      }
    )
  })

  test('rejects image sources for video task requests', () => {
    assert.throws(() => buildVideoTaskLogRequest({}, true, 'grok-image'), {
      message: 'Unsupported video log source: grok-image',
    })
  })
})
