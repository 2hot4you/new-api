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
import { useState } from 'react'

import { cn } from '@/lib/utils'

export const CLAUDEYE_WORDMARK_FALLBACK = '/claudeye-wordmark-neutral.png'

export type ClaudeyeWordmarkSurface = 'light' | 'dark'

// eslint-disable-next-line react/only-export-components -- shared by branding previews
export function claudeyeWordmarkUrl(surface: ClaudeyeWordmarkSurface): string {
  return `/api/branding/claudeye/wordmark.svg?surface=${surface}`
}

interface ClaudeyeWordmarkProps extends Omit<
  React.ImgHTMLAttributes<HTMLImageElement>,
  'onError' | 'src'
> {
  alt: string
  surface: ClaudeyeWordmarkSurface
}

export function ClaudeyeWordmark({
  alt,
  className,
  surface,
  ...props
}: ClaudeyeWordmarkProps) {
  const [useFallback, setUseFallback] = useState(false)

  return (
    <img
      data-claudeye-wordmark
      src={
        useFallback ? CLAUDEYE_WORDMARK_FALLBACK : claudeyeWordmarkUrl(surface)
      }
      alt={alt}
      width={1995}
      height={440}
      className={cn('w-auto object-contain', className)}
      onError={useFallback ? undefined : () => setUseFallback(true)}
      {...props}
    />
  )
}
