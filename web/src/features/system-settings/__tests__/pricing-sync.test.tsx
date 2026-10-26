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
  act,
  cleanup,
  render,
  screen,
  within,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  pricingOptions,
  applyPriceSyncSelections,
  pricingValuesByModel,
} from '@/features/model-pricing/pricing'
import { api } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { adjustSyncPrice } from '../models/upstream-price-adjustment'
import { UpstreamRatioSync } from '../models/upstream-ratio-sync'
import {
  getSyncPriceLines,
  describeSyncPrice,
  type PricingSourceSelections,
} from '../models/upstream-ratio-sync-helpers'
import { UpstreamRatioSyncTable } from '../models/upstream-ratio-sync-table'
import type { PricingSyncModels } from '../types'

const expression = 'tier("base", p * 2 + c * 8 + cr * 0)'
const dedicatedModels = [
  'doubao-seedance-2-0-260128',
  'doubao-seedance-2-0-fast-260128',
  'doubao-seedance-2-0-mini-260615',
  'doubao-seedance-2-5-260628',
]
const dedicatedGuidance =
  'This model is priced in the Seedance 2.0 tab. Update its price matrix there; the billing expression is generated automatically.'
const differences = {
  m: {
    model_ratio: {
      current: 0.5,
      upstreams: { upstream: 1 },
      confidence: { upstream: true },
    },
    completion_ratio: {
      current: 2,
      upstreams: { upstream: 4 },
      confidence: { upstream: true },
    },
  },
}
const prices: PricingSyncModels = {
  m: {
    current: { model_ratio: 0.5, completion_ratio: 2, billing_mode: 'ratio' },
    upstreams: {
      upstream: {
        model_ratio: 1,
        completion_ratio: 4,
        cache_ratio: 0,
        billing_mode: 'ratio',
      },
    },
  },
}
function TableFixture(props: {
  prices: PricingSyncModels
  disabled?: boolean
  initialSelectedSources?: PricingSourceSelections
}) {
  const [selectedSources, setSelectedSources] =
    useState<PricingSourceSelections>(props.initialSelectedSources ?? {})
  return (
    <UpstreamRatioSyncTable
      prices={props.prices}
      differences={differences}
      selectedSources={selectedSources}
      isDisabled={props.disabled ?? false}
      isSyncing={false}
      onSelectPrices={(selections) =>
        setSelectedSources((previous) => ({
          ...previous,
          ...Object.fromEntries(
            selections.map((selection) => [selection.model, selection.source])
          ),
        }))
      }
      onUnselectPrices={(models) =>
        setSelectedSources((previous) =>
          Object.fromEntries(
            Object.entries(previous).filter(
              ([model]) => !models.includes(model)
            )
          )
        )
      }
    />
  )
}
afterEach(() => {
  cleanup()
  useSystemConfigStore
    .getState()
    .setConfig({ currency: DEFAULT_CURRENCY_CONFIG })
})

