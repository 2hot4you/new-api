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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import type { PricingModel } from '@/features/pricing/types'

import { ProductQuotation } from '../../index'

const mocks = vi.hoisted(() => ({
  download: vi.fn(),
  pricing: {
    models: [] as PricingModel[],
    vendors: [{ id: 10, name: 'Provider Alpha' }],
    groupRatio: { default: 1, vip: 1.5 },
    usableGroup: {
      default: { desc: 'Default', ratio: 1 },
      vip: { desc: 'VIP', ratio: 1.5 },
    },
    endpointMap: {},
    autoGroups: [] as string[],
    pricingVersion: 'pricing-v3',
    dataUpdatedAt: Date.parse('2026-09-08T10:00:00.000Z'),
    isLoading: false,
    error: null as Error | null,
    refetch: vi.fn(async () => ({ isSuccess: true })),
    priceRate: 1,
    usdExchangeRate: 7,
  },
}))

vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => mocks.pricing,
}))

vi.mock('../../lib/download-html', () => ({
  downloadQuotationHtml: mocks.download,
}))

function pricingModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'alpha-chat',
    display_name: 'Alpha Chat',
    vendor_id: 10,
    quota_type: 0,
    model_ratio: 2,
    completion_ratio: 3,
    cache_ratio: 0.25,
    enable_groups: ['default', 'vip'],
    ...overrides,
  }
}

function setPricing(overrides: Partial<typeof mocks.pricing> = {}): void {
  Object.assign(mocks.pricing, {
    models: [pricingModel()],
    vendors: [{ id: 10, name: 'Provider Alpha' }],
    groupRatio: { default: 1, vip: 1.5 },
    usableGroup: {
      default: { desc: 'Default', ratio: 1 },
      vip: { desc: 'VIP', ratio: 1.5 },
    },
    pricingVersion: 'pricing-v3',
    dataUpdatedAt: Date.parse('2026-09-08T10:00:00.000Z'),
    isLoading: false,
    error: null,
    refetch: vi.fn(async () => ({ isSuccess: true })),
    priceRate: 1,
    usdExchangeRate: 7,
    ...overrides,
  })
}

async function fillValidQuotation(): Promise<void> {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('Quotation title'), 'Annual AI quote')
  await user.type(screen.getByLabelText('Global discount (zhe)'), '8')
  await user.click(screen.getByRole('checkbox', { name: 'Select alpha-chat' }))
}

beforeEach(() => {
  localStorage.clear()
  mocks.download.mockReset()
  setPricing()
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: vi.fn(async () => undefined) },
  })
})

afterEach(() => {
  localStorage.clear()
  return i18next.changeLanguage('en')
})

