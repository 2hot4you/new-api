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

import { cn } from '@/lib/utils'

export type ModelDetailAnchor = {
  id: 'pricing' | 'capabilities' | 'performance' | 'api'
  label: string
}

export function ModelDetailsAnchorNav(props: {
  items: ModelDetailAnchor[]
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <nav
      data-model-detail-anchor-nav='true'
      aria-label={t('Model detail sections')}
      className={cn(
        'bg-background/95 sticky top-14 z-20 -mx-4 overflow-x-auto border-y px-4 backdrop-blur supports-[backdrop-filter]:bg-background/80 sm:mx-0 sm:rounded-md sm:border-x',
        props.className
      )}
    >
      <div className='flex min-w-max items-center'>
        {props.items.map((item) => (
          <a
            key={item.id}
            href={`#${item.id}`}
            className='text-muted-foreground hover:text-foreground border-r px-4 py-2.5 text-xs font-medium transition-colors first:border-l'
          >
            {t(item.label)}
          </a>
        ))}
      </div>
    </nav>
  )
}
