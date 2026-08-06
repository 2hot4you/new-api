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
import { Calculator, Clapperboard, Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Label } from '@/components/ui/label'

import type { UsageLog } from '../../data/schema'
import {
  formatGrokVideoCny,
  formatGrokVideoFormula,
  getGrokVideoBillingState,
} from '../../lib/grok-video-billing'
import type { GrokVideoBillingV1 } from '../../types'

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
        title={typeof props.value === 'string' ? props.value : undefined}
      >
        {props.value}
      </div>
    </div>
  )
}

function formatDuration(seconds: number): string {
  return `${seconds}s`
}

function formatResolution(resolution: string): string {
  return resolution ? resolution.toUpperCase() : '-'
}

function operationLabel(
  operation: GrokVideoBillingV1['operation'],
  t: (key: string) => string
): string {
  if (operation === 'image_to_video') return t('Image to Video')
  if (operation === 'video_edit') return t('Video Editing')
  return t('Text to Video')
}

function inputTypeLabel(
  inputType: GrokVideoBillingV1['input_type'],
  t: (key: string) => string
): string {
  if (inputType === 'image') return t('Image')
  if (inputType === 'video') return t('Video')
  return t('Text')
}

export function GrokVideoBillingCard(props: {
  log: UsageLog
  quotaPerUnit: number
}) {
  const { t } = useTranslation()
  const state = getGrokVideoBillingState(props.log)
  if (state.kind === 'not-grok-video') return null

  const historicalFinalCost =
    Number.isFinite(props.quotaPerUnit) && props.quotaPerUnit > 0
      ? formatGrokVideoCny(props.log.quota / props.quotaPerUnit)
      : '-'

  return (
    <section className='min-w-0 space-y-1.5'>
      <Label className='flex items-center gap-1.5 text-xs font-semibold'>
        <Clapperboard className='size-3.5 text-sky-500' aria-hidden='true' />
        {t('Grok Video Billing')}
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
                value={operationLabel(state.billing.operation, t)}
              />
              <BillingMetric
                label={t('Input Type')}
                value={inputTypeLabel(state.billing.input_type, t)}
              />
              {state.billing.requested_duration_seconds > 0 && (
                <BillingMetric
                  label={t('Requested Duration')}
                  value={formatDuration(
                    state.billing.requested_duration_seconds
                  )}
                  mono
                />
              )}
              <BillingMetric
                label={t('Billing Duration')}
                value={formatDuration(state.billing.actual_duration_seconds)}
                mono
              />
              {state.billing.requested_resolution.trim() !== '' && (
                <BillingMetric
                  label={t('Requested Resolution')}
                  value={formatResolution(state.billing.requested_resolution)}
                  mono
                />
              )}
              <BillingMetric
                label={t('Billing Resolution')}
                value={formatResolution(state.billing.actual_resolution)}
                mono
              />
              <BillingMetric
                label={t('Aspect Ratio')}
                value={state.billing.aspect_ratio || '-'}
                mono
              />
              {state.billing.operation === 'image_to_video' && (
                <BillingMetric
                  label={t('Input Images')}
                  value={state.billing.input_image_count}
                  mono
                />
              )}
              {state.billing.operation === 'video_edit' && (
                <BillingMetric
                  label={t('Video Input Billed Seconds')}
                  value={formatDuration(
                    state.billing.video_input_billed_seconds
                  )}
                  mono
                />
              )}
            </div>

            <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
              <BillingMetric
                label={t('Output Unit Price')}
                value={`${formatGrokVideoCny(state.billing.output_unit_price)} / s`}
                mono
              />
              <BillingMetric
                label={t('Output Subtotal')}
                value={formatGrokVideoCny(state.billing.output_cost)}
                mono
              />
              {state.billing.operation === 'image_to_video' && (
                <>
                  <BillingMetric
                    label={t('Image Input Unit Price')}
                    value={`${formatGrokVideoCny(state.billing.image_input_unit_price)} / ${t('Image').toLowerCase()}`}
                    mono
                  />
                  <BillingMetric
                    label={t('Image Input Subtotal')}
                    value={formatGrokVideoCny(state.billing.image_input_cost)}
                    mono
                  />
                </>
              )}
              {state.billing.operation === 'video_edit' && (
                <>
                  <BillingMetric
                    label={t('Video Input Unit Price')}
                    value={`${formatGrokVideoCny(state.billing.video_input_unit_price)} / s`}
                    mono
                  />
                  <BillingMetric
                    label={t('Video Input Subtotal')}
                    value={formatGrokVideoCny(state.billing.video_input_cost)}
                    mono
                  />
                </>
              )}
              <BillingMetric
                label={t('Subtotal')}
                value={formatGrokVideoCny(state.billing.subtotal)}
                mono
              />
              <BillingMetric
                label={t('Group Ratio')}
                value={`${state.billing.group_ratio.toFixed(4)}x`}
                mono
              />
            </div>

            <div className='space-y-1.5 rounded-md border border-sky-200 bg-sky-50/70 p-2 dark:border-sky-900 dark:bg-sky-950/20'>
              <div className='flex items-center gap-1.5 text-xs font-medium text-sky-700 dark:text-sky-300'>
                <Calculator className='size-3.5' aria-hidden='true' />
                {t('Billing Formula')}
              </div>
              <p className='overflow-x-auto font-mono text-xs whitespace-nowrap'>
                {formatGrokVideoFormula(state.billing)}
              </p>
            </div>

            <div className='flex items-center justify-between gap-3 border-t pt-2 text-xs'>
              <span className='text-muted-foreground'>{t('Final Charge')}</span>
              <span className='font-mono font-semibold'>
                {formatGrokVideoCny(state.billing.final_cost)}
              </span>
            </div>
          </>
        )}
      </div>
    </section>
  )
}