describe('product quotation workspace states', () => {
  test('renders loading, fetch failure with retry, and empty pricing states', async () => {
    const user = userEvent.setup()
    setPricing({ models: [], isLoading: true })
    const view = render(<ProductQuotation />)
    expect(
      screen.getByRole('status', { name: 'Loading pricing' })
    ).toBeInTheDocument()

    setPricing({ isLoading: false, error: new Error('offline') })
    view.rerender(<ProductQuotation />)
    expect(screen.getByText('Unable to load pricing')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Retry pricing' }))
    expect(mocks.pricing.refetch).toHaveBeenCalledTimes(1)

    setPricing({ error: null, models: [] })
    view.rerender(<ProductQuotation />)
    expect(screen.getByText('No pricing models available')).toBeInTheDocument()
  })

  test('keeps a local draft through manual price refresh and can clear it', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)
    await user.type(screen.getByLabelText('Quotation title'), 'Retained draft')
    await user.type(screen.getByLabelText('Customer'), 'Browser only customer')
    await user.click(
      screen.getByRole('checkbox', { name: 'Select alpha-chat' })
    )

    await user.click(screen.getByRole('button', { name: 'Refresh pricing' }))
    await waitFor(() => expect(mocks.pricing.refetch).toHaveBeenCalledTimes(1))
    expect(screen.getByLabelText('Quotation title')).toHaveValue(
      'Retained draft'
    )
    expect(
      screen.getByRole('checkbox', { name: 'Select alpha-chat' })
    ).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByText('Pricing refreshed')).toBeInTheDocument()
    expect(screen.getByText(/Last successful pricing fetch/)).toHaveTextContent(
      '2026'
    )

    await user.click(screen.getByRole('button', { name: 'Clear draft' }))
    expect(screen.getByLabelText('Quotation title')).toHaveValue('')
    expect(screen.getByLabelText('Customer')).toHaveValue('')
  })

  test('restores a valid versioned browser draft and blocks pending dynamic prices', () => {
    setPricing({
      models: [
        pricingModel({
          billing_mode: 'tiered_expr',
          billing_expr: 'price depends on an unsupported runtime rule',
        }),
      ],
    })
    localStorage.setItem(
      'new-api:product-quotation:draft:v1',
      JSON.stringify({
        version: 1,
        draft: {
          title: 'Restored quotation',
          customer: 'Restored customer',
          quotedBy: '',
          quoteDate: '2026-09-08',
          globalDiscount: 8,
          priceBasis: { type: 'raw' },
          selectedModelIds: ['alpha-chat'],
          providerOverrides: {},
        },
      })
    )

    render(<ProductQuotation />)

    expect(screen.getByLabelText('Quotation title')).toHaveValue(
      'Restored quotation'
    )
    expect(screen.getByText('Needs confirmation')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Export HTML quotation' })
    ).toBeDisabled()
  })

  test('guards export until the snapshot is valid and exports the live preview snapshot', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)

    const exportButton = screen.getByRole('button', {
      name: 'Export HTML quotation',
    })
    expect(exportButton).toBeDisabled()
    expect(screen.getByText('Select at least one model.')).toBeInTheDocument()

    await fillValidQuotation()
    expect(exportButton).toBeEnabled()
    await user.click(exportButton)

    expect(mocks.download).toHaveBeenCalledTimes(1)
    expect(mocks.download.mock.calls[0]?.[0]).toMatchObject({
      title: 'Annual AI quote',
      globalDiscount: 8,
      pricingVersion: 'pricing-v3',
    })
  })

  test('uses provider discounts over the global discount and renders every price column', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)
    await fillValidQuotation()
    await user.type(screen.getByLabelText('Provider Alpha discount (zhe)'), '5')

    expect(screen.getByText('5 zhe')).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Catalog price' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Basis price' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Quote price' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Currency' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Unit' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Condition' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Status' })
    ).toBeInTheDocument()
    expect(screen.getByText('$2')).toBeInTheDocument()
  })

  test('marks an invalid provider override instead of presenting its global fallback as valid', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)
    await fillValidQuotation()
    await user.type(
      screen.getByLabelText('Provider Alpha discount (zhe)'),
      '12'
    )

    expect(screen.getByText('Invalid provider discount')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Export HTML quotation' })
    ).toBeDisabled()
  })

  test('copies a model ID with inline accessible feedback', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)
    await fillValidQuotation()
    const writeText = vi.fn(async () => undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    await user.click(
      screen.getByRole('button', { name: 'Copy model ID alpha-chat' })
    )

    expect(writeText).toHaveBeenCalledWith('alpha-chat')
    expect(await screen.findByText('Model ID copied')).toBeInTheDocument()
  })

  test('marks a retained selection unavailable when the selected group changes', async () => {
    setPricing({
      models: [pricingModel({ enable_groups: ['default'] })],
    })
    render(<ProductQuotation />)
    await fillValidQuotation()

    fireEvent.change(screen.getByLabelText('Price basis'), {
      target: { value: 'group:vip' },
    })

    expect(
      screen.getAllByText('Unavailable for selected group').length
    ).toBeGreaterThan(0)
    expect(
      screen.getByRole('button', { name: 'Export HTML quotation' })
    ).toBeDisabled()
  })

  test('translates model-specific validation through stable interpolation keys', async () => {
    i18next.addResourceBundle('fr', 'translation', {
      'Model {{model}} is unavailable for this price basis.':
        'Unavailable translated: {{model}}',
      'Price {{dimension}} for {{model}} needs confirmation.':
        'Pending translated: {{dimension}} / {{model}}',
    })
    await i18next.changeLanguage('fr')
    setPricing({
      models: [
        pricingModel({ enable_groups: ['default'] }),
        pricingModel({
          id: 2,
          model_name: 'dynamic-model',
          enable_groups: ['vip'],
          billing_mode: 'tiered_expr',
          billing_expr: 'unsupported pricing expression',
        }),
      ],
    })
    localStorage.setItem(
      'new-api:product-quotation:draft:v1',
      JSON.stringify({
        version: 1,
        draft: {
          title: 'Validation quote',
          customer: '',
          quotedBy: '',
          quoteDate: '2026-09-08',
          globalDiscount: 8,
          priceBasis: { type: 'group', group: 'vip' },
          selectedModelIds: ['alpha-chat', 'dynamic-model'],
          providerOverrides: {},
        },
      })
    )

    render(<ProductQuotation />)

    expect(
      screen.getByText('Unavailable translated: alpha-chat')
    ).toBeInTheDocument()
    expect(
      screen.getByText('Pending translated: Dynamic pricing / dynamic-model')
    ).toBeInTheDocument()
  })

  test('exposes responsive stack and desktop columns without horizontal overflow', () => {
    render(<ProductQuotation />)
    const workspace = screen.getByTestId('quotation-workspace')
    expect(workspace).toHaveClass('grid-cols-1')
    expect(workspace).toHaveClass(
      'xl:grid-cols-[minmax(0,560px)_minmax(0,1fr)]'
    )
    expect(workspace).toHaveClass('min-w-0')
    expect(screen.getByTestId('quotation-preview-pane')).toHaveClass(
      'xl:sticky'
    )
  })
})

describe('exchange and discount calculator', () => {
  test('uses the live exchange rate and fixed system rate in their respective fields', () => {
    setPricing({ priceRate: 7.2, usdExchangeRate: 6.8 })
    render(<ProductQuotation />)

    expect(screen.getByLabelText('Actual exchange rate')).toHaveValue(6.8)
    expect(screen.getByLabelText('Fixed system exchange rate')).toHaveValue(7.2)
  })

  test('calculates and copies the exact suggested group ratio', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn(async () => undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    render(<ProductQuotation />)

    await user.clear(screen.getByLabelText('Actual exchange rate'))
    await user.type(screen.getByLabelText('Actual exchange rate'), '6.8')
    await user.clear(screen.getByLabelText('Calculator discount (zhe)'))
    await user.type(screen.getByLabelText('Calculator discount (zhe)'), '5')
    await user.clear(screen.getByLabelText('Fixed system exchange rate'))
    await user.type(screen.getByLabelText('Fixed system exchange rate'), '7')

    expect(screen.getByText('0.4857142857142857x')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Copy group ratio' }))
    expect(writeText).toHaveBeenCalledWith('0.4857142857142857')
    expect(await screen.findByText('Group ratio copied')).toBeInTheDocument()
  })

  test('reports invalid calculator values instead of producing a ratio', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)

    await user.clear(screen.getByLabelText('Fixed system exchange rate'))
    await user.type(screen.getByLabelText('Fixed system exchange rate'), '0')

    expect(
      screen.getByText(
        'Enter finite positive rates and a discount from 0 to 10.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Copy group ratio' })
    ).toBeDisabled()
  })
})
