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
import type { ColumnDef } from '@tanstack/react-table'
import { CirclePlay, Music } from 'lucide-react'
/* eslint-disable react-refresh/only-export-components */
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { TASK_PLATFORMS, TASK_STATUS } from '../../constants'
import {
  taskActionMapper,
  taskPlatformMapper,
  taskStatusMapper,
} from '../../lib/mappers'
import {
  canPreviewVideoTask,
  isGeneratedVideoTask,
} from '../../lib/task-video-preview'
import type { TaskLog } from '../../types'
import {
  AudioPreviewDialog,
  type AudioClip,
} from '../dialogs/audio-preview-dialog'
import { FailReasonDialog } from '../dialogs/fail-reason-dialog'
import { VideoPreviewDialog } from '../dialogs/video-preview-dialog'
import { useUsageLogsContext } from '../usage-logs-provider'
import { createDurationColumn, createChannelColumn } from './column-helpers'

function parseTaskData(data: unknown): unknown[] {
  if (Array.isArray(data)) return data
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }
  return []
}

function AudioPreviewCell({ log }: { log: TaskLog }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const clips = useMemo(() => {
    const data = parseTaskData(log.data)
    return data.filter(
      (c) =>
        c && typeof c === 'object' && (c as Record<string, unknown>).audio_url
    )
  }, [log.data])

  if (clips.length === 0) return null

  return (
    <>
      <button
        type='button'
        className='group flex items-center gap-1 text-left text-xs'
        onClick={() => setOpen(true)}
      >
        <Music className='text-muted-foreground size-3' />
        <span className='text-foreground leading-snug group-hover:underline'>
          {t('Click to preview audio')}
        </span>
      </button>
      <AudioPreviewDialog
        open={open}
        onOpenChange={setOpen}
        clips={clips as AudioClip[]}
      />
    </>
  )
}

function TaskProgressCell({ log }: { log: TaskLog }) {
  const { t } = useTranslation()
  const [previewOpen, setPreviewOpen] = useState(false)
  const isGeneratedVideo = isGeneratedVideoTask(log)
  const canPreview = canPreviewVideoTask(log)

  if (isGeneratedVideo) {
    return (
      <>
        <div className='flex flex-wrap items-center gap-1.5'>
          <StatusBadge
            label={t('Generated')}
            variant='green'
            size='sm'
            copyable={false}
          />
          {canPreview ? (
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='h-7 gap-1.5 px-2 text-xs'
              onClick={(event) => {
                event.stopPropagation()
                setPreviewOpen(true)
              }}
            >
              <CirclePlay className='size-3.5' />
              {t('Preview')}
            </Button>
          ) : null}
        </div>
        {canPreview ? (
          <VideoPreviewDialog
            open={previewOpen}
            onOpenChange={setPreviewOpen}
            log={log}
          />
        ) : null}
      </>
    )
  }

  if (!log.progress) {
    return <span className='text-muted-foreground/60 text-xs'>-</span>
  }
  return (
    <span className='border-border/60 bg-muted/30 inline-flex items-center rounded-md border px-1.5 py-0.5 font-mono text-xs'>
      {log.progress}
    </span>
  )
}

