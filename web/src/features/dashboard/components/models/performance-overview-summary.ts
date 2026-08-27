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
import type { PerfModelSummary } from '@/features/performance-metrics/types'

type AveragedMetric = 'avg_latency_ms' | 'avg_tps'

export type PerformanceSummary = {
  successCount: number
  totalRequests: number
  avgLatencyMs: number
  avgTps: number
  successRate: number
}

function simpleAverage(
  rows: PerfModelSummary[],
  metric: AveragedMetric,
  isValid: (value: number) => boolean
): number {
  let total = 0
  let count = 0

  for (const row of rows) {
    const value = Number(row[metric])
    if (!isValid(value)) continue
    total += value
    count++
  }

  return count > 0 ? total / count : Number.NaN
}

function validCount(value: unknown): number {
  const count = Number(value)
  return Number.isInteger(count) && count >= 0 ? count : 0
}

export function buildPerformanceSummary(
  rows: PerfModelSummary[]
): PerformanceSummary {
  let successCount = 0
  let totalRequests = 0

  for (const row of rows) {
    const requestCount = validCount(row.request_count)
    const rowSuccessCount = Math.min(
      validCount(row.success_count),
      requestCount
    )
    totalRequests += requestCount
    successCount += rowSuccessCount
  }

  return {
    successCount,
    totalRequests,
    avgLatencyMs: Math.round(
      simpleAverage(
        rows,
        'avg_latency_ms',
        (value) => Number.isFinite(value) && value > 0
      )
    ),
    avgTps: simpleAverage(
      rows,
      'avg_tps',
      (value) => Number.isFinite(value) && value > 0
    ),
    successRate:
      totalRequests > 0 ? (successCount / totalRequests) * 100 : Number.NaN,
  }
}
