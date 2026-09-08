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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'

import { formatQuoteAmount } from '../lib/quotation-format'
import type { QuotePriceDimension } from '../types'

type PriceDimensionTableProps = {
  dimensions: QuotePriceDimension[]
}

const sourceLabels: Record<QuotePriceDimension['sourceType'], string> = {
  fixed_token: 'Fixed token',
  request: 'Per request',
  dynamic: 'Dynamic pricing',
  task_usage: 'Task usage',
  video: 'Video pricing',
  grok: 'Media pricing',
}

const videoConditionPattern =
  /^(.*); fps ([^;]+); extra frames ([^;]+); Token = ceil\(width x height x \(fps x duration \+ extra frames\) \/ 1024\)$/

export function PriceDimensionTable({ dimensions }: PriceDimensionTableProps) {
  const { t } = useTranslation()
  const dimensionLabel = (dimension: QuotePriceDimension): string => {
    const withoutVideoSuffix = ' without video input'
    const withVideoSuffix = ' with video input'
    const outputSuffix = ' output'
    if (
      dimension.sourceType === 'video' &&
      dimension.label.endsWith(withoutVideoSuffix)
    ) {
      return t('{{resolution}} without video input', {
        resolution: dimension.label.slice(0, -withoutVideoSuffix.length),
      })
    }
    if (
      dimension.sourceType === 'video' &&
      dimension.label.endsWith(withVideoSuffix)
    ) {
      return t('{{resolution}} with video input', {
        resolution: dimension.label.slice(0, -withVideoSuffix.length),
      })
    }
    if (
      dimension.sourceType === 'grok' &&
      dimension.label.endsWith(outputSuffix)
    ) {
      return t('{{tier}} output', {
        tier: dimension.label.slice(0, -outputSuffix.length),
      })
    }
    return t(dimension.label)
  }
  const conditionLabel = (condition: string | null): string => {
    if (!condition) return '—'
    const videoCondition = condition.match(videoConditionPattern)
    if (!videoCondition) return condition
    return t(
      '{{resolution}}; fps {{fps}}; extra frames {{extraFrames}}; Token = ceil(width x height x (fps x duration + extra frames) / 1024)',
      {
        resolution: videoCondition[1],
        fps: videoCondition[2],
        extraFrames: videoCondition[3],
      }
    )
  }

  if (dimensions.length === 0) {
    return (
      <p className='text-muted-foreground py-3 text-sm'>
        {t('No price dimensions available')}
      </p>
    )
  }

  return (
    <div className='max-w-full overflow-x-auto rounded-lg border'>
      <table className='w-full min-w-[760px] border-collapse text-xs'>
        <thead className='bg-muted/60'>
          <tr>
            <th className='px-2 py-2 text-left font-medium'>
              {t('Dimension')}
            </th>
            <th className='px-2 py-2 text-left font-medium'>{t('Source')}</th>
            <th className='px-2 py-2 text-right font-medium'>
              {t('Catalog price')}
            </th>
            <th className='px-2 py-2 text-right font-medium'>
              {t('Basis price')}
            </th>
            <th className='px-2 py-2 text-right font-medium'>
              {t('Quote price')}
            </th>
            <th className='px-2 py-2 text-left font-medium'>{t('Currency')}</th>
            <th className='px-2 py-2 text-left font-medium'>{t('Unit')}</th>
            <th className='px-2 py-2 text-left font-medium'>
              {t('Condition')}
            </th>
            <th className='px-2 py-2 text-left font-medium'>{t('Status')}</th>
          </tr>
        </thead>
        <tbody className='divide-y'>
          {dimensions.map((dimension) => (
            <tr key={dimension.key} className='align-top'>
              <td className='px-2 py-2 font-medium'>
                {dimensionLabel(dimension)}
              </td>
              <td className='text-muted-foreground px-2 py-2'>
                {t(sourceLabels[dimension.sourceType])}
              </td>
              <td className='px-2 py-2 text-right font-mono tabular-nums'>
                {formatQuoteAmount(dimension.catalogAmount, dimension.currency)}
              </td>
              <td className='px-2 py-2 text-right font-mono tabular-nums'>
                {formatQuoteAmount(dimension.sourceAmount, dimension.currency)}
              </td>
              <td className='px-2 py-2 text-right font-mono font-semibold tabular-nums'>
                {formatQuoteAmount(dimension.quoteAmount, dimension.currency)}
              </td>
              <td className='px-2 py-2'>{dimension.currency}</td>
              <td className='px-2 py-2'>{t(dimension.unit)}</td>
              <td className='max-w-48 px-2 py-2 break-words'>
                {conditionLabel(dimension.condition)}
              </td>
              <td className='px-2 py-2'>
                <Badge
                  variant={
                    dimension.status === 'needs_confirmation'
                      ? 'warning'
                      : 'secondary'
                  }
                >
                  {dimension.status === 'needs_confirmation'
                    ? t('Needs confirmation')
                    : t('Ready')}
                </Badge>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
