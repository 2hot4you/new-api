/* Copyright (C) 2023-2026 QuantumNous */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef } from 'react'
import { afterEach, expect, it, vi } from 'vitest'

import {
  ModelPricingEditorPanel,
  ModelPricingSheet,
  type ModelPricingEditorPanelHandle,
} from '@/features/system-settings/models/model-pricing-sheet'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import type { ModelPricingEntry } from '../api'
import { ModelPricingPanel } from '../model-pricing-panel'

const clients: QueryClient[] = []
afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  useAuthStore.getState().auth.setUser(null)
  vi.restoreAllMocks()
})

function setup() {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [], vendors: [] },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  return client
}

it.each(['desktop', 'mobile'] as const)(
  'shows missing metadata, not fake pending, in the %s editor',
  async (surface) => {
    const client = setup()
    const save = vi.fn()
    const props = {
      editData: {
        name: 'missing',
        hasMetadata: false,
        billingMode: 'tiered_expr' as const,
        billingExpr: 'p * 2',
      },
      onSave: save,
      isSaving: false,
    }
    render(
      <QueryClientProvider client={client}>
        {surface === 'mobile' ? (
          <ModelPricingSheet {...props} open onOpenChange={() => {}} />
        ) : (
          <ModelPricingEditorPanel {...props} />
        )}
      </QueryClientProvider>
    )
    expect(
      screen.getByRole('button', { name: 'Save model prices' })
    ).toBeDisabled()
    expect(
      screen.queryByRole('button', { name: 'Saving...' })
    ).not.toBeInTheDocument()
    expect(
      screen
        .getByText(/Pricing changes require exact model metadata/)
        .closest('[role="alert"]')
    ).toHaveTextContent('missing')
    expect(
      screen.getByRole('link', { name: 'Manage model metadata' })
    ).toHaveAttribute('target', '_blank')
    expect(save).not.toHaveBeenCalled()
  }
)

it('does not allow embedded save or reset without exact metadata', async () => {
  const client = setup()
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'administrator', role: 100 })
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/option/model_pricing'
          ? {
              entries: [
                {
                  model_name: 'missing',
                  version: 'v1',
                  has_metadata: false,
                  configured: { ModelPrice: 1 },
                  effective: { ModelPrice: 1 },
                },
              ],
              options: {},
              empty_version: 'empty',
            }
          : [],
      vendors: [],
    },
  }))
  render(
    <QueryClientProvider client={client}>
      <ModelPricingPanel modelName='missing' />
    </QueryClientProvider>
  )
  expect(
    await screen.findByRole('button', { name: 'Save model prices' })
  ).toBeDisabled()
  expect(
    screen.queryByRole('button', { name: 'Saving...' })
  ).not.toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Restore default pricing' })
  ).toBeDisabled()
  expect(
    screen.getByRole('link', { name: 'Manage model metadata' })
  ).toHaveAttribute('href', '/models/metadata')
})

it('checks a trimmed Add name against existing exact metadata and preserves unknown-name drafts', async () => {
  const client = setup()
  const ref = createRef<ModelPricingEditorPanelHandle>()
  const metadataEntries: ModelPricingEntry[] = [
    {
      model_name: 'known',
      has_metadata: true,
      billing_currency: 'CNY',
      version: 'metadata-v1',
      configured: {},
      effective: {},
    },
  ]
  render(
    <QueryClientProvider client={client}>
      <ModelPricingEditorPanel
        ref={ref}
        metadataEntries={metadataEntries}
        onSave={() => {}}
      />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  const name = screen.getByRole('textbox', { name: 'Model name' })
  await user.type(name, 'unknown')
  expect(
    screen.getByRole('button', { name: 'Save model prices' })
  ).toBeDisabled()
  let draft: unknown
  await act(async () => {
    draft = await ref.current?.commitDraft()
  })
  expect(draft).toBeNull()
  expect(name).toHaveValue('unknown')
  await user.clear(name)
  await user.type(name, ' known ')
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Save model prices' })
    ).toBeEnabled()
  )
  await act(async () => {
    draft = await ref.current?.commitDraft()
  })
  expect(draft).toMatchObject({
    name: 'known',
    billingCurrency: 'CNY',
    hasMetadata: true,
  })
})
