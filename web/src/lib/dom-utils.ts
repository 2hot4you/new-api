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
import { SITE_BRAND } from '@/config/site-brand'

import type { SiteBrand } from '../../build/site-brand'
import { DEFAULT_FAVICON } from './constants'

export function resolveFaviconUrl(url: string, brand: SiteBrand = SITE_BRAND) {
  try {
    const base =
      typeof window === 'undefined'
        ? 'http://127.0.0.1:3000/'
        : window.location.href
    const parsed = new URL(url, base)
    const brandLogo = new URL(brand.logo, base)
    const localDevelopmentDefault =
      brand.logo.startsWith('/') &&
      parsed.pathname === brandLogo.pathname &&
      (parsed.origin === 'http://127.0.0.1:3000' ||
        parsed.origin === 'http://localhost:3000')
    const sameOriginDefault =
      parsed.origin === brandLogo.origin &&
      parsed.pathname === brandLogo.pathname
    if (sameOriginDefault || localDevelopmentDefault) {
      return brand.favicon
    }
  } catch {
    // Keep malformed custom values unchanged for the caller to reject.
  }
  return url
}

export function applyFaviconToDom(url: string) {
  if (typeof document === 'undefined' || !url) return
  try {
    const faviconUrl = resolveFaviconUrl(url)
    const next = new URL(faviconUrl, window.location.href).href
    const existing =
      document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
    if (existing.length === 1 && existing[0].href === next) return
    const link = document.createElement('link')
    link.rel = 'icon'
    link.href = faviconUrl
    existing.forEach((l) => l.remove())
    document.head.appendChild(link)
  } catch {
    // Ignore malformed URLs
  }
}

export function applySystemFaviconToDom(logo: unknown) {
  const systemLogo = typeof logo === 'string' ? logo.trim() : ''
  applyFaviconToDom(systemLogo || DEFAULT_FAVICON)
}
