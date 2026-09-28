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
import { act, cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  ThemeCustomizationProvider,
  useThemeCustomization,
} from '@/context/theme-customization-provider'
import { ThemeProvider, useTheme } from '@/context/theme-provider'
import { initializeFrontendCache } from '@/lib/frontend-cache'
import {
  DEFAULT_THEME_CUSTOMIZATION,
  resolveThemeFont,
} from '@/lib/theme-customization'
import {
  readThemePreference,
  THEME_STORAGE_KEYS,
  writeThemePreference,
} from '@/lib/theme-storage'

const savedPreferences = {
  [THEME_STORAGE_KEYS.mode]: 'dark',
  [THEME_STORAGE_KEYS.preset]: 'rose-garden',
  [THEME_STORAGE_KEYS.font]: 'serif',
  [THEME_STORAGE_KEYS.radius]: 'lg',
  [THEME_STORAGE_KEYS.scale]: 'lg',
  [THEME_STORAGE_KEYS.contentLayout]: 'centered',
}

function ThemeControls() {
  const theme = useTheme()
  const customization = useThemeCustomization()

  return (
    <>
      <output aria-label='Theme mode'>{theme.theme}</output>
      <output aria-label='Resolved theme'>{theme.resolvedTheme}</output>
      <output aria-label='Theme customization'>
        {JSON.stringify(customization.customization)}
      </output>
      <button
        type='button'
        onClick={() => {
          theme.setTheme('dark')
          customization.setPreset('rose-garden')
          customization.setFont('serif')
          customization.setRadius('lg')
          customization.setScale('lg')
          customization.setContentLayout('centered')
        }}
      >
        Customize
      </button>
      <button
        type='button'
        onClick={() => {
          theme.resetTheme()
          customization.resetCustomization()
        }}
      >
        Reset
      </button>
    </>
  )
}

function ThemeFixture() {
  return (
    <ThemeProvider defaultTheme='dark'>
      <ThemeCustomizationProvider>
        <ThemeControls />
      </ThemeCustomizationProvider>
    </ThemeProvider>
  )
}

function expectLockedDefaults(resolvedTheme = 'light') {
  expect(screen.getByLabelText('Theme mode')).toHaveTextContent('system')
  expect(screen.getByLabelText('Resolved theme')).toHaveTextContent(
    resolvedTheme
  )
  expect(screen.getByLabelText('Theme customization')).toHaveTextContent(
    JSON.stringify(DEFAULT_THEME_CUSTOMIZATION)
  )
  expect(document.documentElement).toHaveClass(resolvedTheme)
  expect(document.body).not.toHaveAttribute('data-theme-preset')
  expect(document.body).toHaveAttribute(
    'data-theme-font',
    resolveThemeFont(
      DEFAULT_THEME_CUSTOMIZATION.font,
      DEFAULT_THEME_CUSTOMIZATION.preset
    )
  )
  expect(document.body).not.toHaveAttribute('data-theme-radius')
  expect(document.body).not.toHaveAttribute('data-theme-scale')
  expect(document.body).toHaveAttribute(
    'data-theme-content-layout',
    DEFAULT_THEME_CUSTOMIZATION.contentLayout
  )
}

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  localStorage.clear()
  for (const cookie of document.cookie.split(';')) {
    const name = cookie.trim().split('=')[0]
    document.cookie = `${name}=; path=/; max-age=0`
  }
  document.documentElement.classList.remove('light', 'dark')
  for (const name of document.body.getAttributeNames()) {
    if (name.startsWith('data-theme-')) document.body.removeAttribute(name)
  }
})

describe('Molii locked theme contract', () => {
  it('ignores legacy cookies, local storage, and the defaultTheme prop', () => {
    for (const [key, value] of Object.entries(savedPreferences)) {
      localStorage.setItem(key, value)
    }
    document.cookie = 'theme_preset=ocean-breeze; path=/'
    document.cookie = 'vite-ui-theme=dark; path=/'
    document.cookie = 'theme_font=serif; path=/'

    render(<ThemeFixture />)

    expectLockedDefaults()
  })

  it('keeps setters and reset as no-ops without changing stored values', async () => {
    for (const [key, value] of Object.entries(savedPreferences)) {
      localStorage.setItem(key, value)
    }
    const user = userEvent.setup()
    render(<ThemeFixture />)

    await user.click(screen.getByRole('button', { name: 'Customize' }))
    await user.click(screen.getByRole('button', { name: 'Reset' }))

    expectLockedDefaults()
    for (const [key, value] of Object.entries(savedPreferences)) {
      expect(localStorage.getItem(key)).toBe(value)
    }
  })

  it('continues following operating-system color changes', () => {
    let prefersDark = false
    let changeListener: (() => void) | undefined
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (query) =>
        ({
          matches: prefersDark,
          media: query,
          onchange: null,
          addListener: () => undefined,
          removeListener: () => undefined,
          addEventListener: (
            _type: string,
            listener: EventListenerOrEventListenerObject
          ) => {
            changeListener = listener as () => void
          },
          removeEventListener: () => undefined,
          dispatchEvent: () => false,
        }) as MediaQueryList
    )
    render(<ThemeFixture />)
    expectLockedDefaults()

    prefersDark = true
    act(() => changeListener?.())

    expectLockedDefaults('dark')
  })

  it('preserves storage-key whitelist entries while clearing stale UI cache', () => {
    for (const [key, value] of Object.entries(savedPreferences)) {
      localStorage.setItem(key, value)
    }
    localStorage.setItem('stale-ui-cache', 'old')

    initializeFrontendCache()
    initializeFrontendCache()
    render(<ThemeFixture />)

    expectLockedDefaults()
    for (const [key, value] of Object.entries(savedPreferences)) {
      expect(localStorage.getItem(key)).toBe(value)
    }
    expect(localStorage.getItem('stale-ui-cache')).toBeNull()
  })
})

describe('theme storage helpers', () => {
  it('accepts only allowed values and falls back for legacy or invalid data', () => {
    const allowed = new Set(['system', 'light', 'dark'] as const)

    localStorage.setItem(THEME_STORAGE_KEYS.mode, 'dark')
    expect(
      readThemePreference(THEME_STORAGE_KEYS.mode, allowed, 'system')
    ).toBe('dark')

    localStorage.setItem(THEME_STORAGE_KEYS.mode, 'legacy-dark')
    expect(
      readThemePreference(THEME_STORAGE_KEYS.mode, allowed, 'system')
    ).toBe('system')
  })

  it('fails safely when browser storage is unavailable', () => {
    vi.spyOn(localStorage, 'getItem').mockImplementation(() => {
      throw new DOMException('Storage unavailable', 'SecurityError')
    })
    vi.spyOn(localStorage, 'setItem').mockImplementation(() => {
      throw new DOMException('Storage full', 'QuotaExceededError')
    })
    vi.spyOn(localStorage, 'removeItem').mockImplementation(() => {
      throw new DOMException('Storage unavailable', 'SecurityError')
    })

    expect(
      readThemePreference(
        THEME_STORAGE_KEYS.mode,
        new Set(['system'] as const),
        'system'
      )
    ).toBe('system')
    expect(() =>
      writeThemePreference(THEME_STORAGE_KEYS.mode, 'system')
    ).not.toThrow()
    expect(() =>
      writeThemePreference(THEME_STORAGE_KEYS.mode, null)
    ).not.toThrow()
  })
})
