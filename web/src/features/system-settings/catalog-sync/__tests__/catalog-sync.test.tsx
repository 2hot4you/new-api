import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import axios from 'axios'
import i18next from 'i18next'
import { afterEach, describe, expect, test, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import viLocale from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { listCatalogSyncHistory } from '../api'
import { CatalogSyncSection } from '../index'
import type { CatalogChange, CatalogSyncPlan } from '../types'

const statusTarget = {
  role: 'target',
  source_id: 'dev',
  target_id: 'prod-a',
  source_ready: false,
  target_ready: true,
  management_ready: true,
}
const emptyHistory = { items: [], total: 0, offset: 0, limit: 20 }

function targetResponseData(
  url: string,
  scope: string,
  history: object = emptyHistory
) {
  if (url.endsWith('/status')) return statusTarget
  if (url.endsWith('/verify/methods')) {
    return {
      scope,
      methods: [{ method: 'session', available: true }],
      oauth_providers: [],
      password_encryption_enabled: false,
    }
  }
  return history
}

function change(partial: Partial<CatalogChange> = {}): CatalogChange {
  return {
    kind: 'model',
    key: 'model-a',
    action: 'update',
    reason: 'source_changed',
    before: {
      kind: 'model',
      key: 'model-a',
      value: '{"billing_currency":"USD","description":"old"}',
    },
    base: {
      kind: 'model',
      key: 'model-a',
      value: '{"billing_currency":"USD","description":"old"}',
    },
    after: {
      kind: 'model',
      key: 'model-a',
      value: '{"billing_currency":"CNY","description":"new"}',
    },
    confirmation_unit: 'opaque-model-unit',
    ...partial,
  }
}

function plan(
  changes: CatalogChange[] = [change()],
  partial: Partial<CatalogSyncPlan> = {}
): CatalogSyncPlan {
  return {
    id: 'a'.repeat(64),
    kind: 'sync',
    source_id: 'dev',
    target_id: 'prod-a',
    source_exported_at: 1700000000,
    source_digest: `sha256:${'b'.repeat(64)}`,
    digest: 'c'.repeat(64),
    expires_at: Math.floor(Date.now() / 1000) + 600,
    resolution: { overwrite_keys: null, confirm_deletes: false },
    changes,
    executable: true,
    ...partial,
  }
}

function mockTargetRequests(
  previewPlan: CatalogSyncPlan,
  resolvedPlan: CatalogSyncPlan = previewPlan
) {
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.endsWith('/status') ? statusTarget : emptyHistory,
    },
  }))
  const post = vi.spyOn(api, 'post').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.endsWith('/preview') ? previewPlan : resolvedPlan,
    },
  }))
  return { get, post }
}

function renderSection(role: number = ROLE.SUPER_ADMIN) {
  useAuthStore.getState().auth.setUser({ id: 7, username: 'root', role })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <CatalogSyncSection />
    </QueryClientProvider>
  )
  return client
}

afterEach(async () => {
  useAuthStore.getState().auth.reset()
  window.sessionStorage.clear()
  vi.restoreAllMocks()
  await i18next.changeLanguage('en')
})

