import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import type { PricingModel } from '../../types'
import * as modelHelpers from '../model-helpers'

describe('pricing group filter visibility', () => {
  test('hides the default group while preserving configured public groups', () => {
    const getPricingFilterGroups = (
      modelHelpers as typeof modelHelpers & {
        getPricingFilterGroups?: (
          groups: Record<string, { desc: string; ratio: number }>
        ) => string[]
      }
    ).getPricingFilterGroups

    assert.equal(typeof getPricingFilterGroups, 'function')
    assert.deepEqual(
      getPricingFilterGroups?.(
        {
          default: { desc: 'User group', ratio: 1 },
          premium: { desc: 'Premium', ratio: 0.8 },
          auto: { desc: 'Automatic', ratio: 1 },
        },
        [
          {
            model_name: 'visible-model',
            enable_groups: ['premium'],
          } as PricingModel,
        ]
      ),
      ['premium']
    )
  })

  test('hides selectable groups that have no visible model', () => {
    assert.deepEqual(
      modelHelpers.getPricingFilterGroups(
        {
          ByteDance: { desc: 'ByteDance routes', ratio: 0.77 },
          empty: { desc: 'No visible models', ratio: 1 },
        },
        [
          {
            model_name: 'doubao-seedance-2-5',
            enable_groups: ['ByteDance'],
          } as PricingModel,
        ]
      ),
      ['ByteDance']
    )
  })
})
