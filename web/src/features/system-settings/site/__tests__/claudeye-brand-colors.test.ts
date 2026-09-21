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
import { describe, expect, test } from 'vitest'

import {
  buildClaudeyePreviewUrl,
  contrastRatio,
  normalizeBrandHex,
} from '../claudeye-brand-colors'

describe('claudeye brand color helpers', () => {
  test('normalizes complete HEX colors and rejects partial or malformed input', () => {
    expect(normalizeBrandHex(' #a1b2c3 ')).toBe('#A1B2C3')

    for (const value of ['', 'A1B2C3', '#abc', '#GG0000', '#11223344']) {
      expect(normalizeBrandHex(value)).toBeNull()
    }
  })

  test('builds preview URLs with URL-encoded uppercase overrides', () => {
    expect(buildClaudeyePreviewUrl('dark', '#ffffff', '#b8b8b8')).toBe(
      '/api/branding/claudeye/wordmark.svg?surface=dark&mark=%23FFFFFF&text=%23B8B8B8'
    )
  })

  test('retains the last valid preview URL while either input is partial', () => {
    const lastValid = { mark: '#242424', text: '#6A6A6A' }

    expect(buildClaudeyePreviewUrl('light', '#123', '#abcdef', lastValid)).toBe(
      '/api/branding/claudeye/wordmark.svg?surface=light&mark=%23242424&text=%236A6A6A'
    )
  })

  test('calculates WCAG contrast ratios for light and dark surfaces', () => {
    expect(contrastRatio('#FFFFFF', '#171717')).toBeGreaterThan(4.5)
    expect(contrastRatio('#FFFFFF', '#FFFFFF')).toBe(1)
  })
})