describe('pricing synchronization', () => {
  it('guides all dedicated Seedance models to their tab without individual or stale generic selections', () => {
    render(
      <TableFixture
        prices={Object.fromEntries(
          dedicatedModels.map((model) => [model, prices.m])
        )}
        initialSelectedSources={Object.fromEntries(
          dedicatedModels.map((model) => [model, 'upstream'])
        )}
      />
    )
    for (const model of dedicatedModels) {
      const row = screen.getByRole('row', { name: new RegExp(`^${model} `) })
      expect(within(row).queryByRole('checkbox')).not.toBeInTheDocument()
      expect(within(row).getByText(dedicatedGuidance)).toBeVisible()
    }
    expect(
      screen.getByRole('checkbox', { name: /Select all prices from upstream$/ })
    ).toHaveAttribute('aria-disabled', 'true')
    expect(screen.getByText('0/0')).toBeVisible()
  })

  it('excludes dedicated Seedance models from mixed bulk selections and filtered eligible counts', async () => {
    render(
      <TableFixture
        prices={{
          ...prices,
          ...Object.fromEntries(
            dedicatedModels.map((model) => [model, prices.m])
          ),
        }}
      />
    )
    const user = userEvent.setup()
    await user.click(
      screen.getByRole('checkbox', { name: /Select all prices from upstream$/ })
    )
    expect(screen.getByText('1/1')).toBeVisible()
    expect(
      screen.getByRole('checkbox', {
        name: /Select price for m from upstream$/,
      })
    ).toBeChecked()
    await user.type(
      screen.getByRole('textbox', { name: 'Search models' }),
      'doubao'
    )
    expect(await screen.findByText('0/0')).toBeVisible()
    expect(
      screen.getByRole('checkbox', { name: /Select all prices from upstream$/ })
    ).toHaveAttribute('aria-disabled', 'true')
    await user.clear(screen.getByRole('textbox', { name: 'Search models' }))
    expect(await screen.findByText('1/1')).toBeVisible()
  })

  it('includes the visible localized bulk action in its accessible name', async () => {
    i18next.addResourceBundle(
      'fr',
      'translation',
      {
        'Select all': 'Tout sélectionner',
        'Select all prices from {{source}}':
          'Sélectionner tous les prix de {{source}}',
      },
      true,
      true
    )
    await i18next.changeLanguage('fr')
    try {
      render(<TableFixture prices={prices} />)
      expect(
        screen.getByText('Tout sélectionner', { exact: true })
      ).toBeVisible()
      expect(
        screen.getByRole('checkbox', {
          name: /Sélectionner tous les prix de upstream$/,
        })
      ).toHaveAccessibleName(
        'Tout sélectionner Sélectionner tous les prix de upstream'
      )
    } finally {
      cleanup()
      await i18next.changeLanguage('en')
    }
  })

  it('shows clickable selection text and preserves contextual names and keyboard focus', async () => {
    render(<TableFixture prices={prices} />)
    const user = userEvent.setup()
    const model = screen.getByRole('checkbox', {
      name: /Select price for m from upstream$/,
    })
    expect(screen.getByText('Select', { exact: true })).toBeVisible()
    expect(screen.getByText('Select all', { exact: true })).toBeVisible()
    await user.click(screen.getByText('Select', { exact: true }))
    expect(model).toBeChecked()
    expect(model).toHaveAccessibleName(
      'Clear selection Select price for m from upstream'
    )
    expect(
      screen.getByRole('checkbox', { name: /Select all prices from upstream$/ })
    ).toHaveAccessibleName('Clear selection Select all prices from upstream')
    const row = screen.getByRole('row', { name: /^m / })
    await user.click(within(row).getByText('Clear selection', { exact: true }))
    expect(model).not.toBeChecked()
    await user.click(model)
    expect(model).toBeChecked()
    await user.keyboard(' ')
    expect(model).not.toBeChecked()
    expect(model).toHaveFocus()
  })

  it('selects all filtered pages through visible text and clears only that source selection', async () => {
    const candidates = Object.fromEntries(
      Array.from({ length: 12 }, (_, index) => [
        `model-${String(index).padStart(2, '0')}`,
        prices.m,
      ])
    )
    render(<TableFixture prices={candidates} />)
    const user = userEvent.setup()
    await user.click(screen.getByText('Select all', { exact: true }))
    expect(screen.getByText('12/12')).toBeVisible()
    await user.type(
      screen.getByRole('textbox', { name: 'Search models' }),
      'model-11'
    )
    const last = await screen.findByRole('checkbox', {
      name: /Select price for model-11 from upstream$/,
    })
    expect(last).toBeChecked()
    const header = screen.getByRole('columnheader', { name: /upstream/ })
    await user.click(
      within(header).getByText('Clear selection', { exact: true })
    )
    expect(last).not.toBeChecked()
    await user.clear(screen.getByRole('textbox', { name: 'Search models' }))
    expect(await screen.findByText('11/12')).toBeVisible()
    expect(
      screen.getByRole('checkbox', { name: /Select all prices from upstream$/ })
    ).toBePartiallyChecked()
  })

  it('keeps visible selection labels inert while prices are loading or saving', async () => {
    render(<TableFixture prices={prices} disabled />)
    const user = userEvent.setup()
    await user.click(screen.getByText('Select', { exact: true }))
    await user.click(screen.getByText('Select all', { exact: true }))
    const model = screen.getByRole('checkbox', {
      name: /Select price for m from upstream$/,
    })
    expect(model).toHaveAttribute('aria-disabled', 'true')
    expect(model).not.toBeChecked()
    expect(screen.getByText('0/1')).toBeVisible()
  })

  it('shows every dollar price inline, including explicit zero cache and audio prices', () => {
    render(<TableFixture prices={prices} />)
    expect(screen.queryAllByText('Model ratio')).toHaveLength(0)
    expect(screen.getAllByText('$2')).toHaveLength(2)
    expect(screen.getByText('$8')).toBeVisible()
    expect(screen.getByText('$0')).toBeVisible()
    expect(screen.getByText('Cache Read')).toBeVisible()
    expect(
      screen.queryByRole('button', { name: /details/i })
    ).not.toBeInTheDocument()
    expect(
      getSyncPriceLines(
        {
          model_ratio: 2,
          audio_ratio: 3,
          audio_completion_ratio: 4,
          image_ratio: 0,
        },
        (key) => key
      )
    ).toEqual([
      { label: 'Input', value: '$4' },
      { label: 'Image input', value: '$0' },
      { label: 'Audio input', value: '$12' },
      { label: 'Audio output', value: '$48' },
    ])
  })

  it('parses expression prices inline while copying the unchanged source expression', async () => {
    render(
      <TableFixture
        prices={{
          m: {
            current: {},
            upstreams: {
              upstream: {
                billing_mode: 'tiered_expr',
                source_currency: 'USD',
                billing_expr: expression,
              },
            },
          },
        }}
      />
    )
    expect(screen.queryByText(expression)).not.toBeInTheDocument()
    expect(screen.getByText('$2')).toBeVisible()
    expect(screen.getByText('$8')).toBeVisible()
    expect(screen.getByText('$0')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Copy billing expression' })
    ).toBeVisible()
    expect(
      screen.queryByRole('button', { name: /details/i })
    ).not.toBeInTheDocument()
    const user = userEvent.setup()
    const copy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue()
    await user.click(
      screen.getByRole('button', { name: 'Copy billing expression' })
    )
    expect(copy).toHaveBeenCalledWith(expression)
  })

  it('shows every parsed tier and falls back to the full expression when pricing cannot be parsed safely', () => {
    const tiered =
      'len <= 128000 ? tier("base", p * 2 + c * 8 + cr * 0.2) : tier("long", p * 4 + c * 12 + cr * 0.4)'
    const custom = 'tier("custom", p * 2 + c * 8) * max(1, param("factor"))'
    render(
      <TableFixture
        prices={{
          tiered: {
            current: {},
            upstreams: {
              upstream: {
                billing_mode: 'tiered_expr',
                billing_expr: tiered,
                source_currency: 'USD',
              },
            },
          },
          custom: {
            current: {},
            upstreams: {
              upstream: { billing_mode: 'tiered_expr', billing_expr: custom },
            },
          },
        }}
      />
    )
    expect(screen.queryByText(tiered)).not.toBeInTheDocument()
    expect(screen.getByText(/128,000/)).toBeVisible()
    expect(screen.getByText('$0.2')).toBeVisible()
    expect(screen.getByText('$0.4')).toBeVisible()
    expect(screen.getByText(custom)).toBeVisible()
  })

  it('shows a mobile comparison list and preserves source-wide selection', async () => {
    const original = window.matchMedia
    vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      ...original(query),
      matches: query === '(max-width: 640px)',
    }))
    render(
      <TableFixture
        prices={{ ...prices, 'doubao-seedance-2-0-260128': prices.m }}
      />
    )
    const user = userEvent.setup()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.getByText(dedicatedGuidance)).toBeVisible()
    expect(screen.getByText('m')).toBeVisible()
    expect(screen.getByText('Select', { exact: true })).toBeVisible()
    expect(screen.getByText('Select all', { exact: true })).toBeVisible()
    await user.click(
      screen.getByRole('checkbox', { name: /Select all prices from upstream$/ })
    )
    expect(screen.getByText('1/1')).toBeVisible()
    const source = screen.getByRole('checkbox', {
      name: /Select price for m from upstream$/,
    })
    expect(source).toBeChecked()
    source.focus()
    await user.keyboard(' ')
    expect(source).not.toBeChecked()
    expect(source).toHaveFocus()
  })

  it('selects one complete source per model and keeps filtered-out selections', async () => {
    const candidates: PricingSyncModels = {
      ...prices,
      z: {
        current: { model_ratio: 1, billing_mode: 'ratio' },
        upstreams: { upstream: { model_price: 0, billing_mode: 'ratio' } },
      },
    }
    candidates.m = {
      ...candidates.m,
      upstreams: {
        ...candidates.m.upstreams,
        alternative: { billing_mode: 'tiered_expr', billing_expr: expression },
      },
    }
    render(<TableFixture prices={candidates} />)
    const user = userEvent.setup()
    const first = screen.getByRole('checkbox', {
      name: /Select price for m from upstream$/,
    })
    await user.click(first)
    await user.click(
      screen.getByRole('checkbox', {
        name: /Select price for m from alternative$/,
      })
    )
    expect(first).not.toBeChecked()
    expect(
      screen.getByRole('checkbox', {
        name: /Select price for m from alternative$/,
      })
    ).toBeChecked()
    await user.click(
      screen.getByRole('checkbox', { name: /Select all prices from upstream$/ })
    )
    expect(first).toBeChecked()
    expect(
      screen.getByRole('checkbox', {
        name: /Select price for z from upstream$/,
      })
    ).toBeChecked()
    await user.type(screen.getByRole('textbox', { name: 'Search models' }), 'm')
    await waitFor(() =>
      expect(
        screen.queryByRole('checkbox', {
          name: /Select price for z from upstream$/,
        })
      ).not.toBeInTheDocument()
    )
    await user.click(
      screen.getByRole('checkbox', { name: /Select all prices from upstream$/ })
    )
    await user.clear(screen.getByRole('textbox', { name: 'Search models' }))
    expect(
      await screen.findByRole('checkbox', {
        name: /Select price for z from upstream$/,
      })
    ).toBeChecked()
    expect(
      screen.getByRole('checkbox', {
        name: /Select price for m from upstream$/,
      })
    ).not.toBeChecked()
  })

  it('replaces the selected pricing bundle without mixing legacy fields into expressions', () => {
    const options = pricingOptions({
      ModelRatio: '{"m":1,"untouched":2}',
      CompletionRatio: '{"m":2}',
      CacheRatio: '{"m":0.5}',
    })
    const applied = applyPriceSyncSelections(options, {
      m: {
        billing_mode: 'tiered_expr',
        source_currency: 'USD',
        billing_expr: expression,
        model_ratio: 9,
        model_price: 1,
      },
    })
    expect(pricingValuesByModel(applied).get('m')).toEqual({
      'billing_setting.billing_mode': 'tiered_expr',
      'billing_setting.billing_expr': expression,
    })
    expect(pricingValuesByModel(applied).get('untouched')).toEqual({
      ModelRatio: 2,
    })
    const legacy = applyPriceSyncSelections(applied, {
      m: { model_ratio: 1, completion_ratio: 4, cache_ratio: 0 },
    })
    expect(pricingValuesByModel(legacy).get('m')).toEqual({
      'billing_setting.billing_mode': 'ratio',
      ModelRatio: 1,
      CompletionRatio: 4,
      CacheRatio: 0,
    })
  })

  it('previews the expression, rejects a stale save and reloads before applying without residual ratio differences', async () => {
    let version = 'v1'
    const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/ratio_sync/channels') {
        return {
          data: {
            success: true,
            data: [
              {
                id: 7,
                name: 'Upstream',
                base_url: 'https://example.test',
                type: 1,
                status: 1,
              },
            ],
          },
        }
      }
      return {
        data: {
          success: true,
          data: {
            entries: [
              {
                model_name: 'm',
                version,
                has_metadata: true,
                billing_currency: 'USD',
                configured: { ModelRatio: 1, CompletionRatio: 2 },
                effective: { ModelRatio: 1, CompletionRatio: 2 },
              },
            ],
            empty_version: 'empty',
            options: pricingOptions({
              ModelRatio: '{"m":1,"untouched":2}',
              CompletionRatio: '{"m":2}',
            }),
          },
        },
      }
    })
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: {
        success: true,
        data: {
          differences: {
            m: {
              billing_expr: {
                current: null,
                upstreams: { 'Upstream(7)': expression },
                confidence: { 'Upstream(7)': true },
              },
            },
          },
          prices: {
            ...Object.fromEntries(
              dedicatedModels.map((model) => [
                model,
                {
                  current: {},
                  upstreams: {
                    'Upstream(7)': {
                      billing_mode: 'tiered_expr',
                      source_currency: 'USD',
                      billing_expr: expression,
                    },
                  },
                },
              ])
            ),
            m: {
              current: {
                model_ratio: 1,
                completion_ratio: 2,
                billing_mode: 'ratio',
              },
              upstreams: {
                'Upstream(7)': {
                  billing_mode: 'tiered_expr',
                  source_currency: 'USD',
                  billing_expr: expression,
                },
              },
            },
          },
          test_results: [{ name: 'Upstream(7)', status: 'success' }],
        },
      },
    })
    const patch = vi
      .spyOn(api, 'patch')
      .mockResolvedValueOnce({
        data: {
          success: false,
          message: 'Model pricing changed; reload before saving',
        },
      })
      .mockResolvedValue({ data: { success: true } })
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    render(
      <QueryClientProvider client={client}>
        <UpstreamRatioSync />
      </QueryClientProvider>
    )
    const user = userEvent.setup()
    await user.click(
      screen.getByRole('button', { name: 'Select price sources' })
    )
    await user.click(
      await screen.findByRole('checkbox', { name: 'Select Upstream' })
    )
    await user.click(screen.getByRole('button', { name: 'Confirm Selection' }))
    await user.click(
      await screen.findByRole('checkbox', {
        name: /Select all prices from Upstream\(7\)$/,
      })
    )
    expect(screen.getByText('1 selected models')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Clear selection' }))
    expect(screen.getByRole('button', { name: 'Apply Sync' })).toBeDisabled()
    await user.click(
      screen.getByRole('checkbox', {
        name: /Select price for m from Upstream\(7\)$/,
      })
    )
    await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
    const preview = screen.getByRole('alertdialog', {
      name: 'Preview price changes',
    })
    for (const model of dedicatedModels) {
      expect(within(preview).queryByText(model)).not.toBeInTheDocument()
    }
    expect(within(preview).getByText(/Expression pricing/)).toHaveTextContent(
      'Input: $2'
    )
    expect(within(preview).getByText(/Expression pricing/)).toHaveTextContent(
      'Output: $8'
    )
    expect(within(preview).queryByText(expression)).not.toBeInTheDocument()
    expect(patch).not.toHaveBeenCalled()
    await user.click(
      within(preview).getByRole('button', { name: 'Confirm Changes' })
    )
    expect(
      await screen.findByText('Model pricing changed; reload before saving')
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Confirm Changes' })
    ).toBeDisabled()
    version = 'v2'
    await user.click(screen.getByRole('button', { name: 'Reload pricing' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Selection' }))
    await user.click(
      await screen.findByRole('checkbox', {
        name: /Select all prices from Upstream\(7\)$/,
      })
    )
    await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
    await user.click(screen.getByRole('button', { name: 'Confirm Changes' }))
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(
      screen.queryByText('No upstream price differences found')
    ).not.toBeInTheDocument()
    expect(screen.getAllByText(dedicatedGuidance)).toHaveLength(4)
    expect(screen.getByText('0/0')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Apply Sync' })).toBeDisabled()
    expect(patch).toHaveBeenLastCalledWith('/api/option/model_pricing', {
      changes: [
        {
          model_name: 'm',
          expected_version: 'v2',
          billing_currency: 'USD',
          pricing: {
            'billing_setting.billing_mode': 'tiered_expr',
            'billing_setting.billing_expr': expression,
          },
        },
      ],
    })
    expect(post).toHaveBeenCalledTimes(2)
    expect(
      get.mock.calls.filter(([url]) => url === '/api/option/model_pricing')
        .length
    ).toBeGreaterThanOrEqual(2)
  })
})

it('shows a source failure instead of a no-differences result and retries the selected source', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/ratio_sync/channels') {
      return {
        data: {
          success: true,
          data: [
            {
              id: 7,
              name: 'Upstream',
              base_url: 'https://example.test',
              type: 1,
              status: 1,
            },
          ],
        },
      }
    }
    return {
      data: {
        success: true,
        data: {
          entries: [],
          empty_version: 'empty',
          options: pricingOptions({}),
        },
      },
    }
  })
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Source unavailable' },
    })
    .mockResolvedValue({
      data: {
        success: true,
        data: {
          prices: {},
          differences: {},
          test_results: [{ name: 'Upstream(7)', status: 'success' }],
        },
      },
    })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <UpstreamRatioSync />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Select price sources' }))
  await user.click(
    await screen.findByRole('checkbox', { name: 'Select Upstream' })
  )
  await user.click(screen.getByRole('button', { name: 'Confirm Selection' }))
  expect(await screen.findByText('Source unavailable')).toBeVisible()
  expect(
    screen.queryByText('No upstream price differences found')
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Apply Sync' })).toBeDisabled()
  await user.click(screen.getByRole('button', { name: 'Retry' }))
  expect(
    await screen.findByText('No upstream price differences found')
  ).toBeVisible()
  expect(post).toHaveBeenCalledTimes(2)
  expect(post.mock.calls[0]).toEqual(post.mock.calls[1])
})

