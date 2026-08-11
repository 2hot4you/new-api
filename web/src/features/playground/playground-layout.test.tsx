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
*/
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLFormElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'localStorage',
  'ResizeObserver',
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { Playground } = await import('./index')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('Playground Base layout', () => {
  after(() => domWindow.close())

  test('provides one new-conversation action without Share and keeps thread and composer separate', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <Playground />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    const newConversationActions = [
      ...container.querySelectorAll('button'),
    ].filter((button) => button.textContent?.includes('New conversation'))
    assert.equal(newConversationActions.length, 1)
    assert.doesNotMatch(container.textContent ?? '', /Share/)

    const thread = container.querySelector<HTMLElement>(
      '[data-slot="playground-thread"]'
    )
    const composer = container.querySelector<HTMLElement>(
      '[data-slot="playground-composer"]'
    )
    assert.ok(thread)
    assert.ok(composer)
    assert.equal(thread.querySelector('[role="log"]') !== null, true)
    assert.equal(thread.classList.contains('min-w-0'), true)
    assert.equal(composer.classList.contains('min-w-0'), true)
    assert.equal(thread.classList.contains('max-w-[44rem]'), true)
    assert.equal(composer.classList.contains('max-w-[44rem]'), true)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })
})
