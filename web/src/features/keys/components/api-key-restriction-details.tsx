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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { getLobeIcon } from '@/lib/lobe-icon'

import type { ApiKey } from '../types'

export type ApiKeyModelDisplayInfo = {
  modelIcon?: string
  providerIcon?: string
  providerName?: string
}

export type ApiKeyModelDisplayStatus = 'error' | 'loading' | 'ready'

type RestrictionDetailsProps = {
  ariaLabel: string
  count: number
  listKind: 'ips' | 'models'
  title: string
  trigger: ReactNode
  headerAction?: ReactNode
  children: ReactNode
}

function withOccurrenceKeys(values: string[]) {
  const occurrences = new Map<string, number>()
  return values.map((value) => {
    const occurrence = (occurrences.get(value) ?? 0) + 1
    occurrences.set(value, occurrence)
    return { key: `${value}-${occurrence}`, value }
  })
}

function RestrictionDetails(props: RestrictionDetailsProps) {
  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='sm'
            aria-label={props.ariaLabel}
            className='-ml-1.5 h-auto max-w-full min-w-0 justify-start px-0 hover:bg-transparent aria-expanded:bg-transparent'
          />
        }
      >
        {props.trigger}
      </PopoverTrigger>
      <PopoverContent
        data-api-key-restriction-details
        align='start'
        className='max-h-(--available-height) min-h-0 w-[min(26rem,calc(100vw-2rem))] gap-3 overflow-hidden p-3'
      >
        <PopoverHeader>
          <div
            data-api-key-restriction-header
            className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'
          >
            <PopoverTitle>{props.title}</PopoverTitle>
            <div
              data-api-key-restriction-header-actions
              className='flex w-full items-center justify-between gap-2 sm:w-auto sm:shrink-0'
            >
              {props.headerAction}
              <Badge variant='secondary' className='tabular-nums'>
                {props.count}
              </Badge>
            </div>
          </div>
        </PopoverHeader>
        <div
          data-api-key-restriction-list={props.listKind}
          className='min-h-0 flex-1 space-y-1.5 overflow-x-hidden overflow-y-auto pr-1'
        >
          {props.children}
        </div>
      </PopoverContent>
    </Popover>
  )
}

type ModelGroup = {
  key: string
  providerIcon?: string
  providerName: string
  rows: Array<{
    displayInfo?: ApiKeyModelDisplayInfo
    key: string
    model: string
  }>
}

function groupModelRows(
  modelRows: Array<{ key: string; value: string }>,
  modelDisplayInfo: Record<string, ApiKeyModelDisplayInfo> | undefined,
  providerFallback: string
): ModelGroup[] {
  const groups = new Map<string, ModelGroup>()
  for (const row of modelRows) {
    const displayInfo = modelDisplayInfo?.[row.value]
    const providerName = displayInfo?.providerName || providerFallback
    let group = groups.get(providerName)
    if (!group) {
      group = {
        key: providerName,
        providerIcon: displayInfo?.providerIcon,
        providerName,
        rows: [],
      }
      groups.set(providerName, group)
    }
    group.rows.push({
      displayInfo,
      key: row.key,
      model: row.value,
    })
  }
  return [...groups.values()]
}

type ModelLimitsCellProps = {
  apiKey: ApiKey
  modelDisplayInfo?: Record<string, ApiKeyModelDisplayInfo>
  modelDisplayStatus?: ApiKeyModelDisplayStatus
}

