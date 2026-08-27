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

import { CopyButton } from '@/components/copy-button'
import type { ApiInfoItem } from '@/features/dashboard/types'

export function ApiKeyApiRoutes(props: { routes: ApiInfoItem[] }) {
  const { t } = useTranslation()

  if (props.routes.length === 0) return null

  return (
    <span className='inline-flex min-w-0 flex-wrap items-center gap-1.5'>
      <span className='text-muted-foreground text-[11px] font-medium whitespace-nowrap'>
        {t('API Info')}
      </span>
      {props.routes.map((route) => (
        <CopyButton
          key={`${route.route}-${route.url}`}
          value={route.url}
          variant='outline'
          size='sm'
          notify
          tooltip={t('Copy URL')}
          successTooltip={t('Copied!')}
          aria-label={`${t('Copy URL')}: ${route.route}`}
          className='h-7 max-w-full gap-1.5 rounded-full px-2.5 text-xs font-normal'
          iconClassName='size-3'
        >
          <span className='shrink-0 font-medium'>{route.route}</span>
          <span className='text-muted-foreground max-w-36 truncate font-mono text-[10px] sm:max-w-52 lg:max-w-64'>
            {route.url}
          </span>
        </CopyButton>
      ))}
    </span>
  )
}
