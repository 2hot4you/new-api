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
import {
  AudioLines,
  Blocks,
  Image,
  LayoutGrid,
  ListFilter,
  MessageSquareText,
  Video,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import type { ModelCategory, ModelCategoryId } from '../lib/model-directory'

export interface ModelCategoryBarProps {
  categories: ModelCategory[]
  value: string
  onChange: (value: ModelCategoryId) => void
  className?: string
}

const CATEGORY_META: Record<
  ModelCategoryId,
  {
    label: string
    icon: React.ComponentType<{ className?: string }>
  }
> = {
  all: { label: 'All', icon: LayoutGrid },
  text: { label: 'Text', icon: MessageSquareText },
  image: { label: 'Image', icon: Image },
  video: { label: 'Video', icon: Video },
  audio: { label: 'Audio', icon: AudioLines },
  embedding: { label: 'Embeddings', icon: Blocks },
  rerank: { label: 'Rerank', icon: ListFilter },
}

export function ModelCategoryBar(props: ModelCategoryBarProps) {
  const { t } = useTranslation()

  return (
    <nav
      aria-label={t('Model Categories')}
      className={cn(
        'hover-scrollbar flex gap-1.5 overflow-x-auto py-1',
        props.className
      )}
    >
      {props.categories.map((category) => {
        const meta = CATEGORY_META[category.id]
        const Icon = meta.icon
        const active = props.value === category.id
        return (
          <button
            key={category.id}
            type='button'
            data-model-category={category.id}
            aria-pressed={active}
            onClick={() => props.onChange(category.id)}
            className={cn(
              'inline-flex h-8 shrink-0 items-center gap-1.5 rounded-md border px-2.5 text-xs font-medium transition-colors',
              active
                ? 'border-foreground/25 bg-foreground text-background'
                : 'border-border/70 bg-background text-muted-foreground hover:border-foreground/20 hover:text-foreground'
            )}
          >
            <Icon className='size-3.5' />
            <span>{t(meta.label)}</span>
            <span
              className={cn(
                'tabular-nums',
                active ? 'text-background/70' : 'text-muted-foreground/60'
              )}
            >
              {category.count}
            </span>
          </button>
        )
      })}
    </nav>
  )
}
