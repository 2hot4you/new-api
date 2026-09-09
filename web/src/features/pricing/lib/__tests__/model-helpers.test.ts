import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

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
      getPricingFilterGroups?.({
        default: { desc: 'User group', ratio: 1 },
        premium: { desc: 'Premium', ratio: 0.8 },
        auto: { desc: 'Automatic', ratio: 1 },
      }),
      ['premium']
    )
  })
})
