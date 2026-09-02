import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { getEditableQuotaStep, quotaUnitsToEditableAmount } from '../format'

describe('editable quota formatting', () => {
  test('uses the configured small-value precision for currency inputs', () => {
    assert.equal(getEditableQuotaStep(), 0.0001)
    assert.equal(quotaUnitsToEditableAmount(250), 0.0005)
  })
})
