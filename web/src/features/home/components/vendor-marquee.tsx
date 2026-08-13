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

import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import type { HomeVendor } from '../lib/home-model-catalog'

interface VendorMarqueeProps {
  vendors: HomeVendor[]
  isLoading?: boolean
}

function VendorChip(props: { vendor: HomeVendor; duplicate?: boolean }) {
  const { t } = useTranslation()
  const href = `/pricing?vendor=${encodeURIComponent(props.vendor.name)}`

  return (
    <a
      href={href}
      aria-hidden={props.duplicate || undefined}
      tabIndex={props.duplicate ? -1 : undefined}
      className='border-border/70 bg-background/85 hover:border-foreground/30 hover:bg-muted/45 flex h-16 min-w-56 shrink-0 items-center gap-3 rounded-2xl border px-4 shadow-[0_8px_24px_rgb(0_0_0/0.035)] backdrop-blur-sm transition-[border-color,background-color,transform] duration-200 hover:-translate-y-0.5'
    >
      <span className='border-border/50 bg-muted/30 flex size-10 shrink-0 items-center justify-center rounded-xl border'>
        {getLobeIcon(props.vendor.icon || props.vendor.name, 24)}
      </span>
      <span className='min-w-0'>
        <span className='block truncate text-sm font-semibold'>
          {props.vendor.name}
        </span>
        <span className='text-muted-foreground mt-0.5 block text-xs'>
          {t('{{count}} models', { count: props.vendor.modelCount })}
        </span>
      </span>
    </a>
  )
}

function MarqueeRow(props: {
  vendors: HomeVendor[]
  direction: 'forward' | 'reverse'
}) {
  const items = props.vendors.length > 0 ? props.vendors : []

  return (
    <div
      data-vendor-marquee-row
      data-direction={props.direction}
      className='home-marquee-row flex w-max gap-4 py-2'
    >
      <div
        className={cn(
          'home-marquee-track flex gap-4 pr-4',
          props.direction === 'forward'
            ? 'home-marquee-forward'
            : 'home-marquee-reverse'
        )}
      >
        {items.map((vendor) => (
          <VendorChip key={`primary-${vendor.name}`} vendor={vendor} />
        ))}
        {items.map((vendor) => (
          <VendorChip
            key={`duplicate-${vendor.name}`}
            vendor={vendor}
            duplicate
          />
        ))}
      </div>
    </div>
  )
}

export function VendorMarquee(props: VendorMarqueeProps) {
  const { t } = useTranslation()

  if (props.isLoading) {
    return (
      <div className='grid gap-3' aria-label={t('Loading model providers')}>
        {[0, 1].map((row) => (
          <div key={row} className='flex gap-4 overflow-hidden'>
            {[0, 1, 2, 3].map((item) => (
              <div
                key={item}
                className='bg-muted/40 h-16 min-w-56 animate-pulse rounded-2xl'
              />
            ))}
          </div>
        ))}
      </div>
    )
  }

  if (props.vendors.length === 0) return null

  const midpoint = Math.ceil(props.vendors.length / 2)
  const firstRow = props.vendors.slice(0, midpoint)
  const secondRow = props.vendors.slice(midpoint)
  const resolvedSecondRow = secondRow.length > 0 ? secondRow : firstRow

  return (
    <div className='home-marquee border-border/40 overflow-hidden border-y py-4'>
      <MarqueeRow vendors={firstRow} direction='forward' />
      <MarqueeRow vendors={resolvedSecondRow} direction='reverse' />
    </div>
  )
}
