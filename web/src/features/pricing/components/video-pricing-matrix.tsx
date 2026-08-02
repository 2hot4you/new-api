/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { formatVideoPrice, getVideoPriceTokenCount } from '../lib/video-pricing'
import type { TokenUnit, VideoPricing } from '../types'

export function VideoPricingMatrix(props: {
  pricing: VideoPricing
  compact?: boolean
  showFormula?: boolean
  tokenUnit?: TokenUnit
  className?: string
}) {
  const { t } = useTranslation()
  const compact = props.compact ?? false
  const showFormula = props.showFormula ?? !compact
  const tokenUnit = props.tokenUnit ?? 'M'
  const priceTokenCount = getVideoPriceTokenCount(tokenUnit)
  const priceTokenCountLabel = priceTokenCount.toLocaleString('en-US')
  const unsupported = props.pricing.unsupported_resolutions ?? []

  return (
    <div
      className={cn('space-y-2.5', props.className)}
      data-video-pricing-matrix='true'
    >
      <div className='overflow-hidden rounded-lg border'>
        <table className='w-full table-fixed border-collapse'>
          <caption className='sr-only'>{t('Video generation pricing')}</caption>
          <thead className='bg-muted/60'>
            <tr>
              <th
                scope='col'
                className={cn(
                  'text-muted-foreground w-[28%] px-2 text-left font-medium',
                  compact ? 'py-1.5 text-[10px]' : 'px-3 py-2.5 text-xs'
                )}
              >
                {t('Output resolution')}
              </th>
              <th
                scope='col'
                className={cn(
                  'text-muted-foreground w-[36%] px-2 text-right font-medium',
                  compact ? 'py-1.5 text-[10px]' : 'px-3 py-2.5 text-xs'
                )}
              >
                {t('Without video input')}
              </th>
              <th
                scope='col'
                className={cn(
                  'text-muted-foreground w-[36%] px-2 text-right font-medium',
                  compact ? 'py-1.5 text-[10px]' : 'px-3 py-2.5 text-xs'
                )}
              >
                {t('With video input')}
              </th>
            </tr>
          </thead>
          <tbody>
            {props.pricing.rows.map((row) => (
              <tr
                key={row.resolutions.join('/')}
                className='border-t last:border-b-0'
              >
                <th
                  scope='row'
                  className={cn(
                    'text-foreground px-2 text-left font-medium',
                    compact ? 'py-1.5 text-xs' : 'px-3 py-2.5 text-sm'
                  )}
                >
                  {row.resolutions.join(' / ')}
                </th>
                <td
                  className={cn(
                    'text-right font-mono font-semibold tabular-nums',
                    compact ? 'px-2 py-1.5 text-xs' : 'px-3 py-2.5 text-sm'
                  )}
                >
                  {formatVideoPrice(row.without_video, tokenUnit)}
                </td>
                <td
                  className={cn(
                    'text-right font-mono font-semibold tabular-nums',
                    compact ? 'px-2 py-1.5 text-xs' : 'px-3 py-2.5 text-sm'
                  )}
                >
                  {formatVideoPrice(row.with_video, tokenUnit)}
                </td>
              </tr>
            ))}
            {unsupported.map((resolution) => (
              <tr key={resolution} className='border-t last:border-b-0'>
                <th
                  scope='row'
                  className={cn(
                    'text-foreground px-2 text-left font-medium',
                    compact ? 'py-1.5 text-xs' : 'px-3 py-2.5 text-sm'
                  )}
                >
                  {resolution}
                </th>
                <td
                  colSpan={2}
                  className={cn(
                    'text-muted-foreground bg-muted/10 text-center font-medium',
                    compact ? 'px-2 py-1.5 text-xs' : 'px-3 py-2.5 text-sm'
                  )}
                >
                  {t('Not supported')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <div
          className={cn(
            'text-muted-foreground border-t bg-muted/20 text-right',
            compact ? 'px-2 py-1 text-[10px]' : 'px-3 py-2 text-xs'
          )}
        >
          {t('Online inference')} · ¥ / {priceTokenCountLabel} Token
        </div>
      </div>

      {showFormula && (
        <div className='border-border bg-muted/25 rounded-lg border p-3'>
          <div className='flex items-start gap-2'>
            <Info className='text-muted-foreground mt-0.5 size-4 shrink-0' />
            <div className='min-w-0 space-y-1.5'>
              <div className='text-foreground text-xs font-medium'>
                {t('Token Calculation')}
              </div>
              <code className='text-muted-foreground block font-mono text-xs leading-relaxed break-words'>
                Token = {t('Round up')}［{t('output width')} ×{' '}
                {t('output height')} × ({props.pricing.fps} ×{' '}
                {t('duration in seconds')} + {props.pricing.extra_frames}) /
                1024］
              </code>
              <code className='text-muted-foreground block font-mono text-xs leading-relaxed break-words'>
                {t('Price')} = {t('actual tokens')} × {t('tier price')} /
                {priceTokenCountLabel}
              </code>
              <p className='text-muted-foreground text-xs leading-relaxed'>
                {t(
                  'Seedance generates one extra frame for playback; final billing uses the actual token count.'
                )}
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
