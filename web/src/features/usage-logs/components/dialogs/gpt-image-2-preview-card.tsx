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
import { Download, ImageIcon, Info } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatUseTime } from '@/lib/format'

import { getGPTImage2Preview } from '../../api'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'
import { getGPTImage2LogState } from '../../lib/gpt-image-2'
import { ImageDialog } from './image-dialog'

interface GPTImage2PreviewCardProps {
  log: UsageLog
}

function ParameterMetric(props: { label: string; value: string }) {
  return (
    <div className='bg-background min-w-0 rounded-md border px-2.5 py-1.5'>
      <div className='text-muted-foreground truncate text-[11px]'>
        {props.label}
      </div>
      <div className='mt-0.5 truncate font-mono text-xs font-medium'>
        {props.value}
      </div>
    </div>
  )
}

export function GPTImage2PreviewCard(props: GPTImage2PreviewCardProps) {
  const { t } = useTranslation()
  const [selectedImageUrl, setSelectedImageUrl] = useState<string | null>(null)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const other = parseLogOther(props.log.other)
  const logState = getGPTImage2LogState(props.log)
  const snapshot = logState.kind === 'current' ? logState.snapshot : null
  const previewAvailable = other?.gpt_image_2_preview_available === true
  const hasLookupParams =
    previewAvailable &&
    props.log.user_id > 0 &&
    props.log.request_id.trim().length > 0
  const previewQuery = useQuery({
    queryKey: ['gpt-image-2-preview', props.log.user_id, props.log.request_id],
    queryFn: () => getGPTImage2Preview(props.log.user_id, props.log.request_id),
    enabled: hasLookupParams,
    retry: false,
    staleTime: 0,
    gcTime: 60_000,
  })

  useEffect(() => {
    setSelectedImageUrl(null)
    setSelectedIndex(0)
  }, [props.log.request_id, props.log.user_id])

  const responseUrls =
    previewQuery.data?.success === true
      ? previewQuery.data.data?.urls
      : undefined
  const urls = (Array.isArray(responseUrls) ? responseUrls : [])
    .filter((url): url is string => typeof url === 'string' && url !== '')
    .slice(0, 10)
  const showLoading = hasLookupParams && previewQuery.isLoading
  const showExpired =
    hasLookupParams &&
    !showLoading &&
    (previewQuery.data?.expired === true ||
      (previewQuery.data?.success === true && urls.length === 0))
  const showUnavailable =
    !hasLookupParams ||
    previewQuery.isError ||
    (previewQuery.data?.success === false && !previewQuery.data.expired)
  const activeIndex = Math.min(selectedIndex, Math.max(0, urls.length - 1))
  const activeURL = urls[activeIndex]

  useEffect(() => {
    if (selectedIndex >= urls.length && urls.length > 0) setSelectedIndex(0)
    if (selectedImageUrl && !urls.includes(selectedImageUrl)) {
      setSelectedImageUrl(null)
    }
  }, [selectedImageUrl, selectedIndex, urls])

  return (
    <section className='grid min-w-0 gap-3 lg:grid-cols-[minmax(0,1.3fr)_minmax(320px,0.7fr)]'>
      <div className='bg-muted/30 min-w-0 space-y-2.5 rounded-xl border p-2.5'>
        <div className='flex items-center gap-1.5 text-xs font-semibold'>
          <ImageIcon className='size-3.5' aria-hidden='true' />
          {t('GPT Image 2 Preview')}
        </div>

        {showLoading ? (
          <Skeleton className='aspect-video w-full rounded-xl' />
        ) : null}
        {showExpired ? (
          <p className='text-muted-foreground text-xs'>
            {t('Image preview expired')}
          </p>
        ) : null}
        {showUnavailable ? (
          <p className='text-muted-foreground text-xs'>
            {t('Image preview is unavailable')}
          </p>
        ) : null}

        {activeURL && !showLoading && !showUnavailable ? (
          <button
            type='button'
            className='bg-background focus-visible:ring-ring flex w-full items-center justify-center overflow-hidden rounded-xl border focus-visible:ring-2 focus-visible:outline-none'
            onClick={() => setSelectedImageUrl(activeURL)}
            aria-label={t('Open generated image {{index}}', {
              index: activeIndex + 1,
            })}
          >
            <img
              src={activeURL}
              alt={t('Generated image')}
              className='max-h-[52vh] w-full object-contain lg:h-[clamp(14rem,36dvh,22rem)] lg:max-h-none'
              loading='lazy'
              referrerPolicy='no-referrer'
            />
          </button>
        ) : null}

        {urls.length > 1 && !showLoading && !showUnavailable ? (
          <div className='flex gap-2 overflow-x-auto pb-1'>
            {urls.map((url, index) => (
              <button
                key={url}
                type='button'
                className='bg-background focus-visible:ring-ring size-16 shrink-0 overflow-hidden rounded-md border focus-visible:ring-2 focus-visible:outline-none'
                onClick={() => setSelectedIndex(index)}
                aria-label={t('Open generated image {{index}}', {
                  index: index + 1,
                })}
              >
                <img
                  src={url}
                  alt={t('Generated image')}
                  className='size-full object-cover'
                  loading='lazy'
                  referrerPolicy='no-referrer'
                />
              </button>
            ))}
          </div>
        ) : null}
      </div>

      <aside className='min-w-0 space-y-3'>
        <Alert>
          <Info />
          <AlertDescription>
            {t(
              'Molii temporarily retains this preview for 24 hours. Download it before it expires.'
            )}
          </AlertDescription>
        </Alert>

        {activeURL && !showLoading && !showUnavailable ? (
          <Button
            className='w-full'
            render={
              <a
                href={activeURL}
                download
                target='_blank'
                rel='noopener noreferrer'
                referrerPolicy='no-referrer'
              />
            }
          >
            <Download data-icon='inline-start' />
            {t('Download')}
          </Button>
        ) : null}

        {snapshot ? (
          <div className='bg-muted/30 grid grid-cols-2 gap-2 rounded-xl border p-2.5'>
            <ParameterMetric label={t('Model ID')} value={snapshot.model} />
            <ParameterMetric
              label={t('Operation')}
              value={
                snapshot.operation === 'edit'
                  ? t('Image Editing')
                  : t('Image Generation')
              }
            />
            <ParameterMetric label={t('Quality')} value={snapshot.quality} />
            <ParameterMetric
              label={t('Background')}
              value={snapshot.background}
            />
            <ParameterMetric
              label={t('Output Format')}
              value={snapshot.output_format.toUpperCase()}
            />
            <ParameterMetric
              label={t('Moderation')}
              value={snapshot.moderation}
            />
            <ParameterMetric label={t('Size')} value={snapshot.size} />
            <ParameterMetric
              label={t('User')}
              value={snapshot.user || t('Not set')}
            />
            <ParameterMetric
              label={t('Requested Outputs')}
              value={String(snapshot.requested_output_count)}
            />
            <ParameterMetric
              label={t('Actual Outputs')}
              value={String(snapshot.output_count)}
            />
            <ParameterMetric
              label={t('Total Duration')}
              value={formatUseTime(props.log.use_time)}
            />
          </div>
        ) : (
          <p className='text-muted-foreground rounded-xl border p-3 text-xs'>
            {t('Historical request parameters are unavailable')}
          </p>
        )}
      </aside>

      {selectedImageUrl && urls.includes(selectedImageUrl) ? (
        <ImageDialog
          imageUrl={selectedImageUrl}
          open
          onOpenChange={(open) => {
            if (!open) setSelectedImageUrl(null)
          }}
        />
      ) : null}
    </section>
  )
}
