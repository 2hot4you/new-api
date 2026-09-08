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
import type { QuotationSnapshot } from '../types'
import { sanitizeQuotationFilename } from './quotation-format'
import { buildQuotationHtml } from './quotation-html'

export function downloadQuotationHtml(snapshot: QuotationSnapshot): void {
  const blob = new Blob([buildQuotationHtml(snapshot)], {
    type: 'text/html;charset=utf-8',
  })
  const objectUrl = URL.createObjectURL(blob)
  const anchor = document.createElement('a')

  try {
    anchor.href = objectUrl
    anchor.download = sanitizeQuotationFilename(
      snapshot.title,
      snapshot.quoteDate
    )
    anchor.rel = 'noopener'
    anchor.style.display = 'none'
    document.body.append(anchor)
    anchor.click()
  } finally {
    anchor.remove()
    URL.revokeObjectURL(objectUrl)
  }
}
