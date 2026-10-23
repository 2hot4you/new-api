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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState, type ReactNode } from 'react'
import { toast, Toaster } from 'sonner'
import { afterEach, expect, it, vi } from 'vitest'

import type {
  ModelPricingChange,
  ModelPricingConfig,
} from '@/features/model-pricing/api'
import { pricingOptions } from '@/features/model-pricing/pricing'
import { SettingsPageProvider } from '@/features/system-settings/components/settings-page-context'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { RatioSettingsCard } from '../ratio-settings-card'

const protectedModel = 'doubao-seedance-2-0-260128'
const guidance =
  'This model is priced in the Seedance 2.0 tab. Update its price matrix there; the billing expression is generated automatically.'
const clients: QueryClient[] = []

function PageActionsFixture({ children }: { children: ReactNode }) {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)
  return (
    <>
      <header>
        <div ref={setActionsContainer} />
      </header>
      <SettingsPageProvider actionsContainer={actionsContainer}>
        {children}
      </SettingsPageProvider>
    </>
  )
}

afterEach(() => {
  toast.dismiss()
  clients.forEach((client) => client.clear())
  clients.length = 0
  useAuthStore.getState().auth.setUser(null)
})

async function renderSettings(
  ordinary = true,
  extraEntries: ModelPricingConfig['entries'] = []
) {
  const user = userEvent.setup()
  const prices: Record<string, number> = {
    [protectedModel]: 10,
    ...(ordinary ? { ordinary: 1 } : {}),
  }
  let snapshot: ModelPricingConfig = {
    empty_version: 'empty',
    options: pricingOptions({ ModelPrice: JSON.stringify(prices) }),
    entries: Object.entries(prices).map(([model_name, price]) => ({
      model_name,
      version: `${model_name}-v1`,
      configured: { ModelPrice: price },
      effective: { ModelPrice: price },
      billing_currency: 'CNY',
      has_metadata: true,
    })),
  }
  for (const entry of extraEntries) {
    snapshot.entries.push(entry)
    if (entry.effective.ModelPrice !== undefined) {
      prices[entry.model_name] = Number(entry.effective.ModelPrice)
    }
  }
  snapshot.options = pricingOptions({ ModelPrice: JSON.stringify(prices) })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url === '/api/option/model_pricing' ? snapshot : [],
      vendors: [],
    },
  }))
  const patch = vi
    .spyOn(api, 'patch')
    .mockImplementation(async (_url, request) => {
      const changes = (request as { changes: ModelPricingChange[] }).changes
      for (const change of changes) {
        if (change.reset || change.remove) {
          delete prices[change.model_name]
        } else prices[change.model_name] = Number(change.pricing.ModelPrice)
      }
      snapshot = {
        ...snapshot,
        options: pricingOptions({ ModelPrice: JSON.stringify(prices) }),
      }
      return { data: { success: true } }
    })
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'administrator', role: 100 })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  render(
    <QueryClientProvider client={client}>
      <PageActionsFixture>
        <RatioSettingsCard
          modelDefaults={{
            ModelPrice: '{}',
            ModelRatio: '{}',
            CacheRatio: '{}',
            CreateCacheRatio: '{}',
            CompletionRatio: '{}',
            ImageRatio: '{}',
            AudioRatio: '{}',
            AudioCompletionRatio: '{}',
            BillingMode: '{}',
            BillingExpr: '{}',
            PluginBillingExpr: '{}',
            ExposeRatioEnabled: false,
          }}
          groupDefaults={{
            GroupRatio: '{}',
            TopupGroupRatio: '{}',
            UserUsableGroups: '{}',
            GroupGroupRatio: '{}',
            AutoGroups: '[]',
            MaxTokenAutoGroups: 5,
            DefaultUseAutoGroup: false,
            GroupSpecialUsableGroup: '{}',
          }}
          toolPricesDefault='{}'
          visibleTabs={['models']}
        />
        <Toaster />
      </PageActionsFixture>
    </QueryClientProvider>
  )
  await user.click(
    await screen.findByRole('button', { name: 'Switch to JSON' })
  )
  return { user, patch }
}