it('preserves upstream expression text so a repeated comparison remains unchanged', () => {
  const source =
    'tier("base", p * 2 + c * 8) * (header("service-tier") == "fast" ? 2 : 1)'
  const after = applyPriceSyncSelections(pricingOptions({}), {
    model: {
      billing_mode: 'tiered_expr',
      billing_expr: source,
      source_currency: 'USD',
    },
  })
  expect(
    pricingValuesByModel(after).get('model')?.['billing_setting.billing_expr']
  ).toBe(source)
})

describe('sync price adjustments', () => {
  it('keeps source prices by default and scales only the base amount of legacy prices', () => {
    const source = {
      model_ratio: 3.5,
      completion_ratio: 4,
      cache_ratio: 0,
      audio_ratio: 2,
      audio_completion_ratio: 3,
    }
    expect(adjustSyncPrice(source, 'keep', null)).toEqual({ values: source })
    expect(adjustSyncPrice(source, 'divide', 7)).toEqual({
      values: { ...source, model_ratio: 0.5 },
    })
    expect(adjustSyncPrice(source, 'multiply', 7)).toEqual({
      values: { ...source, model_ratio: 24.5 },
    })
    expect(source.model_ratio).toBe(3.5)
    expect(adjustSyncPrice({ model_price: 0 }, 'divide', 7)).toEqual({
      values: { model_price: 0 },
    })
  })

  it('preserves tier thresholds, explicit zero prices, version prefixes and request multipliers', () => {
    const source =
      'v1:(len <= 128000 ? tier("short", p * 7 + c * 21 + cr * 0) : tier("long", fixed(0.7))) * (header("speed") == "fast" ? 3 : 1)'
    expect(
      adjustSyncPrice(
        {
          billing_mode: 'tiered_expr',
          billing_expr: source,
          source_currency: 'USD',
        },
        'divide',
        7
      )
    ).toEqual({
      values: {
        billing_mode: 'tiered_expr',
        source_currency: 'USD',
        billing_expr:
          'v1:(len <= 128000 ? tier("short", p * 1 + c * 3 + cr * 0) : tier("long", fixed(0.09999999999999999))) * (header("speed") == "fast" ? 3 : 1)',
      },
    })
  })

  it('rejects unsupported expressions, invalid rates and overflow without changing source prices', () => {
    for (const rate of [null, 0, -1, Number.NaN, Infinity]) {
      expect(
        adjustSyncPrice({ model_price: 1 }, 'divide', rate)
      ).toHaveProperty('error')
    }
    for (const expr of [
      'max(p * 7, 10)',
      'tier("x", p * 7) + 5',
      'tier("x", p * 7) * max(1, param("factor"))',
    ]) {
      expect(
        adjustSyncPrice(
          { billing_mode: 'tiered_expr', billing_expr: expr },
          'divide',
          7
        )
      ).toHaveProperty('error')
    }
    expect(
      adjustSyncPrice({ model_price: Number.MAX_VALUE }, 'multiply', 7)
    ).toHaveProperty('error')
    expect(
      adjustSyncPrice({ model_price: Number.MIN_VALUE }, 'divide', 7)
    ).toHaveProperty('error')
  })
})

