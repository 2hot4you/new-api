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
import { CircleAlert, ShieldCheck } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { getSuccessRateTextClass } from '@/features/performance-metrics/lib/format'
import type { PerfGroupSummary } from '@/features/performance-metrics/types'
import { cn } from '@/lib/utils'

import type { RankingPeriod } from '../types'

const PERIOD_DESCRIPTIONS: Record<RankingPeriod, string> = {
  today: 'the last 24 hours',
  week: 'the past week',
  month: 'the past month',
  year: 'the past year',
}

type GroupSuccessSectionProps = {
  period: RankingPeriod
  groups: PerfGroupSummary[]
  isLoading?: boolean
  error?: unknown
}

function isMeasured(group: PerfGroupSummary): group is PerfGroupSummary & {
  success_rate: number
} {
  return group.request_count > 0 && Number.isFinite(group.success_rate)
}

function formatSuccessRate(rate: number): string {
  return `${rate.toLocaleString(undefined, {
    maximumFractionDigits: 2,
  })}%`
}

function formatRequestCount(count: number): string {
  return new Intl.NumberFormat().format(count)
}

export function GroupSuccessSection(props: GroupSuccessSectionProps) {
  const { t } = useTranslation()
  const periodDescription = t(PERIOD_DESCRIPTIONS[props.period])
  const groups = useMemo(() => {
    const measured: Array<PerfGroupSummary & { success_rate: number }> = []
    const withoutRequests: PerfGroupSummary[] = []

    for (const group of props.groups) {
      if (isMeasured(group)) {
        measured.push(group)
      } else {
        withoutRequests.push(group)
      }
    }

    measured.sort(
      (a, b) =>
        b.success_rate - a.success_rate || b.request_count - a.request_count
    )

    return [...measured, ...withoutRequests]
  }, [props.groups])

  const errorMessage =
    props.error instanceof Error
      ? props.error.message
      : t('Unable to load group success rates')

  return (
    <section
      aria-label={t('Group success rates for {{period}}', {
        period: periodDescription,
      })}
      className='bg-card overflow-hidden rounded-lg border'
    >
      <header className='border-b px-5 py-4'>
        <h2 className='text-foreground inline-flex items-center gap-2 text-base font-semibold'>
          <ShieldCheck className='text-primary size-4' />
          {t('Group success rates')}
        </h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Successful requests by configured group over {{period}}', {
            period: periodDescription,
          })}
        </p>
      </header>

      {props.isLoading ? (
        <div
          className='space-y-3 px-5 py-5'
          aria-label={t('Loading group success rates')}
        >
          {[0, 1, 2].map((row) => (
            <Skeleton key={row} className='h-10 w-full rounded-md' />
          ))}
        </div>
      ) : props.error ? (
        <div
          className='text-muted-foreground flex items-start gap-3 px-5 py-6 text-sm'
          role='alert'
        >
          <CircleAlert className='mt-0.5 size-4 shrink-0 text-amber-500' />
          <div>
            <p className='text-foreground font-medium'>
              {t('Unable to load group success rates')}
            </p>
            <p className='mt-1'>{errorMessage}</p>
          </div>
        </div>
      ) : groups.length === 0 ? (
        <div className='text-muted-foreground px-5 py-8 text-center text-sm'>
          {t('No configured groups')}
        </div>
      ) : (
        <ul aria-label={t('Group success rate ranking')} className='divide-y'>
          {groups.map((group) => {
            const measured = isMeasured(group)
            return (
              <li
                key={group.group}
                data-group-success-row={group.group}
                className='flex items-center gap-3 px-5 py-3'
              >
                <span className='text-foreground min-w-0 flex-1 truncate font-mono text-sm font-medium'>
                  {group.group}
                </span>
                {measured ? (
                  <div className='flex shrink-0 items-baseline gap-2 text-right sm:gap-3'>
                    <span
                      className={cn(
                        'font-mono text-sm font-semibold tabular-nums',
                        getSuccessRateTextClass(group.success_rate)
                      )}
                    >
                      {formatSuccessRate(group.success_rate)}
                    </span>
                    <span className='text-muted-foreground font-mono text-xs tabular-nums'>
                      {t('{{count}} requests', {
                        count: formatRequestCount(group.request_count),
                      })}
                    </span>
                  </div>
                ) : (
                  <span className='text-muted-foreground shrink-0 text-sm'>
                    {t('No requests')}
                  </span>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}
