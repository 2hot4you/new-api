/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getBulkRefreshAssetIDs,
  refreshTemporaryAssets,
  refreshTemporaryAsset,
  replaceTemporaryAsset,
} from '../asset-actions'
import type { TemporaryAsset } from '../asset-utils'

const originalAssets: TemporaryAsset[] = [
  {
    id: 'asset-image',
    asset_type: 'image',
    status: 'processing',
    created_at: 100,
    expires_at: 1_000,
    verified_at: 90,
  },
  {
    id: 'asset-video',
    asset_type: 'video',
    status: 'active',
    created_at: 200,
    expires_at: 1_000,
    verified_at: 190,
  },
]

describe('temporary asset refresh', () => {
  test('uses the self endpoint and immediately replaces the refreshed asset', async () => {
    const refreshedAsset: TemporaryAsset = {
      ...originalAssets[0],
      status: 'active',
      verified_at: 300,
    }
    let requestedURL = ''

    const refreshed = await refreshTemporaryAsset(
      async (url) => {
        requestedURL = url
        return { data: { data: refreshedAsset } }
      },
      false,
      'asset-image'
    )
    const nextAssets = replaceTemporaryAsset(originalAssets, refreshed)

    assert.equal(requestedURL, '/api/assets/self/asset-image')
    assert.equal(nextAssets[0]?.status, 'active')
    assert.equal(nextAssets[0]?.verified_at, 300)
    assert.equal(nextAssets[1], originalAssets[1])
  })

  test('uses the admin endpoint for platform asset refreshes', async () => {
    let requestedURL = ''

    await refreshTemporaryAsset(
      async (url) => {
        requestedURL = url
        return { data: { data: originalAssets[0] } }
      },
      true,
      'asset-image'
    )

    assert.equal(requestedURL, '/api/assets/admin/asset-image')
  })

  test('refreshes every selected asset even when some are hidden by filters', () => {
    assert.deepEqual(
      getBulkRefreshAssetIDs(new Set(['asset-image', 'asset-hidden']), [
        'asset-video',
      ]),
      ['asset-image', 'asset-hidden']
    )
  })

  test('refreshes the current filtered assets when nothing is selected', () => {
    assert.deepEqual(
      getBulkRefreshAssetIDs(new Set(), ['asset-image', 'asset-video']),
      ['asset-image', 'asset-video']
    )
  })

  test('keeps successful refresh responses when another asset fails', async () => {
    const result = await refreshTemporaryAssets(
      async (url) => {
        if (url.endsWith('/asset-video')) throw new Error('upstream failed')
        return {
          data: {
            data: {
              ...originalAssets[0],
              status: 'active',
              verified_at: 400,
            },
          },
        }
      },
      false,
      ['asset-image', 'asset-video']
    )

    assert.deepEqual(
      result.refreshedAssets.map((asset) => asset.id),
      ['asset-image']
    )
    assert.deepEqual(result.failedAssetIDs, ['asset-video'])
  })
})
