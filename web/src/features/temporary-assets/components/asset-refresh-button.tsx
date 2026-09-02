/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

type AssetRefreshButtonProps = {
  targetCount: number
  selectedCount: number
  refreshing: boolean
  disabled?: boolean
  onClick: () => void
}

export function AssetRefreshButton(props: AssetRefreshButtonProps) {
  const { t } = useTranslation()
  const label =
    props.selectedCount > 0
      ? t('Update selected asset status')
      : t('Update current asset status')

  return (
    <Button
      type='button'
      variant='outline'
      size='sm'
      disabled={props.disabled || props.refreshing || props.targetCount === 0}
      onClick={props.onClick}
    >
      <RefreshCw
        className={cn('size-3.5', props.refreshing && 'animate-spin')}
        aria-hidden='true'
      />
      <span>{label}</span>
      <span className='text-muted-foreground tabular-nums'>
        ({props.targetCount})
      </span>
    </Button>
  )
}
