import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { PricingModel } from '../../types'
import { ModelPriceCell } from '../model-price-cell'

afterEach(() => {
  cleanup()
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

describe('model price cell billing currency', () => {
  test('shows an explicit USD model in its source currency by default', () => {
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7,
      },
    })
    const model: PricingModel = {
      id: 2,
      model_name: 'usd-model',
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 2,
      enable_groups: ['default'],
      billing_currency: 'USD',
    }

    render(
      <ModelPriceCell model={model} options={{ currencyMode: 'source' }} />
    )

    expect(screen.getByText('USD / 1M tokens')).toBeVisible()
    expect(screen.getByText('2')).toBeVisible()
    expect(screen.getByText('4')).toBeVisible()
  })

  test('converts an explicit USD model when renminbi display is selected', () => {
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7,
      },
    })
    const model: PricingModel = {
      id: 4,
      model_name: 'usd-model-cny-display',
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 2,
      enable_groups: ['default'],
      billing_currency: 'USD',
    }

    render(<ModelPriceCell model={model} options={{ currencyMode: 'cny' }} />)

    expect(screen.getByText('CNY / 1M tokens')).toBeVisible()
    expect(screen.getByText('14')).toBeVisible()
    expect(screen.getByText('28')).toBeVisible()
  })

  test('keeps a CNY expression in its source currency when site quotas use tokens', () => {
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'TOKENS',
        usdExchangeRate: 7,
      },
    })
    const model: PricingModel = {
      id: 1,
      model_name: 'qwen3.5-flash',
      quota_type: 0,
      model_ratio: 0,
      completion_ratio: 0,
      enable_groups: ['default'],
      billing_mode: 'tiered_expr',
      billing_currency: 'CNY',
      billing_expr: 'tier("base", p * 0.2 + c * 2)',
    }

    render(
      <ModelPriceCell model={model} options={{ currencyMode: 'source' }} />
    )

    expect(screen.getByText('CNY / 1M tokens')).toBeVisible()
    expect(screen.getByText('0.2')).toBeVisible()
    expect(screen.getByText('2')).toBeVisible()
  })

  test('keeps legacy ratio prices in USD semantics before site conversion', () => {
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7,
      },
    })
    const model: PricingModel = {
      id: 3,
      model_name: 'cny-model',
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 2,
      enable_groups: ['default'],
      billing_currency: 'CNY',
    }

    render(<ModelPriceCell model={model} />)

    expect(screen.getByText('CNY / 1M tokens')).toBeVisible()
    expect(screen.getByText('14')).toBeVisible()
    expect(screen.getByText('28')).toBeVisible()
  })
})
