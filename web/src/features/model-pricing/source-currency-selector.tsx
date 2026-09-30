/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { HelpCircle } from 'lucide-react'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { SourcePricingCurrency } from './api'

export function SourceCurrencySelector(props: {
  value: SourcePricingCurrency
  onChange: (value: SourcePricingCurrency) => void
  legacyMode?: boolean
}) {
  const { t } = useTranslation()
  const id = useId()
  const items = [
    { value: 'USD', label: t('US dollar (USD)') },
    { value: 'CNY', label: t('Chinese yuan (CNY)') },
  ]

  return (
    <Field className='mb-4 gap-2'>
      <div className='flex items-center gap-1'>
        <FieldLabel htmlFor={id}>{t('Source pricing currency')}</FieldLabel>
        <Dialog
          title={t('About source pricing currency')}
          contentClassName='sm:max-w-lg'
          trigger={
            <Button
              type='button'
              size='icon-sm'
              variant='ghost'
              aria-label={t('About source pricing currency')}
            >
              <HelpCircle aria-hidden='true' />
            </Button>
          }
        >
          <p className='text-sm'>
            {t(
              'Expression coefficients are stored exactly in this source currency. User-facing prices still follow the site display currency.'
            )}
          </p>
        </Dialog>
      </div>
      <Select
        items={items}
        value={props.value}
        disabled={props.legacyMode}
        onValueChange={(value) => {
          if (value === 'USD' || value === 'CNY') props.onChange(value)
        }}
      >
        <SelectTrigger id={id} className='w-full sm:w-64'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <FieldDescription>
        {props.legacyMode
          ? t('Legacy ratio and fixed-price billing remains USD-compatible.')
          : t('Changing currency does not rewrite expression coefficients.')}
      </FieldDescription>
    </Field>
  )
}
