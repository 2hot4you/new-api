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
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'

import type { HomeVendor } from '../../lib/home-model-catalog'
import { VendorMarquee } from '../vendor-marquee'

interface ModelEcosystemProps {
  vendors: HomeVendor[]
  isLoading?: boolean
}

export function ModelEcosystem(props: ModelEcosystemProps) {
  const { t } = useTranslation()

  return (
    <section className='overflow-hidden py-24 md:py-32'>
      <AnimateInView className='mx-auto mb-10 max-w-7xl px-6 text-center'>
        <p className='text-muted-foreground mb-3 text-xs font-semibold tracking-[0.18em] uppercase'>
          {t('Model ecosystem')}
        </p>
        <h2 className='text-3xl font-semibold tracking-tight md:text-5xl'>
          {t('One platform, leading AI providers')}
        </h2>
        <p className='text-muted-foreground mx-auto mt-4 max-w-2xl text-sm leading-6 md:text-base'>
          {t(
            'The provider wall is generated from models currently available in the Molii model marketplace.'
          )}
        </p>
      </AnimateInView>
      <VendorMarquee vendors={props.vendors} isLoading={props.isLoading} />
    </section>
  )
}
