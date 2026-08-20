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
import { Download, ImageIcon, TriangleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import { getGrokImagePreview } from '../../api'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'
import { getGrokImageBillingState } from '../../lib/grok-image-billing'
import { GrokImageBillingCard } from './grok-image-billing-card'
import { ImageDialog } from './image-dialog'

interface GrokImagePreviewCardProps {
  log: UsageLog
  quotaPerUnit: number
}

export function GrokImagePreviewCard(props: GrokImagePreviewCardProps) {
  const other = parseLogOther(props.log.other)
  if (other?.grok_image_preview_available !== true) return null
  return <GrokImagePreviewContent {...props} />
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

function GrokImagePreviewContent(props: GrokImagePreviewCardProps) {
  const { t } = useTranslation()
  const [selectedImageUrl, setSelectedImageUrl] = useState<string | null>(null)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const billingState = getGrokImageBillingState(props.log)
  const hasLookupParams =
    props.log.user_id > 0 && props.log.request_id.trim().length > 0
  const previewQuery = useQuery({
    queryKey: ['grok-image-preview', props.log.user_id, props.log.request_id],
    queryFn: () => getGrokImagePreview(props.log.user_id, props.log.request_id),
    enabled: hasLookupParams,
    retry: false,
    staleTime: 0,
    gcTime: 60_000,
  })

  useEffect(() => {
    setSelectedImageUrl(null)
    setSelectedIndex(0)
  }, [props.log.request_id, props.log.user_id])

  const responseSucceeded = previewQuery.data?.success === true
  const responseUrls = responseSucceeded
    ? previewQuery.data?.data?.urls
    : undefined
  const urls = (Array.isArray(responseUrls) ? responseUrls : [])
    .filter((url): url is string => typeof url === 'string' && url !== '')
    .slice(0, 4)
  const isExpiredResponse = previewQuery.data?.expired === true
  const showLoading = previewQuery.isLoading || previewQuery.isFetching
  const showUnavailable =
    !hasLookupParams ||
    previewQuery.isError ||
    (previewQuery.data?.success === false && !isExpiredResponse)
  const showExpired =
    !showLoading &&
    (isExpiredResponse || (!showUnavailable && urls.length === 0))
  const activeIndex = Math.min(selectedIndex, Math.max(0, urls.length - 1))
  const activeURL = urls[activeIndex]
  const isSelectedImageAvailable =
    selectedImageUrl !== null &&
    !showLoading &&
    !showExpired &&
    !showUnavailable &&
    urls.includes(selectedImageUrl)

  useEffect(() => {
    if (selectedImageUrl !== null && !isSelectedImageAvailable) {
      setSelectedImageUrl(null)
    }
  }, [isSelectedImageAvailable, selectedImageUrl])

  useEffect(() => {
    if (selectedIndex >= urls.length && urls.length > 0) setSelectedIndex(0)
  }, [selectedIndex, urls.length])

  const billing = billingState.kind === 'current' ? billingState.billing : null

  return (
    <section
      className='grid min-w-0 gap-3 lg:min-h-0 lg:grid-cols-[minmax(0,1.3fr)_minmax(320px,0.7fr)]'
      data-grok-image-layout
    >
      <div className='bg-muted/30 min-w-0 space-y-2 rounded-xl border p-2.5'>
        <div className='flex items-center gap-1.5 text-xs font-semibold'>
          <ImageIcon className='size-3.5' aria-hidden='true' />
          {t('Grok Image Preview')}
        </div>

        {showLoading ? (
          <div aria-busy='true'>
            <Skeleton className='aspect-video w-full rounded-xl' />
          </div>
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

        {activeURL && !showUnavailable && !showLoading ? (
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
              data-grok-image-main
            />
          </button>
        ) : null}

        {urls.length > 1 && !showUnavailable && !showLoading ? (
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

        {billing ? (
          <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
            <ParameterMetric label={t('Model ID')} value={billing.model} />
            <ParameterMetric
              label={t('Operation')}
              value={
                billing.operation === 'edit'
                  ? t('Image Editing')
                  : t('Image Generation')
              }
            />
            <ParameterMetric
              label={t('Resolution')}
              value={billing.resolution.toUpperCase()}
            />
            <ParameterMetric
              label={t('Aspect Ratio')}
              value={billing.aspect_ratio}
            />
            {billing.quality ? (
              <ParameterMetric
                label={t('Quality')}
                value={billing.quality.toUpperCase()}
              />
            ) : null}
            <ParameterMetric
              label={t('Actual Outputs')}
              value={String(billing.output_count)}
            />
          </div>
        ) : null}
      </div>

      <aside className='min-w-0 space-y-3'>
        <Alert>
          <TriangleAlert />
          <AlertDescription>
            {t(
              'Result links are temporarily provided by the upstream provider and may expire. Please download and save them securely as soon as possible.'
            )}
          </AlertDescription>
        </Alert>

        {activeURL && !showUnavailable && !showLoading ? (
          <Button
            className='w-full'
            render={
              <a
                href={activeURL}
                download
                target='_blank'
                rel='noopener noreferrer'
                referrerPolicy='no-referrer'
                data-grok-image-download
              />
            }
          >
            <Download data-icon='inline-start' />
            {t('Download')}
          </Button>
        ) : null}

        <GrokImageBillingCard
          log={props.log}
          quotaPerUnit={props.quotaPerUnit}
          showParameters={false}
        />
      </aside>

      {isSelectedImageAvailable && selectedImageUrl ? (
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
