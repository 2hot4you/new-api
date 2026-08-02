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
const { CreateAssetCard } = await import('../create-asset-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('temporary asset source options', () => {
  after(() => domWindow.close())

  test('keeps URL creation available when COS local upload is disabled', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <CreateAssetCard
            uploadConfig={{
              enabled: false,
              limits: { image: 30, video: 200, audio: 15 },
            }}
            onCreated={async () => {}}
          />
        </I18nextProvider>
      )
    })

    const localFileInput =
      container.querySelector<HTMLInputElement>('input[type="file"]')
    assert.ok(localFileInput)
    assert.equal(localFileInput.disabled, true)
    assert.match(container.textContent ?? '', /Configure Tencent COS/)

    const urlTab = [...container.querySelectorAll('button')].find(
      (button) => button.textContent === 'Add by URL'
    )
    assert.ok(urlTab)
    await act(async () => urlTab.click())

    assert.ok(container.querySelector('input[type="url"]'))
    assert.match(container.textContent ?? '', /Public media URL/)

    await act(async () => root.unmount())
    container.remove()
  })
})
