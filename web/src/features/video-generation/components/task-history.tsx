/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Eye, RotateCcw } from 'lucide-react'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTaskBillingCny } from '@/features/usage-logs/lib/task-billing'
import dayjs from '@/lib/dayjs'

import {
  taskBooleanLabel,
  taskDimensions,
  taskElapsedSeconds,
  taskMediaCounts,
  taskModel,
} from '../lib/task-display'
import type { VideoStudioTask } from '../types'

const PAGE_SIZES = [10, 20, 50, 100] as const

function statusVariant(status: string): 'green' | 'danger' | 'orange' {
  if (status === 'SUCCESS') return 'green'
  if (status === 'FAILURE') return 'danger'
  return 'orange'
}

export function VideoStudioTaskHistory(props: {
  items: VideoStudioTask[]
  total: number
  pageIndex: number
  pageSize: number
  loading: boolean
  onPageChange: (pageIndex: number) => void
  onPageSizeChange: (pageSize: number) => void
  onPreview: (task: VideoStudioTask) => void
  onReuse: (task: VideoStudioTask) => void
}) {
  const { t } = useTranslation()
  const pageCount = Math.max(1, Math.ceil(props.total / props.pageSize))

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Generation history')}</CardTitle>
        <CardDescription>
          {t(
            'Includes Seedance tasks submitted from this page and through the API.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <Table className='min-w-[1800px]'>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Submitted')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Model / Task ID')}</TableHead>
              <TableHead>{t('API Key / Group')}</TableHead>
              <TableHead className='min-w-64'>{t('Description')}</TableHead>
              <TableHead>{t('Images / Videos / Audio')}</TableHead>
              <TableHead>{t('Size')}</TableHead>
              <TableHead>{t('Resolution / Ratio')}</TableHead>
              <TableHead>{t('Duration / FPS')}</TableHead>
              <TableHead>{t('Audio / Watermark')}</TableHead>
              <TableHead>{t('Elapsed time')}</TableHead>
              <TableHead>{t('Cost')}</TableHead>
              <TableHead className='bg-card sticky right-0 text-right'>
                {t('Actions')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.loading && props.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={13} className='py-12 text-center'>
                  {t('Loading generation history...')}
                </TableCell>
              </TableRow>
            )}
            {!props.loading && props.items.length === 0 && (
              <TableRow>
                <TableCell colSpan={13} className='py-12 text-center'>
                  {t('No Seedance generations yet')}
                </TableCell>
              </TableRow>
            )}
            {props.items.map((item) => {
              const task = item.task
              const request = item.request
              const params = task.video_params
              const resolution =
                request?.resolution || params?.resolution || '—'
              const rawRatio = request?.ratio || params?.ratio || '—'
              const ratio = rawRatio === 'adaptive' ? t('Auto') : rawRatio
              const requestedDuration = request?.duration
              let duration: number | string | undefined
              if (requestedDuration === -1) {
                duration = params?.seconds
                  ? `${t('Auto')} · ${params.seconds}s`
                  : t('Auto')
              } else {
                duration = requestedDuration || params?.seconds
              }
              const elapsedSeconds = taskElapsedSeconds(item)
              return (
                <TableRow key={task.task_id}>
                  <TableCell>
                    {dayjs.unix(task.submit_time).format('YYYY-MM-DD HH:mm:ss')}
                  </TableCell>
                  <TableCell>
                    <StatusBadge variant={statusVariant(task.status)}>
                      {t(task.status)}
                    </StatusBadge>
                  </TableCell>
                  <TableCell>
                    <p className='font-medium'>{taskModel(item)}</p>
                    <p
                      data-table-text='secondary'
                      className='max-w-56 truncate font-mono'
                      title={task.task_id}
                    >
                      {task.task_id}
                    </p>
                  </TableCell>
                  <TableCell>
                    <p>{task.token_name || '—'}</p>
                    <p data-table-text='secondary'>{task.group || '—'}</p>
                  </TableCell>
                  <TableCell className='max-w-80 whitespace-normal'>
                    <p className='line-clamp-3' title={request?.prompt}>
                      {request?.prompt || '—'}
                    </p>
                  </TableCell>
                  <TableCell>{taskMediaCounts(item)}</TableCell>
                  <TableCell>{taskDimensions(item)}</TableCell>
                  <TableCell>{`${resolution} / ${ratio}`}</TableCell>
                  <TableCell>
                    {typeof duration === 'number'
                      ? `${duration}s`
                      : duration || '—'}
                    <p data-table-text='secondary'>
                      {params?.fps ? `${params.fps} FPS` : '—'}
                    </p>
                  </TableCell>
                  <TableCell>
                    {taskBooleanLabel(request?.generate_audio)} /{' '}
                    {taskBooleanLabel(request?.watermark)}
                  </TableCell>
                  <TableCell>
                    {elapsedSeconds === null ? '—' : `${elapsedSeconds}s`}
                  </TableCell>
                  <TableCell>
                    {task.billing
                      ? formatTaskBillingCny(task.billing.final_cost)
                      : t('Pending confirmation')}
                  </TableCell>
                  <TableCell className='bg-card sticky right-0 text-right group-hover:[background-color:color-mix(in_oklch,var(--muted)_50%,var(--background))]'>
                    <div className='flex justify-end gap-1'>
                      <Button
                        type='button'
                        size='sm'
                        variant='ghost'
                        aria-label={t('Preview video')}
                        onClick={() => props.onPreview(item)}
                      >
                        <Eye />
                      </Button>
                      <Button
                        type='button'
                        size='sm'
                        variant='ghost'
                        aria-label={t('Regenerate')}
                        disabled={!request}
                        onClick={() => props.onReuse(item)}
                      >
                        <RotateCcw />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>

        <div className='flex flex-wrap items-center justify-between gap-3 border-t pt-4'>
          <p className='text-muted-foreground text-sm'>
            {t('Total:')} {props.total.toLocaleString()}
          </p>
          <div className='flex flex-wrap items-center gap-3'>
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <span>{t('Rows per page')}</span>
              <Select
                value={String(props.pageSize)}
                onValueChange={(value) => props.onPageSizeChange(Number(value))}
              >
                <SelectTrigger
                  aria-label={t('Rows per page')}
                  className='w-[72px]'
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent side='top' alignItemWithTrigger={false}>
                  {PAGE_SIZES.map((size) => (
                    <SelectItem key={size} value={String(size)}>
                      {size}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <span className='text-sm tabular-nums'>
              {props.pageIndex + 1} / {pageCount}
            </span>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={props.pageIndex === 0}
              onClick={() => props.onPageChange(props.pageIndex - 1)}
            >
              {t('Previous')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={props.pageIndex + 1 >= pageCount}
              onClick={() => props.onPageChange(props.pageIndex + 1)}
            >
              {t('Next')}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
