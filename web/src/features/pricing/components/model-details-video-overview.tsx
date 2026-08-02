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
import {
  AudioLines,
  CheckCircle2,
  Clock3,
  Film,
  Gauge,
  Images,
  Scan,
  Timer,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { getVideoPerfMetrics } from '@/features/performance-metrics/api'
import {
  formatUptimePct,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'

import { formatVideoDuration, getVideoResolutions } from '../lib/video-model'
import type { PricingModel } from '../types'

function VideoOverviewMetric(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: React.ReactNode
  hint: string
  valueClassName?: string
}) {
  const Icon = props.icon
  return (
    <div className='bg-background flex min-w-0 flex-col gap-1 p-3'>
      <span className='text-muted-foreground inline-flex items-center gap-1.5 text-[10px] font-medium tracking-wider uppercase'>
        <Icon className='size-3' aria-hidden='true' />
        {props.label}
      </span>
      <span
        className={cn(
          'text-foreground font-mono text-lg font-semibold tabular-nums',
          props.valueClassName
        )}
      >
        {props.value}
      </span>
      <span className='text-muted-foreground/70 text-[11px]'>{props.hint}</span>
    </div>
  )
}

function VideoSpec(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
}) {
  const Icon = props.icon
  return (
    <div className='bg-muted/20 flex min-w-0 items-start gap-2.5 rounded-lg border p-3'>
      <Icon
        className='text-muted-foreground mt-0.5 size-4 shrink-0'
        aria-hidden='true'
      />
      <div className='min-w-0'>
        <div className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
          {props.label}
        </div>
        <div className='text-foreground mt-0.5 text-sm leading-snug font-semibold'>
          {props.value}
        </div>
      </div>
    </div>
  )
}

export function ModelDetailsVideoOverview(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const metricsQuery = useQuery({
    queryKey: ['video-perf-metrics', props.model.model_name, 24],
    queryFn: () => getVideoPerfMetrics(props.model.model_name, 24),
    staleTime: 60 * 1000,
  })
  const summary = metricsQuery.data?.data.summary
  const terminalCount =
    (summary?.success_count ?? 0) + (summary?.failure_count ?? 0)
  const successRate =
    terminalCount > 0 ? (summary?.success_rate ?? 0) : Number.NaN
  const resolutions = getVideoResolutions(props.model)
  const fps = props.model.video_pricing?.fps ?? 24
  const extraFrames = props.model.video_pricing?.extra_frames ?? 1

  return (
    <div className='space-y-4'>
      <div className='bg-border/60 grid gap-px overflow-hidden rounded-lg border sm:grid-cols-3'>
        <VideoOverviewMetric
          icon={CheckCircle2}
          label={t('Completed tasks')}
          value={(summary?.success_count ?? 0).toLocaleString()}
          hint={t('{{count}} submitted in the last 24 hours', {
            count: summary?.submitted_count ?? 0,
          })}
        />
        <VideoOverviewMetric
          icon={Timer}
          label={t('Average generation time')}
          value={formatVideoDuration(summary?.average_duration_seconds ?? 0)}
          hint={t('Measured from submission to successful completion')}
        />
        <VideoOverviewMetric
          icon={Gauge}
          label={t('Task success rate')}
          value={formatUptimePct(successRate)}
          hint={t('Calculated from completed and failed tasks')}
          valueClassName={getSuccessRateTextClass(successRate)}
        />
      </div>

      <section>
        <h2 className='text-muted-foreground mb-3 text-xs font-semibold tracking-wider uppercase'>
          {t('Video specifications')}
        </h2>
        <div className='grid gap-2 sm:grid-cols-2 @2xl/details:grid-cols-3'>
          <VideoSpec
            icon={Scan}
            label={t('Output resolution')}
            value={resolutions.length > 0 ? resolutions.join(' · ') : '—'}
          />
          <VideoSpec
            icon={Film}
            label={t('Frame rate')}
            value={t('{{fps}} FPS, plus {{count}} startup frame', {
              fps,
              count: extraFrames,
            })}
          />
          <VideoSpec
            icon={Clock3}
            label={t('Duration')}
            value={t('4–15 seconds, or smart duration')}
          />
          <VideoSpec
            icon={Images}
            label={t('Reference inputs')}
            value={t(
              'Text, image, video and audio; asset:// references supported'
            )}
          />
          <VideoSpec
            icon={AudioLines}
            label={t('Generated audio')}
            value={t('Optional synchronized sound and music')}
          />
          <VideoSpec
            icon={Film}
            label={t('Aspect ratios')}
            value={`16:9 · 4:3 · 1:1 · 3:4 · 9:16 · 21:9 · adaptive (${t('Automatic aspect ratio')})`}
          />
        </div>
      </section>
    </div>
  )
}
