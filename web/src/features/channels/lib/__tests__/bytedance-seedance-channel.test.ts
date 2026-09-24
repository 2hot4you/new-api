/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { describe, expect, test } from 'vitest'

import {
  CHANNEL_TYPES,
  CHANNEL_TYPE_BYTEDANCE_SEEDANCE,
  CHANNEL_TYPE_OPTIONS,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  getBaseUrlForChannelTypeChange,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
} from '../channel-form'
import {
  getChannelTypeConfig,
  shouldShowBaseUrlField,
} from '../channel-type-config'
import { getChannelTestAction, getChannelTypeIcon } from '../channel-utils'

const formValues = {
  ...CHANNEL_FORM_DEFAULT_VALUES,
  name: 'ByteDance Seedance upstream',
  type: CHANNEL_TYPE_BYTEDANCE_SEEDANCE,
  base_url: 'https://upstream.example',
  key: 'example-key',
  models: 'doubao-seedance-2-0-260128',
}

describe('ByteDance Seedance channel', () => {
  test('registers type 64 with the Doubao icon and upstream model tools', () => {
    expect(CHANNEL_TYPE_BYTEDANCE_SEEDANCE).toBe(64)
    expect(CHANNEL_TYPES[CHANNEL_TYPE_BYTEDANCE_SEEDANCE]).toBe(
      'ByteDance Seedance'
    )
    expect(CHANNEL_TYPE_OPTIONS).toContainEqual({
      value: 64,
      label: 'ByteDance Seedance',
    })
    expect(getChannelTypeIcon(64)).toBe('Doubao')
    expect(MODEL_FETCHABLE_TYPES.has(64)).toBe(true)
    expect(shouldShowBaseUrlField(64)).toBe(true)
    expect(getChannelTestAction(64)).toEqual({
      direct: false,
      label: 'Test Connection',
    })

    const config = getChannelTypeConfig(64)
    expect(config.name).toBe('ByteDance Seedance')
    expect(config.icon).toBe('Doubao')
    expect(config.requiresBaseUrl).toBe(true)
    expect(config.requiresKey).toBe(true)
    expect(config.supportsFetchModels).toBe(true)
    expect(config.defaultBaseUrl).toBeUndefined()
    expect(config.supportedModels).toBeUndefined()
  })

  test('requires Base URL and Key when creating', () => {
    const result = channelFormSchema.safeParse({
      ...formValues,
      base_url: ' ',
      key: ' ',
    })
    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues.map((issue) => issue.path[0])).toContain(
      'base_url'
    )
    expect(result.error.issues.map((issue) => issue.path[0])).toContain('key')
    expect(channelFormSchema.safeParse(formValues).success).toBe(true)
  })

  test('requires a new Key when editing a channel into type 64', () => {
    const existing = channelSchema.parse({
      id: 45,
      type: 45,
      key: 'saved-volcengine-key',
      status: 1,
      name: 'VolcEngine upstream',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
    })
    const defaults = transformChannelToFormDefaults(existing)
    expect(defaults.original_type).toBe(45)
    const switched = {
      ...defaults,
      type: CHANNEL_TYPE_BYTEDANCE_SEEDANCE,
      base_url: 'https://seedance.example',
      models: formValues.models,
    }
    const result = channelFormSchema.safeParse(switched)
    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues.map((issue) => issue.path[0])).toContain('key')
    expect(
      channelFormSchema.safeParse({ ...switched, key: 'new-seedance-key' })
        .success
    ).toBe(true)
  })

  test('allows an unchanged type-64 edit to keep its saved Key', () => {
    const existing = channelSchema.parse({
      id: 64,
      type: CHANNEL_TYPE_BYTEDANCE_SEEDANCE,
      key: 'saved-seedance-key',
      status: 1,
      name: 'Seedance upstream',
      base_url: formValues.base_url,
      models: formValues.models,
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
    })
    const defaults = transformChannelToFormDefaults(existing)
    expect(defaults.original_type).toBe(CHANNEL_TYPE_BYTEDANCE_SEEDANCE)
    expect(defaults.key).toBe('')
    expect(channelFormSchema.safeParse(defaults).success).toBe(true)
  })

  test('clears an Ark endpoint when changing type 45 to type 64', () => {
    expect(
      getBaseUrlForChannelTypeChange(
        45,
        CHANNEL_TYPE_BYTEDANCE_SEEDANCE,
        'https://ark.cn-beijing.volces.com'
      )
    ).toBe('')
    expect(
      getBaseUrlForChannelTypeChange(
        45,
        CHANNEL_TYPE_BYTEDANCE_SEEDANCE,
        'https://ark.ap-southeast.bytepluses.com'
      )
    ).toBe('')
    expect(
      getBaseUrlForChannelTypeChange(
        45,
        CHANNEL_TYPE_BYTEDANCE_SEEDANCE,
        'https://custom.example'
      )
    ).toBe('https://custom.example')
  })

  test('leaves Base URL unchanged for normal type switches', () => {
    const arkURL = 'https://ark.cn-beijing.volces.com'
    expect(getBaseUrlForChannelTypeChange(45, 60, arkURL)).toBe(arkURL)
    expect(
      getBaseUrlForChannelTypeChange(1, CHANNEL_TYPE_BYTEDANCE_SEEDANCE, arkURL)
    ).toBe(arkURL)
  })

  test('keeps upstream auto-sync settings in the create payload', () => {
    const payload = transformFormDataToCreatePayload({
      ...formValues,
      upstream_model_update_check_enabled: true,
      upstream_model_update_auto_sync_enabled: true,
    })
    expect(payload.channel.type).toBe(64)
    expect(payload.channel.base_url).toBe('https://upstream.example')
    expect(JSON.parse(payload.channel.settings || '{}')).toMatchObject({
      upstream_model_update_check_enabled: true,
      upstream_model_update_auto_sync_enabled: true,
    })
  })
})
