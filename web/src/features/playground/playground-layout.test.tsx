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
const globalKeys = [
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
] as const
const originalGlobalDescriptors = new Map(
  globalKeys.map((key) => [
    key,
    Object.getOwnPropertyDescriptor(globalThis, key),
  ])
)
const originalReactActEnvironmentDescriptor = Object.getOwnPropertyDescriptor(
  globalThis,
  'IS_REACT_ACT_ENVIRONMENT'
)

for (const key of globalKeys) {
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
const en = (await import('../../i18n/locales/en.json')).default
const zh = (await import('../../i18n/locales/zh.json')).default
const { Playground } = await import('./index')

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function createI18n(language: 'en' | 'zhCN', resources: typeof en) {
  const i18n = createInstance()
  return i18n
    .use(initReactI18next)
    .init({ lng: language, resources: { [language]: resources } })
    .then(() => i18n)
}

async function renderPlayground(i18n: Awaited<ReturnType<typeof createI18n>>) {
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

  return { container, queryClient, root }
}

async function finishMessageLoad() {
  await act(
    async () =>
      await new Promise<void>((resolve) => {
        domWindow.setTimeout(resolve, 0)
      })
  )
}

async function waitForMessagePersistence() {
  await act(
    async () =>
      await new Promise<void>((resolve) => {
        domWindow.setTimeout(resolve, 550)
      })
  )
}

function getNewConversationButton(container: HTMLElement) {
  const button = container.querySelector<HTMLButtonElement>(
    '[aria-label="New conversation"]'
  )
  assert.ok(button)
  return button
}

describe('Playground Base layout', () => {
  after(() => {
    domWindow.close()
    for (const key of globalKeys) {
      const descriptor = originalGlobalDescriptors.get(key)
      if (descriptor) {
        Object.defineProperty(globalThis, key, descriptor)
      } else {
        Reflect.deleteProperty(globalThis, key)
      }
    }
    if (originalReactActEnvironmentDescriptor) {
      Object.defineProperty(
        globalThis,
        'IS_REACT_ACT_ENVIRONMENT',
        originalReactActEnvironmentDescriptor
      )
    } else {
      Reflect.deleteProperty(globalThis, 'IS_REACT_ACT_ENVIRONMENT')
    }
  })

  test('waits for stored history before allowing confirmed new-conversation clearing', async () => {
    const storedMessages = [
      {
        key: 'stored-user-message',
        from: 'user',
        versions: [{ id: 'stored-v1', content: 'Stored request' }],
      },
    ]
    localStorage.setItem('playground_messages', JSON.stringify(storedMessages))
    const i18n = await createI18n('en', en)
    const { container, queryClient, root } = await renderPlayground(i18n)
    const newConversation = getNewConversationButton(container)

    assert.equal(newConversation.disabled, true)
    await act(async () => newConversation.click())

    await finishMessageLoad()

    assert.equal(newConversation.disabled, false)
    assert.match(container.textContent ?? '', /Stored request/)
    assert.deepEqual(
      JSON.parse(localStorage.getItem('playground_messages') ?? '').data,
      storedMessages
    )

    await act(async () => newConversation.click())

    assert.match(document.body.textContent ?? '', /Clear chat history\?/)
    assert.match(container.textContent ?? '', /Stored request/)

    const clearButton = [...document.querySelectorAll('button')].find(
      (button) => button.textContent === 'Clear'
    )
    assert.ok(clearButton)

    await act(async () => clearButton.click())

    assert.doesNotMatch(container.textContent ?? '', /Stored request/)
    assert.match(container.textContent ?? '', /Start a playground chat/)

    await waitForMessagePersistence()

    assert.deepEqual(
      JSON.parse(localStorage.getItem('playground_messages') ?? '').data,
      []
    )

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })

  test('keeps a responsive log and composer while omitting unavailable tools and Share', async () => {
    localStorage.clear()
    const i18n = await createI18n('en', en)
    const { container, queryClient, root } = await renderPlayground(i18n)
    container.style.width = '320px'

    const newConversationActions = [
      ...container.querySelectorAll('button'),
    ].filter((button) => button.textContent?.includes('New conversation'))
    assert.equal(newConversationActions.length, 1)
    assert.doesNotMatch(container.textContent ?? '', /Share/)
    assert.equal(container.querySelector('[aria-label="Attach"]'), null)
    assert.equal(container.querySelector('[aria-label="Search"]'), null)

    const thread = container.querySelector<HTMLElement>(
      '[data-slot="playground-thread"]'
    )
    const composer = container.querySelector<HTMLElement>(
      '[data-slot="playground-composer"]'
    )
    assert.ok(thread)
    assert.ok(composer)
    assert.equal(thread.querySelector('[role="log"]') !== null, true)
    assert.match(thread.className, /\bw-full\b/)
    assert.match(thread.className, /\bmin-w-0\b/)
    assert.doesNotMatch(thread.className, /(?:^|\s)w-\[/)
    assert.match(composer.className, /\bw-full\b/)
    assert.match(composer.className, /\bmin-w-0\b/)
    assert.doesNotMatch(composer.className, /(?:^|\s)w-\[/)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })

  test('renders the new-conversation action in Chinese', async () => {
    localStorage.clear()
    const i18n = await createI18n('zhCN', zh)
    const { container, queryClient, root } = await renderPlayground(i18n)

    const newConversation = container.querySelector<HTMLButtonElement>(
      '[aria-label="新建对话"]'
    )
    assert.ok(newConversation)
    assert.equal(newConversation.textContent?.includes('新建对话'), true)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })
})
