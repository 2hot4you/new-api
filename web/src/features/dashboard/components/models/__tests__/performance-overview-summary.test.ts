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

import { buildPerformanceSummary } from '../performance-overview-summary'

describe('performance overview summary', () => {
  test('weights success rate by exact request samples', () => {
    const summary = buildPerformanceSummary([
      {
        model_name: 'model-a',
        avg_latency_ms: 100,
        success_rate: 100,
        avg_tps: 20,
        request_count: 1,
        success_count: 1,
      },
      {
        model_name: 'model-b',
        avg_latency_ms: 300,
        success_rate: 0,
        avg_tps: 40,
        request_count: 9,
        success_count: 0,
      },
    ])

    assert.equal(summary.successCount, 1)
    assert.equal(summary.totalRequests, 10)
    assert.equal(summary.successRate, 10)
    assert.equal(summary.avgLatencyMs, 200)
    assert.equal(summary.avgTps, 30)
  })

  test('ignores malformed sample counts instead of inventing traffic', () => {
    const summary = buildPerformanceSummary([
      {
        model_name: 'legacy-model',
        avg_latency_ms: 120,
        success_rate: 75,
        avg_tps: 10,
      },
    ])

    assert.equal(summary.successCount, 0)
    assert.equal(summary.totalRequests, 0)
    assert.ok(Number.isNaN(summary.successRate))
  })
})
