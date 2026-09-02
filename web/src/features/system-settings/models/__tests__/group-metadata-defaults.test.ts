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
*/
import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { getBillingSectionContent } from '../../billing/section-registry'
import type { BillingSettings } from '../../types'

describe('group pricing metadata defaults', () => {
  test('reloads persisted group metadata into the group pricing form', () => {
    const groupMetadata = JSON.stringify([
      { name: 'vip', icon: 'DeepSeek.Color', recommendation: 4 },
    ])
    const settings = {
      TopupGroupRatio: '{}',
      GroupRatio: '{"vip": 1}',
      UserUsableGroups: '{"vip": "Priority"}',
      GroupGroupRatio: '{}',
      AutoGroups: '[]',
      MaxTokenAutoGroups: 5,
      DefaultUseAutoGroup: false,
      'group_ratio_setting.group_metadata': groupMetadata,
      'group_ratio_setting.group_special_usable_group': '{}',
      'tool_price_setting.prices': '{}',
    } as BillingSettings

    const section = getBillingSectionContent('group-pricing', settings) as {
      props: { groupDefaults: { GroupMetadata?: string } }
    }

    assert.equal(section.props.groupDefaults.GroupMetadata, groupMetadata)
  })
})
