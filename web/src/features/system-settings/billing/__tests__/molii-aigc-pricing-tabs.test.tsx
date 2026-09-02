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
import { afterAll as after, describe, test } from 'vitest'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
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

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { SettingsPageActionsPortal, SettingsPageProvider } =
  await import('../../components/settings-page-context')
const { MoliiAigcPricingTabs } = await import('../molii-aigc-pricing-tabs')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'en', resources: {} })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function MarkerForm(props: { name: string }) {
  return (
    <>
      <div>{props.name}-form</div>
      <SettingsPageActionsPortal>
        <button type='button'>{props.name}-save</button>
      </SettingsPageActionsPortal>
    </>
  )
}

describe('Molii Volcengine Imagine API pricing tabs', () => {
  after(() => {
    domWindow.close()
  })

  test('mounts only the active pricing form and its save action', async () => {
    const container = document.createElement('div')
    const titleContainer = document.createElement('span')
    const actionsContainer = document.createElement('div')
    document.body.append(container, titleContainer, actionsContainer)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <SettingsPageProvider
            actionsContainer={actionsContainer}
            titleStatusContainer={titleContainer}
          >
            <MoliiAigcPricingTabs
              modelPricing={<MarkerForm name='models' />}
              seedance={<MarkerForm name='seedance' />}
              grokImagine={<MarkerForm name='grok' />}
            />
          </SettingsPageProvider>
        </I18nextProvider>
      )
    })

    assert.equal(
      titleContainer.textContent,
      'General modelsSeedance 2.0Grok Imagine'
    )
    assert.match(container.textContent ?? '', /models-form/)
    assert.doesNotMatch(container.textContent ?? '', /seedance-form/)
    assert.doesNotMatch(container.textContent ?? '', /grok-form/)
    assert.equal(actionsContainer.textContent, 'models-save')

    const grokTab = [...titleContainer.querySelectorAll('button')].find(
      (button) => button.textContent === 'Grok Imagine'
    )
    assert.ok(grokTab)

    await act(async () => {
      grokTab.click()
    })

    assert.doesNotMatch(container.textContent ?? '', /seedance-form/)
    assert.match(container.textContent ?? '', /grok-form/)
    assert.equal(actionsContainer.textContent, 'grok-save')

    await act(async () => root.unmount())
    container.remove()
    titleContainer.remove()
    actionsContainer.remove()
  })
})
