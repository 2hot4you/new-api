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

import { describe, test } from 'vitest'

import { buildUsageLogRequestLink } from '../task-log-links'

describe('task log request links', () => {
  test('builds an exact request search with a bounded task time range', () => {
    assert.equal(
      buildUsageLogRequestLink('req /?', 1_000, 1_020),
      '/usage-logs/common?requestId=req+%2F%3F&startTime=700000&endTime=1320000'
    )
  })

  test('keeps the request filter when historical timestamps are unavailable', () => {
    assert.equal(
      buildUsageLogRequestLink('request-legacy'),
      '/usage-logs/common?requestId=request-legacy'
    )
  })
})
