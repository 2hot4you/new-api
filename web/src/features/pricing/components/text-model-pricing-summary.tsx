/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useTranslation } from 'react-i18next'

import type { TextModelCardPricing } from '../lib/dynamic-price'

export function TextModelPricingSummary(props: {
  pricing: TextModelCardPricing
}) {
  const { t } = useTranslation()
  const { pricing } = props

  return (
    <div className='mt-3' data-text-model-billing='true'>
      <p className='text-muted-foreground text-xs font-medium'>
        {t(pricing.explanationKey)}
        <span className='text-muted-foreground/70'>
          {' · '}
          {t('Prices shown per {{unit}} Tokens', { unit: pricing.unitLabel })}
        </span>
      </p>

      {pricing.kind === 'tiered' && pricing.rows.length > 0 && (
        <div
          className='mt-2 overflow-hidden rounded-lg border'
          data-text-model-pricing-matrix='true'
        >
          <table className='w-full table-fixed border-collapse'>
            <caption className='sr-only'>{t('Tiered Token pricing')}</caption>
            <thead className='bg-muted/60'>
              <tr>
                <th
                  scope='col'
                  className='text-muted-foreground w-[31%] px-2 py-1.5 text-left text-[10px] font-medium'
                >
                  {t('Input length')}
                </th>
                <th
                  scope='col'
                  className='text-muted-foreground w-[23%] px-2 py-1.5 text-right text-[10px] font-medium'
                >
                  {t('Input')}
                </th>
                <th
                  scope='col'
                  className='text-muted-foreground w-[23%] px-2 py-1.5 text-right text-[10px] font-medium'
                >
                  {t('Output')}
                </th>
                <th
                  scope='col'
                  className='text-muted-foreground w-[23%] px-2 py-1.5 text-right text-[10px] font-medium'
                >
                  {t('Cached')}
                </th>
              </tr>
            </thead>
            <tbody>
              {pricing.rows.map((row) => (
                <tr key={row.label} className='border-t last:border-b-0'>
                  <th
                    scope='row'
                    className='text-foreground px-2 py-1.5 text-left text-xs font-medium whitespace-nowrap'
                  >
                    {row.label}
                  </th>
                  {[
                    ['input', row.input],
                    ['output', row.output],
                    ['cache', row.cache],
                  ].map(([field, price]) => (
                    <td
                      key={`${row.label}-${field}`}
                      className='px-2 py-1.5 text-right font-mono text-[11px] font-semibold whitespace-nowrap tabular-nums'
                    >
                      {price}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