describe('managed catalog sync', () => {
  test('non-root user cannot mount a sync control or fetch status', () => {
    const get = vi.spyOn(api, 'get')
    renderSection(ROLE.ADMIN)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(get).not.toHaveBeenCalled()
  })

  test('source role displays readiness but cannot preview or apply', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          role: 'source',
          source_id: 'dev',
          source_ready: true,
          target_ready: false,
          management_ready: false,
        },
      },
    })
    const post = vi.spyOn(api, 'post')
    renderSection()
    expect(await screen.findByText(/dev/)).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /Check dev updates/i })
    ).not.toBeInTheDocument()
    expect(post).not.toHaveBeenCalled()
    expect(get).toHaveBeenCalledWith('/api/catalog_sync/status')
  })

  test('disabled role shows only configuration status without a write action', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          role: 'disabled',
          source_ready: false,
          target_ready: false,
          management_ready: false,
          error: 'configuration_invalid',
        },
      },
    })
    const post = vi.spyOn(api, 'post')
    renderSection()
    expect(
      await screen.findByText('Catalog configuration is invalid.')
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Check dev updates' })
    ).not.toBeInTheDocument()
    expect(post).not.toHaveBeenCalled()
  })

  test('target preview needs an explicit user action', async () => {
    const get = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: url.endsWith('/status')
          ? {
              role: 'target',
              source_id: 'dev',
              target_id: 'prod-a',
              source_ready: false,
              target_ready: true,
              management_ready: true,
            }
          : { items: [], total: 0, offset: 0, limit: 20 },
      },
    }))
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: {
        success: true,
        data: {
          id: 'a'.repeat(64),
          kind: 'sync',
          source_id: 'dev',
          target_id: 'prod-a',
          source_exported_at: 1700000000,
          source_digest: `sha256:${'b'.repeat(64)}`,
          digest: 'c'.repeat(64),
          expires_at: Math.floor(Date.now() / 1000) + 600,
          resolution: { overwrite_keys: null, confirm_deletes: false },
          changes: [],
          executable: false,
        },
      },
    })
    renderSection()
    const preview = await screen.findByRole('button', {
      name: /Check dev updates/i,
    })
    expect(post).not.toHaveBeenCalled()
    await userEvent.click(preview)
    expect(post).toHaveBeenCalledWith('/api/catalog_sync/preview', {})
    expect(get).toHaveBeenCalled()
  })

  test('history pages by 20 and caps explicit API page requests at 100', async () => {
    const items = Array.from({ length: 21 }, (_, index) => ({
      id: `op-${index}`,
      plan_id: 'p',
      state: 'succeeded',
      revision: index + 1,
      created_at: 1700000000 + index,
      summary: {
        kind: 'sync',
        source_id: 'dev',
        target_id: 'prod-a',
        actor_user_id: 7,
        digest: 'a'.repeat(64),
        actions: { update: 1 },
      },
    }))
    const get = vi.spyOn(api, 'get').mockImplementation(async (url, config) => {
      if (url.endsWith('/status')) {
        return { data: { success: true, data: statusTarget } }
      }
      const offset = Number(config?.params?.offset ?? 0)
      const limit = Number(config?.params?.limit ?? 20)
      return {
        data: {
          success: true,
          data: {
            items: items.slice(offset, offset + limit),
            total: 21,
            offset,
            limit,
          },
        },
      }
    })
    renderSection()
    expect(await screen.findByText('Items 1–20 of 21')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(await screen.findByText('Items 21–21 of 21')).toBeInTheDocument()
    expect(get).toHaveBeenCalledWith(
      '/api/catalog_sync/history',
      expect.objectContaining({ params: { offset: 20, limit: 20 } })
    )
    await listCatalogSyncHistory(0, 150)
    expect(get).toHaveBeenCalledWith(
      '/api/catalog_sync/history',
      expect.objectContaining({ params: { offset: 0, limit: 100 } })
    )
  })

  test('preview groups all actions and shows raw field, currency, unit and long expression values', async () => {
    const expression =
      '<img src=x onerror=alert(1)> if (usage > 0) { return "CNY" + usage; }'.repeat(
        20
      )
    const priceKey = '{"model":"model-a","option":"ModelPrice","path":"/input"}'
    const changes = [
      change({
        kind: 'vendor',
        key: 'vendor-a',
        action: 'create',
        before: null,
        base: null,
      }),
      change(),
      change({
        kind: 'model_price',
        key: priceKey,
        action: 'delete',
        after: null,
        before: {
          kind: 'model_price',
          key: priceKey,
          value: '{"value":"0.5","billing_currency":"CNY","unit":"1M tokens"}',
        },
      }),
      change({
        kind: 'plugin_price',
        key: '{"model":"model-a","option":"PluginPrice","path":"/tool"}',
        action: 'conflict',
        after: {
          kind: 'plugin_price',
          key: priceKey,
          value: JSON.stringify({
            value: expression,
            billing_currency: 'CNY',
            unit: 'request',
          }),
        },
      }),
      change({
        kind: 'special_price',
        key: '{"option":"SpecialPrice","path":"/x"}',
        action: 'preserve',
        reason: 'target_only',
      }),
      change({
        kind: 'tool_price',
        key: '{"option":"ToolPrice","path":"/x"}',
        action: 'blocked',
        reason: 'active_reference',
      }),
    ]
    mockTargetRequests(plan(changes))
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Check dev updates' })
    )
    expect(await screen.findByText('vendor-a')).toBeInTheDocument()
    expect(
      screen.getByText('Target-only item remains unchanged')
    ).toBeInTheDocument()
    expect(screen.queryByText('target_only')).not.toBeInTheDocument()
    expect(screen.getByText(expression)).toBeInTheDocument()
    expect(document.querySelector('img[src="x"]')).not.toBeInTheDocument()
    expect(screen.getAllByText('CNY').length).toBeGreaterThan(0)
    expect(screen.getByText('1M tokens')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Review and confirm' })
    ).toBeDisabled()
    await userEvent.type(
      screen.getByRole('textbox', { name: 'Search models' }),
      'model-a'
    )
    expect(screen.queryByText('vendor-a')).not.toBeInTheDocument()
  })

  test('conflict unit is echoed once and deletion requires separate consent before final digest confirmation', async () => {
    const priceKey = '{"model":"model-a","option":"ModelPrice","path":"/x"}'
    const initial = plan([
      change({ action: 'conflict', reason: 'local_change' }),
      change({
        kind: 'model_price',
        key: priceKey,
        action: 'conflict',
        reason: 'confirmation_unit_conflict',
      }),
      change({
        kind: 'vendor',
        key: 'old-vendor',
        action: 'delete',
        after: null,
        confirmation_unit: 'vendor-unit',
      }),
    ])
    const final = plan(
      initial.changes.map((item) =>
        item.action === 'conflict' ? { ...item, action: 'update' } : item
      ),
      {
        digest: 'd'.repeat(64),
        resolution: {
          overwrite_keys: ['opaque-model-unit'],
          confirm_deletes: true,
        },
      }
    )
    const { post } = mockTargetRequests(initial, final)
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Check dev updates' })
    )
    const choice = await screen.findByRole('checkbox', {
      name: /Use dev for entire item/,
    })
    expect(
      screen.getAllByRole('checkbox', { name: /Use dev for entire item/ })
    ).toHaveLength(1)
    await userEvent.click(choice)
    await userEvent.click(
      screen.getByRole('button', { name: 'Review and confirm' })
    )
    expect(screen.getByRole('button', { name: 'Save choices' })).toBeDisabled()
    await userEvent.click(
      screen.getByRole('checkbox', { name: /I consent to delete/ })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save choices' }))
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith(
        `/api/catalog_sync/plans/${initial.id}/resolve`,
        {
          digest: initial.digest,
          overwrite_keys: ['opaque-model-unit'],
          confirm_deletes: true,
        }
      )
    )
    expect(await screen.findByText(final.digest)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Confirm sync' })).toBeEnabled()
  })

  test('no-op cannot be applied but ownership-only preview remains confirmable', async () => {
    const unchanged = plan([change({ action: 'unchanged' })], {
      executable: false,
    })
    const adopted = plan([change({ action: 'adopt' })])
    const { post } = mockTargetRequests(unchanged)
    let previewCount = 0
    post.mockImplementation(async () => ({
      data: { success: true, data: previewCount++ === 0 ? unchanged : adopted },
    }))
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Check dev updates' })
    )
    expect(
      await screen.findByRole('button', { name: 'Review and confirm' })
    ).toBeDisabled()
    await userEvent.click(
      screen.getByRole('button', { name: 'Check dev updates' })
    )
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Review and confirm' })
      ).toBeEnabled()
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Review and confirm' })
    )
    expect(screen.getByText('Ownership only')).toBeInTheDocument()
  })

  test('expired preview requires a new preview before confirmation', async () => {
    mockTargetRequests(
      plan([change()], { expires_at: Math.floor(Date.now() / 1000) - 1 })
    )
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Check dev updates' })
    )
    expect(await screen.findByText('Preview expired')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Review and confirm' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Check dev updates' })
    ).toBeEnabled()
  })

  test('pending apply sends once with final digest and disables a second write', async () => {
    const initial = plan([change({ action: 'adopt' })])
    const final = plan(initial.changes, { digest: 'd'.repeat(64) })
    const get = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: targetResponseData(url, 'catalog.sync.apply'),
      },
    }))
    const post = vi.spyOn(api, 'post').mockImplementation(async (url) => {
      if (url.endsWith('/preview')) {
        return { data: { success: true, data: initial } }
      }
      if (url.endsWith('/resolve')) {
        return { data: { success: true, data: final } }
      }
      if (url === '/api/verify') {
        return {
          data: {
            success: true,
            data: {
              proof_token: 'one-use-proof',
              scope: 'catalog.sync.apply',
              method: 'session',
              expires_at: Math.floor(Date.now() / 1000) + 300,
            },
          },
        }
      }
      return new Promise<never>(() => undefined)
    })
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Check dev updates' })
    )
    await userEvent.click(
      await screen.findByRole('button', { name: 'Review and confirm' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save choices' }))
    await userEvent.click(
      await screen.findByRole('button', { name: 'Confirm sync' })
    )
    await waitFor(() =>
      expect(
        post.mock.calls.some(([url]) => String(url).endsWith('/apply'))
      ).toBe(true)
    )
    const applyCall = post.mock.calls.find(([url]) =>
      String(url).endsWith('/apply')
    )
    expect(applyCall?.[1]).toMatchObject({
      plan_id: final.id,
      digest: final.digest,
    })
    expect(applyCall?.[2]).toMatchObject({
      headers: { 'X-Security-Proof': 'one-use-proof' },
      singleUseAuthorization: true,
    })
    expect(
      screen.getByRole('button', { name: 'Check dev updates' })
    ).toBeDisabled()
    expect(
      post.mock.calls.filter(([url]) => String(url).endsWith('/apply'))
    ).toHaveLength(1)
    expect(get).toHaveBeenCalledWith(
      '/api/verify/methods',
      expect.objectContaining({ params: { scope: 'catalog.sync.apply' } })
    )
    expect(post).toHaveBeenCalledWith(
      '/api/verify',
      expect.objectContaining({
        scope: 'catalog.sync.apply',
        context: expect.objectContaining({
          plan_digest: final.digest,
          target_id: final.target_id,
          kind: 'sync',
        }),
      }),
      expect.anything()
    )
  })

  test('disconnected apply keeps original binding and only queries the same receipt after 404', async () => {
    const initial = plan([change({ action: 'adopt' })])
    const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url.includes('/operations/')) {
        throw new Error('receipt not found yet')
      }
      return {
        data: {
          success: true,
          data: targetResponseData(url, 'catalog.sync.apply'),
        },
      }
    })
    const post = vi.spyOn(api, 'post').mockImplementation(async (url) => {
      if (url.endsWith('/preview') || url.endsWith('/resolve')) {
        return { data: { success: true, data: initial } }
      }
      if (url === '/api/verify') {
        return {
          data: {
            success: true,
            data: {
              proof_token: 'one-use-proof',
              scope: 'catalog.sync.apply',
              method: 'session',
              expires_at: Math.floor(Date.now() / 1000) + 300,
            },
          },
        }
      }
      throw new Error('network disconnected')
    })
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Check dev updates' })
    )
    await userEvent.click(
      await screen.findByRole('button', { name: 'Review and confirm' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save choices' }))
    await userEvent.click(
      await screen.findByRole('button', { name: 'Confirm sync' })
    )
    expect(await screen.findByText('Result unknown')).toBeInTheDocument()
    await waitFor(() =>
      expect(
        get.mock.calls.some(([url]) => String(url).includes('/operations/'))
      ).toBe(true)
    )
    const boundCall = get.mock.calls.find(([url]) =>
      String(url).includes('/operations/')
    )
    expect(boundCall?.[1]).toMatchObject({
      params: { plan_id: initial.id, digest: initial.digest },
    })
    await userEvent.click(
      screen.getByRole('button', { name: 'Check original result' })
    )
    expect(
      post.mock.calls.filter(([url]) => String(url).endsWith('/apply'))
    ).toHaveLength(1)
    expect(
      screen.getByRole('button', { name: 'Check dev updates' })
    ).toBeDisabled()
    const operationLabel = screen.getByText(/Operation ID:/).textContent
    cleanup()
    renderSection()
    expect(await screen.findByText(/Result unknown/)).toBeInTheDocument()
    expect(screen.getByText(/Operation ID:/)).toHaveTextContent(
      operationLabel ?? ''
    )
    expect(
      screen.getByRole('button', { name: 'Check dev updates' })
    ).toBeDisabled()
    expect(
      post.mock.calls.filter(([url]) => String(url).endsWith('/apply'))
    ).toHaveLength(1)
  })

  test('restore preview is a local inverse and requests a fresh restore-scoped proof', async () => {
    const restorePlan = plan([change({ action: 'adopt' })], {
      kind: 'restore',
      restore_operation_id: 'last-sync',
      source_exported_at: 1710000000,
      source_digest: `sha256:${'e'.repeat(64)}`,
    })
    const history = {
      items: [
        {
          id: 'last-sync',
          plan_id: 'p',
          state: 'succeeded',
          revision: 2,
          created_at: 1700000000,
          summary: {
            kind: 'sync',
            source_id: 'dev',
            target_id: 'prod-a',
            actor_user_id: 7,
            digest: 'd'.repeat(64),
            actions: { update: 1 },
          },
        },
      ],
      total: 1,
      offset: 0,
      limit: 20,
    }
    const get = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: targetResponseData(url, 'catalog.sync.restore', history),
      },
    }))
    const post = vi.spyOn(api, 'post').mockImplementation(async (url) => {
      if (url.endsWith('/restore-preview') || url.endsWith('/resolve')) {
        return { data: { success: true, data: restorePlan } }
      }
      if (url === '/api/verify') {
        return {
          data: {
            success: true,
            data: {
              proof_token: 'restore-proof',
              scope: 'catalog.sync.restore',
              method: 'session',
              expires_at: Math.floor(Date.now() / 1000) + 300,
            },
          },
        }
      }
      return new Promise<never>(() => undefined)
    })
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Preview restore' })
    )
    expect(
      await screen.findByText(/Inverse snapshot digest/)
    ).toHaveTextContent(restorePlan.source_digest)
    await userEvent.click(
      screen.getByRole('button', { name: 'Review and confirm' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save choices' }))
    await userEvent.click(
      await screen.findByRole('button', { name: 'Confirm restore' })
    )
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith(
        '/api/verify/methods',
        expect.objectContaining({ params: { scope: 'catalog.sync.restore' } })
      )
    )
    expect(post).toHaveBeenCalledWith(
      '/api/verify',
      expect.objectContaining({
        scope: 'catalog.sync.restore',
        context: expect.objectContaining({
          kind: 'restore',
          plan_digest: restorePlan.digest,
        }),
      }),
      expect.anything()
    )
  })

  test('confirmed receipt with null live operation stays distinct and does not unlock another mutation', async () => {
    const previewPlan = plan([change({ action: 'adopt' })])
    const saved = { state: 'committed_pending_publish', revision: 4 }
    const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url.includes('/operations/')) {
        return {
          data: {
            success: true,
            data: {
              receipt: { ...saved, operation_id: url.split('/').at(-1) },
              operation: null,
            },
          },
        }
      }
      return {
        data: {
          success: true,
          data: targetResponseData(url, 'catalog.sync.apply'),
        },
      }
    })
    const post = vi.spyOn(api, 'post').mockImplementation(async (url, body) => {
      if (url.endsWith('/preview') || url.endsWith('/resolve')) {
        return { data: { success: true, data: previewPlan } }
      }
      if (url === '/api/verify') {
        return {
          data: {
            success: true,
            data: {
              proof_token: 'one-use-proof',
              scope: 'catalog.sync.apply',
              method: 'session',
              expires_at: Math.floor(Date.now() / 1000) + 300,
            },
          },
        }
      }
      const pending = new axios.AxiosError('publication pending')
      Object.assign(pending, {
        response: {
          data: {
            code: 'CATALOG_PUBLICATION_PENDING',
            data: {
              ...saved,
              operation_id: (body as { operation_id: string }).operation_id,
            },
          },
        },
      })
      throw pending
    })
    renderSection()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Check dev updates' })
    )
    await userEvent.click(
      await screen.findByRole('button', { name: 'Review and confirm' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save choices' }))
    await userEvent.click(
      await screen.findByRole('button', { name: 'Confirm sync' })
    )
    expect(
      await screen.findByText(
        /Submission is confirmed; live publication state is unavailable/
      )
    ).toBeInTheDocument()
    expect(screen.getByText(/Immutable receipt/)).toHaveTextContent(
      'committed_pending_publish'
    )
    expect(
      screen.getByRole('button', { name: 'Check dev updates' })
    ).toBeDisabled()
    expect(
      get.mock.calls.some(([url]) => String(url).includes('/operations/'))
    ).toBe(true)
    expect(
      post.mock.calls.filter(([url]) => String(url).endsWith('/apply'))
    ).toHaveLength(1)
  })

  test('keyboard preview and language switching update labels and snapshot dates for all seven locales', async () => {
    const resources = { en, zhCN: zh, zhTW, fr, ru, ja, vi: viLocale }
    for (const [code, resource] of Object.entries(resources)) {
      i18next.addResourceBundle(
        code,
        'translation',
        resource.translation,
        true,
        true
      )
    }
    const previewPlan = plan([change({ action: 'adopt' })])
    const { post } = mockTargetRequests(previewPlan)
    renderSection()
    const preview = await screen.findByRole('button', {
      name: 'Check dev updates',
    })
    await userEvent.tab()
    expect(preview).toHaveFocus()
    await userEvent.keyboard('{Enter}')
    expect(await screen.findByText(/Dev snapshot digest/)).toHaveTextContent(
      previewPlan.source_digest
    )
    expect(post).toHaveBeenCalledWith('/api/catalog_sync/preview', {})
    for (const [code, resource] of Object.entries(resources)) {
      await i18next.changeLanguage(code)
      expect(
        screen.getByRole('button', {
          name: resource.translation['Check dev updates'],
        })
      ).toBeInTheDocument()
      let locale = code
      if (code === 'zhCN') locale = 'zh-CN'
      if (code === 'zhTW') locale = 'zh-TW'
      const date = new Intl.DateTimeFormat(locale, {
        dateStyle: 'medium',
        timeStyle: 'short',
      }).format(new Date(previewPlan.source_exported_at * 1000))
      expect(
        screen.getByText(new RegExp(resource.translation['Dev snapshot time']))
      ).toHaveTextContent(date)
    }
  })
})
