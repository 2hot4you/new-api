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
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
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