it('saves ordinary JSON edits in a mixed batch and explicitly routes unsaved dedicated edits', async () => {
  const { user, patch } = await renderSettings()
  fireEvent.input(
    screen.getByRole('textbox', { name: 'Model fixed pricing' }),
    {
      target: { value: JSON.stringify({ [protectedModel]: 99, ordinary: 2 }) },
    }
  )
  await user.click(screen.getByRole('button', { name: 'Save model prices' }))
  await waitFor(() =>
    expect(patch).toHaveBeenCalledWith('/api/option/model_pricing', {
      changes: [
        {
          model_name: 'ordinary',
          expected_version: 'ordinary-v1',
          pricing: { ModelPrice: 2 },
          billing_currency: 'CNY',
        },
      ],
    })
  )
  expect(await screen.findByText(guidance)).toBeVisible()
  expect(screen.queryByText('Model pricing saved')).not.toBeInTheDocument()
  await waitFor(() =>
    expect(
      JSON.parse(
        (
          screen.getByRole('textbox', {
            name: 'Model fixed pricing',
          }) as HTMLTextAreaElement
        ).value
      )
    ).toEqual({ [protectedModel]: 10, ordinary: 2 })
  )
})

it('does not mutate or report success for protected-only JSON edits', async () => {
  const { user, patch } = await renderSettings(false)
  fireEvent.input(
    screen.getByRole('textbox', { name: 'Model fixed pricing' }),
    {
      target: { value: JSON.stringify({ [protectedModel]: 99 }) },
    }
  )
  await user.click(screen.getByRole('button', { name: 'Save model prices' }))
  expect(await screen.findByText(guidance)).toBeVisible()
  expect(patch).not.toHaveBeenCalled()
  expect(screen.queryByText('Model pricing saved')).not.toBeInTheDocument()
})

it.each([true, false])(
  'preserves dedicated pricing when resetting with ordinary entries=%s',
  async (ordinary) => {
    const { user, patch } = await renderSettings(ordinary)
    await user.click(screen.getByRole('button', { name: 'Reset prices' }))
    await user.click(await screen.findByRole('button', { name: /^Reset$/ }))
    if (ordinary) {
      await waitFor(() =>
        expect(patch).toHaveBeenCalledWith('/api/option/model_pricing', {
          changes: [
            {
              model_name: 'ordinary',
              expected_version: 'ordinary-v1',
              pricing: {},
              billing_currency: 'CNY',
              reset: true,
            },
          ],
        })
      )
      expect(
        await screen.findByText('Model prices reset successfully')
      ).toBeVisible()
    } else {
      expect(
        await screen.findByText('No model price changes to save')
      ).toBeVisible()
      expect(patch).not.toHaveBeenCalled()
      expect(
        screen.queryByText('Model prices reset successfully')
      ).not.toBeInTheDocument()
    }
  }
)

it.each([false, true])(
  'rejects missing metadata JSON atomically (mixed=%s) and preserves the draft',
  async (mixed) => {
    const { user, patch } = await renderSettings(mixed, [
      {
        model_name: 'missing',
        version: 'm1',
        configured: { ModelPrice: 1 },
        effective: { ModelPrice: 1 },
        has_metadata: false,
      },
    ])
    const value = JSON.stringify({
      [protectedModel]: 10,
      missing: 2,
      ...(mixed ? { ordinary: 2 } : {}),
    })
    const input = screen.getByRole('textbox', { name: 'Model fixed pricing' })
    fireEvent.input(input, { target: { value } })
    const error = vi.spyOn(toast, 'error')
    const success = vi.spyOn(toast, 'success')
    const put = vi.spyOn(api, 'put')
    await user.click(screen.getByRole('switch', { name: 'Expose ratio API' }))
    await user.click(screen.getByRole('button', { name: 'Save model prices' }))
    await waitFor(() =>
      expect(error).toHaveBeenCalledWith(expect.stringContaining('missing'))
    )
    expect(patch).not.toHaveBeenCalled()
    expect(put).not.toHaveBeenCalled()
    expect(success).not.toHaveBeenCalled()
    expect(input).toHaveValue(value)
  }
)

