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
import {
  ArrowRight,
  CalendarClock,
  CheckCircle2,
  Layers,
  Maximize2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { getModelModalityLabelKey } from '../lib/model-helpers'
import type { ModelCapability, PricingModel } from '../types'

const CAPABILITY_LABEL_KEYS: Record<ModelCapability, string> = {
  function_calling: 'Function calling',
  streaming: 'Streaming',
  vision: 'Vision',
  json_mode: 'JSON mode',
  structured_output: 'Structured output',
  reasoning: 'Reasoning',
  tools: 'Tools',
  system_prompt: 'System prompt',
  web_search: 'Web search',
  code_interpreter: 'Code interpreter',
  caching: 'Prompt caching',
  embeddings: 'Embeddings',
  image_generation: 'Image generation',
  image_editing: 'Image editing',
  video_generation: 'Video generation',
  video_editing: 'Video editing',
  audio_generation: 'Audio generation',
}

const TOKEN_FORMAT = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 1,
})

function normalizeItems(items?: readonly string[]): string[] {
  return (items ?? []).filter((item) => item.trim().length > 0)
}

function formatTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return ''
  if (tokens >= 1_000_000) {
    return `${TOKEN_FORMAT.format(tokens / 1_000_000)}M`
  }
  if (tokens >= 1_000) {
    return `${TOKEN_FORMAT.format(tokens / 1_000)}K`
  }
  return TOKEN_FORMAT.format(tokens)
}

function formatCatalogDate(value?: string): string {
  if (!value) return ''
  const date = new Date(
    `${value.length === 7 ? `${value}-01` : value}T00:00:00Z`
  )
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    ...(value.length > 7 ? { day: 'numeric' as const } : {}),
    timeZone: 'UTC',
  })
}

function formatUpdatedTime(value?: number): string {
  if (!value || !Number.isFinite(value) || value <= 0) return ''
  return new Date(value * 1000).toLocaleDateString()
}

function ModalityValue(props: { items: string[] }) {
  const { t } = useTranslation()

  return (
    <span className='text-foreground flex min-w-0 flex-wrap gap-x-1.5 text-sm font-semibold'>
      {props.items.map((item) => (
        <span key={item}>{t(getModelModalityLabelKey(item))}</span>
      ))}
    </span>
  )
}

