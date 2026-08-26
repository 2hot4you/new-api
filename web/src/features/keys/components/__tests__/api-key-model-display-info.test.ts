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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PricingData } from '@/features/pricing/types'

describe('API key model provider display mapping', () => {
  test('joins restricted model IDs to the configured pricing vendor', async () => {
    const columnsModule = await import('../api-keys-columns')
    const buildModelDisplayInfo = (
      columnsModule as typeof columnsModule & {
        buildApiKeyModelDisplayInfo?: (
          data: PricingData
        ) => Record<string, { providerIcon?: string; providerName?: string }>
      }
    ).buildApiKeyModelDisplayInfo

    assert.equal(typeof buildModelDisplayInfo, 'function')

    const pricingData = {
      success: true,
      data: [
        {
          id: 1,
          model_name: 'deepseek-v4-pro-202606',
          icon: 'DeepSeek.Color',
          quota_type: 0,
          model_ratio: 1,
          completion_ratio: 1,
          enable_groups: ['default'],
          vendor_id: 8,
        },
      ],
      vendors: [
        {
          id: 8,
          name: 'DeepSeek',
          icon: 'DeepSeek.Provider.Color',
        },
      ],
      group_ratio: {},
      usable_group: {},
      supported_endpoint: {},
      auto_groups: [],
    } satisfies PricingData

    assert.deepEqual(buildModelDisplayInfo?.(pricingData), {
      'deepseek-v4-pro-202606': {
        modelIcon: 'DeepSeek.Color',
        providerIcon: 'DeepSeek.Provider.Color',
        providerName: 'DeepSeek',
      },
    })
  })
})
