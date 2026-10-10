import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Checkbox } from '@/components/ui/checkbox'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import type { CatalogSyncPlan } from './types'

type ConfirmSyncDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  plan: CatalogSyncPlan
  finalPlan: CatalogSyncPlan | null
  confirmDeletes: boolean
  onConfirmDeletesChange: (value: boolean) => void
  onConfirm: () => void
  loading: boolean
  selectedCount: number
}

export function ConfirmSyncDialog(props: ConfirmSyncDialogProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const plan = props.finalPlan ?? props.plan
  const count = (action: string) =>
    plan.changes.filter((change) => change.action === action).length
  const deleteCount =
    count('delete') +
    plan.changes.filter(
      (change) => change.action === 'conflict' && change.after === null
    ).length
  const adoptOnly =
    plan.changes.length > 0 &&
    plan.changes.every((change) =>
      ['adopt', 'preserve', 'unchanged'].includes(change.action)
    ) &&
    count('adopt') > 0
  const expired = plan.expires_at * 1000 <= Date.now()
  const blocked =
    count('blocked') > 0 || (props.finalPlan !== null && !plan.executable)
  const summary = [
    ['Create', count('create')],
    ['Update', count('update')],
    ['Delete', count('delete')],
    ['Adopt', count('adopt')],
    ['Conflict', count('conflict')],
    ['Blocked', count('blocked')],
    ['Preserve', count('preserve')],
  ].filter(([, amount]) => Number(amount) > 0)
  return (
    <ConfirmDialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        props.finalPlan
          ? t('Confirm catalog operation')
          : t('Review catalog choices')
      }
      desc={
        props.finalPlan
          ? t('Verify the exact saved plan before applying it.')
          : t(
              'Save conflict choices and deletion consent before security verification.'
            )
      }
      confirmText={
        props.finalPlan
          ? t(plan.kind === 'restore' ? 'Confirm restore' : 'Confirm sync')
          : t('Save choices')
      }
      destructive={deleteCount > 0}
      disabled={
        expired || blocked || (deleteCount > 0 && !props.confirmDeletes)
      }
      isLoading={props.loading}
      handleConfirm={props.onConfirm}
      className='max-h-[min(90dvh,var(--dialog-available-height))] overflow-y-auto data-[size=default]:max-w-[calc(100%-2rem)] data-[size=default]:sm:max-w-2xl'
    >
      <div className='flex flex-col gap-3 text-sm'>
        <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-1'>
          <dt>{t('Target site')}</dt>
          <dd className='font-medium break-all'>{plan.target_id}</dd>
          <dt>{t('Source identity')}</dt>
          <dd className='break-all'>{plan.source_id}</dd>
          <dt>
            {t(
              plan.kind === 'restore'
                ? 'Inverse snapshot time'
                : 'Dev snapshot time'
            )}
          </dt>
          <dd>
            {new Intl.DateTimeFormat(locale, {
              dateStyle: 'medium',
              timeStyle: 'short',
            }).format(new Date(plan.source_exported_at * 1000))}
          </dd>
          <dt>
            {t(
              plan.kind === 'restore'
                ? 'Inverse snapshot digest'
                : 'Dev snapshot digest'
            )}
          </dt>
          <dd className='font-mono text-xs break-all'>{plan.source_digest}</dd>
          <dt>{t('Final plan digest')}</dt>
          <dd className='font-mono text-xs break-all'>{plan.digest}</dd>
          <dt>{t('Expires')}</dt>
          <dd>
            {new Intl.DateTimeFormat(locale, {
              dateStyle: 'medium',
              timeStyle: 'short',
            }).format(new Date(plan.expires_at * 1000))}
          </dd>
        </dl>
        <p>
          {summary
            .map(
              ([action, amount]) =>
                `${t(String(action))}: ${formatNumber(Number(amount), locale)}`
            )
            .join(' · ')}
        </p>
        <p>
          {t('Conflict groups selected: {{count}}', {
            count: formatNumber(props.selectedCount, locale),
          })}
        </p>
        {deleteCount > 0 && !props.finalPlan && (
          <label className='flex items-start gap-2'>
            <Checkbox
              checked={props.confirmDeletes}
              onCheckedChange={(checked) =>
                props.onConfirmDeletesChange(checked)
              }
            />
            <span>
              {t(
                'I consent to delete {{count}} managed items and accept possible pricing fallback.',
                { count: formatNumber(deleteCount, locale) }
              )}
            </span>
          </label>
        )}
        {deleteCount > 0 && props.finalPlan && (
          <p>{t('Deletion consent recorded')}</p>
        )}
        {adoptOnly && (
          <Alert>
            <AlertTitle>{t('Ownership only')}</AlertTitle>
            <AlertDescription>
              {t(
                'This operation adopts existing items without changing their values; confirmation is still required.'
              )}
            </AlertDescription>
          </Alert>
        )}
        {blocked && (
          <Alert variant='destructive'>
            <AlertTitle>{t('Plan blocked')}</AlertTitle>
            <AlertDescription>
              {t('Resolve blockers or preview again before applying.')}
            </AlertDescription>
          </Alert>
        )}
        {expired && (
          <Alert variant='destructive'>
            <AlertTitle>{t('Preview expired')}</AlertTitle>
            <AlertDescription>
              {t('Check dev updates again to create a fresh preview.')}
            </AlertDescription>
          </Alert>
        )}
      </div>
    </ConfirmDialog>
  )
}
