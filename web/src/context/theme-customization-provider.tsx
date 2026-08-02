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
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react'

import {
  type ContentLayout,
  DEFAULT_THEME_CUSTOMIZATION,
  resolveThemeFont,
  type ThemeCustomization,
  type ThemeFont,
  type ThemePreset,
  type ThemeRadius,
  type ThemeScale,
} from '@/lib/theme-customization'

function applyAttribute(name: string, value: string | null) {
  if (typeof document === 'undefined') return
  const body = document.body
  if (!body) return
  if (value === null) {
    body.removeAttribute(name)
  } else {
    body.setAttribute(name, value)
  }
}

type ThemeCustomizationContextType = {
  defaults: ThemeCustomization
  customization: ThemeCustomization
  setPreset: (preset: ThemePreset) => void
  setFont: (font: ThemeFont) => void
  setRadius: (radius: ThemeRadius) => void
  setScale: (scale: ThemeScale) => void
  setContentLayout: (contentLayout: ContentLayout) => void
  resetCustomization: () => void
}

// Fallback used when a consumer renders outside the provider (e.g. an error
// route mounted before providers are ready, or stale HMR boundaries). Keeping
// it permissive prevents the whole tree from crashing — the UI just behaves
// like the defaults until the real provider re-mounts.
const FALLBACK_CONTEXT: ThemeCustomizationContextType = {
  defaults: DEFAULT_THEME_CUSTOMIZATION,
  customization: DEFAULT_THEME_CUSTOMIZATION,
  setPreset: () => {},
  setFont: () => {},
  setRadius: () => {},
  setScale: () => {},
  setContentLayout: () => {},
  resetCustomization: () => {},
}

const ThemeCustomizationContext =
  createContext<ThemeCustomizationContextType>(FALLBACK_CONTEXT)

export function ThemeCustomizationProvider(props: {
  children: React.ReactNode
}) {
  const [preset] = useState<ThemePreset>(DEFAULT_THEME_CUSTOMIZATION.preset)
  const [font] = useState<ThemeFont>(DEFAULT_THEME_CUSTOMIZATION.font)
  const [radius] = useState<ThemeRadius>(DEFAULT_THEME_CUSTOMIZATION.radius)
  const [scale] = useState<ThemeScale>(DEFAULT_THEME_CUSTOMIZATION.scale)
  const [contentLayout] = useState<ContentLayout>(DEFAULT_THEME_CUSTOMIZATION.contentLayout)

  // Mirror state to the <body> via data-* attributes so theme-presets.css can
  // override CSS variables at the right cascade layer.
  useEffect(() => {
    applyAttribute(
      'data-theme-preset',
      preset === DEFAULT_THEME_CUSTOMIZATION.preset ? null : preset
    )
  }, [preset])

  // Font is the one axis where we resolve before writing the attribute:
  // the persisted preference may be `default`, but CSS works in terms of
  // the concrete `sans`/`serif` choice that should drive the cascade.
  // Resolving here (instead of in CSS via `:not()` selectors) keeps the
  // stylesheet to one simple `[data-theme-font='serif']` selector and lets
  // future presets opt into typography via `PRESET_DEFAULT_FONT` alone.
  useEffect(() => {
    applyAttribute('data-theme-font', resolveThemeFont(font, preset))
  }, [font, preset])

  useEffect(() => {
    applyAttribute(
      'data-theme-radius',
      radius === DEFAULT_THEME_CUSTOMIZATION.radius ? null : radius
    )
  }, [radius])

  useEffect(() => {
    applyAttribute(
      'data-theme-scale',
      scale === DEFAULT_THEME_CUSTOMIZATION.scale ? null : scale
    )
  }, [scale])

  useEffect(() => {
    applyAttribute('data-theme-content-layout', contentLayout)
  }, [contentLayout])

  const setPreset = useCallback((_value: ThemePreset) => {}, [])
  const setFont = useCallback((_value: ThemeFont) => {}, [])
  const setRadius = useCallback((_value: ThemeRadius) => {}, [])
  const setScale = useCallback((_value: ThemeScale) => {}, [])
  const setContentLayout = useCallback((_value: ContentLayout) => {}, [])
  const resetCustomization = useCallback(() => {}, [])

  const value = useMemo<ThemeCustomizationContextType>(
    () => ({
      defaults: DEFAULT_THEME_CUSTOMIZATION,
      customization: { preset, font, radius, scale, contentLayout },
      setPreset,
      setFont,
      setRadius,
      setScale,
      setContentLayout,
      resetCustomization,
    }),
    [
      preset,
      font,
      radius,
      scale,
      contentLayout,
      setPreset,
      setFont,
      setRadius,
      setScale,
      setContentLayout,
      resetCustomization,
    ]
  )

  return (
    <ThemeCustomizationContext.Provider value={value}>
      {props.children}
    </ThemeCustomizationContext.Provider>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function useThemeCustomization() {
  return useContext(ThemeCustomizationContext)
}
