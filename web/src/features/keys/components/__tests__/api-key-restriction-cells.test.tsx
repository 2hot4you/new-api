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

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { IpRestrictionsCell, ModelLimitsCell } =
  await import('../api-key-restriction-details')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
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
  { providerName?: string; providerIcon?: string }
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
    `button[aria-label="${label}"]`
  )
  assert.ok(button)
  await act(async () => button.click())
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
  })
}

describe('API key restriction table cells', () => {
  after(() => {
    domWindow.close()
  })

  test('opens every restricted model with its provider and full model ID', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () =>
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelLimitsCellWithDetails
            apiKey={apiKey({
              model_limits_enabled: true,
              model_limits: 'gpt-5.6-sol,deepseek-v4-pro-202606',
            })}
            modelDisplayInfo={{
              'gpt-5.6-sol': {
                providerName: 'OpenAI',
                providerIcon: 'OpenAI.Color',
              },
              'deepseek-v4-pro-202606': {
                providerName: 'DeepSeek',
                providerIcon: 'DeepSeek.Color',
              },
            }}
          />
        </I18nextProvider>
      )
    )

    assert.equal(container.textContent?.includes('2 model(s)'), true)
    await clickButton('View model restrictions')
    assert.match(document.body.textContent ?? '', /OpenAI/)
    assert.match(document.body.textContent ?? '', /gpt-5\.6-sol/)
    assert.match(document.body.textContent ?? '', /DeepSeek/)
    assert.match(document.body.textContent ?? '', /deepseek-v4-pro-202606/)

    const modelIds = document.querySelectorAll<HTMLElement>(
      '[data-model-restriction-id]'
    )
    assert.equal(modelIds.length, 2)
    for (const modelId of modelIds) {
      assert.equal(modelId.classList.contains('truncate'), false)
      assert.equal(modelId.classList.contains('break-all'), true)
    }

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
        'button[aria-label="View model restrictions"]'
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

  test('opens all configured IP addresses and CIDR ranges instead of truncating to the count', async () => {
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

    assert.equal(container.textContent?.includes('3 IP(s)'), true)
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

    await act(async () => root.unmount())
    container.remove()
  })
})
