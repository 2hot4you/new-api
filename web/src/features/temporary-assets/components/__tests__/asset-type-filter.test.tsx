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

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { AssetTypeFilter } = await import('../asset-type-filter')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en' })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function FilterHarness() {
  const [value, setValue] = useState<'all' | 'image' | 'video' | 'audio'>('all')
  return <AssetTypeFilter value={value} onValueChange={setValue} />
}

describe('temporary asset type filter', () => {
  after(() => domWindow.close())

  test('defaults to all assets and switches to the clicked media type', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <FilterHarness />
        </I18nextProvider>
      )
    })

    const buttons = [...container.querySelectorAll('button')]
    assert.equal(buttons.length, 4)
    assert.equal(buttons[0]?.textContent, 'All')
    assert.equal(buttons[0]?.getAttribute('aria-pressed'), 'true')

    const videoButton = buttons.find((button) =>
      button.textContent?.includes('Video')
    )
    assert.ok(videoButton)
    await act(async () => videoButton.click())

    assert.equal(buttons[0]?.getAttribute('aria-pressed'), 'false')
    assert.equal(videoButton.getAttribute('aria-pressed'), 'true')

    await act(async () => root.unmount())
    container.remove()
  })
})
