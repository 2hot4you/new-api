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
import { useTranslation } from 'react-i18next'

import { BadgeCell, TruncatedCell } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import { getLobeIcon } from '@/lib/lobe-icon'

import {
  ApiKeyAutoGroupDetails,
  type ApiKeyGroupDataStatus,
  type ApiKeyGroupDisplayInfo,
} from './api-key-auto-group-details'
import {
  AutoGroupBadge,
  GroupRatioBadge,
  type GroupRatio,
} from './auto-group-visuals'

type ApiKeyGroupCellProps = {
  autoGroups?: string[] | null
  crossGroupRetry: boolean
  defaultAutoGroups?: string[]
  defaultAutoGroupsStatus?: ApiKeyGroupDataStatus
  group: string
  groupDataStatus?: ApiKeyGroupDataStatus
  groupDisplayInfo?: Record<string, ApiKeyGroupDisplayInfo>
  ratio?: GroupRatio
  icon?: string
  shouldReduceMotion: boolean
}

export function ApiKeyGroupCell(props: ApiKeyGroupCellProps) {
  const { t } = useTranslation()

  if (props.group !== 'auto') {
    const ratio = typeof props.ratio === 'number' ? props.ratio : undefined
    return (
      <TruncatedCell
        className='-ml-1.5'
        tooltipContent={props.group || '-'}
        tooltipClassName='break-all'
      >
        <span className='flex min-w-0 items-center gap-1.5'>
          {props.icon && (
            <span
              className='shrink-0'
              data-api-key-group-icon='table'
              data-icon-key={props.icon}
            >
              {getLobeIcon(props.icon, 16)}
            </span>
          )}
          <GroupBadge group={props.group} ratio={ratio} />
        </span>
      </TruncatedCell>
    )
  }

  return (
    <ApiKeyAutoGroupDetails
      autoGroups={props.autoGroups}
      defaultAutoGroups={props.defaultAutoGroups}
      defaultAutoGroupsStatus={props.defaultAutoGroupsStatus}
      groupDataStatus={props.groupDataStatus}
      groupDisplayInfo={props.groupDisplayInfo}
      trigger={
        <BadgeCell
          data-api-key-group-cell='auto'
          className='ml-0 gap-1.5 overflow-visible text-xs'
        >
          <StatusBadge
            label={t('Cross-group')}
            variant='info'
            copyable={false}
          />
          <AutoGroupBadge shouldReduceMotion={props.shouldReduceMotion} />
          <GroupRatioBadge
            ratio={props.ratio}
            isAuto
            shouldReduceMotion={props.shouldReduceMotion}
          />
        </BadgeCell>
      }
    />
  )
}
