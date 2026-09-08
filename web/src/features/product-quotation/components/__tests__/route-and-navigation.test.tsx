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
import { isRedirect } from '@tanstack/react-router'
import { act, render, renderHook, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import type { ComponentType } from 'react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import viLocale from '@/i18n/locales/vi.json'
import zhTw from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import { STATIC_I18N_KEYS } from '@/i18n/static-keys'
import { ROLE } from '@/lib/roles'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    useLocation: vi.fn(
      (options?: { select?: (location: { pathname: string }) => unknown }) =>
        options?.select?.({ pathname: '/dashboard' }) ?? {
          pathname: '/dashboard',
        }
    ),
  }
})

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: {}, loading: false, error: null }),
}))

vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({
    models: [],
    vendors: [],
    groupRatio: {},
    usableGroup: {},
    pricingVersion: null,
    dataUpdatedAt: 0,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    usdExchangeRate: 7,
  }),
}))

vi.mock('@/features/product-quotation/hooks/use-quotation-draft', () => ({
  useQuotationDraft: () => ({
    draft: {
      title: '',
      customer: '',
      quotedBy: '',
      quoteDate: '2026-09-08',
      globalDiscount: null,
      priceBasis: { type: 'raw' },
      selectedModelIds: [],
      providerOverrides: {},
    },
    setDraft: vi.fn(),
    clearDraft: vi.fn(),
    saveStatus: 'saved',
  }),
}))

const routePath =
  '../../../../routes/_authenticated/product-quotation/index.tsx'
const routeModules = import.meta.glob(
  '../../../../routes/_authenticated/product-quotation/index.tsx',
  { eager: true }
)

const { useSidebarView } = await import('@/hooks/use-sidebar-view')

const runtimeQuotationKeys = [
  'Input',
  'Output',
  'Cache read',
  'Cache write',
  'Image input',
  'Video input',
  'Audio input',
  'Audio output',
  'Dynamic pricing',
  'Base charge',
  'Task usage pricing',
  'Request',
  'Input price',
  'Completion price',
  'Cache read price',
  'Cache create price',
  'Cache create (1h) price',
  'Image input price',
  'Image output price',
  'Audio input price',
  'Audio output price',
  '1M token',
  'variable',
  'request',
  'count',
  'image',
  'second',
  '{{resolution}} without video input',
  '{{resolution}} with video input',
  '{{tier}} output',
  '{{resolution}}; fps {{fps}}; extra frames {{extraFrames}}; Token = ceil(width x height x (fps x duration + extra frames) / 1024)',
] as const

const clearEnglishDiscountCopy = {
  ' zhe': ' / 10 of list price',
  '{{discount}} zhe': '{{discount}} / 10 of list price',
  'Calculator discount (zhe)': 'Calculator discount (10 = full price)',
  'Global discount (zhe)': 'Global discount (10 = full price)',
  '{{provider}} discount (zhe)': '{{provider}} discount (10 = full price)',
  'Calculator discount (10 = full price)':
    'Calculator discount (10 = full price)',
  'Global discount (10 = full price)': 'Global discount (10 = full price)',
  '{{provider}} discount (10 = full price)':
    '{{provider}} discount (10 = full price)',
  '{{discount}} tenths of list price': '{{discount}} tenths of list price',
  ' / 10 of list price': ' / 10 of list price',
  'Enter finite positive rates and a discount from 0 to 10.':
    'Enter finite positive rates and a discount greater than 0 and at most 10.',
  'Enter finite positive rates and a discount greater than 0 and at most 10.':
    'Enter finite positive rates and a discount greater than 0 and at most 10.',
} as const

async function translateWith(
  locale: string,
  resource: { translation: Record<string, unknown> }
) {
  const instance = createInstance()
  await instance.init({
    lng: locale,
    fallbackLng: false,
    resources: { [locale]: resource },
  })
  return instance.t.bind(instance)
}

type QuotationRouteModule = {
  Route: {
    options: {
      beforeLoad?: (context: never) => unknown
      component?: ComponentType
    }
  }
}

function userWithRole(role: number): AuthUser {
  return { id: role, username: `role-${role}`, role }
}

function quotationRoute(): QuotationRouteModule {
  const routeModule = routeModules[routePath]
  expect(
    routeModule,
    'the product quotation route must be registered'
  ).toBeDefined()
  return routeModule as QuotationRouteModule
}

function adminUrls(): string[] {
  const { result } = renderHook(() => useSidebarView())
  const admin = result.current.navGroups.find((group) => group.id === 'admin')
  return (admin?.items ?? []).flatMap((item) =>
    'url' in item && item.url ? [String(item.url)] : []
  )
}

afterEach(() => {
  useAuthStore.getState().auth.reset('complete')
})

describe('product quotation route access', () => {
  test.each([ROLE.ADMIN, ROLE.USER])(
    'redirects role %s to /403',
    async (role) => {
      useAuthStore.getState().auth.setUser(userWithRole(role))
      const beforeLoad = quotationRoute().Route.options.beforeLoad

      expect(beforeLoad).toBeTypeOf('function')
      try {
        await beforeLoad?.({} as never)
        throw new Error('expected the route guard to redirect')
      } catch (error) {
        expect(isRedirect(error)).toBe(true)
        if (isRedirect(error)) expect(error.options.to).toBe('/403')
      }
    }
  )

  test('allows a super admin to render the quotation workspace', async () => {
    useAuthStore.getState().auth.setUser(userWithRole(ROLE.SUPER_ADMIN))
    const route = quotationRoute().Route

    expect(() => route.options.beforeLoad?.({} as never)).not.toThrow()
    const Component = route.options.component
    expect(Component).toBeTypeOf('function')
    if (!Component) return

    render(<Component />)
    expect(
      screen.getByRole('heading', {
        name: 'Product quotation and calculator',
      })
    ).toBeInTheDocument()
  })
})

