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

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import { normalizeDiscount } from '../lib/quotation-math'
import type { QuoteProviderOverride } from '../types'

export type ProviderSettingItem = {
  key: string
  name: string
}

type ProviderQuoteSettingsProps = {
  globalDiscount: number | null
  providers: ProviderSettingItem[]
  overrides: Record<string, QuoteProviderOverride>
  onGlobalDiscountChange: (discount: number | null) => void
  onOverrideChange: (
    providerKey: string,
    override: QuoteProviderOverride
  ) => void
}

function parseOptionalNumber(value: string): number | null {
  return value === '' ? null : Number(value)
}

export function ProviderQuoteSettings({
  globalDiscount,
  providers,
  overrides,
  onGlobalDiscountChange,
  onOverrideChange,
}: ProviderQuoteSettingsProps) {
  const { t } = useTranslation()
  const globalInvalid = normalizeDiscount(globalDiscount) === null

  return (
    <section
      aria-labelledby='quotation-provider-settings-title'
      className='space-y-4'
    >
      <div>
        <h3 id='quotation-provider-settings-title' className='font-medium'>
          {t('Discounts and provider notes')}
        </h3>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Provider discounts override the global discount. Leave blank to inherit.'
          )}
        </p>
      </div>

      <div className='space-y-1.5'>
        <Label htmlFor='quotation-global-discount'>
          {t('Global discount (10 = full price)')}
        </Label>
        <Input
          id='quotation-global-discount'
          type='number'
          inputMode='decimal'
          min='0'
          max='10'
          step='any'
          value={globalDiscount ?? ''}
          onChange={(event) =>
            onGlobalDiscountChange(parseOptionalNumber(event.target.value))
          }
          aria-invalid={globalInvalid}
          aria-describedby={
            globalInvalid ? 'quotation-global-discount-error' : undefined
          }
        />
        {globalInvalid && (
          <p
            id='quotation-global-discount-error'
            className='text-destructive text-xs'
          >
            {t('Enter a discount greater than 0 and at most 10.')}
          </p>
        )}
      </div>

      {providers.length === 0 ? (
        <p className='text-muted-foreground rounded-lg border border-dashed p-4 text-center text-sm'>
          {t('Select models to configure provider discounts and notes.')}
        </p>
      ) : (
        <div className='space-y-3'>
          {providers.map((provider) => {
            const override = overrides[provider.key] ?? {
              discount: null,
              note: '',
            }
            const invalid =
              override.discount !== null &&
              normalizeDiscount(override.discount) === null
            const errorId = `quotation-provider-discount-${provider.key}-error`
            return (
              <fieldset
                key={provider.key}
                className='space-y-3 rounded-lg border p-3'
              >
                <legend className='px-1 text-sm font-semibold'>
                  {provider.name}
                </legend>
                <div className='space-y-1.5'>
                  <Label
                    htmlFor={`quotation-provider-discount-${provider.key}`}
                  >
                    {t('{{provider}} discount (10 = full price)', {
                      provider: provider.name,
                    })}
                  </Label>
                  <Input
                    id={`quotation-provider-discount-${provider.key}`}
                    type='number'
                    inputMode='decimal'
                    min='0'
                    max='10'
                    step='any'
                    value={override.discount ?? ''}
                    onChange={(event) =>
                      onOverrideChange(provider.key, {
                        ...override,
                        discount: parseOptionalNumber(event.target.value),
                      })
                    }
                    placeholder={t('Use global discount')}
                    aria-invalid={invalid}
                    aria-describedby={invalid ? errorId : undefined}
                  />
                  {invalid && (
                    <p id={errorId} className='text-destructive text-xs'>
                      {t('Enter a discount greater than 0 and at most 10.')}
                    </p>
                  )}
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor={`quotation-provider-note-${provider.key}`}>
                    {t('{{provider}} note', { provider: provider.name })}
                  </Label>
                  <Textarea
                    id={`quotation-provider-note-${provider.key}`}
                    value={override.note}
                    maxLength={50_000}
                    onChange={(event) =>
                      onOverrideChange(provider.key, {
                        ...override,
                        note: event.target.value,
                      })
                    }
                    placeholder={t('Terms or notes shown in the quotation')}
                  />
                </div>
              </fieldset>
            )
          })}
        </div>
      )}
    </section>
  )
}
