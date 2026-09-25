/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import type { TemporaryAsset } from '@/features/temporary-assets/lib/asset-utils'

import type { VideoStudioMedia } from '../types'

export function chooseDefaultVideoStudioTokenID(
  current: string,
  tokens: Array<{ id: number }>
): string {
  if (current) return current
  return tokens[0] ? String(tokens[0].id) : ''
}

export function hasUnavailableSelectedAsset(
  media: VideoStudioMedia[],
  assets: TemporaryAsset[],
  nowSeconds = Date.now() / 1000
): boolean {
  const assetsByID = new Map(assets.map((asset) => [asset.id, asset]))

  return media.some((item) => {
    if (item.source !== 'asset') return false

    const asset = assetsByID.get(item.value)
    if (!asset) return true

    const status = asset.status.toUpperCase()
    return (
      !['ACTIVE', 'SUCCESS'].includes(status) ||
      (asset.expires_at > 0 && asset.expires_at <= nowSeconds)
    )
  })
}
