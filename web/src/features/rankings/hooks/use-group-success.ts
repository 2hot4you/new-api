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
import { useQuery } from '@tanstack/react-query'

import { getPerfMetricsSummary } from '@/features/performance-metrics/api'

import type { RankingPeriod } from '../types'

export const RANKING_PERIOD_HOURS: Record<RankingPeriod, number> = {
  today: 24,
  week: 168,
  month: 720,
  year: 8760,
}

export function useGroupSuccess(period: RankingPeriod) {
  const hours = RANKING_PERIOD_HOURS[period]

  return useQuery({
    queryKey: ['rankings-group-success', hours],
    queryFn: () => getPerfMetricsSummary(hours),
    staleTime: 60 * 1000,
    retry: false,
  })
}
