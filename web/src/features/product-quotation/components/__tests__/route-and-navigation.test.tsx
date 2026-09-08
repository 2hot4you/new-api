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
import type { ComponentType } from 'react'
import { afterEach, describe, expect, test, vi } from 'vitest'

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
