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

import { evaluateBillingExpression } from '@/features/pricing/lib/billing-expression/runtime'
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
  const view = (entry: Partial<ModelRatioData>) => (
    <QueryClientProvider client={client}>
      <ModelPricingEditorPanel
        ref={ref}
        editData={{
          name: 'currency-model',
          billingMode: 'tiered_expr',
          billingExpr: 'tier("base", p * 2 + c * 4)',
          billingCurrency: 'USD',
          hasMetadata: true,
          ...entry,
        }}
        onDirtyChange={dirty}
      />
    </QueryClientProvider>
  )
  const result = render(view(data))
  return {
    ref,
    dirty,
    reload: (entry: Partial<ModelRatioData>) => result.rerender(view(entry)),
  }
}

async function commit(
  ref: React.RefObject<ModelPricingEditorPanelHandle | null>
) {
  let result: ModelRatioData | null = null
  await act(async () => {
    result = (await ref.current?.commitDraft()) ?? null
  })
  return result as ModelRatioData | null
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

it.each([
  {
    name: 'custom-cache',
    ratio: 0,
    mode: 'standard',
    expression: 'tier("base", p * 2 + c * 4 + cr * 1 + cc * 0)',
  },
  {
    name: 'custom-claude',
    ratio: 1.5,
    mode: 'claude_ttl',
    expression: 'tier("base", p * 2 + c * 4 + cr * 1 + cc * 3 + cc1h * 4.8)',
  },
  {
    name: 'custom-gpt',
    ratio: 1.5,
    mode: 'openai_ttl',
    expression: 'tier("base", p * 2 + c * 4 + cr * 1 + cc * 3 + cc12h * 6)',
  },
])(
  'keeps configured cache prices visible on both sides for $name',
  async (fixture) => {
    const effective = {
      ModelRatio: 1,
      CompletionRatio: 2,
      CacheRatio: 0.5,
      CreateCacheRatio: fixture.ratio,
      ImageRatio: 1,
    }
    vi.spyOn(api, 'post').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: url.endsWith('/convert')
          ? {
              effective,
              expression: fixture.expression,
              cache_write_mode: fixture.mode,
            }
          : { effective, cache_write_mode: fixture.mode },
      },
    }))
    const editor = renderEditor({
      name: fixture.name,
      billingMode: 'per-token',
      billingExpr: '',
      ratio: '1',
      completionRatio: '2',
      createCacheRatio: String(fixture.ratio),
    })
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Convert to expression' }))
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Preview pricing conversion',
    })
    for (const side of ['Before conversion', 'After conversion']) {
      const panel = within(dialog).getByRole('region', { name: side })
      let label =
        side === 'Before conversion'
          ? 'Cache write price'
          : 'Cache create price'
      if (fixture.mode === 'claude_ttl') label = 'Cache Creation (5m)'
      if (fixture.mode === 'openai_ttl') label = 'Cache Creation (30m)'
      expect(
        within(panel).getByText(label).nextElementSibling
      ).toHaveTextContent(`$${fixture.ratio * 2} / 1M token`)
      if (fixture.mode === 'claude_ttl') {
        expect(
          within(panel).getByText('Cache create (1h) price').nextElementSibling
        ).toHaveTextContent('$4.8 / 1M token')
      } else {
        expect(
          within(panel).queryByText('Cache create (1h) price')
        ).not.toBeInTheDocument()
      }
      if (fixture.mode === 'openai_ttl') {
        expect(
          within(panel).getByText('Cache create (12h) price').nextElementSibling
        ).toHaveTextContent('$6 / 1M token')
      } else {
        expect(
          within(panel).queryByText('Cache create (12h) price')
        ).not.toBeInTheDocument()
      }
    }
    await userEvent
      .setup()
      .click(within(dialog).getByRole('button', { name: 'Apply to draft' }))
    expect(await commit(editor.ref)).toMatchObject({
      billingExpr: fixture.expression,
      createCacheRatio: String(fixture.ratio),
    })
  }
)

