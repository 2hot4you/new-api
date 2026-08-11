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

import { PlaygroundInputTools } from './playground-input-tools'
import { PlaygroundParameterContent } from './playground-parameter-panel'

const domWindow = new Window()
const globalKeys = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
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

const config = {
  model: 'test-model',
  group: 'default',
  temperature: 0.7,
  top_p: 1,
  max_tokens: 4096,
  frequency_penalty: 0,
  presence_penalty: 0,
  seed: null,
  stream: true,
}

const allDisabled = {
  temperature: false,
  top_p: false,
  max_tokens: false,
  frequency_penalty: false,
  presence_penalty: false,
  seed: false,
}

const parameterLabels = {
  temperature: 'Temperature',
  top_p: 'Top P',
  max_tokens: 'Max Tokens',
  frequency_penalty: 'Frequency Penalty',
  presence_penalty: 'Presence Penalty',
  seed: 'Seed',
}

async function renderParameterPanel(
  overrides: {
    disabled?: boolean
    onConfigChange?: (key: string, value: unknown) => void
    onParameterEnabledChange?: (key: string, value: boolean) => void
    parameterEnabled?: typeof allDisabled
  } = {}
) {
  await i18nReady
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <I18nextProvider i18n={i18n}>
        <PlaygroundParameterContent
          config={config}
          disabled={overrides.disabled}
          onConfigChange={overrides.onConfigChange ?? (() => {})}
          onParameterEnabledChange={
            overrides.onParameterEnabledChange ?? (() => {})
          }
          parameterEnabled={overrides.parameterEnabled ?? allDisabled}
        />
      </I18nextProvider>
    )
  })

  return { container, root }
}

describe('PlaygroundParameterPanel', () => {
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

  test('starts with each advanced parameter switched off and values disabled', async () => {
    const { container, root } = await renderParameterPanel()

    for (const parameter of Object.keys(allDisabled) as Array<
      keyof typeof allDisabled
    >) {
      const toggle = container.querySelector<HTMLElement>(
        `[aria-label="Enable ${parameterLabels[parameter]}"]`
      )
      assert.ok(toggle)
      assert.equal(toggle.getAttribute('aria-checked'), 'false')
    }

    assert.equal(
      container
        .querySelector<HTMLElement>('[data-slot="slider"]')
        ?.getAttribute('data-disabled'),
      ''
    )
    assert.equal(
      container.querySelector<HTMLInputElement>('#playground-max_tokens')
        ?.disabled,
      true
    )
    assert.equal(
      container.querySelector<HTMLInputElement>('#playground-seed')?.disabled,
      true
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('enables only the selected advanced parameter', async () => {
    const changes: Array<[string, boolean]> = []
    const { container, root } = await renderParameterPanel({
      onParameterEnabledChange: (key, value) => changes.push([key, value]),
    })
    const temperature = container.querySelector<HTMLElement>(
      '[aria-label="Enable Temperature"]'
    )
    assert.ok(temperature)

    await act(async () => temperature.click())

    assert.deepEqual(changes, [['temperature', true]])

    await act(async () => root.unmount())
    container.remove()
  })

  test('changes stream through the ordinary setting switch', async () => {
    const changes: Array<[string, unknown]> = []
    const { container, root } = await renderParameterPanel({
      onConfigChange: (key, value) => changes.push([key, value]),
    })
    const stream = container.querySelector<HTMLElement>('[aria-label="Stream"]')
    assert.ok(stream)

    await act(async () => stream.click())

    assert.deepEqual(changes, [['stream', false]])

    await act(async () => root.unmount())
    container.remove()
  })

  test('disables parameter changes while generation is active', async () => {
    await i18nReady
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <PlaygroundInputTools
            config={config}
            isGenerating
            onConfigChange={() => {}}
            onParameterEnabledChange={() => {}}
            parameterEnabled={allDisabled}
          />
        </I18nextProvider>
      )
    })

    assert.equal(
      container.querySelector<HTMLButtonElement>('[aria-label="Parameters"]')
        ?.disabled,
      true
    )

    await act(async () => root.unmount())
    container.remove()
  })
})
