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
import { Download, RefreshCw, Trash2, TriangleAlert } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'

import { QuotationEditor } from './components/quotation-editor'
import { QuotationPreview } from './components/quotation-preview'
import { useQuotationDraft } from './hooks/use-quotation-draft'
import { downloadQuotationHtml } from './lib/download-html'
import type { QuotationHtmlOptions } from './lib/quotation-html'
import { buildQuotationSnapshot, validateQuotation } from './lib/quotation-math'
import type { QuotePriceDimension, QuotationValidationError } from './types'

const STATIC_DIMENSION_LABELS = [
  'Input',
  'Output',
  'Cache read',
  'Cache write',
  'Image input',
  'Audio input',
  'Audio output',
  'Video input',
  'Dynamic pricing',
  'Base charge',
  'Task usage pricing',
  'Request',
] as const

const STATIC_UNIT_LABELS = [
  '1M token',
  'variable',
  'request',
  'count',
  'image',
  'second',
] as const

const VIDEO_CONDITION_PATTERN =
  /^(.*); fps ([^;]+); extra frames ([^;]+); Token = ceil\(width x height x \(fps x duration \+ extra frames\) \/ 1024\)$/

function generatedDimensionLabel(
  dimension: QuotePriceDimension,
  translate: (key: string, values?: Record<string, unknown>) => string
): string {
  const withoutVideoSuffix = ' without video input'
  const withVideoSuffix = ' with video input'
  const outputSuffix = ' output'
  if (
    dimension.sourceType === 'video' &&
    dimension.label.endsWith(withoutVideoSuffix)
  ) {
    return translate('{{resolution}} without video input', {
      resolution: dimension.label.slice(0, -withoutVideoSuffix.length),
    })
  }
  if (
    dimension.sourceType === 'video' &&
    dimension.label.endsWith(withVideoSuffix)
  ) {
    return translate('{{resolution}} with video input', {
      resolution: dimension.label.slice(0, -withVideoSuffix.length),
    })
  }
  if (
    dimension.sourceType === 'grok' &&
    dimension.label.endsWith(outputSuffix)
  ) {
    return translate('{{tier}} output', {
      tier: dimension.label.slice(0, -outputSuffix.length),
    })
  }
  return translate(dimension.label)
}

function generatedConditionLabel(
  condition: string,
  translate: (key: string, values?: Record<string, unknown>) => string
): string | null {
  const videoCondition = condition.match(VIDEO_CONDITION_PATTERN)
  if (!videoCondition) return null
  return translate(
    '{{resolution}}; fps {{fps}}; extra frames {{extraFrames}}; Token = ceil(width x height x (fps x duration + extra frames) / 1024)',
    {
      resolution: videoCondition[1],
      fps: videoCondition[2],
      extraFrames: videoCondition[3],
    }
  )
}

