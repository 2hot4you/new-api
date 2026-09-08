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
import { Search, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import type { PricingModel, PricingVendor } from '@/features/pricing/types'

type ProviderModelSelectorProps = {
  models: PricingModel[]
  vendors: PricingVendor[]
  selectedModelIds: string[]
  selectedGroup: string | null
  onSelectionChange: (modelIds: string[]) => void
}

type ProviderGroup = {
  key: string
  name: string
  models: PricingModel[]
}

function isAvailableForGroup(
  model: PricingModel,
  selectedGroup: string | null
): boolean {
  return (
    selectedGroup === null ||
    model.enable_groups.includes('all') ||
    model.enable_groups.includes(selectedGroup)
  )
}

export function ProviderModelSelector({
  models,
  vendors,
  selectedModelIds,
  selectedGroup,
  onSelectionChange,
}: ProviderModelSelectorProps) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const selected = useMemo(() => new Set(selectedModelIds), [selectedModelIds])
  const vendorById = useMemo(
    () => new Map(vendors.map((vendor) => [vendor.id, vendor])),
    [vendors]
  )
  const modelIds = useMemo(
    () => new Set(models.map((model) => model.model_name)),
    [models]
  )
  const normalizedSearch = search.trim().toLocaleLowerCase()
  const groups = useMemo(() => {
    const grouped = new Map<string, ProviderGroup>()
    for (const model of models) {
      const providerId = model.vendor_id ?? null
      const key = providerId === null ? 'unknown' : String(providerId)
      const name =
        (providerId === null ? null : vendorById.get(providerId)?.name) ??
        model.vendor_name ??
        t('Unknown provider')
      const haystack =
        `${name} ${model.model_name} ${model.display_name ?? ''}`.toLocaleLowerCase()
      if (normalizedSearch && !haystack.includes(normalizedSearch)) continue
      const group = grouped.get(key) ?? { key, name, models: [] }
      group.models.push(model)
      grouped.set(key, group)
    }
    return [...grouped.values()].sort((left, right) =>
      left.name.localeCompare(right.name)
    )
  }, [models, normalizedSearch, t, vendorById])
  const visibleModelIds = groups.flatMap((group) =>
    group.models.map((model) => model.model_name)
  )
  const missingIds = selectedModelIds.filter(
    (modelId) => !modelIds.has(modelId)
  )
  const allVisibleSelected =
    visibleModelIds.length > 0 &&
    visibleModelIds.every((modelId) => selected.has(modelId))
  const someVisibleSelected =
    !allVisibleSelected &&
    visibleModelIds.some((modelId) => selected.has(modelId))

  const setIdsSelected = (ids: string[], checked: boolean) => {
    const next = new Set(selectedModelIds)
    for (const id of ids) {
      if (checked) next.add(id)
      else next.delete(id)
    }
    onSelectionChange([...next])
  }

  return (
    <section
      aria-labelledby='quotation-model-selector-title'
      className='space-y-3'
    >
      <div className='flex flex-wrap items-end justify-between gap-2'>
        <div>
          <h3 id='quotation-model-selector-title' className='font-medium'>
            {t('Models')}
          </h3>
          <p className='text-muted-foreground text-xs tabular-nums'>
            {t('{{selected}} of {{total}} models selected', {
              selected: selectedModelIds.length,
              total: models.length,
            })}
          </p>
        </div>
        <div className='flex items-center gap-2 text-sm font-medium'>
          <Checkbox
            checked={allVisibleSelected}
            indeterminate={someVisibleSelected}
            disabled={visibleModelIds.length === 0}
            onCheckedChange={(checked) =>
              setIdsSelected(visibleModelIds, Boolean(checked))
            }
            aria-label={t('Select all models')}
          />
          {t('Select all')}
        </div>
      </div>

      <div className='relative'>
        <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
        <Input
          type='search'
          role='searchbox'
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          aria-label={t('Search models')}
          placeholder={t('Search models or providers')}
          className='pr-9 pl-8'
        />
        {search && (
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            onClick={() => setSearch('')}
            className='absolute top-1/2 right-0.5 -translate-y-1/2'
            aria-label={t('Clear search')}
          >
            <X />
          </Button>
        )}
      </div>

      <div className='max-h-[32rem] space-y-3 overflow-y-auto pr-1'>
        {groups.map((group) => {
          const groupIds = group.models.map((model) => model.model_name)
          const allSelected = groupIds.every((modelId) => selected.has(modelId))
          const someSelected =
            !allSelected && groupIds.some((modelId) => selected.has(modelId))
          return (
            <div key={group.key} className='overflow-hidden rounded-lg border'>
              <div className='bg-muted/40 flex items-center justify-between gap-3 border-b px-3 py-2'>
                <div className='min-w-0'>
                  <p className='truncate text-sm font-semibold'>{group.name}</p>
                  <p className='text-muted-foreground text-xs tabular-nums'>
                    {t('{{count}} models', { count: group.models.length })}
                  </p>
                </div>
                <Checkbox
                  checked={allSelected}
                  indeterminate={someSelected}
                  onCheckedChange={(checked) =>
                    setIdsSelected(groupIds, Boolean(checked))
                  }
                  aria-label={t('Select all models from {{provider}}', {
                    provider: group.name,
                  })}
                />
              </div>
              <div className='divide-y'>
                {group.models.map((model) => {
                  const groupAvailable = isAvailableForGroup(
                    model,
                    selectedGroup
                  )
                  return (
                    <div
                      key={model.model_name}
                      className='hover:bg-muted/30 flex items-start gap-3 px-3 py-2.5 transition-colors'
                    >
                      <Checkbox
                        checked={selected.has(model.model_name)}
                        onCheckedChange={(checked) =>
                          setIdsSelected([model.model_name], Boolean(checked))
                        }
                        aria-label={t('Select {{model}}', {
                          model: model.model_name,
                        })}
                      />
                      <span className='min-w-0 flex-1'>
                        {model.display_name &&
                          model.display_name !== model.model_name && (
                            <span className='block text-sm font-medium'>
                              {model.display_name}
                            </span>
                          )}
                        <span className='text-muted-foreground block font-mono text-xs break-all'>
                          {model.model_name}
                        </span>
                      </span>
                      {!groupAvailable && selected.has(model.model_name) && (
                        <Badge
                          variant='warning'
                          className='h-auto whitespace-normal'
                        >
                          {t('Unavailable for selected group')}
                        </Badge>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          )
        })}

        {groups.length === 0 && (
          <div className='text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm'>
            {t('No models match your search')}
          </div>
        )}

        {missingIds.length > 0 && !normalizedSearch && (
          <div className='border-warning/40 bg-warning/5 overflow-hidden rounded-lg border'>
            <div className='border-warning/30 border-b px-3 py-2'>
              <p className='text-sm font-semibold'>
                {t('Unavailable selections')}
              </p>
            </div>
            <div className='divide-warning/20 divide-y'>
              {missingIds.map((modelId) => (
                <div
                  key={modelId}
                  className='flex items-start gap-3 px-3 py-2.5'
                >
                  <Checkbox
                    checked
                    onCheckedChange={(checked) =>
                      setIdsSelected([modelId], Boolean(checked))
                    }
                    aria-label={t('Select {{model}}', { model: modelId })}
                  />
                  <span className='min-w-0 flex-1 font-mono text-xs break-all'>
                    {modelId}
                  </span>
                  <Badge variant='warning' className='h-auto whitespace-normal'>
                    {t('Missing from current pricing')}
                  </Badge>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </section>
  )
}
