/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { ArrowRight, Search, Sparkles } from 'lucide-react'
import { useId, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import type { PricingModel } from '@/features/pricing/types'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { searchHomeModels } from '../lib/home-model-catalog'

interface ModelSearchProps {
  models: PricingModel[]
  isLoading?: boolean
  onSearch?: (query: string) => void
}

export function ModelSearch(props: ModelSearchProps) {
  const { t } = useTranslation()
  const listboxId = useId()
  const [query, setQuery] = useState('')
  const [isFocused, setIsFocused] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const results = useMemo(
    () => searchHomeModels(props.models, query, 6),
    [props.models, query]
  )
  const showResults = isFocused && query.trim().length > 0

  let suggestions: ReactNode
  if (props.isLoading) {
    suggestions = (
      <div className='text-muted-foreground flex items-center gap-2 px-3 py-4 text-sm'>
        <Sparkles className='size-4 animate-pulse' />
        {t('Loading models...')}
      </div>
    )
  } else if (results.length > 0) {
    suggestions = results.map((model, index) => (
      <a
        key={model.model_name}
        id={`${listboxId}-${index}`}
        role='option'
        aria-selected={activeIndex === index}
        href={`/pricing/${encodeURIComponent(model.model_name)}`}
        onMouseEnter={() => setActiveIndex(index)}
        className={cn(
          'flex items-center gap-3 rounded-xl px-3 py-3 transition-colors',
          activeIndex === index ? 'bg-muted' : 'hover:bg-muted/70'
        )}
      >
        <span className='border-border/60 bg-muted/30 flex size-9 shrink-0 items-center justify-center rounded-lg border'>
          {getLobeIcon(model.icon || model.vendor_icon, 20)}
        </span>
        <span className='min-w-0 flex-1'>
          <span className='block truncate text-sm font-medium'>
            {model.model_name}
          </span>
          <span className='text-muted-foreground block truncate text-xs'>
            {model.vendor_name || t('Molii model')}
          </span>
        </span>
        <ArrowRight className='text-muted-foreground size-4' />
      </a>
    ))
  } else {
    suggestions = (
      <div className='text-muted-foreground px-3 py-4 text-sm'>
        {t('No matching models. Search the complete model marketplace.')}
      </div>
    )
  }

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    const normalizedQuery = query.trim()
    if (!normalizedQuery) {
      event.preventDefault()
      return
    }

    if (activeIndex >= 0 && results[activeIndex]) {
      event.preventDefault()
      window.location.assign(
        `/pricing/${encodeURIComponent(results[activeIndex].model_name)}`
      )
      return
    }

    const input = event.currentTarget.elements.namedItem('search')
    if (input instanceof HTMLInputElement) input.value = normalizedQuery
    if (props.onSearch) {
      event.preventDefault()
      props.onSearch(normalizedQuery)
    }
  }

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (!showResults || results.length === 0) {
      if (event.key === 'Escape') setIsFocused(false)
      return
    }

    if (event.key === 'ArrowDown') {
      event.preventDefault()
      setActiveIndex((current) => (current + 1) % results.length)
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      setActiveIndex((current) =>
        current <= 0 ? results.length - 1 : current - 1
      )
    } else if (event.key === 'Escape') {
      event.preventDefault()
      setIsFocused(false)
      setActiveIndex(-1)
    }
  }

  return (
    <div className='relative w-full max-w-3xl'>
      <form
        action='/pricing'
        method='get'
        role='search'
        onSubmit={handleSubmit}
        className='home-search-shell border-border/80 bg-background/90 focus-within:border-foreground/35 focus-within:ring-foreground/8 relative flex min-h-16 items-center gap-3 rounded-2xl border p-2 pl-5 shadow-[0_18px_60px_rgb(0_0_0/0.08)] backdrop-blur-xl transition-[border-color,box-shadow] focus-within:ring-4'
      >
        <Search className='text-muted-foreground size-5 shrink-0' />
        <input
          name='search'
          value={query}
          onChange={(event) => {
            setQuery(event.target.value)
            setActiveIndex(-1)
          }}
          onFocus={() => setIsFocused(true)}
          onBlur={() => window.setTimeout(() => setIsFocused(false), 120)}
          onKeyDown={handleKeyDown}
          autoComplete='off'
          aria-controls={showResults ? listboxId : undefined}
          aria-expanded={showResults}
          aria-activedescendant={
            activeIndex >= 0 ? `${listboxId}-${activeIndex}` : undefined
          }
          placeholder={t('Search models, providers, or capabilities')}
          className='placeholder:text-muted-foreground/55 min-w-0 flex-1 bg-transparent text-base outline-none md:text-lg'
        />
        <button
          type='submit'
          className='bg-foreground text-background hover:bg-foreground/88 inline-flex size-12 shrink-0 items-center justify-center rounded-xl transition-[background-color,transform]'
          aria-label={t('Search model marketplace')}
        >
          <ArrowRight className='size-5' />
        </button>
      </form>

      {showResults && (
        <div
          id={listboxId}
          role='listbox'
          className='border-border/70 bg-popover absolute top-[calc(100%+0.6rem)] right-0 left-0 z-30 overflow-hidden rounded-2xl border p-2 shadow-2xl'
        >
          {suggestions}
        </div>
      )}
    </div>
  )
}
