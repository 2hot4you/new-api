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
  test('converts an explicit USD model into the site CNY currency', () => {
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

    render(<ModelPriceCell model={model} />)

    expect(screen.getByText('CNY / 1M tokens')).toBeVisible()
    expect(screen.getByText('14')).toBeVisible()
    expect(screen.getByText('28')).toBeVisible()
  })

  test('uses billing USD when the site display mode is tokens', () => {
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

    render(<ModelPriceCell model={model} />)

    expect(screen.getByText('USD / 1M tokens')).toBeVisible()
    expect(screen.getByText('0.028571')).toBeVisible()
    expect(screen.getByText('0.285714')).toBeVisible()
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
