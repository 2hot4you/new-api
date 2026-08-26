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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
for (const key of [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'HTMLInputElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'PointerEvent',
  'KeyboardEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'matchMedia',
  'customElements',
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
const { GroupRatioVisualEditor } = await import('../group-ratio-visual-editor')

const i18n = createInstance()
await i18n
  .use(initReactI18next)
  .init({ lng: 'en', resources: { en: { translation: {} } } })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const initialMetadata = JSON.stringify([
  { name: 'default', icon: 'OpenAI.Color', recommendation: 0 },
  { name: 'vip', icon: 'DeepSeek.Color', recommendation: 4 },
])

async function changeInput(input: HTMLInputElement, value: string) {
  await act(async () => {
    const inputWindow = input.ownerDocument.defaultView
    assert.ok(inputWindow)
    const valueSetter = Object.getOwnPropertyDescriptor(
      inputWindow.HTMLInputElement.prototype,
      'value'
    )?.set
    assert.ok(valueSetter)
    valueSetter.call(input, value)
    input.dispatchEvent(
      new inputWindow.Event('input', { bubbles: true }) as unknown as Event
    )
  })
}

async function mountEditor(calls: Array<[string, string]>) {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <GroupRatioVisualEditor
          groupRatio='{"default":1,"vip":2}'
          topupGroupRatio='{}'
          userUsableGroups='{"default":"Default","vip":"Priority"}'
          groupGroupRatio='{}'
          autoGroups='[]'
          groupMetadata={initialMetadata}
          maxTokenAutoGroupsField={null}
          groupSpecialUsableGroup='{}'
          onChange={(field, value) => calls.push([field, value])}
        />
      </I18nextProvider>
    )
  )
  return { host, root }
}

const reorderedMetadata = JSON.stringify([
  { name: 'vip', icon: 'DeepSeek.Color', recommendation: 4 },
  { name: 'default', icon: 'OpenAI.Color', recommendation: 0 },
])

function FocusPersistenceHarness() {
  const [groupMetadata, setGroupMetadata] = useState(reorderedMetadata)

  return (
    <I18nextProvider i18n={i18n}>
      <GroupRatioVisualEditor
        groupRatio='{"default":1,"vip":2}'
        topupGroupRatio='{}'
        userUsableGroups='{"default":"Default","vip":"Priority"}'
        groupGroupRatio='{}'
        autoGroups='[]'
        groupMetadata={groupMetadata}
        maxTokenAutoGroupsField={null}
        groupSpecialUsableGroup='{}'
        onChange={(field, value) => {
          if (field === 'GroupMetadata') setGroupMetadata(value)
        }}
      />
    </I18nextProvider>
  )
}

async function mountFocusPersistenceHarness() {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  await act(async () => root.render(<FocusPersistenceHarness />))
  return { host, root }
}

