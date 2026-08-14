/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export function getHomeFooterVariant(
  footerHtml: string | undefined
): 'custom' | 'molii' {
  return footerHtml?.trim() ? 'custom' : 'molii'
}

export function buildHomeDocsUrl(
  baseUrl: string | undefined,
  path: string
): string | undefined {
  const normalizedBase = baseUrl?.trim()
  if (!normalizedBase) return undefined

  const normalizedPath = path.replace(/^\/+/, '')
  if (normalizedBase.startsWith('/')) {
    if (normalizedBase.startsWith('//')) return undefined
    return `${normalizedBase.replace(/\/+$/, '')}/${normalizedPath}`
  }

  try {
    const parsedBase = new URL(normalizedBase)
    if (!['http:', 'https:'].includes(parsedBase.protocol)) return undefined
    parsedBase.search = ''
    parsedBase.hash = ''
    parsedBase.pathname = `${parsedBase.pathname.replace(/\/+$/, '')}/`
    return new URL(normalizedPath, parsedBase).href
  } catch {
    return undefined
  }
}