it('rejects the entire reset when a configured override lacks metadata', async () => {
  const { user, patch } = await renderSettings(true, [
    {
      model_name: 'missing',
      version: 'm1',
      configured: { ModelPrice: 1 },
      effective: { ModelPrice: 1 },
      has_metadata: false,
    },
  ])
  const error = vi.spyOn(toast, 'error')
  await user.click(screen.getByRole('button', { name: 'Reset prices' }))
  await user.click(await screen.findByRole('button', { name: /^Reset$/ }))
  await waitFor(() =>
    expect(error).toHaveBeenCalledWith(expect.stringContaining('missing'))
  )
  expect(patch).not.toHaveBeenCalled()
})

it('does not reset built-in-only rows', async () => {
  const { user, patch } = await renderSettings(false, [
    {
      model_name: 'builtin',
      version: 'b1',
      configured: {},
      effective: { ModelPrice: 1 },
      has_metadata: true,
      billing_currency: 'USD',
    },
  ])
  await user.click(screen.getByRole('button', { name: 'Reset prices' }))
  await user.click(await screen.findByRole('button', { name: /^Reset$/ }))
  expect(
    await screen.findByText('No model price changes to save')
  ).toBeVisible()
  expect(patch).not.toHaveBeenCalled()
})

it('keeps a save action after deleting the selected last ordinary override', async () => {
  const { user, patch } = await renderSettings()
  await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))
  await user.click(await screen.findByRole('button', { name: 'Edit' }))
  await user.click(screen.getByRole('button', { name: 'Open menu' }))
  await user.click(
    await screen.findByRole('menuitem', { name: /Remove pricing override/ })
  )
  expect(
    screen.queryByRole('region', { name: 'Edit model pricing' })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Save model prices' }))
  await waitFor(() =>
    expect(patch).toHaveBeenCalledWith('/api/option/model_pricing', {
      changes: [
        {
          model_name: 'ordinary',
          expected_version: 'ordinary-v1',
          pricing: {},
          billing_currency: 'CNY',
        },
      ],
    })
  )
})

it('preserves the local deletion after a failed save', async () => {
  const { user, patch } = await renderSettings()
  patch.mockRejectedValue(new Error('Save failed'))
  await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))
  await user.click(await screen.findByRole('button', { name: 'Edit' }))
  await user.click(screen.getByRole('button', { name: 'Open menu' }))
  await user.click(
    await screen.findByRole('menuitem', { name: /Remove pricing override/ })
  )
  await user.click(screen.getByRole('button', { name: 'Save model prices' }))
  expect(await screen.findByText('Save failed')).toBeVisible()
  expect(
    screen.queryByRole('row', { name: /ordinary/ })
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Save model prices' }))
  await waitFor(() => expect(patch).toHaveBeenCalledTimes(2))
  expect(patch.mock.calls[1][1]).toEqual(patch.mock.calls[0][1])
})

it.each([false, true])(
  'removes both orphan qwen overrides through the existing controls (save fails=%s)',
  async (saveFails) => {
    const { user, patch } = await renderSettings(false, [
      {
        model_name: 'qwen3.7-max',
        version: 'qwen37-original',
        configured: { ModelPrice: 1 },
        effective: { ModelPrice: 1 },
        has_metadata: false,
      },
      {
        model_name: 'qwen3.8-max',
        version: 'qwen38-original',
        configured: { ModelPrice: 2 },
        effective: { ModelPrice: 2 },
        has_metadata: false,
      },
    ])
    if (saveFails) patch.mockRejectedValue(new Error('Save failed'))
    const put = vi.spyOn(api, 'put')
    await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))
    for (const name of ['qwen3.7-max', 'qwen3.8-max']) {
      const row = await screen.findByRole('row', { name: new RegExp(name) })
      await user.click(within(row).getByRole('button', { name: 'Edit' }))
      await user.click(within(row).getByRole('button', { name: 'Open menu' }))
      await user.click(
        await screen.findByRole('menuitem', { name: /Remove pricing override/ })
      )
    }
    await user.click(screen.getByRole('button', { name: 'Save model prices' }))
    const request = {
      changes: [
        {
          model_name: 'qwen3.7-max',
          expected_version: 'qwen37-original',
          pricing: {},
          remove: true,
        },
        {
          model_name: 'qwen3.8-max',
          expected_version: 'qwen38-original',
          pricing: {},
          remove: true,
        },
      ],
    }
    await waitFor(() =>
      expect(patch).toHaveBeenCalledWith('/api/option/model_pricing', request)
    )
    expect(put).not.toHaveBeenCalled()
    expect(
      screen.queryByRole('row', { name: /qwen3\.[78]-max/ })
    ).not.toBeInTheDocument()
    if (saveFails) {
      expect(await screen.findByText('Save failed')).toBeVisible()
      expect(screen.queryByText('Model pricing saved')).not.toBeInTheDocument()
      await user.click(
        screen.getByRole('button', { name: 'Save model prices' })
      )
      await waitFor(() => expect(patch).toHaveBeenCalledTimes(2))
      expect(patch.mock.calls[1][1]).toEqual(request)
    } else {
      expect(await screen.findByText('Model pricing saved')).toBeVisible()
    }
  }
)

