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
import { useQueries } from '@tanstack/react-query'
import { useMemo } from 'react'

import { getPerfMetricsSummary } from '@/features/performance-metrics/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import { DEFAULT_TOKEN_UNIT } from '../constants'
import type { PricingCurrencyMode } from '../lib/pricing-currency'
import type { PricingModel, TokenUnit } from '../types'
import { ModelCard } from './model-card'
import type { ModelPerfBadgeData } from './model-perf-badge'

export interface ModelCardGridProps {
  models: PricingModel[]
  onModelClick: (modelName: string) => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  selectedGroup?: string
  currencyMode?: PricingCurrencyMode
}

export function ModelCardGrid(props: ModelCardGridProps) {
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT

  const recentModelBatches = useMemo(() => {
    // The backend accepts at most 50 recent-model lookups per request, while
    // the directory can grow as the user scrolls through more cards.
    const names = [
      ...new Set(props.models.map((model) => model.model_name).filter(Boolean)),
    ]
    const batches: string[][] = []
    for (let index = 0; index < names.length; index += 50) {
      batches.push(names.slice(index, index + 50))
    }
    return batches
  }, [props.models])
  const perfSummaries = useQueries({
    queries: recentModelBatches.map((recentModels) => ({
      queryKey: ['perf-metrics-summary', 24, recentModels],
      queryFn: async () =>
        requireServerSuccess(await getPerfMetricsSummary(24, recentModels)),
      staleTime: 60 * 1000,
      retry: false as const,
    })),
    combine: (results) => results.map((result) => result.data?.data),
  })

  const perfMap = useMemo(() => {
    const map = new Map<string, ModelPerfBadgeData>()
    for (const summary of perfSummaries) {
      for (const model of summary?.models ?? []) {
        map.set(model.model_name, {
          ...map.get(model.model_name),
          ...model,
          window_start: summary?.window_start,
          window_end: summary?.window_end,
        })
      }
      for (const [name, series] of Object.entries(
        summary?.recent_success_series ?? {}
      )) {
        map.set(name, {
          avg_latency_ms: Number.NaN,
          avg_tps: Number.NaN,
          success_rate: Number.NaN,
          ...map.get(name),
          recent_success_series: series,
        })
      }
    }
    return map
  }, [perfSummaries])
  if (props.models.length === 0) {
    return null
  }

  return (
    <div>
      <div
        className='grid grid-cols-1 border-t border-l md:grid-cols-2 xl:grid-cols-3'
        data-model-directory-grid='true'
      >
        {props.models.map((model) => (
          <ModelCard
            key={model.id ?? model.model_name}
            model={model}
            tokenUnit={tokenUnit}
            priceRate={props.priceRate}
            usdExchangeRate={props.usdExchangeRate}
            showRechargePrice={props.showRechargePrice}
            selectedGroup={props.selectedGroup}
            currencyMode={props.currencyMode}
            perf={perfMap.get(model.model_name || '')}
            onClick={() => props.onModelClick(model.model_name || '')}
          />
        ))}
      </div>
    </div>
  )
}