it.each([
  {
    name: 'gpt-4-32k',
    inputRatio: 30,
    cacheRatio: 1,
    expression: 'tier("base", p * 60 + c * 120)',
  },
  {
    name: 'free-cache-model',
    inputRatio: 0,
    cacheRatio: 0,
    expression: 'tier("base", p * 0 + c * 0 + cr * 0)',
  },
  {
    name: 'overlapping-cache',
    inputRatio: 30,
    cacheRatio: 1,
    createCacheRatio: 1.25,
    expression: 'tier("base", p * 60 + c * 120 + cr * 60 + cc * 75)',
  },
])(
  'merges equal cache prices while preserving explicit free cache for $name',
  async (fixture) => {
    const effective = {
      ModelRatio: fixture.inputRatio,
      CompletionRatio: 2,
      CacheRatio: fixture.cacheRatio,
      CreateCacheRatio: 1.25,
      ImageRatio: 1,
    }
    vi.spyOn(api, 'post').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: url.endsWith('/convert')
          ? {
              effective,
              expression: fixture.expression,
              cache_write_mode:
                fixture.createCacheRatio === undefined ? 'none' : 'standard',
            }
          : {
              effective,
              cache_write_mode:
                fixture.createCacheRatio === undefined ? 'none' : 'standard',
            },
      },
    }))
    const editor = renderEditor({
      name: fixture.name,
      billingMode: 'per-token',
      billingExpr: '',
      ratio: String(fixture.inputRatio),
      cacheRatio: String(fixture.cacheRatio),
      createCacheRatio: fixture.createCacheRatio?.toString(),
    })
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Convert to expression' }))
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Preview pricing conversion',
    })
    for (const side of ['Before conversion', 'After conversion']) {
      const panel = within(dialog).getByRole('region', { name: side })
      if (fixture.cacheRatio === 1) {
        expect(
          within(panel).queryByText('Cache read price')
        ).not.toBeInTheDocument()
      } else {
        expect(
          within(panel).getByText('Cache read price').nextElementSibling
        ).toHaveTextContent('$0 / 1M token')
      }
      expect(
        within(panel).getByText('Input price').nextElementSibling
      ).toHaveTextContent(`$${fixture.inputRatio * 2} / 1M token`)
    }
    await userEvent
      .setup()
      .click(within(dialog).getByRole('button', { name: 'Apply to draft' }))
    expect(await commit(editor.ref)).toMatchObject({
      billingExpr: fixture.expression,
      cacheRatio: String(fixture.cacheRatio),
    })
  }
)

it('discards a conversion preview when the model being edited changes', async () => {
  const effective = { ModelPrice: 0 }
  vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: { expression: 'tier("request", fixed(0))', effective },
    },
  })
  const editor = renderEditor({
    billingMode: 'per-request',
    billingExpr: '',
    price: '0',
  })
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Convert to expression' }))
  const dialog = await screen.findByRole('alertdialog', {
    name: 'Preview pricing conversion',
  })
  expect(within(dialog).getAllByText(/^\$0 \/ request$/)).toHaveLength(2)
  editor.reload({
    name: 'another-model',
    billingMode: 'per-token',
    billingExpr: '',
    ratio: '3',
  })
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  expect(await commit(editor.ref)).toMatchObject({
    name: 'another-model',
    billingMode: 'per-token',
    ratio: '3',
  })
  expect((await commit(editor.ref))?.billingExpr).toBeUndefined()
})

it('previews and retains image quantity and nested quality rules when applying a conversion', async () => {
  const condition =
    'param("quality") == "hd" && (param("size") == "1024x1792" || param("size") == "1792x1024")'
  const expression = `tier("image", fixed(0.04)) * image_count * (${condition} ? 3 : 1)`
  vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: {
        expression,
        effective: { ModelPrice: 0.04 },
        billing_details: {
          image_count: true,
          request_rules: [{ condition, multiplier: 3 }],
        },
      },
    },
  })
  const editor = renderEditor({
    name: 'dall-e-3',
    billingMode: 'per-request',
    billingExpr: '',
    price: '0.04',
  })
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Convert to expression' }))
  const dialog = await screen.findByRole('alertdialog', {
    name: 'Preview pricing conversion',
  })
  for (const name of ['Before conversion', 'After conversion']) {
    const panel = within(dialog).getByRole('region', { name })
    expect(
      within(panel).getByText('Price per image').nextElementSibling
    ).toHaveTextContent('$0.04 / image')
    expect(
      within(panel).getByText('Image count').nextElementSibling
    ).toHaveTextContent('Reserve requested images; settle returned images.')
    expect(within(panel).getByText('× 3')).toBeInTheDocument()
  }
  expect((await commit(editor.ref))?.billingMode).toBe('per-request')
  await userEvent
    .setup()
    .click(within(dialog).getByRole('button', { name: 'Apply to draft' }))
  const committed = await commit(editor.ref)
  expect(committed?.billingExpr).toContain('image_count')
  expect(
    evaluateBillingExpression(committed?.billingExpr || '', {
      imageCount: 2,
      request: { body: { quality: 'hd', size: '1024x1792' } },
    })
  ).toMatchObject({ status: 'success', cost: 240000 })
})

