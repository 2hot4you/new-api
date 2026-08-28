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
import { Cancel01Icon, Drag01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Check, ChevronsUpDown, Search } from 'lucide-react'
import { Reorder, useDragControls } from 'motion/react'
import {
  useMemo,
  useId,
  useState,
  type ComponentProps,
  type KeyboardEvent,
  type PointerEvent,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { GroupSuccessRateBadge } from '../hooks/use-group-success-rates'
import { GroupRatioBadge } from './auto-group-visuals'

export type ApiKeyGroupOption = {
  value: string
  label: string
  desc?: string
  ratio?: number | string
  icon?: string
  successRate?: number | null
  providers?: ApiKeyGroupProvider[]
}

export type ApiKeyGroupProvider = {
  id: number
  name: string
  icon?: string
  display_order?: number
}

type ApiKeyGroupProviderSection = {
  provider?: ApiKeyGroupProvider
  options: ApiKeyGroupOption[]
}

type AutoGroupOrderEditorProps = Omit<ComponentProps<'div'>, 'onChange'> & {
  value: string[]
  mode: 'inherit' | 'custom'
  options: ApiKeyGroupOption[]
  globalOptions: ApiKeyGroupOption[]
  maxCount: number
  onChange: (value: { groups: string[]; mode: 'inherit' | 'custom' }) => void
  'data-slot'?: string
  'data-form-root'?: string
}

type AutoGroupOrderItemProps = {
  option: ApiKeyGroupOption
  index: number
  onMove: (index: number, direction: 'up' | 'down') => void
  onRemove: (group: string) => void
}

function CompactRatioBadge(props: { ratio?: number | string }) {
  const { t } = useTranslation()
  if (props.ratio === undefined || props.ratio === null || props.ratio === '') {
    return null
  }

  const label =
    typeof props.ratio === 'number'
      ? `${props.ratio}x`
      : `${t('Auto')} ${t('Ratio')}`
  return (
    <Badge
      variant='outline'
      aria-label={`${label} ${t('Ratio')}`}
      className='max-w-16 truncate px-1.5 text-[10px]'
    >
      {label}
    </Badge>
  )
}

function AutoGroupOrderItem(props: AutoGroupOrderItemProps) {
  const { t } = useTranslation()
  const dragControls = useDragControls()

  const handleDragStart = (event: PointerEvent<HTMLButtonElement>) => {
    dragControls.start(event)
  }

  const handleDragKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      props.onMove(props.index, 'up')
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      props.onMove(props.index, 'down')
    }
  }

  return (
    <Reorder.Item
      value={props.option.value}
      dragListener={false}
      dragControls={dragControls}
      data-selected-group-item={props.option.value}
      className='bg-background flex min-w-0 items-center gap-2 overflow-hidden rounded-lg border p-2'
    >
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        className='text-muted-foreground cursor-grab touch-none font-mono active:cursor-grabbing'
        aria-label={t('Drag {{group}} to reorder', {
          group: props.option.label,
        })}
        aria-keyshortcuts='ArrowUp ArrowDown'
        onPointerDown={handleDragStart}
        onKeyDown={handleDragKeyDown}
      >
        <HugeiconsIcon icon={Drag01Icon} strokeWidth={2} aria-hidden='true' />
      </Button>
      <span className='bg-primary/10 text-primary flex size-5 shrink-0 items-center justify-center rounded-full text-xs font-semibold tabular-nums'>
        {props.index + 1}
      </span>
      {props.option.icon && (
        <span className='shrink-0' aria-hidden='true'>
          {getLobeIcon(props.option.icon, 20)}
        </span>
      )}
      <span className='min-w-0 flex-1 overflow-hidden'>
        <span
          data-selected-group-name-line='true'
          className='grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-1.5 overflow-hidden'
        >
          <span className='min-w-0 truncate text-sm font-medium'>
            {props.option.label}
          </span>
          <span
            data-selected-group-metadata='true'
            className='flex min-w-0 shrink-0 items-center gap-1 overflow-hidden'
          >
            <GroupSuccessRateBadge successRate={props.option.successRate} />
            <CompactRatioBadge ratio={props.option.ratio} />
          </span>
        </span>
        {props.option.desc && (
          <span className='text-muted-foreground block truncate text-xs'>
            {props.option.desc}
          </span>
        )}
      </span>
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        className='shrink-0'
        aria-label={t('Remove {{group}}', { group: props.option.label })}
        onClick={() => props.onRemove(props.option.value)}
      >
        <HugeiconsIcon icon={Cancel01Icon} strokeWidth={2} aria-hidden='true' />
      </Button>
    </Reorder.Item>
  )
}

