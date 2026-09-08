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
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
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

vi.mock('sonner', () => ({
  toast: {
    error: mocks.toastError,
    success: mocks.toastSuccess,
  },
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
  await user.type(
    screen.getByLabelText('Global discount (10 = full price)'),
    '8'
  )
  await user.click(screen.getByRole('checkbox', { name: 'Select alpha-chat' }))
}

beforeEach(() => {
  localStorage.clear()
  mocks.download.mockReset()
  mocks.toastError.mockReset()
  mocks.toastSuccess.mockReset()
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
  test.each([
    ['zhCN', 'zh-CN'],
    ['zhTW', 'zh-TW'],
  ])(
    'renders fetched pricing and exports a BCP-47 locale for %s',
    async (interfaceLocale, expectedIntlLocale) => {
      const user = userEvent.setup()
      await i18next.changeLanguage(interfaceLocale)
      localStorage.setItem(
        'new-api:product-quotation:draft:v1',
        JSON.stringify({
          version: 1,
          draft: {
            title: 'Chinese locale quotation',
            customer: '',
            quotedBy: '',
            quoteDate: '2026-09-08',
            globalDiscount: 8,
            priceBasis: { type: 'raw' },
            selectedModelIds: ['alpha-chat'],
            providerOverrides: {},
          },
        })
      )

      expect(() => render(<ProductQuotation />)).not.toThrow()
      expect(
        screen.getByLabelText(i18next.t('Quotation preview'))
      ).toBeInTheDocument()

      await user.click(
        screen.getByRole('button', {
          name: i18next.t('Export HTML quotation'),
        })
      )
      expect(mocks.download.mock.calls[0]?.[1]).toMatchObject({
        locale: expectedIntlLocale,
      })
    }
  )

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
    const refreshedAt = Date.parse('2026-09-09T12:30:00.000Z')
    mocks.pricing.refetch = vi.fn(async () => {
      mocks.pricing.dataUpdatedAt = refreshedAt
      return { isSuccess: true }
    })
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
      new Intl.DateTimeFormat('en', {
        dateStyle: 'medium',
        timeStyle: 'short',
      }).format(refreshedAt)
    )

    await user.type(
      screen.getByLabelText('Global discount (10 = full price)'),
      '8'
    )
    await user.click(
      screen.getByRole('button', { name: 'Export HTML quotation' })
    )
    expect(mocks.download.mock.calls[0]?.[0]).toMatchObject({
      fetchedAt: '2026-09-09T12:30:00.000Z',
    })

    await user.click(screen.getByRole('button', { name: 'Clear draft' }))
    expect(screen.getByLabelText('Quotation title')).toHaveValue('')
    expect(screen.getByLabelText('Customer')).toHaveValue('')
    await waitFor(() =>
      expect(
        localStorage.getItem('new-api:product-quotation:draft:v1')
      ).toBeNull()
    )

    await user.type(screen.getByLabelText('Quotation title'), 'New draft')
    await waitFor(() =>
      expect(
        JSON.parse(
          localStorage.getItem('new-api:product-quotation:draft:v1') ?? '{}'
        )
      ).toMatchObject({ draft: { title: 'New draft' } })
    )
  })

  test('reports local draft save failures without discarding the edited value', async () => {
    const user = userEvent.setup()
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage blocked')
    })
    render(<ProductQuotation />)

    await user.type(screen.getByLabelText('Quotation title'), 'Unsaved draft')

    expect(screen.getByLabelText('Quotation title')).toHaveValue(
      'Unsaved draft'
    )
    expect(
      await screen.findByText('Draft could not be saved locally')
    ).toBeInTheDocument()
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
      fetchedAt: '2026-09-08T10:00:00.000Z',
    })
  })

  test('passes localized row maps to export and reports download failures', async () => {
    const user = userEvent.setup()
    i18next.addResourceBundle('fr', 'translation', {
      Input: 'Entrée',
      '1M token': '1 M de jetons',
      '{{resolution}} without video input': '{{resolution}} sans entrée vidéo',
      '{{resolution}} with video input': '{{resolution}} avec entrée vidéo',
      '{{resolution}}; fps {{fps}}; extra frames {{extraFrames}}; Token = ceil(width x height x (fps x duration + extra frames) / 1024)':
        '{{resolution}} ; ips {{fps}} ; images supplémentaires {{extraFrames}} ; Jeton = ceil(width x height x (fps x duration + extra frames) / 1024)',
      cny_per_million_tokens: 'CNY par million de jetons',
      'Failed to export HTML quotation': 'Échec de l’export HTML',
    })
    await i18next.changeLanguage('fr')
    setPricing({
      models: [
        pricingModel({
          video_pricing: {
            unit: 'cny_per_million_tokens',
            fps: 24,
            extra_frames: 1,
            rows: [
              {
                resolutions: ['720p'],
                without_video: 37,
                with_video: 22,
              },
            ],
          },
        }),
      ],
    })
    mocks.download.mockImplementation(() => {
      throw new Error('download blocked')
    })
    render(<ProductQuotation />)
    await fillValidQuotation()

    await user.click(
      screen.getByRole('button', { name: 'Export HTML quotation' })
    )

    expect(mocks.download.mock.calls[0]?.[1]).toMatchObject({
      locale: 'fr',
      dimensionLabels: {
        Input: 'Entrée',
        '720p without video input': '720p sans entrée vidéo',
      },
      unitLabels: {
        '1M token': '1 M de jetons',
        cny_per_million_tokens: 'CNY par million de jetons',
      },
      conditionLabels: {
        '720p; fps 24; extra frames 1; Token = ceil(width x height x (fps x duration + extra frames) / 1024)':
          '720p ; ips 24 ; images supplémentaires 1 ; Jeton = ceil(width x height x (fps x duration + extra frames) / 1024)',
      },
    })
    expect(mocks.toastError).toHaveBeenCalledWith('Échec de l’export HTML')
    expect(mocks.toastSuccess).not.toHaveBeenCalledWith(
      'HTML quotation downloaded'
    )
  })

  test('uses provider discounts over the global discount and renders every price column', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)
    await fillValidQuotation()
    await user.type(
      screen.getByLabelText('Provider Alpha discount (10 = full price)'),
      '5'
    )

    expect(screen.getByText('5 tenths of list price')).toBeInTheDocument()
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
      screen.getByLabelText('Provider Alpha discount (10 = full price)'),
      '12'
    )

    expect(screen.getByText('Invalid provider discount')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Export HTML quotation' })
    ).toBeDisabled()
  })

  test('supports decimal discounts and wraps long unbroken provider notes', async () => {
    const user = userEvent.setup()
    const longNote = `https://example.test/${'unbroken'.repeat(80)}`
    render(<ProductQuotation />)
    await user.type(screen.getByLabelText('Quotation title'), 'Decimal quote')
    await user.type(
      screen.getByLabelText('Global discount (10 = full price)'),
      '7.5'
    )
    await user.click(
      screen.getByRole('checkbox', { name: 'Select alpha-chat' })
    )
    fireEvent.change(screen.getByLabelText('Provider Alpha note'), {
      target: { value: longNote },
    })

    expect(
      screen.getAllByText('7.5 tenths of list price').length
    ).toBeGreaterThan(0)
    expect(screen.getByText('$3')).toBeInTheDocument()
    const previewNote = screen
      .getAllByText(longNote)
      .find((element) => element.tagName === 'DIV')
    expect(previewNote).toHaveClass('break-words')
  })

  test('renders concrete USD and CNY dimensions without a cross-unit total', async () => {
    const user = userEvent.setup()
    setPricing({
      models: [
        pricingModel(),
        pricingModel({
          id: 2,
          model_name: 'image-model',
          molii_grok_pricing: {
            kind: 'image',
            output_unit: 'image',
            output_prices: { '1024x1024': 0.2 },
            image_input_unit: 'image',
            image_input_price: 0.05,
          },
        }),
      ],
    })
    render(<ProductQuotation />)
    await user.type(screen.getByLabelText('Quotation title'), 'Mixed quote')
    await user.type(
      screen.getByLabelText('Global discount (10 = full price)'),
      '8'
    )
    await user.click(
      screen.getByRole('checkbox', { name: 'Select all models' })
    )

    expect(screen.getAllByText('USD').length).toBeGreaterThan(0)
    expect(screen.getAllByText('CNY').length).toBeGreaterThan(0)
    expect(screen.getAllByText('1M token').length).toBeGreaterThan(0)
    expect(screen.getAllByText('image').length).toBeGreaterThan(0)
    expect(screen.getByText('1024x1024')).toBeInTheDocument()
    expect(screen.getByText('¥0.16')).toBeInTheDocument()
    expect(
      screen.queryByText(/grand total|total price/i)
    ).not.toBeInTheDocument()
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

  test('reports model ID copy failure through localized feedback', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)
    await fillValidQuotation()
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: vi.fn(async () => {
          throw new Error('clipboard blocked')
        }),
      },
    })
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: vi.fn(() => false),
    })

    await user.click(
      screen.getByRole('button', { name: 'Copy model ID alpha-chat' })
    )

    await waitFor(() =>
      expect(mocks.toastError).toHaveBeenCalledWith('Failed to copy model ID')
    )
    expect(screen.queryByText('Model ID copied')).not.toBeInTheDocument()
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

  test('synchronizes delayed pristine rates while preserving an edited rate', async () => {
    const user = userEvent.setup()
    setPricing({ priceRate: 1, usdExchangeRate: 1 })
    const view = render(<ProductQuotation />)

    setPricing({ priceRate: 7, usdExchangeRate: 6.8 })
    view.rerender(<ProductQuotation />)
    expect(screen.getByLabelText('Actual exchange rate')).toHaveValue(6.8)
    expect(screen.getByLabelText('Fixed system exchange rate')).toHaveValue(7)

    await user.clear(screen.getByLabelText('Actual exchange rate'))
    await user.type(screen.getByLabelText('Actual exchange rate'), '6.9')
    setPricing({ priceRate: 7.2, usdExchangeRate: 6.7 })
    view.rerender(<ProductQuotation />)

    expect(screen.getByLabelText('Actual exchange rate')).toHaveValue(6.9)
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
    await user.clear(
      screen.getByLabelText('Calculator discount (10 = full price)')
    )
    await user.type(
      screen.getByLabelText('Calculator discount (10 = full price)'),
      '5'
    )
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
        'Enter finite positive rates and a discount greater than 0 and at most 10.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Copy group ratio' })
    ).toBeDisabled()
    expect(screen.getByLabelText('Fixed system exchange rate')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(screen.getByLabelText('Fixed system exchange rate')).toHaveAttribute(
      'aria-describedby',
      'quotation-calculator-error'
    )

    await user.clear(screen.getByLabelText('Actual exchange rate'))
    expect(screen.getByLabelText('Actual exchange rate')).toHaveAttribute(
      'aria-invalid',
      'true'
    )
    expect(screen.getByLabelText('Actual exchange rate')).toHaveAttribute(
      'aria-describedby',
      'quotation-calculator-error'
    )
  })

  test('associates provider discount errors with their field', async () => {
    const user = userEvent.setup()
    render(<ProductQuotation />)
    await fillValidQuotation()
    const providerDiscount = screen.getByLabelText(
      'Provider Alpha discount (10 = full price)'
    )
    await user.type(providerDiscount, '12')

    expect(providerDiscount).toHaveAttribute('aria-invalid', 'true')
    expect(providerDiscount).toHaveAttribute(
      'aria-describedby',
      'quotation-provider-discount-10-error'
    )
    expect(
      document.querySelector('#quotation-provider-discount-10-error')
    ).toHaveTextContent('Enter a discount greater than 0 and at most 10.')
  })
})