async function renderAdjustmentSync(
  candidates: PricingSyncModels = {
    a: {
      current: {},
      upstreams: {
        'Upstream(7)': { model_price: 7 },
        alternative: { model_price: 14 },
      },
    },
    b: {
      current: {},
      upstreams: {
        'Upstream(7)': {
          model_ratio: 3.5,
          completion_ratio: 4,
          cache_ratio: 0,
        },
      },
    },
    c: { current: {}, upstreams: { 'Upstream(7)': { model_price: 21 } } },
  }
) {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7,
    },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/ratio_sync/channels'
          ? [
              {
                id: 7,
                name: 'Upstream',
                base_url: 'https://example.test',
                type: 1,
                status: 1,
              },
            ]
          : {
              entries: Object.keys(candidates).map((model_name) => ({
                model_name,
                version: 'empty',
                has_metadata: true,
                billing_currency: 'USD',
                configured: {},
                effective: {},
              })),
              empty_version: 'empty',
              options: pricingOptions({}),
            },
    },
  }))
  vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: {
        prices: candidates,
        differences: {},
        test_results: [{ name: 'Upstream(7)', status: 'success' }],
      },
    },
  })
  const patch = vi
    .spyOn(api, 'patch')
    .mockResolvedValue({ data: { success: true } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <UpstreamRatioSync />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Select price sources' }))
  await user.click(
    await screen.findByRole('checkbox', { name: 'Select Upstream' })
  )
  await user.click(screen.getByRole('button', { name: 'Confirm Selection' }))
  await waitFor(() =>
    expect(
      screen.queryByText('Fetching upstream prices...')
    ).not.toBeInTheDocument()
  )
  return { user, patch }
}

it('applies bulk changes only to existing selections, allows row overrides and saves the frozen preview', async () => {
  const { user, patch } = await renderAdjustmentSync()
  await user.click(
    await screen.findByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from Upstream(7)'),
    })
  )
  await user.click(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for b from Upstream(7)'),
    })
  )
  expect(
    screen.getByRole('button', {
      name: 'Price adjustment for a from Upstream(7)',
    })
  ).toHaveAttribute('title', 'Keep source price')
  await user.click(
    screen.getByRole('combobox', { name: 'Set selected prices' })
  )
  await user.click(
    screen.getByRole('option', { name: 'Divide by exchange rate (÷ 7)' })
  )
  await user.click(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for c from Upstream(7)'),
    })
  )
  expect(
    screen.getByRole('button', {
      name: 'Price adjustment for c from Upstream(7)',
    })
  ).toHaveAttribute('title', 'Keep source price')
  await user.click(
    screen.getByRole('button', {
      name: 'Price adjustment for b from Upstream(7)',
    })
  )
  await user.click(
    screen.getByRole('menuitemradio', {
      name: 'Multiply by exchange rate (× 7)',
    })
  )
  await user.type(screen.getByRole('textbox', { name: 'Search models' }), 'a')
  await waitFor(() =>
    expect(
      screen.queryByRole('button', {
        name: 'Price adjustment for b from Upstream(7)',
      })
    ).not.toBeInTheDocument()
  )
  await user.clear(screen.getByRole('textbox', { name: 'Search models' }))
  expect(
    await screen.findByRole('button', {
      name: 'Price adjustment for b from Upstream(7)',
    })
  ).toHaveAttribute('title', 'Multiply by exchange rate (× 7)')
  await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
  const dialog = screen.getByRole('alertdialog', {
    name: 'Preview price changes',
  })
  const rowA = within(dialog).getByText('a').closest('tr')
  expect(rowA).toHaveTextContent('Per-request: $1')
  expect(rowA).toHaveTextContent('Per-request: ¥7')
  expect(within(dialog).getByText('b').closest('tr')).toHaveTextContent(
    'Input: $49'
  )
  expect(within(dialog).getByText('c').closest('tr')).toHaveTextContent(
    'Per-request: $21'
  )
  await act(async () =>
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 8,
      },
    })
  )
  expect(
    within(dialog).getByText('Current exchange rate: 1 USD = 7 CNY')
  ).toBeVisible()
  await user.click(
    within(dialog).getByRole('button', { name: 'Confirm Changes' })
  )
  await waitFor(() =>
    expect(patch).toHaveBeenCalledWith('/api/option/model_pricing', {
      changes: expect.arrayContaining([
        {
          model_name: 'a',
          expected_version: 'empty',
          billing_currency: 'USD',
          pricing: { 'billing_setting.billing_mode': 'ratio', ModelPrice: 1 },
        },
        {
          model_name: 'b',
          expected_version: 'empty',
          billing_currency: 'USD',
          pricing: {
            'billing_setting.billing_mode': 'ratio',
            ModelRatio: 24.5,
            CompletionRatio: 4,
            CacheRatio: 0,
          },
        },
        {
          model_name: 'c',
          expected_version: 'empty',
          billing_currency: 'USD',
          pricing: { 'billing_setting.billing_mode': 'ratio', ModelPrice: 21 },
        },
      ]),
    })
  )
  expect(patch.mock.calls[0][1]).toHaveProperty('changes.length', 3)
})

