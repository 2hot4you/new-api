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
import { after, afterEach, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
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
  'KeyboardEvent',
  'PointerEvent',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
  'customElements',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: domWindow.matchMedia.bind(domWindow),
})

const { act, useState } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { AutoGroupOrderEditor } = await import('../auto-group-order-editor')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: {} } },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

const options = [
  { value: 'auto', label: 'auto', desc: 'Hidden internal route' },
  {
    value: 'vip',
    label: 'VIP',
    desc: 'Priority access',
    ratio: 3,
    successRate: 97.5,
  },
  {
    value: 'default',
    label: 'Default',
    desc: 'Standard access',
    ratio: 1,
    successRate: null,
  },
  {
    value: 'team',
    label: 'Team',
    desc: 'Shared access',
    ratio: 2,
    successRate: 0,
  },
]
const globalOptions = options.filter((option) => option.value !== 'auto')

function Harness(props: {
  initialGroups?: string[]
  initialMode?: 'inherit' | 'custom'
  maxCount?: number
  optionSet?: typeof options
}) {
  const [groups, setGroups] = useState(
    props.initialGroups ?? ['vip', 'default']
  )
  const [mode, setMode] = useState<'inherit' | 'custom'>(
    props.initialMode ?? 'custom'
  )

  return (
    <I18nextProvider i18n={i18n}>
      <AutoGroupOrderEditor
        value={groups}
        mode={mode}
        options={props.optionSet ?? options}
        globalOptions={
          props.optionSet?.filter((option) => option.value !== 'auto') ??
          globalOptions
        }
        maxCount={props.maxCount ?? 3}
        onChange={(value) => {
          setGroups(value.groups)
          setMode(value.mode)
        }}
      />
      <output data-testid='order'>{groups.join(',')}</output>
      <output data-testid='mode'>{mode}</output>
    </I18nextProvider>
  )
}

let mounted:
  | {
      container: HTMLDivElement
      root: ReturnType<typeof createRoot>
    }
  | undefined

async function renderHarness(
  props: {
    initialGroups?: string[]
    initialMode?: 'inherit' | 'custom'
    maxCount?: number
    optionSet?: typeof options
  } = {}
) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  mounted = { container, root }
  await act(async () => root.render(<Harness {...props} />))
  return container
}

function findButton(container: ParentNode, label: string): HTMLButtonElement {
  const button = container.querySelector<HTMLButtonElement>(
    `button[aria-label="${label}"]`
  )
  assert.ok(button, `Expected button "${label}"`)
  return button
}

function output(container: ParentNode, testId: string): string | null {
  return (
    container.querySelector(`[data-testid="${testId}"]`)?.textContent ?? null
  )
}

afterEach(async () => {
  if (mounted) {
    await act(async () => mounted?.root.unmount())
    mounted.container.remove()
    mounted = undefined
  }
  document.body.replaceChildren()
})

after(() => {
  domWindow.close()
})

