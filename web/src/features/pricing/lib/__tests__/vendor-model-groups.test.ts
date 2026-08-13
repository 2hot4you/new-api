import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PricingModel } from '../../types'
import { groupModelsByVendor } from '../vendor-model-groups'

function model(
  id: number,
  modelName: string,
  vendor?: { id?: number; name?: string }
): PricingModel {
  return {
    id,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    vendor_id: vendor?.id,
    vendor_name: vendor?.name,
  }
}

describe('model marketplace vendor groups', () => {
  test('keeps first vendor appearance and model order stable', () => {
    const models = [
      model(1, 'vendor-a-first', { id: 10, name: 'Vendor A' }),
      model(2, 'vendor-b-first', { id: 20, name: 'Vendor B' }),
      model(3, 'vendor-a-second', { id: 10, name: 'Vendor A' }),
    ]

    assert.deepEqual(
      groupModelsByVendor(models).map((group) =>
        group.map((item) => item.model_name)
      ),
      [['vendor-a-first', 'vendor-a-second'], ['vendor-b-first']]
    )
  })

  test('falls back to vendor name when no vendor id exists', () => {
    const models = [
      model(1, 'first', { name: 'Named Vendor' }),
      model(2, 'second', { name: 'Named Vendor' }),
    ]

    assert.deepEqual(groupModelsByVendor(models), [[models[0], models[1]]])
  })

  test('does not merge models without vendor metadata', () => {
    const first = model(1, 'unknown-one')
    const second = model(2, 'unknown-two')

    assert.deepEqual(groupModelsByVendor([first, second]), [[first], [second]])
  })
})
