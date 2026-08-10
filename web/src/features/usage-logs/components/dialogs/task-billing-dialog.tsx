/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Calculator, Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Label } from '@/components/ui/label'

import {
  formatTaskBillingCny,
  formatTaskBillingFormula,
} from '../../lib/task-billing'
import type { GrokVideoBillingV1, TaskBillingSummary } from '../../types'

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

function operationLabel(
  operation: GrokVideoBillingV1['operation'],
  t: (key: string) => string
) {
  if (operation === 'image_to_video') return t('Image to Video')
  if (operation === 'video_edit') return t('Video Editing')
  return t('Text to Video')
}

function inputTypeLabel(
  inputType: GrokVideoBillingV1['input_type'],
  t: (key: string) => string
) {
  if (inputType === 'image') return t('Image')
  if (inputType === 'video') return t('Video')
  return t('Text')
}

export function TaskBillingDialog(props: {
  billing: TaskBillingSummary
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const { billing } = props
  const formula = formatTaskBillingFormula(billing)
  const seedance = billing.seedance
  const grok = billing.grok_video

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Billing Details')}
      description={t('Generation Records')}
      contentClassName='sm:max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      <div className='space-y-3 py-4'>
        <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
          <BillingMetric
            label={t('Model ID')}
            value={billing.model || '-'}
            mono
          />
          <BillingMetric
            label={t('Final Charge')}
            value={formatTaskBillingCny(billing.final_cost)}
            mono
          />
          <BillingMetric
            label={t('Group Ratio')}
            value={`${billing.group_ratio.toFixed(4)}x`}
            mono
          />
        </div>

        {!billing.detail_available ? (
          <div className='flex items-start gap-1.5 rounded-md border border-amber-200 bg-amber-50/70 p-2 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950/20 dark:text-amber-300'>
            <Info className='mt-0.5 size-3.5 shrink-0' aria-hidden='true' />
            <span>{t('Historical billing breakdown unavailable')}</span>
          </div>
        ) : null}

        {billing.detail_available && billing.mode === 'seedance' && seedance ? (
          <section className='space-y-2'>
            <Label className='text-xs font-semibold'>
              {t('Seedance Video Billing')}
            </Label>
            <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
              <BillingMetric
                label={t('Actual Tokens')}
                value={seedance.actual_tokens}
                mono
              />
              <BillingMetric
                label={t('Unit Price')}
                value={`${formatTaskBillingCny(seedance.unit_price)} / 1M Tokens`}
                mono
              />
              <BillingMetric
                label={t('Resolution')}
                value={seedance.resolution?.toUpperCase() || '-'}
                mono
              />
              <BillingMetric
                label={t('Aspect Ratio')}
                value={seedance.ratio || '-'}
                mono
              />
              <BillingMetric
                label={t('Duration')}
                value={seedance.seconds ? `${seedance.seconds}s` : '-'}
                mono
              />
              <BillingMetric
                label={t('Input Type')}
                value={t(
                  seedance.has_video
                    ? 'With reference video'
                    : 'Without reference video'
                )}
              />
            </div>
          </section>
        ) : null}

        {billing.detail_available && billing.mode === 'grok_video' && grok ? (
          <section className='space-y-2'>
            <Label className='text-xs font-semibold'>
              {t('Grok Video Billing')}
            </Label>
            <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
              <BillingMetric
                label={t('Operation')}
                value={operationLabel(grok.operation, t)}
              />
              <BillingMetric
                label={t('Input Type')}
                value={inputTypeLabel(grok.input_type, t)}
              />
              <BillingMetric
                label={t('Billing Duration')}
                value={`${grok.actual_duration_seconds}s`}
                mono
              />
              {grok.requested_duration_seconds > 0 ? (
                <BillingMetric
                  label={t('Requested Duration')}
                  value={`${grok.requested_duration_seconds}s`}
                  mono
                />
              ) : null}
              <BillingMetric
                label={t('Billing Resolution')}
                value={grok.actual_resolution.toUpperCase()}
                mono
              />
              {grok.requested_resolution ? (
                <BillingMetric
                  label={t('Requested Resolution')}
                  value={grok.requested_resolution.toUpperCase()}
                  mono
                />
              ) : null}
              <BillingMetric
                label={t('Aspect Ratio')}
                value={grok.aspect_ratio || '-'}
                mono
              />
              {grok.operation === 'image_to_video' ? (
                <BillingMetric
                  label={t('Input Images')}
                  value={grok.input_image_count}
                  mono
                />
              ) : null}
              {grok.operation === 'video_edit' ? (
                <BillingMetric
                  label={t('Video Input Billed Seconds')}
                  value={`${grok.video_input_billed_seconds}s`}
                  mono
                />
              ) : null}
            </div>
            <div className='grid grid-cols-2 gap-2 sm:grid-cols-3'>
              <BillingMetric
                label={t('Output Unit Price')}
                value={`${formatTaskBillingCny(grok.output_unit_price)} / s`}
                mono
              />
              <BillingMetric
                label={t('Output Subtotal')}
                value={formatTaskBillingCny(grok.output_cost)}
                mono
              />
              {grok.operation === 'image_to_video' ? (
                <BillingMetric
                  label={t('Image Input Unit Price')}
                  value={`${formatTaskBillingCny(grok.image_input_unit_price)} / image`}
                  mono
                />
              ) : null}
              {grok.operation === 'video_edit' ? (
                <BillingMetric
                  label={t('Video Input Unit Price')}
                  value={`${formatTaskBillingCny(grok.video_input_unit_price)} / s`}
                  mono
                />
              ) : null}
              <BillingMetric
                label={t('Subtotal')}
                value={formatTaskBillingCny(grok.subtotal)}
                mono
              />
            </div>
          </section>
        ) : null}

        {formula ? (
          <div className='space-y-1.5 rounded-md border border-sky-200 bg-sky-50/70 p-2 dark:border-sky-900 dark:bg-sky-950/20'>
            <div className='flex items-center gap-1.5 text-xs font-medium text-sky-700 dark:text-sky-300'>
              <Calculator className='size-3.5' aria-hidden='true' />
              {t('Billing Formula')}
            </div>
            <p className='overflow-x-auto font-mono text-xs whitespace-nowrap'>
              {formula}
            </p>
          </div>
        ) : null}
      </div>
    </Dialog>
  )
}