export function ProductQuotation() {
  const { t, i18n } = useTranslation()
  const {
    models,
    vendors,
    groupRatio,
    usableGroup,
    pricingVersion,
    dataUpdatedAt,
    isLoading,
    error,
    refetch,
    priceRate,
    usdExchangeRate,
  } = usePricingData(true)
  const { draft, setDraft, clearDraft, saveStatus } = useQuotationDraft()
  const [refreshing, setRefreshing] = useState(false)
  const [refreshFeedback, setRefreshFeedback] = useState<string | null>(null)
  const fetchedAt = dataUpdatedAt ? new Date(dataUpdatedAt).toISOString() : ''
  const snapshot = useMemo(
    () =>
      buildQuotationSnapshot({
        draft,
        models,
        vendors,
        groupRatio,
        pricingVersion,
        fetchedAt,
      }),
    [draft, fetchedAt, groupRatio, models, pricingVersion, vendors]
  )
  const validation = useMemo(() => validateQuotation(snapshot), [snapshot])
  const groups = useMemo(
    () =>
      Object.keys(usableGroup).sort((left, right) => left.localeCompare(right)),
    [usableGroup]
  )
  const canExport =
    !isLoading &&
    !error &&
    dataUpdatedAt > 0 &&
    models.length > 0 &&
    validation.valid
  const formattedFetchedAt = dataUpdatedAt
    ? new Intl.DateTimeFormat(i18n.resolvedLanguage || i18n.language, {
        dateStyle: 'medium',
        timeStyle: 'short',
      }).format(new Date(dataUpdatedAt))
    : t('Not fetched yet')
  const htmlOptions = useMemo<QuotationHtmlOptions>(() => {
    const dimensionLabels: Record<string, string> = Object.fromEntries(
      STATIC_DIMENSION_LABELS.map((label) => [label, t(label)])
    )
    const unitLabels: Record<string, string> = Object.fromEntries(
      STATIC_UNIT_LABELS.map((unit) => [unit, t(unit)])
    )
    const conditionLabels: Record<string, string> = {}
    for (const provider of snapshot.providers) {
      for (const model of provider.models) {
        for (const dimension of model.dimensions) {
          dimensionLabels[dimension.label] = generatedDimensionLabel(
            dimension,
            t
          )
          unitLabels[dimension.unit] = t(dimension.unit)
          if (dimension.condition) {
            const translatedCondition = generatedConditionLabel(
              dimension.condition,
              t
            )
            if (translatedCondition) {
              conditionLabels[dimension.condition] = translatedCondition
            }
          }
        }
      }
    }
    return {
      locale: i18n.resolvedLanguage || i18n.language,
      labels: {
        emptyValue: '—',
        customer: t('Customer'),
        quotedBy: t('Quoted by'),
        quoteDate: t('Quote date'),
        globalDiscount: t('Global discount'),
        priceBasis: t('Price basis'),
        pricingVersion: t('Pricing version'),
        fetchedAt: t('Pricing fetched at'),
        rawPriceBasis: t('Raw model price (1x)'),
        groupPriceBasis: t('User group'),
        discount: t('Discount'),
        discountCoefficient: t('Discount coefficient'),
        discountSuffix: t(' / 10 of list price'),
        providerNote: t('Provider note'),
        priceDimension: t('Dimension'),
        sourceType: t('Source'),
        catalogPrice: t('Catalog price'),
        basisPrice: t('Basis price'),
        quotePrice: t('Quote price'),
        currency: t('Currency'),
        unit: t('Unit'),
        condition: t('Condition'),
        status: t('Status'),
        ready: t('Ready'),
        needsConfirmation: t('Needs confirmation'),
        groupUnavailable: t('Unavailable for selected group'),
        catalogMissing: t('Missing from current pricing'),
        noDimensions: t('No price dimensions available'),
        noProviderModels: t('No selected models for this provider.'),
        noModels: t('No models selected.'),
        usageExamples: t('Usage examples'),
        sourceTypes: {
          fixed_token: t('Fixed token'),
          request: t('Per request'),
          dynamic: t('Dynamic pricing'),
          task_usage: t('Task usage'),
          video: t('Video pricing'),
          grok: t('Media pricing'),
        },
      },
      dimensionLabels,
      unitLabels,
      conditionLabels,
    }
  }, [i18n.language, i18n.resolvedLanguage, snapshot, t])

  const handleRefresh = async () => {
    setRefreshing(true)
    setRefreshFeedback(null)
    try {
      const result = await refetch()
      if (result.isSuccess) {
        setRefreshFeedback(t('Pricing refreshed'))
        toast.success(t('Pricing refreshed'))
      } else {
        setRefreshFeedback(t('Pricing refresh failed'))
        toast.error(t('Pricing refresh failed'))
      }
    } catch {
      setRefreshFeedback(t('Pricing refresh failed'))
      toast.error(t('Pricing refresh failed'))
    } finally {
      setRefreshing(false)
    }
  }

  const handleClear = () => {
    clearDraft()
    setRefreshFeedback(null)
    toast.success(t('Quotation draft cleared'))
  }

  const handleExport = () => {
    if (!canExport) {
      toast.error(t('Resolve quotation errors before exporting.'))
      return
    }
    try {
      downloadQuotationHtml(snapshot, htmlOptions)
      toast.success(t('HTML quotation downloaded'))
    } catch {
      toast.error(t('Failed to export HTML quotation'))
    }
  }

  const actions = (
    <>
      <Button
        type='button'
        variant='outline'
        onClick={() => void handleRefresh()}
        disabled={refreshing || isLoading}
        aria-label={t('Refresh pricing')}
      >
        <RefreshCw className={refreshing ? 'animate-spin' : undefined} />
        <span className='hidden sm:inline'>{t('Refresh pricing')}</span>
      </Button>
      <Button
        type='button'
        variant='outline'
        onClick={handleClear}
        aria-label={t('Clear draft')}
      >
        <Trash2 />
        <span className='hidden sm:inline'>{t('Clear draft')}</span>
      </Button>
      <Button
        type='button'
        onClick={handleExport}
        disabled={!canExport}
        aria-label={t('Export HTML quotation')}
      >
        <Download />
        <span className='hidden sm:inline'>{t('Export HTML')}</span>
      </Button>
    </>
  )

  const statusLine = (
    <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
      <span>
        {t('Last successful pricing fetch')}: {formattedFetchedAt}
      </span>
      <span>
        {saveStatus === 'saved'
          ? t('Draft saved locally')
          : t('Draft could not be saved locally')}
      </span>
      {refreshFeedback && <span role='status'>{refreshFeedback}</span>}
    </div>
  )

  const validationErrorLabel = (
    validationError: QuotationValidationError
  ): string => {
    switch (validationError.code) {
      case 'missing_title':
        return t('Quotation title is required.')
      case 'missing_quote_date':
        return t('Quote date is required.')
      case 'no_models':
        return t('Select at least one model.')
      case 'invalid_discount':
        return t('Global discount must be greater than 0 and at most 10.')
      case 'invalid_provider_discount': {
        const provider = snapshot.providers.find(
          (section) => section.providerId === validationError.providerId
        )
        return t(
          'Provider {{provider}} discount must be greater than 0 and at most 10.',
          {
            provider:
              provider?.providerName ??
              validationError.providerId ??
              t('Unknown provider'),
          }
        )
      }
      case 'invalid_price_basis':
        return t('The selected price basis has no valid ratio.')
      case 'model_unavailable':
        return t('Model {{model}} is unavailable for this price basis.', {
          model: validationError.modelId ?? t('Unknown model'),
        })
      case 'price_needs_confirmation': {
        const model = snapshot.providers
          .flatMap((provider) => provider.models)
          .find((section) => section.modelId === validationError.modelId)
        const dimension = model?.dimensions.find(
          (item) => item.key === validationError.dimensionKey
        )
        return t('Price {{dimension}} for {{model}} needs confirmation.', {
          dimension: t(
            dimension?.label ??
              validationError.dimensionKey ??
              'Unknown price dimension'
          ),
          model: validationError.modelId ?? t('Unknown model'),
        })
      }
    }
  }

  const renderContent = () => {
    if (isLoading) {
      return (
        <div
          role='status'
          aria-label={t('Loading pricing')}
          className='grid min-w-0 gap-4 xl:grid-cols-2'
        >
          <Skeleton className='h-[36rem] w-full' />
          <Skeleton className='h-[44rem] w-full' />
        </div>
      )
    }
    if (error) {
      return (
        <Alert variant='destructive' className='mx-auto max-w-2xl'>
          <TriangleAlert />
          <AlertTitle>{t('Unable to load pricing')}</AlertTitle>
          <AlertDescription>
            {t(
              'The pricing catalog could not be loaded. Your local draft is unchanged.'
            )}
          </AlertDescription>
          <div className='mt-3 pl-6'>
            <Button
              type='button'
              variant='outline'
              onClick={() => void handleRefresh()}
              disabled={refreshing}
              aria-label={t('Retry pricing')}
            >
              <RefreshCw className={refreshing ? 'animate-spin' : undefined} />
              {t('Retry')}
            </Button>
          </div>
        </Alert>
      )
    }
    if (models.length === 0) {
      return (
        <div className='text-muted-foreground mx-auto max-w-2xl rounded-xl border border-dashed px-6 py-16 text-center'>
          <p className='text-foreground font-medium'>
            {t('No pricing models available')}
          </p>
          <p className='mt-1 text-sm'>
            {t('Refresh pricing after models have been configured.')}
          </p>
        </div>
      )
    }
    return (
      <div
        data-testid='quotation-workspace'
        className='grid min-w-0 grid-cols-1 items-start gap-4 overflow-hidden xl:grid-cols-[minmax(0,560px)_minmax(0,1fr)]'
      >
        <QuotationEditor
          draft={draft}
          models={models}
          vendors={vendors}
          groups={groups}
          actualExchangeRate={usdExchangeRate}
          fixedExchangeRate={priceRate}
          onDraftChange={setDraft}
        />
        <div
          data-testid='quotation-preview-pane'
          className='min-w-0 xl:sticky xl:top-0'
        >
          {!validation.valid && (
            <Alert variant='destructive' className='mb-3'>
              <TriangleAlert />
              <AlertTitle>{t('Quotation is not ready to export')}</AlertTitle>
              <AlertDescription>
                <ul className='mt-1 list-disc space-y-0.5 pl-4'>
                  {validation.errors.map((validationError) => (
                    <li
                      key={`${validationError.code}-${validationError.providerId ?? ''}-${validationError.modelId ?? ''}-${validationError.dimensionKey ?? ''}`}
                    >
                      {validationErrorLabel(validationError)}
                    </li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          )}
          <QuotationPreview snapshot={snapshot} />
        </div>
      </div>
    )
  }

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Product quotation and calculator')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>{actions}</SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='h-full min-w-0 overflow-y-auto pb-4'>
          <div className='mb-3'>{statusLine}</div>
          {renderContent()}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
