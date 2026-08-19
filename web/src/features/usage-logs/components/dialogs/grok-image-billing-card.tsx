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
import { Calculator, ImageIcon, Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Label } from '@/components/ui/label'

import type { UsageLog } from '../../data/schema'
import {
  formatGrokImageCny,
  formatGrokImageFormula,
  getGrokImageBillingState,
} from '../../lib/grok-image-billing'

function BillingMetric(props: {
  label: React.ReactNode
  value: React.ReactNode
  mono?: boolean
}) {
  return (
    <div className='bg-background/70 min-w-0 rounded-md border px-2.5 py-2'>
      <div className='text-muted-foreground truncate text-[11px] leading-4'>
        {props.label}
      </div>
      <div
        className={`mt-0.5 min-w-0 truncate text-xs leading-5 font-medium ${props.mono ? 'font-mono' : ''}`}
      >
        {props.value}
      </div>
    </div>
  )
}

export function GrokImageBillingCard(props: {
  log: UsageLog
  quotaPerUnit: number
}) {
  const { t } = useTranslation()
  const state = getGrokImageBillingState(props.log)
  if (state.kind === 'not-grok-image') return null

  const historicalFinalCost =
    Number.isFinite(props.quotaPerUnit) && props.quotaPerUnit > 0
      ? formatGrokImageCny(props.log.quota / props.quotaPerUnit)
      : '-'

  return (
    <section className='min-w-0 space-y-1.5'>
      <Label className='flex items-center gap-1.5 text-xs font-semibold'>
        <ImageIcon className='size-3.5 text-violet-500' aria-hidden='true' />
        {t('Grok Image Billing')}
      </Label>
      <div className='bg-muted/30 min-w-0 space-y-2 overflow-hidden rounded-md border p-2.5 max-sm:p-2'>
        {state.kind === 'history' ? (
          <>
            <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
              <BillingMetric label={t('Model ID')} value={state.model} mono />
              <BillingMetric
                label={t('Final Charge')}
                value={historicalFinalCost}
                mono
              />
            </div>
            <div className='flex items-start gap-1.5 rounded-md border border-amber-200 bg-amber-50/70 p-2 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950/20 dark:text-amber-300'>
              <Info className='mt-0.5 size-3.5 shrink-0' aria-hidden='true' />
              <span>{t('Historical billing breakdown unavailable')}</span>
            </div>
          </>
        ) : (
          <>
            <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
              <BillingMetric
                label={t('Model ID')}
                value={state.billing.model}
                mono
              />
              <BillingMetric
                label={t('Operation')}
                value={
                  state.billing.operation === 'edit'
                    ? t('Image Editing')
                    : t('Image Generation')
                }
              />
              <BillingMetric
                label={t('Resolution')}
                value={state.billing.resolution.toUpperCase()}
                mono
              />
              {state.billing.quality && (
                <BillingMetric
                  label={t('Quality')}
                  value={state.billing.quality.toUpperCase()}
                  mono
                />
              )}
              <BillingMetric
                label={t('Aspect Ratio')}
                value={state.billing.aspect_ratio}
                mono
              />
              <BillingMetric
                label={t('Requested Outputs')}
                value={state.billing.requested_output_count}
                mono
              />
              <BillingMetric
                label={t('Actual Outputs')}
                value={state.billing.output_count}
                mono
              />
              {state.billing.operation === 'edit' && (
                <BillingMetric
                  label={t('Input Images')}
                  value={state.billing.input_image_count}
                  mono
                />
              )}
            </div>

            <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
              <BillingMetric
                label={t('Output Unit Price')}
                value={formatGrokImageCny(state.billing.output_unit_price)}
                mono
              />
              <BillingMetric
                label={t('Output Subtotal')}
                value={formatGrokImageCny(state.billing.output_cost)}
                mono
              />
              {state.billing.operation === 'edit' && (
                <>
                  <BillingMetric
                    label={t('Input Unit Price')}
                    value={formatGrokImageCny(state.billing.input_unit_price)}
                    mono
                  />
                  <BillingMetric
                    label={t('Input Subtotal')}
                    value={formatGrokImageCny(state.billing.input_cost)}
                    mono
                  />
                </>
              )}
              <BillingMetric
                label={t('Subtotal')}
                value={formatGrokImageCny(state.billing.subtotal)}
                mono
              />
              <BillingMetric
                label={t('Group Ratio')}
                value={`${state.billing.group_ratio.toFixed(4)}x`}
                mono
              />
            </div>

            <div className='space-y-1.5 rounded-md border border-violet-200 bg-violet-50/70 p-2 dark:border-violet-900 dark:bg-violet-950/20'>
              <div className='flex items-center gap-1.5 text-xs font-medium text-violet-700 dark:text-violet-300'>
                <Calculator className='size-3.5' aria-hidden='true' />
                {t('Billing Formula')}
              </div>
              <p className='overflow-x-auto font-mono text-xs whitespace-nowrap'>
                {formatGrokImageFormula(state.billing)}
              </p>
            </div>

            <div className='flex items-center justify-between gap-3 border-t pt-2 text-xs'>
              <span className='text-muted-foreground'>{t('Final Charge')}</span>
              <span className='font-mono font-semibold'>
                {formatGrokImageCny(state.billing.final_cost)}
              </span>
            </div>
          </>
        )}
      </div>
    </section>
  )
}
