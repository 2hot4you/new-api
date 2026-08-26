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
*/
import { useQuery } from '@tanstack/react-query'
import { createElement, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { getPerfMetricsSummary } from '@/features/performance-metrics/api'

export type GroupSuccessRatesStatus = 'error' | 'loading' | 'ready'

type GroupSuccessRatesResult = {
  data: Record<string, number | null>
  status: GroupSuccessRatesStatus
}

export function useGroupSuccessRates(enabled = true): GroupSuccessRatesResult {
  const query = useQuery({
    queryKey: ['perf-metrics-summary', 24],
    queryFn: () => getPerfMetricsSummary(24),
    staleTime: 60 * 1000,
    retry: false,
    enabled,
  })

  const data = useMemo(() => {
    const rates: Record<string, number | null> = {}
    for (const group of query.data?.data?.groups ?? []) {
      rates[group.group] =
        group.request_count > 0 && Number.isFinite(group.success_rate)
          ? group.success_rate
          : null
    }
    return rates
  }, [query.data])

  if (query.isPending) return { data, status: 'loading' }
  if (query.isError || !query.data?.success || !query.data.data) {
    return { data: {}, status: 'error' }
  }
  return { data, status: 'ready' }
}

export function getGroupSuccessRate(
  group: string,
  result: GroupSuccessRatesResult
): number | null | undefined {
  if (result.status !== 'ready') return undefined
  return result.data[group] ?? null
}

export function GroupSuccessRateBadge(props: { successRate?: number | null }) {
  const { t } = useTranslation()
  if (props.successRate === undefined) return null

  if (props.successRate === null) {
    return createElement(
      Badge,
      { variant: 'secondary', className: 'max-w-full px-1.5 text-[10px]' },
      t('No requests')
    )
  }

  if (!Number.isFinite(props.successRate)) return null
  const rate = `${props.successRate.toLocaleString(undefined, {
    maximumFractionDigits: 2,
  })}%`
  return createElement(
    Badge,
    {
      variant: 'outline',
      'aria-label': `${t('Success rate')}: ${rate}`,
      className: 'max-w-full px-1.5 text-[10px] tabular-nums',
    },
    rate
  )
}
