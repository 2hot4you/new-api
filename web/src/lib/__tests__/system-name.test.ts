import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { DEFAULT_SYSTEM_NAME, resolveSystemName } from '../constants'

describe('system name branding', () => {
  test('replaces the legacy New API name with the Molii default', () => {
    assert.equal(DEFAULT_SYSTEM_NAME, 'Molii Gateway')
    assert.equal(resolveSystemName(undefined), 'Molii Gateway')
    assert.equal(resolveSystemName('New API'), 'Molii Gateway')
    assert.equal(resolveSystemName(' NEWAPI '), 'Molii Gateway')
  })

  test('preserves a configured non-legacy system name', () => {
    assert.equal(resolveSystemName('Customer Gateway'), 'Customer Gateway')
  })
})