it('rejects an orphan removal mixed with a metadata-less edit before any write', async () => {
  const { user, patch } = await renderSettings(false, [
    {
      model_name: 'qwen3.7-max',
      version: 'qwen37-original',
      configured: { ModelPrice: 1 },
      effective: { ModelPrice: 1 },
      has_metadata: false,
    },
    {
      model_name: 'invalid-edit',
      version: 'invalid-original',
      configured: { ModelPrice: 2 },
      effective: { ModelPrice: 2 },
      has_metadata: false,
    },
  ])
  const value = JSON.stringify({ [protectedModel]: 10, 'invalid-edit': 3 })
  const input = screen.getByRole('textbox', { name: 'Model fixed pricing' })
  fireEvent.input(input, { target: { value } })
  const put = vi.spyOn(api, 'put')
  const error = vi.spyOn(toast, 'error')
  await user.click(screen.getByRole('switch', { name: 'Expose ratio API' }))
  await user.click(screen.getByRole('button', { name: 'Save model prices' }))
  await waitFor(() =>
    expect(error).toHaveBeenCalledWith(expect.stringContaining('invalid-edit'))
  )
  expect(patch).not.toHaveBeenCalled()
  expect(put).not.toHaveBeenCalled()
  expect(input).toHaveValue(value)
  expect(screen.queryByText('Model pricing saved')).not.toBeInTheDocument()
})

it('prices an existing unconfigured exact metadata row through Add', async () => {
  const { user, patch } = await renderSettings(true, [
    {
      model_name: 'new',
      version: 'new-metadata-v1',
      configured: {},
      effective: {},
      has_metadata: true,
      billing_currency: 'CNY',
    },
  ])
  await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))
  await user.click(screen.getAllByRole('button', { name: 'Add model' })[0])
  await user.type(screen.getByRole('textbox', { name: 'Model name' }), ' new ')
  await user.click(screen.getByRole('button', { name: 'Save model prices' }))
  await waitFor(() =>
    expect(patch).toHaveBeenCalledWith('/api/option/model_pricing', {
      changes: [
        expect.objectContaining({
          model_name: 'new',
          expected_version: 'new-metadata-v1',
          billing_currency: 'CNY',
        }),
      ],
    })
  )
})

it('does not remove a built-in-only row or submit it as a removal', async () => {
  const { user, patch } = await renderSettings(false, [
    {
      model_name: 'builtin',
      version: 'b1',
      configured: {},
      effective: { ModelPrice: 1 },
      has_metadata: true,
      billing_currency: 'USD',
    },
  ])
  await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))
  await user.click(screen.getByRole('button', { name: 'Open menu' }))
  await user.click(
    await screen.findByRole('menuitem', { name: /Remove pricing override/ })
  )
  expect(
    await screen.findByText(
      'This model uses built-in defaults and has no pricing override to remove.'
    )
  ).toBeVisible()
  expect(screen.getByRole('row', { name: /builtin/ })).toBeVisible()
  expect(patch).not.toHaveBeenCalled()
})

it('does not guess a source currency in an existing metadata row editor', async () => {
  const { user, patch } = await renderSettings(false, [
    {
      model_name: 'unknown-currency',
      version: 'u1',
      configured: { ModelPrice: 1 },
      effective: { ModelPrice: 1 },
      has_metadata: true,
    },
  ])
  await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))
  await user.click(await screen.findByRole('button', { name: 'Edit' }))
  expect(
    screen.getByRole('button', { name: 'Save model prices' })
  ).toBeDisabled()
  expect(patch).not.toHaveBeenCalled()
})
