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

import { Window } from 'happy-dom'
import { afterAll as after, describe, test } from 'vitest'

import zh from '@/i18n/locales/zh.json'

import type { PricingModel } from '../../types'

const domWindow = new Window()
domWindow.document.write('<!doctype html><html><body></body></html>')
Object.defineProperty(domWindow.document, 'compatMode', {
  configurable: true,
  value: 'CSS1Compat',
})
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
  'matchMedia',
  'customElements',
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
const { ModelCard } = await import('../model-card')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'zh',
  resources: { zh },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('Seedance marketplace model card pricing', () => {
  after(() => domWindow.close())

  test('labels the tier price per million tokens instead of as one month', async () => {
    const model: PricingModel = {
      id: 1,
      model_name: 'doubao-seedance-2-0-260128',
      billing_currency: 'CNY',
      quota_type: 0,
      model_ratio: 23,
      completion_ratio: 1,
      model_price: 0,
      enable_groups: ['ByteDance'],
      supported_endpoint_types: ['openai-video'],
      video_pricing: {
        unit: 'cny_per_million_tokens',
        fps: 24,
        extra_frames: 1,
        rows: [
          {
            resolutions: ['480p', '720p'],
            without_video: 46,
            with_video: 28,
          },
        ],
      },
    }
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <ModelCard model={model} onClick={() => undefined} />
        </I18nextProvider>
      )
    })

    assert.match(container.textContent ?? '', /每百万 Token/)
    assert.doesNotMatch(container.textContent ?? '', /1 个月/)

    await act(async () => root.unmount())
    container.remove()
  })
})
