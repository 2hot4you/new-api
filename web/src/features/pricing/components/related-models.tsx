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
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { getLobeIcon } from '@/lib/lobe-icon'

import { getPricingModelDescription } from '../lib/model-description'
import { getRelatedModels } from '../lib/related-models'
import type { PricingModel } from '../types'

export function RelatedModels(props: {
  currentModel: PricingModel
  models: PricingModel[]
  onSelect: (modelName: string) => void
}) {
  const { t } = useTranslation()
  const related = getRelatedModels(props.currentModel, props.models)
  if (related.length === 0) return null

  return (
    <section className='scroll-mt-24 border-t pt-7' data-related-models='true'>
      <div className='mb-3'>
        <h2 className='text-lg font-semibold'>{t('Related Models')}</h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('More models from {{vendor}}', {
            vendor: props.currentModel.vendor_name || t('this vendor'),
          })}
        </p>
      </div>
      <div className='grid grid-cols-1 border-t border-l md:grid-cols-2 lg:grid-cols-3'>
        {related.map((model) => {
          const iconKey = model.icon || model.vendor_icon
          return (
            <button
              key={model.id ?? model.model_name}
              type='button'
              onClick={() => props.onSelect(model.model_name)}
              className='hover:bg-muted/30 group flex min-h-28 items-start gap-3 border-r border-b p-4 text-left transition-colors'
            >
              <span className='bg-muted/50 flex size-8 shrink-0 items-center justify-center rounded-full border'>
                {iconKey
                  ? getLobeIcon(iconKey, 22)
                  : model.model_name.charAt(0).toUpperCase()}
              </span>
              <span className='min-w-0 flex-1'>
                <span className='block truncate font-mono text-sm font-semibold'>
                  {model.model_name}
                </span>
                <span className='text-muted-foreground mt-1 line-clamp-2 block text-xs leading-5'>
                  {getPricingModelDescription(model, t) ||
                    t('No description available.')}
                </span>
                {model.release_date && (
                  <span className='text-muted-foreground/60 mt-1.5 block text-[10px]'>
                    {model.release_date}
                  </span>
                )}
              </span>
              <ArrowRight className='text-muted-foreground mt-0.5 size-4 shrink-0 transition-transform group-hover:translate-x-0.5' />
            </button>
          )
        })}
      </div>
    </section>
  )
}
