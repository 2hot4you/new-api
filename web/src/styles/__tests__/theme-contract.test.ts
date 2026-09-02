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
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, test } from 'vitest'

import { Window } from 'happy-dom'

const themeCss = readFileSync(resolve(process.cwd(), 'src/styles/theme.css'), 'utf8')

const LIGHT_CHARCOAL_TOKENS = {
  '--primary': 'oklch(0.28 0 0)',
  '--primary-foreground': 'oklch(0.985 0 0)',
  '--secondary-foreground': 'oklch(0.28 0 0)',
  '--ring': 'oklch(0.5 0 0)',
  '--sidebar-primary': 'oklch(0.28 0 0)',
  '--sidebar-primary-foreground': 'oklch(0.985 0 0)',
  '--sidebar-accent-foreground': 'oklch(0.22 0 0)',
  '--sidebar-ring': 'oklch(0.5 0 0)',
} as const

const DARK_CHARCOAL_TOKENS = {
  '--primary': 'oklch(0.82 0 0)',
  '--primary-foreground': 'oklch(0.2 0 0)',
  '--secondary-foreground': 'oklch(0.9 0 0)',
  '--ring': 'oklch(0.7 0 0)',
  '--sidebar-primary': 'oklch(0.82 0 0)',
  '--sidebar-primary-foreground': 'oklch(0.2 0 0)',
  '--sidebar-accent-foreground': 'oklch(0.95 0 0)',
  '--sidebar-ring': 'oklch(0.7 0 0)',
} as const

const COLORED_SEMANTIC_TOKENS = [
  '--success',
  '--warning',
  '--info',
  '--chart-1',
] as const

function createThemeWindow() {
  const window = new Window()
  const style = window.document.createElement('style')
  style.textContent = themeCss
  window.document.head.append(style)
  return window
}

function readToken(window: Window, name: string): string {
  return window
    .getComputedStyle(window.document.documentElement)
    .getPropertyValue(name)
    .trim()
}

function readOklchChroma(value: string): number {
  const match = /^oklch\(\s*[\d.]+\s+([\d.]+)/.exec(value)
  assert.ok(match, `expected an OKLCH color, received ${value}`)
  return Number(match[1])
}

describe('system theme color contract', () => {
  test('uses the approved charcoal interaction palette in light and dark modes', () => {
    const window = createThemeWindow()

    for (const [name, expected] of Object.entries(LIGHT_CHARCOAL_TOKENS)) {
      assert.equal(readToken(window, name), expected, `light ${name}`)
    }

    window.document.documentElement.classList.add('dark')

    for (const [name, expected] of Object.entries(DARK_CHARCOAL_TOKENS)) {
      assert.equal(readToken(window, name), expected, `dark ${name}`)
    }

    window.close()
  })

  test('keeps semantic states and charts chromatic in both modes', () => {
    const window = createThemeWindow()

    for (const name of COLORED_SEMANTIC_TOKENS) {
      assert.ok(readOklchChroma(readToken(window, name)) > 0, `light ${name}`)
    }

    window.document.documentElement.classList.add('dark')

    for (const name of COLORED_SEMANTIC_TOKENS) {
      assert.ok(readOklchChroma(readToken(window, name)) > 0, `dark ${name}`)
    }

    window.close()
  })
})
