/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

export type AssetType = 'image' | 'video' | 'audio'

export const TEMPORARY_ASSET_GRID_CLASS_NAME =
  'grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-6'

export type TemporaryAsset = {
  id: string
  user_id?: number
  username?: string
  asset_type: AssetType
  name?: string
  source_url?: string
  preview_url?: string
  source_kind?: 'url' | 'cos'
  file_name?: string
  content_type?: string
  file_size?: number
  status: string
  created_at: number
  expires_at: number
  verified_at: number
}

type AssetFilters = {
  assetType: 'all' | AssetType
  assetID: string
  startTimestamp?: number
  endTimestamp?: number
}

export function getAssetTypeLabel(assetType: AssetType) {
  switch (assetType) {
    case 'image':
      return 'Image'
    case 'video':
      return 'Video'
    default:
      return 'Audio'
  }
}

export function getAssetStatusLabel(status: string) {
  switch (status.toUpperCase()) {
    case 'ACTIVE':
    case 'SUCCESS':
      return 'Available'
    case 'FAILED':
      return 'Failed'
    case 'EXPIRED':
      return 'Expired'
    default:
      return 'Processing'
  }
}

export function getAssetStatusVariant(
  status: string
): 'green' | 'red' | 'orange' {
  switch (status.toUpperCase()) {
    case 'ACTIVE':
    case 'SUCCESS':
      return 'green'
    case 'FAILED':
      return 'red'
    case 'EXPIRED':
      return 'red'
    default:
      return 'orange'
  }
}

export function getPendingAssetIDs(items: TemporaryAsset[]) {
  return items
    .filter(
      (item) =>
        !['ACTIVE', 'SUCCESS', 'FAILED', 'EXPIRED'].includes(
          item.status.toUpperCase()
        )
    )
    .map((item) => item.id)
}

export function filterTemporaryAssets(
  items: TemporaryAsset[],
  filters: AssetFilters
) {
  const normalizedID = filters.assetID.trim().toLowerCase()
  return items.filter((item) => {
    if (filters.assetType !== 'all' && item.asset_type !== filters.assetType) {
      return false
    }
    if (normalizedID && !item.id.toLowerCase().includes(normalizedID)) {
      return false
    }
    if (filters.startTimestamp && item.created_at < filters.startTimestamp) {
      return false
    }
    if (filters.endTimestamp && item.created_at > filters.endTimestamp) {
      return false
    }
    return true
  })
}

export function toggleTemporaryAssetSelection(
  selectedIDs: Set<string>,
  assetID: string
) {
  const next = new Set(selectedIDs)
  if (next.has(assetID)) {
    next.delete(assetID)
  } else {
    next.add(assetID)
  }
  return next
}

export function toggleVisibleTemporaryAssetSelection(
  selectedIDs: Set<string>,
  visibleIDs: string[]
) {
  const next = new Set(selectedIDs)
  const allVisibleSelected =
    visibleIDs.length > 0 && visibleIDs.every((id) => next.has(id))
  for (const id of visibleIDs) {
    if (allVisibleSelected) {
      next.delete(id)
    } else {
      next.add(id)
    }
  }
  return next
}