it('does not compound repeated bulk conversion and resets adjustments when switching sources or clearing selection', async () => {
  const { user } = await renderAdjustmentSync()
  await user.click(
    await screen.findByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from Upstream(7)'),
    })
  )
  for (let i = 0; i < 2; i++) {
    await user.click(
      screen.getByRole('combobox', { name: 'Set selected prices' })
    )
    await user.click(
      screen.getByRole('option', { name: 'Divide by exchange rate (÷ 7)' })
    )
  }
  await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
  expect(
    within(screen.getByRole('alertdialog')).getByText(/Per-request: \$1/)
  ).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Cancel' }))
  await user.click(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from alternative'),
    })
  )
  expect(
    screen.getByRole('button', {
      name: 'Price adjustment for a from alternative',
    })
  ).toHaveAttribute('title', 'Keep source price')
  await user.click(
    screen.getByRole('button', {
      name: 'Price adjustment for a from alternative',
    })
  )
  await user.click(
    screen.getByRole('menuitemradio', {
      name: 'Multiply by exchange rate (× 7)',
    })
  )
  await user.click(screen.getByRole('button', { name: 'Clear selection' }))
  expect(
    screen.getByRole('combobox', { name: 'Set selected prices' })
  ).toBeDisabled()
  await user.click(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from alternative'),
    })
  )
  expect(
    screen.getByRole('button', {
      name: 'Price adjustment for a from alternative',
    })
  ).toHaveAttribute('title', 'Keep source price')
})

