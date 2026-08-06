import { useQuery } from '@tanstack/react-query'
import { Images, Layers3, Scan, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { getPerfMetrics } from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatUptimePct,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'

import { getGrokModelCapabilities } from '../lib/grok-model'
import type { PricingModel } from '../types'

function Card(props: {
  label: string
  value: React.ReactNode
  hint?: string
  className?: string
}) {
  return (
    <div className='bg-background rounded-lg border p-3'>
      <div className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
        {props.label}
      </div>
      <div
        className={cn(
          'mt-1 font-mono text-lg font-semibold tabular-nums',
          props.className
        )}
      >
        {props.value}
      </div>
      {props.hint && (
        <div className='text-muted-foreground/70 mt-1 text-[11px]'>
          {props.hint}
        </div>
      )}
    </div>
  )
}

export function ModelDetailsGrokOverview(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const caps = getGrokModelCapabilities(props.model.model_name)
  const items = [
    {
      icon: Layers3,
      label: t('Input'),
      value: caps.input.map((item) => t(item)).join(' · '),
    },
    { icon: Sparkles, label: t('Output'), value: t(caps.output) },
    { icon: Scan, label: t('Resolution'), value: caps.resolutions.join(' · ') },
    {
      icon: Images,
      label: t('Supported operations'),
      value: caps.operations.map((item) => t(item)).join(' · '),
    },
  ]
  return (
    <div className='grid gap-2 sm:grid-cols-2'>
      {items.map((item) => {
        const Icon = item.icon
        return (
          <div
            key={item.label}
            className='bg-muted/20 flex gap-3 rounded-lg border p-3'
          >
            <Icon className='text-muted-foreground mt-0.5 size-4 shrink-0' />
            <div>
              <div className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
                {item.label}
              </div>
              <div className='mt-1 text-sm font-semibold'>{item.value}</div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

export function ModelDetailsGrokPricing(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const pricing = props.model.molii_grok_pricing
  if (!pricing) {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('Pricing is temporarily unavailable.')}
      </p>
    )
  }
  const outputUnit = pricing.output_unit === 'second' ? t('second') : t('image')
  return (
    <div className='space-y-3'>
      <div className='grid gap-2 sm:grid-cols-2 @2xl/details:grid-cols-3'>
        {Object.entries(pricing.output_prices).map(([resolution, price]) => (
          <Card
            key={resolution}
            label={`${resolution.toUpperCase()} ${t('Output')}`}
            value={`¥${price}`}
            hint={`${t('Per')} ${outputUnit}`}
          />
        ))}
        {pricing.image_input_price != null && (
          <Card
            label={t('Image input')}
            value={`¥${pricing.image_input_price}`}
            hint={t('Per input image')}
          />
        )}
        {pricing.video_input_price != null && pricing.video_input_price > 0 && (
          <Card
            label={t('Video input')}
            value={`¥${pricing.video_input_price}`}
            hint={t('Per input video second')}
          />
        )}
      </div>
      <p className='text-muted-foreground text-xs'>
        {pricing.kind === 'image'
          ? t(
              'Total = output unit price × output quantity + input image unit price × input image quantity.'
            )
          : t(
              'Total = output price per second × output duration + image input price × image quantity + video input price per second × input duration.'
            )}
      </p>
    </div>
  )
}

export function ModelDetailsGrokImagePerformance(props: {
  model: PricingModel
}) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['perf-metrics', props.model.model_name, 24],
    queryFn: () => getPerfMetrics(props.model.model_name, 24),
    staleTime: 60_000,
  })
  if (query.isLoading) {
    return (
      <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
        {t('Loading performance…')}
      </div>
    )
  }
  if (query.isError || !query.data?.data) {
    return (
      <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
        {t('Performance data is temporarily unavailable.')}
      </div>
    )
  }
  const groups = query.data.data.groups
  const requestCount = groups.reduce(
    (sum, group) => sum + (group.request_count ?? 0),
    0
  )
  const weightedLatency = groups.reduce(
    (sum, group) => sum + group.avg_latency_ms * (group.request_count ?? 0),
    0
  )
  const successCount = groups.reduce(
    (sum, group) =>
      sum + (group.success_rate / 100) * (group.request_count ?? 0),
    0
  )
  const successRate =
    requestCount > 0 ? (successCount / requestCount) * 100 : Number.NaN
  return (
    <div className='space-y-5'>
      <div className='grid gap-2 sm:grid-cols-3'>
        <Card
          label={t('Request count')}
          value={requestCount.toLocaleString()}
          hint={t('Last 24 hours')}
        />
        <Card
          label={t('Average response time')}
          value={formatLatency(
            requestCount > 0 ? weightedLatency / requestCount : Number.NaN
          )}
          hint={t('End-to-end request latency')}
        />
        <Card
          label={t('Request success rate')}
          value={formatUptimePct(successRate)}
          hint={t('Successful requests divided by all requests')}
          className={getSuccessRateTextClass(successRate)}
        />
      </div>
      {groups.length === 0 && (
        <div className='text-muted-foreground rounded-lg border p-6 text-center text-sm'>
          {t('No request data is available for the last 24 hours.')}
        </div>
      )}
    </div>
  )
}
