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
  test('uses one CNY caption without mixing in site currency or a second symbol', () => {
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'TOKENS',
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

    expect(screen.getByText('CNY / 1M tokens')).toBeVisible()
    expect(screen.getByText('0.2')).toBeVisible()
    expect(screen.getByText('2')).toBeVisible()
    expect(screen.queryByText(/¥|USD/)).not.toBeInTheDocument()
  })
})