it('allows matching prices to be selected and adjusted in the mobile list', async () => {
  const original = window.matchMedia
  vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
    ...original(query),
    matches: query === '(max-width: 640px)',
  }))
  const { user, patch } = await renderAdjustmentSync({
    same: {
      current: { model_price: 7 },
      upstreams: { 'Upstream(7)': { model_price: 7 } },
    },
  })
  expect(
    screen.queryByRole('checkbox', {
      name: (name) => name.endsWith('Select price for same from Upstream(7)'),
    })
  ).not.toBeInTheDocument()
  await user.click(
    screen.getByRole('checkbox', { name: 'Show models with matching prices' })
  )
  await user.click(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for same from Upstream(7)'),
    })
  )
  const selector = screen.getByRole('button', {
    name: 'Price adjustment for same from Upstream(7)',
  })
  selector.focus()
  await user.keyboard('{Enter}')
  await user.click(
    screen.getByRole('menuitemradio', { name: 'Divide by exchange rate (÷ 7)' })
  )
  await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
  expect(screen.getByRole('alertdialog')).toHaveTextContent('Per-request: $1')
  await user.click(screen.getByRole('button', { name: 'Confirm Changes' }))
  await waitFor(() => expect(patch).toHaveBeenCalled())
})

it('blocks unsupported expressions until restored and disables conversion for invalid site rates', async () => {
  const { user } = await renderAdjustmentSync({
    custom: {
      current: {},
      upstreams: {
        'Upstream(7)': {
          billing_mode: 'tiered_expr',
          source_currency: 'USD',
          billing_expr: 'max(p * 7, 10)',
        },
      },
    },
  })
  await user.click(
    await screen.findByRole('checkbox', {
      name: (name) => name.endsWith('Select price for custom from Upstream(7)'),
    })
  )
  await user.click(
    screen.getByRole('button', {
      name: 'Price adjustment for custom from Upstream(7)',
    })
  )
  await user.click(
    screen.getByRole('menuitemradio', { name: 'Divide by exchange rate (÷ 7)' })
  )
  expect(screen.getByRole('alert')).toHaveTextContent(
    'This expression cannot be adjusted automatically.'
  )
  expect(screen.getByRole('button', { name: 'Apply Sync' })).toBeDisabled()
  await user.click(
    screen.getByRole('button', {
      name: 'Price adjustment for custom from Upstream(7)',
    })
  )
  await user.click(
    screen.getByRole('menuitemradio', { name: 'Keep source price' })
  )
  expect(screen.getByRole('button', { name: 'Apply Sync' })).toBeEnabled()
  await act(async () =>
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 0,
      },
    })
  )
  await user.click(
    screen.getByRole('combobox', { name: 'Set selected prices' })
  )
  expect(
    screen.getByRole('option', { name: /Divide by exchange rate/ })
  ).toHaveAttribute('aria-disabled', 'true')
  expect(
    screen.getByRole('option', { name: 'Keep source price' })
  ).not.toHaveAttribute('aria-disabled', 'true')
})

