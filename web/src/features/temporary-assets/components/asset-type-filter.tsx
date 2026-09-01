/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Image, LayoutGrid, Music, Video } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import type { AssetType } from '../lib/asset-utils'

export type AssetTypeFilterValue = 'all' | AssetType

type AssetTypeFilterProps = {
  value: AssetTypeFilterValue
  onValueChange: (value: AssetTypeFilterValue) => void
}

const filterOptions = [
  { value: 'all', label: 'All', Icon: LayoutGrid },
  { value: 'image', label: 'Image', Icon: Image },
  { value: 'video', label: 'Video', Icon: Video },
  { value: 'audio', label: 'Audio', Icon: Music },
] as const

export function AssetTypeFilter(props: AssetTypeFilterProps) {
  const { t } = useTranslation()

  return (
    <div
      className='grid grid-cols-2 gap-2 sm:grid-cols-4'
      role='group'
      aria-label={t('All Types')}
    >
      {filterOptions.map((option) => {
        const selected = props.value === option.value
        return (
          <button
            key={option.value}
            type='button'
            aria-pressed={selected}
            className={cn(
              'flex min-h-11 items-center justify-center gap-2 rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
              selected
                ? 'border-primary bg-primary/10 text-primary shadow-sm'
                : 'bg-card text-muted-foreground hover:border-primary/40 hover:bg-muted/50 hover:text-foreground'
            )}
            onClick={() => props.onValueChange(option.value)}
          >
            <option.Icon className='size-4' aria-hidden='true' />
            <span>{t(option.label)}</span>
          </button>
        )
      })}
    </div>
  )
}
