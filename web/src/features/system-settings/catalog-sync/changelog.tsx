import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { CatalogChange, CatalogChangeAction, CatalogEntry } from './types'

const actions: CatalogChangeAction[] = [
  'create',
  'update',
  'delete',
  'conflict',
  'preserve',
  'blocked',
  'adopt',
  'unchanged',
]
const actionLabels: Record<CatalogChangeAction, string> = {
  create: 'Create',
  update: 'Update',
  delete: 'Delete',
  conflict: 'Conflict',
  preserve: 'Preserve',
  blocked: 'Blocked',
  adopt: 'Adopt',
  unchanged: 'Unchanged',
}
const kindLabels: Record<CatalogChange['kind'], string> = {
  vendor: 'Vendor',
  model: 'Model metadata',
  model_price: 'Model price',
  plugin_price: 'Plugin price',
  special_price: 'Special price',
  tool_price: 'Tool price',
}
const reasonLabels: Record<string, string> = {
  target_only: 'Target-only item remains unchanged',
  already_absent: 'Item is already absent',
  local_modified: 'Target item was modified locally',
  source_removed: 'Item was removed from dev',
  source_matches_target: 'Dev and target values already match',
  managed_unchanged: 'Managed item has not changed',
  source_added: 'Item was added in dev',
  source_updated: 'Item was updated in dev',
  missing_object_version: 'Object version is unavailable',
  object_recreated: 'Target item was recreated',
  recreated_object_not_managed: 'Recreated item is not managed',
  preserved_price_currency_mismatch:
    'Preserved price uses a different currency',
  confirmation_unit_conflict: 'Related model changes need one choice',
  restore_preimage: 'Restore the saved prior value',
  vendor_still_referenced: 'Vendor is still referenced',
  price_fallback_unproven: 'Safe price fallback cannot be verified',
  reference_identity_unproven: 'Reference identity cannot be verified',
  enabled_route_reference: 'Enabled route still references this item',
  unfinished_task_reference: 'Unfinished task still references this item',
}
type Category = 'all' | 'vendor' | 'model' | 'price'

function parsedValue(entry: CatalogEntry | null): unknown {
  if (!entry) return null
  try {
    return JSON.parse(entry.value) as unknown
  } catch {
    return entry.value
  }
}

function flatten(value: unknown, prefix = ''): Record<string, string> {
  if (value === null || value === undefined) return {}
  if (typeof value !== 'object' || Array.isArray(value)) {
    return { [prefix || 'value']: String(value) }
  }
  const result: Record<string, string> = {}
  for (const [key, nested] of Object.entries(value)) {
    const name = prefix ? `${prefix}.${key}` : key
    if (
      nested !== null &&
      typeof nested === 'object' &&
      !Array.isArray(nested)
    ) {
      Object.assign(result, flatten(nested, name))
    } else {
      result[name] =
        typeof nested === 'string' ? nested : JSON.stringify(nested)
    }
  }
  return result
}

function displayKey(change: CatalogChange): string {
  if (change.kind === 'vendor' || change.kind === 'model') return change.key
  try {
    const key = JSON.parse(change.key) as {
      model?: string
      plugin?: string
      option?: string
      path?: string
    }
    return [key.model, key.plugin, key.option, key.path]
      .filter(Boolean)
      .join(' · ')
  } catch {
    return change.key
  }
}

function modelSearchText(change: CatalogChange): string {
  return `${displayKey(change)} ${change.key}`.toLocaleLowerCase()
}

function categoryOf(change: CatalogChange): Category {
  if (change.kind === 'vendor') return 'vendor'
  if (change.kind === 'model') return 'model'
  return 'price'
}