describe('direct API key group selection', () => {
  test('shows every real group as a checkbox option and hides internal auto', async () => {
    const container = await renderHarness()
    const trigger = container.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(trigger)

    await act(async () => trigger.click())

    const checkboxes = [
      ...document.querySelectorAll<HTMLElement>(
        '[data-group-selection-checkbox]'
      ),
    ]
    assert.deepEqual(
      checkboxes.map((checkbox) => checkbox.dataset.groupSelectionCheckbox),
      ['vip', 'default', 'team']
    )
    assert.deepEqual(
      checkboxes.map((checkbox) => ({
        role: checkbox.getAttribute('role'),
        label: checkbox.getAttribute('aria-label'),
        checked: checkbox.getAttribute('aria-checked'),
      })),
      [
        { role: 'checkbox', label: 'Select VIP', checked: 'true' },
        { role: 'checkbox', label: 'Select Default', checked: 'true' },
        { role: 'checkbox', label: 'Select Team', checked: 'false' },
      ]
    )
    assert.equal(
      document.body.textContent?.includes('Hidden internal route'),
      false
    )
    assert.equal(trigger.getAttribute('aria-expanded'), 'true')
    assert.equal(
      trigger.getAttribute('aria-label'),
      'Group selection order: VIP → Default'
    )
    const descriptionId = trigger.getAttribute('aria-describedby')
    assert.ok(descriptionId)
    assert.equal(
      container.querySelector(`#${descriptionId}`)?.textContent,
      '2 groups will be tried in order'
    )
  })

  test('toggles single and multiple groups without closing the picker', async () => {
    const container = await renderHarness({ initialGroups: ['vip'] })
    const trigger = container.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.click())

    const defaultOption = [
      ...document.querySelectorAll<HTMLElement>(
        '[data-group-selection-checkbox]'
      ),
    ].find((option) => option.textContent?.includes('Standard access'))
    assert.ok(defaultOption)
    await act(async () => defaultOption.click())

    assert.equal(output(container, 'order'), 'vip,default')
    assert.equal(output(container, 'mode'), 'custom')
    assert.equal(trigger.getAttribute('aria-expanded'), 'true')
    assert.equal(
      container.textContent?.includes('2 groups will be tried in order'),
      true
    )

    const vipOption = [
      ...document.querySelectorAll<HTMLElement>(
        '[data-group-selection-checkbox]'
      ),
    ].find((option) => option.textContent?.includes('Priority access'))
    assert.ok(vipOption)
    await act(async () => vipOption.click())

    assert.equal(output(container, 'order'), 'default')
    assert.equal(
      container.textContent?.includes('One group uses fixed routing'),
      true
    )
  })

  test('shows today’s real success rate in selected rows and keeps drag ordering', async () => {
    const container = await renderHarness()
    const vipItem = container.querySelector<HTMLElement>(
      '[data-selected-group-item="vip"]'
    )
    assert.ok(vipItem)
    const nameLine = vipItem.querySelector<HTMLElement>(
      '[data-selected-group-name-line]'
    )
    assert.ok(nameLine)
    assert.equal(nameLine.textContent?.includes('VIP'), true)
    const metadata = nameLine.querySelector<HTMLElement>(
      '[data-selected-group-metadata]'
    )
    assert.ok(metadata)
    assert.equal(metadata.textContent?.includes('97.5%'), true)
    assert.equal(metadata.textContent?.includes('Recommendation'), false)
    assert.equal(nameLine.textContent?.includes('3x'), true)
    assert.equal(vipItem.textContent?.includes('Priority access'), true)
    assert.equal(
      vipItem.querySelector('button[aria-label="Move VIP up"]'),
      null
    )
    assert.equal(
      vipItem.querySelector('button[aria-label="Move VIP down"]'),
      null
    )
    assert.ok(vipItem.querySelector('button[aria-label="Remove VIP"]'))
    assert.equal(
      findButton(container, 'Drag VIP to reorder').getAttribute(
        'aria-keyshortcuts'
      ),
      'ArrowUp ArrowDown'
    )

    await act(async () => {
      findButton(container, 'Drag VIP to reorder').dispatchEvent(
        new domWindow.KeyboardEvent('keydown', {
          key: 'ArrowDown',
          bubbles: true,
        }) as unknown as KeyboardEvent
      )
    })
    assert.equal(output(container, 'order'), 'default,vip')
  })

  test('shows real success rates in picker rows and no requests for groups without today’s metrics', async () => {
    const container = await renderHarness()
    const trigger = container.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.click())

    const vip = document.querySelector<HTMLElement>(
      '[data-group-selection-checkbox="vip"]'
    )
    const defaultGroup = document.querySelector<HTMLElement>(
      '[data-group-selection-checkbox="default"]'
    )
    const team = document.querySelector<HTMLElement>(
      '[data-group-selection-checkbox="team"]'
    )
    assert.ok(vip)
    assert.ok(defaultGroup)
    assert.ok(team)
    assert.equal(vip.textContent?.includes('97.5%'), true)
    assert.equal(defaultGroup.textContent?.includes('No requests'), true)
    assert.equal(team.textContent?.includes('0%'), true)
  })

  test('copies the configured system order without exposing auto', async () => {
    const container = await renderHarness({
      initialGroups: ['team'],
      maxCount: 2,
    })
    const restore = [...container.querySelectorAll('button')].find((button) =>
      button.textContent?.includes('Use system default order')
    )
    assert.ok(restore)
    await act(async () => restore.click())

    assert.equal(output(container, 'mode'), 'custom')
    assert.equal(output(container, 'order'), 'vip,default')
    assert.equal(container.textContent?.includes('2 / 2 groups selected'), true)
    assert.equal(container.textContent?.includes('auto'), false)
  })

  test('shows a distinct empty-selection state', async () => {
    const container = await renderHarness({ initialGroups: [] })

    assert.equal(container.textContent?.includes('No groups selected'), true)
    assert.equal(
      container.textContent?.includes('One group uses fixed routing'),
      false
    )
  })

  test('groups options by provider order with shared selection and uncategorized fallback', async () => {
    const providerOptions = [
      {
        value: 'shared',
        label: 'Shared Claude',
        desc: 'Primary Claude group',
        providers: [
          {
            id: 2,
            name: 'Anthropic',
            icon: 'Anthropic.Color',
            display_order: 2,
          },
          {
            id: 1,
            name: 'OpenAI',
            icon: 'OpenAI.Color',
            display_order: 1,
          },
        ],
      },
      {
        value: 'openai',
        label: 'OpenAI Backup',
        desc: 'Secondary route',
        providers: [
          {
            id: 1,
            name: 'OpenAI',
            icon: 'OpenAI.Color',
            display_order: 1,
          },
        ],
      },
      {
        value: 'legacy',
        label: 'Legacy',
        desc: 'No provider configured',
      },
    ]
    const container = await renderHarness({
      initialGroups: [],
      optionSet: providerOptions,
    })
    const trigger = container.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.click())

    const sections = [
      ...document.querySelectorAll<HTMLElement>(
        '[data-group-provider-section]'
      ),
    ]
    assert.deepEqual(
      sections.map((section) => section.dataset.groupProviderSection),
      ['OpenAI', 'Anthropic', 'Uncategorized']
    )
    assert.equal(
      sections[0]
        ?.querySelector('[data-provider-icon-key]')
        ?.getAttribute('data-provider-icon-key'),
      'OpenAI.Color'
    )
    assert.deepEqual(
      [
        ...document.querySelectorAll<HTMLElement>(
          '[data-group-selection-checkbox="shared"]'
        ),
      ].length,
      2
    )

    const firstShared = document.querySelector<HTMLElement>(
      '[data-group-selection-checkbox="shared"]'
    )
    assert.ok(firstShared)
    await act(async () => firstShared.click())
    assert.equal(output(container, 'order'), 'shared')
    assert.deepEqual(
      [
        ...document.querySelectorAll<HTMLElement>(
          '[data-group-selection-checkbox="shared"]'
        ),
      ].map((option) => option.getAttribute('aria-checked')),
      ['true', 'true']
    )
  })

  test('matches provider names when searching categorized groups', async () => {
    const providerOptions = [
      {
        value: 'claude-primary',
        label: 'Primary',
        desc: 'Official route',
        providers: [
          {
            id: 1,
            name: 'Anthropic',
            icon: 'Anthropic.Color',
            display_order: 1,
          },
        ],
      },
      { value: 'legacy', label: 'Legacy', desc: 'No provider configured' },
    ]
    const container = await renderHarness({
      initialGroups: [],
      optionSet: providerOptions,
    })
    const trigger = container.querySelector<HTMLButtonElement>(
      'button[role="combobox"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.click())
    const search = document.querySelector<HTMLInputElement>(
      'input[aria-label="Search groups..."]'
    )
    assert.ok(search)
    await act(async () => {
      const valueSetter = Object.getOwnPropertyDescriptor(
        domWindow.HTMLInputElement.prototype,
        'value'
      )?.set
      assert.ok(valueSetter)
      valueSetter.call(search, 'Anthropic')
      search.dispatchEvent(
        new domWindow.Event('input', { bubbles: true }) as unknown as Event
      )
    })

    assert.ok(
      document.querySelector('[data-group-selection-checkbox="claude-primary"]')
    )
    assert.equal(
      document.querySelector('[data-group-selection-checkbox="legacy"]'),
      null
    )
  })
})
