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
// @ts-expect-error Bun provides this test-runner module at runtime.
import { afterAll, describe, test } from 'bun:test'
import assert from 'node:assert/strict'

import { Window } from 'happy-dom'
import { createInstance } from 'i18next'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { I18nextProvider, initReactI18next } from 'react-i18next'

import { PlaygroundInputControls } from './playground-input-controls'

const domWindow = new Window()
const globalKeys = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
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

const i18n = createInstance()
const i18nReady = i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

async function renderControls(isGenerating: boolean) {
  await i18nReady
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <PlaygroundInputControls
          groups={[{ label: 'Default', value: 'default', ratio: 1 }]}
          groupValue='default'
          isGenerating={isGenerating}
          models={[{ label: 'Test model', value: 'test-model' }]}
          modelValue='test-model'
          onGroupChange={() => {}}
          onModelChange={() => {}}
          onStop={() => {}}
          text='Hello'
          tools={<button type='button'>Parameters</button>}
        />
      </I18nextProvider>
    )
  })

  return { container, root }
}

describe('PlaygroundInputControls', () => {
  afterAll(() => {
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

  test('locks model selection and exposes only stop while generating', async () => {
    const { container, root } = await renderControls(true)

    assert.equal(
      container.querySelector<HTMLButtonElement>('[role="combobox"]')?.disabled,
      true
    )
    assert.match(container.textContent ?? '', /Stop/)
    assert.doesNotMatch(container.textContent ?? '', /Send/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('exposes send and no stop when idle', async () => {
    const { container, root } = await renderControls(false)

    assert.match(container.textContent ?? '', /Send/)
    assert.doesNotMatch(container.textContent ?? '', /Stop/)

    await act(async () => root.unmount())
    container.remove()
  })
})
