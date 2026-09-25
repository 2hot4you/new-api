/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { CreditCard } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

export function VideoStudioPaidRequestDialog(props: {
  open: boolean
  estimatedCost: string
  estimatedTokens?: number
  pending: boolean
  onConfirm: () => void
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  return (
    <AlertDialog open={props.open} onOpenChange={props.onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogMedia>
            <CreditCard />
          </AlertDialogMedia>
          <AlertDialogTitle>{t('Confirm paid request')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'This action will submit a paid video generation request. The final charge is based on the provider result.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <div className='bg-muted/40 grid gap-3 rounded-lg border p-4 sm:grid-cols-2'>
          <div>
            <p className='text-muted-foreground text-xs'>
              {t('Estimated cost')}
            </p>
            <p className='mt-1 text-xl font-semibold'>{props.estimatedCost}</p>
          </div>
          {props.estimatedTokens != null && props.estimatedTokens > 0 && (
            <div>
              <p className='text-muted-foreground text-xs'>
                {t('Estimated tokens')}
              </p>
              <p className='mt-1 text-xl font-semibold'>
                {props.estimatedTokens.toLocaleString()}
              </p>
            </div>
          )}
        </div>
        <p className='text-muted-foreground text-xs'>
          {t(
            'The estimate uses the selected API key, model, parameters, and effective group ratio. It does not reserve quota.'
          )}
        </p>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={props.pending}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction disabled={props.pending} onClick={props.onConfirm}>
            {props.pending ? t('Submitting...') : t('Confirm and pay')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