it('uses the custom currency rate and blocks an invalid rate after a conversion is selected', async () => {
  const { user } = await renderAdjustmentSync()
  await act(async () =>
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CUSTOM',
        customCurrencySymbol: '€',
        customCurrencyExchangeRate: 2,
        usdExchangeRate: 7,
      },
    })
  )
  await user.click(
    await screen.findByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from Upstream(7)'),
    })
  )
  await user.click(
    screen.getByRole('button', {
      name: 'Price adjustment for a from Upstream(7)',
    })
  )
  await user.click(
    screen.getByRole('menuitemradio', { name: 'Divide by exchange rate (÷ 2)' })
  )
  await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
  const dialog = screen.getByRole('alertdialog')
  expect(dialog).toHaveTextContent('Per-request: $3.5')
  expect(dialog).toHaveTextContent('Per-request: €7')
  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  await act(async () =>
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CUSTOM',
        customCurrencySymbol: '€',
        customCurrencyExchangeRate: 0,
      },
    })
  )
  expect(screen.getByRole('alert')).toHaveTextContent(
    'A valid site exchange rate is required.'
  )
  expect(screen.getByRole('button', { name: 'Apply Sync' })).toBeDisabled()
  await user.click(
    screen.getByRole('button', {
      name: 'Price adjustment for a from Upstream(7)',
    })
  )
  await user.click(
    screen.getByRole('menuitemradio', { name: 'Keep source price' })
  )
  expect(screen.getByRole('button', { name: 'Apply Sync' })).toBeEnabled()
})

it('shows a positive converted price below display precision without labeling it free', () => {
  const adjusted = adjustSyncPrice({ model_price: 1e-10 }, 'divide', 1e7)
  expect(adjusted.values).toBeDefined()
  expect(getSyncPriceLines(adjusted.values ?? {}, (key) => key)).toEqual([
    { label: 'Per-request', value: '$1e-17' },
  ])
})

it('adjusts a price from its source cell without expanding the model column or requiring prior selection', async () => {
  const { user } = await renderAdjustmentSync()
  const button = await screen.findByRole('button', {
    name: 'Price adjustment for a from Upstream(7)',
  })
  const row = button.closest('tr')
  expect(row).not.toBeNull()
  expect(button.closest('td')).toHaveTextContent('$7')
  expect(
    screen.queryByRole('combobox', { name: /Price adjustment for/ })
  ).not.toBeInTheDocument()
  expect(screen.queryByText('Final saved price')).not.toBeInTheDocument()
  await user.click(button)
  expect(
    screen.getByRole('menuitemradio', { name: 'Keep source price' })
  ).toHaveAttribute('aria-checked', 'true')
  await user.keyboard('{Escape}')
  expect(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from Upstream(7)'),
    })
  ).not.toBeChecked()
  expect(button).toHaveFocus()
  await user.click(button)
  await user.click(
    screen.getByRole('menuitemradio', { name: 'Divide by exchange rate (÷ 7)' })
  )
  expect(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from Upstream(7)'),
    })
  ).toBeChecked()
  expect(button).toHaveTextContent('÷ 7')
  expect(button.closest('td')).toHaveTextContent('$1')
  expect(button.closest('td')).not.toHaveTextContent('$7')
  expect(screen.getByText('a').closest('td')).not.toHaveTextContent('÷ 7')
  expect(screen.queryByText('Final saved price')).not.toBeInTheDocument()
  await user.click(button)
  expect(
    screen.getByRole('menuitemradio', { name: 'Divide by exchange rate (÷ 7)' })
  ).toHaveAttribute('aria-checked', 'true')
  await user.click(
    screen.getByRole('menuitemradio', { name: 'Keep source price' })
  )
  expect(button.closest('td')).toHaveTextContent('$7')
  expect(button).toHaveAttribute('title', 'Keep source price')
})

