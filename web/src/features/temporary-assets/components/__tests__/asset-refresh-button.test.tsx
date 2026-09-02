/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
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
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
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
const { AssetRefreshButton } = await import('../asset-refresh-button')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('temporary asset bulk refresh button', () => {
  after(() => domWindow.close())

  test('shows the filtered asset count when no assets are selected', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let clickCount = 0

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <AssetRefreshButton
            targetCount={3}
            selectedCount={0}
            refreshing={false}
            onClick={() => {
              clickCount += 1
            }}
          />
        </I18nextProvider>
      )
    })

    const button = container.querySelector('button')
    assert.ok(button)
    assert.equal(button.disabled, false)
    assert.match(button.textContent ?? '', /Update current asset status/)
    assert.match(button.textContent ?? '', /3/)
    await act(async () => button.click())
    assert.equal(clickCount, 1)

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows the selected scope and disables the button while refreshing', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <AssetRefreshButton
            targetCount={2}
            selectedCount={2}
            refreshing
            onClick={() => {}}
          />
        </I18nextProvider>
      )
    })

    const button = container.querySelector('button')
    assert.ok(button)
    assert.equal(button.disabled, true)
    assert.match(button.textContent ?? '', /Update selected asset status/)
    assert.match(button.textContent ?? '', /2/)
    assert.ok(container.querySelector('.animate-spin'))

    await act(async () => root.unmount())
    container.remove()
  })
})
