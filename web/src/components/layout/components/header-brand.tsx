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
import { Skeleton } from '@/components/ui/skeleton'
import { DEFAULT_LOGO } from '@/lib/constants'

import { HeaderLogo } from './header-logo'

const MOLII_WORDMARK = '/molii-wordmark.png'

export interface HeaderBrandProps {
  systemLogo: string
  siteName: string
  loading: boolean
  logoLoaded: boolean
  customLogo?: React.ReactNode
}

export function HeaderBrand({
  systemLogo,
  siteName,
  loading,
  logoLoaded,
  customLogo,
}: HeaderBrandProps) {
  const useMoliiWordmark = !customLogo && systemLogo === DEFAULT_LOGO

  if (useMoliiWordmark) {
    if (loading) {
      return (
        <Skeleton
          data-header-wordmark-skeleton='true'
          className='h-7 w-[4.375rem] rounded-md'
        />
      )
    }

    return (
      <img
        data-header-wordmark='true'
        src={MOLII_WORDMARK}
        alt={siteName}
        width={375}
        height={150}
        className='h-7 w-auto max-w-[4.375rem] object-contain transition-transform duration-300 group-hover:scale-105'
      />
    )
  }

  let logoContent: React.ReactNode
  if (loading) {
    logoContent = <Skeleton className='size-full rounded-lg' />
  } else if (customLogo) {
    logoContent = customLogo
  } else {
    logoContent = (
      <HeaderLogo
        src={systemLogo}
        alt={siteName}
        loading={loading}
        logoLoaded={logoLoaded}
        className='size-full rounded-lg object-contain'
      />
    )
  }

  return (
    <>
      <div className='flex size-7 shrink-0 items-center justify-center transition-transform duration-300 group-hover:scale-105'>
        {logoContent}
      </div>
      <span
        data-header-site-name='true'
        className='text-sm font-semibold tracking-tight'
      >
        {loading ? <Skeleton className='h-4 w-16' /> : siteName}
      </span>
    </>
  )
}
