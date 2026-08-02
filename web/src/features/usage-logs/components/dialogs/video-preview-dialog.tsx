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
import { Film, MonitorPlay } from 'lucide-react'
import { type ReactNode, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { IconBadge } from '@/components/ui/icon-badge'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { TaskLog } from '../../types'

interface VideoPreviewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  log: TaskLog
}

function DetailItem({
  label,
  value,
  mono = false,
}: {
  label: string
  value: ReactNode
  mono?: boolean
}) {
  return (
    <div className='border-border/60 bg-background/70 rounded-lg border px-3 py-2.5 shadow-sm'>
      <div className='text-muted-foreground mb-1 text-[11px] font-medium tracking-wide uppercase'>
        {label}
      </div>
      <div className={mono ? 'font-mono text-xs break-all' : 'text-sm'}>
        {value}
      </div>
    </div>
  )
}

export function VideoPreviewDialog({
  open,
  onOpenChange,
  log,
}: VideoPreviewDialogProps) {
  const { t } = useTranslation()
  const [playbackFailed, setPlaybackFailed] = useState(false)
  const params = log.video_params

  const ratioParts = params?.ratio
    ?.split(':')
    .map((part) => Number.parseFloat(part.trim()))
  const ratioWidth = ratioParts?.[0] || params?.width || 16
  const ratioHeight = ratioParts?.[1] || params?.height || 9
  const ratioValue = ratioWidth / ratioHeight
  const isPortrait = ratioValue < 0.95
  const isSquare = ratioValue >= 0.95 && ratioValue <= 1.05
  const playerAspectRatio = `${ratioWidth} / ${ratioHeight}`
  let dialogMaxWidthClass = 'sm:max-w-6xl'
  let layoutGridClass = 'lg:grid-cols-[minmax(0,1.65fr)_minmax(310px,0.85fr)]'
  if (isPortrait) {
    dialogMaxWidthClass = 'sm:max-w-4xl'
    layoutGridClass = 'lg:grid-cols-[minmax(260px,0.8fr)_minmax(340px,1fr)]'
  } else if (isSquare) {
    dialogMaxWidthClass = 'sm:max-w-5xl'
    layoutGridClass = 'lg:grid-cols-[minmax(360px,1fr)_minmax(320px,0.8fr)]'
  }

  useEffect(() => {
    if (open) setPlaybackFailed(false)
  }, [open, log.result_url])

  const dimensions =
    params?.width && params?.height
      ? `${params.width.toLocaleString()} × ${params.height.toLocaleString()}`
      : '-'
  const generationSeconds =
    log.finish_time && log.submit_time
      ? Math.max(0, log.finish_time - log.submit_time)
      : undefined

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <>
          <IconBadge tone='chart-2' size='sm'>
            <MonitorPlay />
          </IconBadge>
          {t('Video Preview')}
        </>
      }
      description={t(
        'The video is streamed from the upstream provider and is not stored by Molii.'
      )}
      contentClassName={cn(
        'max-sm:w-[calc(100vw-1.5rem)]',
        dialogMaxWidthClass
      )}
      titleClassName='flex items-center gap-2'
      contentHeight={isPortrait ? 'min(76vh, 760px)' : 'min(72vh, 700px)'}
      bodyClassName='h-full'
    >
      <div className={cn('grid min-h-0 gap-4 lg:h-full', layoutGridClass)}>
        <section
          className={cn(
            'from-muted/80 via-background to-muted/30 relative flex min-h-[300px] items-center justify-center overflow-hidden rounded-2xl border bg-gradient-to-br p-3 sm:p-5 lg:min-h-0',
            isPortrait ? 'h-[min(62vh,600px)] lg:h-full' : 'lg:h-full'
          )}
        >
          <div className='bg-primary/10 absolute -top-24 -left-16 size-64 rounded-full blur-3xl' />
          <div className='bg-chart-2/10 absolute -right-20 -bottom-28 size-72 rounded-full blur-3xl' />

          <div
            className={cn(
              'relative flex max-h-full max-w-full items-center justify-center overflow-hidden rounded-xl bg-black shadow-[0_24px_70px_rgba(0,0,0,0.38)] ring-1 ring-black/10 dark:ring-white/15',
              isPortrait || isSquare ? 'h-full w-auto' : 'h-auto w-full'
            )}
            style={{ aspectRatio: playerAspectRatio }}
          >
            {log.result_url && !playbackFailed ? (
              <video
                key={log.result_url}
                src={log.result_url}
                controls
                playsInline
                preload='metadata'
                className='h-full w-full bg-black object-contain'
                onError={() => setPlaybackFailed(true)}
              >
                {t('Your browser does not support video playback.')}
              </video>
            ) : (
              <div className='flex max-w-sm flex-col items-center gap-3 px-6 text-center text-white/70'>
                <Film className='size-10 opacity-50' />
                <div>
                  <p className='text-sm font-medium text-white'>
                    {t('Video playback failed')}
                  </p>
                  <p className='mt-1 text-xs leading-relaxed'>
                    {t(
                      'The playback link may have expired. Refresh the task logs and try again.'
                    )}
                  </p>
                </div>
              </div>
            )}

            <div className='absolute top-2.5 left-2.5 flex items-center gap-1.5'>
              <span className='rounded-md border border-white/15 bg-black/55 px-2 py-1 font-mono text-[11px] text-white/90 shadow-sm backdrop-blur-md'>
                {params?.ratio || `${ratioWidth}:${ratioHeight}`}
              </span>
              {params?.resolution ? (
                <span className='rounded-md border border-white/15 bg-black/55 px-2 py-1 text-[11px] font-medium text-white/90 shadow-sm backdrop-blur-md'>
                  {params.resolution}
                </span>
              ) : null}
            </div>
          </div>
        </section>

        <aside className='border-border/60 bg-card/60 min-h-0 rounded-2xl border p-3 shadow-sm sm:p-4 lg:overflow-y-auto'>
          <div className='mb-3 flex flex-wrap gap-1.5'>
            <StatusBadge
              label={t('Generated')}
              variant='green'
              size='sm'
              copyable={false}
            />
            {params?.resolution && (
              <StatusBadge
                label={params.resolution}
                variant='blue'
                size='sm'
                copyable={false}
              />
            )}
            {params?.ratio && (
              <StatusBadge
                label={params.ratio}
                variant='violet'
                size='sm'
                copyable={false}
              />
            )}
            {params?.seconds ? (
              <StatusBadge
                label={`${params.seconds}s`}
                variant='green'
                size='sm'
                copyable={false}
              />
            ) : null}
          </div>

          <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2'>
            <div className='sm:col-span-2 lg:col-span-1 xl:col-span-2'>
              <DetailItem
                label={t('Task ID')}
                value={
                  <StatusBadge
                    label={log.task_id}
                    copyText={log.task_id}
                    variant='neutral'
                    size='sm'
                    className='max-w-full font-mono'
                  />
                }
              />
            </div>
            <DetailItem
              label={t('Resolution')}
              value={params?.resolution || '-'}
            />
            <DetailItem label={t('Dimensions')} value={dimensions} mono />
            <DetailItem
              label={t('Aspect Ratio')}
              value={params?.ratio || '-'}
            />
            <DetailItem
              label={t('Duration')}
              value={
                params?.seconds ? `${params.seconds} ${t('seconds')}` : '-'
              }
            />
            <DetailItem
              label={t('Frame Rate')}
              value={params?.fps ? `${params.fps} FPS` : '-'}
            />
            <DetailItem
              label={t('Reference Video')}
              value={t(params?.has_video ? 'Included' : 'Not included')}
            />
            {log.properties?.origin_model_name ? (
              <div className='sm:col-span-2 lg:col-span-1 xl:col-span-2'>
                <DetailItem
                  label={t('Model')}
                  value={log.properties.origin_model_name}
                  mono
                />
              </div>
            ) : null}
            {log.finish_time ? (
              <div className='sm:col-span-2 lg:col-span-1 xl:col-span-2'>
                <DetailItem
                  label={t('Generated At')}
                  value={formatTimestampToDate(log.finish_time, 'seconds')}
                  mono
                />
              </div>
            ) : null}
            {generationSeconds != null ? (
              <div className='sm:col-span-2 lg:col-span-1 xl:col-span-2'>
                <DetailItem
                  label={t('Generation Time')}
                  value={`${generationSeconds} ${t('seconds')}`}
                />
              </div>
            ) : null}
          </div>
        </aside>
      </div>
    </Dialog>
  )
}
