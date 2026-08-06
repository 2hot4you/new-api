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

import { buildGrokImageLogRequest, buildMidjourneyLogRequest } from '../../api'

describe('usage log data-source routing', () => {
  test('routes admin Image API logs to the common log endpoint with a category', () => {
    assert.deepEqual(buildGrokImageLogRequest({ p: 3 }, true), {
      path: '/api/log',
      params: { p: 3, log_category: 'grok_image' },
    })
  })

  test('routes self Image API logs to the self common log endpoint', () => {
    assert.deepEqual(buildGrokImageLogRequest({ page_size: 20 }, false), {
      path: '/api/log/self',
      params: { page_size: 20, log_category: 'grok_image' },
    })
  })

  test('keeps Midjourney on its existing independent endpoint', () => {
    assert.deepEqual(buildMidjourneyLogRequest({ mj_id: 'task-1' }, true), {
      path: '/api/mj',
      params: { mj_id: 'task-1' },
    })
  })
})
