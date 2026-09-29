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
import { describe, expect, it } from 'vitest'

import {
  getExpressionPricingKind,
  getPriceSummary,
} from '../model-pricing-snapshots'

const t = (key: string) => key

describe('expression pricing summaries', () => {
  it('classifies only expressions with multiple parsed branches as tiered', () => {
    expect(
      getExpressionPricingKind('tier("base", p * 3 + c * 15 + cr * 0.3)')
    ).toBe('expression')
    expect(
      getExpressionPricingKind(
        'len <= 200000 ? tier("base", p * 3 + c * 15) : tier("long_context", p * 6 + c * 22.5)'
      )
    ).toBe('tiered')
    expect(getExpressionPricingKind('max(p * 3, c * 15)')).toBe('expression')
  })

  it('shows one unconditional tier as expression pricing', () => {
    expect(
      getPriceSummary(
        {
          name: 'claude-sonnet-5',
          billingMode: 'tiered_expr',
          billingExpr: 'tier("base", p * 3 + c * 15 + cr * 0.3)',
          hasConflict: false,
        },
        t
      )
    ).toBe('Expression pricing')
  })

  it('shows conditional branches as tiered pricing', () => {
    expect(
      getPriceSummary(
        {
          name: 'long-context-model',
          billingMode: 'tiered_expr',
          billingExpr:
            'len <= 200000 ? tier("base", p * 3 + c * 15) : tier("long_context", p * 6 + c * 22.5)',
          hasConflict: false,
        },
        t
      )
    ).toBe('Tiered pricing · 2 tiers')
  })
})
