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

import type { TFunction } from 'i18next'

import { apiKeySchema, type ApiKey } from '../../types'
import {
  getApiKeyFormDefaultValues,
  getApiKeyFormSchema,
  transformApiKeyToFormDefaults,
  transformFormDataToPayload,
} from '../api-key-form'

const t = ((key: string, options?: Record<string, unknown>) => {
  if (options?.max !== undefined) {
    return key.replace('{{max}}', String(options.max))
  }
  return key
}) as TFunction

const baseApiKey: ApiKey = {
  id: 1,
  name: 'test',
  key: 'sk-test',
  status: 1,
  remain_quota: 0,
  used_quota: 0,
  unlimited_quota: true,
  expired_time: -1,
  created_time: 1,
  accessed_time: 0,
  group: 'auto',
  auto_groups: null,
  cross_group_retry: true,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}

describe('API key Auto group form mapping', () => {
  test('starts empty until the available real groups are loaded', () => {
    const defaults = getApiKeyFormDefaultValues(false)

    assert.equal(defaults.group, '')
    assert.deepEqual(defaults.auto_groups, [])
    assert.equal(defaults.auto_groups_mode, 'custom')
  })

  test('maps one selected group to fixed routing and multiple groups to ordered fallback', () => {
    const defaults = getApiKeyFormDefaultValues(false)

    assert.deepEqual(
      transformFormDataToPayload({
        ...defaults,
        auto_groups_mode: 'custom',
        auto_groups: ['vip'],
      }),
      {
        name: '',
        remain_quota: 0,
        expired_time: -1,
        unlimited_quota: true,
        model_limits_enabled: false,
        model_limits: '',
        allow_ips: '',
        group: 'vip',
        auto_groups: [],
        cross_group_retry: false,
      }
    )

    const multi = transformFormDataToPayload({
      ...defaults,
      auto_groups_mode: 'custom',
      auto_groups: ['vip', 'default'],
    })
    assert.equal(multi.group, 'auto')
    assert.deepEqual(multi.auto_groups, ['vip', 'default'])
    assert.equal(multi.cross_group_retry, true)
  })

  test('treats legacy token responses without auto_groups as inheritance', () => {
    const legacyApiKey: Record<string, unknown> = { ...baseApiKey }
    delete legacyApiKey.auto_groups

    assert.equal(apiKeySchema.parse(legacyApiKey).auto_groups, null)
  })

  test('creates an internally routed token with an explicit configured order', () => {
    const defaults = getApiKeyFormDefaultValues(true, ['vip', 'default'])

    assert.equal(defaults.group, 'auto')
    assert.equal(defaults.auto_groups_mode, 'custom')
    assert.deepEqual(defaults.auto_groups, ['vip', 'default'])
    assert.deepEqual(transformFormDataToPayload(defaults).auto_groups, [
      'vip',
      'default',
    ])
  })

  test('caps a copied system order to the per-key limit', () => {
    const defaults = getApiKeyFormDefaultValues(
      true,
      ['vip', 'default', 'team'],
      2
    )

    assert.deepEqual(defaults.auto_groups, ['vip', 'default'])
    assert.deepEqual(transformFormDataToPayload(defaults).auto_groups, [
      'vip',
      'default',
    ])
  })

  test('maps omitted, null, and empty snapshots to the current explicit order on edit', () => {
    const legacyApiKey: Record<string, unknown> = { ...baseApiKey }
    delete legacyApiKey.auto_groups
    const inheritedApiKeys = [
      apiKeySchema.parse(legacyApiKey),
      baseApiKey,
      { ...baseApiKey, auto_groups: [] },
    ]

    for (const apiKey of inheritedApiKeys) {
      const defaults = transformApiKeyToFormDefaults(
        apiKey,
        ['default', 'vip'],
        2,
        ['vip', 'default']
      )

      assert.equal(defaults.auto_groups_mode, 'custom')
      assert.deepEqual(defaults.auto_groups, ['vip', 'default'])
    }
  })

  test('filters a stored snapshot before applying a lowered limit', () => {
    const defaults = transformApiKeyToFormDefaults(
      {
        ...baseApiKey,
        auto_groups: ['revoked', 'vip', 'default'],
      },
      ['default', 'vip'],
      2
    )

    assert.equal(defaults.auto_groups_mode, 'custom')
    assert.deepEqual(defaults.auto_groups, ['vip', 'default'])
  })

  test('caps a legacy inherited token when resolving the current system order', () => {
    const defaults = transformApiKeyToFormDefaults(
      baseApiKey,
      ['vip', 'default', 'team'],
      1,
      ['vip', 'default', 'team']
    )

    assert.equal(defaults.auto_groups_mode, 'custom')
    assert.deepEqual(defaults.auto_groups, ['vip'])
  })

  test('keeps a fully filtered snapshot custom and rejects it until resolved', () => {
    const defaults = transformApiKeyToFormDefaults(
      { ...baseApiKey, auto_groups: ['revoked'] },
      ['default'],
      2
    )

    assert.equal(defaults.auto_groups_mode, 'custom')
    assert.deepEqual(defaults.auto_groups, [])

    const result = getApiKeyFormSchema(t, 2).safeParse(defaults)
    assert.equal(result.success, false)
    if (result.success) return
    assert.deepEqual(result.error.issues[0]?.path, ['auto_groups'])
    assert.equal(result.error.issues[0]?.message, 'Select at least one group')
  })

  test('submits a valid custom snapshot in its configured order', () => {
    const custom = {
      ...getApiKeyFormDefaultValues(true, ['vip', 'default']),
      auto_groups_mode: 'custom' as const,
      auto_groups: ['vip', 'default'],
    }

    assert.deepEqual(transformFormDataToPayload(custom).auto_groups, [
      'vip',
      'default',
    ])
  })

  test('submits an explicit snapshot for ordered fallback and none for fixed routing', () => {
    const inherited = getApiKeyFormDefaultValues(true, ['vip', 'default'])
    assert.deepEqual(transformFormDataToPayload(inherited).auto_groups, [
      'vip',
      'default',
    ])

    const fixed = {
      ...inherited,
      group: 'default',
      auto_groups_mode: 'custom' as const,
      auto_groups: ['default'],
    }
    assert.deepEqual(transformFormDataToPayload(fixed).auto_groups, [])
    assert.equal(transformFormDataToPayload(fixed).group, 'default')
    assert.equal(transformFormDataToPayload(fixed).cross_group_retry, false)
  })

  test('rejects snapshots over the configured limit', () => {
    const result = getApiKeyFormSchema(t, 1).safeParse({
      ...getApiKeyFormDefaultValues(true, ['default']),
      name: 'limited token',
      auto_groups_mode: 'custom',
      auto_groups: ['default', 'vip'],
    })

    assert.equal(result.success, false)
    if (result.success) return
    assert.equal(result.error.issues[0]?.path[0], 'auto_groups')
    assert.equal(result.error.issues[0]?.message, 'Select at most 1 groups')
  })

  test('rejects duplicate custom groups', () => {
    const result = getApiKeyFormSchema(t).safeParse({
      ...getApiKeyFormDefaultValues(true, ['vip']),
      name: 'duplicate token',
      auto_groups_mode: 'custom',
      auto_groups: ['vip', 'vip'],
    })

    assert.equal(result.success, false)
    if (result.success) return
    assert.equal(
      result.error.issues[0]?.message,
      'Groups must not contain duplicates'
    )
  })
})
