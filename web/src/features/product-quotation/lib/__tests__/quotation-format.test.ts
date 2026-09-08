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
import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import {
  formatQuoteAmount,
  sanitizeQuotationFilename,
} from '../quotation-format'

describe('quotation amount formatting', () => {
  test('formats USD, direct CNY, explicit zero, and tiny non-zero values', () => {
    assert.equal(formatQuoteAmount(12.5, 'USD'), '$12.5')
    assert.equal(formatQuoteAmount(12.5, 'CNY'), '¥12.5')
    assert.equal(formatQuoteAmount(0, 'USD'), '$0')
    assert.equal(formatQuoteAmount(0.00000002, 'USD'), '$0.00000002')
  })

  test('does not present unavailable or non-finite amounts as zero', () => {
    assert.equal(formatQuoteAmount(null, 'USD'), '—')
    assert.equal(formatQuoteAmount(Number.NaN, 'CNY'), '—')
    assert.equal(formatQuoteAmount(Number.POSITIVE_INFINITY, 'USD'), '—')
  })
})

describe('quotation filename sanitation', () => {
  test('removes controls and reserved filename characters and trims trailing dots', () => {
    assert.equal(
      sanitizeQuotationFilename('  Acme\u0000 / Q3:*?"<>|...  ', '2026-09-08'),
      'Acme  Q3-2026-09-08.html'
    )
  })

  test('falls back to quotation and keeps the complete filename within 120 characters', () => {
    assert.equal(
      sanitizeQuotationFilename(' /\\:*?"<>|. ', '2026-09-08'),
      'quotation-2026-09-08.html'
    )
    const filename = sanitizeQuotationFilename('a'.repeat(200), '2026-09-08')
    assert.equal(filename.length, 120)
    assert.ok(filename.endsWith('-2026-09-08.html'))
  })
})
