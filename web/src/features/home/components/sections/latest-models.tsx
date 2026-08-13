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
import { ArrowUpRight, CalendarDays } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { getPricingModelDescription } from '@/features/pricing/lib/model-description'
import type { Modality, PricingModel } from '@/features/pricing/types'
import { getLobeIcon } from '@/lib/lobe-icon'

interface LatestModelsProps {
  models: PricingModel[]
}

const MODALITY_LABELS: Record<Modality, string> = {
  text: 'Text',
  image: 'Image',
  audio: 'Audio',
  video: 'Video',
  file: 'File',
}

export function LatestModels(props: LatestModelsProps) {
  const { t } = useTranslation()
  if (props.models.length === 0) return null

  return (
    <section className='border-border/50 border-t px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView className='mb-10 flex flex-col justify-between gap-4 md:flex-row md:items-end'>
          <div>
            <p className='text-muted-foreground mb-3 text-xs font-semibold tracking-[0.18em] uppercase'>
              {t('Model updates')}
            </p>
            <h2 className='text-3xl font-semibold tracking-tight md:text-5xl'>
              {t('Latest models on Molii')}
            </h2>
          </div>
          <a
            href='/pricing?sort=release_date'
            className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1 text-sm transition-colors'
          >
            {t('View all models')}
            <ArrowUpRight className='size-4' />
          </a>
        </AnimateInView>

        <div className='grid overflow-hidden rounded-3xl border md:grid-cols-3'>
          {props.models.map((model, index) => {
            const description =
              getPricingModelDescription(model, t) ||
              model.vendor_description ||
              t('Explore this model in the Molii model marketplace.')
            const modalities = [
              ...(model.input_modalities ?? []),
              ...(model.output_modalities ?? []),
            ].filter(
              (item, itemIndex, items) => items.indexOf(item) === itemIndex
            )

            return (
              <AnimateInView
                key={model.model_name}
                delay={index * 90}
                className='group bg-background hover:bg-muted/20 border-border/60 relative min-h-80 border-b p-7 transition-colors last:border-b-0 md:border-r md:border-b-0 md:last:border-r-0'
              >
                <a
                  href={`/pricing/${encodeURIComponent(model.model_name)}`}
                  className='absolute inset-0 z-10'
                  aria-label={t('View {{model}} details', {
                    model: model.model_name,
                  })}
                />
                <div className='flex items-start justify-between gap-4'>
                  <div className='flex items-center gap-3'>
                    <span className='border-border/60 bg-muted/30 flex size-11 items-center justify-center rounded-xl border'>
                      {getLobeIcon(model.icon || model.vendor_icon, 25)}
                    </span>
                    <div>
                      <div className='text-muted-foreground text-xs'>
                        {model.vendor_name || t('Molii model')}
                      </div>
                      <h3 className='mt-0.5 text-base font-semibold break-all'>
                        {model.model_name}
                      </h3>
                    </div>
                  </div>
                  <ArrowUpRight className='text-muted-foreground size-5 transition-transform group-hover:translate-x-0.5 group-hover:-translate-y-0.5' />
                </div>

                <p className='text-muted-foreground mt-7 line-clamp-4 text-sm leading-6'>
                  {description}
                </p>

                <div className='mt-7 flex flex-wrap gap-2'>
                  {modalities.map((modality) => (
                    <span
                      key={modality}
                      className='border-border/60 bg-muted/30 rounded-full border px-2.5 py-1 text-[11px]'
                    >
                      {t(MODALITY_LABELS[modality])}
                    </span>
                  ))}
                </div>

                <div className='text-muted-foreground mt-8 flex items-center gap-2 text-xs'>
                  <CalendarDays className='size-3.5' />
                  <time dateTime={model.release_date}>
                    {model.release_date}
                  </time>
                </div>
              </AnimateInView>
            )
          })}
        </div>
      </div>
    </section>
  )
}
