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
import type { QuoteCurrency } from '../types'

const MAX_FILENAME_LENGTH = 120
const HTML_EXTENSION = '.html'
const CONTROL_CHARACTERS = /\p{Cc}/gu
const RESERVED_FILENAME_CHARACTERS = /[/\\:*?"<>|]/g

export function formatQuoteAmount(
  amount: number | null | undefined,
  currency: QuoteCurrency
): string {
  if (typeof amount !== 'number' || !Number.isFinite(amount)) return '—'

  const formatted = new Intl.NumberFormat('en-US', {
    maximumFractionDigits: 8,
  }).format(amount)
  return `${currency === 'CNY' ? '¥' : '$'}${formatted}`
}

function sanitizeFilenamePart(value: string): string {
  return value
    .replaceAll(CONTROL_CHARACTERS, '')
    .replaceAll(RESERVED_FILENAME_CHARACTERS, '')
    .trim()
    .replaceAll(/[. ]+$/g, '')
}

export function sanitizeQuotationFilename(title: string, date: string): string {
  const safeDate = sanitizeFilenamePart(date)
  const suffix = `${safeDate ? `-${safeDate}` : ''}${HTML_EXTENSION}`
  const maximumTitleLength = Math.max(0, MAX_FILENAME_LENGTH - suffix.length)
  let safeTitle = sanitizeFilenamePart(title) || 'quotation'
  safeTitle = sanitizeFilenamePart(safeTitle.slice(0, maximumTitleLength))
  if (!safeTitle) safeTitle = 'quotation'.slice(0, maximumTitleLength)
  return `${safeTitle}${suffix}`
}
