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
import { afterAll as after, afterEach, describe, test } from 'vitest'

import { Window } from 'happy-dom'
import type { ComponentType } from 'react'

import type { ApiKey } from '../../types'

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
  'MouseEvent',
  'PointerEvent',
  'FocusEvent',
  'KeyboardEvent',
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
const clipboardWrites: string[] = []
Object.defineProperty(domWindow.navigator, 'clipboard', {
  configurable: true,
  value: {
    writeText: async (value: string) => {
      clipboardWrites.push(value)
    },
  },
})

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { toast } = await import('sonner')
const { IpRestrictionsCell, ModelLimitsCell } =
  await import('../api-key-restriction-details')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  interpolation: { escapeValue: false },
  resources: {
    en: {
      translation: {
        '{{count}} model(s)': '{{count}} model(s)',
        '{{count}} IP(s)': '{{count}} IP(s)',
        'Model restrictions': 'Model restrictions',
        'IP restrictions': 'IP restrictions',
        'View model restrictions': 'View model restrictions',
        'View IP restrictions': 'View IP restrictions',
        'Loading model providers': 'Loading model providers',
        'Provider unavailable': 'Provider unavailable',
        'Unknown provider': 'Unknown provider',
        'Copy all models': 'Copy all models',
        'Copy all IPs': 'Copy all IPs',
        'Copy model {{model}}': 'Copy model {{model}}',
        'Copy IP {{ip}}': 'Copy IP {{ip}}',
        'Copied to clipboard': 'Copied to clipboard',
        'Copied!': 'Copied!',
        Copied: 'Copied',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

type ModelDisplayInfo = Record<
  string,
  { modelIcon?: string; providerName?: string; providerIcon?: string }
>

type ModelLimitsCellWithDetailsProps = {
  apiKey: ApiKey
  modelDisplayInfo?: ModelDisplayInfo
  modelDisplayStatus?: 'error' | 'loading' | 'ready'
}

const ModelLimitsCellWithDetails =
  ModelLimitsCell as ComponentType<ModelLimitsCellWithDetailsProps>

function apiKey(overrides: Partial<ApiKey>): ApiKey {
  return {
    id: 1,
    name: 'restricted-key',
    key: 'masked',
    status: 1,
    remain_quota: 0,
    used_quota: 0,
    unlimited_quota: true,
    expired_time: -1,
    created_time: 1,
    accessed_time: 0,
    group: 'default',
    auto_groups: null,
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    is_default: false,
    ...overrides,
  }
}

async function clickButton(label: string): Promise<void> {
  const button = document.querySelector<HTMLButtonElement>(
    `button[aria-label^="${label}"]`
  )
  assert.ok(button)
  await act(async () => button.click())
}

async function clickCopyButton(label: string): Promise<void> {
  const button = document.querySelector<HTMLButtonElement>(
    `button[aria-label="${label}"]`
  )
  assert.ok(button)
  await act(async () => {
    button.click()
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()
  })
}

async function pressEnter(button: HTMLButtonElement): Promise<void> {
  button.focus()
  await act(async () => {
    const keyDown = new KeyboardEvent('keydown', {
      bubbles: true,
      cancelable: true,
      key: 'Enter',
    })
    const shouldRunNativeActivation = button.dispatchEvent(keyDown)
    if (shouldRunNativeActivation) button.click()
    button.dispatchEvent(
      new KeyboardEvent('keyup', { bubbles: true, key: 'Enter' })
    )
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()
  })
}

describe('API key restriction table cells', () => {
  after(() => {
    domWindow.close()
  })
  afterEach(() => {
    toast.dismiss()
    document.body.replaceChildren()
  })

  test('shows the first five configured model icons in token order and an overflow ellipsis', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    const modelIds = [
      'claude-opus-5',
      'claude-opus-4-8',
      'gpt-5.6-sol',
      'deepseek-v4-pro',
      'gemini-3.7-flash',
      'grok-4.6',
    ]

    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelLimitsCellWithDetails
            apiKey={apiKey({
              model_limits_enabled: true,
              model_limits: modelIds.join(','),
            })}
            modelDisplayInfo={{
              'claude-opus-5': { modelIcon: 'Claude.Color' },
              'claude-opus-4-8': { modelIcon: 'Claude.Color' },
              'gpt-5.6-sol': {
                modelIcon: 'OpenAI.Color',
              },
              'deepseek-v4-pro': { modelIcon: 'DeepSeek.Color' },
              'gemini-3.7-flash': { modelIcon: 'Gemini.Color' },
              'grok-4.6': { modelIcon: 'Grok.Color' },
            }}
          />
        </I18nextProvider>
      )
    )

    const summaryIcons = container.querySelectorAll<HTMLElement>(
      '[data-model-summary-icon]'
    )
    assert.deepEqual(
      Array.from(summaryIcons, (icon) => icon.dataset.modelSummaryIcon),
      [
        'Claude.Color',
        'Claude.Color',
        'OpenAI.Color',
        'DeepSeek.Color',
        'Gemini.Color',
      ]
    )
    assert.ok(container.querySelector('[data-model-summary-overflow]'))
    assert.equal(container.querySelector('[data-slot="status-badge"]'), null)
    const trigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label^="View model restrictions"]'
    )
    assert.ok(trigger)
    assert.match(trigger.getAttribute('aria-label') ?? '', /6 model\(s\)/)

    await act(async () => root.unmount())
    container.remove()
  })

  test('groups models by provider, uses model icons, and copies one or all model IDs', async () => {
    clipboardWrites.length = 0
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelLimitsCellWithDetails
            apiKey={apiKey({
              model_limits_enabled: true,
              model_limits: 'claude-opus-5,claude-opus-4-8,gpt-5.6-sol',
            })}
            modelDisplayInfo={{
              'claude-opus-5': {
                modelIcon: 'Claude.Color',
                providerName: 'Anthropic',
                providerIcon: 'Anthropic.Color',
              },
              'claude-opus-4-8': {
                modelIcon: 'Claude.Color',
                providerName: 'Anthropic',
                providerIcon: 'Anthropic.Color',
              },
              'gpt-5.6-sol': {
                modelIcon: 'OpenAI.Color',
                providerName: 'OpenAI',
                providerIcon: 'OpenAI.Color',
              },
            }}
          />
        </I18nextProvider>
      )
    )

    await clickButton('View model restrictions')
    const header = document.querySelector<HTMLElement>(
      '[data-api-key-restriction-header]'
    )
    assert.ok(header)
    assert.equal(header.classList.contains('flex-col'), true)
    assert.equal(header.classList.contains('sm:flex-row'), true)
    const headerActions = document.querySelector<HTMLElement>(
      '[data-api-key-restriction-header-actions]'
    )
    assert.ok(headerActions)
    assert.equal(headerActions.classList.contains('w-full'), true)
    assert.equal(headerActions.classList.contains('sm:w-auto'), true)
    const providerHeaders = document.querySelectorAll<HTMLElement>(
      '[data-provider-icon]'
    )
    assert.deepEqual(
      Array.from(providerHeaders, (header) => header.dataset.providerIcon),
      ['Anthropic.Color', 'OpenAI.Color']
    )
    assert.match(providerHeaders[0]?.textContent ?? '', /Anthropic/)
    assert.match(providerHeaders[1]?.textContent ?? '', /OpenAI/)
    const providerGroups = document.querySelectorAll<HTMLElement>(
      '[data-model-provider-group]'
    )
    assert.equal(providerGroups.length, 2)
    assert.equal(providerGroups[0]?.classList.contains('border'), true)
    assert.equal(providerGroups[0]?.classList.contains('overflow-hidden'), true)
    assert.equal(providerHeaders[0]?.classList.contains('bg-muted/50'), true)
    assert.equal(
      document
        .querySelector('[data-model-provider-rows]')
        ?.classList.contains('divide-y'),
      true
    )

    const modelIds = document.querySelectorAll<HTMLButtonElement>(
      '[data-model-restriction-id]'
    )
    assert.deepEqual(
      Array.from(modelIds, (modelId) => [
        modelId.dataset.modelIcon,
        modelId.textContent?.trim(),
      ]),
      [
        ['Claude.Color', 'claude-opus-5'],
        ['Claude.Color', 'claude-opus-4-8'],
        ['OpenAI.Color', 'gpt-5.6-sol'],
      ]
    )
    for (const modelId of modelIds) {
      assert.equal(modelId.classList.contains('truncate'), false)
      assert.equal(modelId.classList.contains('break-all'), true)
    }

    const modelCopyButton = document.querySelector<HTMLButtonElement>(
      'button[aria-label="Copy model claude-opus-5"]'
    )
    assert.ok(modelCopyButton)
    await pressEnter(modelCopyButton)
    assert.equal(modelCopyButton.getAttribute('aria-label'), 'Copied')
    assert.equal(
      toast
        .getToasts()
        .some(
          (notification) =>
            'title' in notification &&
            notification.title === 'Copied to clipboard'
        ),
      true
    )
    toast.dismiss()
    await clickCopyButton('Copy all models')
    assert.equal(
      toast
        .getToasts()
        .some(
          (notification) =>
            'title' in notification &&
            notification.title === 'Copied to clipboard'
        ),
      true
    )
    assert.deepEqual(clipboardWrites, [
      'claude-opus-5',
      'claude-opus-5,claude-opus-4-8,gpt-5.6-sol',
    ])

    const list = document.querySelector<HTMLElement>(
      '[data-api-key-restriction-list="models"]'
    )
    assert.ok(list)
    assert.equal(list.classList.contains('overflow-y-auto'), true)
    assert.equal(list.classList.contains('overflow-x-hidden'), true)
    const details = document.querySelector<HTMLElement>(
      '[data-api-key-restriction-details]'
    )
    assert.ok(details)
    assert.equal(details.classList.contains('max-h-(--available-height)'), true)
    assert.equal(
      details.classList.contains('w-[min(26rem,calc(100vw-2rem))]'),
      true
    )

    await act(async () => root.unmount())
    container.remove()
  })

  test('keeps model IDs visible while provider metadata is loading, unavailable, or missing', async () => {
    const statuses = [
      ['loading', 'Loading model providers'],
      ['error', 'Provider unavailable'],
      ['ready', 'Unknown provider'],
    ] as const

    for (const [status, providerLabel] of statuses) {
      const container = document.createElement('div')
      document.body.append(container)
      const root = createRoot(container)

      await act(async () =>
        root.render(
          <I18nextProvider i18n={i18n}>
            <ModelLimitsCellWithDetails
              apiKey={apiKey({
                model_limits_enabled: true,
                model_limits: 'unmapped-model-id',
              })}
              modelDisplayStatus={status}
            />
          </I18nextProvider>
        )
      )

      const trigger = container.querySelector<HTMLButtonElement>(
        'button[aria-label^="View model restrictions"]'
      )
      assert.ok(trigger)
      assert.equal(trigger.tagName, 'BUTTON')
      assert.equal(trigger.type, 'button')
      assert.equal(trigger.disabled, false)
      assert.equal(trigger.getAttribute('aria-expanded'), 'false')
      await pressEnter(trigger)

      assert.equal(trigger.getAttribute('aria-expanded'), 'true')
      assert.match(document.body.textContent ?? '', /unmapped-model-id/)
      assert.match(document.body.textContent ?? '', new RegExp(providerLabel))

      await act(async () => root.unmount())
      container.remove()
    }
  })

  test('shows the first IP plus an ellipsis and copies any IP from the detail badges', async () => {
    clipboardWrites.length = 0
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <IpRestrictionsCell
            apiKey={apiKey({
              allow_ips: '203.0.113.10\n198.51.100.0/24\n2001:db8::1',
            })}
          />
        </I18nextProvider>
      )
    )

    assert.match(container.textContent ?? '', /203\.0\.113\.10/)
    assert.match(container.textContent ?? '', /…/)
    assert.equal(
      container.querySelector('[data-slot="status-badge"]')?.textContent,
      '203.0.113.10'
    )
    const trigger = container.querySelector<HTMLButtonElement>(
      'button[aria-label^="View IP restrictions"]'
    )
    assert.ok(trigger)
    assert.match(
      trigger.getAttribute('aria-label') ?? '',
      /203\.0\.113\.10.*3 IP\(s\)/
    )
    await clickButton('View IP restrictions')
    const bodyText = document.body.textContent ?? ''
    assert.match(bodyText, /203\.0\.113\.10/)
    assert.match(bodyText, /198\.51\.100\.0\/24/)
    assert.match(bodyText, /2001:db8::1/)

    const list = document.querySelector<HTMLElement>(
      '[data-api-key-restriction-list="ips"]'
    )
    assert.ok(list)
    assert.equal(list.classList.contains('overflow-y-auto'), true)
    assert.equal(list.classList.contains('overflow-x-hidden'), true)
    const ipBadges = document.querySelectorAll<HTMLButtonElement>(
      '[data-ip-restriction-value]'
    )
    assert.equal(ipBadges.length, 3)
    const targetIp = document.querySelector<HTMLElement>(
      '[data-ip-restriction-value="198.51.100.0/24"]'
    )
    const targetIpButton = targetIp?.closest('button')
    assert.ok(targetIpButton)
    assert.equal(
      targetIpButton.getAttribute('aria-label'),
      'Copy IP 198.51.100.0/24'
    )
    assert.equal(targetIpButton.classList.contains('bg-muted/60'), true)
    await clickCopyButton('Copy IP 198.51.100.0/24')
    assert.equal(
      toast
        .getToasts()
        .some(
          (notification) =>
            'title' in notification &&
            notification.title === 'Copied to clipboard'
        ),
      true
    )
    toast.dismiss()
    await clickCopyButton('Copy all IPs')
    assert.equal(
      toast
        .getToasts()
        .some(
          (notification) =>
            'title' in notification &&
            notification.title === 'Copied to clipboard'
        ),
      true
    )
    assert.deepEqual(clipboardWrites, [
      '198.51.100.0/24',
      '203.0.113.10,198.51.100.0/24,2001:db8::1',
    ])

    await act(async () => root.unmount())
    container.remove()
  })
})