export function useTaskLogsColumns(isAdmin: boolean): ColumnDef<TaskLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<TaskLog>[] = [
    {
      accessorKey: 'submit_time',
      header: t('Submit Time'),
      cell: ({ row }) => {
        const log = row.original
        const submitTime = row.getValue('submit_time') as number

        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            <span className='truncate font-mono text-xs tabular-nums'>
              {formatTimestampToDate(submitTime, 'seconds')}
            </span>
            {log.finish_time ? (
              <span className='text-muted-foreground/60 truncate font-mono text-[11px] tabular-nums'>
                {formatTimestampToDate(log.finish_time, 'seconds')}
              </span>
            ) : (
              <span className='text-muted-foreground/50 text-[11px]'>-</span>
            )}
          </div>
        )
      },
      size: 180,
    },
  ]

  if (isAdmin) {
    columns.push(createChannelColumn<TaskLog>({ headerLabel: t('Channel') }), {
      id: 'user',
      header: t('User'),
      accessorFn: (row) => row.username || row.user_id,
      cell: function UserCell({ row }) {
        const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
          useUsageLogsContext()
        const log = row.original
        const displayName = log.username || String(log.user_id || '?')

        return (
          <button
            type='button'
            className='flex items-center gap-1.5 text-left'
            onClick={(e) => {
              e.stopPropagation()
              setSelectedUserId(log.user_id)
              setUserInfoDialogOpen(true)
            }}
          >
            <Avatar className='ring-border/60 size-6 ring-1 max-sm:hidden'>
              <AvatarFallback
                className={cn(
                  'text-[11px] font-semibold',
                  !sensitiveVisible && 'bg-muted text-muted-foreground'
                )}
                style={
                  sensitiveVisible ? getUserAvatarStyle(displayName) : undefined
                }
              >
                {sensitiveVisible ? getUserAvatarFallback(displayName) : '•'}
              </AvatarFallback>
            </Avatar>
            <span className='text-muted-foreground truncate text-sm hover:underline'>
              {sensitiveVisible ? displayName : '••••'}
            </span>
          </button>
        )
      },
    })
  }

  columns.push(
    {
      accessorKey: 'task_id',
      header: t('Task ID'),
      cell: ({ row }) => {
        const log = row.original
        const taskId = row.getValue('task_id') as string
        if (!taskId) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }
        return (
          <div className='flex max-w-[170px] flex-col gap-0.5'>
            <StatusBadge
              label={taskId}
              copyText={taskId}
              variant='neutral'
              size='sm'
              className='border-border/60 bg-muted/30 !text-foreground max-w-full truncate rounded-md border px-1.5 py-0.5 font-mono'
            />
            <span className='text-muted-foreground/60 truncate text-[11px]'>
              {t(taskPlatformMapper.getLabel(log.platform, log.platform))} ·{' '}
              {t(taskActionMapper.getLabel(log.action))}
            </span>
          </div>
        )
      },
      meta: { mobileTitle: true },
    },
    createDurationColumn<TaskLog>({
      submitTimeKey: 'submit_time',
      finishTimeKey: 'finish_time',
      unit: 'seconds',
      headerLabel: t('Duration'),
      warningThresholdSec: 300,
      getWarningThresholdSec: (log) =>
        log.platform === TASK_PLATFORMS.STARAI ? 10 * 60 : 300,
    }),
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return (
          <StatusBadge
            label={t(taskStatusMapper.getLabel(status, status || 'Submitting'))}
            variant={taskStatusMapper.getVariant(status)}
            size='sm'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
    },
    {
      accessorKey: 'progress',
      header: t('Progress'),
      cell: ({ row }) => <TaskProgressCell log={row.original} />,
    },
    {
      accessorKey: 'fail_reason',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const log = row.original
        const failReason = row.getValue('fail_reason') as string
        const status = log.status
        const [dialogOpen, setDialogOpen] = useState(false)
        const videoParams = log.video_params

        const isSunoSuccess =
          log.platform === 'suno' && status === TASK_STATUS.SUCCESS
        if (isSunoSuccess) {
          const data = parseTaskData(log.data)
          if (
            data.some(
              (c) =>
                c &&
                typeof c === 'object' &&
                (c as Record<string, unknown>).audio_url
            )
          ) {
            return <AudioPreviewCell log={log} />
          }
        }

        if (!failReason && !videoParams) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }

        return (
          <div className='flex max-w-[280px] flex-col gap-1.5'>
            {videoParams && (
              <div className='flex flex-wrap gap-1'>
                {videoParams.resolution && (
                  <StatusBadge
                    label={videoParams.resolution}
                    variant='blue'
                    size='sm'
                    copyable={false}
                    className='border-blue-200/70 bg-blue-100/80 !text-blue-700 dark:border-blue-800/70 dark:bg-blue-950/60 dark:!text-blue-300'
                  />
                )}
                {videoParams.ratio && (
                  <StatusBadge
                    label={videoParams.ratio}
                    variant='violet'
                    size='sm'
                    copyable={false}
                    className='border-violet-200/70 bg-violet-100/80 !text-violet-700 dark:border-violet-800/70 dark:bg-violet-950/60 dark:!text-violet-300'
                  />
                )}
                {videoParams.seconds != null && videoParams.seconds > 0 && (
                  <StatusBadge
                    label={`${videoParams.seconds}s`}
                    variant='green'
                    size='sm'
                    copyable={false}
                    className='border-emerald-200/70 bg-emerald-100/80 !text-emerald-700 dark:border-emerald-800/70 dark:bg-emerald-950/60 dark:!text-emerald-300'
                  />
                )}
                {videoParams.fps != null && videoParams.fps > 0 && (
                  <StatusBadge
                    label={`${videoParams.fps} FPS`}
                    variant='orange'
                    size='sm'
                    copyable={false}
                    className='border-orange-200/70 bg-orange-100/80 !text-orange-700 dark:border-orange-800/70 dark:bg-orange-950/60 dark:!text-orange-300'
                  />
                )}
                <StatusBadge
                  label={t(
                    videoParams.has_video
                      ? 'With reference video'
                      : 'Without reference video'
                  )}
                  variant='cyan'
                  size='sm'
                  copyable={false}
                  className='border-cyan-200/70 bg-cyan-100/80 !text-cyan-700 dark:border-cyan-800/70 dark:bg-cyan-950/60 dark:!text-cyan-300'
                />
              </div>
            )}
            {failReason && (
              <>
                <button
                  type='button'
                  className='group flex max-w-[260px] items-center gap-1 text-left text-xs'
                  onClick={() => setDialogOpen(true)}
                  title={t('Click to view full error message')}
                >
                  <span className='truncate leading-snug text-red-600 group-hover:underline dark:text-red-400'>
                    {failReason}
                  </span>
                </button>
                <FailReasonDialog
                  failReason={failReason}
                  open={dialogOpen}
                  onOpenChange={setDialogOpen}
                />
              </>
            )}
          </div>
        )
      },
      size: 280,
      maxSize: 320,
    }
  )

  return columns
}
