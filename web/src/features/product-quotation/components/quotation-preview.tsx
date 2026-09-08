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
import { Check, Copy, FileText } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { toIntlLocale } from '@/i18n/languages'

import { normalizeDiscount } from '../lib/quotation-math'
import type { QuoteModelSection, QuotationSnapshot } from '../types'
import { PriceDimensionTable } from './price-dimension-table'

type QuotationPreviewProps = {
  snapshot: QuotationSnapshot
}

function ModelCopyButton({ modelId }: { modelId: string }) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({
    successMessage: t('Model ID copied'),
    errorMessage: t('Failed to copy model ID'),
  })
  const copied = copiedText === modelId

  return (
    <>
      <Button
        type='button'
        variant='ghost'
        size='icon-sm'
        onClick={() => void copyToClipboard(modelId)}
        aria-label={t('Copy model ID {{model}}', { model: modelId })}
      >
        {copied ? <Check /> : <Copy />}
      </Button>
      {copied && (
        <span role='status' className='sr-only'>
          {t('Model ID copied')}
        </span>
      )}
    </>
  )
}

function UnavailableBadge({ model }: { model: QuoteModelSection }) {
  const { t } = useTranslation()
  if (model.available) return null
  return (
    <Badge variant='warning' className='h-auto whitespace-normal'>
      {model.unavailableReason === 'group_unavailable'
        ? t('Unavailable for selected group')
        : t('Missing from current pricing')}
    </Badge>
  )
}

export function QuotationPreview({ snapshot }: QuotationPreviewProps) {
  const { t, i18n } = useTranslation()
  const fetchedAt = snapshot.fetchedAt
    ? new Intl.DateTimeFormat(
        toIntlLocale(i18n.resolvedLanguage || i18n.language),
        {
          dateStyle: 'medium',
          timeStyle: 'short',
        }
      ).format(new Date(snapshot.fetchedAt))
    : '—'
  const basisLabel =
    snapshot.priceBasis.type === 'raw'
      ? t('Raw model price (1x)')
      : t('User group {{group}} ({{ratio}}x)', {
          group: snapshot.priceBasis.group ?? '—',
          ratio: snapshot.priceBasis.ratio ?? '—',
        })
  const providerDiscountLabel = (
    provider: QuotationSnapshot['providers'][number]
  ) => {
    if (
      provider.discount !== null &&
      normalizeDiscount(provider.discount) === null
    ) {
      return t('Invalid provider discount')
    }
    if (provider.discountCoefficient === null) return t('Invalid discount')
    return t('{{discount}} tenths of list price', {
      discount: provider.discount ?? snapshot.globalDiscount ?? '—',
    })
  }

  return (
    <article
      aria-label={t('Quotation preview')}
      className='bg-white text-slate-900 shadow-sm ring-1 ring-slate-200'
    >
      <header className='border-b-2 border-slate-900 px-4 py-6 sm:px-7'>
        <div className='flex items-center gap-2 text-sm font-semibold tracking-wide text-slate-500 uppercase'>
          <FileText className='size-4' />
          {t('Quotation')}
        </div>
        <h2 className='mt-2 text-2xl font-semibold tracking-tight break-words'>
          {snapshot.title || t('Untitled quotation')}
        </h2>
        <dl className='mt-5 grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2'>
          <div className='min-w-0'>
            <dt className='text-xs font-semibold text-slate-500 uppercase'>
              {t('Customer')}
            </dt>
            <dd className='mt-0.5 break-words'>{snapshot.customer || '—'}</dd>
          </div>
          <div className='min-w-0'>
            <dt className='text-xs font-semibold text-slate-500 uppercase'>
              {t('Quoted by')}
            </dt>
            <dd className='mt-0.5 break-words'>{snapshot.quotedBy || '—'}</dd>
          </div>
          <div>
            <dt className='text-xs font-semibold text-slate-500 uppercase'>
              {t('Quote date')}
            </dt>
            <dd className='mt-0.5'>{snapshot.quoteDate || '—'}</dd>
          </div>
          <div>
            <dt className='text-xs font-semibold text-slate-500 uppercase'>
              {t('Global discount')}
            </dt>
            <dd className='mt-0.5'>
              {snapshot.globalDiscount === null
                ? '—'
                : t('{{discount}} tenths of list price', {
                    discount: snapshot.globalDiscount,
                  })}
            </dd>
          </div>
          <div className='min-w-0'>
            <dt className='text-xs font-semibold text-slate-500 uppercase'>
              {t('Price basis')}
            </dt>
            <dd className='mt-0.5 break-words'>{basisLabel}</dd>
          </div>
          <div className='min-w-0'>
            <dt className='text-xs font-semibold text-slate-500 uppercase'>
              {t('Pricing version')}
            </dt>
            <dd className='mt-0.5 break-words'>
              {snapshot.pricingVersion || '—'}
            </dd>
          </div>
          <div className='min-w-0 sm:col-span-2'>
            <dt className='text-xs font-semibold text-slate-500 uppercase'>
              {t('Pricing fetched at')}
            </dt>
            <dd className='mt-0.5 break-words'>{fetchedAt}</dd>
          </div>
        </dl>
      </header>

      <div className='space-y-7 px-4 py-6 sm:px-7'>
        {snapshot.providers.length === 0 ? (
          <div className='rounded-lg border border-dashed border-slate-300 px-4 py-12 text-center text-sm text-slate-500'>
            {t('Select models to build the quotation preview.')}
          </div>
        ) : (
          snapshot.providers.map((provider) => (
            <section
              key={provider.providerId ?? `unknown-${provider.providerName}`}
              className='min-w-0 space-y-4'
            >
              <div className='flex flex-wrap items-end justify-between gap-2 border-b border-slate-300 pb-2'>
                <h3 className='text-lg font-semibold break-words'>
                  {provider.providerName}
                </h3>
                <span className='text-sm text-slate-600'>
                  {providerDiscountLabel(provider)}
                </span>
              </div>
              {provider.note && (
                <div className='border-l-2 border-slate-400 bg-slate-50 px-3 py-2 text-sm break-words whitespace-pre-wrap'>
                  <span className='font-semibold'>{t('Provider note')}: </span>
                  {provider.note}
                </div>
              )}
              <div className='space-y-5'>
                {provider.models.map((model) => (
                  <section key={model.modelId} className='min-w-0 space-y-2'>
                    <div className='flex min-w-0 items-start justify-between gap-3'>
                      <div className='min-w-0 flex-1'>
                        <div className='flex flex-wrap items-center gap-2'>
                          <h4 className='font-semibold break-words'>
                            {model.displayName}
                          </h4>
                          <UnavailableBadge model={model} />
                        </div>
                        <div className='mt-0.5 flex min-w-0 items-start gap-1'>
                          <p className='min-w-0 font-mono text-xs break-all text-slate-500'>
                            {model.modelId}
                          </p>
                          <ModelCopyButton modelId={model.modelId} />
                        </div>
                      </div>
                    </div>
                    <PriceDimensionTable dimensions={model.dimensions} />
                  </section>
                ))}
              </div>
            </section>
          ))
        )}
      </div>
    </article>
  )
}