export function ModelDetailsCapabilities(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const model = props.model
  const capabilities = normalizeItems(model.capabilities)
  const inputModalities = normalizeItems(model.input_modalities)
  const outputModalities = normalizeItems(model.output_modalities)

  const coreSpecs: {
    key: string
    label: string
    value: string
    icon: React.ComponentType<{ className?: string }>
  }[] = []

  const contextLength = formatTokenCount(model.context_length ?? 0)
  if (contextLength) {
    coreSpecs.push({
      key: 'context',
      label: t('Context'),
      value: contextLength,
      icon: Layers,
    })
  }

  const maxOutput = formatTokenCount(model.max_output_tokens ?? 0)
  if (maxOutput) {
    coreSpecs.push({
      key: 'max-output',
      label: t('Max output'),
      value: maxOutput,
      icon: Maximize2,
    })
  }

  const timelineValue = formatCatalogDate(
    model.release_date || model.knowledge_cutoff
  )
  if (timelineValue) {
    coreSpecs.push({
      key: model.release_date ? 'release' : 'knowledge',
      label: model.release_date ? t('Released') : t('Knowledge cutoff'),
      value: timelineValue,
      icon: CalendarClock,
    })
  }

  const hasModalities =
    inputModalities.length > 0 || outputModalities.length > 0
  const hasPrimaryContent =
    coreSpecs.length > 0 || hasModalities || capabilities.length > 0
  const updatedTime = formatUpdatedTime(model.metadata_updated_time)
  const metadataRows: Array<{ label: string; values: string[] }> = []
  const addMetadataRow = (label: string, values?: readonly string[]) => {
    const normalized = normalizeItems(values)
    if (normalized.length > 0) metadataRows.push({ label, values: normalized })
  }
  addMetadataRow(t('Supported parameters'), model.supported_parameters)
  addMetadataRow(t('Supported resolutions'), model.supported_resolutions)
  addMetadataRow(t('Supported aspect ratios'), model.supported_aspect_ratios)
  if ((model.max_input_images ?? 0) > 0) {
    metadataRows.push({
      label: t('Maximum input images'),
      values: [String(model.max_input_images)],
    })
  }
  addMetadataRow(t('Output formats'), model.output_formats)
  if ((model.min_duration ?? 0) > 0 || (model.max_duration ?? 0) > 0) {
    const minimum = model.min_duration ?? model.max_duration
    const maximum = model.max_duration ?? model.min_duration
    metadataRows.push({
      label: t('Duration'),
      values: [minimum === maximum ? `${minimum}s` : `${minimum}–${maximum}s`],
    })
  }
  addMetadataRow(
    t('Reference modalities'),
    model.reference_modalities?.map((item) => t(getModelModalityLabelKey(item)))
  )
  let coreSpecsGridClass = 'grid-cols-1'
  if (coreSpecs.length >= 3) {
    coreSpecsGridClass = 'grid-cols-2 @2xl/details:grid-cols-3'
  } else if (coreSpecs.length === 2) {
    coreSpecsGridClass = 'grid-cols-2'
  }

  if (!hasPrimaryContent) return null

  return (
    <section
      className='overflow-hidden rounded-xl border'
      data-model-capabilities-card='true'
    >
      <div className='flex items-center justify-between gap-3 px-4 py-3'>
        <h2 className='text-foreground text-sm font-semibold'>
          {t('Capabilities')}
        </h2>
        <span className='text-muted-foreground text-[11px]'>
          {t('Supported modalities')}
        </span>
      </div>

      {coreSpecs.length > 0 && (
        <div
          className={`grid divide-x border-t ${coreSpecsGridClass}`}
          data-model-core-specs='true'
        >
          {coreSpecs.map((spec) => {
            const Icon = spec.icon
            return (
              <div
                key={spec.key}
                className='bg-muted/10 flex min-w-0 items-center gap-3 px-4 py-3'
              >
                <div className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-lg'>
                  <Icon className='size-4' />
                </div>
                <div className='min-w-0'>
                  <div className='text-muted-foreground truncate text-[11px] font-medium'>
                    {spec.label}
                  </div>
                  <div className='text-foreground truncate font-mono text-base font-semibold tabular-nums'>
                    {spec.value}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {hasModalities && (
        <div
          className='flex flex-wrap items-center gap-3 border-t px-4 py-3'
          data-model-modalities='true'
        >
          <div className='flex min-w-0 flex-1 items-center gap-2'>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('Input')}
            </span>
            <ModalityValue items={inputModalities} />
          </div>
          {inputModalities.length > 0 && outputModalities.length > 0 && (
            <ArrowRight className='text-muted-foreground/50 size-4 shrink-0' />
          )}
          <div className='flex min-w-0 flex-1 items-center justify-end gap-2'>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('Output')}
            </span>
            <ModalityValue items={outputModalities} />
          </div>
        </div>
      )}

      {capabilities.length > 0 && (
        <div className='grid grid-cols-2 gap-2 border-t p-3 @2xl/details:grid-cols-3'>
          {capabilities.map((capability) => (
            <div
              key={capability}
              className='bg-muted/30 flex min-w-0 items-center gap-2 rounded-lg px-3 py-2.5'
              data-model-capability='true'
            >
              <CheckCircle2 className='text-primary size-4 shrink-0' />
              <span className='text-foreground truncate text-xs font-medium'>
                {t(
                  CAPABILITY_LABEL_KEYS[capability as ModelCapability] ??
                    capability
                )}
              </span>
            </div>
          ))}
        </div>
      )}

      {metadataRows.length > 0 && (
        <div className='bg-border grid gap-px border-t sm:grid-cols-2'>
          {metadataRows.map((row) => (
            <div
              key={row.label}
              className='bg-background min-w-0 px-4 py-3'
              data-model-capability-metadata='true'
            >
              <div className='text-muted-foreground text-[11px] font-medium'>
                {row.label}
              </div>
              <div className='mt-1 flex flex-wrap gap-1.5'>
                {row.values.map((value) => (
                  <span
                    key={value}
                    className='bg-muted/60 text-foreground rounded px-2 py-1 font-mono text-[11px]'
                  >
                    {value}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {updatedTime && (
        <div
          className='text-muted-foreground/70 flex flex-wrap items-center gap-x-2 gap-y-1 border-t px-4 py-2 text-[10px]'
          data-model-metadata-note='true'
        >
          <CalendarClock className='size-3' />
          <span>
            {t('Metadata updated')} {updatedTime}
          </span>
        </div>
      )}
    </section>
  )
}
