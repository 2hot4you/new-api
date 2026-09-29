import { isRedirect } from '@tanstack/react-router'
import { afterEach, describe, expect, test } from 'vitest'

import { ROLE } from '@/lib/roles'
import { Route } from '@/routes/_authenticated/billing-calendar'
import { useAuthStore } from '@/stores/auth-store'

const originalAuth = useAuthStore.getState().auth

afterEach(() => useAuthStore.setState({ auth: originalAuth }))

describe('billing calendar route access', () => {
  test.each([ROLE.GUEST, ROLE.USER])(
    'redirects role %s to 403',
    async (role) => {
      useAuthStore.setState({
        auth: {
          ...originalAuth,
          user: { id: 1, username: 'viewer', role },
        },
      })
      try {
        await Route.options.beforeLoad?.({} as never)
        throw new Error('expected redirect')
      } catch (error) {
        expect(isRedirect(error)).toBe(true)
        if (isRedirect(error)) expect(error.options.to).toBe('/403')
      }
    }
  )

  test.each([ROLE.ADMIN, ROLE.SUPER_ADMIN])('allows role %s', async (role) => {
    useAuthStore.setState({
      auth: {
        ...originalAuth,
        user: { id: 1, username: 'admin', role },
      },
    })
    expect(() => Route.options.beforeLoad?.({} as never)).not.toThrow()
  })
})