export function ModelLimitsCell(props: ModelLimitsCellProps) {
  const { t } = useTranslation()

  if (!props.apiKey.model_limits_enabled || !props.apiKey.model_limits) {
    return (
      <StatusBadge
        label={t('Unlimited')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  const models = props.apiKey.model_limits
    .split(',')
    .map((model) => model.trim())
    .filter(Boolean)
  const modelRows = withOccurrenceKeys(models)
  const displayStatus = props.modelDisplayStatus ?? 'ready'
  let providerFallback = t('Unknown provider')
  if (displayStatus === 'loading') {
    providerFallback = t('Loading model providers')
  } else if (displayStatus === 'error') {
    providerFallback = t('Provider unavailable')
  }
  const modelGroups = groupModelRows(
    modelRows,
    props.modelDisplayInfo,
    providerFallback
  )
  const summaryRows = modelRows.slice(0, 5)

  return (
    <RestrictionDetails
      ariaLabel={`${t('View model restrictions')}: ${t('{{count}} model(s)', {
        count: models.length,
      })}`}
      count={models.length}
      listKind='models'
      title={t('Model restrictions')}
      trigger={
        <span className='flex min-w-0 items-center gap-1'>
          <span className='sr-only'>
            {t('{{count}} model(s)', { count: models.length })}
          </span>
          {summaryRows.map(({ key, value: model }) => {
            const modelIcon = props.modelDisplayInfo?.[model]?.modelIcon
            return (
              <span
                key={key}
                data-model-summary-icon={modelIcon || ''}
                className='shrink-0'
                aria-hidden='true'
              >
                {getLobeIcon(modelIcon, 18)}
              </span>
            )
          })}
          {models.length > 5 && (
            <span
              data-model-summary-overflow
              className='text-muted-foreground pl-0.5 text-sm font-medium'
              aria-hidden='true'
            >
              …
            </span>
          )}
        </span>
      }
      headerAction={
        <CopyButton
          value={models.join(',')}
          variant='outline'
          size='sm'
          className='h-7 gap-1.5 px-2 text-xs'
          iconClassName='size-3.5'
          notify
          aria-label={t('Copy all models')}
        >
          {t('Copy all models')}
        </CopyButton>
      }
    >
      {modelGroups.map((group) => (
        <section
          key={group.key}
          data-model-provider-group
          className='overflow-hidden rounded-lg border'
        >
          <div
            data-provider-icon={group.providerIcon || ''}
            data-model-provider-header
            className='bg-muted/50 text-muted-foreground flex items-center gap-2 border-b px-2.5 py-2 text-xs font-medium'
          >
            <span className='shrink-0' aria-hidden='true'>
              {getLobeIcon(group.providerIcon, 18)}
            </span>
            <span>{group.providerName}</span>
          </div>
          <div data-model-provider-rows className='divide-y'>
            {group.rows.map((row) => (
              <CopyButton
                key={row.key}
                value={row.model}
                variant='ghost'
                size='sm'
                className='hover:bg-muted/40 h-auto w-full min-w-0 justify-start gap-2 rounded-none px-2.5 py-2 font-normal'
                iconClassName='hidden'
                notify
                aria-label={t('Copy model {{model}}', { model: row.model })}
              >
                <span className='shrink-0' aria-hidden='true'>
                  {getLobeIcon(row.displayInfo?.modelIcon, 20)}
                </span>
                <span
                  data-model-restriction-id={row.model}
                  data-model-icon={row.displayInfo?.modelIcon || ''}
                  className='min-w-0 text-left font-mono text-xs break-all whitespace-normal'
                >
                  {row.model}
                </span>
              </CopyButton>
            ))}
          </div>
        </section>
      ))}
    </RestrictionDetails>
  )
}

export function IpRestrictionsCell(props: { apiKey: ApiKey }) {
  const { t } = useTranslation()
  const allowIps = props.apiKey.allow_ips?.trim()

  if (!allowIps) {
    return (
      <StatusBadge
        label={t('No restriction')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  const ips = allowIps
    .split('\n')
    .map((ip) => ip.trim())
    .filter(Boolean)
  const ipRows = withOccurrenceKeys(ips)

  return (
    <RestrictionDetails
      ariaLabel={`${t('View IP restrictions')}: ${ips[0]}, ${t(
        '{{count}} IP(s)',
        { count: ips.length }
      )}`}
      count={ips.length}
      listKind='ips'
      title={t('IP restrictions')}
      trigger={
        <span className='flex min-w-0 items-center gap-1'>
          <StatusBadge label={ips[0]} variant='neutral' copyable={false} />
          {ips.length > 1 && (
            <span
              className='text-muted-foreground text-sm font-medium'
              aria-hidden='true'
            >
              …
            </span>
          )}
          <span className='sr-only'>
            {t('{{count}} IP(s)', { count: ips.length })}
          </span>
        </span>
      }
      headerAction={
        <CopyButton
          value={ips.join(',')}
          variant='outline'
          size='sm'
          className='h-7 gap-1.5 px-2 text-xs'
          iconClassName='size-3.5'
          notify
          aria-label={t('Copy all IPs')}
        >
          {t('Copy all IPs')}
        </CopyButton>
      }
    >
      {ipRows.map(({ key, value: ip }) => (
        <CopyButton
          key={key}
          value={ip}
          variant='outline'
          size='sm'
          className='bg-muted/60 hover:bg-muted h-auto max-w-full gap-1.5 rounded-full border-transparent px-2.5 py-1 font-mono text-xs'
          iconClassName='hidden'
          notify
          aria-label={t('Copy IP {{ip}}', { ip })}
        >
          <span data-ip-restriction-value={ip} className='break-all'>
            {ip}
          </span>
        </CopyButton>
      ))}
    </RestrictionDetails>
  )
}
