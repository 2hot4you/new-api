/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { AlertCircle, CheckCircle2, EyeOff, Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SideDrawerSection } from '@/components/drawer-layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'

const missingFieldLabels: Record<string, string> = {
  model_name: 'Model ID',
  display_name: 'Display name',
  description: 'Chinese description',
  vendor_id: 'Vendor',
  release_date: 'Release date',
  input_modalities: 'Input modalities',
  output_modalities: 'Output modalities',
  capabilities: 'Capabilities',
  supported_parameters: 'Supported parameters',
  context_length: 'Context length',
  max_output_tokens: 'Maximum output tokens',
  supported_resolutions: 'Supported resolutions',
  supported_aspect_ratios: 'Supported aspect ratios',
  output_formats: 'Output formats',
  max_input_images: 'Maximum input images',
  min_duration: 'Minimum duration',
  max_duration: 'Maximum duration',
  reference_modalities: 'Reference modalities',
}

const blockerLabels: Record<string, string> = {
  vendor_disabled: 'The selected vendor is disabled',
  pricing_missing: 'Pricing is not configured',
  group_unavailable: 'No available group',
  endpoint_unavailable: 'No available endpoint',
}

export type ModelPublicationStatusProps = {
  enabled: boolean
  onEnabledChange: (enabled: boolean) => void
  modelEnabled: boolean
  onModelEnabledChange?: (enabled: boolean) => void
  complete: boolean
  missingFields: string[]
  visible: boolean
  blockers: string[]
  withdrawn: boolean
}

export function ModelPublicationStatus({
  enabled,
  onEnabledChange,
  modelEnabled,
  onModelEnabledChange,
  complete,
  missingFields,
  visible,
  blockers,
  withdrawn,
}: ModelPublicationStatusProps) {
  const { t } = useTranslation()
  const publicationDisabled = !enabled && !complete
  let publicationLabel = t('Draft')
  if (enabled) publicationLabel = t('Publication blocked')
  if (visible) publicationLabel = t('Published in model marketplace')

  return (
    <div data-marketplace-publication='true'>
      <SideDrawerSection>
        <div className='space-y-1'>
          <h3 className='text-sm font-semibold'>{t('Publication status')}</h3>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Drafts can be saved at any time. Publication requires complete metadata.'
            )}
          </p>
        </div>

        <div className='flex items-center justify-between gap-4 rounded-lg border p-3'>
          <div className='space-y-0.5'>
            <div className='text-sm font-medium'>{t('Model enabled')}</div>
            <div className='text-muted-foreground text-xs'>
              {t('Controls whether the model can be used by the system.')}
            </div>
          </div>
          <Switch
            checked={modelEnabled}
            onCheckedChange={onModelEnabledChange}
            aria-label={t('Model enabled')}
          />
        </div>

        <div className='flex items-center justify-between gap-4 rounded-lg border p-3'>
          <div className='space-y-0.5'>
            <div className='flex flex-wrap items-center gap-2 text-sm font-medium'>
              {t('Publish in model marketplace')}
              <Badge variant={visible ? 'default' : 'outline'}>
                {publicationLabel}
              </Badge>
            </div>
            <div className='text-muted-foreground text-xs'>
              {t('Makes this model visible on the public pricing page.')}
            </div>
          </div>
          <Switch
            checked={enabled}
            onCheckedChange={onEnabledChange}
            disabled={publicationDisabled}
            aria-label={t('Publish in model marketplace')}
          />
        </div>

        {!complete && missingFields.length > 0 && (
          <Alert>
            <AlertCircle />
            <AlertTitle>{t('Missing required metadata')}</AlertTitle>
            <AlertDescription>
              <ul className='mt-1 list-disc space-y-1 pl-4'>
                {missingFields.map((field) => (
                  <li key={field}>{t(missingFieldLabels[field] ?? field)}</li>
                ))}
              </ul>
            </AlertDescription>
          </Alert>
        )}

        {blockers.length > 0 && (
          <Alert>
            <EyeOff />
            <AlertTitle>{t('Temporarily not visible')}</AlertTitle>
            <AlertDescription>
              <ul className='mt-1 list-disc space-y-1 pl-4'>
                {blockers.map((blocker) => (
                  <li key={blocker}>{t(blockerLabels[blocker] ?? blocker)}</li>
                ))}
              </ul>
            </AlertDescription>
          </Alert>
        )}

        {withdrawn && (
          <Alert>
            <Info />
            <AlertTitle>{t('Publication automatically withdrawn')}</AlertTitle>
            <AlertDescription>
              {t(
                'The saved metadata became incomplete, so the model was automatically withdrawn from the marketplace.'
              )}
            </AlertDescription>
          </Alert>
        )}

        {visible && (
          <div className='text-muted-foreground flex items-center gap-2 text-xs'>
            <CheckCircle2 className='size-4 text-emerald-600' />
            {t('All publication requirements are currently satisfied.')}
          </div>
        )}
      </SideDrawerSection>
    </div>
  )
}