describe('group pricing metadata editing', () => {
  let host: HTMLDivElement | null = null
  let root: ReturnType<typeof createRoot> | null = null

  afterEach(async () => {
    if (root) await act(async () => root?.unmount())
    host?.remove()
    host = null
    root = null
    document.body.replaceChildren()
  })

  after(() => domWindow.close())

  test('emits only GroupMetadata when changing an icon', async () => {
    const calls: Array<[string, string]> = []
    ;({ host, root } = await mountEditor(calls))

    const iconInput = [
      ...document.querySelectorAll<HTMLInputElement>('input'),
    ].find((input) => input.value === 'DeepSeek.Color')
    assert.ok(iconInput)
    await act(async () => changeInput(iconInput, 'Claude.Color'))

    assert.deepEqual(
      calls.map(([field]) => field),
      ['GroupMetadata']
    )
    const metadata = JSON.parse(calls[0]?.[1] ?? '[]')
    assert.equal(metadata[1]?.icon, 'Claude.Color')
  })

  test('serializes accessible moves as metadata-only order changes', async () => {
    const calls: Array<[string, string]> = []
    ;({ host, root } = await mountEditor(calls))

    const moveDown = document.querySelector<HTMLButtonElement>(
      'button[aria-label="Move group down"]'
    )
    assert.ok(moveDown)
    await act(async () => moveDown.click())

    assert.deepEqual(
      calls.map(([field]) => field),
      ['GroupMetadata']
    )
    assert.deepEqual(JSON.parse(calls[0]?.[1] ?? '[]'), [
      { name: 'vip', icon: 'DeepSeek.Color' },
      { name: 'default', icon: 'OpenAI.Color' },
    ])
  })

  test('does not render a recommendation control for legacy metadata', async () => {
    const calls: Array<[string, string]> = []
    ;({ host, root } = await mountEditor(calls))

    assert.equal(
      document.querySelector('input[aria-label^="Recommendation:"]'),
      null
    )
    assert.equal(
      [...document.querySelectorAll('span')].some(
        (element) => element.textContent === 'Recommendation'
      ),
      false
    )
  })

  test('uses a pointer/touch sortable handle and preserves exact icon keys', async () => {
    const calls: Array<[string, string]> = []
    ;({ host, root } = await mountEditor(calls))

    assert.ok(document.querySelector('[data-group-pricing-sortable-list]'))
    const sortableRow = document.querySelector('[data-group-pricing-sortable]')
    assert.ok(sortableRow)
    const handle = document.querySelector<HTMLButtonElement>(
      'button[aria-label="Drag to reorder group"]'
    )
    assert.ok(handle)
    assert.equal(handle.getAttribute('draggable'), null)
    assert.equal(handle.classList.contains('touch-none'), true)

    const preview = document.querySelector<HTMLElement>(
      '[data-group-pricing-icon-preview]'
    )
    assert.ok(preview)
    assert.equal(preview.getAttribute('data-icon-key'), 'OpenAI.Color')

    const iconInput = [
      ...document.querySelectorAll<HTMLInputElement>('input'),
    ].find((input) => input.value === 'OpenAI.Color')
    assert.ok(iconInput)
    await act(async () => changeInput(iconInput, 'MissingLobeIcon'))
    const fallbackPreview = document.querySelector<HTMLElement>(
      '[data-group-pricing-icon-preview]'
    )
    assert.ok(fallbackPreview)
    assert.equal(
      fallbackPreview.getAttribute('data-icon-key'),
      'MissingLobeIcon'
    )
    assert.equal(fallbackPreview.textContent, 'M')
  })

  test('gives every repeated group-row input an accessible name', async () => {
    const calls: Array<[string, string]> = []
    ;({ host, root } = await mountEditor(calls))

    for (const label of [
      'Group name: default',
      'Icon: default',
      'Ratio: default',
      'Top-up ratio: default',
      'Description: default',
    ]) {
      assert.ok(
        document.querySelector(`input[aria-label="${label}"]`),
        `Expected accessible input label: ${label}`
      )
    }
  })

  test('keeps the icon input mounted and focused while metadata syncs', async () => {
    ;({ host, root } = await mountFocusPersistenceHarness())

    const iconInput = document.querySelector<HTMLInputElement>(
      'input[aria-label="Icon: vip"]'
    )
    assert.ok(iconInput)
    iconInput.focus()

    await act(async () => changeInput(iconInput, 'DeepSeek.ColorX'))

    assert.equal(iconInput.isConnected, true)
    assert.equal(document.activeElement, iconInput)
    assert.equal(iconInput.value, 'DeepSeek.ColorX')
  })

  test('serializes structural add, rename, and delete changes across legacy maps', async () => {
    const calls: Array<[string, string]> = []
    ;({ host, root } = await mountEditor(calls))

    const addGroup = [
      ...document.querySelectorAll<HTMLButtonElement>('button'),
    ].find((button) => button.textContent?.includes('Add group'))
    assert.ok(addGroup)
    await act(async () => addGroup.click())
    assert.deepEqual(
      calls.map(([field]) => field),
      ['GroupRatio', 'UserUsableGroups', 'TopupGroupRatio', 'GroupMetadata']
    )
    const addedMetadata = JSON.parse(calls[3]?.[1] ?? '[]')
    assert.equal(addedMetadata.at(-1)?.name, 'group_1')
    assert.equal(
      addedMetadata.some((entry: object) =>
        Object.hasOwn(entry, 'recommendation')
      ),
      false
    )

    calls.length = 0
    const newGroupName = [
      ...document.querySelectorAll<HTMLInputElement>('input'),
    ].find((input) => input.value === 'group_1')
    assert.ok(newGroupName)
    await act(async () => changeInput(newGroupName, 'enterprise'))
    assert.equal(JSON.parse(calls[0]?.[1] ?? '{}').enterprise, 1)
    assert.equal(JSON.parse(calls[3]?.[1] ?? '[]').at(-1)?.name, 'enterprise')

    calls.length = 0
    const deleteButtons = [
      ...document.querySelectorAll<HTMLButtonElement>(
        'button[aria-label="Delete"]'
      ),
    ]
    const deleteNewGroup = deleteButtons.at(-1)
    assert.ok(deleteNewGroup)
    await act(async () => deleteNewGroup.click())
    assert.equal(JSON.parse(calls[0]?.[1] ?? '{}').enterprise, undefined)
    assert.equal(
      JSON.parse(calls[3]?.[1] ?? '[]').some(
        (entry: { name: string }) => entry.name === 'enterprise'
      ),
      false
    )
  })
})
