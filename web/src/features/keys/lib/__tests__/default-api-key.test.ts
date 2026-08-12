import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  apiKeySchema,
  canDeleteApiKey,
  canSelectApiKey,
  getApiKeyRowActionPolicy,
  toDisplayApiKey,
} from '../../types'

describe('default API key UI policy', () => {
  test('shows the default flag from the API and keeps default keys out of delete and selection actions', () => {
    const defaultKey = apiKeySchema.parse({
      id: 1,
      name: 'Default',
      key: 'masked',
      status: 1,
      remain_quota: 0,
      used_quota: 0,
      unlimited_quota: true,
      expired_time: -1,
      created_time: 1,
      accessed_time: 1,
      model_limits_enabled: false,
      is_default: true,
    })

    assert.equal(defaultKey.is_default, true)
    assert.equal(canDeleteApiKey(defaultKey), false)
    assert.equal(canSelectApiKey(defaultKey), false)
    assert.deepEqual(getApiKeyRowActionPolicy(defaultKey), {
      showRotate: true,
      showDelete: false,
    })
  })

  test('keeps existing API keys selectable and deletable when the backend omits the new flag', () => {
    const regularKey = apiKeySchema.parse({
      id: 2,
      name: 'Regular',
      key: 'masked',
      status: 1,
      remain_quota: 0,
      used_quota: 0,
      unlimited_quota: true,
      expired_time: -1,
      created_time: 1,
      accessed_time: 1,
      model_limits_enabled: false,
    })

    assert.equal(regularKey.is_default, false)
    assert.equal(canDeleteApiKey(regularKey), true)
    assert.equal(canSelectApiKey(regularKey), true)
  })

  test('formats raw and already-prefixed rotated keys exactly once', () => {
    assert.equal(toDisplayApiKey('rotated-key'), 'sk-rotated-key')
    assert.equal(toDisplayApiKey('sk-rotated-key'), 'sk-rotated-key')
  })
})
