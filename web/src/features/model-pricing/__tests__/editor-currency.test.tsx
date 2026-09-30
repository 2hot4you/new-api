/* Copyright (C) 2023-2026 QuantumNous */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef } from 'react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import {
  ModelPricingEditorPanel,
  type ModelPricingEditorPanelHandle,
  type ModelRatioData,
} from '@/features/system-settings/models/model-pricing-sheet'
import { api } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { previewModelPricing } from '../api'
import { formatPricingAmount, getSourcePreviewCurrency } from '../currency'

const clients: QueryClient[] = []

beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7,
    },
  })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [], vendors: [] },
  })
})

afterEach(() => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  vi.restoreAllMocks()
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
})

function renderEditor(data: Partial<ModelRatioData> = {}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  const ref = createRef<ModelPricingEditorPanelHandle>()
  const dirty = vi.fn()
  render(
    <QueryClientProvider client={client}>
      <ModelPricingEditorPanel
        ref={ref}
        editData={{
          name: 'currency-model',
          billingMode: 'tiered_expr',
          billingExpr: 'tier("base", p * 2 + c * 4)',
          billingCurrency: 'USD',
          hasMetadata: true,
          ...data,
        }}
        onDirtyChange={dirty}
      />
    </QueryClientProvider>
  )
  return { ref, dirty }
}

async function commit(
  ref: React.RefObject<ModelPricingEditorPanelHandle | null>
) {
  let result: ModelRatioData | null = null
  await act(async () => {
    result = (await ref.current?.commitDraft()) ?? null
  })
  return result
}

async function selectSourceCurrency(label: string) {
  const user = userEvent.setup()
  await user.click(
    screen.getByRole('combobox', { name: 'Source pricing currency' })
  )
  await user.click(await screen.findByRole('option', { name: label }))
}

it('reads preview prices and billing metadata from the common data envelope', async () => {
  const effective = { ModelRatio: 0, CompletionRatio: 2 }
  const billingDetails = { audio_input_price: 1 }
  vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: {
        effective,
        cache_write_mode: 'none',
        billing_details: billingDetails,
      },
    },
  })
  await expect(
    previewModelPricing({ model_name: 'gemini-2.5-flash', pricing: effective })
  ).resolves.toEqual({ effective, cacheWriteMode: 'none', billingDetails })
})

it('loads CNY as the controlled source currency without rewriting coefficients', async () => {
  const editor = renderEditor({ billingCurrency: 'CNY' })
  expect(
    screen.getByRole('combobox', { name: 'Source pricing currency' })
  ).toHaveTextContent('Chinese yuan (CNY)')
  expect(
    screen.getAllByRole('textbox', { name: 'Input price' })[0]
  ).toHaveValue('2')
  expect(await commit(editor.ref)).toMatchObject({
    billingCurrency: 'CNY',
    billingExpr: 'tier("base", p * 2 + c * 4)',
  })
})

it('marks a currency-only change dirty and preserves the raw expression', async () => {
  const editor = renderEditor()
  editor.dirty.mockClear()
  await selectSourceCurrency('Chinese yuan (CNY)')
  await waitFor(() => expect(editor.dirty).toHaveBeenLastCalledWith(true))
  expect(await commit(editor.ref)).toMatchObject({
    billingCurrency: 'CNY',
    billingExpr: 'tier("base", p * 2 + c * 4)',
  })
})

it('normalizes source prices before the CNY site preview', () => {
  const config = useSystemConfigStore.getState().config.currency
  expect(formatPricingAmount(5, getSourcePreviewCurrency('USD', config))).toBe(
    '¥35'
  )
  expect(formatPricingAmount(2, getSourcePreviewCurrency('CNY', config))).toBe(
    '¥2'
  )
})

it('keeps legacy pricing in USD compatibility mode', () => {
  renderEditor({
    billingMode: 'per-request',
    billingExpr: '',
    price: '2',
    billingCurrency: 'CNY',
  })
  expect(
    screen.getByRole('combobox', { name: 'Source pricing currency' })
  ).toBeDisabled()
  expect(
    screen.getByText(
      'Legacy ratio and fixed-price billing remains USD-compatible.'
    )
  ).toBeVisible()
  expect(screen.getByRole('textbox', { name: 'Fixed price' })).toHaveValue('2')
})

it('explains source currency semantics and restores focus', async () => {
  renderEditor()
  const user = userEvent.setup()
  const help = screen.getByRole('button', {
    name: 'About source pricing currency',
  })
  help.focus()
  await user.keyboard('{Enter}')
  const dialog = await screen.findByRole('dialog', {
    name: 'About source pricing currency',
  })
  expect(
    within(dialog).getByText(/Expression coefficients are stored exactly/)
  ).toBeVisible()
  await user.keyboard('{Escape}')
  await waitFor(() => expect(help).toHaveFocus())
})
