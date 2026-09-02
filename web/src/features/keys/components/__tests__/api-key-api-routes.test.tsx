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
import { afterAll as after, afterEach, describe, test } from 'vitest'

import { Window } from 'happy-dom'

const domWindow = new Window()
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'MouseEvent',
  'PointerEvent',
  'FocusEvent',
  'CustomEvent',
  'customElements',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: domWindow.matchMedia.bind(domWindow),
})

const clipboardWrites: string[] = []
Object.defineProperty(domWindow.navigator, 'clipboard', {
  configurable: true,
  value: {
    writeText: async (value: string) => {
      clipboardWrites.push(value)
    },
  },
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ApiKeyApiRoutes } = await import('../api-key-api-routes')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  interpolation: { escapeValue: false },
  resources: {
    en: {
      translation: {
        'API Info': 'API Access URLs',
        'Copy URL': 'Copy URL',
        'Copied!': 'Copied!',
        Copied: 'Copied',
        'Copied to clipboard': 'Copied to clipboard',
        'Failed to copy to clipboard': 'Failed to copy to clipboard',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const routes = [
  {
    route: 'Asia Pacific',
    url: 'https://api-ap.example.com',
    description: 'Nearest route',
    color: 'blue',
  },
  {
    route: 'Global',
    url: 'https://api.example.com',
    description: 'Global route',
    color: 'green',
  },
]

describe('API key access routes', () => {
  after(() => domWindow.close())
  afterEach(() => {
    clipboardWrites.length = 0
    document.body.replaceChildren()
  })

  test('shows configured routes in order and copies the clicked URL', async () => {
    const host = document.createElement('div')
    document.body.append(host)
    const root = createRoot(host)

    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <ApiKeyApiRoutes routes={routes} />
        </I18nextProvider>
      )
    )

    assert.equal(host.textContent?.includes('API Access URLs'), true)
    assert.ok(
      (host.textContent?.indexOf('Asia Pacific') ?? -1) <
        (host.textContent?.indexOf('Global') ?? -1)
    )
    assert.equal(host.textContent?.includes('https://api-ap.example.com'), true)
    assert.equal(host.textContent?.includes('https://api.example.com'), true)

    const buttons = host.querySelectorAll<HTMLButtonElement>('button')
    assert.equal(buttons.length, 2)
    await act(async () => {
      buttons[1].click()
      await Promise.resolve()
      await Promise.resolve()
    })
    assert.deepEqual(clipboardWrites, ['https://api.example.com'])
    assert.equal(buttons[1].getAttribute('aria-label'), 'Copied')

    await act(async () => root.unmount())
  })

  test('renders nothing when no routes are configured', async () => {
    const host = document.createElement('div')
    document.body.append(host)
    const root = createRoot(host)

    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <ApiKeyApiRoutes routes={[]} />
        </I18nextProvider>
      )
    )

    assert.equal(host.textContent, '')
    assert.equal(host.querySelector('button'), null)

    await act(async () => root.unmount())
  })
})