it('shows Gemini audio pricing when the legacy text price is zero without creating a ratio override', async () => {
  const effective = {
    ModelRatio: 0,
    CompletionRatio: 2,
    CacheRatio: 1,
    ImageRatio: 1,
  }
  const expression = 'tier("base", p * 0 + c * 0 + ai * 1)'
  vi.spyOn(api, 'post').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url.endsWith('/convert')
        ? { expression, effective, billing_details: { audio_input_price: 1 } }
        : { effective, billing_details: { audio_input_price: 1 } },
    },
  }))
  const editor = renderEditor({
    name: 'gemini-2.5-flash',
    billingMode: 'per-token',
    billingExpr: '',
    ratio: '0',
  })
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Convert to expression' }))
  const dialog = await screen.findByRole('alertdialog', {
    name: 'Preview pricing conversion',
  })
  for (const name of ['Before conversion', 'After conversion']) {
    const panel = within(dialog).getByRole('region', { name })
    expect(
      within(panel).getByText('Audio input price').nextElementSibling
    ).toHaveTextContent('$1 / 1M token')
  }
  await userEvent
    .setup()
    .click(within(dialog).getByRole('button', { name: 'Apply to draft' }))
  const committed = await commit(editor.ref)
  expect(committed?.billingExpr).toBe(expression)
  expect(committed?.audioRatio).toBe('')
})

it.each([0, 2])(
  'keeps an independent image ratio of %s visible in both conversion panels',
  async (imageRatio) => {
    const effective = {
      ModelRatio: 1,
      CompletionRatio: 2,
      ImageRatio: imageRatio,
    }
    vi.spyOn(api, 'post').mockResolvedValue({
      data: {
        success: true,
        data: {
          expression: `tier("base", p * 2 + c * 4 + img * ${imageRatio * 2})`,
          effective,
        },
      },
    })
    renderEditor({
      billingMode: 'per-token',
      billingExpr: '',
      ratio: '1',
      completionRatio: '2',
      imageRatio: String(imageRatio),
    })
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Convert to expression' }))
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Preview pricing conversion',
    })
    for (const title of ['Before conversion', 'After conversion']) {
      const panel = within(dialog).getByRole('region', { name: title })
      expect(within(panel).getByText('Image input price')).toBeVisible()
      expect(
        within(panel).getByText('Image input price').nextElementSibling
      ).toHaveTextContent(`$${imageRatio * 2} / 1M token`)
    }
  }
)

it('opens expression pricing by default when an existing model has no configured prices', () => {
  const editor = renderEditor({
    billingExpr: '',
    ratio: '',
    completionRatio: '',
    billingMode: 'per-token',
  })
  expect(screen.getByRole('tab', { name: 'Expression' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  expect(editor.dirty).toHaveBeenLastCalledWith(false)
})

it('preserves a legacy per-request draft when conversion is unsupported', async () => {
  vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: true,
      data: {
        unsupported_reason:
          'Image prices with count, size or quality adjustments must be converted manually.',
      },
    },
  })
  const editor = renderEditor({
    name: 'gpt-image-2',
    billingMode: 'per-request',
    billingExpr: '',
    price: '0',
  })
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Convert to expression' }))
  expect(await screen.findByRole('status')).toHaveTextContent(
    'Image prices with count, size or quality adjustments must be converted manually.'
  )
  const draft = await commit(editor.ref)
  expect(draft).toMatchObject({ billingMode: 'per-request', price: '0' })
  expect(draft?.billingExpr).toBeUndefined()
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
