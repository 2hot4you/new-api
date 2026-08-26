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
          <div className='flex items-center justify-between gap-3'>
            <PopoverTitle>{props.title}</PopoverTitle>
            <Badge variant='secondary' className='shrink-0 tabular-nums'>
              {props.count}
            </Badge>
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

  return (
    <RestrictionDetails
      ariaLabel={t('View model restrictions')}
      count={models.length}
      listKind='models'
      title={t('Model restrictions')}
      trigger={
        <StatusBadge
          label={t('{{count}} model(s)', { count: models.length })}
          variant='neutral'
          copyable={false}
        />
      }
    >
      {modelRows.map(({ key, value: model }) => {
        const displayInfo = props.modelDisplayInfo?.[model]
        let providerName = displayInfo?.providerName || t('Unknown provider')
        if (displayStatus === 'loading') {
          providerName = t('Loading model providers')
        } else if (displayStatus === 'error') {
          providerName = t('Provider unavailable')
        }

        return (
          <div
            key={key}
            className='bg-muted/30 flex min-w-0 items-center gap-2 rounded-lg border p-2'
          >
            <span className='shrink-0' aria-hidden='true'>
              {getLobeIcon(
                displayInfo?.providerIcon || displayInfo?.providerName,
                20
              )}
            </span>
            <span className='min-w-0 flex-1'>
              <span className='text-muted-foreground block text-xs break-words whitespace-normal'>
                {providerName}
              </span>
              <span
                data-model-restriction-id={model}
                className='block font-mono text-xs break-all whitespace-normal'
              >
                {model}
              </span>
            </span>
          </div>
        )
      })}
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
      ariaLabel={t('View IP restrictions')}
      count={ips.length}
      listKind='ips'
      title={t('IP restrictions')}
      trigger={
        <StatusBadge
          label={t('{{count}} IP(s)', { count: ips.length })}
          variant='neutral'
          copyable={false}
        />
      }
    >
      {ipRows.map(({ key, value: ip }) => (
        <div
          key={key}
          className='bg-muted/30 min-w-0 rounded-lg border px-2.5 py-2 font-mono text-xs break-all'
        >
          {ip}
        </div>
      ))}
    </RestrictionDetails>
  )
}