it('applies multiply and divide shortcuts to selected models across filters without affecting later selections', async () => {
  const { user } = await renderAdjustmentSync()
  const multiply = screen.getByRole('button', {
    name: 'Multiply by exchange rate (× 7)',
  })
  const divide = screen.getByRole('button', {
    name: 'Divide by exchange rate (÷ 7)',
  })
  expect(multiply).toBeDisabled()
  expect(divide).toBeDisabled()
  await user.click(
    await screen.findByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from Upstream(7)'),
    })
  )
  await user.click(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for b from Upstream(7)'),
    })
  )
  await user.type(screen.getByRole('textbox', { name: 'Search models' }), 'a')
  await waitFor(() =>
    expect(
      screen.queryByRole('checkbox', {
        name: (name) => name.endsWith('Select price for b from Upstream(7)'),
      })
    ).not.toBeInTheDocument()
  )
  await user.click(multiply)
  expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  expect(
    screen.getByRole('button', {
      name: 'Price adjustment for a from Upstream(7)',
    })
  ).toHaveAttribute('title', 'Multiply by exchange rate (× 7)')
  await user.click(divide)
  await user.click(divide)
  await user.clear(screen.getByRole('textbox', { name: 'Search models' }))
  expect(
    await screen.findByRole('button', {
      name: 'Price adjustment for b from Upstream(7)',
    })
  ).toHaveAttribute('title', 'Divide by exchange rate (÷ 7)')
  await user.click(
    screen.getByRole('checkbox', {
      name: (name) => name.endsWith('Select price for c from Upstream(7)'),
    })
  )
  expect(
    screen.getByRole('button', {
      name: 'Price adjustment for c from Upstream(7)',
    })
  ).toHaveAttribute('title', 'Keep source price')
  await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
  const dialog = screen.getByRole('alertdialog')
  expect(within(dialog).getByText('a').closest('tr')).toHaveTextContent(
    'Per-request: $1'
  )
  expect(within(dialog).getByText('b').closest('tr')).toHaveTextContent(
    'Input: $1'
  )
  expect(within(dialog).getByText('c').closest('tr')).toHaveTextContent(
    'Per-request: $21'
  )
})

it('disables both batch shortcuts when the configured exchange rate becomes invalid', async () => {
  const { user } = await renderAdjustmentSync()
  await user.click(
    await screen.findByRole('checkbox', {
      name: (name) => name.endsWith('Select price for a from Upstream(7)'),
    })
  )
  await act(async () =>
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 0,
      },
    })
  )
  expect(
    screen.getByRole('button', { name: /Multiply by exchange rate/ })
  ).toBeDisabled()
  expect(
    screen.getByRole('button', { name: /Divide by exchange rate/ })
  ).toBeDisabled()
})

it('displays source CNY expressions without relabeling coefficients as dollars', () => {
  const value = {
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("base", p * 7 + c * 14)',
    source_currency: 'CNY',
  }
  expect(describeSyncPrice(value, (key) => key)).toContain('CNY / 1M token')
  expect(describeSyncPrice(value, (key) => key)).toContain('¥7')
  expect(describeSyncPrice(value, (key) => key)).not.toContain('$7')
})

it('does not assign a USD denomination to an expression with unknown source currency', () => {
  expect(
    describeSyncPrice(
      { billing_mode: 'tiered_expr', billing_expr: 'tier("base", p * 7)' },
      (key) => key
    )
  ).toContain('Source pricing currency unavailable')
})

it('freezes CNY source denomination and the public version with the confirmation payload', async () => {
  const { user, patch } = await renderAdjustmentSync({
    cny: {
      current: {
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("base", p * 2 + c * 4)',
        source_currency: 'USD',
      },
      upstreams: {
        'Upstream(7)': {
          billing_mode: 'tiered_expr',
          billing_expr: 'tier("base", p * 7 + c * 14)',
          source_currency: 'CNY',
        },
      },
    },
  })
  await user.click(
    await screen.findByRole('checkbox', {
      name: (name) => name.endsWith('Select price for cny from Upstream(7)'),
    })
  )
  await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
  const dialog = screen.getByRole('alertdialog', {
    name: 'Preview price changes',
  })
  expect(within(dialog).getByText('cny').closest('tr')).toHaveTextContent(
    'CNY / 1M token'
  )
  await act(async () =>
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 8,
      },
    })
  )
  await user.click(
    within(dialog).getByRole('button', { name: 'Confirm Changes' })
  )
  await waitFor(() =>
    expect(patch).toHaveBeenCalledWith('/api/option/model_pricing', {
      changes: [
        {
          model_name: 'cny',
          expected_version: 'empty',
          billing_currency: 'CNY',
          pricing: {
            'billing_setting.billing_mode': 'tiered_expr',
            'billing_setting.billing_expr': 'tier("base", p * 7 + c * 14)',
          },
        },
      ],
    })
  )
})

it('rejects the entire sync selection when one expression has unknown source denomination', async () => {
  const { user, patch } = await renderAdjustmentSync({
    valid: { current: {}, upstreams: { 'Upstream(7)': { model_price: 2 } } },
    unknown: {
      current: {},
      upstreams: {
        'Upstream(7)': {
          billing_mode: 'tiered_expr',
          billing_expr: 'tier("base", p * 7 + c * 14)',
        },
      },
    },
  })
  await user.click(
    await screen.findByRole('checkbox', {
      name: (name) => name.includes('Select all prices from Upstream(7)'),
    })
  )
  await user.click(screen.getByRole('button', { name: 'Apply Sync' }))
  expect(
    screen.queryByRole('alertdialog', { name: 'Preview price changes' })
  ).not.toBeInTheDocument()
  expect(patch).not.toHaveBeenCalled()
})
