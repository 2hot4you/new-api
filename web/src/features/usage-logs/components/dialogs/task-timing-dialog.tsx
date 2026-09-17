/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { Clock3 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { IconBadge } from '@/components/ui/icon-badge'
import { formatTimestampToDate } from '@/lib/format'

import type { TaskTimingInfo } from '../../types'

function TimingMetric(props: {
  label: string
  value?: number
  kind: 'timestamp' | 'duration'
  notRecordedLabel: string
}) {
  let displayValue = props.notRecordedLabel
  if (props.value != null) {
    displayValue =
      props.kind === 'timestamp'
        ? formatTimestampToDate(props.value, 'seconds')
        : `${Number.isInteger(props.value) ? props.value : props.value.toFixed(1)}s`
  }

  return (
    <div className='bg-background/70 min-w-0 rounded-lg border px-3 py-2.5'>
      <div className='text-muted-foreground text-[11px] font-medium tracking-wide uppercase'>
        {props.label}
      </div>
      <div className='mt-1 truncate font-mono text-xs' title={displayValue}>
        {displayValue}
      </div>
    </div>
  )
}

export function TaskTimingDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  timing: TaskTimingInfo
}) {
  const { t } = useTranslation()
  const notRecordedLabel = t('Not recorded')

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <>
          <IconBadge tone='chart-2' size='sm'>
            <Clock3 />
          </IconBadge>
          {t('Task Timing')}
        </>
      }
      description={t('Generation Records')}
      contentClassName='sm:max-w-3xl'
      contentHeight='auto'
      titleClassName='flex items-center gap-2'
      bodyClassName='space-y-5 py-4'
    >
      <section className='space-y-2'>
        <h3 className='text-sm font-semibold'>{t('Timeline')}</h3>
        <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
          <TimingMetric
            label={t('Platform Submitted')}
            value={props.timing.platform_submitted_at}
            kind='timestamp'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Upstream Submitted')}
            value={props.timing.upstream_submitted_at}
            kind='timestamp'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Upstream Started')}
            value={props.timing.upstream_started_at}
            kind='timestamp'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Platform First In Progress')}
            value={props.timing.platform_first_in_progress_at}
            kind='timestamp'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Upstream Finished')}
            value={props.timing.upstream_finished_at}
            kind='timestamp'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Platform Finish Observed')}
            value={props.timing.platform_finished_observed_at}
            kind='timestamp'
            notRecordedLabel={notRecordedLabel}
          />
        </div>
      </section>

      <section className='space-y-2'>
        <h3 className='text-sm font-semibold'>{t('Duration Breakdown')}</h3>
        <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
          <TimingMetric
            label={t('Submission Handoff')}
            value={props.timing.submission_seconds}
            kind='duration'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Upstream Queue')}
            value={props.timing.upstream_queue_seconds}
            kind='duration'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Upstream Generation')}
            value={props.timing.upstream_generation_seconds}
            kind='duration'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Start Detection Delay')}
            value={props.timing.start_detection_delay_seconds}
            kind='duration'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Completion Polling Delay')}
            value={props.timing.finish_detection_delay_seconds}
            kind='duration'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Upstream Total')}
            value={props.timing.upstream_total_seconds}
            kind='duration'
            notRecordedLabel={notRecordedLabel}
          />
          <TimingMetric
            label={t('Total Duration')}
            value={props.timing.platform_total_seconds}
            kind='duration'
            notRecordedLabel={notRecordedLabel}
          />
        </div>
      </section>
    </Dialog>
  )
}