describe('product quotation sidebar navigation', () => {
  test.each([ROLE.ADMIN, ROLE.USER])(
    'omits the quotation entry for non-root role %s',
    (role) => {
      act(() => useAuthStore.getState().auth.setUser(userWithRole(role)))

      expect(adminUrls()).not.toContain('/product-quotation')
    }
  )

  test('places the root-only entry exactly between Task Plugins and System Settings', () => {
    act(() =>
      useAuthStore.getState().auth.setUser(userWithRole(ROLE.SUPER_ADMIN))
    )

    const urls = adminUrls()
    const taskPluginsIndex = urls.indexOf('/task-plugins')
    const systemSettingsIndex = urls.indexOf('/system-settings/site')

    expect(urls.slice(taskPluginsIndex, systemSettingsIndex + 1)).toEqual([
      '/task-plugins',
      '/product-quotation',
      '/system-settings/site',
    ])
  })
})

describe('product quotation runtime translations', () => {
  test('registers every runtime dimension, unit, and generated export key', () => {
    const registry = new Set<string>(STATIC_I18N_KEYS)

    expect(runtimeQuotationKeys.filter((key) => !registry.has(key))).toEqual([])
    expect(
      runtimeQuotationKeys.filter((key) => !Object.hasOwn(en.translation, key))
    ).toEqual([])
  })

  test('provides polished Simplified Chinese for live and exported quotation rows', async () => {
    const t = await translateWith('zh', zh)

    expect(
      Object.fromEntries(
        [
          'Unit',
          'Base charge',
          'Input',
          'Output',
          'Cache read',
          'Cache write',
          'Image input',
          'Video input',
          'Audio input',
          'Audio output',
          'Dynamic pricing',
          'Task usage pricing',
          'Request',
          'Input price',
          'Completion price',
          'Cache read price',
          'Cache create price',
          'Cache create (1h) price',
          'Image input price',
          'Image output price',
          'Audio input price',
          'Audio output price',
          '1M token',
          'variable',
          'request',
          'count',
          'image',
          'second',
        ].map((key) => [key, t(key)])
      )
    ).toEqual({
      Unit: '单位',
      'Base charge': '基础费用',
      Input: '输入',
      Output: '输出',
      'Cache read': '缓存读取',
      'Cache write': '缓存写入',
      'Image input': '图片输入',
      'Video input': '视频输入',
      'Audio input': '音频输入',
      'Audio output': '音频输出',
      'Dynamic pricing': '动态计价',
      'Task usage pricing': 'Task 用量计价',
      Request: '请求',
      'Input price': '输入价格',
      'Completion price': '补全价格',
      'Cache read price': '缓存读取价格',
      'Cache create price': '缓存写入价格',
      'Cache create (1h) price': '1 小时缓存写入价格',
      'Image input price': '图像输入价格',
      'Image output price': '图像输出价格',
      'Audio input price': '音频输入价格',
      'Audio output price': '音频输出价格',
      '1M token': '每百万 Token',
      variable: '按变量计费',
      request: '每次请求',
      count: '每次',
      image: '每张图片',
      second: '每秒',
    })

    expect(
      t('{{resolution}} without video input', { resolution: '720p' })
    ).toBe('720p（不含视频输入）')
    expect(t('{{resolution}} with video input', { resolution: '1080p' })).toBe(
      '1080p（包含视频输入）'
    )
    expect(t('{{tier}} output', { tier: 'high' })).toBe('high 输出')
    expect(t('Calculator discount (10 = full price)')).toBe(
      '计算器折扣（10 = 原价）'
    )
    expect(t('{{discount}} tenths of list price', { discount: 5 })).toBe('5 折')
    expect(t(' / 10 of list price')).toBe(' 折')
    expect(t('Failed to export HTML quotation')).toBe('HTML 报价单导出失败')
    expect(
      t(
        '{{resolution}}; fps {{fps}}; extra frames {{extraFrames}}; Token = ceil(width x height x (fps x duration + extra frames) / 1024)',
        { resolution: '720p', fps: 24, extraFrames: 4 }
      )
    ).toBe(
      '720p；FPS 24；额外帧 4；Token = ceil(宽 × 高 × (FPS × 时长 + 额外帧) / 1024)'
    )
  })

  test.each([
    ['en', en],
    ['fr', fr],
    ['ja', ja],
    ['ru', ru],
    ['vi', viLocale],
    ['zh-TW', zhTw],
  ] as const)(
    'uses clear discount boundaries and no unexplained zhe copy in %s',
    async (locale, resource) => {
      const t = await translateWith(locale, resource)

      for (const [key, expected] of Object.entries(clearEnglishDiscountCopy)) {
        expect(t(key)).toBe(expected)
      }
    }
  )
})
