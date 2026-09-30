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
  CalendarDays,
  Copy,
  FileText,
  Image as ImageIcon,
  MessageSquareText,
  Video,
} from 'lucide-react'
import { memo, useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { DEFAULT_TOKEN_UNIT } from '../constants'
import { useBillingTime } from '../hooks/use-billing-time'
import {
  getCardExamplePrice,
  getDynamicDisplayGroupRatio,
  getDynamicPriceUnitLabelKey,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import {
  getCompactPricingSummary,
  type CompactPricingSummary,
} from '../lib/model-card-summary'
import { getPricingModelDescription } from '../lib/model-description'
import { getModelInputModalities } from '../lib/model-directory'
import { taskPriceLabel, taskUsageUnitLabel } from '../lib/task-price-display'
import type {
  Modality,
  ModelCapability,
  PricingModel,
  TokenUnit,
} from '../types'
import { ModelBillingModeBadge } from './model-billing-mode-badge'
import { ModelPerfBadge, type ModelPerfBadgeData } from './model-perf-badge'

export interface ModelCardProps {
  model: PricingModel
  onClick: (modelName: string) => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  selectedGroup?: string
  perf?: ModelPerfBadgeData
}

const CAPABILITY_PRIORITY: ModelCapability[] = [
  'reasoning',
  'tools',
  'structured_output',
  'function_calling',
  'vision',
  'streaming',
  'image_generation',
  'video_generation',
]

const CAPABILITY_LABELS: Partial<Record<ModelCapability, string>> = {
  reasoning: 'Reasoning',
  tools: 'Tools',
  structured_output: 'Structured output',
  function_calling: 'Function calling',
  vision: 'Vision',
  streaming: 'Streaming',
  image_generation: 'Image generation',
  video_generation: 'Video generation',
}

const MODALITY_META: Record<
  Modality,
  {
    label: string
    icon: React.ComponentType<{ className?: string }>
  }
> = {
  text: { label: 'Text', icon: MessageSquareText },
  image: { label: 'Image', icon: ImageIcon },
  file: { label: 'File', icon: FileText },
  audio: { label: 'Audio', icon: AudioLines },
  video: { label: 'Video', icon: Video },
}

function formatTokenCount(tokens?: number): string {
  if (!tokens || !Number.isFinite(tokens) || tokens <= 0) return '—'
  if (tokens >= 1_000_000) {
    return `${Number((tokens / 1_000_000).toFixed(1))}M`
  }
  if (tokens >= 1_000) return `${Number((tokens / 1_000).toFixed(1))}K`
  return String(tokens)
}

function formatEndpoint(endpoint: string): string {
  const labels: Record<string, string> = {
    openai: 'OpenAI',
    'openai-response': 'Responses',
    anthropic: 'Anthropic',
    gemini: 'Gemini',
    'image-generation': 'Images',
    'openai-video': 'Videos',
    embeddings: 'Embeddings',
    'jina-rerank': 'Rerank',
  }
  return labels[endpoint] ?? endpoint
}

function ModalityList(props: { label: string; modalities: Modality[] }) {
  const { t } = useTranslation()
  if (props.modalities.length === 0) return null
  return (
    <div className='flex min-w-0 items-center gap-1.5'>
      <span className='text-muted-foreground/60 shrink-0 text-[10px] uppercase'>
        {t(props.label)}
      </span>
      <div className='flex min-w-0 flex-wrap gap-1'>
        {props.modalities.map((modality) => {
          const meta = MODALITY_META[modality]
          const Icon = meta.icon
          return (
            <span
              key={modality}
              title={t(meta.label)}
              className='text-muted-foreground inline-flex items-center gap-1 text-[11px]'
            >
              <Icon className='size-3' />
              <span className='sr-only'>{t(meta.label)}</span>
            </span>
          )
        })}
      </div>
    </div>
  )
}

function CompactPricing(props: { summary: CompactPricingSummary }) {
  const { t, i18n } = useTranslation()
  const { summary } = props

  if (summary.kind === 'token') {
    return (
      <div className='grid grid-cols-3 gap-2'>
        {summary.items.map((item) => (
          <div key={item.label} className='min-w-0'>
            <div className='text-muted-foreground/60 text-[10px]'>
              {t(item.label)}
            </div>
            <div className='text-foreground truncate font-mono text-xs font-semibold'>
              {item.value} / {summary.unit}
            </div>
          </div>
        ))}
      </div>
    )
  }

  if (summary.kind === 'request') {
    return (
      <div className='flex items-baseline justify-between gap-3'>
        <span className='text-muted-foreground text-xs'>
          {t('Per Request')}
        </span>
        <span className='font-mono text-sm font-semibold'>
          {summary.value} / {t(summary.unit)}
        </span>
      </div>
    )
  }

  if (summary.kind === 'task') {
    return (
      <div className='flex items-end justify-between gap-3'>
        <div className='min-w-0'>
          <div className='text-muted-foreground text-xs'>
            {taskPriceLabel(
              summary.description,
              summary.label,
              i18n.resolvedLanguage ?? i18n.language
            )}
          </div>
          {summary.example && (
            <div className='text-muted-foreground/50 mt-0.5 text-[10px]'>
              {summary.example}
            </div>
          )}
        </div>
        <div className='shrink-0 text-right'>
          <div className='font-mono text-sm font-semibold'>{summary.value}</div>
          <div className='text-muted-foreground/60 text-[10px]'>
            {' / '}
            {t(summary.unit)}
          </div>
        </div>
      </div>
    )
  }

  if (summary.kind === 'task-unconfigured') {
    return (
      <div className='text-muted-foreground text-xs'>
        {t('Usage-based billing · price not configured')}
      </div>
    )
  }

  if (summary.kind === 'special') {
    return (
      <div className='space-y-1'>
        <div className='text-muted-foreground text-xs'>
          {t('Special billing expression')}
        </div>
        <code className='text-muted-foreground block text-[10px] break-all'>
          {summary.expression}
        </code>
      </div>
    )
  }

  return (
    <div className='flex items-end justify-between gap-3'>
      <div className='min-w-0'>
        <div className='text-muted-foreground text-xs'>{t(summary.label)}</div>
        <div className='text-muted-foreground/50 mt-0.5 text-[10px]'>
          {summary.detail || t('Full pricing is available on the details page')}
        </div>
        {summary.noteKey && (
          <div className='text-muted-foreground/50 mt-0.5 text-[10px]'>
            {t(summary.noteKey)}
          </div>
        )}
      </div>
      {summary.from && (
        <div className='shrink-0 text-right'>
          <div className='font-mono text-sm font-semibold'>{summary.from}</div>
          <div className='text-muted-foreground/60 text-[10px]'>
            {t('from')} / {t(summary.unit === '1M' ? '1M token' : summary.unit)}
          </div>
        </div>
      )}
    </div>
  )
}

export const ModelCard = memo(function ModelCard(props: ModelCardProps) {
  const { t, i18n } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const currency = useSystemConfigStore((state) => state.config.currency)
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? currency.usdExchangeRate
  const showRechargePrice = props.showRechargePrice ?? false
  const billingTime = useBillingTime(props.model.billing_expr)
  const dynamicPriceOptions = useMemo(
    () => ({
      now: billingTime === undefined ? undefined : new Date(billingTime),
      tokenUnit,
      showRechargePrice,
      priceRate,
      usdExchangeRate,
      groupRatioMultiplier: getDynamicDisplayGroupRatio(
        props.model,
        props.selectedGroup
      ),
    }),
    [
      props.model,
      props.selectedGroup,
      billingTime,
      tokenUnit,
      showRechargePrice,
      priceRate,
      usdExchangeRate,
    ]
  )
  const dynamicSummary = useMemo(
    () => getDynamicPricingSummary(props.model, dynamicPriceOptions),
    // Currency is read indirectly by the price formatter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [props.model, dynamicPriceOptions, currency]
  )
  const cardExamplePrice = useMemo(
    () => getCardExamplePrice(props.model, dynamicPriceOptions),
    // Currency is read indirectly by the price formatter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [props.model, dynamicPriceOptions, currency]
  )
  const modelIconKey = props.model.icon || props.model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 24) : null
  const inputModalities = getModelInputModalities(props.model)
  const outputModalities = props.model.output_modalities ?? []
  const summary = getCompactPricingSummary(props.model, {
    tokenUnit,
    priceRate: props.priceRate,
    usdExchangeRate: props.usdExchangeRate,
    showRechargePrice: props.showRechargePrice,
    selectedGroup: props.selectedGroup,
  })
  const description = getPricingModelDescription(
    props.model,
    i18n.resolvedLanguage ?? i18n.language
  )
  const displayName = props.model.display_name?.trim() || props.model.model_name
  const capabilities = CAPABILITY_PRIORITY.filter((capability) =>
    props.model.capabilities?.includes(capability)
  ).slice(0, 4)
  const endpoints = (props.model.supported_endpoint_types ?? []).slice(0, 2)
  const groups = props.model.enable_groups ?? []
  const allEndpoints = props.model.supported_endpoint_types ?? []
  const tags = (props.model.tags ?? '')
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
  let dynamicPriceSummary: ReactNode = null
  if (dynamicSummary) {
    if (dynamicSummary.isSpecialExpression) {
      dynamicPriceSummary = (
        <div className='col-span-full min-w-0'>
          <span className='text-warning'>
            {t('Special billing expression')}
          </span>
          <code className='text-muted-foreground mt-1 line-clamp-2 block font-mono text-xs break-all'>
            {dynamicSummary.rawExpression}
          </code>
        </div>
      )
    } else if (dynamicSummary.primaryEntries.length > 0) {
      dynamicPriceSummary = (
        <>
          {dynamicSummary.primaryEntries
            .slice(0, dynamicSummary.providerCount ? 2 : undefined)
            .map((entry) => {
              const unitLabelKey = getDynamicPriceUnitLabelKey(entry)
              const unitLabel = taskUsageUnitLabel(
                entry,
                i18n.resolvedLanguage ?? i18n.language,
                unitLabelKey ? t(unitLabelKey) : tokenUnitLabel
              )
              const label =
                entry.labelKind === 'schema'
                  ? taskPriceLabel(
                      entry.description,
                      entry.shortLabel,
                      i18n.resolvedLanguage ?? i18n.language
                    )
                  : t(entry.shortLabel)
              return (
                <div
                  key={entry.key}
                  className={cn(
                    'flex min-w-0 flex-col gap-1',
                    dynamicSummary.isTaskUsage && 'col-span-full'
                  )}
                >
                  {label && (
                    <span className='text-muted-foreground text-xs break-words whitespace-normal'>
                      {label}
                    </span>
                  )}
                  <span className='flex flex-wrap items-baseline gap-x-1 font-mono text-sm font-semibold tabular-nums'>
                    <span>{entry.formattedRange ?? entry.formatted}</span>
                    <span className='text-muted-foreground text-xs font-normal whitespace-nowrap'>
                      {' '}
                      / {unitLabel}
                    </span>
                  </span>
                </div>
              )
            })}
          {dynamicSummary.isTimePricing && (
            <span className='text-muted-foreground col-span-full text-xs'>
              {t('Current period price')}
            </span>
          )}
          {dynamicSummary.isMixedBilling && (
            <span className='text-muted-foreground col-span-full text-xs'>
              {t('Token or per-call pricing')}
            </span>
          )}
          {cardExamplePrice && (
            <span className='text-muted-foreground col-span-full text-xs break-words'>
              {cardExamplePrice.label} ≈ {cardExamplePrice.formatted}
            </span>
          )}
          {dynamicSummary.isTaskUsage &&
            dynamicSummary.tier?.label &&
            !dynamicSummary.primaryEntries.some(
              (entry) => entry.formattedRange
            ) && (
              <span className='text-muted-foreground col-span-full text-xs break-words'>
                ({dynamicSummary.tier.label})
              </span>
            )}
        </>
      )
    } else {
      dynamicPriceSummary = (
        <span className='text-muted-foreground col-span-full text-xs'>
          {dynamicSummary.hasUnconfiguredProviders
            ? t('Usage-based billing · price not configured')
            : t('Dynamic Pricing')}
        </span>
      )
    }
  }

  const handleCopy = (event: React.MouseEvent) => {
    event.stopPropagation()
    copyToClipboard(props.model.model_name)
  }

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      props.onClick(props.model.model_name || '')
    }
  }

  return (
    <article
      role='link'
      tabIndex={0}
      data-model-card='true'
      onClick={() => props.onClick(props.model.model_name || '')}
      onKeyDown={handleKeyDown}
      className={cn(
        'group bg-background relative flex min-h-[330px] cursor-pointer flex-col border-r border-b p-4 outline-none transition-colors sm:p-5',
        'hover:bg-muted/25 focus-visible:ring-ring focus-visible:z-10 focus-visible:ring-2 focus-visible:ring-inset'
      )}
    >
      <header className='flex items-start justify-between gap-3'>
        <div className='flex min-w-0 items-start gap-2.5'>
          <span className='bg-muted/50 flex size-8 shrink-0 items-center justify-center rounded-full border'>
            {modelIcon ?? (
              <span className='text-xs font-semibold'>
                {props.model.model_name.charAt(0).toUpperCase()}
              </span>
            )}
          </span>
          <div className='min-w-0'>
            <div className='text-muted-foreground text-[11px] font-medium'>
              {props.model.vendor_name || t('Unknown vendor')}
            </div>
            <h3
              title={props.model.model_name}
              className='text-foreground mt-0.5 line-clamp-2 text-sm leading-5 font-semibold'
            >
              {displayName}
            </h3>
            {displayName !== props.model.model_name && (
              <div className='text-muted-foreground/70 mt-0.5 truncate font-mono text-[10px]'>
                {props.model.model_name}
              </div>
            )}
          </div>
        </div>
        <button
          type='button'
          onClick={handleCopy}
          aria-label={t('Copy model name')}
          title={t('Copy model name')}
          className='text-muted-foreground hover:bg-muted hover:text-foreground -mr-1 rounded-md p-1.5'
        >
          <Copy className='size-3.5' />
        </button>
      </header>

      <p className='text-muted-foreground mt-3 line-clamp-3 min-h-[3.75rem] text-xs leading-5'>
        {description || t('No description available.')}
      </p>

      <div className='mt-3 flex flex-wrap items-center justify-between gap-2 border-y py-2'>
        <ModalityList label='Input' modalities={inputModalities} />
        <ModalityList label='Output' modalities={outputModalities} />
      </div>

      {(groups.length > 0 || allEndpoints.length > 0 || tags.length > 0) && (
        <div className='mt-3 space-y-1.5 text-[10px]'>
          {groups.length > 0 && (
            <div className='flex min-w-0 items-center gap-2'>
              <span className='text-muted-foreground/60 shrink-0'>
                {t('Groups')}
              </span>
              <span className='truncate' title={groups[0]}>
                {groups[0]}
              </span>
              {groups.length > 1 && (
                <span title={groups.slice(1).join(', ')}>
                  +{groups.length - 1}
                </span>
              )}
            </div>
          )}
          {allEndpoints.length > 0 && (
            <div className='flex min-w-0 items-center gap-2'>
              <span className='text-muted-foreground/60 shrink-0'>
                {t('Endpoints')}
              </span>
              <span className='truncate' title={allEndpoints.join(', ')}>
                {allEndpoints.slice(0, 2).join(', ')}
              </span>
              {allEndpoints.length > 2 && (
                <span>+{allEndpoints.length - 2}</span>
              )}
            </div>
          )}
          {tags.length > 0 && (
            <div
              role='group'
              aria-label={t('Tags')}
              className='flex min-w-0 items-center gap-2'
            >
              <span className='truncate' title={tags.join(', ')}>
                {tags.slice(0, 2).join(', ')}
              </span>
              {tags.length > 2 && <span>+{tags.length - 2}</span>}
            </div>
          )}
        </div>
      )}

      <div
        className='mt-3 flex min-w-0 flex-col gap-1.5'
        data-model-card-pricing='true'
        role='group'
        aria-label={t('Pricing')}
      >
        {dynamicSummary && (
          <ModelBillingModeBadge model={props.model} appearance='caption' />
        )}
        {!dynamicSummary && summary.kind === 'token' && (
          <span className='sr-only'>{t('Token-based')}</span>
        )}
        {dynamicSummary?.providerCount && (
          <span className='text-muted-foreground text-xs break-words'>
            {t('{{count}} providers', {
              count: dynamicSummary.providerCount,
            })}
            {dynamicSummary.hasUnconfiguredProviders &&
              ` · ${t('Not configured for some providers')}`}
          </span>
        )}
        {dynamicSummary ? (
          <div className='grid grid-cols-[repeat(auto-fit,minmax(88px,1fr))] gap-x-3 gap-y-2'>
            {dynamicPriceSummary}
          </div>
        ) : (
          <CompactPricing summary={summary} />
        )}
      </div>

      <div className='mt-3 grid grid-cols-2 gap-2' data-model-card-specs='true'>
        {props.model.context_length != null && (
          <div>
            <div className='text-muted-foreground/60 text-[10px]'>
              {t('Context')}
            </div>
            <div className='text-xs font-medium'>
              {formatTokenCount(props.model.context_length)}
            </div>
          </div>
        )}
        {props.model.max_output_tokens != null && (
          <div>
            <div className='text-muted-foreground/60 text-[10px]'>
              {t('Max output')}
            </div>
            <div className='text-xs font-medium'>
              {formatTokenCount(props.model.max_output_tokens)}
            </div>
          </div>
        )}
      </div>

      {capabilities.length > 0 && (
        <div className='mt-3 flex flex-wrap gap-1' data-model-card-capabilities>
          {capabilities.map((capability) => (
            <span
              key={capability}
              className='bg-muted/60 text-muted-foreground rounded px-1.5 py-1 text-[10px]'
            >
              {t(CAPABILITY_LABELS[capability] ?? capability)}
            </span>
          ))}
        </div>
      )}

      <footer className='mt-auto flex items-end justify-between gap-3 border-t pt-3'>
        <div className='text-muted-foreground min-w-0 space-y-1 text-[10px]'>
          {props.model.release_date && (
            <div className='flex items-center gap-1' data-model-release-date>
              <CalendarDays className='size-3' />
              <span>{props.model.release_date}</span>
            </div>
          )}
          {endpoints.length > 0 && (
            <div className='truncate'>
              {endpoints.map(formatEndpoint).join(' · ')}
            </div>
          )}
        </div>
        <ModelPerfBadge perf={props.perf} />
      </footer>
      <button
        type='button'
        onClick={(event) => {
          event.stopPropagation()
          props.onClick(props.model.model_name || '')
        }}
        className='sr-only'
      >
        {t('Details')}
      </button>
    </article>
  )
})
