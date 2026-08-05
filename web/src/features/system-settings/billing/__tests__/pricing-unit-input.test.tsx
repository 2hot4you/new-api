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
  'HTMLFormElement',
  'HTMLInputElement',
  'HTMLLabelElement',
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
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createMemoryHistory, createRootRoute, createRouter, RouterProvider } =
  await import('@tanstack/react-router')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const zh = (await import('../../../../i18n/locales/zh.json')).default
const { MoliiGrokPricingSection } =
  await import('../molii-grok-pricing-section')
const { PricingUnitInput } = await import('../pricing-unit-input')
const { StarAIVideoPricingSection } =
  await import('../starai-video-pricing-section')

const i18n = createInstance()
await i18n.use(initReactI18next).init({ lng: 'zh', resources: { zh } })

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('PricingUnitInput', () => {
  after(() => {
    domWindow.close()
  })

  test('renders the localized unit at the inline end of its input group', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <PricingUnitInput
          aria-label='价格'
          type='number'
          value='0.05'
          unit='¥ / 张'
          readOnly
        />
      )
    })

    const input = container.querySelector('input[aria-label="价格"]')
    assert.ok(input)
    const inputGroup = input.closest('[data-slot="input-group"]')
    assert.ok(inputGroup)
    const addon = inputGroup.querySelector(
      '[data-slot="input-group-addon"][data-align="inline-end"]'
    )
    assert.ok(addon)
    assert.equal(addon.textContent, '¥ / 张')

    await act(async () => root.unmount())
    container.remove()
  })

  test('shows Chinese Grok labels with units inside their inputs', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient()
    const rootRoute = createRootRoute({
      component: () => (
        <MoliiGrokPricingSection
          defaultValues={{
            image_standard_input: 0.002,
            image_standard_1k: 0.02,
            image_standard_2k: 0.02,
            image_quality_input: 0.01,
            image_quality_1k: 0.05,
            image_quality_2k: 0.07,
            video_15_image_input: 0.01,
            video_15_480p: 0.08,
            video_15_720p: 0.14,
            video_15_1080p: 0.25,
            video_image_input: 0.002,
            video_video_input: 0.01,
            video_480p: 0.05,
            video_720p: 0.07,
            tool_web_search: 5,
            tool_x_search: 5,
            tool_code_execution: 5,
            tool_attachment_search: 10,
            tool_collections_search: 10,
            tool_image_generation: 0.07,
          }}
        />
      ),
    })
    const router = createRouter({
      routeTree: rootRoute,
      history: createMemoryHistory({ initialEntries: ['/'] }),
    })
    await router.load()

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <QueryClientProvider client={queryClient}>
            <RouterProvider router={router} />
          </QueryClientProvider>
        </I18nextProvider>
      )
    })

    const labels = [...container.querySelectorAll('label')]
    assert.deepEqual(
      labels.map((item) => item.textContent),
      [
        '图片 · 标准 · 素材输入',
        '图片 · 标准 · 1K 输出',
        '图片 · 标准 · 2K 输出',
        '图片 · 高质量 · 素材输入',
        '图片 · 高质量 · 1K 输出',
        '图片 · 高质量 · 2K 输出',
        '视频 1.5 · 图片输入',
        '视频 1.5 · 480p 输出',
        '视频 1.5 · 720p 输出',
        '视频 1.5 · 1080p 输出',
        '视频 · 图片输入',
        '视频 · 视频输入',
        '视频 · 480p 输出',
        '视频 · 720p 输出',
        '工具 · 网页搜索',
        '工具 · X 搜索',
        '工具 · 代码执行',
        '工具 · 附件搜索',
        '工具 · 集合搜索',
        '工具 · 图片生成',
      ]
    )
    assert.deepEqual(
      [...container.querySelectorAll('[data-slot="input-group-addon"]')].map(
        (item) => item.textContent
      ),
      [
        '¥ / 张',
        '¥ / 张',
        '¥ / 张',
        '¥ / 张',
        '¥ / 张',
        '¥ / 张',
        '¥ / 张',
        '¥ / 秒',
        '¥ / 秒',
        '¥ / 秒',
        '¥ / 张',
        '¥ / 秒',
        '¥ / 秒',
        '¥ / 秒',
        '¥ / 千次调用',
        '¥ / 千次调用',
        '¥ / 千次调用',
        '¥ / 千次调用',
        '¥ / 千次调用',
        '¥ / 张成品图',
      ]
    )

    const label = labels[0]
    assert.ok(label)
    const formItem = label.closest('[data-slot="form-item"]')
    assert.ok(formItem)
    assert.equal(
      formItem.querySelector('[data-slot="input-group-addon"]')?.textContent,
      '¥ / 张'
    )
    assert.equal(formItem.querySelector('[data-slot="form-description"]'), null)

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })

  test('keeps the Seedance explanation below while moving its unit into the input', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    const queryClient = new QueryClient()
    const rootRoute = createRootRoute({
      component: () => (
        <StarAIVideoPricingSection
          defaultValues={{
            standard_720p: 1,
            standard_720p_video: 1,
            standard_1080p: 1,
            standard_1080p_video: 1,
            standard_4k: 1,
            standard_4k_video: 1,
            fast_720p: 1,
            fast_720p_video: 1,
          }}
        />
      ),
    })
    const router = createRouter({
      routeTree: rootRoute,
      history: createMemoryHistory({ initialEntries: ['/'] }),
    })
    await router.load()

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <QueryClientProvider client={queryClient}>
            <RouterProvider router={router} />
          </QueryClientProvider>
        </I18nextProvider>
      )
    })

    const label = [...container.querySelectorAll('label')].find(
      (item) => item.textContent === 'Seedance 2.0 · 480p/720p · 文本/图片输入'
    )
    assert.ok(label)
    const formItem = label.closest('[data-slot="form-item"]')
    assert.ok(formItem)
    assert.equal(
      formItem.querySelector('[data-slot="input-group-addon"]')?.textContent,
      '¥ / 百万 Token'
    )
    assert.equal(
      formItem.querySelector('[data-slot="form-description"]')?.textContent,
      '输入不包含参考视频'
    )

    await act(async () => root.unmount())
    queryClient.clear()
    container.remove()
  })
})