export function AutoGroupOrderEditor(props: AutoGroupOrderEditorProps) {
  const { t } = useTranslation()
  const routingDescriptionId = useId()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const maxCount =
    Number.isInteger(props.maxCount) && props.maxCount > 0 ? props.maxCount : 5
  const options = useMemo(
    () => props.options.filter((option) => option.value !== 'auto'),
    [props.options]
  )
  const optionsByValue = useMemo(
    () => new Map(options.map((option) => [option.value, option])),
    [options]
  )
  const selectedValues = useMemo(
    () => props.value.filter((group) => optionsByValue.has(group)),
    [props.value, optionsByValue]
  )
  const selectedOptions = useMemo(
    () =>
      selectedValues.flatMap((group) => {
        const option = optionsByValue.get(group)
        return option ? [option] : []
      }),
    [optionsByValue, selectedValues]
  )
  const defaultGroupValues = useMemo(
    () => props.globalOptions.slice(0, maxCount).map((option) => option.value),
    [maxCount, props.globalOptions]
  )
  const filteredOptions = useMemo(() => {
    const search = searchValue.trim().toLowerCase()
    if (!search) return options
    return options.filter((option) => {
      const ratio = String(option.ratio ?? '').toLowerCase()
      const providerMatches = option.providers?.some((provider) =>
        provider.name.toLowerCase().includes(search)
      )
      return (
        option.value.toLowerCase().includes(search) ||
        option.label.toLowerCase().includes(search) ||
        option.desc?.toLowerCase().includes(search) ||
        ratio.includes(search) ||
        providerMatches
      )
    })
  }, [options, searchValue])
  const providerSections = useMemo<ApiKeyGroupProviderSection[]>(() => {
    const providersByID = new Map<number, ApiKeyGroupProvider>()
    for (const option of filteredOptions) {
      for (const provider of option.providers ?? []) {
        const current = providersByID.get(provider.id)
        if (
          !current ||
          (provider.display_order ?? Number.MAX_SAFE_INTEGER) <
            (current.display_order ?? Number.MAX_SAFE_INTEGER)
        ) {
          providersByID.set(provider.id, provider)
        }
      }
    }
    const providers = [...providersByID.values()].sort(
      (left, right) =>
        (left.display_order ?? Number.MAX_SAFE_INTEGER) -
          (right.display_order ?? Number.MAX_SAFE_INTEGER) ||
        left.name.localeCompare(right.name) ||
        left.id - right.id
    )
    const sections: ApiKeyGroupProviderSection[] = providers.map(
      (provider) => ({
        provider,
        options: filteredOptions.filter((option) =>
          option.providers?.some((candidate) => candidate.id === provider.id)
        ),
      })
    )
    const uncategorized = filteredOptions.filter(
      (option) => !option.providers || option.providers.length === 0
    )
    if (uncategorized.length > 0) {
      sections.push({ options: uncategorized })
    }
    return sections
  }, [filteredOptions])

  const emitCustomGroups = (groups: string[]) => {
    props.onChange({ groups, mode: 'custom' })
  }

  const handleToggle = (group: string) => {
    if (selectedValues.includes(group)) {
      emitCustomGroups(selectedValues.filter((item) => item !== group))
      return
    }
    if (selectedValues.length >= maxCount) return
    emitCustomGroups([...selectedValues, group])
  }

  const handleMove = (index: number, direction: 'up' | 'down') => {
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= selectedValues.length) return
    const next = [...selectedValues]
    ;[next[index], next[targetIndex]] = [next[targetIndex], next[index]]
    emitCustomGroups(next)
  }

  let routingDescription = t('No groups selected')
  if (selectedOptions.length === 1) {
    routingDescription = t('One group uses fixed routing')
  } else if (selectedOptions.length > 1) {
    routingDescription = t('{{count}} groups will be tried in order', {
      count: selectedOptions.length,
    })
  }

  return (
    <div
      id={props.id}
      data-slot={props['data-slot']}
      data-form-root={props['data-form-root']}
      role='group'
      tabIndex={-1}
      aria-label={props['aria-label'] || t('Group selection order')}
      aria-describedby={props['aria-describedby']}
      aria-invalid={props['aria-invalid']}
      className={cn(
        'flex w-full min-w-0 max-w-full flex-col gap-3 overflow-x-hidden',
        props.className
      )}
    >
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <Button
              type='button'
              variant='outline'
              role='combobox'
              aria-label={
                selectedOptions.length > 0
                  ? `${t('Group selection order')}: ${selectedOptions
                      .map((option) => option.label)
                      .join(' → ')}`
                  : t('Group selection order')
              }
              aria-describedby={routingDescriptionId}
              aria-expanded={open}
              className='h-auto min-h-14 w-full justify-between gap-3 rounded-lg px-4 py-3 text-start shadow-none'
            />
          }
        >
          <span className='min-w-0 flex-1'>
            <span className='block truncate font-medium'>
              {selectedOptions.length > 0
                ? selectedOptions.map((option) => option.label).join(' → ')
                : t('Select groups')}
            </span>
            <span
              id={routingDescriptionId}
              className='text-muted-foreground block text-xs'
            >
              {routingDescription}
            </span>
          </span>
          <ChevronsUpDown
            aria-hidden='true'
            className='size-4 shrink-0 opacity-50'
          />
        </PopoverTrigger>
        <PopoverContent
          className='w-[var(--anchor-width)] overflow-hidden rounded-xl p-0 shadow-lg'
          onWheel={(event) => event.stopPropagation()}
          onTouchMove={(event) => event.stopPropagation()}
        >
          <div className='border-b p-2'>
            <div className='relative'>
              <Search
                aria-hidden='true'
                className='text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2'
              />
              <Input
                placeholder={t('Search groups...')}
                aria-label={t('Search groups...')}
                value={searchValue}
                onChange={(event) => setSearchValue(event.target.value)}
                className='h-9 pl-8 shadow-none'
              />
            </div>
          </div>
          <div
            role='group'
            aria-label={t('Select groups')}
            className='max-h-[360px] overflow-y-auto p-1'
          >
            {providerSections.length === 0 && (
              <p className='text-muted-foreground py-6 text-center text-sm'>
                {t('No group found.')}
              </p>
            )}
            {providerSections.map((section) => {
              const sectionLabel = section.provider?.name ?? t('Uncategorized')
              return (
                <section
                  key={section.provider?.id ?? 'uncategorized'}
                  data-group-provider-section={sectionLabel}
                  aria-label={sectionLabel}
                  className='not-first:border-t'
                >
                  <div className='bg-muted/40 text-muted-foreground flex items-center gap-2 px-3 py-2 text-xs font-medium'>
                    {section.provider && (
                      <span
                        data-provider-icon-key={section.provider.icon ?? ''}
                        className='shrink-0'
                        aria-hidden='true'
                      >
                        {getLobeIcon(
                          section.provider.icon || section.provider.name,
                          16
                        )}
                      </span>
                    )}
                    <span className='min-w-0 flex-1 truncate'>
                      {sectionLabel}
                    </span>
                    <Badge variant='outline' className='px-1.5 text-[10px]'>
                      {section.options.length}
                    </Badge>
                  </div>
                  {section.options.map((option) => {
                    const selected = selectedValues.includes(option.value)
                    const disabled =
                      !selected && selectedValues.length >= maxCount
                    return (
                      <button
                        key={`${section.provider?.id ?? 'uncategorized'}-${option.value}`}
                        type='button'
                        role='checkbox'
                        aria-checked={selected}
                        aria-label={t('Select {{group}}', {
                          group: option.label,
                        })}
                        disabled={disabled}
                        data-group-selection-checkbox={option.value}
                        onClick={() => handleToggle(option.value)}
                        className='hover:bg-accent focus-visible:ring-ring flex w-full items-start gap-3 rounded-lg px-3 py-3 text-left outline-none focus-visible:ring-2 disabled:pointer-events-none disabled:opacity-50'
                      >
                        <span
                          aria-hidden='true'
                          className={cn(
                            'border-input mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-[4px] border',
                            selected &&
                              'bg-primary text-primary-foreground border-primary'
                          )}
                        >
                          {selected && (
                            <Check className='size-3' strokeWidth={3} />
                          )}
                        </span>
                        {option.icon && (
                          <span className='mt-0.5 shrink-0' aria-hidden='true'>
                            {getLobeIcon(option.icon, 20)}
                          </span>
                        )}
                        <span className='min-w-0 flex-1'>
                          <span className='block truncate font-medium'>
                            {option.label}
                          </span>
                          {option.desc && (
                            <span className='text-muted-foreground block truncate text-xs'>
                              {option.desc}
                            </span>
                          )}
                        </span>
                        <GroupRatioBadge ratio={option.ratio} />
                        <GroupSuccessRateBadge
                          successRate={option.successRate}
                        />
                      </button>
                    )
                  })}
                </section>
              )
            })}
          </div>
        </PopoverContent>
      </Popover>

      <div className='flex items-center justify-between gap-3'>
        <p className='text-muted-foreground text-xs' aria-live='polite'>
          {t('{{count}} / {{max}} groups selected', {
            count: selectedOptions.length,
            max: maxCount,
          })}
        </p>
        {props.globalOptions.length > 0 && (
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={
              selectedValues.length === defaultGroupValues.length &&
              selectedValues.every(
                (group, index) => group === defaultGroupValues[index]
              )
            }
            onClick={() =>
              props.onChange({
                groups: defaultGroupValues,
                mode: 'custom',
              })
            }
          >
            {t('Use system default order')}
          </Button>
        )}
      </div>

      {selectedOptions.length > 0 && (
        <Reorder.Group
          axis='y'
          values={selectedValues}
          onReorder={emitCustomGroups}
          className='flex w-full max-w-full min-w-0 flex-col gap-2 overflow-x-hidden'
          aria-label={t('Selected group priority')}
        >
          {selectedOptions.map((option, index) => (
            <AutoGroupOrderItem
              key={option.value}
              option={option}
              index={index}
              onMove={handleMove}
              onRemove={handleToggle}
            />
          ))}
        </Reorder.Group>
      )}
    </div>
  )
}
