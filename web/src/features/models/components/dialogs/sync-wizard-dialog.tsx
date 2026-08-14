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
import { Database, Loader2, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/hooks/use-mobile'

import { syncUpstream } from '../../api'
import { modelsQueryKeys, vendorsQueryKeys } from '../../lib'

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

  const handleSync = async () => {
    setIsSyncing(true)
    try {
      const response = await syncUpstream()
      if (!response.success) {
        throw new Error(response.message || t('Sync failed'))
      }

      const { created_models = 0, updated_models = 0 } = response.data || {}
      toast.success(
        t(
          'Model metadata sync completed: {{created}} created, {{updated}} updated',
          {
            created: created_models,
            updated: updated_models,
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
      <div className='bg-muted/50 flex gap-3 rounded-lg border p-4'>
        <Database className='text-muted-foreground mt-0.5 h-5 w-5 shrink-0' />
        <div className='space-y-1 text-sm'>
          <p className='font-medium'>{t('Source: models.dev')}</p>
          <p className='text-muted-foreground'>
            {t(
              'Local pricing, channels, enabled state, and administrator-authored descriptions are preserved.'
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
