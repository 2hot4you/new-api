/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { expect, test } from 'vitest'

import {
  chooseDefaultVideoStudioTokenID,
  hasUnavailableSelectedAsset,
} from '../authoring'

test('defaults to the newest compatible key without overriding a selection', () => {
  const tokens = [{ id: 9 }, { id: 4 }]

  expect(chooseDefaultVideoStudioTokenID('', tokens)).toBe('9')
  expect(chooseDefaultVideoStudioTokenID('4', tokens)).toBe('4')
})

test('treats missing, failed, pending, and expired referenced assets as unavailable', () => {
  const media = (id: string) => ({
    clientId: id,
    type: 'image' as const,
    source: 'asset' as const,
    role: 'reference_image' as const,
    value: id,
    name: id,
  })

  expect(
    hasUnavailableSelectedAsset(
      [media('active')],
      [
        {
          id: 'active',
          asset_type: 'image',
          status: 'ACTIVE',
          created_at: 1,
          expires_at: 200,
          verified_at: 1,
        },
      ],
      100
    )
  ).toBe(false)
  expect(hasUnavailableSelectedAsset([media('missing')], [], 100)).toBe(true)
  expect(
    hasUnavailableSelectedAsset(
      [media('expired')],
      [
        {
          id: 'expired',
          asset_type: 'image',
          status: 'SUCCESS',
          created_at: 1,
          expires_at: 99,
          verified_at: 1,
        },
      ],
      100
    )
  ).toBe(true)
})
