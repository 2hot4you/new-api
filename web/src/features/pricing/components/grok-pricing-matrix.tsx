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

import { buildGrokPricingRows } from '../lib/grok-pricing-table'
import type { MoliiGrokPricing } from '../types'

function DirectPrice(props: { value?: number; unit: string }) {
  const { t } = useTranslation()
  if (props.value == null) {
    return <span className='text-muted-foreground/60'>—</span>
  }
  return (
    <span className='font-mono font-semibold whitespace-nowrap tabular-nums'>
      ¥{props.value} / {t(props.unit)}
    </span>
  )
}

export function GrokPricingMatrix(props: { pricing: MoliiGrokPricing }) {
  const { t } = useTranslation()
  const rows = buildGrokPricingRows(props.pricing)
  if (rows.length === 0) {
    return (
      <div className='text-muted-foreground mt-3 rounded-lg border px-3 py-4 text-center text-xs'>
        {t('Pricing is temporarily unavailable.')}
      </div>
    )
  }

  const imagePricing = props.pricing.kind === 'image'
  const hasVideoInput = rows.some((row) => row.videoInputPrice != null)
  const outputHeader = imagePricing ? t('Image output') : t('Video output')
  const inputHeader = hasVideoInput ? t('Media input') : t('Image input')
  const outputUnit = props.pricing.output_unit
  const imageInputUnit = props.pricing.image_input_unit ?? 'image'
  const videoInputUnit = props.pricing.video_input_unit ?? 'second'

  return (
    <div
      className='mt-3 overflow-hidden rounded-lg border'
      data-grok-pricing-matrix='true'
    >
      <table className='w-full table-fixed border-collapse'>
        <caption className='sr-only'>{t('Grok Imagine pricing')}</caption>
        <thead className='bg-muted/60'>
          <tr>
            <th
              scope='col'
              className='text-muted-foreground w-[28%] px-2 py-1.5 text-left text-[10px] font-medium'
            >
              {t('Output resolution')}
            </th>
            <th
              scope='col'
              className='text-muted-foreground w-[32%] px-2 py-1.5 text-right text-[10px] font-medium'
            >
              {outputHeader}
            </th>
            <th
              scope='col'
              className='text-muted-foreground w-[40%] px-2 py-1.5 text-right text-[10px] font-medium'
            >
              {inputHeader}
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.resolution} className='border-t last:border-b-0'>
              <th
                scope='row'
                className='text-foreground px-2 py-1.5 text-left text-xs font-medium'
              >
                {row.resolution}
              </th>
              <td className='px-2 py-1.5 text-right text-xs'>
                <DirectPrice value={row.outputPrice} unit={outputUnit} />
              </td>
              <td className='px-2 py-1.5 text-right text-[11px]'>
                {hasVideoInput ? (
                  <div className='space-y-0.5'>
                    <div className='flex items-center justify-end gap-1.5'>
                      <span className='text-muted-foreground'>
                        {t('Image input')}
                      </span>
                      <DirectPrice
                        value={row.imageInputPrice}
                        unit={imageInputUnit}
                      />
                    </div>
                    <div className='flex items-center justify-end gap-1.5'>
                      <span className='text-muted-foreground'>
                        {t('Video input')}
                      </span>
                      <DirectPrice
                        value={row.videoInputPrice}
                        unit={videoInputUnit}
                      />
                    </div>
                  </div>
                ) : (
                  <DirectPrice
                    value={row.imageInputPrice}
                    unit={imageInputUnit}
                  />
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
