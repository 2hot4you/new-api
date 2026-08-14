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
import { useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, Database, Loader2, RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useIsMobile } from '@/hooks/use-mobile'
import { cn } from '@/lib/utils'

import { syncUpstream } from '../../api'
import {
  DEFAULT_MODEL_METADATA_SYNC_MODE,
  MODEL_METADATA_SYNC_MODES,
  modelsQueryKeys,
  vendorsQueryKeys,
} from '../../lib'
import type { ModelMetadataSyncMode } from '../../types'

type SyncWizardDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SyncWizardDialog({
  open,
  onOpenChange,
}: SyncWizardDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isMobile = useIsMobile()
  const [isSyncing, setIsSyncing] = useState(false)
  const [syncMode, setSyncMode] = useState<ModelMetadataSyncMode>(
    DEFAULT_MODEL_METADATA_SYNC_MODE
  )

  useEffect(() => {
    if (open) {
      setSyncMode(DEFAULT_MODEL_METADATA_SYNC_MODE)
    }
  }, [open])

  const handleSync = async () => {
    setIsSyncing(true)
    try {
      const response = await syncUpstream({ sync_mode: syncMode })
      if (!response.success) {
        throw new Error(response.message || t('Sync failed'))
      }

      const {
        created_models = 0,
        updated_models = 0,
        skipped_models = [],
      } = response.data || {}
      toast.success(
        t(
          'Model metadata sync completed: {{created}} created, {{updated}} updated, {{skipped}} skipped',
          {
            created: created_models,
            updated: updated_models,
            skipped: skipped_models.length,
          }
        )
      )
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: ['pricing'] }),
      ])
      onOpenChange(false)
    } catch (error: unknown) {
      toast.error((error as Error)?.message || t('Sync failed'))
    } finally {
      setIsSyncing(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Sync model metadata from models.dev')}
      description={t(
        'Fetch the latest public model capabilities and descriptions from models.dev.'
      )}
      initialFocus={!isMobile}
      contentHeight='auto'
      bodyClassName='flex flex-col gap-4'
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={isSyncing}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={handleSync} disabled={isSyncing}>
            {isSyncing ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : (
              <RefreshCw className='mr-2 h-4 w-4' />
            )}
            {isSyncing ? t('Syncing...') : t('Sync Now')}
          </Button>
        </>
      }
    >
      <div className='space-y-3'>
        <Label className='text-base'>{t('Metadata priority')}</Label>
        <RadioGroup
          value={syncMode}
          onValueChange={(value) => setSyncMode(value as ModelMetadataSyncMode)}
          className='grid gap-3 sm:grid-cols-2'
        >
          {MODEL_METADATA_SYNC_MODES.map((option) => (
            <Label
              key={option.value}
              htmlFor={`metadata-sync-mode-${option.value}`}
              className={cn(
                'hover:border-primary/40 has-data-[checked]:border-primary has-data-[checked]:ring-primary/20 bg-card flex cursor-pointer items-start gap-3 rounded-xl border p-4 font-normal transition-all has-data-[checked]:ring-2',
                option.destructive &&
                  'has-data-[checked]:border-destructive has-data-[checked]:ring-destructive/20'
              )}
            >
              <RadioGroupItem
                id={`metadata-sync-mode-${option.value}`}
                value={option.value}
                className='mt-0.5'
              />
              <div className='space-y-1'>
                <p className='font-medium'>{t(option.titleKey)}</p>
                <p className='text-muted-foreground text-sm'>
                  {t(option.descriptionKey)}
                </p>
                {option.destructive && (
                  <p className='text-destructive flex items-start gap-1.5 pt-1 text-xs'>
                    <AlertTriangle className='mt-0.5 size-3.5 shrink-0' />
                    {t(
                      'This overwrites existing descriptions, vendors, icons, limits, modalities, capabilities, and dates.'
                    )}
                  </p>
                )}
              </div>
            </Label>
          ))}
        </RadioGroup>
      </div>

      <div className='bg-muted/50 flex gap-3 rounded-lg border p-4'>
        <Database className='text-muted-foreground mt-0.5 h-5 w-5 shrink-0' />
        <div className='space-y-1 text-sm'>
          <p className='font-medium'>{t('Source: models.dev')}</p>
          <p className='text-muted-foreground'>
            {t(
              'Local pricing, channels, enabled state, and routing configuration are always preserved.'
            )}
          </p>
          <p className='text-muted-foreground'>
            {t(
              'Chinese model descriptions are manually reviewed in Molii and are not machine-translated during synchronization.'
            )}
          </p>
        </div>
      </div>
    </Dialog>
  )
}
