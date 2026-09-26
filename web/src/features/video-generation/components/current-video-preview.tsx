/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import {
  Download,
  Film,
  Image,
  LoaderCircle,
  Music,
  RotateCcw,
  Sparkles,
} from 'lucide-react'
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
import { Progress } from '@/components/ui/progress'
import dayjs from '@/lib/dayjs'

import {
  formatElapsedTime,
  taskElapsedSeconds,
  taskMediaCounts,
  taskModel,
  taskProgress,
} from '../lib/task-display'
import type { VideoStudioTask } from '../types'

function statusVariant(status: string): 'green' | 'danger' | 'orange' {
  if (status === 'SUCCESS') return 'green'
  if (status === 'FAILURE') return 'danger'
  return 'orange'
}

export function VideoStudioCurrentPreview(props: {
  task?: VideoStudioTask
  loading: boolean
  onReuse: (task: VideoStudioTask) => void
}) {
  const { t } = useTranslation()
  const item = props.task
  const task = item?.task
  const running = Boolean(task && !['SUCCESS', 'FAILURE'].includes(task.status))
  const progress = item ? taskProgress(item) : null
  const rawRatio = item?.request?.ratio || task?.video_params?.ratio || '—'
  const ratio = rawRatio === 'adaptive' ? t('Auto') : rawRatio
  let previewContent
  if (props.loading && !task) {
    previewContent = (
      <div className='text-muted-foreground flex flex-1 items-center justify-center gap-2'>
        <LoaderCircle className='size-5 animate-spin' />
        {t('Loading generation task...')}
      </div>
    )
  } else if (!task) {
    previewContent = (
      <div className='bg-muted/25 text-muted-foreground flex flex-1 flex-col items-center justify-center rounded-xl border border-dashed p-8 text-center'>
        <Film className='mb-3 size-10' />
        <p className='font-medium'>{t('No video selected')}</p>
        <p className='mt-1 max-w-md text-sm'>
          {t('Submit a task or choose Preview from generation history.')}
        </p>
      </div>
    )
  } else if (running) {
    previewContent = (
      <div
        data-testid='video-generation-progress'
        className='bg-muted/25 flex flex-1 animate-pulse flex-col items-center justify-center rounded-xl border p-8 text-center'
      >
        <div className='bg-primary/10 text-primary mb-5 rounded-full p-4'>
          <Sparkles className='size-8 animate-bounce' />
        </div>
        <h3 className='text-lg font-semibold'>{t('Generating video')}</h3>
        <p className='text-muted-foreground mt-2 max-w-md text-sm'>
          {t(
            'The task is being processed. The preview will appear automatically.'
          )}
        </p>
        <div className='mt-6 w-full max-w-sm'>
          <Progress
            value={progress}
            aria-label={t('Video generation progress')}
          />
          <div className='text-muted-foreground mt-2 flex justify-between text-xs'>
            <span>{t(task.status)}</span>
            <span>{task.progress || t('Processing')}</span>
          </div>
        </div>
      </div>
    )
  } else if (task.status === 'SUCCESS' && task.result_url) {
    previewContent = (
      <div className='flex min-h-[320px] flex-1 items-center justify-center overflow-hidden rounded-xl border bg-black'>
        <video
          src={task.result_url}
          controls
          preload='metadata'
          className='max-h-[680px] min-h-[320px] w-full object-contain'
        />
      </div>
    )
  } else {
    previewContent = (
      <div className='border-destructive/30 bg-destructive/5 text-destructive flex flex-1 flex-col items-center justify-center rounded-xl border p-8 text-center'>
        <Film className='mb-3 size-10' />
        <h3 className='text-lg font-semibold'>
          {t('Video generation failed')}
        </h3>
        <p className='mt-2 max-w-lg text-sm break-words'>
          {task.fail_reason || t('No failure reason was returned.')}
        </p>
      </div>
    )
  }

  return (
    <Card className='h-full min-h-[560px]'>
      <CardHeader>
        <CardTitle>
          <h2 className='flex items-center gap-2 text-base font-medium'>
            <Film className='size-5' />
            {t('Video preview')}
          </h2>
        </CardTitle>
        <CardDescription>
          {item
            ? `${taskModel(item)} · ${item.task.task_id}`
            : t('The latest submitted or selected task appears here.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex min-h-0 flex-1 flex-col gap-4'>
        {previewContent}

        {item && task && (
          <>
            <div className='grid gap-2 text-sm sm:grid-cols-2 2xl:grid-cols-4'>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('Status')}</p>
                <StatusBadge variant={statusVariant(task.status)}>
                  {t(task.status)}
                </StatusBadge>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>
                  {t('Parameters')}
                </p>
                <p>
                  {item.request?.resolution ||
                    task.video_params?.resolution ||
                    '—'}{' '}
                  · {ratio}
                </p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>
                  {t('Input media')}
                </p>
                <p
                  className='flex items-center gap-3'
                  aria-label={t('Images / Videos / Audio')}
                >
                  <Image className='size-3.5' />
                  <Film className='size-3.5' />
                  <Music className='size-3.5' />
                  {taskMediaCounts(item)}
                </p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>
                  {t('Total time')}
                </p>
                <p>{formatElapsedTime(taskElapsedSeconds(item))}</p>
              </div>
            </div>

            <div className='flex flex-wrap items-center justify-between gap-2 border-t pt-3'>
              <p className='text-muted-foreground text-xs'>
                <span className='font-mono'>{task.task_id}</span>
                <span>
                  {' '}
                  · {dayjs.unix(task.submit_time).format('YYYY-MM-DD HH:mm:ss')}
                </span>
              </p>
              <div className='flex flex-wrap gap-2'>
                {item.request && (
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => props.onReuse(item)}
                  >
                    <RotateCcw />
                    {t('Regenerate')}
                  </Button>
                )}
                {task.status === 'SUCCESS' && task.result_url && (
                  <Button
                    variant='outline'
                    render={
                      <a
                        href={`${task.result_url}${task.result_url.includes('?') ? '&' : '?'}download=1`}
                        download
                      />
                    }
                  >
                    <Download />
                    {t('Download video')}
                  </Button>
                )}
              </div>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  )
}
