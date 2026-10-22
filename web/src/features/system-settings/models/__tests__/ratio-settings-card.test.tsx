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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast, Toaster } from 'sonner'
import { afterEach, expect, it, vi } from 'vitest'

import type {
  ModelPricingChange,
  ModelPricingConfig,
} from '@/features/model-pricing/api'
import { pricingOptions } from '@/features/model-pricing/pricing'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { RatioSettingsCard } from '../ratio-settings-card'

const protectedModel = 'doubao-seedance-2-0-260128'
const guidance =
  'This model is priced in the Seedance 2.0 tab. Update its price matrix there; the billing expression is generated automatically.'
const clients: QueryClient[] = []

afterEach(() => {
  toast.dismiss()
  clients.forEach((client) => client.clear())
  clients.length = 0
  useAuthStore.getState().auth.setUser(null)
})

async function renderSettings(ordinary = true) {
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
        if (change.reset) delete prices[change.model_name]
        else prices[change.model_name] = Number(change.pricing.ModelPrice)
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
