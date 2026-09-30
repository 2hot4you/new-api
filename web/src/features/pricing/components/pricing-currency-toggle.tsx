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

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import type { PricingCurrencyMode } from '../lib/pricing-currency'

export function PricingCurrencyToggle(props: {
  value: PricingCurrencyMode
  onChange: (value: PricingCurrencyMode) => void
}) {
  const { t } = useTranslation()
  const options: Array<{ value: PricingCurrencyMode; label: string }> = [
    { value: 'source', label: t('Original currency') },
    { value: 'cny', label: t('Renminbi') },
  ]

  return (
    <div
      role='group'
      aria-label={t('Pricing currency')}
      className='bg-muted/60 inline-flex h-8 items-center rounded-lg border p-0.5'
    >
      {options.map((option) => {
        const isActive = option.value === props.value
        return (
          <Button
            key={option.value}
            type='button'
            variant='ghost'
            size='sm'
            aria-pressed={isActive}
            onClick={() => {
              if (!isActive) props.onChange(option.value)
            }}
            className={cn(
              'h-full rounded-md px-3 text-xs font-medium shadow-none',
              isActive
                ? 'bg-primary text-primary-foreground hover:bg-primary hover:text-primary-foreground'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {option.label}
          </Button>
        )
      })}
    </div>
  )
}