function FieldDiff(props: { change: CatalogChange }) {
  const { t } = useTranslation()
  const before = flatten(parsedValue(props.change.before))
  const base = flatten(parsedValue(props.change.base))
  const after = flatten(parsedValue(props.change.after))
  const names = [
    ...new Set([
      ...Object.keys(before),
      ...Object.keys(base),
      ...Object.keys(after),
    ]),
  ]
  const changed = names.filter((name) => {
    if (before[name] !== after[name] || props.change.action === 'conflict') {
      return true
    }
    const price = props.change.kind.endsWith('_price')
    return price && (name === 'billing_currency' || name === 'unit')
  })
  if (changed.length === 0) return <span>{t('No field values changed')}</span>
  return (
    <div className='flex max-w-full flex-col gap-2'>
      {changed.map((name) => (
        <div key={name} className='min-w-0 border-b pb-2 last:border-b-0'>
          <strong className='block break-all'>{name}</strong>
          <div className='grid gap-1 sm:grid-cols-3'>
            <div>
              <span className='text-muted-foreground'>{t('Current')}</span>
              <pre className='overflow-x-auto break-words whitespace-pre-wrap'>
                {before[name] ?? '—'}
              </pre>
            </div>
            <div>
              <span className='text-muted-foreground'>{t('Baseline')}</span>
              <pre className='overflow-x-auto break-words whitespace-pre-wrap'>
                {base[name] ?? '—'}
              </pre>
            </div>
            <div>
              <span className='text-muted-foreground'>{t('After sync')}</span>
              <pre className='overflow-x-auto break-words whitespace-pre-wrap'>
                {after[name] ?? '—'}
              </pre>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

type ChangelogProps = {
  changes: CatalogChange[]
  selectedUnits: string[]
  onSelectedUnitsChange: (units: string[]) => void
  disabled?: boolean
}

export function CatalogChangelog(props: ChangelogProps) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState<Category>('all')
  const visible = useMemo(
    () =>
      props.changes.filter(
        (change) =>
          (category === 'all' || categoryOf(change) === category) &&
          modelSearchText(change).includes(search.toLocaleLowerCase().trim())
      ),
    [props.changes, category, search]
  )
  const shownUnits = new Set<string>()
  const categories = [
    { value: 'all', label: t('All categories') },
    { value: 'vendor', label: t('Vendors') },
    { value: 'model', label: t('Model metadata') },
    { value: 'price', label: t('Prices') },
  ]
  return (
    <section
      aria-label={t('Catalog changelog')}
      className='flex flex-col gap-4'
    >
      <div className='flex flex-wrap gap-2'>
        <Input
          aria-label={t('Search models')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          className='max-w-xs'
        />
        <Select
          items={categories}
          value={category}
          onValueChange={(value) => {
            if (value) setCategory(value as Category)
          }}
        >
          <SelectTrigger aria-label={t('Filter category')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {categories.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
      {actions.map((action) => {
        const rows = visible.filter((change) => change.action === action)
        return (
          <section
            key={action}
            aria-label={t(actionLabels[action])}
            className='flex flex-col gap-2'
          >
            <h3 className='font-medium'>
              {t(actionLabels[action])}{' '}
              <Badge variant='secondary'>{rows.length}</Badge>
            </h3>
            {rows.length > 0 && (
              <StaticDataTable
                className='max-h-[32rem] overflow-auto'
                tableClassName='min-w-[720px]'
                data={rows}
                getRowKey={(row) => `${row.kind}:${row.key}`}
                columns={[
                  {
                    id: 'item',
                    header: t('Catalog item'),
                    cell: (change) => (
                      <div className='max-w-60 break-all'>
                        <strong>{displayKey(change)}</strong>
                        <p className='text-muted-foreground'>
                          {t(kindLabels[change.kind])}
                        </p>
                        <p className='text-muted-foreground'>
                          {t(reasonLabels[change.reason] ?? change.reason)}
                        </p>
                      </div>
                    ),
                  },
                  {
                    id: 'fields',
                    header: t('Field changes'),
                    cell: (change) => <FieldDiff change={change} />,
                  },
                  {
                    id: 'choice',
                    header: t('Conflict choice'),
                    cell: (change) => {
                      if (
                        change.action !== 'conflict' ||
                        shownUnits.has(change.confirmation_unit)
                      ) {
                        return null
                      }
                      shownUnits.add(change.confirmation_unit)
                      const unit = change.confirmation_unit
                      return (
                        <label className='flex items-center gap-2 text-sm'>
                          <Checkbox
                            checked={props.selectedUnits.includes(unit)}
                            disabled={props.disabled}
                            onCheckedChange={(checked) =>
                              props.onSelectedUnitsChange(
                                checked
                                  ? [...props.selectedUnits, unit]
                                  : props.selectedUnits.filter(
                                      (item) => item !== unit
                                    )
                              )
                            }
                            aria-label={`${t('Use dev for entire item')} ${displayKey(change)}`}
                          />
                          {t('Use dev for entire item')}
                        </label>
                      )
                    },
                  },
                ]}
              />
            )}
          </section>
        )
      })}
    </section>
  )
}
