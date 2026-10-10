import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { listCatalogSyncHistory } from './api'
import type { CatalogOperation } from './types'

const pageSize = 20

export function CatalogHistory(props: {
  onRestore: (operationId: string) => void
  disabled?: boolean
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [offset, setOffset] = useState(0)
  const query = useQuery({
    queryKey: ['catalog-sync', 'history', offset],
    queryFn: () => listCatalogSyncHistory(offset, pageSize),
    retry: false,
    meta: { errorToast: false },
  })
  const page = query.data
  const latestSucceeded =
    offset === 0
      ? page?.items.find((item) => item.state === 'succeeded')
      : undefined
  return (
    <section aria-label={t('Sync history')} className='flex flex-col gap-3'>
      <h2 className='text-lg font-medium'>{t('Sync history')}</h2>
      {query.isPending && <LoadingState />}
      {query.isError && (
        <ErrorState
          title={t('History is unavailable')}
          onRetry={() => void query.refetch()}
        />
      )}
      {page && (
        <>
          {latestSucceeded && (
            <p className='text-muted-foreground text-sm'>
              {t('Latest successful operation')}:{' '}
              {new Intl.DateTimeFormat(locale, {
                dateStyle: 'medium',
                timeStyle: 'short',
              }).format(new Date(latestSucceeded.created_at * 1000))}{' '}
              · {t('Revision')}:{' '}
              {formatNumber(latestSucceeded.revision, locale)}
            </p>
          )}
          <StaticDataTable
            data={page.items}
            getRowKey={(row) => row.id}
            emptyContent={t('No synchronization history')}
            tableClassName='min-w-[680px]'
            columns={[
              {
                id: 'time',
                header: t('Time'),
                cell: (row: CatalogOperation) =>
                  new Intl.DateTimeFormat(locale, {
                    dateStyle: 'medium',
                    timeStyle: 'short',
                  }).format(new Date(row.created_at * 1000)),
              },
              {
                id: 'kind',
                header: t('Operation'),
                cell: (row: CatalogOperation) =>
                  t(row.summary.kind === 'restore' ? 'Restore' : 'Sync'),
              },
              {
                id: 'state',
                header: t('State'),
                cell: (row: CatalogOperation) => t(row.state),
              },
              {
                id: 'scope',
                header: t('Source and target'),
                cell: (row: CatalogOperation) =>
                  `${row.summary.source_id} → ${row.summary.target_id}`,
              },
              {
                id: 'actions',
                header: t('Affected items'),
                cell: (row: CatalogOperation) =>
                  Object.entries(row.summary.actions)
                    .map(
                      ([action, count]) =>
                        `${t(action)}: ${formatNumber(count, locale)}`
                    )
                    .join(' · '),
              },
              {
                id: 'restore',
                header: t('Restore'),
                cell: (row: CatalogOperation) =>
                  row.id === latestSucceeded?.id ? (
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={props.disabled}
                      onClick={() => props.onRestore(row.id)}
                    >
                      {t('Preview restore')}
                    </Button>
                  ) : null,
              },
            ]}
          />
          <nav
            aria-label={t('History pages')}
            className='flex items-center justify-end gap-2'
          >
            <span className='text-muted-foreground text-sm'>
              {t('Items {{start}}–{{end}} of {{total}}', {
                start: formatNumber(page.total === 0 ? 0 : offset + 1, locale),
                end: formatNumber(
                  Math.min(offset + page.items.length, page.total),
                  locale
                ),
                total: formatNumber(page.total, locale),
              })}
            </span>
            <Button
              type='button'
              variant='outline'
              disabled={offset === 0 || query.isFetching}
              onClick={() => setOffset(Math.max(0, offset - pageSize))}
            >
              {t('Previous')}
            </Button>
            <Button
              type='button'
              variant='outline'
              disabled={offset + pageSize >= page.total || query.isFetching}
              onClick={() => setOffset(offset + pageSize)}
            >
              {t('Next')}
            </Button>
          </nav>
        </>
      )}
    </section>
  )
}
