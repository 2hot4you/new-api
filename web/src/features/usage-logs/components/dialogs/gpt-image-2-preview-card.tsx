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
import { Calculator, Download, ImageIcon, Info } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatUseTime } from '@/lib/format'
import { cn } from '@/lib/utils'

import { getGPTImage2Preview } from '../../api'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'
import { getGPTImage2LogState } from '../../lib/gpt-image-2'
import { buildGPTImage2Billing } from '../../lib/gpt-image-2-billing'
import { ImageDialog } from './image-dialog'

interface GPTImage2PreviewCardProps {
  log: UsageLog
  quotaPerUnit: number
}

type MediaOrientation = 'portrait' | 'landscape' | 'square' | 'unknown'

function getMediaOrientation(width: number, height: number): MediaOrientation {
  if (
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width <= 0 ||
    height <= 0
  ) {
    return 'unknown'
  }
  if (Math.abs(width - height) / Math.max(width, height) < 0.04) return 'square'
  return width > height ? 'landscape' : 'portrait'
}

function parseRequestedDimensions(size: string | undefined): {
  width: number
  height: number
} | null {
  const match = size?.trim().match(/^(\d+)x(\d+)$/i)
  if (!match) return null
  const width = Number(match[1])
  const height = Number(match[2])
  return width > 0 && height > 0 ? { width, height } : null
}

