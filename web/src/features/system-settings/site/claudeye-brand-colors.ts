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
export const CLAUDEYE_BRAND_DEFAULTS = {
  'brand_setting.claudeye_light_mark_color': '#242424',
  'brand_setting.claudeye_light_text_color': '#6A6A6A',
  'brand_setting.claudeye_dark_mark_color': '#FFFFFF',
  'brand_setting.claudeye_dark_text_color': '#B8B8B8',
} as const

export type ClaudeyeBrandColorKey = keyof typeof CLAUDEYE_BRAND_DEFAULTS
export type ClaudeyeBrandSurface = 'light' | 'dark'
export type ClaudeyePreviewColors = { mark: string; text: string }

export const BRAND_HEX_PATTERN = /^#[0-9A-Fa-f]{6}$/

export function normalizeBrandHex(value: string): string | null {
  const trimmed = value.trim()
  return BRAND_HEX_PATTERN.test(trimmed) ? trimmed.toUpperCase() : null
}

export function buildClaudeyePreviewUrl(
  surface: ClaudeyeBrandSurface,
  mark: string,
  text: string,
  lastValid?: ClaudeyePreviewColors
): string | null {
  const currentMark = normalizeBrandHex(mark)
  const currentText = normalizeBrandHex(text)
  const fallbackMark = lastValid ? normalizeBrandHex(lastValid.mark) : null
  const fallbackText = lastValid ? normalizeBrandHex(lastValid.text) : null
  const resolvedMark = currentMark && currentText ? currentMark : fallbackMark
  const resolvedText = currentMark && currentText ? currentText : fallbackText

  if (!resolvedMark || !resolvedText) return null

  const search = new URLSearchParams({
    surface,
    mark: resolvedMark,
    text: resolvedText,
  })
  return `/api/branding/claudeye/wordmark.svg?${search.toString()}`
}

function relativeLuminance(color: string): number {
  const normalized = normalizeBrandHex(color)
  if (!normalized) {
    throw new Error('Color must use #RRGGBB format')
  }

  const channels = [1, 3, 5].map((offset) => {
    const value =
      Number.parseInt(normalized.slice(offset, offset + 2), 16) / 255
    return value <= 0.04045
      ? value / 12.92
      : Math.pow((value + 0.055) / 1.055, 2.4)
  })

  return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722
}

export function contrastRatio(first: string, second: string): number {
  const firstLuminance = relativeLuminance(first)
  const secondLuminance = relativeLuminance(second)
  const lighter = Math.max(firstLuminance, secondLuminance)
  const darker = Math.min(firstLuminance, secondLuminance)
  return (lighter + 0.05) / (darker + 0.05)
}
