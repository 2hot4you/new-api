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
import { CheckCircle2, CircleAlert, Clock3, Gauge, Timer } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  StaticDataTable,
  staticDataTableClassNames as tableStyles,
} from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { getVideoPerfMetrics } from '@/features/performance-metrics/api'
import {
  formatUptimePct,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { VideoPerformancePoint } from '@/features/performance-metrics/types'
import { cn } from '@/lib/utils'

import {
  formatVideoDuration,
  VIDEO_SLOW_TASK_SECONDS,
} from '../lib/video-model'
import type { PricingModel } from '../types'

function VideoStatCard(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: React.ReactNode
  hint: string
  valueClassName?: string
}) {
  const Icon = props.icon
  return (
    <div className='bg-background flex flex-col gap-1 rounded-lg border p-3'>
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

function VideoDurationTrend(props: { series: VideoPerformancePoint[] }) {
  const { t } = useTranslation()
  const values = props.series.filter(
    (point) => point.average_duration_seconds > 0
  )
  if (values.length === 0) {
    return (
      <div className='text-muted-foreground flex h-44 items-center justify-center rounded-lg border text-xs'>
        {t('No completed video tasks in this period.')}
      </div>
    )
  }
  const maxDuration = Math.max(
    VIDEO_SLOW_TASK_SECONDS,
    ...values.map((point) => point.average_duration_seconds)
  )

  return (
    <div className='overflow-x-auto rounded-lg border px-3 pt-4 pb-2'>
      <div className='flex h-40 min-w-[480px] items-end gap-2'>
        {values.map((point) => {
          const height = Math.max(
            8,
            Math.round((point.average_duration_seconds / maxDuration) * 116)
          )
          const isSlow =
            point.average_duration_seconds > VIDEO_SLOW_TASK_SECONDS
          const label = new Date(point.ts * 1000).toLocaleTimeString([], {
            hour: '2-digit',
            minute: '2-digit',
          })
          return (
            <div
              key={point.ts}
              className='flex min-w-9 flex-1 flex-col items-center justify-end gap-1'
              title={t('{{time}}: {{duration}}, {{count}} completed', {
                time: label,
                duration: formatVideoDuration(point.average_duration_seconds),
                count: point.success_count,
              })}
            >
              <span className='text-muted-foreground font-mono text-[9px] tabular-nums'>
                {formatVideoDuration(point.average_duration_seconds)}
              </span>
              <div
                className={cn(
                  'w-full max-w-10 rounded-t-sm',
                  isSlow ? 'bg-red-500' : 'bg-emerald-500'
                )}
                style={{ height }}
                aria-label={t('{{time}} average generation time {{duration}}', {
                  time: label,
                  duration: formatVideoDuration(point.average_duration_seconds),
                })}
              />
              <span className='text-muted-foreground text-[9px]'>{label}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

export function ModelDetailsVideoPerformance(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const metricsQuery = useQuery({
    queryKey: ['video-perf-metrics', props.model.model_name, 24],
    queryFn: () => getVideoPerfMetrics(props.model.model_name, 24),
    staleTime: 60 * 1000,
  })

  if (metricsQuery.isLoading) {
    return (
      <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
        {t('Loading video performance…')}
      </div>
    )
  }
  if (metricsQuery.isError || !metricsQuery.data?.data) {
    return (
      <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
        {t('Video performance data is temporarily unavailable.')}
      </div>
    )
  }

  const metrics = metricsQuery.data.data
  const summary = metrics.summary
  const terminalCount = summary.success_count + summary.failure_count
  const successRate = terminalCount > 0 ? summary.success_rate : Number.NaN

  return (
    <div className='flex flex-col gap-5'>
      <div className='grid grid-cols-1 gap-2 sm:grid-cols-2 @2xl/details:grid-cols-4'>
        <VideoStatCard
          icon={CheckCircle2}
          label={t('Completed tasks')}
          value={summary.success_count.toLocaleString()}
          hint={t('{{submitted}} submitted · {{pending}} processing', {
            submitted: summary.submitted_count,
            pending: summary.pending_count,
          })}
        />
        <VideoStatCard
          icon={Gauge}
          label={t('Task success rate')}
          value={formatUptimePct(successRate)}
          hint={t('{{success}} successful · {{failed}} failed', {
            success: summary.success_count,
            failed: summary.failure_count,
          })}
          valueClassName={getSuccessRateTextClass(successRate)}
        />
        <VideoStatCard
          icon={Timer}
          label={t('Average generation time')}
          value={formatVideoDuration(summary.average_duration_seconds)}
          hint={t('P50 {{duration}}', {
            duration: formatVideoDuration(summary.p50_duration_seconds),
          })}
        />
        <VideoStatCard
          icon={Clock3}
          label={t('P95 generation time')}
          value={formatVideoDuration(summary.p95_duration_seconds)}
          hint={t('{{count}} tasks exceeded 10 minutes', {
            count: summary.slow_task_count,
          })}
          valueClassName={
            summary.p95_duration_seconds > VIDEO_SLOW_TASK_SECONDS
              ? 'text-red-600 dark:text-red-400'
              : undefined
          }
        />
      </div>

      <section>
        <div className='mb-2 flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-foreground text-sm font-semibold'>
              {t('Generation time trend (last 24h)')}
            </h2>
            <p className='text-muted-foreground text-xs'>
              {t('Average time from task submission to successful completion')}
            </p>
          </div>
          <span className='text-muted-foreground inline-flex items-center gap-1 text-xs'>
            <CircleAlert className='size-3.5 text-red-500' aria-hidden='true' />
            {t('Only durations over 10 minutes are highlighted')}
          </span>
        </div>
        <VideoDurationTrend series={metrics.series} />
      </section>

      <section>
        <h2 className='text-foreground mb-2 text-sm font-semibold'>
          {t('Performance by group')}
        </h2>
        {metrics.groups.length === 0 ? (
          <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
            {t('No video task data is available for the last 24 hours.')}
          </div>
        ) : (
          <StaticDataTable
            className={tableStyles.sectionContainer}
            headerRowClassName={tableStyles.compactHeaderRow}
            data={metrics.groups}
            getRowKey={(group) => group.group}
            columns={[
              {
                id: 'group',
                header: t('Group'),
                className: tableStyles.compactHeaderCell,
                cellClassName: tableStyles.compactCell,
                cell: (group) => <GroupBadge group={group.group} size='sm' />,
              },
              {
                id: 'submitted',
                header: t('Submitted'),
                className: tableStyles.compactHeaderCellRight,
                cellClassName: tableStyles.compactNumericCell,
                cell: (group) => group.submitted_count.toLocaleString(),
              },
              {
                id: 'success-rate',
                header: t('Success rate'),
                className: tableStyles.compactHeaderCellRight,
                cellClassName: tableStyles.compactNumericCell,
                cell: (group) =>
                  formatUptimePct(
                    group.success_count + group.failure_count > 0
                      ? group.success_rate
                      : Number.NaN
                  ),
              },
              {
                id: 'average',
                header: t('Average duration'),
                className: tableStyles.compactHeaderCellRight,
                cellClassName: tableStyles.compactNumericCell,
                cell: (group) => (
                  <span
                    className={cn(
                      group.average_duration_seconds >
                        VIDEO_SLOW_TASK_SECONDS &&
                        'text-red-600 dark:text-red-400'
                    )}
                  >
                    {formatVideoDuration(group.average_duration_seconds)}
                  </span>
                ),
              },
              {
                id: 'p95',
                header: t('P95 duration'),
                className: tableStyles.compactHeaderCellRight,
                cellClassName: tableStyles.compactNumericCell,
                cell: (group) =>
                  formatVideoDuration(group.p95_duration_seconds),
              },
              {
                id: 'slow',
                header: t('Over 10 min'),
                className: tableStyles.compactHeaderCellRight,
                cellClassName: tableStyles.compactNumericCell,
                cell: (group) => group.slow_task_count.toLocaleString(),
              },
            ]}
          />
        )}
      </section>
    </div>
  )
}
