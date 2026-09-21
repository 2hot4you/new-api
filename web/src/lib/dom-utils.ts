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

function setFaviconMetadata(
  link: HTMLLinkElement,
  url: string,
  kind: 'custom' | 'dynamic' | 'fallback'
) {
  if (kind === 'dynamic') {
    link.type = 'image/svg+xml'
    link.setAttribute('sizes', 'any')
    return
  }

  const pathname = new URL(
    url,
    link.ownerDocument.baseURI
  ).pathname.toLowerCase()
  if (kind === 'fallback' && pathname.endsWith('.png')) {
    link.type = 'image/png'
    link.setAttribute('sizes', '32x32')
    return
  }

  link.removeAttribute('type')
  link.removeAttribute('sizes')
}

type FaviconDocumentState = {
  failedUrls: Set<string>
  pending?: { url: string; version: number }
  version: number
}

const faviconDocumentStates = new WeakMap<Document, FaviconDocumentState>()

function getFaviconDocumentState(document: Document): FaviconDocumentState {
  let state = faviconDocumentStates.get(document)
  if (!state) {
    state = { failedUrls: new Set(), version: 0 }
    faviconDocumentStates.set(document, state)
  }
  return state
}

function createFaviconLink(
  document: Document,
  url: string,
  kind: 'custom' | 'dynamic' | 'fallback'
) {
  const link = document.createElement('link')
  link.rel = 'icon'
  link.href = url
  setFaviconMetadata(link, url, kind)
  return link
}

function installOnlyFavicon(
  document: Document,
  url: string,
  kind: 'custom' | 'fallback'
) {
  const existing =
    document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
  const next = new URL(url, document.baseURI).href
  const reusable = [...existing].find((link) => link.href === next)

  existing.forEach((link) => {
    if (link !== reusable) link.remove()
  })

  const link = reusable || createFaviconLink(document, url, kind)
  setFaviconMetadata(link, url, kind)
  if (!link.isConnected) document.head.appendChild(link)
  return link
}

function probeFaviconImage(
  document: Document,
  url: string,
  onLoad: () => void,
  onError: () => void
) {
  const image = document.createElement('img')
  image.addEventListener('load', onLoad, { once: true })
  image.addEventListener('error', onError, { once: true })
  image.src = url
}

export function applyFaviconToDom(url: string, brand: SiteBrand = SITE_BRAND) {
  if (typeof document === 'undefined' || !url) return
  try {
    const activeDocument = document
    const state = getFaviconDocumentState(activeDocument)
    const faviconUrl = resolveFaviconUrl(url, brand)
    const next = new URL(faviconUrl, window.location.href).href
    const fallbackUrl = brand.faviconFallback || brand.favicon
    const dynamicSvg =
      faviconUrl === brand.favicon && fallbackUrl !== brand.favicon

    if (dynamicSvg) {
      const existing =
        activeDocument.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
      const fallback = new URL(fallbackUrl, activeDocument.baseURI).href
      const hasFallback = [...existing].some((link) => link.href === fallback)
      const hasDynamic = [...existing].some((link) => link.href === next)

      if (hasFallback && hasDynamic) {
        state.version += 1
        state.pending = undefined
        return
      }

      installOnlyFavicon(activeDocument, fallbackUrl, 'fallback')

      if (state.failedUrls.has(next) || state.pending?.url === next) return

      const version = ++state.version
      state.pending = { url: next, version }
      probeFaviconImage(
        activeDocument,
        next,
        () => {
          if (
            state.version !== version ||
            state.pending?.url !== next ||
            state.pending.version !== version
          ) {
            return
          }
          state.pending = undefined
          installOnlyFavicon(activeDocument, fallbackUrl, 'fallback')
          activeDocument.head.appendChild(
            createFaviconLink(activeDocument, faviconUrl, 'dynamic')
          )
        },
        () => {
          if (
            state.version !== version ||
            state.pending?.url !== next ||
            state.pending.version !== version
          ) {
            return
          }
          state.pending = undefined
          state.failedUrls.add(next)
        }
      )
      return
    }

    state.version += 1
    state.pending = undefined
    const existing =
      activeDocument.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
    if (existing.length === 1 && existing[0].href === next) return

    installOnlyFavicon(
      activeDocument,
      faviconUrl,
      faviconUrl === brand.favicon ? 'fallback' : 'custom'
    )
  } catch {
    // Ignore malformed URLs
  }
}

export function applySystemFaviconToDom(logo: unknown) {
  const systemLogo = typeof logo === 'string' ? logo.trim() : ''
  applyFaviconToDom(systemLogo || DEFAULT_FAVICON)
}
