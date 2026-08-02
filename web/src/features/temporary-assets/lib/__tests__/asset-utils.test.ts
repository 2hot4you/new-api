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
  TEMPORARY_ASSET_GRID_CLASS_NAME,
  type TemporaryAsset,
  filterTemporaryAssets,
  getAssetStatusLabel,
  getPendingAssetIDs,
  toggleTemporaryAssetSelection,
  toggleVisibleTemporaryAssetSelection,
} from '../asset-utils'

const assets: TemporaryAsset[] = [
  {
    id: 'asset-molii-image',
    asset_type: 'image',
    status: 'active',
    created_at: 100,
    expires_at: 1_000,
    verified_at: 90,
  },
  {
    id: 'asset-molii-video',
    asset_type: 'video',
    status: 'processing',
    created_at: 200,
    expires_at: 1_000,
    verified_at: 190,
  },
  {
    id: 'asset-molii-audio',
    asset_type: 'audio',
    status: 'failed',
    created_at: 300,
    expires_at: 1_000,
    verified_at: 290,
  },
]

describe('temporary asset display behavior', () => {
  test('caps the responsive card grid at six columns', () => {
    const classes = TEMPORARY_ASSET_GRID_CLASS_NAME.split(/\s+/)
    assert.ok(classes.includes('grid-cols-1'))
    assert.ok(classes.includes('2xl:grid-cols-6'))
    assert.equal(
      classes.some((className) =>
        /grid-cols-(?:[7-9]|\d{2,})$/.test(className)
      ),
      false
    )
  })

  test('treats active assets as available and stops polling them', () => {
    assert.equal(getAssetStatusLabel('active'), 'Available')
    assert.deepEqual(getPendingAssetIDs(assets), ['asset-molii-video'])
  })

  test('treats upstream-expired assets as terminal', () => {
    const active = assets.at(0)
    assert.ok(active)
    const expired = { ...active, status: 'EXPIRED' }
    assert.equal(getAssetStatusLabel(expired.status), 'Expired')
    assert.deepEqual(getPendingAssetIDs([expired]), [])
  })

  test('combines type, partial ID, and timestamp filters', () => {
    assert.deepEqual(
      filterTemporaryAssets(assets, {
        assetType: 'video',
        assetID: 'MOLII-VID',
        startTimestamp: 150,
        endTimestamp: 250,
      }).map((item) => item.id),
      ['asset-molii-video']
    )
  })

  test('supports individual and select-all selection without dropping hidden assets', () => {
    const individual = toggleTemporaryAssetSelection(
      new Set(['asset-hidden']),
      'asset-molii-image'
    )
    assert.deepEqual([...individual].sort(), [
      'asset-hidden',
      'asset-molii-image',
    ])

    const allVisible = toggleVisibleTemporaryAssetSelection(individual, [
      'asset-molii-image',
      'asset-molii-video',
    ])
    assert.deepEqual([...allVisible].sort(), [
      'asset-hidden',
      'asset-molii-image',
      'asset-molii-video',
    ])

    const clearedVisible = toggleVisibleTemporaryAssetSelection(allVisible, [
      'asset-molii-image',
      'asset-molii-video',
    ])
    assert.deepEqual([...clearedVisible], ['asset-hidden'])
  })
})
