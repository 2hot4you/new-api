import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'

import { rotateApiKey } from '../api'
import { ERROR_MESSAGES } from '../constants'
import { toDisplayApiKey } from '../types'
import { useApiKeys } from './api-keys-provider'

export function ApiKeysRotateDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh, replaceResolvedKey } =
    useApiKeys()
  const [isRotating, setIsRotating] = useState(false)

  const handleConfirm = async () => {
    if (!currentRow) return
    setIsRotating(true)
    try {
      const result = await rotateApiKey(currentRow.id)
      if (result.success && result.data?.key) {
        replaceResolvedKey(currentRow.id, toDisplayApiKey(result.data.key))
        toast.success(
          t('The new API key is shown in the list. Copy and save it now.')
        )
        triggerRefresh()
        setOpen(null)
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.UNEXPECTED))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsRotating(false)
    }
  }

  return (
    <ConfirmDialog
      destructive
      open={open === 'rotate'}
      onOpenChange={(isOpen) => !isOpen && setOpen(null)}
      handleConfirm={handleConfirm}
      isLoading={isRotating}
      title={t('Rotate API key?')}
      desc={t(
        'The old API key will stop working immediately. This action cannot be undone.'
      )}
      confirmText={t('Rotate')}
    />
  )
}