function bilingualLabel(
  english: string,
  translated: string,
  language: string
): string {
  if (language.toLowerCase().startsWith('zh') && translated !== english) {
    return `${english}（${translated}）`
  }
  return translated
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
  const { t, i18n } = useTranslation()
  const [selectedImageUrl, setSelectedImageUrl] = useState<string | null>(null)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [mediaDimensions, setMediaDimensions] = useState<{
    width: number
    height: number
  } | null>(null)
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
  const requestedDimensions = parseRequestedDimensions(snapshot?.size)
  const dimensions = mediaDimensions ?? requestedDimensions
  const mediaOrientation = dimensions
    ? getMediaOrientation(dimensions.width, dimensions.height)
    : 'unknown'
  const billing = buildGPTImage2Billing({
    promptTokens: props.log.prompt_tokens,
    completionTokens: props.log.completion_tokens,
    imageTokens: other?.image_output,
    quota: props.log.quota,
    quotaPerUnit: props.quotaPerUnit,
    modelRatio: other?.model_ratio,
    completionRatio: other?.completion_ratio,
    imageRatio: other?.image_ratio,
    groupRatio: other?.group_ratio,
    userGroupRatio: other?.user_group_ratio,
  })
  const label = (key: string) =>
    bilingualLabel(key, t(key), i18n.resolvedLanguage ?? i18n.language)
  const formatPrice = (usd: number) =>
    formatBillingCurrencyFromUSD(usd, {
      digitsLarge: 4,
      digitsSmall: 6,
      abbreviate: false,
    })
  const downloadURL = activeURL
    ? `/api/log/gpt-image-2-preview/${encodeURIComponent(props.log.user_id)}/${encodeURIComponent(props.log.request_id)}/download/${activeIndex}`
    : null

  useEffect(() => {
    if (selectedIndex >= urls.length && urls.length > 0) setSelectedIndex(0)
    if (selectedImageUrl && !urls.includes(selectedImageUrl)) {
      setSelectedImageUrl(null)
    }
  }, [selectedImageUrl, selectedIndex, urls])

  useEffect(() => {
    setMediaDimensions(null)
  }, [activeURL])

  return (
    <section
      className={cn(
        'grid min-w-0 gap-3',
        mediaOrientation === 'portrait'
          ? 'lg:grid-cols-[minmax(280px,0.8fr)_minmax(380px,1.2fr)]'
          : 'lg:grid-cols-[minmax(0,1.4fr)_minmax(360px,0.6fr)]'
      )}
      data-gpt-image-2-layout
      data-media-orientation={mediaOrientation}
    >
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
              className={cn(
                'h-auto max-h-[58vh] max-w-full object-contain',
                mediaOrientation === 'portrait' ? 'w-auto' : 'w-full'
              )}
              style={
                dimensions
                  ? {
                      aspectRatio: `${dimensions.width} / ${dimensions.height}`,
                    }
                  : undefined
              }
              loading='lazy'
              referrerPolicy='no-referrer'
              data-gpt-image-2-main
              onLoad={(event) => {
                const image = event.currentTarget
                if (image.naturalWidth > 0 && image.naturalHeight > 0) {
                  setMediaDimensions({
                    width: image.naturalWidth,
                    height: image.naturalHeight,
                  })
                }
              }}
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

        {downloadURL && !showLoading && !showUnavailable ? (
          <Button
            className='w-full'
            render={<a href={downloadURL} download data-gpt-image-2-download />}
          >
            <Download data-icon='inline-start' />
            {t('Download')}
          </Button>
        ) : null}

        {snapshot ? (
          <div className='bg-muted/30 grid grid-cols-2 gap-2 rounded-xl border p-2.5'>
            <ParameterMetric label={label('Model ID')} value={snapshot.model} />
            <ParameterMetric
              label={label('Operation')}
              value={
                snapshot.operation === 'edit'
                  ? t('Image Editing')
                  : t('Image Generation')
              }
            />
            <ParameterMetric
              label={label('Quality')}
              value={snapshot.quality}
            />
            <ParameterMetric
              label={label('Background')}
              value={snapshot.background}
            />
            <ParameterMetric
              label={label('Output Format')}
              value={snapshot.output_format.toUpperCase()}
            />
            <ParameterMetric
              label={label('Moderation')}
              value={snapshot.moderation}
            />
            <ParameterMetric label={label('Size')} value={snapshot.size} />
            <ParameterMetric
              label={label('User')}
              value={snapshot.user || t('Not set')}
            />
            <ParameterMetric
              label={label('Requested Outputs')}
              value={String(snapshot.requested_output_count)}
            />
            <ParameterMetric
              label={label('Actual Outputs')}
              value={String(snapshot.output_count)}
            />
            <ParameterMetric
              label={label('Total Duration')}
              value={formatUseTime(props.log.use_time)}
            />
          </div>
        ) : (
          <p className='text-muted-foreground rounded-xl border p-3 text-xs'>
            {t('Historical request parameters are unavailable')}
          </p>
        )}

        {billing ? (
          <div className='bg-muted/30 min-w-0 space-y-2 rounded-xl border p-2.5'>
            <div className='flex items-center gap-1.5 text-xs font-semibold'>
              <Calculator
                className='size-3.5 text-violet-500'
                aria-hidden='true'
              />
              {t('Billing Rules and Formula')}
            </div>
            <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
              <ParameterMetric
                label={t('Input Tokens')}
                value={billing.inputTokens.toLocaleString('en-US')}
              />
              <ParameterMetric
                label={t('Input Unit Price')}
                value={`${formatPrice(billing.inputUnitPriceUSD)} / 1M`}
              />
              <ParameterMetric
                label={t('Input Subtotal')}
                value={formatPrice(billing.inputCostUSD)}
              />
              {billing.imageInputTokens > 0 ? (
                <>
                  <ParameterMetric
                    label={t('Image Tokens')}
                    value={billing.imageInputTokens.toLocaleString('en-US')}
                  />
                  <ParameterMetric
                    label={t('Image Input Unit Price')}
                    value={`${formatPrice(billing.imageInputUnitPriceUSD)} / 1M`}
                  />
                  <ParameterMetric
                    label={t('Image Input Subtotal')}
                    value={formatPrice(billing.imageInputCostUSD)}
                  />
                </>
              ) : null}
              <ParameterMetric
                label={t('Output Tokens')}
                value={billing.outputTokens.toLocaleString('en-US')}
              />
              <ParameterMetric
                label={t('Output Unit Price')}
                value={`${formatPrice(billing.outputUnitPriceUSD)} / 1M`}
              />
              <ParameterMetric
                label={t('Output Subtotal')}
                value={formatPrice(billing.outputCostUSD)}
              />
              <ParameterMetric
                label={t('Subtotal')}
                value={formatPrice(billing.subtotalUSD)}
              />
              <ParameterMetric
                label={t('Group Ratio')}
                value={`${billing.groupRatio.toFixed(4)}x`}
              />
              <ParameterMetric
                label={t('Savings Compared with Base Price')}
                value={`${billing.savingsPercent.toFixed(2)}%`}
              />
            </div>
            <div className='space-y-1.5 rounded-md border border-violet-200 bg-violet-50/70 p-2 dark:border-violet-900 dark:bg-violet-950/20'>
              <div className='text-xs font-medium text-violet-700 dark:text-violet-300'>
                {t('Billing Formula')}
              </div>
              <p className='overflow-x-auto font-mono text-xs whitespace-nowrap'>
                ({billing.textInputTokens.toLocaleString('en-US')} ×{' '}
                {formatPrice(billing.inputUnitPriceUSD)} / 1M +{' '}
                {billing.imageInputTokens > 0 ? (
                  <>
                    {billing.imageInputTokens.toLocaleString('en-US')} ×{' '}
                    {formatPrice(billing.imageInputUnitPriceUSD)} / 1M +{' '}
                  </>
                ) : null}
                {billing.outputTokens.toLocaleString('en-US')} ×{' '}
                {formatPrice(billing.outputUnitPriceUSD)} / 1M) ×{' '}
                {billing.groupRatio.toFixed(4)} ={' '}
                {formatPrice(billing.finalCostUSD)}
              </p>
            </div>
            <div className='flex items-center justify-between gap-3 border-t pt-2 text-xs'>
              <span className='text-muted-foreground'>{t('Final Charge')}</span>
              <span className='font-mono font-semibold'>
                {formatPrice(billing.finalCostUSD)}
              </span>
            </div>
          </div>
        ) : (
          <p className='text-muted-foreground rounded-xl border p-3 text-xs'>
            {t('Historical billing breakdown unavailable')}
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
