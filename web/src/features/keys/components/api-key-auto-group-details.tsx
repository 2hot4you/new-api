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
import { Star } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { getLobeIcon } from '@/lib/lobe-icon'

import { GroupRatioBadge, type GroupRatio } from './auto-group-visuals'

export type ApiKeyGroupDisplayInfo = {
  desc?: string
  icon?: string
  ratio?: GroupRatio
  recommendation?: number
}

export type ApiKeyGroupDataStatus = 'error' | 'loading' | 'ready'

type ApiKeyAutoGroupDetailsProps = {
  autoGroups?: string[] | null
  defaultAutoGroups?: string[]
  defaultAutoGroupsStatus?: ApiKeyGroupDataStatus
  groupDataStatus?: ApiKeyGroupDataStatus
  groupDisplayInfo?: Record<string, ApiKeyGroupDisplayInfo>
  trigger: ReactNode
}

function RecommendationBadge(props: { score?: number }) {
  const { t } = useTranslation()
  if (
    props.score === undefined ||
    !Number.isFinite(props.score) ||
    props.score <= 0 ||
    props.score > 5
  ) {
    return null
  }

  return (
    <Badge
      variant='warning'
      aria-label={`${t('Recommendation')} ${props.score.toFixed(1)}`}
      className='gap-1 px-1.5 text-[10px]'
    >
      <Star aria-hidden='true' className='size-2.5 fill-current' />
      {props.score.toFixed(1)}
    </Badge>
  )
}

export function ApiKeyAutoGroupDetails(props: ApiKeyAutoGroupDetailsProps) {
  const { t } = useTranslation()
  const usesSystemDefault = !props.autoGroups?.length
  const orderedGroups = usesSystemDefault
    ? (props.defaultAutoGroups ?? [])
    : (props.autoGroups ?? [])
  let detailsStatus = props.groupDataStatus ?? 'ready'
  if (detailsStatus === 'ready' && usesSystemDefault) {
    detailsStatus = props.defaultAutoGroupsStatus ?? 'ready'
  }

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='sm'
            aria-label={t('View cross-group details')}
            className='h-auto max-w-full min-w-0 justify-start px-0 hover:bg-transparent aria-expanded:bg-transparent'
          />
        }
      >
        {props.trigger}
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='max-h-(--available-height) min-h-0 w-[min(24rem,calc(100vw-2rem))] gap-3 overflow-hidden p-3'
        data-api-key-auto-group-details='true'
      >
        <PopoverHeader>
          <div className='flex items-center justify-between gap-3'>
            <PopoverTitle>{t('Group failover order')}</PopoverTitle>
            {detailsStatus === 'ready' && (
              <Badge variant='secondary' className='shrink-0 tabular-nums'>
                {t('{{count}} groups', { count: orderedGroups.length })}
              </Badge>
            )}
          </div>
          <PopoverDescription>
            {detailsStatus === 'loading' && t('Loading...')}
            {detailsStatus === 'error' && t('Failed to load')}
            {detailsStatus === 'ready' && usesSystemDefault
              ? t('Uses system default order')
              : null}
            {detailsStatus === 'ready' && !usesSystemDefault
              ? t(
                  'Automatically selects the best available group with circuit breaker mechanism'
                )
              : null}
          </PopoverDescription>
        </PopoverHeader>

        <div
          data-auto-group-detail-list='true'
          className='min-h-0 flex-1 space-y-1.5 overflow-x-hidden overflow-y-auto pr-1'
        >
          {detailsStatus === 'loading' && (
            <p className='text-muted-foreground py-5 text-center text-sm'>
              {t('Loading...')}
            </p>
          )}
          {detailsStatus === 'error' && (
            <p className='text-destructive py-5 text-center text-sm'>
              {t('Failed to load')}
            </p>
          )}
          {detailsStatus === 'ready' && orderedGroups.length === 0 && (
            <p className='text-muted-foreground py-5 text-center text-sm'>
              {t('No groups configured.')}
            </p>
          )}
          {detailsStatus === 'ready' &&
            orderedGroups.map((group, index) => {
              const info = props.groupDisplayInfo?.[group]
              return (
                <div
                  key={group}
                  data-auto-group-detail={group}
                  className='bg-muted/30 flex min-w-0 items-center gap-2 rounded-lg border p-2'
                >
                  <span className='bg-primary/10 text-primary flex size-5 shrink-0 items-center justify-center rounded-full text-xs font-semibold tabular-nums'>
                    {index + 1}
                  </span>
                  {info?.icon && (
                    <span className='shrink-0' aria-hidden='true'>
                      {getLobeIcon(info.icon, 20)}
                    </span>
                  )}
                  <span className='min-w-0 flex-1'>
                    <span className='block truncate text-sm font-medium'>
                      {group}
                    </span>
                    {info?.desc && (
                      <span className='text-muted-foreground block truncate text-xs'>
                        {info.desc}
                      </span>
                    )}
                  </span>
                  {info ? (
                    <span className='flex shrink-0 items-center gap-1'>
                      <RecommendationBadge score={info.recommendation} />
                      <GroupRatioBadge ratio={info.ratio} />
                    </span>
                  ) : (
                    <StatusBadge
                      label={t('Unavailable')}
                      variant='danger'
                      copyable={false}
                    />
                  )}
                </div>
              )
            })}
        </div>
      </PopoverContent>
    </Popover>
  )
}
