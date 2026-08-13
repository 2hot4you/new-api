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
import { ChevronDown, RotateCcw } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import {
  ENDPOINT_TYPES,
  FILTER_ALL,
  QUOTA_TYPES,
  getEndpointTypeLabels,
  getQuotaTypeLabels,
} from '../constants'
import { parseTags } from '../lib/filters'
import {
  getContextBuckets,
  getContextBucketId,
  getModelInputModalities,
} from '../lib/model-directory'
import type {
  Modality,
  ModelCapability,
  PricingModel,
  PricingVendor,
} from '../types'

type FilterOption = {
  value: string
  label: string
  count?: number
  suffix?: string
  icon?: ReactNode
}

type FilterSectionProps = {
  id: string
  title: string
  value: string
  options: FilterOption[]
  onChange: (value: string) => void
}

export interface PricingSidebarProps {
  quotaTypeFilter: string
  endpointTypeFilter: string
  vendorFilter: string
  groupFilter: string
  tagFilter: string
  inputModalityFilter: string
  contextFilter: string
  capabilityFilter: string
  onQuotaTypeChange: (value: string) => void
  onEndpointTypeChange: (value: string) => void
  onVendorChange: (value: string) => void
  onGroupChange: (value: string) => void
  onTagChange: (value: string) => void
  onInputModalityChange: (value: string) => void
  onContextChange: (value: string) => void
  onCapabilityChange: (value: string) => void
  vendors: PricingVendor[]
  groups: string[]
  groupRatios?: Record<string, number>
  tags: string[]
  models: PricingModel[]
  hasActiveFilters: boolean
  onClearFilters: () => void
  className?: string
}

function countBy(
  models: PricingModel[],
  predicate: (model: PricingModel) => boolean
): number {
  return models.reduce((count, model) => count + (predicate(model) ? 1 : 0), 0)
}

function formatGroupRatio(ratio: number | undefined): string | undefined {
  if (ratio == null) return undefined
  const formatted = Number.isInteger(ratio)
    ? ratio.toString()
    : ratio.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
  return `x${formatted}`
}

function FilterOptionButton(props: {
  option: FilterOption
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      type='button'
      onClick={props.onClick}
      className={cn(
        'group flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs font-medium transition-colors',
        props.active
          ? 'bg-foreground/8 text-foreground'
          : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
      )}
      title={props.option.label}
    >
      {props.option.icon && (
        <span className='shrink-0'>{props.option.icon}</span>
      )}
      <span className='min-w-0 flex-1 truncate'>{props.option.label}</span>
      {(props.option.suffix || props.option.count != null) && (
        <span
          className={cn(
            'ml-auto shrink-0 rounded px-1.5 py-0.5 text-[11px] tabular-nums',
            props.active
              ? 'bg-background text-foreground'
              : 'bg-muted text-muted-foreground'
          )}
        >
          {props.option.suffix ?? props.option.count}
        </span>
      )}
    </button>
  )
}

