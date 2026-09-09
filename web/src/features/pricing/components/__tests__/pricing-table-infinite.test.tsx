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
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import type { PricingModel } from '../../types'
import { PricingTable } from '../pricing-table'

function createModels(count: number): PricingModel[] {
  return Array.from({ length: count }, (_, index) => ({
    id: index + 1,
    model_name: `table-model-${index + 1}`,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: [],
  }))
}

describe('PricingTable continuous results', () => {
  it('renders every supplied model without table pagination', () => {
    render(<PricingTable models={createModels(25)} />)

    expect(screen.getByText('table-model-1')).toBeVisible()
    expect(screen.getByText('table-model-25')).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Go to next page' })
    ).not.toBeInTheDocument()
    expect(screen.getAllByRole('row')).toHaveLength(26)
  })
})
