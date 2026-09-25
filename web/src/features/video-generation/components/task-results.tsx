/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Download, Film, Image, Music, RotateCcw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import dayjs from '@/lib/dayjs'

import type { VideoStudioTask } from '../types'

function durationLabel(start?: number, finish?: number): string {
  if (!start || !finish || finish < start) return '—'
  const seconds = finish - start
  return seconds >= 60
    ? `${Math.floor(seconds / 60)}m ${seconds % 60}s`
    : `${seconds}s`
}

function resultStatus(status: string): 'green' | 'danger' | 'orange' {
  if (status === 'SUCCESS') return 'green'
  if (status === 'FAILURE') return 'danger'
  return 'orange'
}

export function VideoStudioTaskResults(props: {
  tasks: VideoStudioTask[]
  loading: boolean
  onReuse: (task: VideoStudioTask) => void
}) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <Card>
        <CardContent className='text-muted-foreground py-12 text-center'>
          {t('Loading generation history...')}
        </CardContent>
      </Card>
    )
  }
  if (props.tasks.length === 0) {
    return (
      <Card>
        <CardContent className='text-muted-foreground py-12 text-center'>
          <Film className='mx-auto mb-3 size-8' />
          {t('No Seedance generations yet')}
        </CardContent>
      </Card>
    )
  }
  return (
    <div className='space-y-4'>
      {props.tasks.map((item) => {
        const task = item.task
        const params = task.video_params
        const downloadURL = task.result_url
          ? `${task.result_url}${task.result_url.includes('?') ? '&' : '?'}download=1`
          : ''
        return (
          <Card key={task.task_id}>
            <CardHeader>
              <div className='flex flex-wrap items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <CardTitle className='text-base break-all'>
                    {item.request?.model ||
                      task.properties?.origin_model_name ||
                      t('Seedance video')}
                  </CardTitle>
                  <CardDescription className='mt-1 break-all'>
                    {task.task_id} ·{' '}
                    {dayjs.unix(task.submit_time).format('YYYY-MM-DD HH:mm:ss')}
                  </CardDescription>
                </div>
                <StatusBadge variant={resultStatus(task.status)}>
                  {t(task.status)}
                </StatusBadge>
              </div>
            </CardHeader>
            <CardContent className='space-y-4'>
              {task.result_url && task.status === 'SUCCESS' && (
                <div className='overflow-hidden rounded-lg border bg-black'>
                  <video
                    src={task.result_url}
                    controls
                    preload='metadata'
                    className='max-h-[460px] w-full'
                  />
                </div>
              )}
              <div className='grid gap-2 text-sm sm:grid-cols-2 xl:grid-cols-4'>
                <div className='rounded-md border p-3'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Parameters')}
                  </p>
                  <p>
                    {item.request?.resolution || params?.resolution || '—'} ·{' '}
                    {item.request?.ratio || params?.ratio || '—'} ·{' '}
                    {item.request?.duration || params?.seconds || '—'}s
                  </p>
                </div>
                <div className='rounded-md border p-3'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Input media')}
                  </p>
                  <p className='flex gap-3'>
                    <span className='inline-flex items-center gap-1'>
                      <Image className='size-3.5' />
                      {params?.input_image_count ?? 0}
                    </span>
                    <span className='inline-flex items-center gap-1'>
                      <Film className='size-3.5' />
                      {params?.input_video_count ?? 0}
                    </span>
                    <span className='inline-flex items-center gap-1'>
                      <Music className='size-3.5' />
                      {params?.input_audio_count ?? 0}
                    </span>
                  </p>
                </div>
                <div className='rounded-md border p-3'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Elapsed time')}
                  </p>
                  <div className='space-y-0.5'>
                    <p>
                      {t('Queue time')}:{' '}
                      {durationLabel(task.submit_time, task.start_time)}
                    </p>
                    <p>
                      {t('Generation time')}:{' '}
                      {durationLabel(task.start_time, task.finish_time)}
                    </p>
                    <p>
                      {t('Total time')}:{' '}
                      {durationLabel(task.submit_time, task.finish_time)}
                    </p>
                  </div>
                </div>
                <div className='rounded-md border p-3'>
                  <p className='text-muted-foreground text-xs'>
                    {t('Billing')}
                  </p>
                  <p>
                    {task.billing?.state
                      ? t(task.billing.state)
                      : t('Pending confirmation')}
                    {task.billing?.final_cost
                      ? ` · ${task.billing.final_cost}`
                      : ''}
                  </p>
                </div>
              </div>
              {item.request?.prompt && (
                <div>
                  <p className='mb-1 text-sm font-medium'>{t('Prompt')}</p>
                  <p className='bg-muted/30 max-h-32 overflow-auto rounded-md border p-3 text-sm whitespace-pre-wrap'>
                    {item.request.prompt}
                  </p>
                </div>
              )}
              {item.unavailable_asset_ids &&
                item.unavailable_asset_ids.length > 0 && (
                  <p className='text-destructive text-sm'>
                    {t(
                      'Some original temporary assets expired and must be replaced before reuse.'
                    )}
                  </p>
                )}
              {task.fail_reason && (
                <p className='text-destructive rounded-md border border-current/20 p-3 text-sm'>
                  {task.fail_reason}
                </p>
              )}
              <div className='flex flex-wrap gap-2'>
                {item.request && (
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => props.onReuse(item)}
                  >
                    <RotateCcw />
                    {t('Reuse settings')}
                  </Button>
                )}
                {downloadURL && (
                  <Button
                    variant='outline'
                    render={<a href={downloadURL} download />}
                  >
                    <Download />
                    {t('Download video')}
                  </Button>
                )}
              </div>
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}
