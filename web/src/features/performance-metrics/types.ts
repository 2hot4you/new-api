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
export type PerformanceSeriesPoint = {
  ts: number
  request_count: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
}

export type PerformanceGroup = {
  group: string
  request_count: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  series: PerformanceSeriesPoint[]
}

export type PerformanceMetricsData = {
  success: boolean
  message?: string
  data: {
    model_name: string
    series_schema?: string
    groups: PerformanceGroup[]
  }
}

export type PerfModelSummary = {
  model_name: string
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_rates?: number[]
  request_count?: number
}

export type PerfGroupSummary = {
  group: string
  request_count: number
  success_rate: number | null
}

export type PerfSummaryAllData = {
  success: boolean
  message?: string
  data: {
    models: PerfModelSummary[]
    groups: PerfGroupSummary[]
  }
}

export type VideoPerformanceAggregate = {
  submitted_count: number
  success_count: number
  failure_count: number
  pending_count: number
  success_rate: number
  average_duration_seconds: number
  p50_duration_seconds: number
  p95_duration_seconds: number
  slow_task_count: number
}

export type VideoPerformanceGroup = VideoPerformanceAggregate & {
  group: string
}

export type VideoPerformancePoint = VideoPerformanceAggregate & {
  ts: number
}

export type VideoPerformanceMetricsData = {
  success: boolean
  message?: string
  data: {
    model_name: string
    hours: number
    summary: VideoPerformanceAggregate
    groups: VideoPerformanceGroup[]
    series: VideoPerformancePoint[]
  }
}
