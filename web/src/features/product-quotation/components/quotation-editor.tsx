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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { PricingModel, PricingVendor } from '@/features/pricing/types'

import type { QuotationDraft } from '../types'
import { ExchangeDiscountCalculator } from './exchange-discount-calculator'
import { ProviderModelSelector } from './provider-model-selector'
import {
  ProviderQuoteSettings,
  type ProviderSettingItem,
} from './provider-quote-settings'

type QuotationEditorProps = {
  draft: QuotationDraft
  models: PricingModel[]
  vendors: PricingVendor[]
  groups: string[]
  actualExchangeRate: number
  fixedExchangeRate: number
  onDraftChange: (draft: QuotationDraft) => void
}

export function QuotationEditor({
  draft,
  models,
  vendors,
  groups,
  actualExchangeRate,
  fixedExchangeRate,
  onDraftChange,
}: QuotationEditorProps) {
  const { t } = useTranslation()
  const vendorById = useMemo(
    () => new Map(vendors.map((vendor) => [vendor.id, vendor])),
    [vendors]
  )
  const selectedProviders = useMemo(() => {
    const selected = new Set(draft.selectedModelIds)
    const providers = new Map<string, ProviderSettingItem>()
    for (const model of models) {
      if (!selected.has(model.model_name)) continue
      const providerId = model.vendor_id ?? null
      const key = providerId === null ? 'unknown' : String(providerId)
      providers.set(key, {
        key,
        name:
          (providerId === null ? null : vendorById.get(providerId)?.name) ??
          model.vendor_name ??
          t('Unknown provider'),
      })
    }
    return [...providers.values()].sort((left, right) =>
      left.name.localeCompare(right.name)
    )
  }, [draft.selectedModelIds, models, t, vendorById])
  const selectedGroup =
    draft.priceBasis.type === 'group' ? draft.priceBasis.group : null

  return (
    <div className='min-w-0 space-y-4'>
      <ExchangeDiscountCalculator
        defaultActualRate={actualExchangeRate}
        defaultFixedRate={fixedExchangeRate}
      />

      <Card>
        <CardHeader>
          <CardTitle>{t('Quotation details')}</CardTitle>
          <CardDescription>
            {t(
              'This information appears in the live preview and exported HTML.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-1.5 sm:col-span-2'>
            <Label htmlFor='quotation-title'>{t('Quotation title')}</Label>
            <Input
              id='quotation-title'
              value={draft.title}
              maxLength={240}
              onChange={(event) =>
                onDraftChange({ ...draft, title: event.target.value })
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='quotation-customer'>{t('Customer')}</Label>
            <Input
              id='quotation-customer'
              value={draft.customer}
              maxLength={240}
              onChange={(event) =>
                onDraftChange({ ...draft, customer: event.target.value })
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='quotation-by'>{t('Quoted by')}</Label>
            <Input
              id='quotation-by'
              value={draft.quotedBy}
              maxLength={200}
              onChange={(event) =>
                onDraftChange({ ...draft, quotedBy: event.target.value })
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='quotation-date'>{t('Quote date')}</Label>
            <Input
              id='quotation-date'
              type='date'
              value={draft.quoteDate}
              onChange={(event) =>
                onDraftChange({ ...draft, quoteDate: event.target.value })
              }
            />
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('Price basis')}</CardTitle>
          <CardDescription>
            {t(
              'Choose raw pricing or one explicit user group. The group ratio is applied once.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-1.5'>
          <Label htmlFor='quotation-price-basis'>{t('Price basis')}</Label>
          <select
            id='quotation-price-basis'
            value={
              draft.priceBasis.type === 'raw'
                ? 'raw'
                : `group:${draft.priceBasis.group}`
            }
            onChange={(event) => {
              const value = event.target.value
              onDraftChange({
                ...draft,
                priceBasis:
                  value === 'raw'
                    ? { type: 'raw' }
                    : { type: 'group', group: value.slice('group:'.length) },
              })
            }}
            className='border-input focus-visible:border-ring focus-visible:ring-ring/50 h-8 w-full rounded-lg border bg-transparent px-2.5 text-sm outline-none focus-visible:ring-3'
          >
            <option value='raw'>{t('Raw model price (1x)')}</option>
            {groups.map((group) => (
              <option key={group} value={`group:${group}`}>
                {t('User group: {{group}}', { group })}
              </option>
            ))}
          </select>
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <ProviderModelSelector
            models={models}
            vendors={vendors}
            selectedModelIds={draft.selectedModelIds}
            selectedGroup={selectedGroup}
            onSelectionChange={(selectedModelIds) =>
              onDraftChange({ ...draft, selectedModelIds })
            }
          />
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <ProviderQuoteSettings
            globalDiscount={draft.globalDiscount}
            providers={selectedProviders}
            overrides={draft.providerOverrides}
            onGlobalDiscountChange={(globalDiscount) =>
              onDraftChange({ ...draft, globalDiscount })
            }
            onOverrideChange={(providerKey, override) =>
              onDraftChange({
                ...draft,
                providerOverrides: {
                  ...draft.providerOverrides,
                  [providerKey]: override,
                },
              })
            }
          />
        </CardContent>
      </Card>
    </div>
  )
}
