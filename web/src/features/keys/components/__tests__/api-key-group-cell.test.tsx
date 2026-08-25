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
import { after, describe, test } from 'node:test'

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
  'customElements',
  'MutationObserver',
  'ResizeObserver',
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
Object.defineProperty(globalThis, 'matchMedia', {
  configurable: true,
  value: domWindow.matchMedia.bind(domWindow),
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { TooltipProvider } = await import('@/components/ui/tooltip')
const { ApiKeyGroupCell } = await import('../api-key-group-cell')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        Auto: 'Auto',
        'Cross-group': 'Cross-group',
        'View cross-group details': 'View cross-group details',
        'Group failover order': 'Group failover order',
        '{{count}} groups': '{{count}} groups',
        'Uses system default order': 'Uses system default order',
        Unavailable: 'Unavailable',
        'No groups configured.': 'No groups configured.',
        Recommendation: 'Recommendation',
        Ratio: 'Ratio',
        'Automatically selects the best available group with circuit breaker mechanism':
          'Automatically selects the best available group with circuit breaker mechanism',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

function CellHarness(props: {
  group: string
  ratio?: number | string
  icon?: string
  crossGroupRetry?: boolean
  shouldReduceMotion?: boolean
  autoGroups?: string[] | null
  defaultAutoGroups?: string[]
  groupDisplayInfo?: Record<
    string,
    {
      ratio?: number | string
      icon?: string
      recommendation?: number
      desc?: string
    }
  >
  groupDataStatus?: 'loading' | 'error' | 'ready'
  defaultAutoGroupsStatus?: 'loading' | 'error' | 'ready'
}) {
  return (
    <I18nextProvider i18n={i18n}>
      <TooltipProvider>
        <ApiKeyGroupCell
          group={props.group}
          ratio={props.ratio}
          icon={props.icon}
          crossGroupRetry={props.crossGroupRetry ?? false}
          shouldReduceMotion={props.shouldReduceMotion ?? false}
          autoGroups={props.autoGroups}
          defaultAutoGroups={props.defaultAutoGroups}
          groupDisplayInfo={props.groupDisplayInfo}
          groupDataStatus={props.groupDataStatus}
          defaultAutoGroupsStatus={props.defaultAutoGroupsStatus}
        />
      </TooltipProvider>
    </I18nextProvider>
  )
}

describe('API key group table cell', () => {
  after(() => {
    domWindow.close()
  })

  test('renders two unclipped rings and a localized Auto ratio when API data uses a nonlocalized string', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness
          group='auto'
          ratio='自动'
          crossGroupRetry
          shouldReduceMotion={false}
        />
      )
    )

    const badgeCell = container.querySelector<HTMLElement>(
      '[data-api-key-group-cell="auto"]'
    )
    assert.ok(badgeCell)
    assert.equal(badgeCell.classList.contains('overflow-visible'), true)
    assert.equal(badgeCell.classList.contains('overflow-hidden'), false)

    const frames = container.querySelectorAll('[data-auto-group-frame]')
    const movingRings = container.querySelectorAll(
      '[data-auto-group-flow-border]'
    )
    assert.equal(frames.length, 2)
    assert.equal(movingRings.length, 2)
    for (const frame of frames) {
      assert.equal(frame.classList.contains('relative'), true)
      assert.equal(frame.classList.contains('overflow-visible'), true)
      assert.equal(frame.classList.contains('rounded-4xl'), true)
      assert.equal(frame.classList.contains('p-px'), true)
    }

    const ratio = container.querySelector<HTMLElement>(
      '[data-auto-group-effect="ratio"]'
    )
    assert.ok(ratio)
    assert.equal(ratio.textContent, 'Auto Ratio')
    assert.equal(ratio.textContent?.includes('x'), false)
    assert.equal(container.textContent?.includes('自动'), false)
    assert.equal(container.textContent?.includes('Cross-group'), true)

    const crossGroupBadge = [
      ...container.querySelectorAll<HTMLElement>('[data-slot="status-badge"]'),
    ].find((badge) => badge.textContent === 'Cross-group')
    assert.ok(crossGroupBadge)
    assert.equal(crossGroupBadge.closest('[data-auto-group-frame]'), null)

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps static Auto frames but omits both moving layers for reduced motion', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(<CellHarness group='auto' ratio='Auto' shouldReduceMotion />)
    )

    assert.equal(
      container.querySelectorAll('[data-auto-group-frame]').length,
      2
    )
    assert.equal(
      container.querySelectorAll('[data-auto-group-flow-border]').length,
      0
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows only the Auto badge when ratio data is unavailable', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(<CellHarness group='auto' shouldReduceMotion={false} />)
    )

    assert.equal(
      container.querySelectorAll('[data-auto-group-frame]').length,
      1
    )
    assert.equal(
      container.querySelectorAll('[data-auto-group-flow-border]').length,
      1
    )
    assert.equal(
      container.querySelector('[data-auto-group-effect="ratio"]'),
      null
    )
    assert.equal(container.textContent?.includes('Auto'), true)
    assert.equal(container.textContent?.includes('Ratio'), false)

    await act(async () => root.unmount())
    container.remove()
  })

  test('narrows normal group ratios to numbers and never applies Auto rings', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness group='vip' ratio='自动' shouldReduceMotion={false} />
      )
    )

    assert.equal(container.textContent?.includes('vip'), true)
    assert.equal(container.textContent?.includes('自动'), false)
    assert.equal(container.querySelector('[data-auto-group-frame]'), null)
    assert.equal(container.querySelector('[data-auto-group-flow-border]'), null)

    await act(async () =>
      root.render(
        <CellHarness group='vip' ratio={3} shouldReduceMotion={false} />
      )
    )

    assert.equal(container.textContent?.includes('3x'), true)
    assert.equal(container.querySelector('[data-auto-group-frame]'), null)

    await act(async () => root.unmount())
    container.remove()
  })

  test('renders a configured normal-group icon at the table size', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness group='vip' icon='OpenAI.Color' shouldReduceMotion />
      )
    )

    const icon = container.querySelector<HTMLElement>(
      '[data-api-key-group-icon="table"]'
    )
    assert.ok(icon)
    assert.equal(icon.getAttribute('data-icon-key'), 'OpenAI.Color')
    assert.equal(icon.querySelector('svg')?.getAttribute('width'), '16')

    await act(async () => root.unmount())
    container.remove()
  })

  test('opens the saved cross-group failover order with group metadata and keeps removed groups visible', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness
          group='auto'
          autoGroups={['vip', 'removed', 'default']}
          defaultAutoGroups={['default', 'vip']}
          groupDisplayInfo={{
            vip: {
              ratio: 2,
              recommendation: 4.5,
              desc: 'Priority route',
              icon: 'OpenAI.Color',
            },
            default: { ratio: 1, recommendation: 5 },
          }}
        />
      )
    )

    const trigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label="View cross-group details"]'
    )
    assert.ok(trigger)
    assert.equal(trigger.getAttribute('aria-expanded'), 'false')

    await act(async () => trigger.click())

    assert.equal(trigger.getAttribute('aria-expanded'), 'true')
    const details = document.body.querySelector<HTMLElement>(
      '[data-api-key-auto-group-details]'
    )
    assert.ok(details)
    assert.equal(details.textContent?.includes('Group failover order'), true)
    assert.equal(details.textContent?.includes('3 groups'), true)
    assert.equal(details.textContent?.includes('Priority route'), true)
    assert.equal(details.textContent?.includes('4.5'), true)
    assert.equal(details.textContent?.includes('2x'), true)
    assert.equal(details.classList.contains('max-h-(--available-height)'), true)
    const detailList = details.querySelector<HTMLElement>(
      '[data-auto-group-detail-list]'
    )
    assert.ok(detailList)
    assert.equal(detailList.classList.contains('min-h-0'), true)
    assert.equal(detailList.classList.contains('overflow-y-auto'), true)

    const items = [
      ...details.querySelectorAll<HTMLElement>('[data-auto-group-detail]'),
    ]
    assert.deepEqual(
      items.map((item) => item.dataset.autoGroupDetail),
      ['vip', 'removed', 'default']
    )
    assert.equal(items[1]?.textContent?.includes('Unavailable'), true)

    await act(async () => root.unmount())
    container.remove()
  })

  test('labels legacy cross-group keys and shows the current system default order', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness
          group='auto'
          autoGroups={null}
          defaultAutoGroups={['default', 'vip']}
          groupDisplayInfo={{
            default: { ratio: 1 },
            vip: { ratio: 2 },
          }}
        />
      )
    )

    const trigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label="View cross-group details"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.click())

    const details = document.body.querySelector<HTMLElement>(
      '[data-api-key-auto-group-details]'
    )
    assert.ok(details)
    assert.equal(
      details.textContent?.includes('Uses system default order'),
      true
    )
    assert.deepEqual(
      [
        ...details.querySelectorAll<HTMLElement>('[data-auto-group-detail]'),
      ].map((item) => item.dataset.autoGroupDetail),
      ['default', 'vip']
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows an explicit empty state when no fallback groups are available', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness group='auto' autoGroups={null} defaultAutoGroups={[]} />
      )
    )

    const trigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label="View cross-group details"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.click())

    const details = document.body.querySelector<HTMLElement>(
      '[data-api-key-auto-group-details]'
    )
    assert.ok(details)
    assert.equal(details.textContent?.includes('No groups configured.'), true)

    await act(async () => root.unmount())
    container.remove()
  })

  test('does not label saved groups unavailable while group metadata is loading', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness
          group='auto'
          autoGroups={['vip']}
          groupDataStatus='loading'
        />
      )
    )

    const trigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label="View cross-group details"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.click())

    const details = document.body.querySelector<HTMLElement>(
      '[data-api-key-auto-group-details]'
    )
    assert.ok(details)
    assert.equal(details.textContent?.includes('Loading...'), true)
    assert.equal(details.textContent?.includes('Unavailable'), false)

    await act(async () => root.unmount())
    container.remove()
  })

  test('does not report an empty legacy order when the default order request fails', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <CellHarness
          group='auto'
          autoGroups={null}
          groupDataStatus='ready'
          defaultAutoGroupsStatus='error'
        />
      )
    )

    const trigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label="View cross-group details"]'
    )
    assert.ok(trigger)
    await act(async () => trigger.click())

    const details = document.body.querySelector<HTMLElement>(
      '[data-api-key-auto-group-details]'
    )
    assert.ok(details)
    assert.equal(details.textContent?.includes('Failed to load'), true)
    assert.equal(details.textContent?.includes('No groups configured.'), false)

    await act(async () => root.unmount())
    container.remove()
  })
})
