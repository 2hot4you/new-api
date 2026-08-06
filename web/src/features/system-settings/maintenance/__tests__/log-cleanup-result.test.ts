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

import { getLogCleanupResultCounts } from '../log-cleanup-result'

describe('log cleanup result counts', () => {
  test('prefers category counts and preserves legacy aggregate fallback', () => {
    assert.deepEqual(
      getLogCleanupResultCounts({
        deleted_count: 7,
        deleted_log_count: 5,
        deleted_generation_count: 2,
      }),
      { total: 7, logs: 5, generations: 2, categorized: true }
    )
    assert.deepEqual(getLogCleanupResultCounts({ deleted_count: 3 }), {
      total: 3,
      logs: 3,
      generations: 0,
      categorized: false,
    })
  })

  test('clamps malformed historical values to zero', () => {
    assert.deepEqual(
      getLogCleanupResultCounts({
        deleted_count: Number.NaN,
        deleted_log_count: -1,
        deleted_generation_count: Number.POSITIVE_INFINITY,
      }),
      { total: 0, logs: 0, generations: 0, categorized: true }
    )
  })
})