function FilterSection(props: FilterSectionProps) {
  return (
    <Collapsible
      defaultOpen
      data-filter-section={props.id}
      className='border-border/70 border-b pb-3 last:border-b-0'
    >
      <CollapsibleTrigger className='group flex w-full items-center justify-between py-2.5 text-left'>
        <span className='text-foreground text-sm font-semibold'>
          {props.title}
        </span>
        <ChevronDown className='text-muted-foreground size-4 transition-transform group-data-[panel-open]:rotate-180' />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className='space-y-0.5'>
          {props.options.map((option) => (
            <FilterOptionButton
              key={option.value}
              option={option}
              active={props.value === option.value}
              onClick={() => props.onChange(option.value)}
            />
          ))}
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}

export function PricingSidebar(props: PricingSidebarProps) {
  const { t } = useTranslation()
  const quotaTypeLabels = getQuotaTypeLabels(t)
  const endpointTypeLabels = getEndpointTypeLabels(t)

  const vendorOptions: FilterOption[] = [
    {
      value: FILTER_ALL,
      label: t('All Vendors'),
      count: props.models.length,
    },
    ...props.vendors
      .map((vendor) => ({
        value: vendor.name,
        label: vendor.name,
        count: countBy(
          props.models,
          (model) => model.vendor_name === vendor.name
        ),
        icon: vendor.icon ? getLobeIcon(vendor.icon, 14) : undefined,
      }))
      .filter((vendor) => vendor.count > 0),
  ]

  const groupOptions: FilterOption[] = [
    {
      value: FILTER_ALL,
      label: t('All Groups'),
    },
    ...props.groups.map((group) => ({
      value: group,
      label: group,
      suffix: formatGroupRatio(props.groupRatios?.[group]),
    })),
  ]

  const quotaOptions: FilterOption[] = [
    {
      value: QUOTA_TYPES.ALL,
      label: quotaTypeLabels[QUOTA_TYPES.ALL],
      count: props.models.length,
    },
    {
      value: QUOTA_TYPES.TOKEN,
      label: quotaTypeLabels[QUOTA_TYPES.TOKEN],
      count: countBy(
        props.models,
        (model) =>
          model.quota_type === 0 && model.billing_mode !== 'tiered_expr'
      ),
    },
    {
      value: QUOTA_TYPES.REQUEST,
      label: quotaTypeLabels[QUOTA_TYPES.REQUEST],
      count: countBy(props.models, (model) => model.quota_type === 1),
    },
    {
      value: QUOTA_TYPES.DYNAMIC,
      label: quotaTypeLabels[QUOTA_TYPES.DYNAMIC],
      count: countBy(
        props.models,
        (model) => model.billing_mode === 'tiered_expr'
      ),
    },
  ].filter((option) => option.value === QUOTA_TYPES.ALL || option.count > 0)

  const tagOptions: FilterOption[] = [
    {
      value: FILTER_ALL,
      label: t('All Tags'),
      count: props.models.length,
    },
    ...props.tags.map((tag) => ({
      value: tag,
      label: tag,
      count: countBy(props.models, (model) =>
        parseTags(model.tags)
          .map((item) => item.toLowerCase())
          .includes(tag.toLowerCase())
      ),
    })),
  ]

  const endpointOptions: FilterOption[] = [
    {
      value: ENDPOINT_TYPES.ALL,
      label: endpointTypeLabels[ENDPOINT_TYPES.ALL],
      count: props.models.length,
    },
    ...Object.entries(endpointTypeLabels)
      .filter(([value]) => value !== ENDPOINT_TYPES.ALL)
      .map(([value, label]) => ({
        value,
        label,
        count: countBy(
          props.models,
          (model) => model.supported_endpoint_types?.includes(value) ?? false
        ),
      })),
  ].filter((option) => option.value === ENDPOINT_TYPES.ALL || option.count > 0)

  const inputModalities = new Set<Modality>()
  const capabilities = new Set<ModelCapability>()
  for (const model of props.models) {
    for (const modality of getModelInputModalities(model)) {
      inputModalities.add(modality)
    }
    for (const capability of model.capabilities ?? []) {
      capabilities.add(capability)
    }
  }

  const inputLabels: Record<Modality, string> = {
    text: t('Text'),
    image: t('Image'),
    file: t('File'),
    audio: t('Audio'),
    video: t('Video'),
  }
  const inputOptions: FilterOption[] = [
    {
      value: FILTER_ALL,
      label: t('All Input Types'),
      count: props.models.length,
    },
    ...(['text', 'image', 'file', 'audio', 'video'] as Modality[])
      .filter((modality) => inputModalities.has(modality))
      .map((modality) => ({
        value: modality,
        label: inputLabels[modality],
        count: countBy(props.models, (model) =>
          getModelInputModalities(model).includes(modality)
        ),
      })),
  ]

  const contextLabels: Record<string, string> = {
    'lte-128k': '≤ 128K',
    'lte-256k': '128K–256K',
    'lte-1m': '256K–1M',
    'gt-1m': '> 1M',
  }
  const contextOptions: FilterOption[] = [
    {
      value: FILTER_ALL,
      label: t('All Context Lengths'),
      count: countBy(
        props.models,
        (model) => getContextBucketId(model.context_length) != null
      ),
    },
    ...getContextBuckets(props.models).map((bucket) => ({
      value: bucket.id,
      label: contextLabels[bucket.id],
      count: bucket.count,
    })),
  ]

  const capabilityLabels: Partial<Record<ModelCapability, string>> = {
    reasoning: t('Reasoning'),
    tools: t('Tools'),
    function_calling: t('Function calling'),
    structured_output: t('Structured output'),
    streaming: t('Streaming'),
    vision: t('Vision'),
    image_generation: t('Image generation'),
    image_editing: t('Image editing'),
    video_generation: t('Video generation'),
    video_editing: t('Video editing'),
    audio_generation: t('Audio generation'),
    web_search: t('Web search'),
    code_interpreter: t('Code interpreter'),
    caching: t('Prompt caching'),
    embeddings: t('Embeddings'),
    system_prompt: t('System prompt'),
    json_mode: t('JSON mode'),
  }
  const capabilityOptions: FilterOption[] = [
    {
      value: FILTER_ALL,
      label: t('All Capabilities'),
      count: props.models.length,
    },
    ...[...capabilities]
      .map((capability) => ({
        value: capability,
        label: capabilityLabels[capability] ?? capability,
        count: countBy(
          props.models,
          (model) => model.capabilities?.includes(capability) ?? false
        ),
      }))
      .sort((left, right) => left.label.localeCompare(right.label)),
  ]

  return (
    <aside className={cn('border-r px-3 py-4', props.className)}>
      <div className='mb-2.5 flex items-center justify-between gap-2'>
        <div>
          <h2 className='text-foreground text-sm font-bold'>{t('Filter')}</h2>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Refine models by provider, group, type, and tags.')}
          </p>
        </div>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={props.onClearFilters}
          disabled={!props.hasActiveFilters}
          className='h-7 gap-1.5 px-2 text-xs'
        >
          <RotateCcw className='size-3.5' />
          {t('Reset')}
        </Button>
      </div>

      {props.hasActiveFilters && (
        <Badge variant='secondary' className='mb-3'>
          {t('Filters active')}
        </Badge>
      )}

      <div className='space-y-1'>
        <FilterSection
          id='input-types'
          title={t('Input Types')}
          value={props.inputModalityFilter}
          options={inputOptions}
          onChange={props.onInputModalityChange}
        />
        <FilterSection
          id='context-length'
          title={t('Context Length')}
          value={props.contextFilter}
          options={contextOptions}
          onChange={props.onContextChange}
        />
        <FilterSection
          id='vendors'
          title={t('Vendors')}
          value={props.vendorFilter}
          options={vendorOptions}
          onChange={props.onVendorChange}
        />
        <FilterSection
          id='capabilities'
          title={t('Supported Capabilities')}
          value={props.capabilityFilter}
          options={capabilityOptions}
          onChange={props.onCapabilityChange}
        />
        <FilterSection
          id='endpoint-types'
          title={t('Supported Protocols')}
          value={props.endpointTypeFilter}
          options={endpointOptions}
          onChange={props.onEndpointTypeChange}
        />
        <FilterSection
          id='pricing-types'
          title={t('Pricing Type')}
          value={props.quotaTypeFilter}
          options={quotaOptions}
          onChange={props.onQuotaTypeChange}
        />
        <FilterSection
          id='groups'
          title={t('Groups')}
          value={props.groupFilter}
          options={groupOptions}
          onChange={props.onGroupChange}
        />
        <FilterSection
          id='tags'
          title={t('Model Tags')}
          value={props.tagFilter}
          options={tagOptions}
          onChange={props.onTagChange}
        />
      </div>
    </aside>
  )
}
