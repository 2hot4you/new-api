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
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import type { PricingModel, PricingVendor } from '@/features/pricing/types'

import { ProviderModelSelector } from '../provider-model-selector'

const vendors: PricingVendor[] = [
  { id: 10, name: 'Provider Alpha' },
  { id: 20, name: 'Provider Beta' },
]

function model(
  id: number,
  modelName: string,
  vendorId: number,
  enableGroups = ['default', 'vip']
): PricingModel {
  return {
    id,
    model_name: modelName,
    display_name: `${modelName} display`,
    vendor_id: vendorId,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 2,
    enable_groups: enableGroups,
  }
}

const longModelId = `provider-alpha/${'reasoning-model-'.repeat(18)}latest`
const models = [
  model(1, 'alpha-chat', 10),
  model(2, longModelId, 10, ['default']),
  model(3, 'beta-image', 20),
]

describe('provider grouped quotation model selection', () => {
  test('filters by model ID or provider while keeping long IDs wrap-safe', async () => {
    const user = userEvent.setup()
    render(
      <ProviderModelSelector
        models={models}
        vendors={vendors}
        selectedModelIds={[]}
        selectedGroup={null}
        onSelectionChange={() => undefined}
      />
    )

    expect(screen.getByText(longModelId)).toHaveClass('break-all')

    await user.type(
      screen.getByRole('searchbox', { name: 'Search models' }),
      'beta'
    )

    expect(screen.getByText('Provider Beta')).toBeInTheDocument()
    expect(screen.getByText('beta-image')).toBeInTheDocument()
    expect(screen.queryByText('alpha-chat')).not.toBeInTheDocument()
  })

  test('supports single, provider, and all-model selection with a live count', async () => {
    const user = userEvent.setup()
    const onSelectionChange = vi.fn()
    const { rerender } = render(
      <ProviderModelSelector
        models={models}
        vendors={vendors}
        selectedModelIds={[]}
        selectedGroup={null}
        onSelectionChange={onSelectionChange}
      />
    )

    await user.click(
      screen.getByRole('checkbox', { name: 'Select alpha-chat' })
    )
    expect(onSelectionChange).toHaveBeenLastCalledWith(['alpha-chat'])

    rerender(
      <ProviderModelSelector
        models={models}
        vendors={vendors}
        selectedModelIds={['alpha-chat']}
        selectedGroup={null}
        onSelectionChange={onSelectionChange}
      />
    )
    expect(screen.getByText('1 of 3 models selected')).toBeInTheDocument()

    await user.click(
      screen.getByRole('checkbox', {
        name: 'Select all models from Provider Alpha',
      })
    )
    expect(onSelectionChange).toHaveBeenLastCalledWith([
      'alpha-chat',
      longModelId,
    ])

    rerender(
      <ProviderModelSelector
        models={models}
        vendors={vendors}
        selectedModelIds={['alpha-chat', longModelId]}
        selectedGroup={null}
        onSelectionChange={onSelectionChange}
      />
    )
    await user.click(
      screen.getByRole('checkbox', { name: 'Select all models' })
    )
    expect(onSelectionChange).toHaveBeenLastCalledWith([
      'alpha-chat',
      longModelId,
      'beta-image',
    ])
  })

  test('supports keyboard selection for model checkboxes', async () => {
    const user = userEvent.setup()
    const onSelectionChange = vi.fn()
    render(
      <ProviderModelSelector
        models={models}
        vendors={vendors}
        selectedModelIds={[]}
        selectedGroup={null}
        onSelectionChange={onSelectionChange}
      />
    )

    const checkbox = screen.getByRole('checkbox', {
      name: 'Select alpha-chat',
    })
    checkbox.focus()
    await user.keyboard(' ')

    expect(onSelectionChange).toHaveBeenCalledWith(['alpha-chat'])
  })

  test('retains missing selections and marks group-unavailable selections', () => {
    render(
      <ProviderModelSelector
        models={models}
        vendors={vendors}
        selectedModelIds={[longModelId, 'removed-model']}
        selectedGroup='vip'
        onSelectionChange={() => undefined}
      />
    )

    expect(
      screen.getByText('Unavailable for selected group')
    ).toBeInTheDocument()
    expect(screen.getByText('removed-model')).toBeInTheDocument()
    expect(screen.getByText('Missing from current pricing')).toBeInTheDocument()
  })
})
