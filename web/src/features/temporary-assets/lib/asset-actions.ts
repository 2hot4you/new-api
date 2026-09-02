/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { TemporaryAsset } from './asset-utils'

type AssetDetailRequester = (
  url: string
) => Promise<{ data?: { data?: TemporaryAsset } }>

export function getBulkRefreshAssetIDs(
  selectedIDs: Set<string>,
  visibleIDs: string[]
) {
  return selectedIDs.size > 0 ? [...selectedIDs] : visibleIDs
}

export async function refreshTemporaryAsset(
  request: AssetDetailRequester,
  isAdmin: boolean,
  assetID: string
) {
  const endpoint = isAdmin ? '/api/assets/admin' : '/api/assets/self'
  const response = await request(`${endpoint}/${assetID}`)
  const refreshedAsset = response.data?.data
  if (!refreshedAsset) throw new Error('Temporary asset response is empty')
  return refreshedAsset
}

export async function refreshTemporaryAssets(
  request: AssetDetailRequester,
  isAdmin: boolean,
  assetIDs: string[]
) {
  const results = await Promise.allSettled(
    assetIDs.map((assetID) => refreshTemporaryAsset(request, isAdmin, assetID))
  )
  const refreshedAssets: TemporaryAsset[] = []
  const failedAssetIDs: string[] = []
  results.forEach((result, index) => {
    if (result.status === 'fulfilled') {
      refreshedAssets.push(result.value)
    } else {
      failedAssetIDs.push(assetIDs[index])
    }
  })
  return { refreshedAssets, failedAssetIDs }
}

export function replaceTemporaryAsset(
  items: TemporaryAsset[],
  refreshedAsset: TemporaryAsset
) {
  return items.map((item) =>
    item.id === refreshedAsset.id ? refreshedAsset : item
  )
}
