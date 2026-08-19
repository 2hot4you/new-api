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
import { ImageIcon, TriangleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'

import { getGrokImagePreview } from '../../api'
import type { UsageLog } from '../../data/schema'
import { parseLogOther } from '../../lib/format'
import { ImageDialog } from './image-dialog'

interface GrokImagePreviewCardProps {
  log: UsageLog
}

export function GrokImagePreviewCard(props: GrokImagePreviewCardProps) {
  const other = parseLogOther(props.log.other)
  if (other?.grok_image_preview_available !== true) return null

  return <GrokImagePreviewContent log={props.log} />
}

function GrokImagePreviewContent(props: GrokImagePreviewCardProps) {
  const { t } = useTranslation()
  const [selectedImageUrl, setSelectedImageUrl] = useState<string | null>(null)
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

  return (
    <section className='bg-muted/30 space-y-2.5 rounded-md border p-2.5 max-sm:p-2'>
      <div className='flex items-center gap-1.5 text-xs font-semibold'>
        <ImageIcon className='size-3.5' aria-hidden='true' />
        {t('Grok Image Preview')}
      </div>

      <Alert>
        <TriangleAlert />
        <AlertDescription>
          {t(
            'Result links are temporarily provided by the upstream provider and may expire. Please download and save them securely as soon as possible.'
          )}
        </AlertDescription>
      </Alert>

      {showLoading && (
        <div className='grid grid-cols-2 gap-2 sm:grid-cols-4' aria-busy='true'>
          <Skeleton className='aspect-square w-full rounded-md' />
          <Skeleton className='aspect-square w-full rounded-md' />
        </div>
      )}

      {showExpired && (
        <p className='text-muted-foreground text-xs'>
          {t('Image preview expired')}
        </p>
      )}

      {showUnavailable && (
        <p className='text-muted-foreground text-xs'>
          {t('Image preview is unavailable')}
        </p>
      )}

      {urls.length > 0 && !showUnavailable && !showLoading && (
        <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
          {urls.map((url, index) => (
            <button
              key={url}
              type='button'
              className='bg-background focus-visible:ring-ring overflow-hidden rounded-md border focus-visible:ring-2 focus-visible:outline-none'
              onClick={() => setSelectedImageUrl(url)}
              aria-label={t('Open generated image {{index}}', {
                index: index + 1,
              })}
            >
              <img
                src={url}
                alt={t('Generated image')}
                className='aspect-square w-full object-cover'
                loading='lazy'
                referrerPolicy='no-referrer'
              />
            </button>
          ))}
        </div>
      )}

      {isSelectedImageAvailable && selectedImageUrl && (
        <ImageDialog
          imageUrl={selectedImageUrl}
          open={selectedImageUrl !== null}
          onOpenChange={(open) => {
            if (!open) setSelectedImageUrl(null)
          }}
        />
      )}
    </section>
  )
}
